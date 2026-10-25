package notify

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

// This file covers two related hardening measures:
//   1) delivery addresses must not turn the server into a pivot to attack internal networks / cloud metadata (SSRF)
//   2) address-validation errors must not leak credentials embedded in the address
//
// About the test environment: many cases in this package use httptest fake receivers on 127.0.0.1, and the guard blocks them by default.
// So TestMain enables AllowLocalTargetsEnv globally, and each SSRF case below
// explicitly clears it to assert the **deny-by-default** behavior.

func TestMain(m *testing.M) {
	// Let regular cases reach the local fake receivers; SSRF cases clear it themselves temporarily.
	_ = os.Setenv(AllowLocalTargetsEnv, "1")
	os.Exit(m.Run())
}

// TestDialGuardRejectsLoopbackByDefault is the core SSRF-protection assertion:
// under the default config, delivery to a loopback address must be rejected at the **connection layer**.
func TestDialGuardRejectsLoopbackByDefault(t *testing.T) {
	var hit bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hit = true
		_, _ = io.WriteString(w, `{"errcode":0}`)
	}))
	defer srv.Close()

	t.Setenv(AllowLocalTargetsEnv, "") // Disable the escape hatch = default behavior
	_, err := (dingTalkChannel{}).Send(context.Background(),
		map[string]any{"webhook": srv.URL + "/robot/send"}, Message{Items: []Item{{Severity: "high"}}})
	if err == nil {
		t.Fatal("delivery to a loopback address must be blocked by default")
	}
	if hit {
		t.Fatal("the request reached the local service; the guard did not work")
	}
	// The error message must guide users on how to allow it (a local SMTP relay is a legitimate configuration).
	if !strings.Contains(err.Error(), AllowLocalTargetsEnv) {
		t.Errorf("the rejection must explain how to allow it explicitly: %v", err)
	}
}

