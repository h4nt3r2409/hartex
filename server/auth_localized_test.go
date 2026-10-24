package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Regression guard protecting the Korean localization (backlog F3b) of auth response
// messages. It verifies two paths that run without a DB: (1) the requireAuth middleware
// does not go through pg, so check the actual HTTP response body, and (2) the remaining
// handler messages are named constants, so check the constants themselves. The Korean
// check reuses the assertKoreanError helper from F3a (contains Hangul, zero Chinese Han
// characters).

// TestRequireAuthMessagesLocalized verifies through the real HTTP handler that the 401
// response body returned by requireAuth when the token is missing or invalid is Korean and
// free of Chinese Han characters. This middleware does not use the database, so it runs to
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
		{"토큰 없음", "", authErrUnauthorized},
		{"토큰 무효", "Bearer not-a-valid-token", authErrTokenInvalid},
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
				t.Fatalf("상태 코드 = %d, 기대 401", rec.Code)
			}
			body := rec.Body.String()
			want := trMsg(c.want)
			if !strings.Contains(body, want) {
				t.Errorf("응답 본문에 %q 가 없습니다: %s", want, body)
			}
			assertKoreanError(t, c.name, body)
		})
	}
}

// TestAuthErrorConstantsLocalized asserts that all user-facing message constants used by
// auth handlers contain Hangul and no Chinese Han characters. It catches regressions
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
		assertKoreanError(t, name, msg)
	}
}
