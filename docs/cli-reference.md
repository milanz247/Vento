# CLI Reference

Vento ships an Artisan-style developer CLI. Its source is
`cmd/vento/main.go` — a single readable file: a `switch` over
`os.Args[1]`, the seeder registry, and the scaffolding stub. It reads
`./.env` and writes under `./controllers`, so **always run it from the
project root**.

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

Runs GORM `AutoMigrate` over every model in `models.All()` — additive,
idempotent, never drops anything. Details and limits in
[Database § Migrations](database.md#migrations).

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
The command **refuses to overwrite** an existing file.

## How the DB commands boot

`db:migrate` and `db:seed` perform a *partial* application bootstrap —
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
- **New scaffold:** follow the `make:controller` pattern — a Go string
  template constant, `fmt.Sprintf` substitution, and an `os.Stat` guard
  before `os.WriteFile` so existing files are never clobbered.
