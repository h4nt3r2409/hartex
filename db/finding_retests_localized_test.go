package db

import (
	"testing"
	"unicode"
)

// assertRetestReasonEnglish pins a revalidation reason constant to its English text
// and asserts that no non-English characters remain (F9). FinishFindingRetest and
// RecoverFindingRetests append the constants directly into SQL literal positions, so
// pinning the constants also protects the user-facing text exposed in the panel.
func assertRetestReasonEnglish(t *testing.T, name, got, want string) {
	t.Helper()
	if got == "" {
		t.Fatalf("%s: empty string", name)
	}
	for _, r := range got {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hangul, r) {
			t.Fatalf("%s: non-English characters remain: %q", name, got)
		}
	}
	if got != want {
		t.Fatalf("%s: got %q, want %q", name, got, want)
	}
}

// TestFindingRetestReasonsLocalized asserts that the two termination-reason constants
// stored in the finding_retests.error column and exposed as item.error in the
// revalidation panel (finding-retest-panel) are in English. The sibling reasons
// (convRetest*) in server/conversations.go share the same column/panel, so all of
// them must stay in the same language to avoid mixed-language text in the panel.
func TestFindingRetestReasonsLocalized(t *testing.T) {
	assertRetestReasonEnglish(t, "retestNoConclusionReason", retestNoConclusionReason,
		"The agent did not save a revalidation conclusion. Review the conversation and revalidate again.")
	assertRetestReasonEnglish(t, "retestServiceRestartReason", retestServiceRestartReason,
		"The service restarted and revalidation was interrupted. Please start it again.")
}
