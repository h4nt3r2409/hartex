package notify

import (
	"strings"
	"testing"
)

// This file protects the English-only email body and HTML template skeleton against
// regressions. It catches labels in html.go and validation errors in email.go
// reverting to Chinese or Korean.
// The helpers hasHan, hasHangul, and assertEnglish live in notify_localized_test.go (same package).
//
// Keep all data (title, type, asset, summary) ASCII. That way the assertion "zero Han characters in the entire output"
// captures only skeleton-label regressions, not content. Severity and status
// labels are English, so they pass the assertion.

// TestHTMLItemLabelsLocalized enables every label branch of a single-item email body and then
// verifies the skeleton is English-only and contains no Chinese Han characters.
func TestHTMLItemLabelsLocalized(t *testing.T) {
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
	out := htmlBody(m, 0)
	assertEnglish(t, "htmlBody(single)", out)
	for _, want := range []string{
		"Status change", "Type", "Asset", "Summary", "View details", "View all in the platform",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("email body is missing the %q label:\n%s", want, out)
		}
	}
}

// TestHTMLBatchIntroLocalized checks that both branches (with and without a time window) of the batch (digest) intro
// render in English.
func TestHTMLBatchIntroLocalized(t *testing.T) {
	items := []Item{{Name: "a", Severity: "high"}, {Name: "b", Severity: "low"}}

	withWindow := htmlBatchIntro(Message{Batch: true, WindowMinutes: 30, Items: items})
	assertEnglish(t, "htmlBatchIntro(window)", withWindow)
	for _, want := range []string{"New vulnerabilities in the last 30 minutes", "New vulnerabilities", ": 2"} {
		if !strings.Contains(withWindow, want) {
			t.Errorf("time-window intro is missing %q: %q", want, withWindow)
		}
	}

	noWindow := htmlBatchIntro(Message{Batch: true, WindowMinutes: 0, Items: items})
	assertEnglish(t, "htmlBatchIntro(no window)", noWindow)
	if !strings.Contains(noWindow, "New vulnerabilities: 2") {
		t.Errorf("intro without a window must contain %q: %q", "New vulnerabilities: 2", noWindow)
	}
	if strings.Contains(noWindow, "minutes") {
		t.Errorf("intro without a window must not mention 'minutes': %q", noWindow)
	}
}

// TestEmailValidateLocalized checks that the four SMTP config validation errors are English-only
// and that each error tells which field is the problem. These messages are shown verbatim to users
// configuring a notification channel.
func TestEmailValidateLocalized(t *testing.T) {
	cases := []struct {
		name   string
		cfg    map[string]any
		substr string
	}{
		{"missing server address", map[string]any{"port": float64(25), "from": "a@b.c", "to": []any{"d@e.f"}}, "SMTP"},
		{"port out of range", map[string]any{"host": "h"}, "port"},
		{"missing sender", map[string]any{"host": "h", "port": float64(25), "to": []any{"d@e.f"}}, "sender"},
		{"missing recipient", map[string]any{"host": "h", "port": float64(25), "from": "a@b.c"}, "recipient"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := (emailChannel{}).Validate(tc.cfg)
			if err == nil {
				t.Fatalf("validation must fail: %v", tc.cfg)
			}
			assertEnglish(t, "Validate("+tc.name+")", err.Error())
			if !strings.Contains(err.Error(), tc.substr) {
				t.Errorf("error message must contain %q, got %q", tc.substr, err.Error())
			}
		})
	}
}
