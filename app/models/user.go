// Package models holds GORM data models shared across controllers, plus
// the migration registry the CLI drives.
package models

import "gorm.io/gorm"

// User is an example GORM model. Copy it to shape your own: embed
// gorm.Model for the ID/CreatedAt/UpdatedAt/DeletedAt columns, add fields,
// then register the model in All() below so db:migrate picks it up.
type User struct {
	gorm.Model
	Name  string
	Email string
}

// All returns every model registered for schema migration. The CLI's
// db:migrate command feeds this straight into GORM's AutoMigrate, so
// adding a new model here is the only step needed to have it migrated.
func All() []any {
	return []any{
		&User{},
	}
}
