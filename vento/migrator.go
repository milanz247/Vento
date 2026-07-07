package vento

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Migration is one ordered, reversible schema change - Vento's equivalent
// of a single file under Laravel's database/migrations. Application code
// declares these in the migrations package (see migrations/migrations.go);
// the framework only knows how to run and track them.
//
//   - ID is a unique, sortable identifier. The convention is a timestamp
//     prefix so lexical order equals chronological order, e.g.
//     "20260707_120000_create_posts_table". It is what gets recorded in the
//     schema_migrations table, so it must never change once a migration has
//     run anywhere.
//   - Up applies the change. It receives the same *gorm.DB the app uses, so
//     it can call tx.AutoMigrate, tx.Migrator(), or raw tx.Exec.
//   - Down reverses it, and is optional: a migration with a nil Down is
//     irreversible and db:rollback will refuse to revert it.
type Migration struct {
	ID   string
	Up   func(tx *gorm.DB) error
	Down func(tx *gorm.DB) error
}

// schemaMigration is one row of the schema_migrations tracking table: the
// ID of a migration that has been applied, and when. Its presence is what
// makes RunMigrations idempotent - an already-recorded ID is skipped.
type schemaMigration struct {
	ID        string `gorm:"primaryKey;size:191"`
	AppliedAt time.Time
}

// TableName pins the tracking table's name so it reads clearly in the
// database and never gets pluralized by GORM's naming strategy.
func (schemaMigration) TableName() string { return "schema_migrations" }

// ensureMigrationsTable creates the schema_migrations tracking table if it
// does not exist yet. It is safe to call before every migrate/rollback.
func ensureMigrationsTable(db *gorm.DB) error {
	if err := db.AutoMigrate(&schemaMigration{}); err != nil {
		return fmt.Errorf("vento: creating schema_migrations table: %w", err)
	}
	return nil
}

// RunMigrations applies every migration in list whose ID is not already
// recorded in schema_migrations, in slice order, and records each one as it
// succeeds. It returns the IDs it applied (empty when the database is
// already up to date), so the caller can report progress.
//
// Each migration runs inside its own transaction together with the tracking
// insert, so the two can never drift apart. Note the standard MySQL caveat:
// DDL statements (CREATE TABLE, ALTER TABLE, ...) cause an implicit commit,
// so a migration that fails partway through a schema change cannot be rolled
// back by the transaction - author each migration to be a single, coherent
// step and, where a change spans several statements, make it re-runnable.
func RunMigrations(db *gorm.DB, list []Migration) ([]string, error) {
	if err := ensureMigrationsTable(db); err != nil {
		return nil, err
	}

	applied, err := appliedIDs(db)
	if err != nil {
		return nil, err
	}

	var ran []string
	for _, m := range list {
		if applied[m.ID] {
			continue
		}
		if m.Up == nil {
			return ran, fmt.Errorf("vento: migration %q has no Up function", m.ID)
		}

		err := db.Transaction(func(tx *gorm.DB) error {
			if err := m.Up(tx); err != nil {
				return err
			}
			return tx.Create(&schemaMigration{ID: m.ID, AppliedAt: time.Now()}).Error
		})
		if err != nil {
			return ran, fmt.Errorf("vento: migration %q failed: %w", m.ID, err)
		}
		ran = append(ran, m.ID)
	}
	return ran, nil
}

// RollbackLastMigration reverts the most recently applied migration: it runs
// that migration's Down function and deletes its tracking row. It returns
// the ID it reverted, or "" when there is nothing left to roll back.
//
// It errors if the last-applied migration is no longer present in list (the
// registry and the database have diverged) or is irreversible (nil Down).
func RollbackLastMigration(db *gorm.DB, list []Migration) (string, error) {
	if err := ensureMigrationsTable(db); err != nil {
		return "", err
	}

	var last schemaMigration
	err := db.Order("applied_at DESC, id DESC").First(&last).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil // nothing has been applied yet
	}
	if err != nil {
		return "", fmt.Errorf("vento: reading schema_migrations: %w", err)
	}

	var target *Migration
	for i := range list {
		if list[i].ID == last.ID {
			target = &list[i]
			break
		}
	}
	if target == nil {
		return "", fmt.Errorf("vento: applied migration %q is not in the registry - cannot roll back", last.ID)
	}
	if target.Down == nil {
		return "", fmt.Errorf("vento: migration %q is irreversible (no Down function)", target.ID)
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		if err := target.Down(tx); err != nil {
			return err
		}
		return tx.Delete(&last).Error
	})
	if err != nil {
		return "", fmt.Errorf("vento: rolling back %q failed: %w", target.ID, err)
	}
	return target.ID, nil
}

// appliedIDs returns the set of migration IDs already recorded as applied.
func appliedIDs(db *gorm.DB) (map[string]bool, error) {
	var rows []schemaMigration
	if err := db.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("vento: reading schema_migrations: %w", err)
	}
	set := make(map[string]bool, len(rows))
	for _, r := range rows {
		set[r.ID] = true
	}
	return set, nil
}

// AutoMigrateModels runs GORM's AutoMigrate over each model, in order. It
// backs the CLI's db:automigrate command: an additive, idempotent,
// untracked schema sync driven straight off models.All(), handy for rapid
// prototyping. Prefer versioned migrations (RunMigrations) once a schema
// needs ordered, reversible history.
func AutoMigrateModels(db *gorm.DB, models []any) error {
	for _, model := range models {
		if err := db.AutoMigrate(model); err != nil {
			return fmt.Errorf("vento: auto-migrating %T failed: %w", model, err)
		}
	}
	return nil
}
