# Routing

Vento routes with a hand-written **Trie (prefix tree) router** — one tree per
HTTP method, dynamic `:param` segments, and static-beats-wildcard matching
with backtracking. This page covers the registration API, the matching
rules with worked examples, and the internals in `vento/router.go`.

## Registering routes

All registration happens in `routes/web.go`, inside `RegisterRoutes`. The
starter ships with just the welcome route:

```go
app.GET("/", controllers.Index)
```

You add your own alongside it — the examples below show the full range of
the API:

```go
app.GET("/hello", controllers.Hello)
app.POST("/submit", controllers.Submit)

app.POST("/users", controllers.CreateUser)
app.GET("/users", controllers.ListUsers)
app.GET("/users/:id", controllers.ShowUser)
app.GET("/users/:id/posts/:post_id", controllers.ShowUserPost)

app.GET("/secret", controllers.Secret, TokenAuthMiddleware)
```

`GET`, `POST`, `PUT`, and `DELETE` are available, all with the same shape:

```go
app.GET(path string, handler vento.HandlerFunc, middlewares ...vento.HandlerFunc)
```

Trailing arguments are **route-specific middlewares**, which run after the
global chain and before the controller — `/secret` above is guarded by
`TokenAuthMiddleware` without affecting any other route.

Registration must come **after** `app.Use(...)`: each route's full handler
chain (globals + route middlewares + controller) is compiled at this moment
and stored on the route's Trie node. See
[Bootstrapping](bootstrapping.md#step-6--routesregisterroutesapp) for why.

## Dynamic parameters

A segment starting with `:` matches any single segment and captures it:

```go
app.GET("/users/:id", controllers.ShowUser)
// GET /users/42        → c.Param("id") == "42"
// GET /users/abc       → c.Param("id") == "abc"
// GET /users/42/extra  → 404 (:id matches exactly one segment)
```

Captures nest:

```go
app.GET("/users/:id/posts/:post_id", controllers.ShowUserPost)
// GET /users/7/posts/99 → c.Param("id") == "7", c.Param("post_id") == "99"
```

There is no catch-all/`*wildcard` segment and no regex constraint —
validate parameter shape in the handler (or a route middleware) instead.

## Matching rules

1. **Method trees are independent.** `GET /users` and `POST /users` are
   separate registrations in separate trees; registering one does not
   create the other. An unregistered method — including `HEAD` — is a 404.
2. **Static beats wildcard at every level.** If both `/users/me` and
   `/users/:id` are registered, `GET /users/me` hits the literal route;
   `GET /users/42` falls through to `:id`.
3. **Backtracking is clean.** If a static branch matches part-way and then
   dead-ends, the search backs up and tries the wildcard branch. Given
   `/users/me/settings` and `/users/:id/posts`, a request for
   `/users/me/posts` first descends the static `me` branch, fails at
   `posts`, backtracks, and matches `:id/posts` with `id="me"`.
4. **Slashes normalize.** Paths split on `/` with empty segments dropped,
   so `/users`, `/users/`, and `//users` all resolve identically.
5. **A prefix is not a match.** Matching must consume the whole path *and*
   end on a node where a route was actually registered. With only
   `/users/:id/posts/:post_id` registered, `GET /users/1/posts` is a 404 —
   `posts` is an intermediate node with no handlers.

## How the Trie works

Registration decomposes the path into segments and walks/creates one node
per segment:

```
app.GET("/users/:id", h1)          GET tree:
app.GET("/users/me", h2)               "/"
app.GET("/users/:id/posts", h3)         └── users
                                          ├── me        [h2's chain]
                                          └── :id       [h1's chain]
                                               └── posts [h3's chain]
```

```go
type node struct {
    path     string        // one segment: "users" or ":id"
    children []*node
    isWild   bool          // is this a ":name" segment?
    handlers []HandlerFunc // compiled chain; non-nil only where a route ends
}
```

The compiled handler chain lives directly on the terminal node — matching a
route yields the exact slice to execute, with no further lookup, assembly,
or allocation. That's the "zero-allocation matching" claim: the only
allocation on a matched request is the `params` map itself.

Lookup (`getRoute`) splits the request path once, then `search` recurses
down the tree trying static children first, wildcard child second (rules 2
and 3 above are literally the order of two loops — see
[Architecture](architecture.md#the-router) for the code). Parameter values
are recorded as the successful recursion unwinds, producing the map that
handlers read through `c.Param`.

## Route middleware in practice

A route middleware is just a `HandlerFunc` you keep next to the routes —
for example, add this to `routes/web.go`:

```go
// TokenAuthMiddleware is routing policy, so it lives in routes/web.go.
func TokenAuthMiddleware(c *vento.Context) {
    if c.Query("token") != "secret" {
        c.Abort(http.StatusUnauthorized, "Unauthorized")
        return
    }
    c.Next()
}

app.GET("/secret", controllers.Secret, TokenAuthMiddleware)
```

The compiled chain for `/secret` is then:

```
Logger → Recovery → SecurityHeaders → BodyLimit → RateLimiter → CSRFProtection
    → TokenAuthMiddleware → Secret
```

`Abort` inside the token check stops the chain before `Secret` runs.
(A real application should carry tokens in an `Authorization` header, not a
query string — query strings land in access logs. This is just to
illustrate route-scoped middleware.)

## Static file mounts vs. routes

`app.Static("/public", "./public")` registers a *prefix* mount, checked
**before** the Trie in `ServeHTTP`. Pick a prefix that no route uses. Like
routes, static mounts compile their middleware chain at registration time —
call `Static` after `Use`. Details in
[Front-end & Static Assets](frontend-tailwind.md).

## Example route map

The starter ships only `GET /` (the welcome page). The routes below are
**examples** — patterns you can build on top of it, each demonstrating one
framework capability:

| Route | Demonstrates |
|---|---|
| `GET /` | View rendering with the shared layout (shipped) |
| `GET /hello?name=X` | Query parameters, JSON response |
| `POST /submit` | Form binding behind CSRF |
| `POST /users` | JSON body binding, input structs |
| `GET /users` | GORM list query |
| `GET /users/:id` | Single dynamic parameter |
| `GET /users/:id/posts/:post_id` | Nested dynamic parameters |
| `GET /secret?token=secret` | Route-specific middleware |

The [Todo tutorial](tutorial-todo.md) builds a real feature that ties these
patterns together.
