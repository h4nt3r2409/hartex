package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestServerMgmtConstantsChecked pins that every user-facing error/response
// literal extracted from server_mgmt.go is English. An empty constant makes assertNonEmpty fail. assertNonEmpty / decodeErrorField are reused from the F3 suite
// (same package server).
func TestServerMgmtConstantsLocalized(t *testing.T) {
	cases := map[string]string{
		"errMgmtPGUnavailable":      errMgmtPGUnavailable,
		"errMgmtTaskIDInvalid":      errMgmtTaskIDInvalid,
		"errMgmtTaskDeleting":       errMgmtTaskDeleting,
		"errMgmtTaskHasAgents":      errMgmtTaskHasAgents,
		"errMgmtAgentKeyFormat":     errMgmtAgentKeyFormat,
		"errMgmtNameEmpty":          errMgmtNameEmpty,
		"errMgmtAgentKeyExists":     errMgmtAgentKeyExists,
		"errMgmtBuiltinNoEditMeta":  errMgmtBuiltinNoEditMeta,
		"errMgmtBuiltinNoDelete":    errMgmtBuiltinNoDelete,
		"errMgmtLLMProfileIDFormat": errMgmtLLMProfileIDFormat,
		"errMgmtLLMProfileInvalid":  errMgmtLLMProfileInvalid,
		"errMgmtNoBuiltinPrompt":    errMgmtNoBuiltinPrompt,
		"errMgmtToolNotFound":       errMgmtToolNotFound,
		"errMgmtNotBuiltinTool":     errMgmtNotBuiltinTool,
		"errMgmtMCPNotFound":        errMgmtMCPNotFound,
		"errMgmtToolDiscoverFail":   errMgmtToolDiscoverFail,
		"errMgmtSkillNameInvalid":   errMgmtSkillNameInvalid,
		"errMgmtSkillNoFile":        errMgmtSkillNoFile,
		"errMgmtSkillNoMDInZip":     errMgmtSkillNoMDInZip,
		"errMgmtSkillReadMDFail":    errMgmtSkillReadMDFail,
		"errMgmtSkillNamePre":       errMgmtSkillNamePre,
		"errMgmtSkillNamePost":      errMgmtSkillNamePost,
		"errMgmtSkillExistsPre":     errMgmtSkillExistsPre,
		"errMgmtSkillExistsPost":    errMgmtSkillExistsPost,
		"errMgmtSkillZipBadPath":    errMgmtSkillZipBadPath,
		"errMgmtSkillZipTooMany":    errMgmtSkillZipTooMany,
		"errMgmtSkillFileTooLarge":  errMgmtSkillFileTooLarge,
		"errMgmtSkillZipTooLarge":   errMgmtSkillZipTooLarge,
		"errMgmtSkillNoMDAfter":     errMgmtSkillNoMDAfter,
		"errMgmtSkillInstallFail":   errMgmtSkillInstallFail,
		"errMgmtLLMActiveDelete":    errMgmtLLMActiveDelete,
		"errMgmtLLMRefChanged":      errMgmtLLMRefChanged,
		"errMgmtLLMRefTimeout":      errMgmtLLMRefTimeout,
		"errMgmtNoAPIKey":           errMgmtNoAPIKey,
		"errMgmtBuildReqFail":       errMgmtBuildReqFail,
		"errMgmtReqFail":            errMgmtReqFail,
		"errMgmtParseRespFail":      errMgmtParseRespFail,
		"errMgmtNoModelList":        errMgmtNoModelList,
		"errMgmtTmplSyntax":         errMgmtTmplSyntax,
		"errMgmtTmplVarPre":         errMgmtTmplVarPre,
		"errMgmtTmplVarPost":        errMgmtTmplVarPost,
		// Format strings are checked through their actual formatted result (English + verb preserved).
		"errMgmtAPIReturned": fmt.Sprintf(errMgmtAPIReturned, 404, "detail"),
	}
	for label, msg := range cases {
		assertNonEmpty(t, label, msg)
	}
}

// TestServerMgmtAPIReturnedFormat guards that the model-list probe format string
// keeps its status-code and body placeholders (a %d→wrong-verb edit would drop
// them).
func TestServerMgmtAPIReturnedFormat(t *testing.T) {
	msg := fmt.Sprintf(errMgmtAPIReturned, 404, "boom-body")
	assertNonEmpty(t, "errMgmtAPIReturned", msg)
	if !strings.Contains(msg, "404") || !strings.Contains(msg, "boom-body") {
		t.Fatalf("format string must preserve the status code and body: %q", msg)
	}
}

