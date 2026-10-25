package server

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Regression guard keeping the chat attachment upload API error responses in chatupload.go
// in English. The check reuses assertNonEmpty (non-empty messages
// Han characters), and body extraction reuses decodeErrorField from the task_categories
// tests (same server package).

// TestChatUploadErrorConstantsChecked asserts that all 7 response constants are English.
// ...could not be reached directly: three of them are prefix constants with err.Error()
// appended, so their text ends with ": ", but they pass the check as long as they contain
// English text and non-empty English text.
func TestChatUploadErrorConstantsLocalized(t *testing.T) {
	cases := map[string]string{
		"scope_invalid": errChatUploadScopeInvalid,
		"bad_id":        errChatUploadBadID,
		"task_deleting": errChatUploadTaskDeleting,
		"mkdir":         errChatUploadMkdir,
		"parse":         errChatUploadParse,
		"no_file":       errChatUploadNoFile,
		"save_failed":   errChatUploadSaveFailed,
	}
	for label, msg := range cases {
		assertNonEmpty(t, label, msg)
	}
}

// TestChatUploadHandlerResponsesChecked exercises the chatUpload paths that finish without
// touching the DB through to the actual HTTP response body. scope/id validation returns
// before touching Manager, and no-file uses scope=session (not a task), so it finishes with
// only a temp directory and no engine or DB. task-deleting returns 409 via the same deletion
// barrier as the goals tests (engine.deleting).
func TestChatUploadHandlerResponsesLocalized(t *testing.T) {
	// multipart body with fields but no "file" — ParseMultipartForm passes but there are 0 files.
	noFileBody := func() (*bytes.Buffer, string) {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		_ = mw.WriteField("other", "x")
		_ = mw.Close()
		return &buf, mw.FormDataContentType()
	}

	t.Run("scope-invalid", func(t *testing.T) {
		s := &Server{}
		req := httptest.NewRequest(http.MethodPost, "/api/chat/upload?scope=bogus&id=x", nil)
		rec := httptest.NewRecorder()
		s.chatUpload(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 (body %q)", rec.Code, rec.Body.String())
		}
		if got := decodeErrorField(t, rec.Body.Bytes()); got != errChatUploadScopeInvalid {
			t.Fatalf("message = %q, want %q", got, errChatUploadScopeInvalid)
		}
		assertNonEmpty(t, "scope-invalid", errChatUploadScopeInvalid)
	})

	t.Run("bad-id", func(t *testing.T) {
		s := &Server{}
		// scope=session (not a task), so it returns at id validation before touching Manager.
		req := httptest.NewRequest(http.MethodPost, "/api/chat/upload?scope=session&id=../evil", nil)
		rec := httptest.NewRecorder()
		s.chatUpload(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 (body %q)", rec.Code, rec.Body.String())
		}
		if got := decodeErrorField(t, rec.Body.Bytes()); got != errChatUploadBadID {
			t.Fatalf("message = %q, want %q", got, errChatUploadBadID)
		}
		assertNonEmpty(t, "bad-id", errChatUploadBadID)
	})

	t.Run("no-file", func(t *testing.T) {
		s := &Server{m: &Manager{dir: t.TempDir()}}
		body, ctype := noFileBody()
		req := httptest.NewRequest(http.MethodPost, "/api/chat/upload?scope=session&id=sess1", body)
		req.Header.Set("Content-Type", ctype)
		rec := httptest.NewRecorder()
		s.chatUpload(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 (body %q)", rec.Code, rec.Body.String())
		}
		if got := decodeErrorField(t, rec.Body.Bytes()); got != errChatUploadNoFile {
			t.Fatalf("message = %q, want %q", got, errChatUploadNoFile)
		}
		assertNonEmpty(t, "no-file", errChatUploadNoFile)
	})

	t.Run("task-deleting", func(t *testing.T) {
		s := &Server{m: &Manager{tasks: map[string]*Task{"t1": {ID: "t1"}}}, engine: &Engine{}}
		s.engine.deleting.Store("t1", true)
		req := httptest.NewRequest(http.MethodPost, "/api/chat/upload?scope=task&id=t1", nil)
		rec := httptest.NewRecorder()
		s.chatUpload(rec, req)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status = %d, want 409 (body %q)", rec.Code, rec.Body.String())
		}
		if got := decodeErrorField(t, rec.Body.Bytes()); got != errChatUploadTaskDeleting {
			t.Fatalf("message = %q, want %q", got, errChatUploadTaskDeleting)
		}
		assertNonEmpty(t, "task-deleting", errChatUploadTaskDeleting)
	})
}
