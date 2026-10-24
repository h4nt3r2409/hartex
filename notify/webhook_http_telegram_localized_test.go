package notify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// This file protects the F4 4/5 (telegram, webhook, shared HTTP transport layer) Korean localization
// against regressions. It catches regressions to Chinese in telegram.go's message labels and batch title, webhook.go's config validation errors,
// http.go's transport errors, and the validateHTTPURL helper shared by five channels.
// The helpers hasHan, hasHangul, and assertKorean are defined
// in notify_localized_test.go (same package).
//
// Keep all data (title, type, asset, summary) ASCII. That way the assertion "zero Han characters in the entire output"
// captures only skeleton-label regressions, not content. Severity and status
// labels are already Korean from F4 1 and appear as Hangul in the output (not Han characters), so they pass the assertion.

// TestTelegramItemLabelsLocalized enables every label branch of a single-item Telegram message and then
// verifies the skeleton is Korean and contains no Chinese Han characters.
func TestTelegramItemLabelsLocalized(t *testing.T) {
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
	out, kept := telegramHTML(m)
	if kept != 1 {
		t.Fatalf("단건 메시지는 1 을 보고해야 합니다, 받은 값 %d", kept)
	}
	if hasHan(out) {
		t.Errorf("Telegram 본문에 중국어 한자가 남아 있습니다:\n%s", out)
	}
	for _, want := range []string{"Status change", "Type", "Assets", "Summary", "View details"} {
		if !strings.Contains(out, want) {
			t.Errorf("Telegram 본문에 %q 라벨이 없습니다:\n%s", want, out)
		}
	}
}

// TestTelegramBatchTitleLocalized checks that the overflow and time-window branches of the batch (digest) title
// render in Korean. It must match markdownTitle character for character so channels do not become
// mixed-language.
func TestTelegramBatchTitleLocalized(t *testing.T) {
	items := []Item{{Name: "a", Severity: "high"}, {Name: "b", Severity: "low"}}

	// With overflow + time window: summary, total count, first N items, remainder, next message, last N minutes.
	over := telegramBatchTitle(Message{WindowMinutes: 30}, items, 5)
	assertKorean(t, "telegramBatchTitle(넘침)", over)
	for _, want := range []string{"Vulnerability summary · 5 total", "showing only the first 2", "the remaining 3", "next message", "Last 30 minutes"} {
		if !strings.Contains(over, want) {
			t.Errorf("넘침 제목에 %q 가 없습니다: %q", want, over)
		}
	}

	// No overflow + no time window: there must be no next-message notice.
	full := telegramBatchTitle(Message{}, items, 2)
	assertKorean(t, "telegramBatchTitle(전량)", full)
	if !strings.Contains(full, "Vulnerability summary · 2 total") {
		t.Errorf("전량 제목에 'Vulnerability summary · 2 total' 이 있어야 합니다: %q", full)
	}
	if strings.Contains(full, "next message") {
		t.Errorf("넘치지 않았는데 'next message' 안내가 들어갔습니다: %q", full)
	}
}

// TestTelegramValidateLocalized checks that required-field-missing errors are Korean and tell
// which field is empty. Technical terms Bot Token and Chat ID are preserved verbatim.
func TestTelegramValidateLocalized(t *testing.T) {
	if err := (telegramChannel{}).Validate(map[string]any{}); err == nil {
		t.Fatal("bot_token 누락은 검증 실패여야 합니다")
	} else {
		assertKorean(t, "telegram Validate(bot_token)", err.Error())
		if !strings.Contains(err.Error(), "Bot Token") {
			t.Errorf("Bot Token 누락을 알려줘야 합니다: %q", err.Error())
		}
	}
	if err := (telegramChannel{}).Validate(map[string]any{"bot_token": "t"}); err == nil {
		t.Fatal("chat_id 누락은 검증 실패여야 합니다")
	} else {
		assertKorean(t, "telegram Validate(chat_id)", err.Error())
		if !strings.Contains(err.Error(), "Chat ID") {
			t.Errorf("Chat ID 누락을 알려줘야 합니다: %q", err.Error())
		}
	}
}

