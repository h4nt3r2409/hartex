package server

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// F3b(update.go): regression test that keeps the self-update endpoint's user-facing messages in
// English. The frontend (system/settings update-card) renders progress.message, reason, and the
// writeErr body as-is, so reverting these messages to a non-English source would bring non-English toasts and
// progress messages back to the update screen. The assertNonEmpty/decodeErrorField helpers are
// reused from the earlier F3b test files (same server package).

func TestUpdateMessageConstantsLocalized(t *testing.T) {
	assertNonEmpty(t, "updateMsgPreparing", updateMsgPreparing)
	assertNonEmpty(t, "updateMsgFailed", updateMsgFailed)
	assertNonEmpty(t, "updateMsgStaged", updateMsgStaged)
	assertNonEmpty(t, "updateErrInProgress", updateErrInProgress)
	assertNonEmpty(t, "updateErrRollbackInProgress", updateErrRollbackInProgress)

	// Format string constants are checked after filling in their placeholders (%q/%s are substituted and zero Han characters remain).
	notRelease := fmt.Sprintf(updateErrNotReleaseFmt, "v0.0.0-dev")
	assertNonEmpty(t, "updateErrNotReleaseFmt", notRelease)
	if !strings.Contains(notRelease, "v0.0.0-dev") {
		t.Fatalf("updateErrNotReleaseFmt: version placeholder was not substituted: %q", notRelease)
	}
	latest := fmt.Sprintf(updateErrAlreadyLatestFmt, "v1.2.3")
	assertNonEmpty(t, "updateErrAlreadyLatestFmt", latest)
	if !strings.Contains(latest, "v1.2.3") {
		t.Fatalf("updateErrAlreadyLatestFmt: version placeholder was not substituted: %q", latest)
	}
}

// Verify through the actual code path that the progress messages begin/finish emit over SSE are
// set from the English constants. A local instance is used to avoid polluting the global updHub
// (no DB or network needed).
func TestUpdateProgressMessagesLocalized(t *testing.T) {
	h := &updateHub{subs: map[chan updateProgress]struct{}{}}

	if !h.begin("v1.2.3") {
		t.Fatal("begin must be true on the first call")
	}
	cur, running := h.snapshot()
	if !running {
		t.Fatal("state must be running after begin")
	}
	if cur.Message != updateMsgPreparing {
		t.Fatalf("preparing message mismatch: %q", cur.Message)
	}
	if h.begin("v1.2.4") {
		t.Fatal("begin must be false while in progress")
	}

	h.finish(errors.New("download failed"))
	cur, running = h.snapshot()
	if running {
		t.Fatal("running must be cleared after finish")
	}
	if cur.Message != updateMsgFailed {
		t.Fatalf("failure message mismatch: %q", cur.Message)
	}

	h.finish(nil)
	cur, _ = h.snapshot()
	if cur.Message != updateMsgStaged {
		t.Fatalf("ready message mismatch: %q", cur.Message)
	}
}

// Verify over real HTTP, without a DB, that updateRollback's "cannot roll back while in progress" 409 response is English.
func TestUpdateRollbackInProgressLocalized(t *testing.T) {
	updHub.mu.Lock()
	prev := updHub.running
	updHub.running = true
	updHub.mu.Unlock()
	defer func() {
		updHub.mu.Lock()
		updHub.running = prev
		updHub.mu.Unlock()
	}()

	s := &Server{}
	req := httptest.NewRequest(http.MethodPost, "/api/update/rollback", nil)
	rec := httptest.NewRecorder()
	s.updateRollback(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("want status 409, got %d (body %s)", rec.Code, rec.Body.Bytes())
	}
	msg := decodeErrorField(t, rec.Body.Bytes())
	assertNonEmpty(t, "updateRollback in-progress 409", msg)
	if msg != updateErrRollbackInProgress {
		t.Fatalf("rollback-rejection message mismatch: %q", msg)
	}
}
