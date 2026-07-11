package vento

import (
	"net/http"
	"testing"
)

func TestSetCookieUsesSecureDefaults(t *testing.T) {
	c, rec := newTestContext(http.MethodGet, "/")
	c.SetCookie("pref", "dark", 3600)

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected exactly one cookie, got %d", len(cookies))
	}
	ck := cookies[0]
	if ck.Name != "pref" || ck.Value != "dark" {
		t.Fatalf("unexpected cookie: %+v", ck)
	}
	if !ck.HttpOnly {
		t.Error("expected HttpOnly by default")
	}
	if ck.SameSite != http.SameSiteLaxMode {
		t.Error("expected SameSite=Lax by default")
	}
	if ck.Path != "/" {
		t.Errorf("expected Path=/, got %q", ck.Path)
	}
	if ck.MaxAge != 3600 {
		t.Errorf("expected MaxAge=3600, got %d", ck.MaxAge)
	}
}

func TestCookieReadsValue(t *testing.T) {
	c, _ := newTestContext(http.MethodGet, "/")
	c.Request.AddCookie(&http.Cookie{Name: "sid", Value: "abc"})

	v, err := c.Cookie("sid")
	if err != nil || v != "abc" {
		t.Fatalf("expected to read sid=abc, got %q (err=%v)", v, err)
	}

	if _, err := c.Cookie("missing"); err == nil {
		t.Fatal("expected an error reading a missing cookie")
	}
}

func TestClearCookieExpiresIt(t *testing.T) {
	c, rec := newTestContext(http.MethodGet, "/")
	c.ClearCookie("sid")

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].MaxAge >= 0 {
		t.Fatalf("expected a single expired cookie (MaxAge<0), got %+v", cookies)
	}
}
