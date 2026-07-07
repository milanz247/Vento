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

### The migration registry

`models.All()` is the single source of truth for what gets migrated:

```go
func All() []any {
    return []any{
        &User{},
    }
}
```

Adding a model to the application is a two-step: define the struct, append
`&YourModel{}` here. `vento db:migrate` reads this list and nothing else —
there is no directory scanning or struct-tag magic to trip over.

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

There is no migration-file system — Vento uses GORM's `AutoMigrate`, driven
entirely by `models.All()`:

```bash
./bin/vento db:migrate
```

`AutoMigrate` is **additive and idempotent**: it creates missing tables,
columns, and indexes, and never drops or renames anything. That fits a
starter/small-project framework; it is *not* a versioned migration history.
If your project grows into needing reversible, ordered migrations (column
renames, data backfills), pair the registry with a dedicated tool
(golang-migrate, atlas, goose) and keep `AutoMigrate` for development only.

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

`db:migrate` and `db:seed` don't boot the web application — they run
`LoadEnv → BuildMySQLDSN → New → ConnectDB` themselves (the `openDB` helper
in `cmd/vento/main.go`) and operate directly on the pool. Same code path as
the app's boot, minus templates, routes, and the server. See
[CLI Reference](cli-reference.md).
