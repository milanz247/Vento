// Package models holds GORM data models shared across controllers, plus
// the migration registry the CLI drives.
package models

// All returns every model registered for schema migration. The CLI's
// db:automigrate command feeds this straight into GORM's AutoMigrate, so
// adding a model here (vento make:model Name scaffolds one and reminds you
// to do this) is the only step needed to have it migrated.
func All() []any {
	return []any{}
}
