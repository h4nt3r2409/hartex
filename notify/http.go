package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"syscall"
	"time"
)

// allowLocalTargets determines whether delivering messages to loopback /
// link-local addresses is allowed.
//
// Denied by default. These address ranges are not where IM bots or public email
// servers live, and what they can reach is very sensitive: the management port
// of another service on the same machine, and cloud environment metadata
// endpoints (169.254.169.254, from which instance credentials can be read). The
// delivery address is configured by an administrator, but an admin session
// hijacked via XSS/CSRF, or a second person sharing the same JWT, can read back
// response content by changing the configuration — doJSON writes the first 200
// bytes of a 4xx/5xx response body into last_error, and the delivery history
// endpoint echoes it, which is a semi-blind read primitive.
//
// But a "local SMTP relay" (postfix on 127.0.0.1:25) is a common setup for
// self-hosted email, and a blanket block would get in people's way. So an
// explicit escape hatch is provided rather than hardcoding an allowance: setting
// ARTEX_NOTIFY_ALLOW_LOCAL=1 allows it.
//
// It is exported as AllowLocalTargetsEnv so tests can turn it on explicitly — the
// test cases in this package and the server package make heavy use of httptest
// fake receivers on 127.0.0.1, and without turning it on they would all be
// stopped by the guard.
const AllowLocalTargetsEnv = "ARTEX_NOTIFY_ALLOW_LOCAL"

func allowLocalTargets() bool {
	v := strings.TrimSpace(os.Getenv(AllowLocalTargetsEnv))
	return v == "1" || strings.EqualFold(v, "true")
}

// isBlockedDialIP reports whether the target IP belongs to an address range that
// is "not allowed for delivery by default".
//
// Only loopback, link-local (including the cloud metadata address
// 169.254.169.254), unspecified, and multicast are rejected. RFC1918 private
// networks are **not rejected**: a self-hosted internal Mattermost / SMTP relay
// is a very common legitimate use, and blocking them as well would make the
// feature directly unusable in real environments. This trade-off is deliberate —
// the protection must stop genuinely sensitive targets without also disabling
// normal deployments.
func isBlockedDialIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	// IPv4-mapped IPv6 (::ffff:127.0.0.1) must be converted back to IPv4 before
	// checking, otherwise the check is bypassed.
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	return ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast()
}

// blockInternalDial is the Control hook of the http.Transport dialer, checking
// the target address **when the connection is established**.
//
// Why set it at the dial stage rather than only validating when saving
// configuration: this is the point where it finally takes effect. It also covers
// two cases that bypass configuration validation — DNS rebinding (resolving to a
// public IP at validation time and to an internal IP at connection time) and
// redirects (although cross-host redirects are already rejected, same-host
// redirects may still point the path elsewhere).
func blockInternalDial(_, address string, _ syscall.RawConn) error {
	if allowLocalTargets() {
		return nil
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("cannot resolve destination address: %q", host)
	}
	if isBlockedDialIP(ip) {
		return fmt.Errorf("not sending to local or link-local address %s (if you need to send to a local service, set %s=1)", ip, AllowLocalTargetsEnv)
	}
	return nil
}

// notifyTransport adds only a dial guard on top of the default Transport.
// Clone preserves all of the default tuning (connection pool, HTTP/2, timeouts,
// proxy, etc.), avoiding changes to other behavior just to add one check.
var notifyTransport = func() *http.Transport {
	t, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return &http.Transport{}
	}
	clone := t.Clone()
	clone.DialContext = (&net.Dialer{Timeout: 10 * time.Second, Control: blockInternalDial}).DialContext
	return clone
}()

// httpClient is the client shared by all channel deliveries.
//
// It deliberately does **not** reuse the project's global egress proxy (the
// server-side GlobalProxy): that proxy is for penetration target traffic and is
// often an unstable tunnel, and notification availability should not be held
// hostage by jitter in the target network. Direct IM push is fine. The timeout is
// set to 15 seconds — a peer slower than that is effectively already in a failed
// state.
//
// Cross-host redirects are rejected: the delivery addresses of this feature are
// all in the form of "one fixed endpoint", and normally never redirect to another
// host; while the credentials of these providers (DingTalk access_token, WeCom
// key, Telegram bot token) are **in the URL**, following a cross-host redirect
// amounts to handing the credentials to the redirect target. Same-host redirects
// (e.g. appending a trailing slash) are still allowed.
var httpClient = &http.Client{
	Timeout:   15 * time.Second,
	Transport: notifyTransport,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("too many redirects")
		}
		if len(via) > 0 && req.URL.Host != via[0].URL.Host {
			return fmt.Errorf("redirects to a different host are not allowed (%s → %s)", via[0].URL.Host, req.URL.Host)
		}
		return nil
	},
}

// respBodyLimit limits the size of the response body read. An abnormal peer may
// spit back an oversized body, while we only need the error code and a short
// error description to display in the delivery history.
const respBodyLimit = 8 << 10

