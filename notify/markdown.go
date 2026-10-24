package notify

import (
	"fmt"
	"strings"
)

// This file is the shared message rendering for the "Markdown family" channels
// (DingTalk, WeCom). Feishu uses card JSON, Telegram uses HTML, and email uses
// HTML; each renders in its own adapter.

// maxAssetsShown is the maximum number of assets listed in a message. A
// vulnerability may anchor dozens of assets, and listing them all would crowd out
// the message with no informational value — nobody reads domains past the fourth
// one in an IM.
const maxAssetsShown = 3

// maxSummaryRunes is how many characters the summary is compressed to. An IM
// message is a "hint to go look at the details", not the report itself; the full
// content is in the platform.
const maxSummaryRunes = 120

// markdownReservedBytes is reserved for the message header (digest line +
// severity distribution + possible truncation notice) and footer (platform
// link). When packing whole items this amount is deducted from the budget so the
// header and footer are never truncated — once they are cut, the reader cannot
// even tell "which batch this is and how many items are not shown".
const markdownReservedBytes = 320

// markdownEscape escapes markdown metacharacters.
//
// Why it is mandatory: vulnerability titles, summaries, types, and asset display
// names all come from **untrusted sources** — titles and summaries come from
// model output (the model reads the responses of the target under test), while
// asset urls are full URLs obtained by scanning (including target-controllable
// query strings). Without escaping, a vulnerability whose title is
//
//	Login SQL injection\n[Urgent: verify account here](http://attacker.tld)
//
// would render as a **clickable external link** in a security engineer's
// DingTalk/Feishu; and `![](http://attacker.tld/beacon)` would be fetched by the
// client at render time, effectively announcing "this vulnerability has been
// viewed" and leaking the reader's IP. Even for harmless content, injected bold
// or blockquotes can push the serious vulnerabilities below out of the collapsed
// view.
//
// The escape set covers the characters for titles/links/emphasis/lists/quotes/
// strikethrough that change structure or produce clickable elements. `\` must be
// handled first, otherwise it would re-escape the backslashes added later.
func markdownEscape(s string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		"`", "\\`",
		"*", `\*`,
		"_", `\_`,
		"[", `\[`,
		"]", `\]`,
		"(", `\(`,
		")", `\)`,
		"!", `\!`,
		"#", `\#`,
		">", `\>`,
		"|", `\|`,
		"~", `\~`,
	)
	return replacer.Replace(s)
}

// markdownText compresses untrusted text into a single line and escapes it, for
// markdown body use.
// Single-lining is the other half besides escaping: a newline by itself can forge
// a new list item or blockquote, and escaping characters cannot stop it.
func markdownText(s string, maxRunes int) string {
	return markdownEscape(OneLine(s, maxRunes))
}

// markdownTitle returns the message title (the title bar/card title of the IM
// platform); the content is the **unescaped original text**.
//
// Escaping is deliberately not done here: this title is shared by renderers in
// four contexts — the markdown body, Telegram's HTML, Feishu card plain_text, and
// the generic webhook's JSON and the email subject. Each context has different
// escaping rules (markdown escapes stuffed into HTML leave visible backslashes,
// and stuffed into JSON they pollute the data), so escaping must be done by each
// output end; see writeItem / feishuItemLines / telegramEscape. Markdown escaping
// was once added in the shared function, and the result was visible backslashes
// like `\(1\)` in Telegram messages.
func markdownTitle(m Message) string {
	if m.Batch {
		return fmt.Sprintf("Vulnerability summary · %d total", len(m.Items))
	}
	if len(m.Items) == 0 {
		return "Vulnerability alert"
	}
	it := m.Items[0]
	return fmt.Sprintf("[%s] %s", SeverityLabel(it.Severity), OneLine(it.Title(), 0))
}

