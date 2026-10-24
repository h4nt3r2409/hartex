package server

import (
	"testing"
	"unicode"

	"github.com/Autumn-27/artex/config"
)

// TestTrMsgSwitchesLanguage pins the backend output-language switching. The
// source of truth is now English, so under en (or when no runtime language is
// wired for an English deployment) the source passes through verbatim, while
// ko/zh/es/ja come from the catalog. The unwired test fallback remains ko, which
// is asserted explicitly, and unknown codes resolve through that fallback.
func TestTrMsgSwitchesLanguage(t *testing.T) {
	t.Cleanup(func() { config.CurrentLanguage = nil })

	hasHangul := func(s string) bool {
		for _, r := range s {
			if unicode.Is(unicode.Hangul, r) {
				return true
			}
		}
		return false
	}

	// A representative catalog key (an existing server error constant value).
	const src = errAssetIDRequired
	if hasHangul(src) {
		t.Fatalf("precondition: source message should be English, got %q", src)
	}

	// Unwired → the documented ko fallback (localization golden tests rely on it).
	config.CurrentLanguage = nil
	if got := trMsg(src); got == src || !hasHangul(got) {
		t.Fatalf("unwired: trMsg must fall back to Korean, got %q", got)
	}

	// ko → translated (the source is English now).
	config.CurrentLanguage = func() string { return "ko" }
	if got := trMsg(src); got == src || !hasHangul(got) {
		t.Fatalf("ko: trMsg must return the Korean translation, got %q", got)
	}

	// en → passthrough of the English source.
	config.CurrentLanguage = func() string { return "en" }
	if got := trMsg(src); got != src {
		t.Fatalf("en: trMsg must pass through the English source, got %q", got)
	}

	// zh/es/ja → a translated, non-empty string with no Hangul.
	for _, lang := range []string{"zh", "es", "ja"} {
		config.CurrentLanguage = func() string { return lang }
		got := trMsg(src)
		if got == "" || got == src {
			t.Fatalf("%s: trMsg must translate away from the English source, got %q", lang, got)
		}
		if hasHangul(got) {
			t.Fatalf("%s: translation must not contain Hangul, got %q", lang, got)
		}
	}

	// Unknown language code → ko fallback (config.NormalizeLanguage rejects it).
	config.CurrentLanguage = func() string { return "fr" }
	if got := trMsg(src); got == src || !hasHangul(got) {
		t.Fatalf("unknown lang: trMsg must fall back to Korean, got %q", got)
	}

	// A string not in the catalog (propagated lower-layer error) passes through.
	config.CurrentLanguage = func() string { return "en" }
	const notInCatalog = "some-lower-layer-error: connection refused"
	if got := trMsg(notInCatalog); got != notInCatalog {
		t.Fatalf("non-catalog string must pass through unchanged, got %q", got)
	}
}
