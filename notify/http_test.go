package notify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// This file covers doJSON's HTTP-layer error classification.
//
// Why test separately: channel adapters only handle each platform's own business error codes (DingTalk errcode,
// Feishu code, Telegram ok field), while **HTTP-layer** classification is done uniformly by doJSON;
// the two are independent lines of defense. Without this one, a relaying gateway returning 503 would be treated as permanent failure
// and retries abandoned outright; while a 403 would be treated as retryable and back off three pointless rounds.

func replyServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestDoJSONClassifiesHTTPStatus(t *testing.T) {
	cases := []struct {
		name      string
		status    int
		permanent bool
	}{
		{"200 success is not an error", 200, false},
		{"429 rate limit is retryable", 429, false},
		{"408 request timeout is retryable", 408, false},
		{"500 server error is retryable", 500, false},
		{"502 gateway error is retryable", 502, false},
		{"503 service unavailable is retryable", 503, false},
		{"400 bad request is permanent", 400, true},
		{"401 auth failure is permanent", 401, true},
		{"403 forbidden is permanent", 403, true},
		{"404 not found is permanent", 404, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := replyServer(t, tc.status, `{"detail":"upstream says no"}`)
			_, err := doJSON(context.Background(), "GET", srv.URL, nil, nil)
			if tc.status < 300 {
				if err != nil {
					t.Fatalf("2xx must not be an error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("non-2xx must be an error")
			}
			if got := IsPermanent(err); got != tc.permanent {
				t.Fatalf("wrong permanent classification for HTTP %d: want %v, got %v (%v)",
					tc.status, tc.permanent, got, err)
			}
			// The status code must appear in the error; otherwise users cannot tell whether they misconfigured something or the other side is down.
			// Assert the number rather than Go's English StatusText: the number is the
			// language-independent, stably assertable part of the message.
			if !strings.Contains(err.Error(), strconv.Itoa(tc.status)) {
				t.Errorf("the error must include HTTP status %d, got %v", tc.status, err)
			}
		})
	}
}

// TestDoJSONIncludesResponseSnippet covers the snippet: the error explanation returned by the other side must be brought back,
// otherwise users only know "it failed" and not why the other side rejected it.
func TestDoJSONIncludesResponseSnippet(t *testing.T) {
	srv := replyServer(t, 400, `{"error":"invalid webhook token"}`)
	_, err := doJSON(context.Background(), "GET", srv.URL, nil, nil)
	if err == nil {
		t.Fatal("want an error")
	}
	if !strings.Contains(err.Error(), "invalid webhook token") {
		t.Errorf("the error must carry the peer explanation, got %v", err)
	}
}

// TestDoJSONSnippetIsSingleLineAndBounded constrains the shape of the snippet:
// the peer response goes verbatim into the last_error column and the frontend table; multiple lines/an excessive length would break layout and payloads.
func TestDoJSONSnippetIsSingleLineAndBounded(t *testing.T) {
	// A response with newlines, tabs, and overlong 5000-character content.
	long := strings.Repeat("x", 5000)
	srv := replyServer(t, 500, "line1\nline2\r\n\tline3 "+long)
	_, err := doJSON(context.Background(), "GET", srv.URL, nil, nil)
	if err == nil {
		t.Fatal("want an error")
	}
	msg := err.Error()
	if strings.ContainsAny(msg, "\r\n\t") {
		t.Errorf("the error must collapse to one line, got %q", msg)
	}
	// The snippet is capped at 200 characters plus a fixed prefix; the total must be far smaller than the original response.
	if len(msg) > 400 {
		t.Errorf("error message is too long (%d bytes) and must be snippet-truncated: %q", len(msg), msg)
	}
}

// TestDoJSONRejectsOversizedResponse confirms the read is bounded: when the other side abnormally returns oversized content,
// the entire response must not be read into memory (every delivery-history entry stores a copy of last_error).
func TestDoJSONRejectsOversizedResponse(t *testing.T) {
	huge := strings.Repeat("A", 1<<20) // 1 MiB
	srv := replyServer(t, 400, huge)
	_, err := doJSON(context.Background(), "GET", srv.URL, nil, nil)
	if err == nil {
		t.Fatal("want an error")
	}
	if len(err.Error()) > 400 {
		t.Errorf("an oversized response must be read with a limit and truncated; error length %d", len(err.Error()))
	}
}
