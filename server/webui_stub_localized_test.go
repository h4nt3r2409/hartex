//go:build !embedui

package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestWebUIStubErrorChecked guards the no-embed stub's HTTP response: a request
// served by the default (!embedui) build must get an English 404 body, not the old
// non-English one. This is the last user-facing HTTP error response in server/ that the
// earlier `writeErr(`-only sweeps missed (it uses http.Error, not writeErr). If an
// upstream re-sync reintroduces a non-English literal, this test fails.
func TestWebUIStubErrorLocalized(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	(&Server{}).webuiHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status is not 404: %d", rec.Code)
	}
	body := strings.TrimSpace(rec.Body.String())
	assertNonEmpty(t, "webui.stub", body)
	// Command references must be preserved verbatim (they are not translation targets).
	for _, want := range []string{"next dev", "-tags embedui"} {
		if !strings.Contains(body, want) {
			t.Fatalf("command reference %q is missing from the response: %q", want, body)
		}
	}
}
