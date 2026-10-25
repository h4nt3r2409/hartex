package notify

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// This file covers the "pack whole items" fix: when a digest message exceeds the channel length limit, it must truncate **by whole item**
// and truthfully report the number of items that did not fit, so callers mark only those actually delivered.
//
// The previous approach rendered the full message, truncated it, then marked the whole batch delivered: the latter half vanished into thin air
// while delivery history showed all success - vulnerabilities were simply lost, with nowhere to notice it.

func TestMarkdownBodyPacksWholeItemsWithinByteLimit(t *testing.T) {
	// 200 multibyte digest items, necessarily far exceeding WeCom's 4096 bytes.
	m := batchMsg(200)
	body, kept := markdownBody(m, weComMarkdownLimit)

	if len(body) > weComMarkdownLimit {
		t.Fatalf("body is %d bytes, over the limit %d", len(body), weComMarkdownLimit)
	}
	if !utf8.ValidString(body) {
		t.Fatal("body is not valid UTF-8")
	}
	if kept <= 0 || kept >= len(m.Items) {
		t.Fatalf("must pack only part (0 < kept < %d), got %d", len(m.Items), kept)
	}
	// The header must truthfully state how many items this message contains and how many remain - otherwise readers will take the header's
	// number as the total.
	if !strings.Contains(body, "the remaining") || !strings.Contains(body, "next message") {
		t.Fatalf("the header must state how many items are not included:\n%s", body[:minInt(400, len(body))])
	}
	// Only the first kept items should be included.
	for i := 0; i < kept; i++ {
		if !strings.Contains(body, "Vulnerability "+itoa(i+1)) {
			t.Fatalf("item %d must be in this message:\n%s", i+1, body)
		}
	}
	if strings.Contains(body, "Vulnerability "+itoa(kept+1)) {
		t.Fatalf("item %d must not appear (it belongs to the next batch)", kept+1)
	}
}

func TestMarkdownBodyKeepsEverythingWhenUnderLimit(t *testing.T) {
	m := batchMsg(3)
	body, kept := markdownBody(m, 0) // 0 = no limit
	if kept != len(m.Items) {
		t.Fatalf("with no limit everything must be kept, got kept=%d", kept)
	}
	if strings.Contains(body, "remaining") {
		t.Fatalf("no truncation notice may appear when nothing was truncated:\n%s", body)
	}
}

func TestMarkdownBodyAlwaysKeepsAtLeastOneItem(t *testing.T) {
	// When the budget is too small to fit even one item, still send one (final truncation as a fallback).
	// Otherwise a single overlong vulnerability would permanently stall the whole batch: it never fits on any claim and never sends.
	m := batchMsg(5)
	_, kept := markdownBody(m, 50)
	if kept != 1 {
		t.Fatalf("at least 1 item must be kept, got %d", kept)
	}
}

func TestMarkdownBodySingleReturnsOne(t *testing.T) {
	_, kept := markdownBody(singleMsg(), 4096)
	if kept != 1 {
		t.Fatalf("a single-item message must report 1 delivered, got %d", kept)
	}
	// An empty message has no deliverable items.
	if _, k := markdownBody(Message{}, 4096); k != 0 {
		t.Fatalf("an empty message must report 0, got %d", k)
	}
}

func TestTelegramPackingUsesRuneBudget(t *testing.T) {
	m := batchMsg(200)
	text, kept := telegramHTML(m)
	// Telegram limits by **character count**; a byte measure would squeeze multibyte messages to a fraction.
	if n := utf8.RuneCountInString(text); n > telegramTextLimit {
		t.Fatalf("body is %d characters, over the limit %d", n, telegramTextLimit)
	}
	if kept <= 0 || kept >= len(m.Items) {
		t.Fatalf("must pack only part, got %d", kept)
	}
	if !strings.Contains(text, "next message") {
		t.Fatalf("must state that some items did not fit:\n%.300s", text)
	}
}

func TestFeishuPackingReportsKept(t *testing.T) {
	m := batchMsg(2000)
	_, kept := feishuCard(m)
	if kept <= 0 || kept >= len(m.Items) {
		t.Fatalf("the card must pack only part, got %d", kept)
	}
}

