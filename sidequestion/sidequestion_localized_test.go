package sidequestion

import (
	"errors"
	"fmt"
	"testing"
	"unicode"
)

// assertEnglish fails if msg is empty, carries a CJK Han or Hangul character
// (= leftover untranslated text), or has no Latin letters at all. ASCII field names
// and the "1–4000"-style ranges are fine.
func assertEnglish(t *testing.T, label, msg string) {
	t.Helper()
	if msg == "" {
		t.Fatalf("%s: empty message", label)
	}
	latin := false
	for _, r := range msg {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hangul, r) {
			t.Fatalf("%s: non-English characters remain: %q", label, msg)
		}
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			latin = true
		}
	}
	if !latin {
		t.Fatalf("%s: message is not English: %q", label, msg)
	}
}

// TestSideQuestionOutputsLocalized guards F21: every user-facing side-question
// (side-question) error and answer-text literal produced by this package must be English.
// These surface through /api/.../side-questions into the chat side-question panel, so a
// revert to Chinese here is a user-visible regression and must fail the build.
func TestSideQuestionOutputsLocalized(t *testing.T) {
	assertEnglish(t, "ErrContextBudget", ErrContextBudget.Error())
	assertEnglish(t, "errSideModelInterrupted", errSideModelInterrupted.Error())
	assertEnglish(t, "errSideNoAnswer", errSideNoAnswer.Error())
	assertEnglish(t, "msgSideToolUnavailable", msgSideToolUnavailable)
	assertEnglish(t, "errSideSummaryCallCap", errSideSummaryCallCap.Error())
	assertEnglish(t, "sideSummaryFailedPrefix", sideSummaryFailedPrefix)
	assertEnglish(t, "errSideSummaryIncomplete", errSideSummaryIncomplete.Error())
	assertEnglish(t, "errSideSummaryOverBudget", errSideSummaryOverBudget.Error())
	assertEnglish(t, "errSideHistoryCursor", errSideHistoryCursor.Error())
	assertEnglish(t, "errSideCompactionStalled", errSideCompactionStalled.Error())
}

// TestSideQuestionBudgetSentinelPreserved: producing English messages must not break
// callers that branch on the ErrContextBudget sentinel with errors.Is (see
// context_test.go, which relies on it after the recovery path gives up).
func TestSideQuestionBudgetSentinelPreserved(t *testing.T) {
	if !errors.Is(fmt.Errorf("prepare: %w", ErrContextBudget), ErrContextBudget) {
		t.Fatal("the ErrContextBudget sentinel identity (errors.Is) is broken")
	}
}

// TestSideQuestionBrainPromptsEnglish: the agent-brain prompts (the answering
// instruction and the summary instruction) are English. They must contain no Han
// characters and must carry English text, so an accidental revert to Chinese is
// caught here.
func TestSideQuestionBrainPromptsEnglish(t *testing.T) {
	for _, c := range []struct{ label, text string }{
		{"instruction", instruction},
		{"summaryInstruction", summaryInstruction},
	} {
		latin := false
		for _, r := range c.text {
			if unicode.Is(unicode.Han, r) {
				t.Errorf("%s: Han characters remain in the brain prompt: %q", c.label, c.text)
				break
			}
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				latin = true
			}
		}
		if !latin {
			t.Errorf("%s: brain prompt is not English: %q", c.label, c.text)
		}
	}
}
