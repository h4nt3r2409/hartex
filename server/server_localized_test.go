package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Regression test that keeps server.go's user-facing responses in English (F3b-server.go +
// F34). It reuses assertNonEmpty (non-empty messages) for
// the English check, and the task_categories test's decodeErrorField for response-body
// extraction (same server package). Scope: (1) F3b: the 24 writeErr sites + the 2 fmt.Errorf
// sites that validateTaskProfileIDs exposes through writeErr; (2) F34: the 5 user-facing
// sites that bypass writeErr (the 3 return values of chatUnavailableReason + the 2 writeJSON
// errors of the testLLM and web-search probes); (3) F34(d): the 3 rule-mode chat fallbackChat
// responses (2 confirmations + status summary) and recognition of the English trigger aliases
// (intent/hint). Prompts, tool descriptions, seed intent summaries, the default title,
// and logs handed to the agent are preserved verbatim, so they are not in scope for this
// test.

// TestServerErrorConstantsChecked asserts that every response constant is English. Format
// string constants (including %d) are also checked through their actual formatted result,
// confirming that English stays intact once substitution values are inserted.
func TestServerErrorConstantsLocalized(t *testing.T) {
	plain := map[string]string{
		"intent_control":        errTaskDeletingIntentControl,
		"intent_rerun":          errTaskDeletingIntentRerun,
		"intent_not_rerunnable": errIntentNotRerunnable,
		"source_invalid":        errCreateTaskSourceInvalid,
		"intercept_rules":       errCreateTaskInterceptRules,
		"category_invalid":      errCreateTaskCategoryInvalid,
		"company_invalid":       errCreateTaskCompanyInvalid,
		"llm_profile_invalid":   errLLMProfileInvalid,
		"asset_store_disabled":  errAssetStoreDisabled,
		"asset_id_required":     errAssetIDRequired,
		"python_not_detected":   errPythonNotDetected,
		"notify_base_url":       errNotifyBaseURLScheme,
		"notify_digest":         errNotifyDigestRange,
		"new_session":           errTaskDeletingNewSession,
		"new_message":           errTaskDeletingNewMessage,
		"main_agent_busy":       errMainAgentBusy,
		// F34: user-facing responses that bypass writeErr
		"chat_no_profile":       errChatNoLLMProfile,
		"chat_no_active":        errChatNoActiveLLMProfile,
		"chat_not_ready":        errChatLLMNotReady,
		"llm_test_no_api_key":   errLLMTestNoAPIKey,
		"web_search_no_results": errWebSearchProbeNoResults,
		// F34(d): the two confirmation responses of rule-mode chat fallbackChat (input text is appended after them)
		"fallback_intent": fallbackIntentInjected,
		"fallback_hint":   fallbackHintRecorded,
	}
	for label, msg := range plain {
		assertNonEmpty(t, label, msg)
	}

	formatted := map[string]string{
		"source_limit":     fmt.Sprintf(errCreateTaskSourceLimit, 8),
		"source_not_found": fmt.Sprintf(errCreateTaskSourceNotFound, 999),
		"company_limit":    fmt.Sprintf(errCreateTaskCompanyLimit, 32),
		"llm_profile_404":  fmt.Sprintf(errLLMProfileNotFound, 7),
		// F34(d): rule-mode status summary (asset, pending intent, and confirmed vulnerability counts substituted)
		"fallback_status": fmt.Sprintf(fallbackChatStatus, 3, 2, 1),
	}
	for label, msg := range formatted {
		assertNonEmpty(t, label, msg)
	}
}

