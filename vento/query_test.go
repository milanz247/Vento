package vento

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

func newQueryTestContext(t *testing.T, db *gorm.DB) (*Context, *httptest.ResponseRecorder) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := &Context{index: -1}
	c.Reset(rec, req)
	c.db = db
	c.handlers = []HandlerFunc{func(*Context) {}} // Abort needs a chain to truncate
	return c, rec
}

func TestQueryFindOrAbort(t *testing.T) {
	db := newTestDB(t)
	db.Create(&testModel{ID: 1, Name: "alice"})
	c, rec := newQueryTestContext(t, db)

	got, ok := Query[testModel](c).FindOrAbort(1)
	if !ok || got.Name != "alice" {
		t.Fatalf("expected to find alice, got %+v (ok=%v)", got, ok)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected no response written on success, got %d", rec.Code)
	}
}

func TestQueryFindOrAbortMissing(t *testing.T) {
	db := newTestDB(t)
	c, rec := newQueryTestContext(t, db)

	if _, ok := Query[testModel](c).FindOrAbort(999); ok {
		t.Fatal("expected FindOrAbort to fail for a missing record")
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestQueryAll(t *testing.T) {
	db := newTestDB(t)
	db.Create(&testModel{ID: 1, Name: "a"})
	db.Create(&testModel{ID: 2, Name: "b"})
	c, _ := newQueryTestContext(t, db)

	items, err := Query[testModel](c).All()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
}

func TestQueryPaginate(t *testing.T) {
	db := newTestDB(t)
	for i := 1; i <= 25; i++ {
		db.Create(&testModel{ID: uint(i), Name: "item"})
	}
	c, _ := newQueryTestContext(t, db)

	page, err := Query[testModel](c).Paginate(2, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(page.Data) != 10 {
		t.Fatalf("expected 10 rows on page 2, got %d", len(page.Data))
	}
	if page.Page != 2 || page.PerPage != 10 || page.Total != 25 || page.LastPage != 3 {
		t.Fatalf("unexpected page metadata: %+v", *page)
	}
}

func TestQueryPaginateClampsOutOfRangeInput(t *testing.T) {
	origDefault, origMax := DefaultPerPage, MaxPerPage
	defer func() { DefaultPerPage, MaxPerPage = origDefault, origMax }()
	DefaultPerPage, MaxPerPage = 20, 50

	db := newTestDB(t)
	db.Create(&testModel{ID: 1, Name: "a"})
	c, _ := newQueryTestContext(t, db)

	page, err := Query[testModel](c).Paginate(0, 1000) // page<1, perPage>MaxPerPage
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.Page != 1 || page.PerPage != 50 {
		t.Fatalf("expected clamped page=1 perPage=50, got page=%d perPage=%d", page.Page, page.PerPage)
	}
}

func TestQueryCreateOrAbort(t *testing.T) {
	db := newTestDB(t)
	c, rec := newQueryTestContext(t, db)

	m := &testModel{Name: "new"}
	if !Query[testModel](c).CreateOrAbort(m) {
		t.Fatalf("expected create to succeed, got status %d body %s", rec.Code, rec.Body)
	}
	if m.ID == 0 {
		t.Fatal("expected the primary key to be populated after create")
	}
}

func TestQueryCreateOrAbortDuplicateKeyConflict(t *testing.T) {
	db := newTestDB(t)
	if err := db.Exec("CREATE UNIQUE INDEX idx_name_unique ON test_models(name)").Error; err != nil {
		t.Fatalf("creating unique index: %v", err)
	}
	db.Create(&testModel{ID: 1, Name: "taken"})

	c, rec := newQueryTestContext(t, db)
	m := &testModel{Name: "taken"}
	if Query[testModel](c).CreateOrAbort(m) {
		t.Fatal("expected a duplicate-key create to fail")
	}
	// sqlite doesn't produce a MySQL error code, so isDuplicateKeyError
	// won't match here - this exercises the generic-500 fallback path
	// rather than the 409 path (that's covered by isDuplicateKeyError's
	// own unit test using a real *mysql.MySQLError).
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 (sqlite error, not classified as a MySQL duplicate key), got %d", rec.Code)
	}
}

func TestQuerySaveOrAbort(t *testing.T) {
	db := newTestDB(t)
	db.Create(&testModel{ID: 1, Name: "old"})
	c, rec := newQueryTestContext(t, db)

	m := &testModel{ID: 1, Name: "new"}
	if !Query[testModel](c).SaveOrAbort(m) {
		t.Fatalf("expected save to succeed, got status %d", rec.Code)
	}

	var reloaded testModel
	db.First(&reloaded, 1)
	if reloaded.Name != "new" {
		t.Fatalf("expected the row to be updated, got %+v", reloaded)
	}
}

func TestIsDuplicateKeyError(t *testing.T) {
	dup := &mysqldriver.MySQLError{Number: 1062, Message: "Duplicate entry"}
	if !isDuplicateKeyError(dup) {
		t.Fatal("expected a MySQL 1062 error to be recognized as a duplicate key")
	}
	if !isDuplicateKeyError(fmt.Errorf("wrapped: %w", dup)) {
		t.Fatal("expected isDuplicateKeyError to see through error wrapping via errors.As")
	}

	other := &mysqldriver.MySQLError{Number: 1451, Message: "foreign key constraint fails"}
	if isDuplicateKeyError(other) {
		t.Fatal("expected a non-1062 MySQL error to not be classified as a duplicate key")
	}
	if isDuplicateKeyError(errors.New("some other error")) {
		t.Fatal("expected a non-MySQL error to not be classified as a duplicate key")
	}
}

func TestClassifyWriteErrorMapsToStatusCodes(t *testing.T) {
	db := newTestDB(t)

	c, rec := newQueryTestContext(t, db)
	if !classifyWriteError(c, nil) {
		t.Fatal("expected nil error to report ok=true and write nothing")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected no response written for a nil error, got %d", rec.Code)
	}

	c2, rec2 := newQueryTestContext(t, db)
	if classifyWriteError(c2, gorm.ErrRecordNotFound) {
		t.Fatal("expected gorm.ErrRecordNotFound to report ok=false")
	}
	if rec2.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for ErrRecordNotFound, got %d", rec2.Code)
	}

	c3, rec3 := newQueryTestContext(t, db)
	dup := &mysqldriver.MySQLError{Number: 1062}
	if classifyWriteError(c3, dup) {
		t.Fatal("expected a duplicate-key error to report ok=false")
	}
	if rec3.Code != http.StatusConflict {
		t.Fatalf("expected 409 for a duplicate-key error, got %d", rec3.Code)
	}

	c4, rec4 := newQueryTestContext(t, db)
	if classifyWriteError(c4, errors.New("connection reset")) {
		t.Fatal("expected a generic error to report ok=false")
	}
	if rec4.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for a generic error, got %d", rec4.Code)
	}
}

func TestQueryDeleteOrAbort(t *testing.T) {
	db := newTestDB(t)
	db.Create(&testModel{ID: 1, Name: "gone"})
	c, rec := newQueryTestContext(t, db)

	m := &testModel{ID: 1}
	if !Query[testModel](c).DeleteOrAbort(m) {
		t.Fatalf("expected delete to succeed, got status %d", rec.Code)
	}

	var count int64
	db.Model(&testModel{}).Where("id = ?", 1).Count(&count)
	if count != 0 {
		t.Fatal("expected the row to be deleted")
	}
}
