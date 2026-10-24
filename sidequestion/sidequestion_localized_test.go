package sidequestion

import (
	"errors"
	"fmt"
	"testing"
	"unicode"
)

// assertKorean fails if msg is empty, carries a CJK Han ideograph (= leftover
// untranslated Chinese), or has no Hangul at all. ASCII field names and the
// "1–4000"-style ranges are fine; only Han marks an unlocalized string.
func assertKorean(t *testing.T, label, msg string) {
	t.Helper()
	if msg == "" {
		t.Fatalf("%s: 빈 메시지", label)
	}
	hangul := false
	latin := false
	for _, r := range msg {
		if unicode.Is(unicode.Han, r) {
			t.Fatalf("%s: Han characters remain: %q", label, msg)
		}
		if unicode.Is(unicode.Hangul, r) {
			hangul = true
		}
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			latin = true
		}
	}
	if !hangul && !latin {
		t.Fatalf("%s: message is neither Korean nor English: %q", label, msg)
	}
}

// TestSideQuestionOutputsLocalized guards F21: every user-facing side-question
// (side-question) error and answer-text literal produced by this package must be Korean.
// These surface through /api/.../side-questions into the chat side-question panel, so a
// revert to Chinese here is a user-visible regression and must fail the build.
func TestSideQuestionOutputsLocalized(t *testing.T) {
	assertKorean(t, "ErrContextBudget", ErrContextBudget.Error())
	assertKorean(t, "errSideModelInterrupted", errSideModelInterrupted.Error())
	assertKorean(t, "errSideNoAnswer", errSideNoAnswer.Error())
	assertKorean(t, "msgSideToolUnavailable", msgSideToolUnavailable)
	assertKorean(t, "errSideSummaryCallCap", errSideSummaryCallCap.Error())
	assertKorean(t, "sideSummaryFailedPrefix", sideSummaryFailedPrefix)
	assertKorean(t, "errSideSummaryIncomplete", errSideSummaryIncomplete.Error())
	assertKorean(t, "errSideSummaryOverBudget", errSideSummaryOverBudget.Error())
	assertKorean(t, "errSideHistoryCursor", errSideHistoryCursor.Error())
	assertKorean(t, "errSideCompactionStalled", errSideCompactionStalled.Error())
}

// TestSideQuestionBudgetSentinelPreserved: localizing the message must not break
// callers that branch on the ErrContextBudget sentinel with errors.Is (see
// context_test.go, which relies on it after the recovery path gives up).
func TestSideQuestionBudgetSentinelPreserved(t *testing.T) {
	if !errors.Is(fmt.Errorf("prepare: %w", ErrContextBudget), ErrContextBudget) {
		t.Fatal("ErrContextBudget 센티넬 식별(errors.Is)이 깨졌습니다")
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
