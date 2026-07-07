<p align="center">
  <img src="assets/logo.svg" alt="Vento — a high-performance Go web framework" width="420">
</p>

<p align="center">
  <strong>A lightweight, high-performance Go web framework built on the standard library.</strong>
</p>

<p align="center">
  <a href="#getting-started">Getting Started</a> ·
  <a href="docs/README.md">Documentation</a> ·
  <a href="#project-structure">Project Structure</a> ·
  <a href="SECURITY_AUDIT.md">Security Audit</a>
</p>

---

## What is Vento?

Vento is a small, opinionated web framework for Go — a single `Context`
object, chainable middleware, an MVC-style project layout, and an
Artisan-style CLI — built directly on `net/http` and `html/template`, with
GORM/MySQL as its only external dependency. It's designed to be **read and
understood end to end**, not just used as a black box: the entire framework
is eight files under `vento/`, and the [documentation](docs/README.md)
explains how every piece is built and why.

## Features

- **Custom Trie router** — one prefix tree per HTTP method, dynamic
  `:param` segments, static routes beating wildcards with clean
  backtracking.
- **Pooled, zero-allocation `Context`** — recycled via `sync.Pool`; handler
  chains are compiled once at route-registration time, never per request.
- **MVC-style structure** — `controllers/`, `models/`, `middleware/`,
  `migrations/`, `routes/`, with a strict one-way import graph
  (`main → routes → controllers → models`, everything → `vento`).
- **Layout-based HTML templating** — every page is pre-stitched into the
  shared layout at startup (`Engine.LoadHTMLGlob`); rendering is a single
  `ExecuteTemplate` call, with `html/template`'s XSS-safe escaping built in.
- **Reactive UIs with HTMX** — `c.IsHTMX()` and `c.Partial()` let one
  handler serve full pages and DOM fragments; opt in when you need it. The
  [Todo tutorial](docs/tutorial-todo.md) builds a full example. See
  [docs/htmx.md](docs/htmx.md).
- **GORM + MySQL** — models, a versioned migration registry (`db:migrate` /
  `db:rollback` over self-registering migration files, tracked in
  `schema_migrations`), and idempotent seeders, all driven by a single
  `.env` file.
- **Security on by default** — per-IP rate limiting, double-submit-cookie
  CSRF, request body limits, hardened server timeouts, security headers,
  panic recovery, and an integrity-pinned CDN script. The full threat
  model is written down in [`SECURITY_AUDIT.md`](SECURITY_AUDIT.md).
- **Artisan-style CLI (`vento`)** — `run` (hot reload via air), the `db:*`
  commands (`db:migrate`, `db:rollback`, `db:automigrate`, `db:seed`), and
  `make:controller` / `make:model` / `make:middleware` / `make:migration`
  scaffolding.
- **Tailwind CSS, built locally** — a clean, minimal welcome page; no CSS
  CDN at runtime.

## Getting Started

**Prerequisites:** Go 1.22+, a running MySQL server, Node.js + npm (for the
one-time CSS build).

```bash
# 1. Configure the database in .env
#    DB_HOST, DB_USER, DB_NAME required; DB_PORT defaults to 3306

# 2. (optional) install the vento CLI locally
go run setup.go 1

# 3. Build the Tailwind CSS bundle
npm install
npm run build:css

# 4. Migrate and seed
./bin/vento db:migrate
./bin/vento db:seed

# 5. Run
./bin/vento run     # or: go run .
```

Open **http://localhost:8080** — a clean, minimal welcome page. The full
walkthrough, including a tour of every demo route, is in
[docs/getting-started.md](docs/getting-started.md).

## Documentation

The [`docs/`](docs/README.md) folder is the complete reference — including
deep dives into how the framework itself is built:

| Guide | Covers |
|---|---|
| [Getting Started](docs/getting-started.md) | Setup, CLI install, first run, a tour of every demo route |
| [Project Structure](docs/project-structure.md) | Every file/folder, the one-way import graph |
| [Bootstrapping](docs/bootstrapping.md) | The complete startup sequence, step by step, and why its order matters |
| [Architecture](docs/architecture.md) | `Engine`, `Context`, router internals, the request lifecycle, the performance model |
| [Routing](docs/routing.md) | Registering routes, `:params`, Trie matching rules |
| [The Context API](docs/context.md) | Every method on `*vento.Context` |
| [Middleware](docs/middleware.md) | The chain model, all built-ins, writing your own |
| [Views & Templates](docs/views-templates.md) | Startup template compilation, layouts, rendering |
| [Reactive UIs with HTMX](docs/htmx.md) | `IsHTMX()`, `Partial()`, the CSRF bridge |
| [Front-end: Tailwind & Static Assets](docs/frontend-tailwind.md) | `Engine.Static`, the Tailwind build |
| [Database](docs/database.md) | Models, the migration registry, seeders, querying |
| [Configuration](docs/configuration.md) | `.env` parsing, MySQL DSN assembly |
| [CLI Reference](docs/cli-reference.md) | Every `vento` command |
| [Security](docs/security.md) | Every protection, the threat model, scope boundaries |
| [Tutorial: Todo CRUD](docs/tutorial-todo.md) | Hands-on: build a complete CRUD feature end to end |

## Project Structure

```
vento-app/
├── vento/          # The framework: Engine, Context, router, middleware, security, migrator, static, config
├── controllers/   # Request handlers
├── models/        # GORM data models + model registry
├── middleware/    # Your own middleware (e.g. RequestID)
├── migrations/    # Versioned, self-registering schema migrations
├── routes/        # kernel.go (global middleware stack) + web.go (the route table)
├── views/         # HTML templates (layouts + pages + partials)
├── cmd/vento/      # The `vento` CLI
├── docs/          # Full documentation
├── assets/        # Logo/icon + Tailwind source CSS
├── public/        # Compiled Tailwind CSS, served at /public via Engine.Static
├── main.go        # Thin application bootstrap
└── setup.go       # Zero-config CLI installer
```

Full breakdown in [docs/project-structure.md](docs/project-structure.md).

## Example: a minimal endpoint

```go
// controllers/hello_controller.go
func Hello(c *vento.Context) {
    name := c.Query("name")
    if name == "" {
        name = "World"
    }
    c.JSON(http.StatusOK, map[string]string{"message": "Hello, " + name})
}

// routes/web.go
app.GET("/hello", controllers.Hello)
```

## Security

Vento ships rate limiting, CSRF protection, body-size limits, hardened
server timeouts, security headers, and panic recovery enabled by default —
and documents what's deliberately out of scope (TLS termination, CSP
tuning, proxy-aware rate limiting) rather than leaving it implicit. See
[`SECURITY_AUDIT.md`](SECURITY_AUDIT.md) for the full audit and
[docs/security.md](docs/security.md) for the day-to-day summary.

## License

No license file is currently included — add one before distributing this
project beyond internal use.
