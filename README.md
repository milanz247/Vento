<p align="center">
  <img src="public/assets/logo.svg" alt="Vento — a high-performance Go web framework" width="360">
</p>

<p align="center">
  <strong>A lightweight, high-performance Go web framework built on the standard library.</strong>
</p>

<p align="center">
  <a href="#getting-started">Getting Started</a> ·
  <a href="#project-structure">Project Structure</a> ·
  <a href="#testing">Testing</a>
</p>

---

## What is Vento?

Vento is a small, opinionated web framework for Go: a single `Context`,
chainable middleware, an MVC-style project layout, and a CLI — built
directly on `net/http` and `html/template`, with GORM/MySQL as its only
external dependency.

## Features

- **Trie router** — one prefix tree per HTTP method, `:param` segments,
  static routes beating wildcards.
- **Pooled, zero-allocation `Context`** — recycled via `sync.Pool`; handler
  chains compiled once at route registration, never per request.
- **Secure by default** — `vento.New()` installs logging, panic recovery,
  security headers, a body-size cap, per-IP rate limiting, and CSRF
  protection out of the box (see `vento/kernel.go`).
- **Sessions** — signed-cookie sessions via `vento.Sessions(key)` and
  `c.Session().Get/Set`, opt-in through `APP_KEY` in `.env`.
- **CORS** — `vento.CORS(origins...)` for APIs consumed by a separate
  frontend.
- **Request binding & validation** — `c.Bind(&form)` decodes JSON or form
  bodies and checks `validate:"required,email,min=8"`-style struct tags.
- **Graceful shutdown** — `app.Run` drains in-flight requests on
  SIGINT/SIGTERM instead of dropping them.
- **Layout-based templating** — every page is pre-stitched into the shared
  layout at startup; `c.View("index", data)` is a single call.
- **HTMX-friendly** — `c.IsHTMX()` and `c.Partial()` let one handler serve
  full pages and DOM fragments.
- **GORM + MySQL** — models, a versioned migration registry
  (`db:migrate` / `db:rollback`, tracked in `schema_migrations`), and
  idempotent seeders, all driven by a single `.env` file.
- **CLI (`vento`)** — `run` (hot reload via [air](https://github.com/air-verse/air)
  when installed), the `db:*` commands, and `make:controller` /
  `make:model` / `make:middleware` / `make:migration` scaffolding.
- **Tailwind CSS, built locally** — no CSS CDN at runtime.

## Getting Started

**Prerequisites:** Go 1.22+, a running MySQL server, Node.js + npm (for the
one-time CSS build).

```bash
# 1. Clone and enter the project
git clone <this-repo-url> vento-app
cd vento-app

# 2. Configure the environment
cp .env.example .env
# edit .env: DB_HOST / DB_USER / DB_PASSWORD / DB_NAME are required.
# APP_KEY is optional - set it (openssl rand -hex 32) to enable sessions.

# 3. Install the vento CLI locally (builds ./bin/vento)
go run setup.go 1

# 4. Build the Tailwind CSS bundle
npm install
npm run build:css

# 5. Create the database, migrate, and seed
./bin/vento db:migrate
./bin/vento db:seed

# 6. Run
./bin/vento run     # or: go run .
```

Open **http://localhost:8080**.

Don't want the CLI installed? Every command also works as
`go run ./vento/cmd/vento <command>`.

## Project Structure

```
vento-app/
├── vento/              # The framework: Engine, Context, router, security,
│                        # sessions, CORS, bind/validate, migrator, static
│   └── cmd/vento/       # The `vento` CLI (run, db:*, make:*)
├── app/
│   ├── controllers/    # Request handlers
│   ├── models/         # GORM data models + model registry
│   └── middleware/     # Your own middleware (e.g. RequestID)
├── routes/             # web.go (page routes) + api.go (/api routes)
├── migrations/         # Versioned, self-registering schema migrations
├── views/              # HTML templates (layouts + pages)
├── public/
│   ├── assets/          # Logo/icon + Tailwind source CSS
│   └── css/             # Compiled Tailwind output, served at /public
├── main.go             # Application bootstrap
└── setup.go             # Zero-config CLI installer
```

`main.go` is the one place everything is assembled: it loads `.env`,
connects MySQL, compiles templates, wires app-specific middleware, maps
every route table, and starts the server.

## Example: a minimal endpoint

```go
// app/controllers/hello_controller.go
func Hello(c *vento.Context) {
    name := c.Query("name")
    if name == "" {
        name = "World"
    }
    c.JSON(http.StatusOK, vento.H{"message": "Hello, " + name})
}

// routes/web.go
app.GET("/hello", controllers.Hello)
```

## Testing

```bash
go test ./...          # run the suite
go test ./... -race    # with the race detector
```

## License

No license file is currently included — add one before distributing this
project beyond internal use.
