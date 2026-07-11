package controllers_test

import (
	"net/http"
	"strings"
	"testing"

	"vento-app/app/controllers"
	"vento-app/app/models"
	"vento-app/vento/hash"
	"vento-app/vento/vtest"
)

func TestAuthRegisterCreatesUserAndLogsIn(t *testing.T) {
	db := newTestDB(t)
	c, rec := vtest.NewContext(http.MethodPost, "/api/auth/register", controllers.RegisterForm{
		Name:     "Ann",
		Email:    "ann@example.com",
		Password: "correct-horse-battery-staple",
	}, nil)
	c.SetDB(db)

	controllers.AuthRegister(c)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body)
	}
	if !c.Authenticated() {
		t.Fatal("expected AuthRegister to log the new user in")
	}

	got, err := vtest.DecodeJSON[models.User](rec)
	if err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if got.Name != "Ann" || got.Email != "ann@example.com" {
		t.Fatalf("unexpected response body: %+v", got)
	}
}

func TestAuthRegisterResponseNeverIncludesPasswordHash(t *testing.T) {
	db := newTestDB(t)
	c, rec := vtest.NewContext(http.MethodPost, "/api/auth/register", controllers.RegisterForm{
		Name:     "Ann",
		Email:    "ann@example.com",
		Password: "correct-horse-battery-staple",
	}, nil)
	c.SetDB(db)

	controllers.AuthRegister(c)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body)
	}
	// The struct decodes fine even with json:"-" (PasswordHash is simply
	// absent), so assert directly against the raw JSON text instead - this
	// is the test that actually fails if the json:"-" tag is ever removed
	// from models.User.PasswordHash.
	got := rec.Body.String()
	if strings.Contains(got, "correct-horse-battery-staple") || strings.Contains(got, "PasswordHash") || strings.Contains(got, "password_hash") {
		t.Fatalf("response body leaks the password/hash: %s", got)
	}
}

func TestAuthRegisterRejectsDuplicateEmail(t *testing.T) {
	db := newTestDB(t)
	db.Create(&models.User{Name: "Existing", Email: "taken@example.com", PasswordHash: "x"})

	c, rec := vtest.NewContext(http.MethodPost, "/api/auth/register", controllers.RegisterForm{
		Name:     "New",
		Email:    "taken@example.com",
		Password: "correct-horse-battery-staple",
	}, nil)
	c.SetDB(db)

	controllers.AuthRegister(c)

	// The unique-index violation itself is real (this test uses the same
	// sqlite database Email's gorm:"uniqueIndex" tag creates it against) -
	// only the resulting status code differs from production. GORM's MySQL
	// driver and its sqlite driver surface a duplicate-key violation as
	// different error types; CreateOrAbort's 409 classification
	// (isDuplicateKeyError, see vento/query.go) recognizes only the
	// MySQL shape, which is what production actually runs - that specific
	// classification is already unit-tested directly against a real
	// *mysql.MySQLError in vento/query_test.go. Here, on sqlite, it falls
	// through to the generic 500 path - what matters for this test is that
	// the duplicate is rejected at all, not silently accepted as a second
	// account.
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected the duplicate email to be rejected (500 on sqlite; 409 on MySQL - see comment above), got %d: %s", rec.Code, rec.Body)
	}

	var count int64
	db.Model(&models.User{}).Where("email = ?", "taken@example.com").Count(&count)
	if count != 1 {
		t.Fatalf("expected exactly one user with this email after the rejected duplicate, got %d", count)
	}
}

func TestAuthRegisterRejectsShortPassword(t *testing.T) {
	db := newTestDB(t)
	c, rec := vtest.NewContext(http.MethodPost, "/api/auth/register", controllers.RegisterForm{
		Name:     "Ann",
		Email:    "ann@example.com",
		Password: "short",
	}, nil)
	c.SetDB(db)

	controllers.AuthRegister(c)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for a too-short password, got %d: %s", rec.Code, rec.Body)
	}
	if c.Authenticated() {
		t.Fatal("expected a rejected registration to not log anyone in")
	}
}

func TestAuthLoginSucceedsWithCorrectCredentials(t *testing.T) {
	db := newTestDB(t)
	hashed, err := hash.Make("correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("hashing password: %v", err)
	}
	db.Create(&models.User{Name: "Ann", Email: "ann@example.com", PasswordHash: hashed})

	c, rec := vtest.NewContext(http.MethodPost, "/api/auth/login", controllers.LoginForm{
		Email:    "ann@example.com",
		Password: "correct-horse-battery-staple",
	}, nil)
	c.SetDB(db)

	controllers.AuthLogin(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body)
	}
	if !c.Authenticated() {
		t.Fatal("expected AuthLogin to authenticate the session")
	}
}

func TestAuthLoginRejectsWrongPassword(t *testing.T) {
	db := newTestDB(t)
	hashed, _ := hash.Make("correct-horse-battery-staple")
	db.Create(&models.User{Name: "Ann", Email: "ann@example.com", PasswordHash: hashed})

	c, rec := vtest.NewContext(http.MethodPost, "/api/auth/login", controllers.LoginForm{
		Email:    "ann@example.com",
		Password: "wrong-password",
	}, nil)
	c.SetDB(db)

	controllers.AuthLogin(c)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body)
	}
	if c.Authenticated() {
		t.Fatal("expected a failed login to not authenticate the session")
	}
}

func TestAuthLoginRejectsUnknownEmailWithSameStatusAsWrongPassword(t *testing.T) {
	// Regression guard: a login endpoint that returns a different error for
	// "no such account" vs "wrong password" lets an attacker enumerate
	// registered emails one guess at a time. Both must produce the same
	// response.
	db := newTestDB(t)
	c, rec := vtest.NewContext(http.MethodPost, "/api/auth/login", controllers.LoginForm{
		Email:    "nobody@example.com",
		Password: "whatever12345",
	}, nil)
	c.SetDB(db)

	controllers.AuthLogin(c)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for an unknown email, got %d: %s", rec.Code, rec.Body)
	}
}

func TestAuthLogoutClearsAuthentication(t *testing.T) {
	db := newTestDB(t)
	c, rec := vtest.NewContext(http.MethodPost, "/api/auth/logout", nil, nil)
	c.SetDB(db)
	c.Login(uint(1))

	controllers.AuthLogout(c)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if c.Authenticated() {
		t.Fatal("expected AuthLogout to clear authentication")
	}
}

func TestAuthMeRequiresLogin(t *testing.T) {
	db := newTestDB(t)
	c, rec := vtest.NewContext(http.MethodGet, "/api/auth/me", nil, nil)
	c.SetDB(db)

	controllers.AuthMe(c)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when not logged in, got %d: %s", rec.Code, rec.Body)
	}
}

func TestAuthMeReturnsCurrentUser(t *testing.T) {
	db := newTestDB(t)
	user := models.User{Name: "Ann", Email: "ann@example.com", PasswordHash: "x"}
	db.Create(&user)

	c, rec := vtest.NewContext(http.MethodGet, "/api/auth/me", nil, nil)
	c.SetDB(db)
	c.Login(user.ID)

	controllers.AuthMe(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body)
	}
	got, err := vtest.DecodeJSON[models.User](rec)
	if err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if got.Email != "ann@example.com" {
		t.Fatalf("expected the logged-in user's own record, got %+v", got)
	}
}
