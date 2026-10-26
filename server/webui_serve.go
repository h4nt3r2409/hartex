package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/klauspost/compress/gzip"
)

// webuiAsset is one embedded UI file, cached with its content hash and a
// pre-compressed gzip representation. The embedded tree is immutable for the
// life of the process, so caching raw + gzip bytes is safe and makes repeat
// requests a single write instead of another compression pass.
type webuiAsset struct {
	raw          []byte
	gz           []byte
	etag         string
	contentType  string
	cacheControl string
	compressible bool
}

type webuiServer struct {
	root   fs.FS
	assets sync.Map // map[string]*webuiAsset
}

// newWebUIHandler serves a statically exported Next.js tree from fsys. It is
// shared by the embedded build and the tests; the no-embed stub remains
// separate.
func newWebUIHandler(fsys fs.FS) http.Handler {
	s := &webuiServer{root: fsys}
	return http.HandlerFunc(s.serveHTTP)
}

func (s *webuiServer) serveHTTP(w http.ResponseWriter, r *http.Request) {
	p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if p == "" {
		p = "index.html"
	}
	// 1) exact file (assets: _next/*, favicon.ico, ...)
	// 2) route dir → <p>/index.html (trailingSlash export)
	// 3) <p>.html
	// 4) SPA fallback → index.html (client-side routing)
	for _, cand := range []string{p, p + "/index.html", p + ".html"} {
		if s.serveFile(w, r, cand) {
			return
		}
	}
	if !s.serveFile(w, r, "index.html") {
		http.NotFound(w, r)
	}
}

func (s *webuiServer) serveFile(w http.ResponseWriter, r *http.Request, name string) bool {
	a, ok := s.asset(name)
	if !ok {
		return false
	}

	w.Header().Set("ETag", a.etag)
	w.Header().Set("Cache-Control", a.cacheControl)
	if a.compressible {
		w.Header().Add("Vary", "Accept-Encoding")
	}
	if etagMatches(r.Header.Get("If-None-Match"), a.etag) {
		w.WriteHeader(http.StatusNotModified)
		return true
	}

	// Range requests are served from the raw representation; compressing a
	// partial response would make the byte range meaningless.
	if a.compressible && acceptsGzip(r) && r.Header.Get("Range") == "" {
		w.Header().Set("Content-Type", a.contentType)
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Length", strconv.Itoa(len(a.gz)))
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return true
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(a.gz)
		return true
	}

	w.Header().Set("Content-Type", a.contentType)
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(a.raw))
	return true
}

func (s *webuiServer) asset(name string) (*webuiAsset, bool) {
	if v, ok := s.assets.Load(name); ok {
		return v.(*webuiAsset), true
	}
	f, err := s.root.Open(name)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.IsDir() {
		return nil, false
	}
	raw, err := io.ReadAll(f)
	if err != nil {
		return nil, false
	}
	sum := sha256.Sum256(raw)
	a := &webuiAsset{
		raw:          raw,
		etag:         `W/"` + hex.EncodeToString(sum[:]) + `"`,
		contentType:  contentTypeFor(name),
		cacheControl: cacheControlFor(name),
	}
	a.compressible = isCompressible(a.contentType)
	if a.compressible {
		var buf bytes.Buffer
		zw, err := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
		if err == nil {
			_, _ = zw.Write(raw)
			_ = zw.Close()
			a.gz = buf.Bytes()
		}
	}
	actual, _ := s.assets.LoadOrStore(name, a)
	return actual.(*webuiAsset), true
}

var webuiContentTypes = map[string]string{
	".css":         "text/css; charset=utf-8",
	".html":        "text/html; charset=utf-8",
	".ico":         "image/x-icon",
	".js":          "text/javascript; charset=utf-8",
	".json":        "application/json; charset=utf-8",
	".map":         "application/json; charset=utf-8",
	".mjs":         "text/javascript; charset=utf-8",
	".png":         "image/png",
	".svg":         "image/svg+xml",
	".txt":         "text/plain; charset=utf-8",
	".webmanifest": "application/manifest+json",
	".woff":        "font/woff",
	".woff2":       "font/woff2",
}

func contentTypeFor(name string) string {
	ext := strings.ToLower(path.Ext(name))
	if ct, ok := webuiContentTypes[ext]; ok {
		return ct
	}
	if ct := mime.TypeByExtension(ext); ct != "" {
		return ct
	}
	return "application/octet-stream"
}

func isCompressible(contentType string) bool {
	switch strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0])) {
	case "text/html", "text/css", "text/javascript", "text/plain",
		"application/javascript", "application/json", "application/manifest+json",
		"application/xml", "text/xml", "image/svg+xml":
		return true
	}
	return false
}

func cacheControlFor(name string) string {
	name = strings.TrimPrefix(name, "/")
	if strings.HasPrefix(name, "_next/static/") {
		return "public, max-age=31536000, immutable"
	}
	switch strings.ToLower(path.Ext(name)) {
	case ".html":
		return "no-cache"
	case ".png", ".ico", ".svg", ".woff", ".woff2":
		return "public, max-age=86400"
	case ".webmanifest":
		return "public, max-age=3600"
	default:
		return "no-cache"
	}
}

func acceptsGzip(r *http.Request) bool {
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		fields := strings.Split(part, ";")
		if !strings.EqualFold(strings.TrimSpace(fields[0]), "gzip") {
			continue
		}
		for _, param := range fields[1:] {
			param = strings.TrimSpace(param)
			if strings.HasPrefix(param, "q=") && strings.TrimSpace(param[2:]) == "0" {
				return false
			}
		}
		return true
	}
	return false
}

func etagMatches(header, etag string) bool {
	header = strings.TrimSpace(header)
	if header == "" {
		return false
	}
	if header == "*" {
		return true
	}
	for _, part := range strings.Split(header, ",") {
		if strings.TrimSpace(part) == etag {
			return true
		}
	}
	return false
}
