package notify

import (
	"strings"
	"unicode/utf8"
)

const ellipsis = "…"

// TruncateBytes truncates s to at most max bytes, guaranteeing that the result is
// valid UTF-8 and does not split a character.
//
// Why cutting on character boundaries is mandatory: WeCom group robot markdown
// has a hard limit of 4096 **bytes** (not characters), while a single Chinese
// character is 3 bytes. Slicing directly by bytes would cut a Han character in
// half and produce invalid UTF-8 — the platform either rejects the whole message
// or displays it as garbled boxes. The approach here is to back up from the
// budget position to the nearest rune start byte (utf8.RuneStart determines
// continuation bytes 0b10xxxxxx).
//
// max<=0 means no limit. An ellipsis is appended after truncation, unless max is
// too small to fit the ellipsis.
func TruncateBytes(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	budget := max - len(ellipsis)
	suffix := ellipsis
	if budget < 0 {
		// max is shorter than the ellipsis: give up the ellipsis and truncate
		// purely, so the result does not exceed max.
		budget = max
		suffix = ""
	}
	cut := budget
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + suffix
}

// OneLine flattens multi-line text into a single line: collapses all whitespace,
// then truncates by character count.
// Used for IM message title lines — summaries often contain newlines, and putting
// them directly into a table/title breaks the layout.
// max<=0 means no length limit.
func OneLine(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	return TruncateRunes(s, max)
}

// TruncateRunes truncates s to at most max characters (rather than bytes),
// appending an ellipsis when it overflows.
// max<=0 means no limit.
//
// The difference from TruncateBytes is the platform's metric: WeCom limits by
// bytes, Telegram limits by character count. Using the wrong metric does not
// error, it just makes the message much shorter than expected (1 Chinese
// character = 3 bytes, so a 4096-byte cut leaves about 1365 characters), so both
// functions must be kept and chosen per channel.
func TruncateRunes(s string, max int) string {
	if max <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 1 {
		return string(runes[:max])
	}
	return string(runes[:max-1]) + ellipsis
}

// TruncateHTML truncates an HTML fragment by character count while guaranteeing
// that no half tag is produced.
//
// Directly truncating HTML by characters can cut out a broken tag like
// `<a href="htt`; the platform parser either errors and rejects the whole
// message, or swallows the following body as an attribute value. The approach
// here is: truncate by characters first, then check whether the tail has an
// unclosed `<`, and back up to before it if so.
//
// No tag balancing (adding a closing </b> etc.): Telegram's HTML parser closes
// unclosed tags automatically, and implementing balancing ourselves would have
// to handle quotes in attributes, comments, and self-closing tags, a complexity
// disproportionate to the benefit.
func TruncateHTML(s string, max int) string {
	if max <= 0 || len([]rune(s)) <= max {
		return s
	}
	cut := TruncateRunes(s, max)
	// If the tail is a fragment starting with `<` (no `>` after the last `<`),
	// back up to before the `<`.
	if lt := strings.LastIndex(cut, "<"); lt >= 0 && !strings.Contains(cut[lt:], ">") {
		cut = cut[:lt]
	}
	// If the tail is a cut-off HTML entity (e.g. `&amp;` cut to `&amp`), back up
	// as well. An entity fragment in a parser that only understands entities may
	// cause the **whole message** to be rejected — a digest message exceeding the
	// length limit is common to begin with, and it is not worth losing the entire
	// notification over this.
	if amp := strings.LastIndex(cut, "&"); amp >= 0 && !strings.Contains(cut[amp:], ";") {
		cut = cut[:amp]
	}
	return cut
}

// packItemCount computes how many items fit **completely** within the budget, so
// a digest message can pack whole items.
//
// Why pack whole items instead of rendering the full document and truncating:
// truncation makes the latter half of the entries vanish into thin air, while
// their delivery records are still marked as delivered — it is invisible in the
// message and invisible in the delivery history, and the vulnerabilities are
// simply gone. After packing whole items, entries that do not fit stay in the
// database for the next batch, and the kept value the caller receives is the
// number of entries this message actually delivered.
//
// Parameters: maxSize<=0 means no limit; reserve is the amount reserved for the
// message header/footer; size handles measurement (metrics differ per platform:
// WeCom/DingTalk by bytes, Telegram by character count — using the wrong metric
// does not error, it only compresses a Chinese message far below the limit);
// render turns the idx-th item into its actual text — lengths vary with content
// and cannot be estimated.
//
// Returns at least 1 (as long as there are items). Even a single extremely
// overlong item must be sent, with the caller's final truncation as the
// fallback; otherwise one overlong vulnerability would permanently block the
// whole batch.
func packItemCount(items []Item, maxSize, reserve int, footer string, size func(string) int, render func(Item, int) string) int {
	if maxSize <= 0 {
		return len(items)
	}
	budget := maxSize - reserve - size(footer)
	if budget < 0 {
		budget = 0
	}
	used := 0
	for i, it := range items {
		used += size(render(it, i))
		if used > budget && i > 0 {
			return i
		}
	}
	return len(items)
}

// byteSize / runeSize are the two measurement metrics for packItemCount, named
// to avoid a bare func(s string) int closure at the call site, where it would be
// hard to tell at a glance which metric is used.
func byteSize(s string) int { return len(s) }
func runeSize(s string) int { return utf8.RuneCountInString(s) }

// assetLine renders an asset list as one line of display text; when there are
// more than limit, the rest are omitted and the total count is noted.
// A vulnerability may anchor dozens of assets, and listing them all would crowd
// out the message.
func assetLine(assets []string, limit int) string {
	if len(assets) == 0 {
		return ""
	}
	if limit <= 0 || len(assets) <= limit {
		return strings.Join(assets, ", ")
	}
	return strings.Join(assets[:limit], ", ") + ", and " + itoa(len(assets)) + " total"
}

// itoa is a short alias for strconv.Itoa, used only for assembling display text,
// avoiding importing strconv everywhere.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
