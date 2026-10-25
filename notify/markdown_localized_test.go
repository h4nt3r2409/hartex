package notify

import (
	"strings"
	"testing"
)

// hanFreeItems builds only items with no CJK Han characters (ASCII title, type, summary).
// If labels regress to Chinese, Han characters appear throughout the rendered result, so when the data
// has zero Han characters, the "zero Han characters in the entire output" assertion precisely catches label regressions.
func hanFreeItems(n int) []Item {
	items := make([]Item, 0, n)
	for i := 0; i < n; i++ {
		items = append(items, Item{
			FindingID: int64(i + 1),
			Name:      "login-flaw",
			VulnClass: "SQLi",
			Severity:  "high",
			Summary:   "SQL injection via q param",
			Assets:    []string{"a.example.com"},
		})
	}
	return items
}

// TestMarkdownTitleLocalized verifies that markdownTitle - shared by markdown-family channels (dingtalk, wecom)
// and used together by feishu, html, telegram, and webhook - is Korean.
// If any one regresses to Chinese, the titles across all five channels become mixed-language.
func TestMarkdownTitleLocalized(t *testing.T) {
	// Batch (summary): "취약점 요약 · 총 N건"
	got := markdownTitle(Message{Batch: true, Items: hanFreeItems(3)})
	assertKorean(t, "markdownTitle(batch)", got)
	if !strings.Contains(got, "Vulnerability summary") || !strings.Contains(got, "3 total") {
		t.Errorf("다건 제목이 'Vulnerability summary · 3 total' 형태여야 합니다, 받은 값 %q", got)
	}
	// No items: "Vulnerability alert"
	empty := markdownTitle(Message{})
	assertKorean(t, "markdownTitle(empty)", empty)
	if empty != "Vulnerability alert" {
		t.Errorf("빈 메시지 제목은 'Vulnerability alert' 이어야 합니다, 받은 값 %q", empty)
	}
}

// TestMarkdownBatchIntroLocalized checks that the digest intro (time window, count, overflow notice) is
// Korean. Since item data has zero Han characters, any Han characters in the output mean the intro
// wording has regressed to Chinese.
func TestMarkdownBatchIntroLocalized(t *testing.T) {
	items := hanFreeItems(3)

	// With time window: "최근 N분간 신규 취약점 N건"
	win := markdownBatchIntro(Message{Batch: true, WindowMinutes: 30}, items, 3)
	if hasHan(win) {
		t.Errorf("시간창 머리말에 중국어 한자가 남았습니다: %q", win)
	}
	if !strings.Contains(win, "New vulnerabilities in the last 30 minutes") || !strings.Contains(win, ": 3") {
		t.Errorf("시간창 머리말이 'New vulnerabilities in the last 30 minutes: 3' 형태여야 합니다, 받은 값 %q", win)
	}

	// No time window: "신규 취약점 N건" (no per-minute notation)
	noWin := markdownBatchIntro(Message{Batch: true}, items, 3)
	if hasHan(noWin) {
		t.Errorf("머리말에 중국어 한자가 남았습니다: %q", noWin)
	}
	if !strings.Contains(noWin, "New vulnerabilities: 3") || strings.Contains(noWin, "minutes") {
		t.Errorf("시간창 없는 머리말은 'New vulnerabilities: 3'(분간 표기 없음)이어야 합니다, 받은 값 %q", noWin)
	}

	// When only some fit: "(이 메시지에는 앞 N건만 ... 나머지 N건은 다음 메시지에서 ...)"
	trunc := markdownBatchIntro(Message{Batch: true, WindowMinutes: 30}, items, 5)
	if hasHan(trunc) {
		t.Errorf("초과 안내에 중국어 한자가 남았습니다: %q", trunc)
	}
	for _, want := range []string{"the first 3 items", "the remaining 2", "next message"} {
		if !strings.Contains(trunc, want) {
			t.Errorf("초과 안내에 %q 가 있어야 합니다, 받은 값 %q", want, trunc)
		}
	}
}

// TestWriteItemLabelsLocalized checks that the field labels (status change, type, asset, summary) of the single-item detail render
// (the shared markdown 3-channel path) and the detail link are Korean.
func TestWriteItemLabelsLocalized(t *testing.T) {
	it := Item{
		Name:       "login-flaw",
		VulnClass:  "SQLi",
		Severity:   "high",
		Summary:    "SQL injection via q param",
		Assets:     []string{"a.example.com"},
		DetailURL:  "https://platform.example/finding/1",
		FromStatus: "pending",
		ToStatus:   "fixed",
	}
	var b strings.Builder
	writeItem(&b, it, "", true)
	got := b.String()

	assertKorean(t, "writeItem(single)", got)
	for _, want := range []string{
		"**Status change**: Pending → Fixed",
		"**Type**: SQLi",
		"**Asset**: a.example.com",
		"**Summary**: SQL injection via q param",
		"[View details](https://platform.example/finding/1)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("단건 렌더에 %q 가 있어야 합니다:\n%s", want, got)
		}
	}
}

// TestMarkdownBodyFooterLocalized checks that the platform entry link at the end of the batch body
// is Korean (it is appended only when HomeURL is present).
func TestMarkdownBodyFooterLocalized(t *testing.T) {
	m := Message{Batch: true, HomeURL: "https://platform.example", Items: hanFreeItems(2)}
	body, kept := markdownBody(m, 0)
	if kept != 2 {
		t.Fatalf("한도 없음(0)이면 2건 모두 담겨야 합니다, 받은 값 %d", kept)
	}
	if !strings.Contains(body, "[View all in the platform](https://platform.example)") {
		t.Errorf("본문 끝에 '플랫폼에서 전체 보기' 링크가 있어야 합니다:\n%s", body)
	}
}
