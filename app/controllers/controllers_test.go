package controllers_test

import (
	"testing"

	"vento-app/app/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newTestDB returns a fresh in-memory sqlite database migrated for
// models.User, shared by every controller test in this package.
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