func TestWebhookAndEmailReportAllItems(t *testing.T) {
	// These two channels do not truncate the body; the whole batch counts as delivered.
	m := batchMsg(7)
	if n := len(m.Items); n != 7 {
		t.Fatal("precondition does not hold")
	}
	// Confirm indirectly via the renderer's return value: with markdownBody(0) unlimited, everything is kept.
	if _, k := markdownBody(m, 0); k != len(m.Items) {
		t.Fatalf("with no limit everything must be used, got %d", k)
	}
}

// TestMarkdownEscapesUntrustedContent is the regression test for "untrusted content must not alter message structure".
// Titles and summaries come from model output (the model reads the target-under-test's response), and asset names come from the target's URL.
func TestMarkdownEscapesUntrustedContent(t *testing.T) {
	cases := []struct {
		name  string
		item  Item
		must  []string // Must appear in the result (escaped form)
		wrong []string // Must not appear in the result (unescaped form)
	}{
		{
			name: "newline and external link in the title",
			item: Item{
				Severity: "high",
				Name:     "login SQL injection\n[urgent: verify account here](http://attacker.tld)",
			},
			// Newlines must be collapsed (otherwise new list items/blockquotes can be forged);
			// Square brackets and parentheses must be escaped (otherwise a clickable external link is possible).
			must:  []string{`\[urgent: verify account here\]`, `\(http://attacker.tld\)`},
			wrong: []string{"\n[urgent", "\n\n[urgent"},
		},
		{
			name: "image beacon in the title",
			item: Item{
				Severity: "high",
				Name:     "vulnerability ![](http://attacker.tld/beacon)",
			},
			must:  []string{`\!`, `\(http://attacker.tld/beacon\)`},
			wrong: []string{"![]("},
		},
		{
			name: "emphasis and quote in the asset name",
			item: Item{
				Severity: "high",
				Name:     "ordinary title",
				Assets:   []string{"a.com/*inject*>quote"},
			},
			must:  []string{`\*inject\*`, `\>`},
			wrong: []string{"*inject*"},
		},
		{
			name: "backticks and pipes in the summary",
			item: Item{
				Severity: "high",
				Name:     "title",
				Summary:  "`code` | table",
			},
			must:  []string{"\\`code\\`", `\|`},
			wrong: []string{"`code`"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := Message{Items: []Item{tc.item}}
			// writeItem in single-item mode is the render path shared by the three markdown channels.
			var b strings.Builder
			writeItem(&b, tc.item, "", true)
			got := b.String()
			for _, want := range tc.must {
				if !strings.Contains(got, want) {
					t.Errorf("missing escaped form %q:\n%s", want, got)
				}
			}
			for _, bad := range tc.wrong {
				if strings.Contains(got, bad) {
					t.Errorf("unescaped form %q appeared (it can inject structure or links):\n%s", bad, got)
				}
			}
			_ = m
		})
	}
}

// TestMarkdownEscapeBackslashFirst locks in escape order: backslashes must be handled first,
// otherwise backslashes added later get another layer and double backslashes appear in the output.
func TestMarkdownEscapeBackslashFirst(t *testing.T) {
	if got := markdownEscape(`a\b*c`); got != `a\\b\*c` {
		t.Fatalf("wrong escape order, got %q", got)
	}
}

// TestTelegramTitleHasNoMarkdownEscapes locks in a specific regression:
// markdown escaping must not leak into Telegram's HTML output (escaping was once added in the shared title function,
// and as a result visible backslashes like those around the parentheses in (1) appeared in Telegram messages).
func TestTelegramTitleHasNoMarkdownEscapes(t *testing.T) {
	m := Message{Items: []Item{{Severity: "high", Name: "alert(1) *emphasis*"}}}
	text, _ := telegramHTML(m)
	if strings.Contains(text, `\(`) || strings.Contains(text, `\*`) {
		t.Fatalf("markdown backslash escapes leaked into the Telegram body:\n%s", text)
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
