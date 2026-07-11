package vento

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSessionRoundTrip(t *testing.T) {
	mw := Sessions("test-secret")

	// Request 1: set a value, expect a Set-Cookie in the response.
	rec1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	c1 := &Context{index: -1}
	c1.Reset(rec1, req1)
	c1.handlers = []HandlerFunc{mw, func(c *Context) {
		c.Session().Set("user_id", "123")
		c.String(http.StatusOK, "ok")
	}}
	c1.Next()

	cookies := rec1.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, ck := range cookies {
		if ck.Name == SessionCookieName {
			sessionCookie = ck
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected a session cookie to be set")
	}

	// Request 2: replay the cookie, expect the value back.
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.AddCookie(sessionCookie)

	var got any
	var ok bool
	c2 := &Context{index: -1}
	c2.Reset(rec2, req2)
	c2.handlers = []HandlerFunc{mw, func(c *Context) {
		got, ok = c.Session().Get("user_id")
	}}
	c2.Next()

	if !ok || got != "123" {
		t.Fatalf("expected user_id=123 to round-trip, got %v (ok=%v)", got, ok)
	}
}

func TestSessionTamperedCookieIsRejected(t *testing.T) {
	mw := Sessions("test-secret")

	rec1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	c1 := &Context{index: -1}
	c1.Reset(rec1, req1)
	c1.handlers = []HandlerFunc{mw, func(c *Context) {
		c.Session().Set("user_id", "123")
		c.String(http.StatusOK, "ok")
	}}
	c1.Next()

	var sessionCookie *http.Cookie
	for _, ck := range rec1.Result().Cookies() {
		if ck.Name == SessionCookieName {
			sessionCookie = ck
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected a session cookie to be set")
	}

	// Flip a character in the signed value - simulating tampering.
	tampered := *sessionCookie
	tampered.Value = tampered.Value[:len(tampered.Value)-1] + "x"

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.AddCookie(&tampered)

	var ok bool
	c2 := &Context{index: -1}
	c2.Reset(rec2, req2)
	c2.handlers = []HandlerFunc{mw, func(c *Context) {
		_, ok = c.Session().Get("user_id")
	}}
	c2.Next()

	if ok {
		t.Fatal("expected a tampered session cookie to be rejected, not read back")
	}
}

func TestSessionWrongSecretIsRejected(t *testing.T) {
	rec1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	c1 := &Context{index: -1}
	c1.Reset(rec1, req1)
	c1.handlers = []HandlerFunc{Sessions("secret-a"), func(c *Context) {
		c.Session().Set("user_id", "123")
		c.String(http.StatusOK, "ok")
	}}
	c1.Next()

	var sessionCookie *http.Cookie
	for _, ck := range rec1.Result().Cookies() {
		if ck.Name == SessionCookieName {
			sessionCookie = ck
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected a session cookie to be set")
	}

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.AddCookie(sessionCookie)

	var ok bool
	c2 := &Context{index: -1}
	c2.Reset(rec2, req2)
	c2.handlers = []HandlerFunc{Sessions("secret-b"), func(c *Context) {
		_, ok = c.Session().Get("user_id")
	}}
	c2.Next()

	if ok {
		t.Fatal("expected a cookie signed with a different secret to be rejected")
	}
}

func TestSessionCookieSecureFlag(t *testing.T) {
	orig := TrustProxyHeaders
	defer func() { TrustProxyHeaders = orig }()
	TrustProxyHeaders = false

	mw := Sessions("test-secret")

	newReq := func(forwardedProto string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		if forwardedProto != "" {
			r.Header.Set("X-Forwarded-Proto", forwardedProto)
		}
		return r
	}

	run := func(r *http.Request) *http.Cookie {
		rec := httptest.NewRecorder()
		c := &Context{index: -1}
		c.Reset(rec, r)
		c.handlers = []HandlerFunc{mw, func(c *Context) {
			c.Session().Set("k", "v")
			c.String(http.StatusOK, "ok")
		}}
		c.Next()
		for _, ck := range rec.Result().Cookies() {
			if ck.Name == SessionCookieName {
				return ck
			}
		}
		return nil
	}

	if ck := run(newReq("https")); ck == nil || ck.Secure {
		t.Fatal("expected Secure=false: X-Forwarded-Proto must be ignored when TrustProxyHeaders is disabled")
	}

	TrustProxyHeaders = true
	if ck := run(newReq("https")); ck == nil || !ck.Secure {
		t.Fatal("expected Secure=true once TrustProxyHeaders is enabled and X-Forwarded-Proto=https")
	}
	if ck := run(newReq("http")); ck == nil || ck.Secure {
		t.Fatal("expected Secure=false when the forwarded proto isn't https")
	}
}

func TestContextSessionWithoutMiddlewareDoesNotPersist(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := &Context{index: -1}
	c.Reset(rec, req)
	c.handlers = []HandlerFunc{func(c *Context) {
		c.Session().Set("k", "v") // no Sessions middleware installed
		c.String(http.StatusOK, "ok")
	}}
	c.Next()

	for _, ck := range rec.Result().Cookies() {
		if ck.Name == SessionCookieName {
			t.Fatal("expected no session cookie when Sessions middleware isn't installed")
		}
	}
}
