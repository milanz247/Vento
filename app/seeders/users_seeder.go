package seeders

import (
	"vento-app/app/models"

	"gorm.io/gorm"
)

func init() {
	register(Seeder{Name: "users", Run: seedUsers})
}

// seedUsers inserts five test users, keyed on email so re-running the
// seeder is a no-op for rows that already exist.
func seedUsers(db *gorm.DB) error {
	testUsers := []models.User{
		{Name: "Ada Lovelace", Email: "ada@example.com"},
		{Name: "Grace Hopper", Email: "grace@example.com"},
		{Name: "Alan Turing", Email: "alan@example.com"},
		{Name: "Edsger Dijkstra", Email: "edsger@example.com"},
		{Name: "Barbara Liskov", Email: "barbara@example.com"},
	}
	for i := range testUsers {
		err := db.Where(models.User{Email: testUsers[i].Email}).
			FirstOrCreate(&testUsers[i]).Error
		if err != nil {
			return err
		}
	}
	return nil
}
