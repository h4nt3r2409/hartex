package server

import "testing"

// A regression guard test that keeps the user-facing HTTP error response messages
// in finding_retests.go in English. The check reuses assertNonEmpty
// (non-empty messages). The
// startFindingRetest handler that returns these four messages passes through the
// s.pg(w) (DB) gate at the very beginning, so it cannot run to completion on this
// host without a database; therefore the constants themselves are pinned (the same
// approach as the DB-gated paths in finding_traffic, goals_api, and notify_api). It
// fails if a message becomes empty.
//
// Tool descriptions (get_finding_retest_context, record_finding_retest_result),
// parameter descriptions, actool.Errorf (the message for an unlinked retest round),
// and the DB seed agent name, profile, and note in seedFindingRetester are brain
// input or seed values that the agent reads, so they intentionally preserve the
// original Chinese text and are not assertions in this test (see the constant block
// comment in finding_retests.go).
func TestFindingRetestErrorsLocalized(t *testing.T) {
	for _, c := range []struct {
		label, msg string
	}{
		{"notes_too_long", errFindingRetestNotesTooLong},
		{"agent_missing", errFindingRetestAgentMissing},
		{"tool_required", errFindingRetestToolRequired},
		{"service_stopping", errFindingRetestServiceStopping},
	} {
		assertNonEmpty(t, c.label, c.msg)
	}
}
