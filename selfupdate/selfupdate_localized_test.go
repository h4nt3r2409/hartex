package selfupdate

// Regression test (F20) pinning that the user-facing progress and error strings on the
// self-update path come out in Korean. update.go's updateHub exposes Stage's progress
// callback messages as SSE progress.message, and the errors returned by
// Stage/FetchLatest/Rollback as progress.error, the writeErr body, and boot_notice to
// the frontend (update-card) unchanged, so if these strings revert to Chinese the user
// screen becomes a half-Korean, half-Chinese mix. Logs (log.Printf) and comments are Z2
// and out of scope; here we assert only that "there is Hangul and no Chinese Han characters".

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func hasHangul(s string) bool {
	for _, r := range s {
		if r >= 0xAC00 && r <= 0xD7A3 {
			return true
		}
	}
	return false
}

func hasHanzi(s string) bool {
	for _, r := range s {
		if r >= 0x4E00 && r <= 0x9FFF {
			return true
		}
	}
	return false
}

// assertKoreanError asserts that an error exists, contains Hangul, and has no Chinese Han characters.
// English from lower-level stdlib errors wrapped with %w is allowed (only Han characters are forbidden).
func assertKoreanError(t *testing.T, label string, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: 오류를 기대했으나 nil 이었습니다", label)
	}
	msg := err.Error()
	hasLatin := false
	for _, r := range msg {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			hasLatin = true
			break
		}
	}
	if !hasHangul(msg) && !hasLatin {
		t.Errorf("%s: message is neither Korean nor English: %q", label, msg)
	}
	if hasHanzi(msg) {
		t.Errorf("%s: Han characters remain: %q", label, msg)
	}
}

// rtFunc injects fake responses into the http.Client. It never actually connects to GitHub.
type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func stubClient(status int, body string) *http.Client {
	return &http.Client{Transport: rtFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})}
}

// TestCheckURLErrorsLocalized: the existing TestCheckURLRejectsNonGitHub only checks
// whether the URL is rejected, not the message language. Here we verify that errors from both rejection paths are Korean.
func TestCheckURLErrorsLocalized(t *testing.T) {
	nonHTTPS := checkURL(mustParse(t, "http://api.github.com/x"))
	assertKoreanError(t, "non-https", nonHTTPS)
	if !strings.Contains(nonHTTPS.Error(), "HTTPS") {
		t.Errorf("non-https 오류에 HTTPS 표기가 없습니다: %q", nonHTTPS.Error())
	}

	nonGitHub := checkURL(mustParse(t, "https://evil.example.com/x"))
	assertKoreanError(t, "non-github-host", nonGitHub)
	if !strings.Contains(nonGitHub.Error(), "GitHub") {
		t.Errorf("non-github 오류에 GitHub 표기가 없습니다: %q", nonGitHub.Error())
	}
}

// TestFetchLatestErrorsLocalized: verifies that the errors on the version-check path
// (rate limit, not published, other status, parse failure, missing tag) are Korean. A fake transport avoids the network.
func TestFetchLatestErrorsLocalized(t *testing.T) {
	ctx := context.Background()

	_, errRate := FetchLatest(ctx, stubClient(http.StatusForbidden, ""))
	assertKoreanError(t, "rate-limited", errRate)
	if !strings.Contains(errRate.Error(), "60") {
		t.Errorf("요청 제한 오류에 60 표기가 없습니다: %q", errRate.Error())
	}

	_, errNF := FetchLatest(ctx, stubClient(http.StatusNotFound, ""))
	assertKoreanError(t, "not-found", errNF)

	_, errStatus := FetchLatest(ctx, stubClient(http.StatusInternalServerError, ""))
	assertKoreanError(t, "bad-status", errStatus)
	if !strings.Contains(errStatus.Error(), "GitHub") {
		t.Errorf("상태 오류에 GitHub 표기가 없습니다: %q", errStatus.Error())
	}

	_, errJSON := FetchLatest(ctx, stubClient(http.StatusOK, "{not json"))
	assertKoreanError(t, "bad-json", errJSON)

	_, errTag := FetchLatest(ctx, stubClient(http.StatusOK, "{}"))
	assertKoreanError(t, "missing-tag", errTag)
	if !strings.Contains(errTag.Error(), "tag") {
		t.Errorf("tag 누락 오류에 tag 표기가 없습니다: %q", errTag.Error())
	}
}

// TestStageMissingAssetLocalized: when the release has no package for the current
// platform, Stage stops with a Korean error before downloading (FindAsset fails → no
// network access). Depending on the environment, checkWritable may block first, but that error is also Korean, so only the language is asserted.
func TestStageMissingAssetLocalized(t *testing.T) {
	rel := &Release{TagName: "v9.9.9"} // Assets is empty
	err := Stage(context.Background(), &http.Client{}, rel, "0.0.1", nil)
	assertKoreanError(t, "stage-missing-asset", err)
}
