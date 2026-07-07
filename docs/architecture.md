# Architecture

This page explains how Vento is built: the three core types (`Engine`,
`Context`, the Trie router), how they cooperate on each request, and the
performance model that shapes every design decision. The whole framework is
seven files under `vento/` — this page maps them.

| File | Owns |
|---|---|
| `vento/engine.go` | `Engine`: registration, template compilation, `ServeHTTP`, dispatch, the hardened server |
| `vento/context.go` | `Context`: the per-request object every handler receives |
| `vento/router.go` | The per-method Trie: `node`, insert, search |
| `vento/middlewares.go` | `Logger`, `Recovery` |
| `vento/security.go` | `SecurityHeaders`, `BodyLimit`, `RateLimiter`, `CSRFProtection` |
| `vento/static.go` | Static file mounts |
| `vento/config.go` | `.env` loading, MySQL DSN assembly |

## The guiding principle: move work to startup

Vento's performance model is one sentence: **anything that *can* be computed
at startup *is* computed at startup, so the request path only executes
pre-built artifacts.** Concretely, three things are precomputed:

1. **Handler chains.** Global middlewares + route middlewares + controller
   are flattened into one `[]HandlerFunc` per route, at registration time,
   and stored on the route's Trie node. Serving never assembles a chain.
2. **Templates.** Every page is parsed and stitched into the shared layout
   once, by `LoadHTMLGlob`. Rendering is a single `ExecuteTemplate` call.
3. **Contexts.** Not precomputed, but *recycled*: a `sync.Pool` means a
   warm server allocates zero Contexts per request.

The cost of this model is that **startup order matters** — `Use` before
routes, `Static` after routes. The [Bootstrapping](bootstrapping.md) guide
covers the exact sequence and the reasons.

## Engine

```go
type Engine struct {
    router      *router
    middlewares []HandlerFunc // globals, snapshotted into each route's chain
    pool        sync.Pool     // recycled *Context instances

    DB        *gorm.DB            // injected into every Context
    templates map[string]*viewSet // view name -> pre-stitched template set
    statics   []staticMount       // prefix -> pre-compiled handler chain
}
```

`Engine` is the coordinator, and it earns its keep by implementing
**`http.Handler`**. That single interface is the framework's only contract
with the outside world: an `*Engine` plugs into `http.Server`,
`httptest.Server`, or any standard mux with zero adapter code. `Engine.Run`
is a convenience that wraps it in an `http.Server` with hardened timeouts
(`ReadHeaderTimeout` 5s, `ReadTimeout`/`WriteTimeout` 30s, `IdleTimeout`
120s — see [Security](security.md)); anything fancier is the application's
choice to build.

### Registration: compiling chains

`GET`/`POST`/`PUT`/`DELETE` all funnel into `addRoute`:

```go
func (e *Engine) addRoute(method, path string, handler HandlerFunc, middlewares []HandlerFunc) {
    chain := make([]HandlerFunc, 0, len(e.middlewares)+len(middlewares)+1)
    chain = append(chain, e.middlewares...)   // globals first
    chain = append(chain, middlewares...)     // then route-specific
    chain = append(chain, handler)            // controller last
    e.router.addRoute(method, path, chain)
}
```

Note what is *absent*: no linked list of wrappers, no `next http.Handler`
closures, no per-request composition. The chain is a flat slice, built
once. Middleware execution order is therefore trivially predictable — it's
the slice order — and coverage is auditable by reading the registration
code.

### Dispatch: the request hot path

```go
func (e *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    if handlers := e.matchStatic(r.URL.Path); handlers != nil {
        e.dispatch(w, r, handlers, nil)
        return
    }
    matched, params := e.router.getRoute(r.Method, r.URL.Path)
    if matched == nil {
        http.NotFound(w, r)
        return
    }
    e.dispatch(w, r, matched.handlers, params)
}
```

Static mounts are checked first (simple prefix scan — there is typically
one), then the Trie. Both roads lead to `dispatch`:

```go
func (e *Engine) dispatch(w http.ResponseWriter, r *http.Request, handlers []HandlerFunc, params map[string]string) {
    ctx := e.pool.Get().(*Context)
    ctx.Reset(w, r)                  // scrub previous request's state
    ctx.params = params
    ctx.handlers = handlers
    ctx.db = e.DB                    // dependency injection, the boring way
    ctx.templates = e.templates

    ctx.Next()                       // run the chain

    e.pool.Put(ctx)                  // recycle
}
```

Two subtleties:

- **`Reset` before use, not after.** The Context is scrubbed when taken
  from the pool, so even if a previous cycle ended abnormally, stale state
  never reaches a handler.
