package db

import (
	"testing"
	"unicode"
)

// TestManualTaskScopeSummaryLocalized pins the manual task-scope provenance label.
// It is stored on task_asset_links.source_summary (AddTaskScope / SetTaskAssetSource)
// and rendered verbatim on the task detail sessions/assets tabs, so reverting it to
// non-English text would surface mixed-language provenance labels in the same panel. The
// existing task_assets_test.go already asserts the stored value equals this constant
// via the symbol, so pinning the constant here protects the user-facing text too.
func TestManualTaskScopeSummaryLocalized(t *testing.T) {
	const want = "added directly by the user from the test assets page"
	if manualTaskScopeSummary == "" {
		t.Fatal("manualTaskScopeSummary: empty string")
	}
	for _, r := range manualTaskScopeSummary {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hangul, r) {
			t.Fatalf("manualTaskScopeSummary: non-English characters remain: %q", manualTaskScopeSummary)
		}
	}
	if manualTaskScopeSummary != want {
		t.Fatalf("manualTaskScopeSummary: got %q, want %q", manualTaskScopeSummary, want)
	}
}