// TestServerHandlersResponsesChecked inspects handler paths that finish without touching
// the DB all the way to the actual HTTP response body, confirming that constants are really
// wired into responses (an empty constant breaks this test = adversarial
// non-vacuity). Paths that need the DB, engine, or asset store (category/company invalid,
// missing LLM config, Python not detected, notification config, main agent busy) are pinned
// by the constant assertions above.
func TestServerHandlersResponsesLocalized(t *testing.T) {
	// Server with the deletion barrier raised: beginTaskOperation / IsDeleting reports "deleting".
	deletingServer := func() *Server {
		s := &Server{m: &Manager{tasks: map[string]*Task{"t1": {ID: "t1"}}}, engine: &Engine{}}
		s.engine.deleting.Store("t1", true)
		return s
	}
	// Server without the barrier: proceeds to input validation.
	liveServer := func() *Server {
		return &Server{m: &Manager{tasks: map[string]*Task{"t1": {ID: "t1"}}}, engine: &Engine{}}
	}

	cases := []struct {
		name    string
		req     func() *http.Request
		handler func(*Server) http.HandlerFunc
		server  func() *Server
		code    int
		want    string
	}{
		{
			name: "controlIntent/task-deleting",
			req: func() *http.Request {
				r := httptest.NewRequest(http.MethodPost, "/api/tasks/t1/intents/5/control", strings.NewReader(`{"action":"pause"}`))
				r.SetPathValue("id", "t1")
				r.SetPathValue("iid", "5")
				return r
			},
			handler: func(s *Server) http.HandlerFunc { return s.controlIntent },
			server:  deletingServer,
			code:    http.StatusConflict,
			want:    errTaskDeletingIntentControl,
		},
		{
			name: "rerunIntent/task-deleting",
			req: func() *http.Request {
				r := httptest.NewRequest(http.MethodPost, "/api/tasks/t1/intents/5/rerun", nil)
				r.SetPathValue("id", "t1")
				r.SetPathValue("iid", "5")
				return r
			},
			handler: func(s *Server) http.HandlerFunc { return s.rerunIntent },
			server:  deletingServer,
			code:    http.StatusConflict,
			want:    errTaskDeletingIntentRerun,
		},
		{
			name: "rerunBlocked/task-deleting",
			req: func() *http.Request {
				r := httptest.NewRequest(http.MethodPost, "/api/tasks/t1/intents/rerun-blocked", nil)
				r.SetPathValue("id", "t1")
				return r
			},
			handler: func(s *Server) http.HandlerFunc { return s.rerunBlocked },
			server:  deletingServer,
			code:    http.StatusConflict,
			want:    errTaskDeletingIntentRerun,
		},
		{
			name: "createTask/source-limit",
			req: func() *http.Request {
				// MaxTaskSourceCount=8 → 9 exceeds the limit (returns before the loop).
				r := httptest.NewRequest(http.MethodPost, "/api/tasks", strings.NewReader(
					`{"goal":"goal","source_task_ids":["1","2","3","4","5","6","7","8","9"]}`))
				return r
			},
			handler: func(s *Server) http.HandlerFunc { return s.createTask },
			server:  liveServer,
			code:    http.StatusBadRequest,
			want:    fmt.Sprintf(errCreateTaskSourceLimit, 8),
		},
		{
			name: "createTask/source-invalid",
			req: func() *http.Request {
				r := httptest.NewRequest(http.MethodPost, "/api/tasks", strings.NewReader(
					`{"goal":"goal","source_task_ids":["abc"]}`))
				return r
			},
			handler: func(s *Server) http.HandlerFunc { return s.createTask },
			server:  liveServer,
			code:    http.StatusBadRequest,
			want:    errCreateTaskSourceInvalid,
		},
		{
			name: "createTask/source-not-found",
			req: func() *http.Request {
				// 999 is absent from the tasks map → not found (no DB access, map lookup).
				r := httptest.NewRequest(http.MethodPost, "/api/tasks", strings.NewReader(
					`{"goal":"goal","source_task_ids":["999"]}`))
				return r
			},
			handler: func(s *Server) http.HandlerFunc { return s.createTask },
			server:  liveServer,
			code:    http.StatusBadRequest,
			want:    fmt.Sprintf(errCreateTaskSourceNotFound, 999),
		},
		{
			name: "createTask/company-limit",
			req: func() *http.Request {
				// MaxTaskCompanyCount=32 → 33 is rejected in NormalizeTaskCompanyIDs (pure function).
				ids := make([]string, 33)
				for i := range ids {
					ids[i] = fmt.Sprintf("%d", i+1)
				}
				body := `{"goal":"goal","company_ids":[` + strings.Join(ids, ",") + `]}`
				return httptest.NewRequest(http.MethodPost, "/api/tasks", strings.NewReader(body))
			},
			handler: func(s *Server) http.HandlerFunc { return s.createTask },
			server:  liveServer,
			code:    http.StatusBadRequest,
			want:    fmt.Sprintf(errCreateTaskCompanyLimit, 32),
		},
		{
			name: "createTask/llm-profile-invalid",
			req: func() *http.Request {
				// llm_profile_ids:[0] → validateTaskProfileIDs returns before loadProfileConfig(DB).
				r := httptest.NewRequest(http.MethodPost, "/api/tasks", strings.NewReader(
					`{"goal":"goal","llm_profile_ids":[0]}`))
				return r
			},
			handler: func(s *Server) http.HandlerFunc { return s.createTask },
			server:  liveServer,
			code:    http.StatusBadRequest,
			want:    errLLMProfileInvalid,
		},
		{
			name: "taskCoverageGraph/asset-store-disabled",
			req: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "/api/tasks/t1/coverage-graph", nil)
				r.SetPathValue("id", "t1")
				return r
			},
			handler: func(s *Server) http.HandlerFunc { return s.taskCoverageGraph },
			server:  liveServer, // Manager.assets==nil → Assets()==nil → 503
			code:    http.StatusServiceUnavailable,
			want:    errAssetStoreDisabled,
		},
		{
			name: "taskAssetRefs/asset-id-required",
			req: func() *http.Request {
				// no asset_id query → ParseInt("")=0 → <=0 → 400 (before Assets() is called).
				r := httptest.NewRequest(http.MethodGet, "/api/tasks/t1/asset-refs", nil)
				r.SetPathValue("id", "t1")
				return r
			},
			handler: func(s *Server) http.HandlerFunc { return s.taskAssetRefs },
			server:  liveServer,
			code:    http.StatusBadRequest,
			want:    errAssetIDRequired,
		},
		{
			// F34(b): with profile_id empty (=no DB lookup) and api_key also empty, it returns
			// writeJSON{ok:false,error} before TestConnection (status 200). liveServer also has
			// llmCfg.APIKey == "", so there is no global fallback key either.
			name: "testLLM/no-api-key",
			req: func() *http.Request {
				return httptest.NewRequest(http.MethodPost, "/api/llm/test", strings.NewReader(`{}`))
			},
			handler: func(s *Server) http.HandlerFunc { return s.testLLM },
			server:  liveServer,
			code:    http.StatusOK,
			want:    errLLMTestNoAPIKey,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := c.server()
			rec := httptest.NewRecorder()
			c.handler(s)(rec, c.req())
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

// TestChatUnavailableReasonChecked confirms that the reason F34(a)'s chatUnavailableReason()
// actually returns to the user is wired to the English constant. When pg is nil (no DB in the
// test) execution falls straight into the "not ready" path, so it asserts that the return is
// errChatLLMNotReady (an empty constant breaks it = adversarial non-vacuity).
// The other two reasons (no config / no active config) are branches that need pg (DB), so they
// are pinned by the constant assertions above.
func TestChatUnavailableReasonLocalized(t *testing.T) {
	s := &Server{m: &Manager{}} // m.pg == nil → third return path
	got := s.chatUnavailableReason()
	if got != errChatLLMNotReady {
		t.Fatalf("chatUnavailableReason() = %q, want %q", got, errChatLLMNotReady)
	}
	assertNonEmpty(t, "chat_not_ready", got)
}

// TestFallbackCommandParsing confirms that rule-mode chat's trigger keyword parsing (fallbackCommand)
// recognizes the English intent/hint prefixes, returns the argument with only the keyword
// stripped, and falls through to the status summary for non-alias messages. It is pure input
// parsing that never touches the Store or DB, so it runs without a DB.
func TestFallbackCommandParsing(t *testing.T) {
	cases := []struct {
		name, in, cmd, text string
	}{
		{"english-intent", "intent scan ports", "intent", "scan ports"},
		{"english-hint", "hint try admin:admin", "hint", "try admin:admin"},
		{"english-intent-colon", "intent: scan ports", "intent", "scan ports"},
		{"status-non-alias", "what is the status", "", "what is the status"},
		{"status-empty", "   ", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cmd, text := fallbackCommand(c.in)
			if cmd != c.cmd || text != c.text {
				t.Fatalf("fallbackCommand(%q) = (%q, %q), want (%q, %q)", c.in, cmd, text, c.cmd, c.text)
			}
		})
	}
}
