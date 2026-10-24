package notify

import (
	"strings"
	"testing"
)

// This file protects the F4 ③ (email body and HTML template) Korean localization against regressions.
// It catches labels in html.go and validation errors in email.go reverting to Chinese.
// The helpers hasHan, hasHangul, and assertKorean live in notify_localized_test.go (same package).
//
// Keep all data (title, type, asset, summary) ASCII. That way the assertion "zero Han characters in the entire output"
// captures only skeleton-label regressions, not content. Severity and status
// labels are already Korean from F4 ① and appear as Hangul in the output (not Han characters), so they pass the assertion.

// TestHTMLItemLabelsLocalized enables every label branch of a single-item email body and then
// verifies the skeleton is Korean and contains no Chinese Han characters.
func TestHTMLItemLabelsLocalized(t *testing.T) {
	m := Message{
		HomeURL: "https://example.com/panel",
		Items: []Item{{
			Name:       "sqli-login", // Title() = Name
			VulnClass:  "injection",  // Must differ from Title() for the type line to render
			Severity:   "high",       // SeverityLabel -> 높음 (Hangul)
			Summary:    "login form is injectable",
			Assets:     []string{"host-a.example.com"},
			DetailURL:  "https://example.com/f/1",
			FromStatus: "pending", // IsStatusChange()=true -> status-change line
			ToStatus:   "fixed",
		}},
	}
	out := htmlBody(m, 0)
	if hasHan(out) {
		t.Errorf("이메일 본문에 중국어 한자가 남아 있습니다:\n%s", out)
	}
	for _, want := range []string{
		"Status change", "Type", "Asset", "Summary", "View details", "View all in the platform",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("이메일 본문에 %q 라벨이 없습니다:\n%s", want, out)
		}
	}
}

// TestHTMLBatchIntroLocalized checks that both branches (with and without a time window) of the batch (digest) intro
// render in Korean.
func TestHTMLBatchIntroLocalized(t *testing.T) {
	items := []Item{{Name: "a", Severity: "high"}, {Name: "b", Severity: "low"}}

	withWindow := htmlBatchIntro(Message{Batch: true, WindowMinutes: 30, Items: items})
	assertKorean(t, "htmlBatchIntro(시간창)", withWindow)
	for _, want := range []string{"New vulnerabilities in the last 30 minutes", "New vulnerabilities", ": 2"} {
		if !strings.Contains(withWindow, want) {
			t.Errorf("시간창 머리말에 %q 가 없습니다: %q", want, withWindow)
		}
	}

	noWindow := htmlBatchIntro(Message{Batch: true, WindowMinutes: 0, Items: items})
	assertKorean(t, "htmlBatchIntro(시간창 없음)", noWindow)
	if !strings.Contains(noWindow, "New vulnerabilities: 2") {
		t.Errorf("시간창 없는 머리말이 %q 를 포함해야 합니다: %q", "New vulnerabilities: 2", noWindow)
	}
	if strings.Contains(noWindow, "minutes") {
		t.Errorf("시간창이 없는데 'minutes' 가 들어갔습니다: %q", noWindow)
	}
}

// TestEmailValidateLocalized checks that the four SMTP config validation errors are Korean
// and that each error tells which field is the problem. These messages are shown verbatim to users
// configuring a notification channel.
func TestEmailValidateLocalized(t *testing.T) {
	cases := []struct {
		name   string
		cfg    map[string]any
		substr string
	}{
		{"서버 주소 누락", map[string]any{"port": float64(25), "from": "a@b.c", "to": []any{"d@e.f"}}, "SMTP"},
		{"포트 범위 벗어남", map[string]any{"host": "h"}, "port"},
		{"발신자 누락", map[string]any{"host": "h", "port": float64(25), "to": []any{"d@e.f"}}, "sender"},
		{"수신자 누락", map[string]any{"host": "h", "port": float64(25), "from": "a@b.c"}, "recipient"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := (emailChannel{}).Validate(tc.cfg)
			if err == nil {
				t.Fatalf("검증이 실패해야 합니다: %v", tc.cfg)
			}
			assertKorean(t, "Validate("+tc.name+")", err.Error())
			if !strings.Contains(err.Error(), tc.substr) {
				t.Errorf("오류 메시지에 %q 가 있어야 합니다, 받은 값 %q", tc.substr, err.Error())
			}
		})
	}
}
