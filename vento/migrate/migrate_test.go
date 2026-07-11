package migrate

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("opening in-memory sqlite: %v", err)
	}
	return db
}

func TestRunAppliesPendingMigrationsInOrder(t *testing.T) {
	db := newTestDB(t)
	var order []string

	list := []Migration{
		{ID: "b", Up: func(tx *gorm.DB) error { order = append(order, "b"); return nil }},
		{ID: "a", Up: func(tx *gorm.DB) error { order = append(order, "a"); return nil }},
	}

	applied, err := Run(db, list)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(applied) != 2 || applied[0] != "b" || applied[1] != "a" {
		t.Fatalf("expected migrations applied in slice order [b a], got %v", applied)
	}
	if len(order) != 2 || order[0] != "b" || order[1] != "a" {
		t.Fatalf("expected Up() called in slice order, got %v", order)
	}
}

func TestRunSkipsAlreadyAppliedMigrations(t *testing.T) {
	db := newTestDB(t)
	calls := 0
	list := []Migration{{ID: "only", Up: func(tx *gorm.DB) error { calls++; return nil }}}

	if _, err := Run(db, list); err != nil {
		t.Fatalf("unexpected error on first run: %v", err)
	}
	applied, err := Run(db, list)
	if err != nil {
		t.Fatalf("unexpected error on second run: %v", err)
	}
	if len(applied) != 0 {
		t.Fatalf("expected nothing to apply on the second run, got %v", applied)
	}
	if calls != 1 {
		t.Fatalf("expected Up() to run exactly once across both calls, ran %d times", calls)
	}
}

func TestRunRequiresUpFunction(t *testing.T) {
	db := newTestDB(t)
	list := []Migration{{ID: "broken"}} // no Up

	if _, err := Run(db, list); err == nil {
		t.Fatal("expected an error for a migration with no Up function")
	}
}

func TestRollbackLastRevertsMostRecent(t *testing.T) {
	db := newTestDB(t)
	var reverted string

	list := []Migration{
		{ID: "a", Up: func(tx *gorm.DB) error { return nil }, Down: func(tx *gorm.DB) error { reverted = "a"; return nil }},
		{ID: "b", Up: func(tx *gorm.DB) error { return nil }, Down: func(tx *gorm.DB) error { reverted = "b"; return nil }},
	}
	if _, err := Run(db, list); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	id, err := RollbackLast(db, list)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "b" || reverted != "b" {
		t.Fatalf("expected the most recently applied migration (b) to be reverted, got id=%q reverted=%q", id, reverted)
	}
}

func TestRollbackLastNoopWhenNothingApplied(t *testing.T) {
	db := newTestDB(t)
	id, err := RollbackLast(db, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "" {
		t.Fatalf("expected empty id when nothing has been applied, got %q", id)
	}
}

func TestRollbackLastRejectsIrreversibleMigration(t *testing.T) {
	db := newTestDB(t)
	list := []Migration{{ID: "a", Up: func(tx *gorm.DB) error { return nil }}} // no Down
	if _, err := Run(db, list); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := RollbackLast(db, list); err == nil {
		t.Fatal("expected an error rolling back a migration with no Down function")
	}
}

func TestStatusReportsAppliedAndPending(t *testing.T) {
	db := newTestDB(t)
	list := []Migration{
		{ID: "a", Up: func(tx *gorm.DB) error { return nil }},
		{ID: "b", Up: func(tx *gorm.DB) error { return nil }},
	}
	if _, err := Run(db, list[:1]); err != nil { // only apply "a"
		t.Fatalf("unexpected error: %v", err)
	}

	status, err := Status(db, list)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(status) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(status))
	}
	if !status[0].Applied || status[0].AppliedAt.IsZero() {
		t.Fatalf("expected %q to be applied with a timestamp, got %+v", "a", status[0])
	}
	if status[1].Applied {
		t.Fatalf("expected %q to be pending, got %+v", "b", status[1])
	}
}

func TestStatusOnFreshDatabase(t *testing.T) {
	db := newTestDB(t)
	list := []Migration{{ID: "a", Up: func(tx *gorm.DB) error { return nil }}}

	status, err := Status(db, list)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(status) != 1 || status[0].Applied {
		t.Fatalf("expected a single pending entry, got %+v", status)
	}
}

func TestAutoMigrateModels(t *testing.T) {
	db := newTestDB(t)
	type widget struct {
		ID   uint `gorm:"primaryKey"`
		Name string
	}

	if err := AutoMigrateModels(db, []any{&widget{}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !db.Migrator().HasTable(&widget{}) {
		t.Fatal("expected the widget table to exist after AutoMigrateModels")
	}
}
