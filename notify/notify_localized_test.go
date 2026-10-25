package notify

import (
	"strings"
	"testing"
)

// hasHan reports whether the string contains at least one CJK unified Han character.
// English-only output must never contain one.
func hasHan(s string) bool {
	for _, r := range s {
		if r >= 0x4E00 && r <= 0x9FFF {
			return true
		}
	}
	return false
}

// hasHangul reports whether the string contains at least one Hangul syllable.
// English-only output must never contain one.
func hasHangul(s string) bool {
	for _, r := range s {
		if r >= 0xAC00 && r <= 0xD7A3 {
			return true
		}
	}
	return false
}

// assertEnglish fails unless the label is English-only: no Han or Hangul characters,
// and at least one Latin letter.
func assertEnglish(t *testing.T, where, got string) {
	t.Helper()
	if hasHan(got) {
		t.Errorf("%s: Han characters remain: %q", where, got)
	}
	if hasHangul(got) {
		t.Errorf("%s: Hangul characters remain: %q", where, got)
	}
	latin := false
	for _, r := range got {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			latin = true
			break
		}
	}
	if !latin {
		t.Errorf("%s: message contains no English text: %q", where, got)
	}
}

// TestItemTitlePlaceholderLocalized checks that the fallback title for an item with
// both name and type empty is the English placeholder "(unnamed vulnerability)". It
// never emits an empty title.
func TestItemTitlePlaceholderLocalized(t *testing.T) {
	got := Item{}.Title()
	assertEnglish(t, "Item{}.Title()", got)
	if got != "(unnamed vulnerability)" {
		t.Errorf("fallback title = %q, want %q", got, "(unnamed vulnerability)")
	}
	// If a name exists use it; if only a type exists prefer the type (confirming the fallback logic has not regressed).
	if n := (Item{Name: "login SQLi"}).Title(); n != "login SQLi" {
		t.Errorf("name-first logic broke, got %q", n)
	}
	if v := (Item{VulnClass: "XSS"}).Title(); v != "XSS" {
		t.Errorf("type fallback logic broke, got %q", v)
	}
}

// TestAssetLineLocalized checks that the asset-list wording's separator and total-count
// notation are ASCII English. If non-ASCII punctuation or CJK count words come back,
// the notification body becomes mixed-language.
func TestAssetLineLocalized(t *testing.T) {
	// At or below the limit: list everything, ASCII comma separator.
	if got := assetLine([]string{"a.example.com", "b.example.com"}, 3); got != "a.example.com, b.example.com" {
		t.Errorf("separator must be an ASCII comma, got %q", got)
	}
	// Above the limit: first limit items + English "and N total" (N is the total count).
	got := assetLine([]string{"a", "b", "c", "d", "e"}, 2)
	assertEnglish(t, "assetLine(over limit)", got)
	if !strings.Contains(got, "and 5 total") {
		t.Errorf("the total count 5 must be written as 'and 5 total', got %q", got)
	}
}
