package migrations

import (
	"gorm.io/gorm"

	"vento-app/app/models"
	"vento-app/vento/migrate"
)

// 20260711_145840_add_password_to_users adds the password_hash column and a
// unique index on email to the users table, backing the new
// register/login flow (see app/controllers/auth_controller.go). AutoMigrate
// is additive and idempotent, so re-running it here against the
// already-created users table only adds what's missing.
func init() {
	register(migrate.Migration{
		ID: "20260711_145840_add_password_to_users",
		Up: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&models.User{})
		},
		Down: func(tx *gorm.DB) error {
			if err := tx.Migrator().DropIndex(&models.User{}, "email"); err != nil {
				return err
			}
			return tx.Migrator().DropColumn(&models.User{}, "PasswordHash")
		},
	})
}