// TestWebhookValidateLocalized checks that the three generic Webhook config validation errors are Korean
// and that each error tells the cause of the problem.
func TestWebhookValidateLocalized(t *testing.T) {
	if err := (webhookChannel{}).Validate(map[string]any{}); err == nil {
		t.Fatal("url 누락은 검증 실패여야 합니다")
	} else {
		assertKorean(t, "webhook Validate(url)", err.Error())
		if !strings.Contains(err.Error(), "target URL") {
			t.Errorf("대상 URL 누락을 알려줘야 합니다: %q", err.Error())
		}
	}
	// Unsupported method: Korean + expose the allowed list.
	if err := (webhookChannel{}).Validate(map[string]any{"url": "https://example.com/hook", "method": "DELETE"}); err == nil {
		t.Fatal("DELETE 는 검증 실패여야 합니다")
	} else {
		assertKorean(t, "webhook Validate(method)", err.Error())
		if !strings.Contains(err.Error(), "GET/POST/PUT/PATCH") {
			t.Errorf("허용 메서드 목록을 알려줘야 합니다: %q", err.Error())
		}
	}
	// Template syntax error: Korean (the wrapped cause is a Go template error, so it has no Han characters).
	if err := (webhookChannel{}).Validate(map[string]any{"url": "https://example.com/hook", "body_template": "{{"}); err == nil {
		t.Fatal("깨진 템플릿은 검증 실패여야 합니다")
	} else {
		assertKorean(t, "webhook Validate(template)", err.Error())
		if !strings.Contains(err.Error(), "template") {
			t.Errorf("템플릿 오류임을 알려줘야 합니다: %q", err.Error())
		}
	}
}

// TestValidateHTTPURLLocalized checks that the error from the URL validation helper shared by five channels
// (telegram, webhook, dingtalk, feishu, wecom) is Korean. If this helper remained Chinese,
// it would mix with the Korean prefix ("API 주소가 올바르지 않습니다: ...") into half-Korean half-Chinese text.
func TestValidateHTTPURLLocalized(t *testing.T) {
	// Unsupported scheme.
	if err := validateHTTPURL("ftp://example.com/x"); err == nil {
		t.Fatal("ftp 스킴은 거부되어야 합니다")
	} else {
		assertKorean(t, "validateHTTPURL(scheme)", err.Error())
		if !strings.Contains(err.Error(), "http") {
			t.Errorf("지원 스킴(http/https)을 알려줘야 합니다: %q", err.Error())
		}
	}
	// No hostname.
	if err := validateHTTPURL("http://"); err == nil {
		t.Fatal("호스트 없는 주소는 거부되어야 합니다")
	} else {
		assertKorean(t, "validateHTTPURL(host)", err.Error())
	}
}

// TestHTTPLocalTargetBlockedLocalized checks that the local/link-local address blocking error is Korean
// and tells the workaround (environment variable).
func TestHTTPLocalTargetBlockedLocalized(t *testing.T) {
	t.Setenv(AllowLocalTargetsEnv, "") // Explicitly disable it to take the blocking path
	err := blockInternalDial("tcp", "127.0.0.1:25", nil)
	if err == nil {
		t.Fatal("로컬 주소는 기본적으로 차단되어야 합니다")
	}
	assertKorean(t, "blockInternalDial", err.Error())
	if !strings.Contains(err.Error(), AllowLocalTargetsEnv) {
		t.Errorf("차단 오류는 우회 방법(%s)을 알려줘야 합니다: %q", AllowLocalTargetsEnv, err.Error())
	}
}

// TestHTTPRedactHelpersLocalized checks that the address-redaction placeholder and the transport error's
// "알 수 없는 오류" branch are Korean.
func TestHTTPRedactHelpersLocalized(t *testing.T) {
	if got := redactRequestTarget(""); !strings.Contains(got, "cannot parse address") {
		t.Errorf("해석 불가 주소는 한국어 자리표시자여야 합니다, 받은 값 %q", got)
	}
	// The branch where *url.Error's Err is nil.
	got := redactTransportError(&url.Error{Op: "Get", URL: "http://api.example", Err: nil})
	if hasHan(got) {
		t.Errorf("전송 오류 탈감 결과에 한자가 남았습니다: %q", got)
	}
	if !strings.Contains(got, "unknown error") {
		t.Errorf("Err 가 nil 이면 'unknown error' 여야 합니다, 받은 값 %q", got)
	}
}

// TestHTTPStatusErrorsLocalized uses a real doJSON round trip to check that the user-facing errors
// for status-code classification (rejection/server error/rate limit) are Korean. Transport to a 127.0.0.1 httptest
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
			t.Fatalf("HTTP %d 는 오류여야 합니다", tc.code)
		}
		assertKorean(t, "doJSON(HTTP status)", err.Error())
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("HTTP %d 오류에 %q 가 있어야 합니다, 받은 값 %q", tc.code, tc.want, err.Error())
		}
	}
}
