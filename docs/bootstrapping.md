# Bootstrapping

This guide walks through everything that happens between typing `go run .`
and the server printing `vento: listening on :8080` — line by line, with the
reasons behind the ordering. If you understand this page, you understand
half the framework, because Vento deliberately front-loads work into startup
so the request path stays minimal.

## The entry point

The whole application bootstrap is `main.go`, and it is intentionally thin —
six framework calls, in a specific order:

```go
func main() {
    env := vento.LoadEnv(".env")                    // 1. configuration

    dsn, ok := vento.BuildMySQLDSN(env)             // 2. validate DB config
    if !ok {
        log.Fatal("vento: DB_HOST/DB_USER/DB_NAME missing from .env - MySQL configuration is required")
    }

    app := vento.New()                              // 3. create the Engine

    if err := app.ConnectDB(dsn); err != nil {     // 4. connect MySQL
        log.Fatalf("vento: could not connect to MySQL: %v", err)
    }

    app.LoadHTMLGlob("views/**/*")                 // 5. compile all templates

    routes.RegisterRoutes(app)                     // 6. middlewares + routes

    app.Static("/public", "./public")              // 7. static mounts (after routes!)

    if err := app.Run(":8080"); err != nil {       // 8. serve
        log.Fatal(err)
    }
}
```

Nothing about *how* a request is handled lives here. Handlers live in
`controllers/`, route wiring in `routes/`, schema management in the CLI.
`main.go` only sequences the boot. Each step below explains what actually
happens inside the framework.

## Step 1 — `vento.LoadEnv(".env")`

`vento/config.go`. Reads a plain `KEY=VALUE` file:

- Blank lines and `#` comments are skipped.
- Each value is trimmed and stripped of surrounding quotes.
- Every pair is returned in a `map[string]string` **and** exported into the
  process environment via `os.Setenv`, so application code can use either
  the returned map or plain `os.Getenv`.
- **A missing file is not an error** — it yields an empty map. This is
  deliberate: in containerized deployments configuration usually arrives as
  real environment variables, and there is no `.env` file at all.

## Step 2 — `vento.BuildMySQLDSN(env)`

Also `vento/config.go`. Assembles the DSN string
`user:pass@tcp(host:port)/name?charset=utf8mb4&parseTime=True&loc=Local`
from the discrete `DB_*` keys, so no raw DSN is ever hand-written or
committed. It returns `ok=false` when `DB_HOST`, `DB_USER`, or `DB_NAME` is
missing (`DB_PORT` defaults to `3306`), and `main.go` treats that as fatal:
**Vento refuses to start half-configured** rather than limping along and
failing on the first query.

## Step 3 — `vento.New()`

`vento/engine.go`. Constructs the `Engine`:

```go
e := &Engine{router: newRouter()}
e.pool.New = func() any { return &Context{} }
```

Two things are born here:

- **The router** — an empty map of HTTP method → Trie root. Roots are
  created lazily on the first route registered for each method.
- **The Context pool** — a `sync.Pool` whose `New` function allocates a
  blank `*Context`. Early requests may trigger a few allocations; once the
  pool is warm, every request reuses a recycled Context and the allocation
  disappears from the GC's workload entirely.

At this point the Engine is inert: no routes, no DB, no templates.

## Step 4 — `app.ConnectDB(dsn)`

Opens a GORM connection pool against MySQL and stores it on `Engine.DB`.
Every request's Context gets this same `*gorm.DB` injected before its
handler chain runs (see the dispatch section below), which is what makes
`c.DB().Find(&users)` work inside any controller with zero setup.

A connection failure aborts startup. MySQL is Vento's sole provider by
design — swapping providers means editing this one function.

## Step 5 — `app.LoadHTMLGlob("views/**/*")`

`vento/engine.go`. This is the most involved startup step, and the reason
controllers never touch template parsing:

1. The directory root is derived from the pattern (everything before the
   first `*` → `views/`).
2. The tree is walked once. Every `.html` file is classified:
   - under `views/layouts/` → part of the **layout set**
   - anywhere else → a **page** (this includes `views/partials/`)
3. The layout files are parsed together into one template set. The
   document entry point is `base.html` if a layout with that name exists,
   otherwise the first layout parsed.
4. **Each page is compiled as its own clone of the layout set**, then the
   page file is parsed into that clone. Cloning is the trick that lets
   every page define a block with the same name — `{{define "content"}}` —
   without colliding, because each page lives in its own namespace.
5. The result is stored in `Engine.templates`, a map keyed by the page's
   path relative to `views/` without the extension: `views/index.html` →
   `"index"`, `views/users/show.html` → `"users/show"`.

Any parse error **panics immediately**, with the offending file named. A
template typo should kill the boot, not surface as a runtime 500 three days
later.

Consequence worth knowing: because each page is an independent clone, one
page cannot `{{template}}` another page's content — pages share *layouts*,
not each other. The [HTMX guide](htmx.md) explains how this interacts with
partials.

