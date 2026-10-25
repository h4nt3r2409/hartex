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
// and used together by feishu, html, telegram, and webhook - is English-only.
// If any one regresses to Chinese, the titles across all five channels become mixed-language.
func TestMarkdownTitleLocalized(t *testing.T) {
	// Batch (summary): "Vulnerability summary · N total"
	got := markdownTitle(Message{Batch: true, Items: hanFreeItems(3)})
	assertEnglish(t, "markdownTitle(batch)", got)
	if !strings.Contains(got, "Vulnerability summary") || !strings.Contains(got, "3 total") {
		t.Errorf("batch title must look like 'Vulnerability summary · 3 total', got %q", got)
	}
	// No items: "Vulnerability alert"
	empty := markdownTitle(Message{})
	assertEnglish(t, "markdownTitle(empty)", empty)
	if empty != "Vulnerability alert" {
		t.Errorf("empty message title must be 'Vulnerability alert', got %q", empty)
	}
}

// TestMarkdownBatchIntroLocalized checks that the digest intro (time window, count, overflow notice) is
// English-only. Since item data has zero Han characters, any Han characters in the output mean the intro
// wording has regressed to Chinese.
func TestMarkdownBatchIntroLocalized(t *testing.T) {
	items := hanFreeItems(3)

	// With time window: "New vulnerabilities in the last N minutes: N"
	win := markdownBatchIntro(Message{Batch: true, WindowMinutes: 30}, items, 3)
	assertEnglish(t, "markdownBatchIntro(window)", win)
	if !strings.Contains(win, "New vulnerabilities in the last 30 minutes") || !strings.Contains(win, ": 3") {
		t.Errorf("time-window intro must look like 'New vulnerabilities in the last 30 minutes: 3', got %q", win)
	}

	// No time window: "New vulnerabilities: N" (no per-minute notation)
	noWin := markdownBatchIntro(Message{Batch: true}, items, 3)
	assertEnglish(t, "markdownBatchIntro(no window)", noWin)
	if !strings.Contains(noWin, "New vulnerabilities: 3") || strings.Contains(noWin, "minutes") {
		t.Errorf("intro without a window must be 'New vulnerabilities: 3' (no minute notation), got %q", noWin)
	}

	// When only some fit: "(this message shows only the first N items; the remaining N will be sent in the next message)"
	trunc := markdownBatchIntro(Message{Batch: true, WindowMinutes: 30}, items, 5)
	assertEnglish(t, "markdownBatchIntro(overflow)", trunc)
	for _, want := range []string{"the first 3 items", "the remaining 2", "next message"} {
		if !strings.Contains(trunc, want) {
			t.Errorf("overflow notice must contain %q, got %q", want, trunc)
		}
	}
}

// TestWriteItemLabelsLocalized checks that the field labels (status change, type, asset, summary) of the single-item detail render
// (the shared markdown 3-channel path) and the detail link are English-only.
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

	assertEnglish(t, "writeItem(single)", got)
	for _, want := range []string{
		"**Status change**: Pending → Fixed",
		"**Type**: SQLi",
		"**Asset**: a.example.com",
		"**Summary**: SQL injection via q param",
		"[View details](https://platform.example/finding/1)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("single-item render must contain %q:\n%s", want, got)
		}
	}
}

// TestMarkdownBodyFooterLocalized checks that the platform entry link at the end of the batch body
// is English-only (it is appended only when HomeURL is present).
func TestMarkdownBodyFooterLocalized(t *testing.T) {
	m := Message{Batch: true, HomeURL: "https://platform.example", Items: hanFreeItems(2)}
	body, kept := markdownBody(m, 0)
	if kept != 2 {
		t.Fatalf("with no limit (0) both items must fit, got %d", kept)
	}
	if !strings.Contains(body, "[View all in the platform](https://platform.example)") {
		t.Errorf("the body must end with the '[View all in the platform]' link:\n%s", body)
	}
}
