# Middleware

A middleware in Vento is not a special type. It is any `vento.HandlerFunc` —
`func(*vento.Context)` — that calls `c.Next()` to hand control to the rest
of the chain. Controllers have the same signature; the only difference is
position (a controller is last, so it has nothing to call `Next()` for).
This page covers the chain model, every built-in middleware in detail, and
how to write your own.

## The chain model

At route-registration time, Vento flattens **global middlewares + route
middlewares + controller** into one `[]HandlerFunc` slice, stored on the
route's Trie node (see [Architecture](architecture.md#registration-compiling-chains)).
At request time, `c.Next()` simply walks that slice:

```go
func (c *Context) Next() {
    c.index++
    for c.index < len(c.handlers) {
        c.handlers[c.index](c)
        c.index++
    }
}
```

Three consequences:

- **Order is slice order.** Whatever order you pass to `Use` and to the
  route registration is exactly the execution order. Nothing reorders.
- **Code after `Next()` runs on the way back out.** A middleware brackets
  everything downstream of it — the basis for timing, logging, and
  recovery.
- **`Abort` ends the walk.** `c.Abort(code, msg)` sets the index past the
  end of the slice and writes a JSON error; no later handler runs.

```
request ──▶ Logger ▶ Recovery ▶ SecurityHeaders ▶ BodyLimit ▶ RateLimiter ▶ CSRF ▶ controller
response ◀── Logger ◀ Recovery ◀──────────────────────────────────────────────────────┘
              (post-Next code runs in reverse order)
```

## Global vs. route middleware

```go
app.Use(vento.Logger, vento.Recovery)               // global: every route registered after this
app.GET("/secret", controllers.SecretDemo,
        TokenAuthMiddleware)                      // route-specific: this route only
```

**`Use` must be called before the routes it should cover** — chains are
compiled per route at registration, so a later `Use` cannot retrofit
earlier routes. The idiomatic layout (`routes/web.go` calls `Use` first,
then maps endpoints) satisfies this naturally.

## Writing your own middleware

Your own middleware lives in the **`middleware/` package** — Vento's
equivalent of Laravel's `app/Http/Middleware`, kept separate from the
framework's built-ins under `vento/`. Scaffold one with:

```bash
./bin/vento make:middleware RequireAuth   # -> middleware/require_auth.go
```

A plain middleware is just a `func(*vento.Context)` that ends in `c.Next()`.
The shipped example, `middleware/request_id.go`, stamps a trace ID onto
every request and response:

```go
// middleware/request_id.go
func RequestID(c *vento.Context) {
    id := c.Request.Header.Get("X-Request-ID")
    if id == "" {
        id = newID()
        c.Request.Header.Set("X-Request-ID", id)
    }
    c.Writer.Header().Set("X-Request-ID", id)
    c.Next()
}
```

Register it in `routes/web.go` — globally via `app.Use(middleware.RequestID)`
(as the demo does), or per route as a trailing argument to
`app.GET`/`app.POST`/...

A **configurable** middleware is a factory: a function that runs once (at
registration) and returns the closure that runs per request. Expensive
setup — allocating a bucket map, compiling a list — belongs in the factory,
outside the closure. This is exactly how `RateLimiter`, `BodyLimit`, and
`CSRFProtection` are built:

```go
func MaxDuration(d time.Duration) vento.HandlerFunc {   // runs once
    return func(c *vento.Context) {                     // runs per request
        start := time.Now()
        c.Next()
        if time.Since(start) > d {
            log.Printf("slow request: %s %s", c.Request.Method, c.Request.URL.Path)
        }
    }
}

app.Use(MaxDuration(2 * time.Second))
```

A gatekeeping middleware rejects with `Abort` and **must `return` after
it** (Abort stops the chain, not your function):

```go
func RequireJSON(c *vento.Context) {
    if c.Request.Header.Get("Content-Type") != "application/json" {
        c.Abort(http.StatusUnsupportedMediaType, "expected application/json")
        return
    }
    c.Next()
}
```

## Built-in middleware

### Logger

`vento/middlewares.go`. Times the rest of the chain and logs method, path,
final status, and latency:

```
[vento] GET /users 200 1.2ms
```

Register it **first** so it wraps everything — including `Recovery`, which
means even a recovered panic still produces a timed log line. It reads
`c.StatusCode`, which every response helper records.

### Recovery