// TestDialGuardAllowsLoopbackWhenOptedIn is the reverse case: once explicitly enabled it must work,
// otherwise legitimate deployments such as local postfix / intranet relays would be wiped out wholesale.
func TestDialGuardAllowsLoopbackWhenOptedIn(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"errcode":0,"errmsg":"ok"}`)
	}))
	defer srv.Close()

	t.Setenv(AllowLocalTargetsEnv, "1")
	if _, err := (dingTalkChannel{}).Send(context.Background(),
		map[string]any{"webhook": srv.URL + "/robot/send"}, Message{Items: []Item{{Severity: "high"}}}); err != nil {
		t.Fatalf("after explicitly allowing it delivery must work: %v", err)
	}
}

func TestIsBlockedDialIP(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "127.1.2.3", "::1",
		"169.254.169.254", // Cloud metadata endpoint - the main reason this function exists
		"169.254.1.1", "fe80::1",
		"0.0.0.0", "::",
		"224.0.0.1", "ff02::1",
		"::ffff:127.0.0.1", // IPv4-mapped forms must be unmapped before judging; otherwise this is a bypass hole
		"",
	}
	for _, s := range blocked {
		if !isBlockedDialIP(net.ParseIP(s)) {
			t.Errorf("%s must be rejected", s)
		}
	}
	// RFC1918 private networks are **deliberately allowed**: self-hosted Mattermost / SMTP relays on the intranet are common legitimate uses.
	// This assertion pins down that trade-off - if someone later casually adds private-network checks, this fails,
	// forcing a conscious decision (rather than silently killing a set of deployments).
	allowed := []string{"10.0.0.5", "172.16.3.4", "192.168.1.10", "8.8.8.8", "2606:4700::1111"}
	for _, s := range allowed {
		if isBlockedDialIP(net.ParseIP(s)) {
			t.Errorf("%s must be allowed (private networks are common legitimate targets)", s)
		}
	}
}

// TestValidateHTTPURLRejectsLiteralPrivateTargets covers the upfront warning at configuration time:
// literal IPs should be rejected at save time rather than waiting for the first delivery to fail.
func TestValidateHTTPURLRejectsLiteralPrivateTargets(t *testing.T) {
	t.Setenv(AllowLocalTargetsEnv, "")
	for _, raw := range []string{
		"http://127.0.0.1:8080/hook",
		"http://169.254.169.254/latest/meta-data/",
		"http://[::1]:8080/hook",
	} {
		if err := validateHTTPURL(raw); err == nil {
			t.Errorf("%s must be rejected at config time", raw)
		}
	}
	// Public and private addresses pass as usual (private is left to the dial stage, which does not block it).
	for _, raw := range []string{"https://oapi.dingtalk.com/robot/send", "http://10.0.0.9/hook"} {
		if err := validateHTTPURL(raw); err != nil {
			t.Errorf("%s must pass validation: %v", raw, err)
		}
	}
}

// TestValidateHTTPURLErrorNeverLeaksCredentials is the branch the audit noted I missed last round.
//
// When url.Parse **fails** it returns *url.Error, whose Error() contains the full original address. Last round I only
// redacted the error returned by http.Client.Do and missed this; and the "permanent failure path" cases added then
// (file://, gopher://, ftp://) can all actually be parsed successfully by url.Parse and take the scheme branch,
// so all-green proved nothing about this path being safe - a false guarantee.
func TestValidateHTTPURLErrorNeverLeaksCredentials(t *testing.T) {
	cases := []string{
		"http://127.0.0.1/%zz?access_token=" + leakProbeToken,         // Invalid percent-encoding
		"https://a.example.com:port/x?access_token=" + leakProbeToken, // Non-numeric port
		"http://[::1?access_token=" + leakProbeToken,                  // Unmatched brackets
	}
	for _, raw := range cases {
		// First confirm this input **really** makes url.Parse fail. Without this step, the case could
		// unknowingly take a different branch (that is exactly how last round's false guarantee arose).
		if _, err := url.Parse(raw); err == nil {
			t.Errorf("%q should have failed to parse, otherwise this case does not cover the target branch", raw)
			continue
		}
		err := validateHTTPURL(raw)
		if err == nil {
			t.Errorf("%q must fail validation", raw)
			continue
		}
		assertNoSecret(t, err.Error(), leakProbeToken)
	}
	// Confirm the channel-layer wrapping does not carry the address out either.
	t.Setenv(AllowLocalTargetsEnv, "")
	err := (dingTalkChannel{}).Validate(map[string]any{"webhook": cases[0]})
	if err == nil {
		t.Fatal("an invalid address must fail validation")
	}
	assertNoSecret(t, err.Error(), leakProbeToken)
}

// TestEmailDialGuardRejectsLoopbackByDefault covers the SMTP channel's dial guard.
//
// The email channel once used a bare net.Dialer, the only gap in the whole SSRF protection: setting host to
// 169.254.169.254 or 127.0.0.1 connected directly, and when the smtp.NewClient handshake failed it wrapped
// the peer's returned line into the error, which the delivery-history API echoed via last_error - exactly the semi-blind
// read primitive the other channels had closed; the timing difference between "connection refused vs timeout" could also probe ports.
//
// This package's TestMain enables AllowLocalTargetsEnv globally (many cases use fake receivers on 127.0.0.1),
// so this case must clear it itself - otherwise it passes whether or not the guard exists,
// which is precisely why the gap went unnoticed by any test.
func TestEmailDialGuardRejectsLoopbackByDefault(t *testing.T) {
	f := newFakeSMTP(t)
	cfg := emailCfg(t, f, nil)

	t.Setenv(AllowLocalTargetsEnv, "") // Disable the escape hatch = default behavior
	_, err := (emailChannel{}).Send(context.Background(), cfg, singleMsg())
	if err == nil {
		t.Fatal("email delivery to a loopback address must be blocked by default")
	}
	// The connection should never be established: the guard intercepts in the Control hook, so EHLO is never sent.
	if f.sawCommand("EHLO") || f.sawCommand("HELO") {
		t.Fatal("the SMTP session was established; the guard did not work")
	}
	// The error message must guide users on how to allow it (a local postfix relay is a legitimate configuration).
	if !strings.Contains(err.Error(), AllowLocalTargetsEnv) {
		t.Errorf("the rejection must explain how to allow it explicitly: %v", err)
	}
}

// TestEmailDialGuardAllowsLoopbackWhenOptedIn is the paired reverse case: once explicitly enabled
// delivery must work normally. Self-hosted intranet SMTP / local relays are very common deployments, and the guard must not be one-size-fits-all.
func TestEmailDialGuardAllowsLoopbackWhenOptedIn(t *testing.T) {
	f := newFakeSMTP(t)
	cfg := emailCfg(t, f, nil)

	t.Setenv(AllowLocalTargetsEnv, "1")
	if _, err := (emailChannel{}).Send(context.Background(), cfg, singleMsg()); err != nil {
		t.Fatalf("after explicitly allowing it local SMTP delivery must work: %v", err)
	}
	if !f.sawCommand("EHLO") {
		t.Fatal("no EHLO was seen; the session never really started")
	}
}
