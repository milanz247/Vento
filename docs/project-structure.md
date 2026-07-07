# Project Structure

Vento ships as a framework (`vento/`) plus a thin demo application built on
it, in one module. This page maps every file, then explains the dependency
rules that keep the layout honest.

```
vento-app/
├── vento/                  # THE FRAMEWORK (import path: vento-app/vento)
│   ├── engine.go          # Engine: registration, template compilation, ServeHTTP, hardened server
│   ├── router.go          # Trie router: one prefix tree per HTTP method
│   ├── context.go         # Context: the object every handler receives
│   ├── middlewares.go     # Logger, Recovery
│   ├── security.go        # SecurityHeaders, BodyLimit, RateLimiter, CSRFProtection
│   ├── migrator.go        # Migration type + registry runner (RunMigrations, rollback, schema_migrations)
│   ├── static.go          # Engine.Static: static file mounts
│   └── config.go          # .env loading, MySQL DSN assembly
│
├── controllers/           # Request handlers (mirrors Laravel's app/Http/Controllers)
│   └── home_controller.go # Index — the welcome page (add your own handlers here)
│
├── models/                # GORM data models + the model registry
│   └── user.go            # Example User model + models.All() (what db:automigrate syncs)
│
├── middleware/            # Your own middleware (mirrors Laravel's app/Http/Middleware)
│   └── request_id.go      # RequestID — example; wired into routes/web.go's global chain
│
├── migrations/            # Ordered, reversible schema changes (mirrors database/migrations)
│   ├── migrations.go      # register() + All() — the self-registering migration registry
│   └── 20260101_000001_create_users_table.go  # Example migration (make:migration scaffolds more)
│
├── routes/                # Endpoint declarations (mirrors Laravel's routes/web.php)
│   └── web.go             # RegisterRoutes: global middlewares + every route
│
├── views/                 # HTML templates, compiled once at startup
│   ├── layouts/base.html  # Shared document shell: <head>, {{template "content" .}}
│   └── index.html         # The welcome page ({{define "content"}})
│
├── cmd/vento/main.go       # The `vento` CLI: run, db:migrate/rollback/automigrate/seed, make:*
│
├── assets/                # Front-end SOURCE (not served)
│   ├── logo.svg, icon.svg # Brand assets (README, favicon)
│   └── css/input.css      # Tailwind source
├── public/                # Served at /public via app.Static
│   └── css/app.css        # Compiled Tailwind output (gitignored, generated)
│
├── docs/                  # This documentation set
├── main.go                # Application entry point: the bootstrap sequence, nothing else
├── setup.go               # Zero-config CLI installer (//go:build ignore — run explicitly)
├── package.json           # npm scripts: build:css, watch:css
├── .air.toml              # Hot-reload configuration for air
├── .env                   # Local configuration (gitignored; never commit real credentials)
├── SECURITY_AUDIT.md      # The threat model: gaps found, mitigations, residual risks
└── README.md
```

## Framework vs. application

The split is strict and worth internalizing:

- **`vento/` is the framework.** It imports only the standard library and
  GORM — never `controllers`, `models`, `middleware`, `migrations`, or
  `routes`. It has no idea what application is built on it. Everything under
  `vento/` is documented in [Architecture](architecture.md).
- **Everything else is the application** — a deliberately small demo whose
  every route exists to exercise one framework feature
  ([the route map](routing.md#the-demo-route-map)). Building your own app
  means replacing the demo controllers/models/views/middleware/migrations,
  not touching `vento/`.

## The import graph is deliberately one-way

```
main.go ─┐
cmd/vento┴─▶ routes ─▶ controllers ─▶ models ◀─ migrations
                └────▶ middleware

  routes, controllers, middleware, migrations, models  ── all import ──▶  vento
  vento imports nothing from the application
```

- `routes` imports `controllers`, `middleware`, and `vento` — never `main`.
  This is what prevents a `routes ↔ main` import cycle.
- `controllers` imports `models` and `vento` — never `routes`. A controller
  doesn't know what URL it's mounted at.
- `middleware` imports only `vento` — the same one-way rule as controllers,
  so your own middleware never reaches back into routes or controllers.
- `migrations` imports `vento` and `models` (to reference the structs a
  migration creates); only the CLI drives it, never `main`/`routes`.
- `models` imports only GORM.
- `vento` imports nothing from the application.

The payoff is that `main.go` stays a pure bootstrap — load config, connect
DB, compile templates, register routes, serve — with zero request-handling
logic. The exact sequence, and why its order matters, is the
[Bootstrapping](bootstrapping.md) guide.

## Why `cmd/vento` and not a root-level binary

The framework package already owns the directory name `vento/`. The CLI
binary also wants to be called `vento`, so its source lives at
`cmd/vento/main.go` (the standard Go convention for a module that is both an
importable package and a companion binary) and `setup.go` compiles it to
`./bin/vento`.

## `setup.go`

Tagged `//go:build ignore`, so `go build ./...` and `go run .` never touch
it — it only runs when invoked explicitly (`go run setup.go`). It exists
purely to compile `cmd/vento` into a convenient binary, locally
(`./bin/vento`) or globally (`/usr/local/bin/vento`). See
[CLI Reference](cli-reference.md#installing-it).

## Generated / local-only paths

Everything a fresh clone regenerates is gitignored:

| Path | Produced by |
|---|---|
| `bin/` | `go run setup.go` |
| `tmp/` | air (build output + logs) |
| `node_modules/`, `public/css/app.css` | `npm install`, `npm run build:css` |
| `.env` | you (copy the keys from [Configuration](configuration.md)) |

## Adding a new feature end to end

A typical feature ("posts") is assembled entirely from scaffolds — every
step stays in the application layer, never `vento/` or `main.go`:

1. **Model** — `./bin/vento make:model Post` scaffolds `models/post.go`; add
   your fields, then append `&Post{}` to `models.All()`.
2. **Migration** — `./bin/vento make:migration create_posts_table` scaffolds
   a timestamped, self-registering file under `migrations/`; fill in `Up`
   (e.g. `tx.AutoMigrate(&models.Post{})`) and `Down`.
3. **Controller** — `./bin/vento make:controller Post` scaffolds
   `controllers/post_controller.go` with `PostIndex`/`PostShow`/`PostStore`
   stubs.
4. **Middleware** (only if the feature needs its own) — `./bin/vento
   make:middleware RequirePostOwner` scaffolds
   `middleware/require_post_owner.go`.
5. **Routes** — wire `app.GET("/posts", controllers.PostIndex)` (plus any
   per-route middleware) in `routes/web.go`.
6. **Views** (if it renders HTML) — add `views/posts/index.html` with a
   `{{define "content"}}` block; render it with `c.View("posts/index", data)`.

Then `./bin/vento db:migrate` applies the new migration and a restart picks
up the new template. The order mirrors the import graph — you build from the
model outward, and no step ever requires editing `vento/` or `main.go`.
