package migrations

import (
	"gorm.io/gorm"

	"vento-app/app/models"
	"vento-app/vento/migrate"
)

// 20260101_000001_create_users_table creates the users table that the
// db:seed users seeder depends on. It bridges to the GORM model so the
// schema stays defined in one place (models/user.go): Up runs AutoMigrate
// for User, Down drops the table. This is the pattern for a model-backed
// migration; for a change without a struct behind it, use raw tx.Exec(...)
// in Up/Down instead.
func init() {
	register(migrate.Migration{
		ID: "20260101_000001_create_users_table",
		Up: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&models.User{})
		},
		Down: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(&models.User{})
		},
	})
}
