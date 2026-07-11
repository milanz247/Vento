// These are end-to-end tests of the actual route wiring in api.go - a real
// *vento.Engine, DefaultMiddleware (including the real CSRF/rate-limit/
// session stack), and the real router, not individual handlers called
// directly. They exist specifically to verify, holistically, the fix for
// two audit findings: unauthenticated CRUD on /api/users (every mutating
// route was reachable with no auth check at all), and the CSRF exemption
// that blanket-trusted all of "/api" as token-authenticated when
// RequireAuth - the framework's only available auth primitive - is
// actually session-cookie-based. A route-level test is the only way to
// prove both middlewares are actually wired onto the real routes in the
// order that matters, not just that the underlying primitives work in
// isolation (which vento's own test suite and app/controllers' tests
// already cover).
package routes_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vento-app/app/models"
	"vento-app/routes"
	"vento-app/vento"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestApp(t *testing.T) *vento.Engine {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("opening in-memory sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}); err != nil {
		t.Fatalf("migrating models.User: %v", err)
	}

	app := vento.New() // DefaultMiddleware, including the real CSRF/rate-limit stack
	app.DB = db
	app.Use(vento.Sessions("test-secret"))
	routes.Api(app)
	return app
}

func do(app *vento.Engine, method, path, body string, cookies []*http.Cookie, headers map[string]string) *httptest.ResponseRecorder {
	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	} else {
		reader = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, ck := range cookies {
		req.AddCookie(ck)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	return rec
}

func findCookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == name {
			return ck
		}
	}
	return nil
}

func TestUnauthenticatedRequestsAreBlockedFromUserRoutes(t *testing.T) {
	app := newTestApp(t)

	// GET/HEAD are CSRF-safe methods, so an unauthenticated GET reaches
	// RequireAuth and is blocked there (401). POST/PUT/DELETE hit
	// CSRFProtection first - it's global middleware, RequireAuth is
	// per-group - and a request with no CSRF cookie at all is rejected
	// there instead (403), before RequireAuth ever runs. Either way the
	// request never reaches the controller; that's what this test actually
	// asserts, not which specific middleware happened to reject it first.
	cases := []struct {
		method   string
		path     string
		wantCode int
	}{
		{http.MethodGet, "/api/users", http.StatusUnauthorized},
		{http.MethodGet, "/api/users/1", http.StatusUnauthorized},
		{http.MethodPost, "/api/users", http.StatusForbidden},
		{http.MethodPut, "/api/users/1", http.StatusForbidden},
		{http.MethodDelete, "/api/users/1", http.StatusForbidden},
	}
	for _, tc := range cases {
		rec := do(app, tc.method, tc.path, "", nil, nil)
		if rec.Code != tc.wantCode {
			t.Errorf("%s %s: expected %d, got %d: %s", tc.method, tc.path, tc.wantCode, rec.Code, rec.Body)
		}
	}
}

// TestUnauthenticatedMutationCannotSuceedEvenWithACSRFToken closes the gap
// TestUnauthenticatedRequestsAreBlockedFromUserRoutes leaves open: a 403
// from a missing CSRF token doesn't by itself prove RequireAuth is wired
// on too. This gets a real CSRF token first (from an anonymous GET, which
// safe methods always issue one for), then proves a mutating request still
// fails - now on authentication, not CSRF - even though the CSRF check
// passes.
func TestUnauthenticatedMutationCannotSucceedEvenWithACSRFToken(t *testing.T) {
	app := newTestApp(t)

	rec := do(app, http.MethodGet, "/api/users", "", nil, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected the anonymous GET itself to be unauthorized, got %d", rec.Code)
	}
	csrf := findCookie(rec, vento.CSRFCookieName)
	if csrf == nil {
		t.Fatal("expected a CSRF cookie to be issued even on a rejected GET (SecurityHeaders/CSRF run before RequireAuth)")
	}

	rec = do(app, http.MethodPost, "/api/users", `{"name":"Eve","email":"eve@example.com"}`,
		[]*http.Cookie{csrf}, map[string]string{vento.CSRFHeaderName: csrf.Value})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401: a valid CSRF token must not substitute for authentication, got %d: %s", rec.Code, rec.Body)
	}
}

