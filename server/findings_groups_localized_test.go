package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A regression guard test that keeps the user-facing messages in findings_groups.go
// in English. The check reuses assertNonEmpty (contains English text, zero
// non-English content characters).

// TestDeepenFindingBodyTooLargeChecked verifies over real HTTP that the
// body-too-large (413) response is in English. This path returns from the
// MaxBytesReader decode step and finishes before reaching s.m.pg (the DB), so it
// runs to completion even with an empty Server.
func TestDeepenFindingBodyTooLargeLocalized(t *testing.T) {
	s := &Server{}
	body := `{"description":"` + strings.Repeat("a", 33<<10) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/exploration/findings/1/deepen", strings.NewReader(body))
	req.SetPathValue("id", "1")
	rec := httptest.NewRecorder()
	s.deepenFinding(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusRequestEntityTooLarge, rec.Body.String())
	}
	var resp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response JSON: %v (body %q)", err, rec.Body.String())
	}
	const want = "The request body is too large"
	if resp.Error != want {
		t.Fatalf("message = %q, want %q", resp.Error, want)
	}
	assertNonEmpty(t, "body_too_large", resp.Error)
}

// TestFindingFollowUpAuditSummaryChecked asserts that the follow-up intent
// activity summary constant is in English. AddFindingFollowUpIntent, which stores
// this summary, needs a DB transaction and therefore cannot run to completion on
// this host without a database, so the constant itself is asserted. This message is
// a summary used only for the activity timeline, separate from the intent payload
// the agent reads (the description entered by the user).
func TestFindingFollowUpAuditSummaryLocalized(t *testing.T) {
	assertNonEmpty(t, "follow_up_summary", auditFindingFollowUpSummary)
}
