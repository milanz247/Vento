// Package models holds GORM data models shared across controllers, plus
// the migration registry the CLI drives.
package models

import "gorm.io/gorm"

// User is an example GORM model. Copy it to shape your own: embed
// gorm.Model for the ID/CreatedAt/UpdatedAt/DeletedAt columns, add fields,
// then register the model in All() below so db:migrate picks it up.
//
// PasswordHash is tagged json:"-" so it can never be serialized into an API
// response - c.OK(user) (or any other path that JSON-encodes a User) must
// not be able to leak it, no matter which handler forgets to build a
// response DTO. It's set via hash.Make (see app/controllers/auth_controller.go),
// never stored or logged as plaintext.
type User struct {
	gorm.Model
	Name         string
	Email        string `gorm:"uniqueIndex;size:255"`
	PasswordHash string `json:"-" gorm:"size:60"`
}

// All returns every model registered for schema migration. The CLI's
// db:migrate command feeds this straight into GORM's AutoMigrate, so
// adding a new model here is the only step needed to have it migrated.
func All() []any {
	return []any{
		&User{},
	}
}
