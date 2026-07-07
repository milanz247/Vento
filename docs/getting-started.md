# Getting Started

This guide takes you from a fresh clone to a running server, then walks the
demo routes so you can see every framework feature working before you read
about how it's built.

## Prerequisites

- **Go 1.22 or newer** (the module targets Go 1.25 — see `go.mod`)
- **A running MySQL server** — MySQL is Vento's only supported database, and
  the app refuses to start without it rather than silently falling back to
  something else
- **Node.js + npm** — only to compile the Tailwind CSS bundle (step 3);
  there is no JavaScript build and no Node runtime dependency
- Optional: [air](https://github.com/air-verse/air) for hot reload during
  development (`go install github.com/air-verse/air@latest`)

## 1. Clone and configure

```bash
git clone <your-repo-url> vento-app
cd vento-app
```

Create a `.env` file in the project root (or edit the one already there):

```dotenv
DB_HOST=127.0.0.1
DB_PORT=3306
DB_USER=root
DB_PASSWORD=secret
DB_NAME=vento_app
```

`DB_HOST`, `DB_USER`, and `DB_NAME` are required; `DB_PORT` defaults to
`3306`. Create the database itself if it doesn't exist yet
(`CREATE DATABASE vento_app;`). Full parsing rules in
[Configuration](configuration.md); why startup is strict about this in
[Bootstrapping](bootstrapping.md#step-2--ventobuildmysqldsnenv).

## 2. Install the `vento` CLI (optional but recommended)

Vento ships a zero-config installer at the project root:

```bash
go run setup.go        # interactive prompt
go run setup.go 1      # non-interactive: local install  -> ./bin/vento
go run setup.go 2      # non-interactive: global install -> /usr/local/bin/vento
```

Local install is recommended: a per-project binary, no PATH changes. (The
binary can't live at the project root itself — the name `vento` is taken by
the framework package directory, hence `./bin/`.) Every command it provides
is in the [CLI Reference](cli-reference.md).

## 3. Build the front-end (Tailwind CSS)

The welcome page is styled with Tailwind, compiled locally — the only
runtime CDN dependency is the htmx script tag, which is integrity-pinned:

```bash
npm install
npm run build:css
```

This writes `public/css/app.css`, which `views/layouts/base.html` links and
`Engine.Static` serves at `/public/css/app.css`. During development, run
`npm run watch:css` in a second terminal to recompile on every template
edit. Details in [Front-end: Tailwind & Static Assets](frontend-tailwind.md).

## 4. Migrate the database

```bash
./bin/vento db:migrate
```

Applies every pending migration in `migrations/` — the starter ships one
that creates the `users` table — and records each in a `schema_migrations`
table, so it only ever runs migrations that haven't run yet. Safe to re-run
at any time. (While a model's shape is still changing, `./bin/vento
db:automigrate` is a quicker, untracked sync straight off `models.All()`.)

## 5. (Optional) seed sample data

```bash
./bin/vento db:seed
```

Inserts five sample users, keyed on email, so re-running never duplicates
rows. Seeder mechanics in [Database](database.md#seeders).

## 6. Run

```bash
./bin/vento run    # hot-reload via air when installed, plain `go run .` otherwise
# or directly:
go run .
```

You should see:

```
vento: listening on :8080
```

Open **http://localhost:8080**. Curious what just happened between those
two lines? That's the whole [Bootstrapping](bootstrapping.md) guide.

The starter ships with a single route — `GET /`, the welcome page — so you
begin from a clean slate rather than deleting demo code.

## Add your first route

Open `routes/web.go` and add a line inside the `web` function:

```go
app.GET("/hello", controllers.Hello)
```

Then add the handler in `controllers/home_controller.go`:

```go
// Hello handles GET /hello?name=Ada.
func Hello(c *vento.Context) {
    name := c.Query("name")
    if name == "" {
        name = "World"
    }
    c.JSON(http.StatusOK, map[string]string{"message": "Hello, " + name})
}
```

Save — `air` rebuilds automatically — and open
**http://localhost:8080/hello?name=Ada** to see `{"message":"Hello, Ada"}`.
That's the whole loop: a route in `web.go`, a handler in `controllers/`.

## Next steps

- [Project Structure](project-structure.md) — orient yourself in the codebase
- [Bootstrapping](bootstrapping.md) — understand the startup you just ran
- [Routing](routing.md) + [The Context API](context.md) — params, JSON, views, middleware
- [Database](database.md) — add a model and write a migration for it
- [Tutorial: Building a Todo List](tutorial-todo.md) — a full CRUD feature end to end
