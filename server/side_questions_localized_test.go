package server

import "testing"

// TestSideQuestionAPIErrorsChecked guards F21: every user-facing side-question
// (side question) message surfaced by the HTTP wrapper (writeErr responses) and by the
// exchange status (e.Error) must be English — non-empty English text.
// These reach the chat side-question panel alongside the sidequestion package errors, so
// both layers are checked together to avoid a mixed-language panel. Reuses
// assertNonEmpty.
func TestSideQuestionAPIErrorsLocalized(t *testing.T) {
	for _, c := range []struct{ label, msg string }{
		{"ctx_not_saved", sideErrCtxNotSaved},
		{"model_config_changed", sideErrModelConfigChanged},
		{"service_unavailable", sideErrServiceUnavailable},
		{"task_archived", sideErrTaskArchived},
		{"worker_deleted", sideErrWorkerDeleted},
		{"bad_question", sideErrBadQuestion},
		{"task_archiving", sideErrTaskArchiving},
		{"request_id_reused", sideErrRequestIDReused},
		{"no_snapshot", sideErrNoSnapshot},
		{"concurrency_limit", sideErrConcurrencyLimit},
		{"answer_stopped", sideErrAnswerStopped},
		{"answer_timeout", sideErrAnswerTimeout},
	} {
		assertNonEmpty(t, c.label, c.msg)
	}
}
