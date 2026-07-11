package vtest_test

import (
	"net/http"
	"testing"

	"vento-app/app/controllers"
	"vento-app/app/models"
	"vento-app/vento/vtest"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("opening in-memory sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}); err != nil {
		t.Fatalf("migrating models.User: %v", err)
	}
	return db
}

func TestUserCreateEndToEnd(t *testing.T) {
	db := newTestDB(t)
	c, rec := vtest.NewContext(http.MethodPost, "/api/users", controllers.UserForm{
		Name:  "Ann",
		Email: "ann@example.com",
	}, nil)
	c.SetDB(db)

	controllers.UserCreate(c)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body)
	}
	got, err := vtest.DecodeJSON[models.User](rec)
	if err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if got.Name != "Ann" || got.Email != "ann@example.com" {
		t.Fatalf("unexpected response body: %+v", got)
	}
}

func TestUserCreateValidationFailure(t *testing.T) {
	db := newTestDB(t)
	c, rec := vtest.NewContext(http.MethodPost, "/api/users", controllers.UserForm{
		Name:  "A", // too short: validate:"min=2"
		Email: "not-an-email",
	}, nil)
	c.SetDB(db)

	controllers.UserCreate(c)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body)
	}
}

func TestUserShowWithRouteParam(t *testing.T) {
	db := newTestDB(t)
	db.Create(&models.User{Name: "Bob", Email: "bob@example.com"})

	c, rec := vtest.NewContext(http.MethodGet, "/api/users/1", nil, map[string]string{"id": "1"})
	c.SetDB(db)

	controllers.UserShow(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body)
	}
	got, err := vtest.DecodeJSON[models.User](rec)
	if err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if got.Name != "Bob" {
		t.Fatalf("expected Bob, got %+v", got)
	}
}

func TestUserShowNotFound(t *testing.T) {
	db := newTestDB(t)
	c, rec := vtest.NewContext(http.MethodGet, "/api/users/999", nil, map[string]string{"id": "999"})
	c.SetDB(db)

	controllers.UserShow(c)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body)
	}
}

func TestUserDeleteEndToEnd(t *testing.T) {
	db := newTestDB(t)
	db.Create(&models.User{Name: "Carol", Email: "carol@example.com"})

	c, rec := vtest.NewContext(http.MethodDelete, "/api/users/1", nil, map[string]string{"id": "1"})
	c.SetDB(db)

	controllers.UserDelete(c)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body)
	}

	var count int64
	db.Model(&models.User{}).Count(&count)
	if count != 0 {
		t.Fatal("expected the user to be deleted")
	}
}