func TestRegisterLoginAndAccessUserRoutes(t *testing.T) {
	app := newTestApp(t)

	// Register - /api/auth is CSRF-exempt (no session exists yet to protect).
	registerBody := `{"name":"Ann","email":"ann@example.com","password":"correct-horse-battery-staple"}`
	rec := do(app, http.MethodPost, "/api/auth/register", registerBody, nil, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d: %s", rec.Code, rec.Body)
	}
	session := findCookie(rec, vento.SessionCookieName)
	if session == nil {
		t.Fatal("expected a session cookie after registering")
	}

	// The now-authenticated session can read /api/users.
	rec = do(app, http.MethodGet, "/api/users", "", []*http.Cookie{session}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("authenticated GET /api/users: expected 200, got %d: %s", rec.Code, rec.Body)
	}
}

func TestAuthenticatedMutationsRequireCSRFToken(t *testing.T) {
	// This is the H1 regression guard: /api/users now runs under
	// RequireAuth (session-cookie auth), so - unlike the old blanket "/api"
	// CSRF exemption - CSRF protection must actually apply to it. A POST
	// with a valid session but no CSRF token must still be rejected.
	app := newTestApp(t)

	registerBody := `{"name":"Ann","email":"ann@example.com","password":"correct-horse-battery-staple"}`
	rec := do(app, http.MethodPost, "/api/auth/register", registerBody, nil, nil)
	session := findCookie(rec, vento.SessionCookieName)
	if session == nil {
		t.Fatal("expected a session cookie after registering")
	}

	// Get a CSRF cookie via a safe (GET) request on the now-authenticated session.
	rec = do(app, http.MethodGet, "/api/users", "", []*http.Cookie{session}, nil)
	csrf := findCookie(rec, vento.CSRFCookieName)
	if csrf == nil {
		t.Fatal("expected a CSRF cookie to be issued on the authenticated GET")
	}

	createBody := `{"name":"Bob","email":"bob@example.com"}`

	// Authenticated, but no CSRF token: must be rejected.
	rec = do(app, http.MethodPost, "/api/users", createBody, []*http.Cookie{session, csrf}, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("authenticated POST without a CSRF token: expected 403, got %d: %s", rec.Code, rec.Body)
	}

	// Authenticated, with a valid CSRF token: must succeed.
	rec = do(app, http.MethodPost, "/api/users", createBody, []*http.Cookie{session, csrf}, map[string]string{
		vento.CSRFHeaderName: csrf.Value,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("authenticated POST with a valid CSRF token: expected 201, got %d: %s", rec.Code, rec.Body)
	}
}

func TestWrongCredentialsDoNotAuthenticate(t *testing.T) {
	app := newTestApp(t)

	registerBody := `{"name":"Ann","email":"ann@example.com","password":"correct-horse-battery-staple"}`
	do(app, http.MethodPost, "/api/auth/register", registerBody, nil, nil)

	loginBody := `{"email":"ann@example.com","password":"wrong-password"}`
	rec := do(app, http.MethodPost, "/api/auth/login", loginBody, nil, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong password, got %d: %s", rec.Code, rec.Body)
	}
	session := findCookie(rec, vento.SessionCookieName)

	// Even if a session cookie happened to be issued, it must not be
	// authenticated - verify by trying to reach a RequireAuth route with it.
	var cookies []*http.Cookie
	if session != nil {
		cookies = []*http.Cookie{session}
	}
	rec = do(app, http.MethodGet, "/api/users", "", cookies, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected a failed login to never grant access to /api/users, got %d: %s", rec.Code, rec.Body)
	}
}

func TestHealthEndpointStillPublic(t *testing.T) {
	app := newTestApp(t)
	rec := do(app, http.MethodGet, "/api/health", "", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected /api/health to remain public, got %d: %s", rec.Code, rec.Body)
	}
}
