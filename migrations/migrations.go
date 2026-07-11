// Package migrations holds the application's ordered, reversible schema
// changes, mirroring Laravel's database/migrations. Each migration lives in
// its own file and registers itself in an init() via register(); the CLI
// drives them:
//
//	vento make:migration create_posts_table   scaffold a new migration file
//	vento db:migrate                           apply every pending migration
//	vento db:rollback                          revert the most recent one
//
// A migration is a migrate.Migration: an ID (timestamp-prefixed, so lexical
// order is chronological), an Up function, and an optional Down. The
// framework records each applied ID in a schema_migrations table, so
// db:migrate only ever runs migrations that have not run yet.
//
// This package imports vento/migrate and models only - never controllers or
// routes - keeping the one-way dependency graph intact.
package migrations

import (
	"sort"

	"vento-app/vento/migrate"
)

// registry accumulates every migration registered by an init() in this
// package. All() returns it sorted, so the physical file order never
// affects apply order.
var registry []migrate.Migration

// register adds one migration to the package registry. Each migration file
// calls it from an init() function, so simply adding a file (as
// make:migration does) is enough to have it picked up - there is no central
// list to hand-edit.
func register(m migrate.Migration) {
	registry = append(registry, m)
}

// All returns every registered migration sorted by ID. Because IDs are
// timestamp-prefixed, sorted order is chronological order, which is exactly
// the order migrations must be applied in. The CLI feeds this straight to
// migrate.Run / migrate.RollbackLast.
func All() []migrate.Migration {
	out := make([]migrate.Migration, len(registry))
	copy(out, registry)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
