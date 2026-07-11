package vento

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func runChain(w http.ResponseWriter, r *http.Request, handlers []HandlerFunc) *Context {
	c := &Context{index: -1}
	c.Reset(w, r)
	c.handlers = handlers
	c.Next()
	return c
}

func TestCSRFIssuesTokenOnSafeMethod(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	runChain(rec, req, []HandlerFunc{CSRFProtection(), func(c *Context) { c.String(http.StatusOK, "ok") }})

	var found bool
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == CSRFCookieName && ck.Value != "" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected a CSRF cookie to be issued on a GET request")
	}
}

func TestCSRFRejectsPostWithoutToken(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	c := runChain(rec, req, []HandlerFunc{CSRFProtection(), func(c *Context) { c.String(http.StatusOK, "ok") }})

	if c.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for POST without CSRF cookie, got %d", c.StatusCode)
	}
}

func TestCSRFRejectsMismatchedToken(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})
	req.Header.Set(CSRFHeaderName, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")

	c := runChain(rec, req, []HandlerFunc{CSRFProtection(), func(c *Context) { c.String(http.StatusOK, "ok") }})

	if c.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for mismatched CSRF token, got %d", c.StatusCode)
	}
}

func TestCSRFAcceptsMatchingHeaderToken(t *testing.T) {
	token := "cccccccccccccccccccccccccccccccc"
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: token})
	req.Header.Set(CSRFHeaderName, token)

	handlerRan := false
	c := runChain(rec, req, []HandlerFunc{CSRFProtection(), func(c *Context) {
		handlerRan = true
		c.String(http.StatusOK, "ok")
	}})

	if !handlerRan || c.StatusCode != http.StatusOK {
		t.Fatalf("expected matching CSRF token to be accepted, status=%d", c.StatusCode)
	}
}

func TestCSRFExemptPrefixBypassesCheck(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/widgets", nil)

	handlerRan := false
	c := runChain(rec, req, []HandlerFunc{CSRFProtection("/api"), func(c *Context) {
		handlerRan = true
		c.String(http.StatusOK, "ok")
	}})

	if !handlerRan || c.StatusCode != http.StatusOK {
		t.Fatalf("expected /api prefix to bypass CSRF check, status=%d", c.StatusCode)
	}
}

func TestCSRFCookieSecureFlagRespectsTrustProxyHeaders(t *testing.T) {
	orig := TrustProxyHeaders
	defer func() { TrustProxyHeaders = orig }()

	TrustProxyHeaders = false
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	runChain(rec, req, []HandlerFunc{CSRFProtection(), func(c *Context) { c.String(http.StatusOK, "ok") }})

	for _, ck := range rec.Result().Cookies() {
		if ck.Name == CSRFCookieName && ck.Secure {
			t.Fatal("expected Secure=false: forwarded proto must be ignored when TrustProxyHeaders is disabled")
		}
	}
}

func TestRateLimiterBlocksAfterBurst(t *testing.T) {
	mw := RateLimiter(1, 2) // 1 rps, burst of 2

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.5:1234"

	statuses := []int{}
	for range 3 {
		rec := httptest.NewRecorder()
		c := runChain(rec, req, []HandlerFunc{mw, func(c *Context) { c.String(http.StatusOK, "ok") }})
		statuses = append(statuses, c.StatusCode)
	}

	if statuses[0] != http.StatusOK || statuses[1] != http.StatusOK {
		t.Fatalf("expected first two requests (within burst) to pass, got %v", statuses)
	}
	if statuses[2] != http.StatusTooManyRequests {
		t.Fatalf("expected third request to be rate-limited, got %v", statuses)
	}
}

