package server

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
)

const webuiTestJS = "console.log('hartex');"

func testWebUIFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":               {Data: []byte("<!doctype html><title>Hartex</title>")},
		"dashboard/index.html":     {Data: []byte("<h1>Dashboard</h1>")},
		"_next/static/chunks/a.js": {Data: []byte(webuiTestJS)},
		"_next/static/chunks/a.css": {
			Data: []byte("body{color:red}"),
		},
		"brand/hartex-mark.svg": {Data: []byte("<svg></svg>")},
		"favicon.ico":           {Data: []byte{0, 1, 2, 3}},
	}
}

func TestWebUICompressionCacheAndETag(t *testing.T) {
	h := newWebUIHandler(testWebUIFS())

	req := httptest.NewRequest(http.MethodGet, "/_next/static/chunks/a.js", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if got := rec.Header().Get("Vary"); !strings.Contains(got, "Accept-Encoding") {
		t.Fatalf("Vary = %q, want Accept-Encoding", got)
	}
	if got := rec.Header().Get("Cache-Control"); !strings.Contains(got, "immutable") {
		t.Fatalf("Cache-Control = %q, want immutable", got)
	}
	zr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	body, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("read gzip body: %v", err)
	}
	if string(body) != webuiTestJS {
		t.Fatalf("body = %q, want %q", body, webuiTestJS)
	}

	etag := rec.Header().Get("ETag")
	if !strings.HasPrefix(etag, `W/"`) {
		t.Fatalf("ETag = %q, want a weak content hash", etag)
	}
	req2 := httptest.NewRequest(http.MethodGet, "/_next/static/chunks/a.js", nil)
	req2.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusNotModified {
		t.Fatalf("revalidation status = %d, want 304", rec2.Code)
	}
	if rec2.Body.Len() != 0 {
		t.Fatalf("304 body = %q, want empty", rec2.Body.String())
	}
}

func TestWebUIHTMLAndSPAFallback(t *testing.T) {
	h := newWebUIHandler(testWebUIFS())

	cases := []struct {
		path string
		want string
	}{
		{"/", "<!doctype html><title>Hartex</title>"},
		{"/dashboard", "<h1>Dashboard</h1>"},
		{"/dashboard/", "<h1>Dashboard</h1>"},
		{"/missing", "<!doctype html><title>Hartex</title>"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", tc.path, rec.Code)
		}
		if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
			t.Fatalf("%s Content-Type = %q, want text/html", tc.path, got)
		}
		if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
			t.Fatalf("%s Cache-Control = %q, want no-cache", tc.path, got)
		}
		if !strings.Contains(rec.Body.String(), tc.want) {
			t.Fatalf("%s body = %q, want to contain %q", tc.path, rec.Body.String(), tc.want)
		}
	}
}

func TestWebUIHeadAndRange(t *testing.T) {
	h := newWebUIHandler(testWebUIFS())

	head := httptest.NewRequest(http.MethodHead, "/_next/static/chunks/a.js", nil)
	headRec := httptest.NewRecorder()
	h.ServeHTTP(headRec, head)
	if headRec.Code != http.StatusOK {
		t.Fatalf("HEAD status = %d, want 200", headRec.Code)
	}
	if headRec.Body.Len() != 0 {
		t.Fatalf("HEAD body = %q, want empty", headRec.Body.String())
	}
	if got := headRec.Header().Get("Content-Length"); got == "" {
		t.Fatal("HEAD Content-Length is missing")
	}

	rangeReq := httptest.NewRequest(http.MethodGet, "/_next/static/chunks/a.js", nil)
	rangeReq.Header.Set("Range", "bytes=0-3")
	rangeRec := httptest.NewRecorder()
	h.ServeHTTP(rangeRec, rangeReq)
	if rangeRec.Code != http.StatusPartialContent {
		t.Fatalf("range status = %d, want 206", rangeRec.Code)
	}
	if got := rangeRec.Body.String(); got != "cons" {
		t.Fatalf("range body = %q, want %q", got, "cons")
	}
	if got := rangeRec.Header().Get("Content-Range"); got == "" {
		t.Fatal("Content-Range is missing")
	}
}

func TestWebUINonCompressibleAndQZero(t *testing.T) {
	h := newWebUIHandler(testWebUIFS())

	ico := httptest.NewRequest(http.MethodGet, "/favicon.ico", nil)
	ico.Header.Set("Accept-Encoding", "gzip")
	icoRec := httptest.NewRecorder()
	h.ServeHTTP(icoRec, ico)
	if got := icoRec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("ico Content-Encoding = %q, want none", got)
	}
	if got := icoRec.Header().Get("Content-Type"); got != "image/x-icon" {
		t.Fatalf("ico Content-Type = %q, want image/x-icon", got)
	}
	if got := icoRec.Header().Get("Cache-Control"); !strings.Contains(got, "max-age") {
		t.Fatalf("ico Cache-Control = %q, want max-age", got)
	}

	qzero := httptest.NewRequest(http.MethodGet, "/_next/static/chunks/a.js", nil)
	qzero.Header.Set("Accept-Encoding", "gzip;q=0")
	qzeroRec := httptest.NewRecorder()
	h.ServeHTTP(qzeroRec, qzero)
	if got := qzeroRec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("q=0 Content-Encoding = %q, want none", got)
	}
}

func TestWebUISVGIsCompressed(t *testing.T) {
	h := newWebUIHandler(testWebUIFS())
	req := httptest.NewRequest(http.MethodGet, "/brand/hartex-mark.svg", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("svg Content-Encoding = %q, want gzip", got)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/svg+xml" {
		t.Fatalf("svg Content-Type = %q, want image/svg+xml", got)
	}
}

// TestWebUIRealDistIfPresent checks the actual embedded tree once it has been
// built. It skips in a checkout that has not run the frontend build.
func TestWebUIRealDistIfPresent(t *testing.T) {
	if _, err := os.Stat("webui/dist/index.html"); err != nil {
		t.Skip("embedded dist not built")
	}
	h := newWebUIHandler(os.DirFS("webui/dist"))

	req := httptest.NewRequest(http.MethodGet, "/dashboard/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("dashboard status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("dashboard Content-Encoding = %q, want gzip", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("dashboard Cache-Control = %q, want no-cache", got)
	}

	zr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatalf("dashboard gzip reader: %v", err)
	}
	body, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("dashboard gzip body: %v", err)
	}
	chunk := regexp.MustCompile(`/_next/static/chunks/[^"]+\.js`).FindString(string(body))
	if chunk == "" {
		t.Skip("dashboard HTML has no script chunk")
	}
	req2 := httptest.NewRequest(http.MethodGet, chunk, nil)
	req2.Header.Set("Accept-Encoding", "gzip")
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("chunk status = %d, want 200", rec2.Code)
	}
	if got := rec2.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("chunk Content-Encoding = %q, want gzip", got)
	}
	if got := rec2.Header().Get("Cache-Control"); !strings.Contains(got, "immutable") {
		t.Fatalf("chunk Cache-Control = %q, want immutable", got)
	}
}