// doJSON sends one request and returns the response body (already length-limited).
//
// When payload is nil an empty body is sent (for GET or scenarios where the
// platform does not require a body).
// The key-value pairs in headers are attached as-is, for custom headers of a
// generic webhook.
//
// Error classification is the core responsibility of this function: network-layer
// failures and 5xx/408/429 are classed as "retryable", and the remaining 4xx as
// "permanent failures" — retrying a 403 merely writes the same error to the log 3
// times.
func doJSON(ctx context.Context, method, url string, headers map[string]string, payload any) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			// Serialization failure is a local bug (wrong configuration field
			// type); retrying will not make it better.
			return nil, Permanent(fmt.Errorf("failed to build request body: %w", err))
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		// Invalid URL — most likely the user entered the address wrong; this is a
		// permanent failure.
		// Here too err must not be passed through: the url.Parse error text contains
		// the full address.
		return nil, Permanent(fmt.Errorf("invalid request URL: %s", redactRequestTarget(url)))
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		// Connection refused, DNS failure, timeout — mostly transient failures,
		// left to backoff retries.
		//
		// The error text must be redacted before going out. Reason:
		// http.Client.Do returns a *url.Error, whose Error() is `Op "fullURL":
		// underlying error`, and the credentials of these providers are **in the
		// URL** (DingTalk access_token, WeCom key, Feishu hook id, Telegram
		// /bot<token>/). Without redaction, credentials would flow along this error
		// string to four places: notification_deliveries' last_error (stored in
		// plaintext), the delivery history endpoint's response (**bypassing channel
		// configuration masking**), server logs, and the 502 text returned to the
		// frontend by the test-send endpoint.
		return nil, fmt.Errorf("request failed: %s", redactTransportError(err))
	}
	defer resp.Body.Close()
	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, respBodyLimit))
	if readErr != nil {
		return nil, fmt.Errorf("failed to read response: %w", readErr)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return raw, nil
	}
	// 429 (rate limited) and 408 (timeout) are worth retrying; the remaining 4xx
	// are configuration or permission problems, and retrying is pointless.
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusRequestTimeout {
		return nil, fmt.Errorf("the remote server rate-limited the request or the response was delayed (HTTP %d): %s", resp.StatusCode, snippet(raw))
	}
	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("the remote server returned an error (HTTP %d): %s", resp.StatusCode, snippet(raw))
	}
	return nil, Permanent(fmt.Errorf("the remote server rejected the request (HTTP %d): %s", resp.StatusCode, snippet(raw)))
}

// snippet compresses the response body into a short one-line text for error
// messages. The response may contain newlines and a lot of whitespace, and
// stuffing it directly into last_error would break the layout of the delivery
// history page.
func snippet(raw []byte) string {
	return OneLine(string(raw), 200)
}

// redactRequestTarget compresses the delivery address into "scheme://host/…" for
// error messages.
//
// This is the package's only address-redaction policy, and it is deliberately
// **crude enough**: everything except scheme and host is discarded. The reason is
// that there is no "universal and safe" way to determine which part of a URL is a
// credential:
//
//	DingTalk  credential in query      /robot/send?access_token=xxx
//	WeCom     credential in query      /cgi-bin/webhook/send?key=xxx
//	Feishu    credential in the **last path segment** /open-apis/bot/v2/hook/<hook_id>
//	Telegram  credential in the **middle path segment** /bot<token>/sendMessage
//
// To "keep only the useful part" you would have to patch per channel, and missing
// any one of them is a credential leak. Keeping the host is already enough for
// troubleshooting (DNS resolution failure, unable to connect, and bad certificate
// can all be located), and which bot it is can be recognized from the masked tail
// hint in the channel configuration.
//
// On parse failure it returns a fixed placeholder — the raw string is never echoed
// back.
func redactRequestTarget(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "(cannot parse address)"
	}
	return u.Scheme + "://" + u.Host + "/…"
}

// redactTransportError strips the address from a transport-layer error, keeping
// only the underlying cause.
//
// The structure of *url.Error is {Op, URL, Err}, and Error() prints the URL along
// with it. Here the Err field is taken explicitly to bypass its Error() — more
// reliable than post-hoc string replacement, because replacement would have to
// correctly handle the various URL-encoded/escaped variants and easily misses
// some.
func redactTransportError(err error) string {
	var uerr *url.Error
	if errors.As(err, &uerr) {
		host := ""
		if u, parseErr := url.Parse(uerr.URL); parseErr == nil {
			host = u.Host
		}
		if uerr.Err != nil {
			return fmt.Sprintf("%s %s: %s", uerr.Op, host, uerr.Err)
		}
		return fmt.Sprintf("%s %s: unknown error", uerr.Op, host)
	}
	// Non-*url.Error (e.g. errors returned by the redirect policy) may also carry
	// an address; route them all through redaction.
	return redactURLsInText(err.Error())
}

// redactURLsInText replaces http(s) addresses appearing in a piece of text with
// the redacted form.
//
// Used to catch errors whose structured fields are unavailable (redirect policy
// errors, custom errors from third-party libraries). Only the http/https prefix
// is recognized, split on whitespace and quotes — addresses cannot contain these
// characters.
func redactURLsInText(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		rest := s[i:]
		if strings.HasPrefix(rest, "http://") || strings.HasPrefix(rest, "https://") {
			end := len(rest)
			if j := strings.IndexAny(rest, " \t\n\"'"); j >= 0 {
				end = j
			}
			b.WriteString(redactRequestTarget(rest[:end]))
			i += end
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}