// markdownBody renders the message body, returning the body and the **number of
// entries actually written**.
//
// The returned kept value is the number of entries this delivery actually
// delivered, and the caller uses it to mark only the first kept entries as
// delivered — entries kept out by the channel's length limit must be left for the
// next batch rather than marked successful along with the rest. This is exactly
// the source of "silent loss": the message was truncated, but the delivery record
// shows everything delivered, and nowhere can it be seen that the latter half was
// never sent.
//
// maxBytes<=0 means no limit.
func markdownBody(m Message, maxBytes int) (string, int) {
	if !m.Batch {
		if len(m.Items) == 0 {
			return "", 0
		}
		var b strings.Builder
		writeItem(&b, m.Items[0], "", true)
		// A single message is sent even if overlong (with final truncation as the
		// fallback): partial information about one vulnerability is better than
		// sending nothing.
		return TruncateBytes(b.String(), maxBytes), 1
	}

	footer := ""
	if m.HomeURL != "" {
		footer = fmt.Sprintf("\n[View all in the platform](%s)\n", m.HomeURL)
	}
	kept := packItemCount(m.Items, maxBytes, markdownReservedBytes, footer, byteSize, func(it Item, idx int) string {
		var b strings.Builder
		writeItem(&b, it, fmt.Sprintf("%d. ", idx+1), false)
		return b.String()
	})

	items := m.Items[:kept]
	var b strings.Builder
	b.WriteString(markdownBatchIntro(m, items, len(m.Items)))
	for i, it := range items {
		writeItem(&b, it, fmt.Sprintf("%d. ", i+1), false)
	}
	b.WriteString(footer)
	return TruncateBytes(b.String(), maxBytes), kept
}

// markdownBatchIntro renders the opening of a digest message: time window, item
// count, and severity distribution.
// With these, someone receiving a digest can judge whether this batch needs
// immediate attention without clicking into the platform.
//
// items are the entries **actually fitted**, and total is the total number this
// batch should have. When the two differ it must explicitly say "how many are in
// the next message" — otherwise the reader thinks the number in the message
// header is everything, while the entries that were never sent do not exist at
// all in the interface.
func markdownBatchIntro(m Message, items []Item, total int) string {
	var b strings.Builder
	if m.WindowMinutes > 0 {
		fmt.Fprintf(&b, "**New vulnerabilities in the last %d minutes: %d**", m.WindowMinutes, total)
	} else {
		fmt.Fprintf(&b, "**New vulnerabilities: %d**", total)
	}
	if extra := total - len(items); extra > 0 {
		fmt.Fprintf(&b, "(this message shows only the first %d items; the remaining %d will be sent in the next message)", len(items), extra)
	}
	// Give the distribution by severity so the reader can see at a glance whether
	// there are serious items. Only entries **actually included in this message**
	// are counted, ensuring "3 critical" matches the entries that can be counted
	// below.
	counts := map[string]int{}
	for _, it := range items {
		counts[it.Severity]++
	}
	var parts []string
	for _, sev := range []string{"critical", "high", "medium", "low"} {
		if n := counts[sev]; n > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", SeverityLabel(sev), n))
		}
	}
	if len(parts) > 0 {
		b.WriteString("\n" + strings.Join(parts, " · "))
	}
	b.WriteString("\n\n")
	return b.String()
}

// writeItem renders a single vulnerability entry.
//
// prefix is used for the index number in a digest list; when single=true the full
// version is rendered (including summary and backlink), while in a digest list
// only a one-line summary is rendered — otherwise 50 digest entries would become
// a long document.
//
// All externally sourced content (title/type/assets/summary) goes through
// markdownText: single-lining + escaping. The backlink is assembled from the
// administrator-configured public_base_url, is not untrusted content, and must be
// a clickable link, so it is output as-is.
func writeItem(b *strings.Builder, it Item, prefix string, single bool) {
	line := fmt.Sprintf("%s**%s · %s**", prefix, SeverityLabel(it.Severity), markdownText(it.Title(), 0))
	if !single {
		// Digest mode: presented on one line, with assets and summary compressed
		// and appended.
		var extras []string
		if a := assetLine(it.Assets, maxAssetsShown); a != "" {
			extras = append(extras, markdownText(a, 0))
		}
		if it.Summary != "" {
			extras = append(extras, markdownText(it.Summary, 60))
		}
		if len(extras) > 0 {
			line += " — " + strings.Join(extras, " · ")
		}
		b.WriteString(line + "\n")
		return
	}
	b.WriteString(line + "\n")
	if it.IsStatusChange() {
		fmt.Fprintf(b, "**Status change**: %s → %s\n",
			markdownText(StatusLabel(it.FromStatus), 0), markdownText(StatusLabel(it.ToStatus), 0))
	}
	if it.VulnClass != "" && it.VulnClass != it.Title() {
		fmt.Fprintf(b, "**Type**: %s\n", markdownText(it.VulnClass, 0))
	}
	if a := assetLine(it.Assets, maxAssetsShown); a != "" {
		fmt.Fprintf(b, "**Asset**: %s\n", markdownText(a, 0))
	}
	if it.Summary != "" {
		if s := markdownText(it.Summary, maxSummaryRunes); s != "" {
			fmt.Fprintf(b, "**Summary**: %s\n", s)
		}
	}
	if it.DetailURL != "" {
		fmt.Fprintf(b, "[View details](%s)\n", it.DetailURL)
	}
}
