package vento

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeadersBaseline(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	runChain(rec, req, []HandlerFunc{SecurityHeaders, func(c *Context) { c.String(http.StatusOK, "ok") }})

	h := rec.Header()
	if h.Get("X-Frame-Options") != "DENY" {
		t.Errorf("expected X-Frame-Options: DENY, got %q", h.Get("X-Frame-Options"))
	}
	if h.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("expected X-Content-Type-Options: nosniff, got %q", h.Get("X-Content-Type-Options"))
	}
}

func TestSecurityHeadersHSTSOnlyWhenSecure(t *testing.T) {
	orig := TrustProxyHeaders
	defer func() { TrustProxyHeaders = orig }()
	TrustProxyHeaders = false

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	runChain(rec, req, []HandlerFunc{SecurityHeaders, func(c *Context) { c.String(http.StatusOK, "ok") }})

	if rec.Header().Get("Strict-Transport-Security") != "" {
		t.Fatal("expected no HSTS header on a plaintext request")
	}

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.TLS = &tls.ConnectionState{}
	runChain(rec2, req2, []HandlerFunc{SecurityHeaders, func(c *Context) { c.String(http.StatusOK, "ok") }})

	if rec2.Header().Get("Strict-Transport-Security") == "" {
		t.Fatal("expected HSTS header on a TLS request")
	}
}

func TestSecurityHeadersCSPOnlyWhenConfigured(t *testing.T) {
	orig := ContentSecurityPolicy
	defer func() { ContentSecurityPolicy = orig }()

	ContentSecurityPolicy = ""
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	runChain(rec, req, []HandlerFunc{SecurityHeaders, func(c *Context) { c.String(http.StatusOK, "ok") }})
	if rec.Header().Get("Content-Security-Policy") != "" {
		t.Fatal("expected no CSP header when ContentSecurityPolicy is unset")
	}

	ContentSecurityPolicy = "default-src 'self'"
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	runChain(rec2, req2, []HandlerFunc{SecurityHeaders, func(c *Context) { c.String(http.StatusOK, "ok") }})
	if rec2.Header().Get("Content-Security-Policy") != "default-src 'self'" {
		t.Fatalf("expected configured CSP to be set, got %q", rec2.Header().Get("Content-Security-Policy"))
	}
}

func TestContextDetachIsIndependentOfPool(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)

	c := &Context{index: -1}
	c.Reset(rec, req)
	c.params = map[string]string{"id": "42"}

	d := c.Detach()
	if d.Method != http.MethodGet || d.Path != "/users/42" || d.Params["id"] != "42" {
		t.Fatalf("unexpected detached snapshot: %+v", d)
	}

	// Mutating the live context's params afterward must not affect the
	// already-taken snapshot.
	c.params["id"] = "99"
	if d.Params["id"] != "42" {
		t.Fatal("expected Detach to take an independent copy of params")
	}

	if d.Context().Err() != nil {
		t.Fatal("expected detached context to not be pre-canceled")
	}
}
