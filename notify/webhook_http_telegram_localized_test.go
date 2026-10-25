package notify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// This file protects the F4 4/5 (telegram, webhook, shared HTTP transport layer) English-only output
// against regressions. It catches regressions to Chinese in telegram.go's message labels and batch title, webhook.go's config validation errors,
// http.go's transport errors, and the validateHTTPURL helper shared by five channels.
// The helpers hasHan, hasHangul, and assertEnglish are defined
// in notify_localized_test.go (same package).
//
// Keep all data (title, type, asset, summary) ASCII. That way the assertion "zero Han characters in the entire output"
// captures only skeleton-label regressions, not content. Severity and status
// labels are already English from F4 1, so they pass the assertion.

// TestTelegramItemLabelsLocalized enables every label branch of a single-item Telegram message and then
// verifies the skeleton is English-only and contains no Chinese Han characters.
func TestTelegramItemLabelsLocalized(t *testing.T) {
	m := Message{
		HomeURL: "https://example.com/panel",
		Items: []Item{{
			Name:       "sqli-login", // Title() = Name
			VulnClass:  "injection",  // Must differ from Title() for the type line to render
			Severity:   "high",       // SeverityLabel -> High (English)
			Summary:    "login form is injectable",
			Assets:     []string{"host-a.example.com"},
			DetailURL:  "https://example.com/f/1",
			FromStatus: "pending", // IsStatusChange()=true -> status-change line
			ToStatus:   "fixed",
		}},
	}
	out, kept := telegramHTML(m)
	if kept != 1 {
		t.Fatalf("a single-item message must report 1, got %d", kept)
	}
	if hasHan(out) {
		t.Errorf("Han characters remain in the Telegram body:\n%s", out)
	}
	for _, want := range []string{"Status change", "Type", "Assets", "Summary", "View details"} {
		if !strings.Contains(out, want) {
			t.Errorf("the Telegram body is missing the %q label:\n%s", want, out)
		}
	}
}

// TestTelegramBatchTitleLocalized checks that the overflow and time-window branches of the batch (digest) title
// render in English. It must match markdownTitle character for character so channels do not become
// mixed-language.
func TestTelegramBatchTitleLocalized(t *testing.T) {
	items := []Item{{Name: "a", Severity: "high"}, {Name: "b", Severity: "low"}}

	// With overflow + time window: summary, total count, first N items, remainder, next message, last N minutes.
	over := telegramBatchTitle(Message{WindowMinutes: 30}, items, 5)
	assertEnglish(t, "telegramBatchTitle(overflow)", over)
	for _, want := range []string{"Vulnerability summary · 5 total", "showing only the first 2", "the remaining 3", "next message", "Last 30 minutes"} {
		if !strings.Contains(over, want) {
			t.Errorf("the overflow title is missing %q: %q", want, over)
		}
	}

	// No overflow + no time window: there must be no next-message notice.
	full := telegramBatchTitle(Message{}, items, 2)
	assertEnglish(t, "telegramBatchTitle(all fit)", full)
	if !strings.Contains(full, "Vulnerability summary · 2 total") {
		t.Errorf("the all-fit title must contain 'Vulnerability summary · 2 total': %q", full)
	}
	if strings.Contains(full, "next message") {
		t.Errorf("a next-message notice appeared although nothing overflowed: %q", full)
	}
}

// TestTelegramValidateLocalized checks that required-field-missing errors are English-only and tell
// which field is empty. Technical terms Bot Token and Chat ID are preserved verbatim.
func TestTelegramValidateLocalized(t *testing.T) {
	if err := (telegramChannel{}).Validate(map[string]any{}); err == nil {
		t.Fatal("a missing bot_token must fail validation")
	} else {
		assertEnglish(t, "telegram Validate(bot_token)", err.Error())
		if !strings.Contains(err.Error(), "Bot Token") {
			t.Errorf("the error must point out the missing Bot Token: %q", err.Error())
		}
	}
	if err := (telegramChannel{}).Validate(map[string]any{"bot_token": "t"}); err == nil {
		t.Fatal("a missing chat_id must fail validation")
	} else {
		assertEnglish(t, "telegram Validate(chat_id)", err.Error())
		if !strings.Contains(err.Error(), "Chat ID") {
			t.Errorf("the error must point out the missing Chat ID: %q", err.Error())
		}
	}
}

