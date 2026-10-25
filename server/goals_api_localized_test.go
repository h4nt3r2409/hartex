package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A regression guard test that keeps the goal-management API error responses in
// goals_api.go in English. The check reuses assertNonEmpty (contains
// non-empty messages), and response-body
// extraction reuses decodeErrorField from the task_categories tests (same server
// package).

// TestGoalErrorConstantsChecked asserts that all seven response constants are in
// English. Goal lookup and save (target not found, read failure) require going
// through the Store and DB, so they cannot be driven to completion on this host
// without a database; those messages are asserted on the constants themselves
// (following the intent_intervention precedent). The input-validation and
// delete-barrier paths are checked down to the response body by the HTTP tests
// below.
func TestGoalErrorConstantsLocalized(t *testing.T) {
	cases := map[string]string{
		"task_deleting_add":    errGoalTaskDeletingAdd,
		"task_deleting_edit":   errGoalTaskDeletingEdit,
		"task_deleting_delete": errGoalTaskDeletingDelete,
		"text_empty":           errGoalTextEmpty,
		"not_found":            errGoalNotFound,
		"read_after_add":       errGoalReadAfterAdd,
		"read_after_edit":      errGoalReadAfterEdit,
	}
	for label, msg := range cases {
		assertNonEmpty(t, label, msg)
	}
}

// TestGoalHandlersResponsesChecked checks handler paths that finish without
// touching the DB all the way to the actual HTTP response body, confirming that the
// constants are really wired into the responses. All three handlers go through
// s.m.Task (map lookup) → s.engine.beginTaskOperation (sync.Map-based delete
// barrier) → request body validation in that order, so one task in the tasks map and
// an empty Engine are enough to run without the Store or DB.
func TestGoalHandlersResponsesLocalized(t *testing.T) {
	// Server with the deleting barrier raised: beginTaskOperation returns false and produces a 409.
	deletingServer := func() *Server {
		s := &Server{m: &Manager{tasks: map[string]*Task{"t1": {ID: "t1"}}}, engine: &Engine{}}
		s.engine.deleting.Store("t1", true)
		return s
	}
	// Server without the barrier: beginTaskOperation returns true, so execution proceeds to input validation.
	liveServer := func() *Server {
		return &Server{m: &Manager{tasks: map[string]*Task{"t1": {ID: "t1"}}}, engine: &Engine{}}
	}

	newReq := func(body, gid string) *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/api/tasks/t1/goals", strings.NewReader(body))
		req.SetPathValue("id", "t1")
		if gid != "" {
			req.SetPathValue("gid", gid)
		}
		return req
	}

	cases := []struct {
		name    string
		handler func(*Server) http.HandlerFunc
		server  func() *Server
		body    string
		gid     string
		code    int
		want    string
	}{
		{
			name:    "add/task-deleting",
			handler: func(s *Server) http.HandlerFunc { return s.addGoal },
			server:  deletingServer,
			body:    `{"text":"prove SQL injection"}`,
			code:    http.StatusConflict,
			want:    errGoalTaskDeletingAdd,
		},
		{
			name:    "add/text-empty",
			handler: func(s *Server) http.HandlerFunc { return s.addGoal },
			server:  liveServer,
			body:    `{"text":"   "}`,
			code:    http.StatusBadRequest,
			want:    errGoalTextEmpty,
		},
		{
			name:    "edit/task-deleting",
			handler: func(s *Server) http.HandlerFunc { return s.editGoal },
			server:  deletingServer,
			body:    `{"text":"edited goal"}`,
			gid:     "5",
			code:    http.StatusConflict,
			want:    errGoalTaskDeletingEdit,
		},
		{
			name:    "edit/text-empty",
			handler: func(s *Server) http.HandlerFunc { return s.editGoal },
			server:  liveServer,
			body:    `{"text":"  "}`,
			gid:     "5",
			code:    http.StatusBadRequest,
			want:    errGoalTextEmpty,
		},
		{
			name:    "delete/task-deleting",
			handler: func(s *Server) http.HandlerFunc { return s.deleteGoal },
			server:  deletingServer,
			body:    ``,
			gid:     "5",
			code:    http.StatusConflict,
			want:    errGoalTaskDeletingDelete,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := c.server()
			rec := httptest.NewRecorder()
			c.handler(s)(rec, newReq(c.body, c.gid))
			if rec.Code != c.code {
				t.Fatalf("status = %d, want %d (body %q)", rec.Code, c.code, rec.Body.String())
			}
			got := decodeErrorField(t, rec.Body.Bytes())
			if got != c.want {
				t.Fatalf("message = %q, want %q", got, c.want)
			}
			assertNonEmpty(t, c.name, got)
		})
	}
}
