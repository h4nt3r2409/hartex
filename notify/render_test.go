package notify

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncateBytesKeepsValidUTF8(t *testing.T) {
	// This is the most important invariant in this package. WeCom limits by **bytes**, while
	// multibyte scripts (Cyrillic here, CJK elsewhere) use two or more bytes per character, so any
	// implementation that hard-cuts by bytes splits a character in half, producing invalid UTF-8
	// that the platform rejects. Use mixed multibyte/Latin inputs with coprime lengths to hit
	// every possible cut point.
	inputs := []string{
		"тестовое содержание",
		"mixed тест content",
		"aтbеcсdтe",
		"🔴🟠🟡🔵", // A 4-byte emoji makes a wrong cut more obvious
		strings.Repeat("π", 100),
	}
	for _, in := range inputs {
		for max := 1; max <= len(in)+2; max++ {
			got := TruncateBytes(in, max)
			if !utf8.ValidString(got) {
				t.Fatalf("input %q max=%d: produced invalid UTF-8 %q", in, max, got)
			}
			if len(got) > max {
				t.Fatalf("input %q max=%d: result is %d bytes, over the limit", in, max, len(got))
			}
			// Content must not be altered when not truncated.
			if len(in) <= max && got != in {
				t.Fatalf("input %q max=%d: content changed although it was under the limit -> %q", in, max, got)
			}
		}
	}
}

func TestTruncateBytesZeroMeansUnlimited(t *testing.T) {
	long := strings.Repeat("x", 10000)
	if got := TruncateBytes(long, 0); got != long {
		t.Fatal("max=0 must mean unlimited")
	}
	if got := TruncateBytes(long, -5); got != long {
		t.Fatal("max<0 must mean unlimited")
	}
}

func TestTruncateBytesEllipsisBudget(t *testing.T) {
	// When max is smaller than the ellipsis itself, appending the ellipsis must not push it over the limit.
	got := TruncateBytes("abcdefgh", 1)
	if len(got) > 1 {
		t.Fatalf("max=1: result %q is %d bytes, over the limit", got, len(got))
	}
	// Normally the ellipsis should be included.
	if got := TruncateBytes("abcdefgh", 5); !strings.HasSuffix(got, ellipsis) {
		t.Fatalf("want a trailing ellipsis, got %q", got)
	}
}

func TestTruncateRunesCountsCharactersNotBytes(t *testing.T) {
	// The difference in measure from TruncateBytes must be preserved: Telegram limits by characters,
	// and a byte measure would cut multibyte messages down to a fraction.
	s := "абвгдеёжзи"
	got := TruncateRunes(s, 5)
	if n := utf8.RuneCountInString(got); n != 5 {
		t.Fatalf("want 5 characters, got %d (%q)", n, got)
	}
	// The same string should be markedly shorter under a byte measure.
	if utf8.RuneCountInString(TruncateBytes(s, 5)) >= 5 {
		t.Fatal("the byte measure must not yield the same character count as the rune measure")
	}
}

func TestOneLineCollapsesWhitespace(t *testing.T) {
	got := OneLine("first line\n\nsecond line\ttabbed   spaces", 0)
	if strings.ContainsAny(got, "\n\t") {
		t.Fatalf("all whitespace must be collapsed, got %q", got)
	}
	if strings.Contains(got, "  ") {
		t.Fatalf("consecutive spaces must not survive, got %q", got)
	}
	// After truncation it must still be readable and valid.
	got = OneLine("абвгдеёжзи", 4)
	if n := utf8.RuneCountInString(got); n != 4 {
		t.Fatalf("want 4 characters, got %d (%q)", n, got)
	}
}

func TestTruncateHTMLNeverCutsTagInHalf(t *testing.T) {
	// Cutting HTML directly produces fragments like `<a href="htt`, and the platform rejects the entire message.
	s := `<b>Title</b>тело тело тело<a href="https://example.com/very/long/path">View details</a>`
	for max := 1; max <= utf8.RuneCountInString(s)+2; max++ {
		got := TruncateHTML(s, max)
		if n := utf8.RuneCountInString(got); max > 0 && n > max {
			t.Fatalf("max=%d: result is %d characters, over the limit", max, n)
		}
		// The tail must not contain an unclosed `<` (i.e. a `<` in the last segment with no `>`).
		if lt := strings.LastIndex(got, "<"); lt >= 0 && !strings.Contains(got[lt:], ">") {
			t.Fatalf("max=%d: trailing tag was cut -> %q", max, got)
		}
	}
}

func TestAssetLineOmitsExcess(t *testing.T) {
	if got := assetLine(nil, 3); got != "" {
		t.Fatalf("no assets must render an empty string, got %q", got)
	}
	if got := assetLine([]string{"a", "b"}, 3); got != "a, b" {
		t.Fatalf("under the limit everything must be listed, got %q", got)
	}
	// When over the limit, the total must be labeled; otherwise readers do not know how many assets remain unlisted.
	got := assetLine([]string{"a", "b", "c", "d", "e"}, 2)
	if !strings.Contains(got, "and 5 total") {
		t.Fatalf("the total 5 must be labeled, got %q", got)
	}
}

func TestSeverityAndStatusLabels(t *testing.T) {
	if AtLeast("", "low") {
		t.Fatal("an empty severity ordinal is 0 and must be blocked by any threshold")
	}
	if !AtLeast("critical", "") {
		t.Fatal("an empty threshold must let everything through")
	}
	if got := StatusLabel("fixed"); got != "Fixed" {
		t.Fatalf("unknown status mapping, got %q", got)
	}
	// An unknown status is echoed as-is; no label is invented.
	if got := StatusLabel("weird_status"); got != "weird_status" {
		t.Fatalf("an unknown status must be echoed as-is, got %q", got)
	}
}

// TestTruncateHTMLNeverCutsEntity covers an omission identified by the audit: truncation must avoid not only
// half tags but also cut HTML entities.
//
// once `&amp;` is cut to `&amp`, an entity-aware parser may reject the **entire** message -
// and overlong digest messages are commonplace, so the cost is too high.
func TestTruncateHTMLNeverCutsEntity(t *testing.T) {
	s := "aaaa&amp;bbbb&lt;cccc&quot;dddd"
	for max := 1; max <= utf8.RuneCountInString(s)+2; max++ {
		got := TruncateHTML(s, max)
		// The tail must not contain an entity fragment with "& but no matching ;".
		if amp := strings.LastIndex(got, "&"); amp >= 0 && !strings.Contains(got[amp:], ";") {
			t.Fatalf("max=%d: an entity fragment remains at the tail %q", max, got[amp:])
		}
		if strings.Contains(got, "&amp\x00") {
			t.Fatalf("max=%d: a malformed entity appeared", max)
		}
	}
}
