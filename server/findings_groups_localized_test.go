package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A regression guard test that keeps the user-facing messages in findings_groups.go
// in Korean. Korean detection reuses assertKoreanError (contains Hangul, zero
// Chinese Han characters) introduced by F3a.

// TestDeepenFindingBodyTooLargeLocalized verifies over real HTTP that the
// body-too-large (413) response is in Korean. This path returns from the
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
		t.Fatalf("상태 코드 = %d, 기대 = %d (본문 %q)", rec.Code, http.StatusRequestEntityTooLarge, rec.Body.String())
	}
	var resp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("응답 JSON 파싱 실패: %v (본문 %q)", err, rec.Body.String())
	}
	const want = "요청 본문이 너무 큽니다"
	if resp.Error != trMsg(want) {
		t.Fatalf("응답 문구 = %q, 기대 = %q", resp.Error, want)
	}
	assertKoreanError(t, "body_too_large", resp.Error)
}

// TestFindingFollowUpAuditSummaryLocalized asserts that the follow-up intent
// activity summary constant is in Korean. AddFindingFollowUpIntent, which stores
// this summary, needs a DB transaction and therefore cannot run to completion on
// this host without a database, so the constant itself is asserted. This message is
// a summary used only for the activity timeline, separate from the intent payload
// the agent reads (the description entered by the user).
func TestFindingFollowUpAuditSummaryLocalized(t *testing.T) {
	assertKoreanError(t, "follow_up_summary", auditFindingFollowUpSummary)
}
