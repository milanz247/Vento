package vento

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoginAndAuthenticated(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := &Context{index: -1}
	c.Reset(rec, req)

	if c.Authenticated() {
		t.Fatal("expected a fresh context to not be authenticated")
	}

	c.Login(uint(7))
	if !c.Authenticated() {
		t.Fatal("expected Authenticated to be true after Login")
	}
	id, ok := c.AuthID()
	if !ok || id != "7" {
		t.Fatalf("expected AuthID to return \"7\", got %q (ok=%v)", id, ok)
	}
}

func TestLogoutClearsAuthentication(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := &Context{index: -1}
	c.Reset(rec, req)

	c.Login(uint(7))
	c.Logout()

	if c.Authenticated() {
		t.Fatal("expected Authenticated to be false after Logout")
	}
}

func TestLoginPersistsAcrossRequestsViaSessionCookie(t *testing.T) {
	mw := Sessions("test-secret")

	rec1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodPost, "/login", nil)
	c1 := &Context{index: -1}
	c1.Reset(rec1, req1)
	c1.handlers = []HandlerFunc{mw, func(c *Context) {
		c.Login(uint(42))
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
		t.Fatal("expected a session cookie after login")
	}

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/me", nil)
	req2.AddCookie(sessionCookie)

	var authenticated bool
	var id string
	c2 := &Context{index: -1}
	c2.Reset(rec2, req2)
	c2.handlers = []HandlerFunc{mw, func(c *Context) {
		authenticated = c.Authenticated()
		id, _ = c.AuthID()
	}}
	c2.Next()

	if !authenticated || id != "42" {
		t.Fatalf("expected login to persist across requests: authenticated=%v id=%q", authenticated, id)
	}
}

func TestCurrentUserLoadsAuthenticatedModel(t *testing.T) {
	db := newTestDB(t)
	db.Create(&testModel{ID: 5, Name: "bob"})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := &Context{index: -1}
	c.Reset(rec, req)
	c.db = db
	c.Login(uint(5))

	user, ok := CurrentUser[testModel](c)
	if !ok {
		t.Fatal("expected CurrentUser to succeed for a logged-in, existing user")
	}
	if user.Name != "bob" {
		t.Fatalf("expected the loaded user's Name to be 'bob', got %q", user.Name)
	}
}

func TestCurrentUserFalseWhenNotLoggedIn(t *testing.T) {
	db := newTestDB(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := &Context{index: -1}
	c.Reset(rec, req)
	c.db = db

	if _, ok := CurrentUser[testModel](c); ok {
		t.Fatal("expected CurrentUser to fail when nobody is logged in")
	}
}

func TestCurrentUserFalseWhenAccountDeleted(t *testing.T) {
	db := newTestDB(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := &Context{index: -1}
	c.Reset(rec, req)
	c.db = db
	c.Login(uint(999)) // never created

	if _, ok := CurrentUser[testModel](c); ok {
		t.Fatal("expected CurrentUser to fail when the session's user ID no longer resolves")
	}
}

func TestRequireAuthBlocksUnauthenticated(t *testing.T) {
	ran := false
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := &Context{index: -1}
	c.Reset(rec, req)
	c.handlers = []HandlerFunc{RequireAuth, func(c *Context) { ran = true }}
	c.Next()

	if ran {
		t.Fatal("expected RequireAuth to block an unauthenticated request")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestRequireAuthAllowsAuthenticated(t *testing.T) {
	mw := Sessions("test-secret")
	ran := false
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := &Context{index: -1}
	c.Reset(rec, req)
	c.handlers = []HandlerFunc{
		mw,
		func(c *Context) { c.Login(uint(1)); c.Next() },
		RequireAuth,
		func(c *Context) { ran = true; c.String(http.StatusOK, "ok") },
	}
	c.Next()

	if !ran {
		t.Fatal("expected RequireAuth to allow an authenticated request through")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