`vento/middlewares.go`. A deferred `recover()` around the rest of the
chain. On panic: logs the stack trace server-side, responds
`500 {"error": "Internal Server Error"}` — the process survives and the
panic details never reach the client. `GET /panic` in the demo app
dereferences a nil pointer specifically so you can watch this work.

Without Recovery, a panic escapes the chain entirely; the Engine then
deliberately discards the Context rather than repooling a possibly-corrupt
instance.

### SecurityHeaders

`vento/security.go`. Stamps on every response, before anything is written:

| Header | Value | Purpose |
|---|---|---|
| `X-Frame-Options` | `DENY` | clickjacking |
| `X-Content-Type-Options` | `nosniff` | MIME sniffing |
| `Referrer-Policy` | `strict-origin-when-cross-origin` | referrer leakage |
| `X-XSS-Protection` | `0` | explicitly disables the legacy browser XSS auditor, per current OWASP guidance — it's gone from modern browsers and was itself exploitable in old ones. XSS defense comes from `html/template` auto-escaping. |

### BodyLimit

`vento/security.go`.

```go
func BodyLimit(maxBytes int64) HandlerFunc
app.Use(vento.BodyLimit(1 << 20))   // 1 MiB
```

Wraps the request body in `http.MaxBytesReader`: any downstream read past
the cap — `json.NewDecoder`, `ParseForm`, `io.ReadAll` — fails with an
error instead of buffering an unbounded body into memory, and the
connection is closed. For a route that genuinely accepts large uploads,
register a route-specific `BodyLimit` with a higher cap there.

### RateLimiter

`vento/security.go`.

```go
func RateLimiter(rps float64, burst float64) HandlerFunc
app.Use(vento.RateLimiter(10, 20))   // 10 req/s sustained, bursts of 20
```

A **per-client-IP token bucket**: each IP accrues `rps` tokens per second
up to `burst`; each request spends one; an empty bucket gets
`429 Too Many Requests` with `Retry-After: 1`. Implementation notes:

- Buckets live in a `sync.Map` (IP → `*bucket`); each bucket has its own
  mutex, so refill/spend is atomic per client without a global lock.
- Refill is computed lazily from elapsed time — no background ticker.
- Idle buckets (3+ minutes) are purged opportunistically, at most once a
  minute, so the map stays bounded even under address-spoofed floods.

The key is `RemoteAddr`. **Behind a reverse proxy every client shares the
proxy's IP** — either rate-limit at the proxy, or extend this to trust
`X-Forwarded-For` from known proxy addresses only (trusting it blindly
lets any client spoof past the limiter).

### CSRFProtection

`vento/security.go`.

```go
func CSRFProtection(exemptPrefixes ...string) HandlerFunc
app.Use(vento.CSRFProtection("/users"))
```

Double-submit-cookie CSRF protection:

- **Safe methods** (`GET`/`HEAD`/`OPTIONS`/`TRACE`) pass through; if the
  client has no token yet, a 32-byte `crypto/rand` token is set as the
  `vento_csrf` cookie — `SameSite=Lax`, `Secure` automatically when the
  request arrived over TLS, and deliberately **not** `HttpOnly` so
  front-end JS can read it back.
- **Unsafe methods** must echo the cookie's value in the `X-CSRF-Token`
  header or a `_csrf` form field. Comparison is constant-time
  (`crypto/subtle.ConstantTimeCompare`); missing or wrong → `403`.
- `exemptPrefixes` skips validation for path prefixes — appropriate for
  token-authenticated JSON APIs like `POST /users`, which browsers never
  attach ambient credentials to automatically.

How HTMX requests pass this automatically is covered in
[HTMX § CSRF](htmx.md#htmx-and-csrf).

## The default chain

```go
app.Use(
    vento.Logger,              // timing + audit line for everything below
    vento.Recovery,            // panics become clean 500s
    middleware.RequestID,      // your own middleware: X-Request-ID for tracing
    vento.SecurityHeaders,     // headers stamped before any body write
    vento.BodyLimit(1<<20),    // bodies capped before any handler reads
    vento.RateLimiter(10, 20), // floods rejected early
    vento.CSRFProtection(),    // browser form posts guarded (exempt prefixes go here)
)
```

`middleware.RequestID` is application code (the `middleware/` package); every
`vento.*` entry is a framework built-in. Read outermost-first, and note the
deliberate ordering: cheap, broad
protections run before expensive or request-specific ones, so
obviously-bad traffic is rejected with minimum work. Keep that convention
when inserting your own.
