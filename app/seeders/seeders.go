// Package seeders holds the application's database seeders - idempotent
// functions that insert starter/test data, mirroring Laravel's
// database/seeders (and this project's own migrations package). Each
// seeder lives in its own file and registers itself in an init() via
// register(), so adding a file (as make:seeder does) is enough to have it
// picked up - there is no central list to hand-edit. The CLI drives them:
//
//	vento make:seeder Products   scaffold app/seeders/products_seeder.go
//	vento db:seed                run every registered seeder (safe to re-run)
package seeders

import "gorm.io/gorm"

// Seeder is one named, idempotent database seeding step. Run must be safe
// to call repeatedly - use FirstOrCreate (or equivalent) so re-seeding
// never inserts duplicates.
type Seeder struct {
	Name string
	Run  func(db *gorm.DB) error
}

// registry accumulates every seeder registered by an init() in this
// package. All() returns it in registration order.
var registry []Seeder

// register adds one seeder to the package registry. Each seeder file calls
// it from an init() function.
func register(s Seeder) {
	registry = append(registry, s)
}

// All returns every registered seeder, in registration order.
func All() []Seeder {
	out := make([]Seeder, len(registry))
	copy(out, registry)
	return out
}
