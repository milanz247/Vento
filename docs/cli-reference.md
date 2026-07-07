# CLI Reference

Vento ships an Artisan-style developer CLI. Its source is
`cmd/vento/main.go` — a single readable file: a `switch` over
`os.Args[1]`, the seeder registry, and the scaffolding stubs. It reads
`./.env` and writes under the app packages (`controllers/`, `models/`,
`middleware/`, `migrations/`), so **always run it from the project root**.

## Installing it

The CLI can't be built at the project root under its natural name — `vento`
is taken by the framework package directory — so `setup.go` (a
`//go:build ignore` script, run explicitly) compiles `cmd/vento` to a
conventional location:

```bash
go run setup.go        # interactive prompt
go run setup.go 1      # local:  ./bin/vento   (bin\vento.exe on Windows)
go run setup.go 2      # global: /usr/local/bin/vento (PATH registration via setx on Windows)
```

Local (option 1) is recommended: per-project binary, no system changes,
and `./bin/` is already gitignored. You can also skip installation
entirely — every command works as `go run ./cmd/vento <command>`.

## Commands

### `vento run` (alias: `vento serve`)

Starts the application:

- If [air](https://github.com/air-verse/air) is on `PATH`, runs it with
  the project's `.air.toml` — **hot reload**: edits to `.go` and `.html`
  files trigger a rebuild and restart (which re-runs the whole
  [boot sequence](bootstrapping.md), including template recompilation).
- Otherwise falls back to a plain `go run .` with a hint on installing
  air (`go install github.com/air-verse/air@latest`).

`.air.toml` notes: builds to `tmp/main`, watches `go`/`html` extensions,
excludes `tmp/`, `bin/`, `.git/`, `.env`, and test files, and coalesces
rapid saves with a 500 ms delay.

### `vento db:migrate`

Applies every pending migration in `migrations.All()`, in `ID` order,
recording each in the `schema_migrations` table so it never runs twice.
Details in [Database § Migrations](database.md#migrations).

### `vento db:rollback`

Reverts the most recently applied migration: runs its `Down` and deletes its
`schema_migrations` row. Refuses a migration that has no `Down`.

### `vento db:automigrate`

Runs GORM `AutoMigrate` over every model in `models.All()` — additive,
idempotent, never drops anything, and **untracked**. The quick path while a
schema is still in flux; graduate to migrations for reversible history.
See [Database § Migrations](database.md#migrations).

### `vento db:seed`

Runs every registered seeder in order. Each seeder is required to be
idempotent (`FirstOrCreate` or equivalent), so re-seeding never
duplicates rows. Adding seeders: [Database § Seeders](database.md#seeders).

### `vento make:controller <Name>`

Scaffolds `controllers/<snake>_controller.go` with stubbed
`<Name>Index` / `<Name>Show` / `<Name>Store` handlers ready to wire into
`routes/web.go`:

```bash
vento make:controller Post        # -> controllers/post_controller.go
vento make:controller blog-post   # -> controllers/blog_post_controller.go (BlogPostIndex, ...)
```

The name normalizes to StudlyCase for Go identifiers and snake_case for
the filename; characters that aren't letters/digits act as word breaks.

### `vento make:model <Name>`

Scaffolds `models/<snake>.go` with a `gorm.Model`-embedding struct, then
reminds you to register it in `models.All()`:

```bash
vento make:model Post        # -> models/post.go (type Post struct { gorm.Model })
```

### `vento make:middleware <Name>`

Scaffolds `middleware/<snake>.go` with a `func(*vento.Context)` stub, then
reminds you to wire it into `routes/web.go`:

```bash
vento make:middleware RequireAuth   # -> middleware/require_auth.go
```

### `vento make:migration <name>`

Scaffolds a timestamped, self-registering migration under `migrations/`. The
name is normalized to snake_case and prefixed with a UTC timestamp, so files
sort chronologically — which is apply order:

```bash
vento make:migration create_posts_table
# -> migrations/20260707_142530_create_posts_table.go
```

Fill in `Up`/`Down`; the file's `init()` registers it automatically, so
there is no list to hand-edit. See
[Database § Migrations](database.md#migrations).

Every `make:` command shares the same normalization and **refuses to
overwrite** an existing file.

## How the DB commands boot

The `db:*` commands (`db:migrate`, `db:rollback`, `db:automigrate`,
`db:seed`) perform a *partial* application bootstrap —
`LoadEnv → BuildMySQLDSN → vento.New() → ConnectDB` (the `openDB` helper) —
then operate on `Engine.DB` directly. No templates, no routes, no HTTP
server. Missing DB config or an unreachable MySQL aborts with a clear
error, exactly like the app itself. That the CLI can cherry-pick boot
steps like this is a property of the boot being plain function calls — see
[Bootstrapping](bootstrapping.md#the-clis-variant-of-the-same-boot).

## Extending the CLI

- **New command:** add a `case` to the `switch os.Args[1]` in
  `cmd/vento/main.go` and a line to `usage()`. Reuse `openDB()` if it needs
  the database.
- **New seeder:** append a `seeder{name, run}` entry to the `seeders`
  slice; slice order is execution order.
- **New scaffold:** follow the `make:*` pattern — a Go string-template
  constant, `fmt.Sprintf` substitution, and a call to the shared
  `writeScaffold` helper, which creates the parent directory and refuses to
  clobber an existing file.
