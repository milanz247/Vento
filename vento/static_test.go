package vento

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// newTestEngine returns a bare Engine with a working Context pool but none
// of DefaultMiddleware, so tests can exercise routing/static/dispatch
// plumbing without CSRF/rate-limiting side effects.
func newTestEngine() *Engine {
	e := &Engine{router: newRouter()}
	e.pool.New = func() any { return &Context{} }
	return e
}

func TestStaticServesFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "app.css"), []byte("body{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := newTestEngine()
	e.Static("/public", dir)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/public/app.css", nil)
	e.dispatch(rec, req, e.matchStatic("/public/app.css"), nil)

	if rec.Code != http.StatusOK || rec.Body.String() != "body{}" {
		t.Fatalf("expected file to be served, got status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestStaticRejectsDirectoryListing(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "assets")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "secret.txt"), []byte("shh"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := newTestEngine()
	e.Static("/public", dir)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/public/assets/", nil)
	e.dispatch(rec, req, e.matchStatic("/public/assets/"), nil)

	if rec.Code == http.StatusOK {
		t.Fatalf("expected directory listing to be blocked, got 200 body=%q", rec.Body.String())
	}
}

func TestStaticServesDirectoryIndex(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<h1>hi</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := newTestEngine()
	e.Static("/public", dir)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/public/", nil)
	e.dispatch(rec, req, e.matchStatic("/public/"), nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected index.html to be served, got status=%d", rec.Code)
	}
}