// TestWebhookValidateLocalized checks that the three generic Webhook config validation errors are English
// and that each error tells the cause of the problem.
func TestWebhookValidateLocalized(t *testing.T) {
	if err := (webhookChannel{}).Validate(map[string]any{}); err == nil {
		t.Fatal("a missing url must fail validation")
	} else {
		assertEnglish(t, "webhook Validate(url)", err.Error())
		if !strings.Contains(err.Error(), "target URL") {
			t.Errorf("the error must point out the missing target URL: %q", err.Error())
		}
	}
	// Unsupported method: English + expose the allowed list.
	if err := (webhookChannel{}).Validate(map[string]any{"url": "https://example.com/hook", "method": "DELETE"}); err == nil {
		t.Fatal("DELETE must fail validation")
	} else {
		assertEnglish(t, "webhook Validate(method)", err.Error())
		if !strings.Contains(err.Error(), "GET/POST/PUT/PATCH") {
			t.Errorf("the error must list the allowed methods: %q", err.Error())
		}
	}
	// Template syntax error: English (the wrapped cause is a Go template error, so it has no Han characters).
	if err := (webhookChannel{}).Validate(map[string]any{"url": "https://example.com/hook", "body_template": "{{"}); err == nil {
		t.Fatal("a broken template must fail validation")
	} else {
		assertEnglish(t, "webhook Validate(template)", err.Error())
		if !strings.Contains(err.Error(), "template") {
			t.Errorf("the error must say the template is bad: %q", err.Error())
		}
	}
}

// TestValidateHTTPURLLocalized checks that the error from the URL validation helper shared by five channels
// (telegram, webhook, dingtalk, feishu, wecom) is English. If this helper regressed to Chinese,
// it would mix with the English prefix into mixed-language text.
func TestValidateHTTPURLLocalized(t *testing.T) {
	// Unsupported scheme.
	if err := validateHTTPURL("ftp://example.com/x"); err == nil {
		t.Fatal("the ftp scheme must be rejected")
	} else {
		assertEnglish(t, "validateHTTPURL(scheme)", err.Error())
		if !strings.Contains(err.Error(), "http") {
			t.Errorf("the error must list the supported schemes (http/https): %q", err.Error())
		}
	}
	// No hostname.
	if err := validateHTTPURL("http://"); err == nil {
		t.Fatal("an address without a host must be rejected")
	} else {
		assertEnglish(t, "validateHTTPURL(host)", err.Error())
	}
}

// TestHTTPLocalTargetBlockedLocalized checks that the local/link-local address blocking error is English
// and tells the workaround (environment variable).
func TestHTTPLocalTargetBlockedLocalized(t *testing.T) {
	t.Setenv(AllowLocalTargetsEnv, "") // Explicitly disable it to take the blocking path
	err := blockInternalDial("tcp", "127.0.0.1:25", nil)
	if err == nil {
		t.Fatal("a local address must be blocked by default")
	}
	assertEnglish(t, "blockInternalDial", err.Error())
	if !strings.Contains(err.Error(), AllowLocalTargetsEnv) {
		t.Errorf("the blocking error must name the workaround (%s): %q", AllowLocalTargetsEnv, err.Error())
	}
}

// TestHTTPRedactHelpersLocalized checks that the address-redaction placeholder and the transport error's
// "unknown error" branch are English.
func TestHTTPRedactHelpersLocalized(t *testing.T) {
	if got := redactRequestTarget(""); !strings.Contains(got, "cannot parse address") {
		t.Errorf("an unparseable address must yield the English placeholder, got %q", got)
	}
	// The branch where *url.Error's Err is nil.
	got := redactTransportError(&url.Error{Op: "Get", URL: "http://api.example", Err: nil})
	if hasHan(got) {
		t.Errorf("Han characters remain in the redacted transport error: %q", got)
	}
	if !strings.Contains(got, "unknown error") {
		t.Errorf("when Err is nil the text must be 'unknown error', got %q", got)
	}
}

// TestHTTPStatusErrorsLocalized uses a real doJSON round trip to check that the user-facing errors
// for status-code classification (rejection/server error/rate limit) are English. Transport to a 127.0.0.1 httptest
// is blocked by default, so it is explicitly allowed only in this test.
func TestHTTPStatusErrorsLocalized(t *testing.T) {
	t.Setenv(AllowLocalTargetsEnv, "1")
	cases := []struct {
		code int
		want string
	}{
		{http.StatusForbidden, "rejected the request"},
		{http.StatusInternalServerError, "returned an error"},
		{http.StatusTooManyRequests, "rate-limited the request"},
	}
	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.code)
			_, _ = w.Write([]byte("body"))
		}))
		_, err := doJSON(context.Background(), http.MethodPost, srv.URL, nil, map[string]any{"a": 1})
		srv.Close()
		if err == nil {
			t.Fatalf("HTTP %d must be an error", tc.code)
		}
		assertEnglish(t, "doJSON(HTTP status)", err.Error())
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("the HTTP %d error must contain %q, got %q", tc.code, tc.want, err.Error())
		}
	}
}
