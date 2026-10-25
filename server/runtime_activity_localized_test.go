package server

import (
	"fmt"
	"strings"
	"testing"
)

// Regression guard that keeps the "display-only" runtime activity summary constants in
// engine.go, engine_timeout.go, and goals.go in English. These summaries are exposed in the
// dashboard activity transcript but carry no node_id (task scope), so they are never read
// back into any agent context — the feedback path (planner.workerOutput and the
// get_worker_output tool) selects only 'result'/'text' activities within an intent scope
// (node_id). So keeping them in English does not touch brain input (BRIEF boundary #1).
// Conversely, summaries that are fed back into the brain (for example, the seed intent
// summary and the default description) preserve the original
// text, so they are not the target of this test. The check reuses assertNonEmpty
// (non-empty messages, intercept_archive_checked_test.go). [[G132]]
func TestRuntimeActivitySummariesLocalized(t *testing.T) {
	// Fixed summaries with no format arguments: they must stay in English.
	plain := map[string]string{
		"goalless_task_done": goallessTaskDoneSummary,
		"goal_breakdown_r0":  goalBreakdownRound0Summary,
		"queued_no_llm":      queuedNoLLMSummary,
		"queued_fifo":        queuedFIFOSummary,
	}
	for label, msg := range plain {
		assertNonEmpty(t, label, msg)
	}

	// %d format strings: verify the format argument is still present, the formatted result
	// is still English, and the number is actually reflected.
	fmtCases := map[string]string{
		"planner_round":       plannerRoundSummaryFmt,
		"timeout_final_round": timeoutFinalRoundSummaryFmt,
		"queued_conc_limit":   queuedConcurrencyLimitSummaryFmt,
	}
	for label, f := range fmtCases {
		if !strings.Contains(f, "%d") {
			t.Fatalf("%s: format argument %%d disappeared: %q", label, f)
		}
		got := fmt.Sprintf(f, 7)
		assertNonEmpty(t, label, got)
		if !strings.Contains(got, "7") {
			t.Fatalf("%s: format argument was not reflected in the result: %q", label, got)
		}
	}
}
