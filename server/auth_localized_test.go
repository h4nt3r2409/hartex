package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Regression guard protecting the English message coverage (backlog F3b) of auth response
// messages. It verifies two paths that run without a DB: (1) the requireAuth middleware
// does not go through pg, so check the actual HTTP response body, and (2) the remaining
// handler messages are named constants, so check the constants themselves. The English
// check reuses the assertNonEmpty helper (contains English text, zero non-English content
// characters).

// TestRequireAuthMessagesChecked verifies through the real HTTP handler that the 401
// response body returned by requireAuth when the token is missing or invalid is English and
// non-empty English text. This middleware does not use the database, so it runs to
// completion even in an environment without a DB.
func TestRequireAuthMessagesLocalized(t *testing.T) {
	s := &Server{jwtKey: []byte(strings.Repeat("k", 32))}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := s.requireAuth(next)

	cases := []struct {
		name   string
		bearer string
		want   string
	}{
		{"missing token", "", authErrUnauthorized},
		{"invalid token", "Bearer not-a-valid-token", authErrTokenInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
			if c.bearer != "" {
				r.Header.Set("Authorization", c.bearer)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, r)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", rec.Code)
			}
			body := rec.Body.String()
			want := c.want
			if !strings.Contains(body, want) {
				t.Errorf("response body is missing %q: %s", want, body)
			}
			assertNonEmpty(t, c.name, body)
		})
	}
}

// TestAuthErrorConstantsChecked asserts that all user-facing message constants used by
// auth handlers contain English text and non-empty English text. It catches regressions
// without a DB, even for handlers that cannot be exercised directly through requireAuth
// (those that go through pg).
func TestAuthErrorConstantsLocalized(t *testing.T) {
	consts := map[string]string{
		"authErrUnauthorized":          authErrUnauthorized,
		"authErrTokenInvalid":          authErrTokenInvalid,
		"authErrPasswordAlreadySet":    authErrPasswordAlreadySet,
		"authErrPasswordEmpty":         authErrPasswordEmpty,
		"authErrNewPasswordEmpty":      authErrNewPasswordEmpty,
		"authErrPasswordHash":          authErrPasswordHash,
		"authErrSaveFailedPrefix":      authErrSaveFailedPrefix,
		"authErrTokenGen":              authErrTokenGen,
		"authErrBadRequest":            authErrBadRequest,
		"authErrPasswordNotInit":       authErrPasswordNotInit,
		"authErrCurrentPasswordWrong":  authErrCurrentPasswordWrong,
		"authErrBadCredential":         authErrBadCredential,
		"authErrPasswordTooShort":      authErrPasswordTooShort,
		"authErrSetupTokenInvalid":     authErrSetupTokenInvalid,
		"authErrTooManyAttempts":       authErrTooManyAttempts,
		"authErrPasswordTooLong":       authErrPasswordTooLong,
		"authErrDataSourceUnavailable": authErrDataSourceUnavailable,
	}
	for name, msg := range consts {
		assertNonEmpty(t, name, msg)
	}
}
