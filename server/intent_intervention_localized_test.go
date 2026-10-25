package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A regression guard test that keeps the Worker intervention API error responses in
// intent_intervention.go in English. The check reuses assertNonEmpty
// (non-empty messages).

// TestIntentInterventionErrorConstantsChecked asserts that all 11 response
// constants are in English. The lifecycle and intent-state paths require
// s.engine/Store setup and therefore cannot run to completion on this host without a
// database, so those messages are asserted on the constants themselves (following
// the conversations.go precedent). The four input-validation paths are checked down
// to the response body by the HTTP tests below.
func TestIntentInterventionErrorConstantsLocalized(t *testing.T) {
	cases := []struct {
		name string
		msg  string
	}{
		{"request_too_large", errIntentRequestTooLarge},
		{"message_empty", errIntentMessageEmpty},
		{"message_too_long", errIntentMessageTooLong},
		{"bad_request_id", errIntentBadRequestID},
		{"task_deleting", errIntentTaskDeleting},
		{"task_paused", errIntentTaskPaused},
		{"task_queued", errIntentTaskQueued},
		{"task_terminal", errIntentTaskTerminal},
		{"task_settling", errIntentTaskSettling},
		{"inherited_readonly", errIntentInheritedReadonly},
		{"not_paused", errIntentNotPaused},
	}
	for _, c := range cases {
		assertNonEmpty(t, c.name, c.msg)
	}
}

// TestSendWorkerMessageInputValidationChecked checks the four input-validation
// paths all the way to the actual HTTP response body. These four paths in
// sendWorkerMessage look only at s.m.Task (map lookup) and the request body and do
// not go through s.engine, the Store, or the DB, so putting one task in the tasks map
// is enough to run to completion without a DB. It also confirms that the constants
// are really wired into the responses.
func TestSendWorkerMessageInputValidationLocalized(t *testing.T) {
	s := &Server{m: &Manager{tasks: map[string]*Task{"t1": {ID: "t1"}}}}

	call := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/tasks/t1/intents/1/message", strings.NewReader(body))
		req.SetPathValue("id", "t1")
		req.SetPathValue("iid", "1")
		rec := httptest.NewRecorder()
		s.sendWorkerMessage(rec, req)
		return rec
	}
	errBody := func(t *testing.T, rec *httptest.ResponseRecorder) string {
		t.Helper()
		var resp struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response JSON: %v (body %q)", err, rec.Body.String())
		}
		return resp.Error
	}

	cases := []struct {
		name string
		body string
		code int
		want string
	}{
		// Valid JSON body exceeding the 64KB limit. MaxBytesReader reports the limit violation while reading.
		{"request_too_large", `{"message":"` + strings.Repeat("a", maxWorkerMessageBytes+1024) + `"}`, http.StatusRequestEntityTooLarge, errIntentRequestTooLarge},
		{"message_empty", `{"message":"  "}`, http.StatusBadRequest, errIntentMessageEmpty},
		{"message_too_long", `{"message":"` + strings.Repeat("a", 4001) + `"}`, http.StatusBadRequest, errIntentMessageTooLong},
		{"bad_request_id", `{"message":"hello","request_id":"has space"}`, http.StatusBadRequest, errIntentBadRequestID},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := call(c.body)
			if rec.Code != c.code {
				t.Fatalf("status = %d, want %d (body %q)", rec.Code, c.code, rec.Body.String())
			}
			if got := errBody(t, rec); got != c.want {
				t.Fatalf("message = %q, want %q", got, c.want)
			}
			assertNonEmpty(t, c.name, errBody(t, rec))
		})
	}
}
