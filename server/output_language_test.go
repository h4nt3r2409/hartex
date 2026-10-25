package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestSettingsPayloadHasNoOutputLanguage pins that the output-language setting is
// gone from the public settings API. The English-only global edition has no runtime
// language selection, so GET /api/settings must not expose the field.
func TestSettingsPayloadHasNoOutputLanguage(t *testing.T) {
	m, err := NewManager(t.TempDir(), "")
	if err != nil {
		t.Skipf("postgres unavailable (%v) — skipping", err)
	}
	defer m.Close()
	s := New(context.Background(), m, t.TempDir(), t.TempDir(), t.TempDir())
	token, err := signJWT(s.jwtKey)
	if err != nil {
		t.Fatal(err)
	}
	do := func(method, path string, body any) *httptest.ResponseRecorder {
		var buf bytes.Buffer
		if body != nil {
			_ = json.NewEncoder(&buf).Encode(body)
		}
		req := httptest.NewRequest(method, path, &buf)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, req)
		return rec
	}

	rec := do(http.MethodGet, "/api/settings", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/settings = %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["output_language"]; ok {
		t.Fatal("GET /api/settings must not expose output_language")
	}
}

// TestPutOutputLanguageIsIgnored pins that a PUT /api/settings carrying an
// output_language field is ignored. The English-only edition has no runtime
// language selection, so the unknown field decodes to nothing: the request
// succeeds without error and a follow-up GET still does not expose the key.
func TestPutOutputLanguageIsIgnored(t *testing.T) {
	m, err := NewManager(t.TempDir(), "")
	if err != nil {
		t.Skipf("postgres unavailable (%v) — skipping", err)
	}
	defer m.Close()
	s := New(context.Background(), m, t.TempDir(), t.TempDir(), t.TempDir())
	token, err := signJWT(s.jwtKey)
	if err != nil {
		t.Fatal(err)
	}
	do := func(method, path string, body any) *httptest.ResponseRecorder {
		var buf bytes.Buffer
		if body != nil {
			_ = json.NewEncoder(&buf).Encode(body)
		}
		req := httptest.NewRequest(method, path, &buf)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, req)
		return rec
	}

	rec := do(http.MethodPut, "/api/settings", map[string]string{"output_language": "ko"})
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /api/settings = %d: %s", rec.Code, rec.Body.String())
	}

	rec = do(http.MethodGet, "/api/settings", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/settings = %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["output_language"]; ok {
		t.Fatal("GET /api/settings must not expose output_language after an ignored PUT")
	}
}