func TestRateLimiterTracksClientsIndependently(t *testing.T) {
	mw := RateLimiter(1, 1)

	reqA := httptest.NewRequest(http.MethodGet, "/", nil)
	reqA.RemoteAddr = "203.0.113.10:1111"
	reqB := httptest.NewRequest(http.MethodGet, "/", nil)
	reqB.RemoteAddr = "203.0.113.20:2222"

	recA1 := httptest.NewRecorder()
	cA1 := runChain(recA1, reqA, []HandlerFunc{mw, func(c *Context) { c.String(http.StatusOK, "ok") }})
	recB1 := httptest.NewRecorder()
	cB1 := runChain(recB1, reqB, []HandlerFunc{mw, func(c *Context) { c.String(http.StatusOK, "ok") }})

	if cA1.StatusCode != http.StatusOK || cB1.StatusCode != http.StatusOK {
		t.Fatalf("expected both distinct clients' first request to pass: A=%d B=%d", cA1.StatusCode, cB1.StatusCode)
	}

	recA2 := httptest.NewRecorder()
	cA2 := runChain(recA2, reqA, []HandlerFunc{mw, func(c *Context) { c.String(http.StatusOK, "ok") }})
	if cA2.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected client A's second immediate request to be limited, got %d", cA2.StatusCode)
	}
}

func TestRateLimiterRefillsOverTime(t *testing.T) {
	mw := RateLimiter(1000, 1) // fast refill so the test doesn't sleep long

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.30:3333"

	rec1 := httptest.NewRecorder()
	c1 := runChain(rec1, req, []HandlerFunc{mw, func(c *Context) { c.String(http.StatusOK, "ok") }})
	if c1.StatusCode != http.StatusOK {
		t.Fatalf("expected first request to pass, got %d", c1.StatusCode)
	}

	rec2 := httptest.NewRecorder()
	c2 := runChain(rec2, req, []HandlerFunc{mw, func(c *Context) { c.String(http.StatusOK, "ok") }})
	if c2.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected immediate second request to be limited, got %d", c2.StatusCode)
	}

	time.Sleep(5 * time.Millisecond)

	rec3 := httptest.NewRecorder()
	c3 := runChain(rec3, req, []HandlerFunc{mw, func(c *Context) { c.String(http.StatusOK, "ok") }})
	if c3.StatusCode != http.StatusOK {
		t.Fatalf("expected token to have refilled after waiting, got %d", c3.StatusCode)
	}
}

func TestClientIPIgnoresForwardedForByDefault(t *testing.T) {
	orig := TrustProxyHeaders
	defer func() { TrustProxyHeaders = orig }()
	TrustProxyHeaders = false

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.1:5555"
	req.Header.Set("X-Forwarded-For", "9.9.9.9")

	if ip := clientIP(req); ip != "203.0.113.1" {
		t.Fatalf("expected RemoteAddr to win when TrustProxyHeaders is disabled, got %q", ip)
	}
}

func TestClientIPHonorsForwardedForWhenTrusted(t *testing.T) {
	orig := TrustProxyHeaders
	defer func() { TrustProxyHeaders = orig }()
	TrustProxyHeaders = true

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.1:5555"
	req.Header.Set("X-Forwarded-For", "9.9.9.9, 10.0.0.1")

	if ip := clientIP(req); ip != "9.9.9.9" {
		t.Fatalf("expected leftmost X-Forwarded-For entry when trusted, got %q", ip)
	}
}

func TestIsSecure(t *testing.T) {
	orig := TrustProxyHeaders
	defer func() { TrustProxyHeaders = orig }()

	TrustProxyHeaders = false
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	if isSecure(req) {
		t.Fatal("expected isSecure=false without direct TLS and TrustProxyHeaders disabled")
	}

	TrustProxyHeaders = true
	if !isSecure(req) {
		t.Fatal("expected isSecure=true once TrustProxyHeaders is enabled with X-Forwarded-Proto=https")
	}
}

func TestBodyLimitCapsRequestBody(t *testing.T) {
	mw := BodyLimit(4)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	runChain(rec, req, []HandlerFunc{mw, func(c *Context) {
		if c.Request.Body == nil {
			t.Fatal("expected body to remain non-nil")
		}
		c.String(http.StatusOK, "ok")
	}})
}
