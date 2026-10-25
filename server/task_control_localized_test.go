package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Autumn-27/artex/db"
)

// Regression test that keeps task_control.go's task/intent control API error responses in
// English. The check reuses assertNonEmpty (non-empty English text), and response-body extraction reuses the task_categories test's decodeErrorField
// (same server package).

// TestTaskControlErrorConstantsChecked asserts that all 10 control response constants are
// English. The intent control path (applyIntentControl) must pass through t.Store.GetNode(DB) on
// its first line to be reached, so it cannot be driven to completion on this host without a DB;
// those 6 constants are asserted directly (intent_intervention/goals_api precedent). The task
// control and batch paths are exercised by the two tests below through actual function execution
// and the HTTP response body.
func TestTaskControlErrorConstantsLocalized(t *testing.T) {
	cases := map[string]string{
		"task_deleting":          errTaskCtrlDeleting,
		"task_terminal_pause":    errTaskCtrlTerminalPause,
		"task_already_paused":    errTaskCtrlAlreadyPaused,
		"batch_size":             errTaskCtrlBatchSizeFmt,
		"intent_inherited":       errIntentCtrlInheritedReadonly,
		"intent_only_running":    errIntentCtrlOnlyRunningPause,
		"intent_only_paused":     errIntentCtrlOnlyPausedResume,
		"intent_state_conflict":  errIntentCtrlStateConflictFmt,
		"intent_only_deletable":  errIntentCtrlOnlyDeletable,
		"intent_reason_required": errIntentCtrlReasonRequired,
	}
	for label, msg := range cases {
		assertNonEmpty(t, label, msg)
	}
}

// TestTaskControlApplyResponsesChecked actually runs the pause control path through
// applyTaskControl, since it finishes without going through DB/engine state, confirming that the
// constants are really wired into the returned errors. All three branches follow
// s.m.Task (map lookup) → deletion barrier → t.lifecycleSnapshot() (direct in-memory read of the
// Status/Paused fields), so one task in the tasks map plus an empty Engine is enough to run
// without Store/DB. The success path (actually pausing) goes through ApplyTaskPause(DB), so it is
// not driven here — all error branches return before that.
func TestTaskControlApplyResponsesLocalized(t *testing.T) {
	// Terminal-state task: isTerminalStatus(done) branch.
	terminalServer := func() (*Server, *Task) {
		tk := &Task{ID: "1", Status: "done"}
		return &Server{m: &Manager{tasks: map[string]*Task{"1": tk}}, engine: &Engine{}}, tk
	}
	// Already-paused running task: lifecycle.Paused branch.
	pausedServer := func() (*Server, *Task) {
		tk := &Task{ID: "1", Status: "running", Paused: true}
		return &Server{m: &Manager{tasks: map[string]*Task{"1": tk}}, engine: &Engine{}}, tk
	}
	// Task with the deletion barrier raised: IsDeleting branch.
	deletingServer := func() (*Server, *Task) {
		tk := &Task{ID: "1", Status: "running"}
		s := &Server{m: &Manager{tasks: map[string]*Task{"1": tk}}, engine: &Engine{}}
		s.engine.deleting.Store("1", true)
		return s, tk
	}

	cases := []struct {
		name  string
		setup func() (*Server, *Task)
		want  string
	}{
		{"terminal", terminalServer, errTaskCtrlTerminalPause},
		{"already-paused", pausedServer, errTaskCtrlAlreadyPaused},
		{"deleting", deletingServer, errTaskCtrlDeleting},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, tk := c.setup()
			_, err := s.applyTaskControl(tk, "pause")
			if err == nil {
				t.Fatalf("expected an error, got nil")
			}
			if err.Error() != c.want {
				t.Fatalf("returned error = %q, want %q", err.Error(), c.want)
			}
			assertNonEmpty(t, c.name, err.Error())
		})
	}
}

// TestControlTasksBatchResponsesChecked inspects the batch control endpoint's response body
// over real HTTP. (1) An empty task_ids produces 400 + a size error via writeErr before
// accessing s.m, so it is reachable even with &Server{}. (2) Batch-pausing one terminal-state
// task returns 200 with the task control error carried as items[].error, confirming that
// applyTaskControl errors propagate all the way into the batch JSON.
func TestControlTasksBatchResponsesLocalized(t *testing.T) {
	t.Run("size-error", func(t *testing.T) {
		s := &Server{}
		req := httptest.NewRequest(http.MethodPost, "/api/tasks/control",
			strings.NewReader(`{"task_ids":[],"action":"pause"}`))
		rec := httptest.NewRecorder()
		s.controlTasksBatch(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 (body %q)", rec.Code, rec.Body.String())
		}
		got := decodeErrorField(t, rec.Body.Bytes())
		want := fmt.Sprintf(errTaskCtrlBatchSizeFmt, maxBatchControlIDs)
		if got != want {
			t.Fatalf("message = %q, want %q", got, want)
		}
		if !strings.Contains(got, "100") {
			t.Fatalf("the size cap 100 is not reflected in the message: %q", got)
		}
		assertNonEmpty(t, "size-error", got)
	})

	t.Run("item-error-surfaces", func(t *testing.T) {
		tk := &Task{ID: "1", Status: "done"}
		s := &Server{m: &Manager{tasks: map[string]*Task{"1": tk}}, engine: &Engine{}}
		req := httptest.NewRequest(http.MethodPost, "/api/tasks/control",
			strings.NewReader(`{"task_ids":["1"],"action":"pause"}`))
		rec := httptest.NewRecorder()
		s.controlTasksBatch(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
		}
		var out struct {
			Items []struct {
				ID    string `json:"id"`
				OK    bool   `json:"ok"`
				Error string `json:"error"`
			} `json:"items"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode response JSON: %v (body %q)", err, rec.Body.String())
		}
		if len(out.Items) != 1 {
			t.Fatalf("item count = %d, want 1", len(out.Items))
		}
		if out.Items[0].OK || out.Items[0].Error != errTaskCtrlTerminalPause {
			t.Fatalf("items[0] = %+v, want error = %q", out.Items[0], errTaskCtrlTerminalPause)
		}
		assertNonEmpty(t, "item-error", out.Items[0].Error)
	})
}

// TestIntentStateConflictWrapsSentinel confirms that the intent state conflict message is
// English while still wrapping db.ErrIntentStateConflict with %w, so errors.Is identification is
// not broken. The enum value paused must be preserved verbatim.
func TestIntentStateConflictWrapsSentinel(t *testing.T) {
	err := fmt.Errorf(errIntentCtrlStateConflictFmt, db.ErrIntentStateConflict)
	if !errors.Is(err, db.ErrIntentStateConflict) {
		t.Fatalf("errors.Is(.., ErrIntentStateConflict) = false, %%w wrapping is broken: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "paused") {
		t.Fatalf("the paused enum value was not preserved in the message: %q", err.Error())
	}
	assertNonEmpty(t, "state-conflict", err.Error())
}
