# The Context API

Every middleware and controller in Vento has the same signature:

```go
func(c *vento.Context)
```

`*vento.Context` (defined in `vento/context.go`) is the single object through
which a handler reads the request and writes the response. This page is the
complete reference. For how a Context is created, injected, and recycled,
see [Architecture](architecture.md#context).

## The pooling contract

Contexts are recycled through a `sync.Pool` — a warm server allocates zero
Contexts per request. The framework guarantees a Context handed to your
handler is fully scrubbed (`Reset` clears every field when the Context
leaves the pool). In exchange, your code must honor one rule:

> **Never retain a `*Context`, or anything reached through it (the Request,
> the Writer, a params map), past the end of the request.** Copy values out
> if you need them later — e.g. in a goroutine.

## Raw access

| Field | Type | Notes |
|---|---|---|
| `c.Request` | `*http.Request` | The standard request. Use it for anything Vento doesn't wrap: headers, cookies, context, body streaming. |
| `c.Writer` | `http.ResponseWriter` | The standard writer. Prefer the response helpers below; write to this directly only when you need streaming or custom headers. |
| `c.StatusCode` | `int` | The status the response was written with. Set by the response helpers; read by `Logger` after the chain finishes. |

## Reading the request

### `c.Param(key string) string`

The value captured for a dynamic route segment. For a route `/users/:id`
and a request to `/users/42`, `c.Param("id")` returns `"42"`. Returns `""`
for keys the route didn't capture.

```go
app.GET("/users/:id/posts/:post_id", func(c *vento.Context) {
    userID := c.Param("id")
    postID := c.Param("post_id")
    ...
})
```

### `c.Query(key string) string`

A URL query parameter: for `/hello?name=Milan`, `c.Query("name")` returns
`"Milan"`, and `""` when absent.

### `c.FormValue(key string) string`

A POST/PUT form field, transparently parsing `application/x-www-form-urlencoded`
and `multipart/form-data` bodies on first access (it delegates to
`http.Request.FormValue`).

### JSON bodies

There is deliberately no `c.BindJSON`. Decode into a **dedicated input
struct** with the standard library, which doubles as mass-assignment
protection (clients can never set fields your struct doesn't declare):

```go
type createUserInput struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

var in createUserInput
if err := json.NewDecoder(c.Request.Body).Decode(&in); err != nil {
    c.Abort(http.StatusBadRequest, "invalid JSON body")
    return
}
```

Bodies are capped by the global `BodyLimit` middleware (1 MiB by default) —
a decode of an oversized body fails rather than buffering it. See
[Middleware](middleware.md#bodylimit).

### `c.IsHTMX() bool`

True when the request carries `HX-Request: true` — i.e. it was issued by
HTMX rather than a normal browser navigation. Used to branch one handler
between a full page and a partial fragment. See [HTMX](htmx.md).

## Writing the response

Each helper writes the status code, the appropriate `Content-Type`, and the
body, and records the status in `c.StatusCode`. Call exactly one of them
per request (headers can only be sent once).

### `c.JSON(statusCode int, data any)`

Marshals `data` and streams it with `Content-Type: application/json`.

```go
c.JSON(http.StatusOK, map[string]string{"message": "Hello, " + name})
c.JSON(http.StatusCreated, user)   // any marshalable value works
```

### `c.String(statusCode int, text string)`

Plain text with `Content-Type: text/plain`.

### `c.View(name string, data any)`

Renders the named page **inside the shared layout** with status 200.
`name` is the page's path relative to `views/` without the extension:
`c.View("index", data)` renders `views/index.html` stitched into
`views/layouts/base.html`. The stitching happened once, at startup
(`LoadHTMLGlob`) — this call is a single `ExecuteTemplate`. See
[Views & Templates](views-templates.md).

```go
c.View("index", map[string]any{
    "Message": "Welcome to Vento",
})
```

### `c.HTML(statusCode int, name string, data any)`

Same as `View` but with an explicit status code — `View` is literally
`c.HTML(http.StatusOK, name, data)`.

### `c.Partial(name string, data any)`

Renders **only the named view's `{{define "content"}}` block** — no layout,
no `<html>`, just the fragment. This is what an HTMX swap needs. Any view
under `views/` can be rendered this way, though fragment-only views
conventionally live in `views/partials/`:

```go
c.Partial("partials/todo_row", todo)
```

An unknown view name (for `View`/`HTML`/`Partial`) writes a 500 naming the
missing view and asking whether `LoadHTMLGlob` is configured.

### `c.Abort(statusCode int, msg string)`

Stops the handler chain immediately — no subsequent middleware or
controller runs — and writes `{"error": msg}` as JSON. This is both the
error-response helper *and* the chain short-circuit; internally it sets the
chain index past the end. Every built-in middleware rejects requests with
it (`429` from the rate limiter, `403` from CSRF, ...).

```go
if title == "" {
    c.Abort(http.StatusUnprocessableEntity, "title is required")
    return   // Abort doesn't panic; you still return yourself
}
```

## Infrastructure access

### `c.DB() *gorm.DB`

The Engine's GORM connection pool, injected before your handler ran. Query
directly:

```go
var users []models.User
if err := c.DB().Find(&users).Error; err != nil {
    log.Printf("listing users failed: %v", err)             // real error: server log
    c.Abort(http.StatusInternalServerError, "could not list users") // generic: client
    return
}
```

Keep the pattern above: **log the real error server-side, send a generic
message to the client** — raw GORM/SQL errors leak schema details.

### `c.Next()`

Advances to and runs the rest of the handler chain. **Only middlewares call
this** — a controller is the last element of its chain, so there is nothing
to advance to. Code placed after the `Next()` call runs when everything
downstream has finished, which is how before/after middleware works:

```go
func Timing(c *vento.Context) {
    start := time.Now()
    c.Next()                                  // ... rest of chain runs ...
    log.Printf("took %s", time.Since(start))  // runs after
}
```

## Quick reference

| Method | Direction | What |
|---|---|---|
| `Param(key)` | in | dynamic route segment capture |
| `Query(key)` | in | URL query parameter |
| `FormValue(key)` | in | form field (urlencoded/multipart) |
| `IsHTMX()` | in | request issued by HTMX? |
| `JSON(code, data)` | out | JSON body |
| `String(code, text)` | out | plain text |
| `View(name, data)` | out | page inside layout, 200 |
| `HTML(code, name, data)` | out | page inside layout, explicit status |
| `Partial(name, data)` | out | content block only, no layout |
| `Abort(code, msg)` | out + control | JSON error and stop the chain |
| `DB()` | infra | GORM connection pool |
| `Next()` | control | run the rest of the chain (middleware only) |
