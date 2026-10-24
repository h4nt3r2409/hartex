package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Autumn-27/artex/db"
)

// Regression guard keeping the validation error responses returned by writeErr from the
// custom tool CRUD and test-run endpoints in customtool.go in Korean. The Korean check
// reuses F3a's assertKoreanError (contains Hangul, zero Chinese Han characters), and body
// extraction reuses decodeErrorField from the task_categories tests (same server package).
// The actool.Errorf tool results and tool schema description read by the agent sit on the
// brain boundary, so they are not translation targets and this test does not touch them.

// TestCustomToolErrorConstantsLocalized asserts that all 8 constants exposed through
// writeErr are Korean. Two of them (errCustomToolKeyExists, errCustomToolEditCustomOnly)
// are only reachable after the handler goes through pg.GetTool (DB), so the real HTTP tests
// below do not reach them. They are pinned by constant assertions, and "which branch uses
// each constant" was confirmed by tracing the code paths (the same DB-path verification
// density as goals_api and task_control).
func TestCustomToolErrorConstantsLocalized(t *testing.T) {
	for _, c := range []struct{ name, msg string }{
		{"keyFormat", errCustomToolKeyFormat},
		{"kindInvalid", errCustomToolKindInvalid},
		{"httpSchemaRequired", errCustomToolHTTPSchemaRequired},
		{"keyExists", errCustomToolKeyExists},
		{"editCustomOnly", errCustomToolEditCustomOnly},
		{"badBody", errCustomToolBadBody},
		{"shellNoExec", errCustomToolShellNoExec},
		{"unknownKindPrefix", errCustomToolUnknownKindPrefix},
	} {
		assertKoreanError(t, "customtool."+c.name, c.msg)
	}
}

// newCustomToolServer creates a Server that passes the pg gate (s.pg) but has no real DB
// connection. &db.DB{} embeds a nil *sql.DB, but the pointer itself is non-nil, so pg() does
// not write 503 and returns it as is. All validation branches below return "before" the
// actual DB call such as pg.GetTool, so they never dereference the nil DB (confirmed via code
// paths).
func newCustomToolServer() *Server {
	return &Server{m: &Manager{pg: &db.DB{}}}
}

// TestCustomToolCreateValidationLocalized drives the 3 input validation paths of
// pgCreateCustomTool through real HTTP and checks that the response body is Korean. All three
// paths return before pg.GetTool (line 68).
func TestCustomToolCreateValidationLocalized(t *testing.T) {
	cases := []struct {
		name, body, want string
	}{
		{"bad-key", `{"key":"BadKey","kind":"command"}`, errCustomToolKeyFormat},
		{"bad-kind", `{"key":"goodkey","kind":"bogus"}`, errCustomToolKindInvalid},
		{"http-no-schema", `{"key":"goodkey","kind":"http"}`, errCustomToolHTTPSchemaRequired},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newCustomToolServer()
			req := httptest.NewRequest(http.MethodPost, "/api/tools/custom", strings.NewReader(c.body))
			rec := httptest.NewRecorder()
			s.pgCreateCustomTool(rec, req)
			if rec.Code != 400 && rec.Code != 409 {
				t.Fatalf("검증 오류 상태 코드를 기대했으나 %d (본문 %q)", rec.Code, rec.Body.String())
			}
			got := decodeErrorField(t, rec.Body.Bytes())
			if got != trMsg(c.want) {
				t.Fatalf("응답 본문 불일치: got %q want %q", got, c.want)
			}
			assertKoreanError(t, "customtool.create."+c.name, got)
		})
	}
}

// TestCustomToolTestRunValidationLocalized drives the 3 input validation paths of
// pgTestCustomTool through real HTTP and checks that the response body is Korean. None of
// the three makes a DB call.
func TestCustomToolTestRunValidationLocalized(t *testing.T) {
	cases := []struct {
		name, body, want string
	}{
		{"bad-body", `not-json`, errCustomToolBadBody},
		{"shell", `{"kind":"shell"}`, errCustomToolShellNoExec},
		{"unknown-kind", `{"kind":"bogus"}`, trMsg(errCustomToolUnknownKindPrefix) + "bogus"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newCustomToolServer()
			req := httptest.NewRequest(http.MethodPost, "/api/tools/custom/test", strings.NewReader(c.body))
			rec := httptest.NewRecorder()
			s.pgTestCustomTool(rec, req)
			if rec.Code != 400 {
				t.Fatalf("상태 코드 400 을 기대했으나 %d (본문 %q)", rec.Code, rec.Body.String())
			}
			got := decodeErrorField(t, rec.Body.Bytes())
			if got != trMsg(c.want) {
				t.Fatalf("응답 본문 불일치: got %q want %q", got, c.want)
			}
			// unknown-kind gets the Latin suffix req.Kind("bogus") appended, so only check for zero Han characters.
			assertKoreanError(t, "customtool.test."+c.name, errCustomToolUnknownKindPrefix)
		})
	}
}