- **Panic behavior.** If a panic escapes the entire chain (meaning
  `Recovery` wasn't installed), `pool.Put` never runs and the
  possibly-corrupt Context is left to the GC. With `Recovery` installed
  (the default), the panic is caught inside the chain and recycling
  proceeds normally.

## Context

`Context` (`vento/context.go`) is the one object handlers see. It wraps the
`http.ResponseWriter`/`*http.Request` pair and carries the injected
dependencies:

```go
type Context struct {
    Writer     http.ResponseWriter
    Request    *http.Request
    StatusCode int

    params   map[string]string   // :param captures from the router
    handlers []HandlerFunc       // the compiled chain being executed
    index    int                 // position in that chain; starts at -1

    db        *gorm.DB           // injected by dispatch
    templates map[string]*viewSet
}
```

The full method-by-method reference is in [The Context API](context.md);
what matters architecturally is the **chain walker**:

```go
func (c *Context) Next() {
    c.index++
    for c.index < len(c.handlers) {
        c.handlers[c.index](c)
        c.index++
    }
}
```

Every handler in the chain is called with the same Context. A middleware
that calls `c.Next()` yields to the rest of the chain and regains control
afterward — which is how `Logger` measures latency (code after its `Next()`
runs last) and how `Recovery`'s deferred `recover()` wraps everything
downstream. `Abort(code, msg)` short-circuits by setting
`c.index = len(c.handlers)`, so the loop terminates and no further handler
runs.

Because Contexts are pooled, one rule is absolute: **never retain a
`*Context` (or anything reached through it) past the end of the request.**
Copy out what you need.

## The router

`vento/router.go` implements one **Trie (prefix tree) per HTTP method**:

```go
type router struct {
    roots map[string]*node   // "GET" -> tree, "POST" -> tree, ...
}

type node struct {
    path     string        // one segment: "users" or ":id"
    children []*node
    isWild   bool          // ":name" parameter segment?
    handlers []HandlerFunc // compiled chain; non-nil only on terminal nodes
}
```

A route decomposes into segments — `/users/:id/posts/:post_id` becomes
`users → :id → posts → :post_id` — and `insert` walks/creates that chain,
storing the compiled handler slice on the terminal node.

### Matching precedence and backtracking

`search` resolves a request path recursively, and its ordering encodes the
precedence rule: **at every level, static children are tried before the
wildcard child, with clean backtracking between them.**

```go
// static children first...
for _, child := range n.children {
    if child.isWild || child.path != segment { continue }
    if found := child.search(segments, depth+1, params); found != nil {
        return found
    }
}
// ...then the wildcard
for _, child := range n.children {
    if !child.isWild { continue }
    if found := child.search(segments, depth+1, params); found != nil {
        params[child.path[1:]] = segment   // capture, e.g. params["id"] = "42"
        return found
    }
}
```

So `/users/me` (literal) and `/users/:id` (wildcard) coexist, with the
literal winning for `GET /users/me`. And because a failed static descent
*backtracks* and still tries the wildcard, a route set like `/users/me/x`
plus `/users/:id/y` resolves `/users/me/y` correctly — the static `me`
branch dead-ends at `y`, the search backtracks, and the `:id` branch
matches with `id="me"`.

Parameters are captured on the way *back up* the successful recursion,
straight into the map that becomes `c.Param()`.

Method isolation (one tree per method) means `GET /users` and
`POST /users` are unrelated registrations. A request whose method has no
tree — e.g. `HEAD`, which the demo app never registers — is simply a 404.

## The request lifecycle, end to end

```
http.Server (Engine.Run: hardened timeouts)
        │
        ▼
 Engine.ServeHTTP
        │  matchStatic(path) → pre-compiled static chain, if a prefix matches
        │  otherwise: router.getRoute(method, path) → (node, params) or 404
        ▼
 dispatch: ctx = pool.Get(); ctx.Reset(w, r); inject params/chain/DB/templates
        ▼
 ctx.Next() ──▶ Logger ──▶ Recovery ──▶ SecurityHeaders ──▶ BodyLimit
                                                              │
                                    CSRFProtection ◀── RateLimiter
                                          │
                                          ▼
                              route middlewares (if any)
                                          │
                                          ▼
                                     controller
                                          │
                    response written via c.JSON / c.View / c.Partial / ...
        ▼
 pool.Put(ctx)
```

Every arrow in the middleware row is just "next element of a slice". The
entire framework overhead per request is: one prefix scan, one Trie walk,
one pool get/put, one slice iteration.

## Design trade-offs, stated honestly

- **Registration-time chains** buy zero-allocation dispatch and auditable
  ordering, at the cost of no dynamic middleware (you cannot add a
  middleware to existing routes at runtime).
- **Per-page template clones** buy conflict-free `{{define "content"}}`
  blocks and zero request-time parsing, at the cost of memory
  proportional to pages × layout size, and pages not being able to include
  each other.
- **One database, one provider** buys a ten-line config surface, at the
  cost of a fork if you need Postgres.
- **`params` as a `map[string]string`** is allocated per request with a
  captured route — the one deliberate allocation kept for API simplicity.

## Related reading

- [Bootstrapping](bootstrapping.md) — the startup sequence that builds everything this page described
- [Routing](routing.md) — Trie behavior with worked examples
- [The Context API](context.md) — every method on `*vento.Context`
- [Middleware](middleware.md) — the built-ins and the chain model in practice
