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

func TestModelLoadsByRouteParam(t *testing.T) {
	db := newTestDB(t)
	db.Create(&testModel{ID: 3, Name: "carol"})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := &Context{index: -1}
	c.Reset(rec, req)
	c.db = db
	c.params = map[string]string{"id": "3"}

	got, ok := Model[testModel](c, "id")
	if !ok {
		t.Fatal("expected Model to succeed for an existing record")
	}
	if got.Name != "carol" {
		t.Fatalf("expected Name=carol, got %+v", got)
	}
}

func TestModelWrites404ForMissingRecord(t *testing.T) {
	db := newTestDB(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := &Context{index: -1}
	c.Reset(rec, req)
	c.db = db
	c.params = map[string]string{"id": "999"}
	c.handlers = []HandlerFunc{func(*Context) {}}

	if _, ok := Model[testModel](c, "id"); ok {
		t.Fatal("expected Model to fail for a missing record")
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestModelWrites400ForMissingRouteParam(t *testing.T) {
	db := newTestDB(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := &Context{index: -1}
	c.Reset(rec, req)
	c.db = db
	c.handlers = []HandlerFunc{func(*Context) {}}

	if _, ok := Model[testModel](c, "id"); ok {
		t.Fatal("expected Model to fail when the route param was never captured")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestModelAcceptsNonNumericIDAsNotFound(t *testing.T) {
	db := newTestDB(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := &Context{index: -1}
	c.Reset(rec, req)
	c.db = db
	c.params = map[string]string{"id": "not-a-number"}
	c.handlers = []HandlerFunc{func(*Context) {}}

	if _, ok := Model[testModel](c, "id"); ok {
		t.Fatal("expected a non-numeric id to fail")
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected a malformed id to 404 (not 400) per Model's documented behavior, got %d", rec.Code)
	}
}

// TestModelParameterizesRouteValue guards against a real bug this test
// suite caught once already: GORM's First(dest, id) inline-condition
// shortcut treats a non-numeric string id as a raw SQL WHERE fragment
// rather than a bound parameter, which - since Model feeds it a
// client-controlled route parameter directly - is a SQL injection vector.
// FindOrNotFound fixes this with an explicit "id = ?" condition; this test
// proves an injection-shaped route value is treated as inert data (a
// normal 404, no rows leaked, no SQL error) rather than executed.
func TestModelParameterizesRouteValue(t *testing.T) {
	db := newTestDB(t)
	db.Create(&testModel{ID: 1, Name: "alice"})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := &Context{index: -1}
	c.Reset(rec, req)
	c.db = db
	c.params = map[string]string{"id": "1 OR 1=1"}
	c.handlers = []HandlerFunc{func(*Context) {}}

	if _, ok := Model[testModel](c, "id"); ok {
		t.Fatal("expected an injection-shaped route value to be treated as inert data, not executed")
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 (no row matches the literal string \"1 OR 1=1\"), got %d - the condition may have been executed as SQL instead of bound as a parameter", rec.Code)
	}
}
