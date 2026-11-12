package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestFrontendNotBuilt(t *testing.T) {
	rec := get(frontend(fstest.MapFS{".gitkeep": {}}), "/")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d, want 503", rec.Code)
	}
}

func TestFrontendServesBuild(t *testing.T) {
	h := frontend(fstest.MapFS{
		"index.html":      {Data: []byte("<html>app</html>")},
		"assets/app-1.js": {Data: []byte("console.log(1)")},
	})
	if rec := get(h, "/"); rec.Code != 200 || rec.Header().Get("Cache-Control") != "no-cache" {
		t.Errorf("index: %d %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
	rec := get(h, "/assets/app-1.js")
	if rec.Code != 200 || rec.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Errorf("asset: %d %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
	if rec := get(h, "/assets/missing.js"); rec.Code != 404 {
		t.Errorf("missing asset: %d", rec.Code)
	}
}