## Step 6 — `routes.RegisterRoutes(app)`

`routes/web.go`. Two phases, and **their order is load-bearing**:

```go
app.Use(                       // phase 1: global middlewares
    vento.Logger,
    vento.Recovery,
    middleware.RequestID,      // your own middleware (the middleware/ package)
    vento.SecurityHeaders,
    vento.BodyLimit(1<<20),
    vento.RateLimiter(10, 20),
    vento.CSRFProtection(),
)

app.GET("/", controllers.Index)   // phase 2: endpoints
// ...
```

Here is why order matters. `Use` just appends to `Engine.middlewares`. But
each `GET`/`POST`/... call runs `addRoute`, which **compiles the route's
complete chain right then**:

```go
chain := append(append(append(
    make([]HandlerFunc, 0, ...),
    e.middlewares...),      // globals, snapshotted NOW
    middlewares...),        // route-specific
    handler)                // the controller, last
e.router.addRoute(method, path, chain)
```

The chain is a flat `[]HandlerFunc` stored on the route's Trie node. A
middleware registered *after* a route simply is not in that route's slice —
there is no late binding, no lookup at request time. This is a deliberate
trade: it removes all per-request chain assembly, and it makes middleware
coverage auditable by reading `routes/web.go` top to bottom.

The `RateLimiter` and `CSRFProtection` entries are factory calls — functions
that return a `HandlerFunc` closed over their configuration (bucket map,
exemption list). They run **once**, here; only the returned closures run per
request. See [Middleware](middleware.md#writing-your-own-middleware).

## Step 7 — `app.Static("/public", "./public")`

`vento/static.go`. Registers a static file mount, and — exactly like a route
— compiles its chain (all current global middlewares + a `http.FileServer`
wrapper) at registration time. **This is why `Static` must come after
`RegisterRoutes`**: called earlier, `e.middlewares` would still be empty and
static responses would bypass `Logger`, `SecurityHeaders`, the rate limiter,
everything.

## Step 8 — `app.Run(":8080")`

`vento/engine.go`. Builds a hardened `http.Server` and starts serving:

```go
srv := &http.Server{
    Addr:              addr,
    Handler:           e,                 // Engine implements http.Handler
    ReadHeaderTimeout: 5 * time.Second,
    ReadTimeout:       30 * time.Second,
    WriteTimeout:      30 * time.Second,
    IdleTimeout:       120 * time.Second,
}
return srv.ListenAndServe()
```

The timeouts replace the standard library's *unlimited* defaults, so a
client that connects and stalls (slow-loris style) cannot pin a connection
forever. Applications needing different values (long-polling, streaming)
can skip `Run` entirely and build their own `http.Server` with the Engine
as `Handler` — the Engine is a plain `http.Handler` and doesn't care who
serves it. The same property is what makes it directly usable with
`httptest.Server` in tests.

## After boot: what a request costs

Everything above ran once. From now on, each request does only this
(`Engine.ServeHTTP` → `dispatch` in `vento/engine.go`):

```
request
  │
  ├─ static mount prefix match?  ──yes──▶ pre-compiled static chain
  │            no
  ▼
  Trie lookup (method root → segment walk) → pre-compiled route chain + params
  │
  ▼
  ctx := pool.Get()          // recycled Context, no allocation when warm
  ctx.Reset(w, r)            // scrub all previous-request state
  ctx.params, ctx.handlers = params, chain
  ctx.db, ctx.templates = e.DB, e.templates   // injection, not lookup
  │
  ▼
  ctx.Next()                 // walk the chain: middlewares … controller
  │
  ▼
  pool.Put(ctx)              // recycle
```

`Reset` clears every per-request field before reuse, so a recycled Context
can never leak one client's data to another. If a panic escapes the whole
chain (i.e. `Recovery` was not installed), the Context is deliberately
*not* repooled — a possibly-corrupt instance is left to the GC instead.

## Boot-order rules, summarized

| Rule | Because |
|---|---|
| `Use` before any route | chains are compiled at route registration; later `Use` calls don't retrofit |
| `LoadHTMLGlob` before serving | `c.View`/`c.Partial` look up the pre-stitched cache; an empty cache is a 500 |
| `Static` after `RegisterRoutes` | static chains snapshot the global middlewares at call time |
| `ConnectDB` before serving | `Engine.DB` is injected into every Context; nil means `c.DB()` panics downstream |
| Config/DB failures are fatal | a half-booted server that 500s on every request is worse than no server |

## The CLI's variant of the same boot

`./bin/vento db:migrate` and `db:seed` (see [CLI Reference](cli-reference.md))
perform a *partial* bootstrap: `LoadEnv` → `BuildMySQLDSN` → `New` →
`ConnectDB`, then use `Engine.DB` directly — no templates, no routes, no
server. That the CLI can reuse the exact same steps is a direct consequence
of the boot being plain, ordered function calls rather than framework magic.
