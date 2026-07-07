# Database

Vento integrates with MySQL through [GORM](https://gorm.io) and treats MySQL
as its **only** supported provider — there is no driver abstraction for
Postgres or SQLite. That's a deliberate trade: the entire database surface
of the framework is one DSN builder (`vento/config.go`) and one ten-line
`ConnectDB` (`vento/engine.go`), and those are exactly what you'd fork to
change providers.

## How the connection flows through the framework

```
.env ──LoadEnv──▶ map ──BuildMySQLDSN──▶ dsn ──ConnectDB──▶ Engine.DB (*gorm.DB)
                                                                │
                                              dispatch injects it per request
                                                                ▼
                                                         c.DB() in handlers
```

The connection pool is opened **once, at startup**
([Bootstrapping](bootstrapping.md#step-4--appconnectdbdsn)); a failure
aborts the boot. Every request's Context then gets the same `*gorm.DB`
injected before its handler chain runs — `c.DB()` is a field read, not a
lookup, and controllers need zero setup to query.

## Models

Models live in `models/` as plain GORM structs:

```go
// models/user.go
type User struct {
    gorm.Model
    Name  string
    Email string
}
```

Embedding `gorm.Model` provides `ID`, `CreatedAt`, `UpdatedAt`, and a
soft-delete `DeletedAt`. Nothing in Vento requires the embed — it's a GORM
convention — but the demo model uses it and the seeder relies on `ID`
existing.

### The model registry

`models.All()` lists every model for the `db:automigrate` shortcut:

```go
func All() []any {
    return []any{
        &User{},
    }
}
```

Adding a model is a two-step: define the struct, append `&YourModel{}` here.
`vento db:automigrate` reads this list and nothing else — there is no
directory scanning or struct-tag magic to trip over. For ordered, reversible
schema history use a migration instead (see [Migrations](#migrations));
`db:automigrate` is the quick, untracked path for prototyping.

## Querying from a controller

`c.DB()` returns the shared `*gorm.DB`; use it exactly as in any GORM
codebase:

```go
func GetUsers(c *vento.Context) {
    var users []models.User
    if err := c.DB().Find(&users).Error; err != nil {
        log.Printf("controllers: listing users failed: %v", err)
        c.Abort(http.StatusInternalServerError, "could not list users")
        return
    }
    c.JSON(http.StatusOK, users)
}
```

Two conventions Vento's own controllers follow — keep them:

- **Log the real error server-side; return a generic message to the
  client.** Raw GORM/SQL error strings disclose schema and driver
  internals — reconnaissance fuel for injection attempts
  ([Security](security.md)).
- **Bind requests to a dedicated input struct, never straight onto the
  GORM model.** Mass-assignment protection: a client sending
  `{"id": 1, "created_at": ...}` in a `POST /users` body can never reach
  those fields, because the input struct doesn't declare them:

  ```go
  type createUserInput struct {
      Name  string `json:"name"`
      Email string `json:"email"`
  }

  var in createUserInput
  if err := json.NewDecoder(c.Request.Body).Decode(&in); err != nil { ... }
  user := models.User{Name: in.Name, Email: in.Email}   // explicit copy
  ```

## Migrations

Vento has a versioned migration registry in the `migrations/` package
(mirroring Laravel's `database/migrations`). A migration is a
`vento.Migration` — a sortable `ID`, an `Up`, and an optional `Down`:

```go
// migrations/20260101_000001_create_users_table.go
func init() {
    register(vento.Migration{
        ID:   "20260101_000001_create_users_table",
        Up:   func(tx *gorm.DB) error { return tx.AutoMigrate(&models.User{}) },
        Down: func(tx *gorm.DB) error { return tx.Migrator().DropTable(&models.User{}) },
    })
}
```

Each migration file **registers itself** in an `init()`, so adding a file is
all it takes — there is no central list to hand-edit. `migrations.All()`
returns them sorted by `ID`, and because IDs are timestamp-prefixed, sorted
order is apply order.

Scaffold a new one (the timestamp prefix is generated for you):

```bash
./bin/vento make:migration create_posts_table
# -> migrations/20260707_142530_create_posts_table.go
```

Fill in `Up`/`Down`, then:

```bash
./bin/vento db:migrate     # apply every pending migration, in order
./bin/vento db:rollback    # revert the most recently applied migration
```

`db:migrate` records each applied `ID` in a **`schema_migrations`** tracking
table and skips anything already recorded, so it is safe to re-run and only
ever runs new migrations. `db:rollback` runs the last migration's `Down` and
deletes its tracking row (it refuses a migration whose `Down` is `nil`).

> **MySQL caveat.** DDL statements (`CREATE TABLE`, `ALTER TABLE`, …) trigger
> an implicit commit in MySQL, so the transaction wrapped around each
> migration cannot undo a half-finished schema change. Keep each migration a
> single coherent step, and make multi-statement changes re-runnable.

### `db:automigrate` — the prototyping shortcut

For rapid iteration, `db:automigrate` runs GORM `AutoMigrate` over
`models.All()` directly — additive, idempotent, and **untracked** (no
`schema_migrations` row):

```bash
./bin/vento db:automigrate
```

It creates missing tables, columns, and indexes and never drops or renames
anything. Reach for it while a model's shape is still in flux; once the
schema needs ordered, reversible history, move to migrations.

## Seeders

Seeders are registered in `cmd/vento/main.go`:

```go
type seeder struct {
    name string
    run  func(db *gorm.DB) error
}

var seeders = []seeder{
    {name: "users", run: seedUsers},
}
```

Run them all with:

```bash
./bin/vento db:seed
```

The contract is that **every seeder is idempotent** — safe to run
repeatedly, on any environment. The built-in `seedUsers` achieves this
with `FirstOrCreate` keyed on a natural unique value (email):

```go
err := db.Where(models.User{Email: testUsers[i].Email}).
    FirstOrCreate(&testUsers[i]).Error
```

Re-running finds the existing row and inserts nothing. To add your own
seeder: write a `func(db *gorm.DB) error` following the same pattern and
append a `seeder{}` entry — order in the slice is execution order, so put
dependency data (e.g. users) before data that references it.

## Why the CLI connects independently

The `db:*` commands (`db:migrate`, `db:rollback`, `db:automigrate`,
`db:seed`) don't boot the web application — they run
`LoadEnv → BuildMySQLDSN → New → ConnectDB` themselves (the `openDB` helper
in `cmd/vento/main.go`) and operate directly on the pool. Same code path as
the app's boot, minus templates, routes, and the server. See
[CLI Reference](cli-reference.md).
