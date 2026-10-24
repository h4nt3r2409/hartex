package server

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Autumn-27/artex/db"
)

// A regression guard test that keeps the "user-facing" messages in Korean among
// engine.go's work-control and intent-intervention errors. Korean detection reuses
// assertKoreanError (contains Hangul, zero Chinese Han characters) introduced by F3a.
//
// Call-graph assessment (see the constant block comment in engine.go):
//   - ControlWork's four errors (no running work, already controlling, wrap-up wait
//     cancelled, wrap-up wait timeout) → applyIntentControl(task_control.go) →
//     controlIntent(server.go:1236) → writeErr 409. User-only, and never reaches the
//     agent tool (actool) path.
//   - runDetachedIntent's two errors (Worker not ready, resume CAS conflict) →
//     sendWorkerMessage(intent_intervention.go:147) → writeErr. User-only.
//   - Conversely, the Chinese text in SteerWork/KillWork (the steer_work/kill_work
//     tools' actool.Errorf) and transitionIntentState (internal state-transition log)
//     is brain input/log data and is preserved, so it is out of scope for this test.

// TestControlWorkNoRunningWorkErrorLocalized drives ControlWork directly without a
// DB or engine to verify that it actually returns a Korean error when there is no
// running work. The run==nil branch only looks at the e.work map, so NewEngine(nil)
// reaches the end. Because the errWorkControlConflict sentinel is wrapped with %w,
// it also checks that the errors.Is relationship is preserved. It fails if someone
// reverts this literal to Chinese.
func TestControlWorkNoRunningWorkErrorLocalized(t *testing.T) {
	e := NewEngine(nil)
	err := e.ControlWork(context.Background(), 42, "pause")
	if err == nil {
		t.Fatal("실행 중 work 가 없는데 ControlWork 가 nil 을 반환했습니다")
	}
	if !errors.Is(err, errWorkControlConflict) {
		t.Fatalf("errors.Is(err, errWorkControlConflict) = false, err=%v", err)
	}
	assertKoreanError(t, "control_work_no_running", err.Error())
}

// TestEngineUserFacingErrorsLocalized pins the remaining user-facing format strings
// that are hard to drive end to end because they sit behind engine/DB gates or
// goroutine timing (the same approach as the gated paths in finding_retests and
// finding_traffic). Each format string is filled with representative arguments
// before being checked.
func TestEngineUserFacingErrorsLocalized(t *testing.T) {
	for _, c := range []struct {
		label, msg string
	}{
		{"work_control_busy", fmt.Errorf(trMsg(errWorkControlBusyFmt), errWorkControlConflict, int64(42), "pause").Error()},
		{"work_control_wait", fmt.Errorf(trMsg(errWorkControlWaitFmt), int64(42), "pause", context.DeadlineExceeded).Error()},
		{"detached_worker_not_ready", errDetachedWorkerNotReady},
		{"detached_state_conflict", fmt.Errorf(trMsg(errIntentCtrlStateConflictFmt), db.ErrIntentStateConflict).Error()},
	} {
		assertKoreanError(t, c.label, c.msg)
	}
}
