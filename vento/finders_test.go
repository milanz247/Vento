package vento

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// testModel is a minimal GORM model for exercising FindOrNotFound against a
// real (in-memory sqlite) database, independent of the app's own models.
type testModel struct {
	ID   uint `gorm:"primaryKey"`
	Name string
}

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("opening in-memory sqlite: %v", err)
	}
	if err := db.AutoMigrate(&testModel{}); err != nil {
		t.Fatalf("migrating testModel: %v", err)
	}
	return db
}

func TestFindOrNotFoundReturnsExistingRecord(t *testing.T) {
	db := newTestDB(t)
	db.Create(&testModel{ID: 1, Name: "alice"})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := &Context{index: -1}
	c.Reset(rec, req)
	c.db = db

	var got testModel
	ok := FindOrNotFound(c, &got, 1)

	if !ok {
		t.Fatal("expected FindOrNotFound to succeed for an existing record")
	}
	if got.Name != "alice" {
		t.Fatalf("expected the record to be loaded into dest, got %+v", got)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected no response written on success, got status %d", rec.Code)
	}
}

func TestFindOrNotFoundWrites404ForMissingRecord(t *testing.T) {
	db := newTestDB(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := &Context{index: -1}
	c.Reset(rec, req)
	c.db = db
	c.handlers = []HandlerFunc{func(*Context) {}} // Abort needs a chain to truncate

	var got testModel
	ok := FindOrNotFound(c, &got, 999)

	if ok {
		t.Fatal("expected FindOrNotFound to fail for a missing record")
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