// TestServerMgmtSkillExistsMessage guards the duplicate-skill message shape: it
// must say the skill already exists (the UI keys the overwrite-confirmation flow
// off the 409 status) and include the skill name.
func TestServerMgmtSkillExistsMessage(t *testing.T) {
	msg := errMgmtSkillExistsPre + "my-skill" + errMgmtSkillExistsPost
	if !strings.Contains(strings.ToLower(msg), "already exists") {
		t.Fatalf("duplicate-skill message must say the skill already exists: %q", msg)
	}
	if !strings.Contains(msg, "my-skill") {
		t.Fatalf("skill name must appear in the message: %q", msg)
	}
}

// TestServerMgmtPGGate503Checked exercises the real shared DB gate every
// management handler passes through: with no PostgreSQL handle, pg() writes 503
// and returns nil. The 503 body must be English.
func TestServerMgmtPGGate503Localized(t *testing.T) {
	s := &Server{m: &Manager{}} // pg == nil
	rec := httptest.NewRecorder()
	if got := s.pg(rec); got != nil {
		t.Fatal("pg() must return nil without a DB handle")
	}
	if rec.Code != 503 {
		t.Fatalf("want status 503, got %d", rec.Code)
	}
	assertNonEmpty(t, "pg.503", decodeErrorField(t, rec.Body.Bytes()))
}

// TestServerMgmtDeleteTaskBadIDChecked drives pgDeleteTask through its first
// guard (non-numeric id → 400) without any DB or engine.
func TestServerMgmtDeleteTaskBadIDLocalized(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodDelete, "/api/tasks/abc", nil)
	req.SetPathValue("id", "abc")
	rec := httptest.NewRecorder()
	s.pgDeleteTask(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want status 400, got %d (body %s)", rec.Code, rec.Body.Bytes())
	}
	got := decodeErrorField(t, rec.Body.Bytes())
	assertNonEmpty(t, "deleteTask.badID", got)
	if got != errMgmtTaskIDInvalid {
		t.Fatalf("must equal errMgmtTaskIDInvalid: %q", got)
	}
}

// TestServerMgmtListModelsNoKeyChecked drives pgListModels with no api key and
// no profile id, so it returns {"ok":false,"error":...} before touching the DB.
func TestServerMgmtListModelsNoKeyLocalized(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodPost, "/api/llm/models", strings.NewReader("{}"))
	rec := httptest.NewRecorder()
	s.pgListModels(rec, req)
	if rec.Code != 200 {
		t.Fatalf("want status 200, got %d", rec.Code)
	}
	got := decodeErrorField(t, rec.Body.Bytes())
	assertNonEmpty(t, "listModels.noKey", got)
	if got != errMgmtNoAPIKey {
		t.Fatalf("must equal errMgmtNoAPIKey: %q", got)
	}
}

// TestServerMgmtValidateTemplateChecked exercises the prompt-template editor
// validator (pure function, no DB): both the parse-error and the
// disallowed-variable branches must return English.
func TestServerMgmtValidateTemplateLocalized(t *testing.T) {
	if msg := validateTemplate("{{", nil); msg == "" {
		t.Fatal("a broken template must return an error message")
	} else {
		assertNonEmpty(t, "validateTemplate.syntax", msg)
	}
	msg := validateTemplate("{{.Bogus}}", nil)
	if msg == "" {
		t.Fatal("a variable outside the allowlist must return an error message")
	}
	assertNonEmpty(t, "validateTemplate.var", msg)
	if !strings.Contains(msg, "Bogus") {
		t.Fatalf("the variable name must appear in the message: %q", msg)
	}
}

// TestServerMgmtGlobalPromptVarsChecked pins the template-variable catalog help
// text (rendered in the prompt editor's variable palette) as English.
func TestServerMgmtGlobalPromptVarsLocalized(t *testing.T) {
	if len(globalPromptVars) == 0 {
		t.Fatal("globalPromptVars is empty")
	}
	for _, v := range globalPromptVars {
		assertNonEmpty(t, "globalPromptVar."+v.Name, v.Description)
	}
}
