package server

import "testing"

// A regression guard test that keeps the task-resume (admitPausedTask) precondition
// validation error messages in goals.go in Korean. Korean detection reuses
// assertKoreanError (contains Hangul, zero Chinese Han characters) introduced by
// F3a. The admitTaskWhen(requirePaused=true) path that returns these two messages
// passes behind the s.concMu lock through the s.m.Task(), s.engine.IsDeleting(),
// and beginTaskOperation() engine gates, so it cannot run to completion on this host
// without a DB or engine; therefore the constants themselves are pinned (the same
// approach as the gated paths in finding_retests and finding_traffic). It fails if
// someone reverts these literals to Chinese.
//
// Call-graph assessment (see the constant block comment in goals.go): the two
// messages are exposed only through work-control responses, either single
// (server.go:1194 → writeErr 409) or batch (task_control.go:298 → items[].error),
// and are user-only. The orchestrator path (orchestration.go:343) is fixed to
// action="pause" and never reaches resume validation, so it is not brain input.
func TestGoalResumeErrorsLocalized(t *testing.T) {
	for _, c := range []struct {
		label, msg string
	}{
		{"resume_terminal", errGoalResumeTerminal},
		{"resume_not_paused", errGoalResumeNotPaused},
	} {
		assertKoreanError(t, c.label, c.msg)
	}
}
