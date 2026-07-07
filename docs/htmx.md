# Reactive UIs with HTMX

Vento supports Livewire/Hotwire-style reactive interfaces — DOM updates
without full-page reloads and without a client-side framework — by pairing
[HTMX](https://htmx.org) with two `Context` additions: `IsHTMX()` and
`Partial()`. This page explains the building blocks and how to bridge CSRF.

HTMX is **opt-in**: the default welcome page is a plain server-rendered
view, so nothing about HTMX is loaded until you add it. The
[Todo tutorial](tutorial-todo.md) builds a complete HTMX-driven feature end
to end; this page is the reference for the pieces it uses.

## The idea in one round trip

1. An element in the page declares intent with `hx-*` attributes:
   *"on click, POST /todos, and swap the response into `#todo-list`"*.
2. The handler does its work, then — instead of a full page — responds with
   **just the fragment** of HTML that changed.
3. HTMX swaps that fragment into the DOM. No reload, no JSON-to-DOM
   client code, no build step.

The server stays the single source of truth for markup; the browser is a
thin patching layer.

## The building blocks

### `Context.IsHTMX() bool`

Reports whether the request was issued by HTMX rather than a normal
browser navigation, by checking the `HX-Request: true` header HTMX
attaches to every request it makes:

```go
func (c *Context) IsHTMX() bool {
    return c.Request.Header.Get("HX-Request") == "true"
}
```

Use it to branch a single handler between a fragment and whatever a
non-HTMX caller should get:

```go
func StoreTodo(c *vento.Context) {
    // ...create the todo...
    if c.IsHTMX() {
        c.Partial("partials/todo_row", todo)   // fragment for the swap
        return
    }
    c.JSON(http.StatusOK, todo)                // curl / JSON clients
}
```

### `Context.Partial(name string, data any)`

Where `c.View` renders a page wrapped in the shared layout, `c.Partial`
executes **only** the view's `{{define "content"}}` block — no `<html>`,
no nav, no layout. That is exactly the DOM fragment HTMX needs for an
`hx-target`/`hx-swap`. It reads the same pre-stitched template cache
`LoadHTMLGlob` built at startup — no extra registration, no request-time
parsing ([Views & Templates](views-templates.md) covers the cache).

## Wiring HTMX into your layout

Because HTMX is opt-in, add its script to your layout (or the specific
page that needs it) when you start building reactive features:

```html
<script src="https://unpkg.com/htmx.org@1.9.12"
        integrity="sha384-ujb1lZYygJmzgSwoxRggbCHcjc0rB2XoQrxeTUQyRjrOnlCoYta87iKBWq3EsdM2"
        crossorigin="anonymous"></script>
```

The `integrity` hash pins the exact htmx artifact so a compromised CDN
response is rejected — see [Security](security.md).

## HTMX and CSRF

`vento.CSRFProtection` issues a `vento_csrf` cookie — deliberately *not*
`HttpOnly`, so JavaScript can read it — and requires non-idempotent
requests to echo its value in the `X-CSRF-Token` header
([Middleware § CSRFProtection](middleware.md#csrfprotection)). HTMX doesn't
do that by itself, so add a small bridge immediately after loading HTMX:

```html
<script>
document.addEventListener('htmx:configRequest', function (evt) {
    var match = document.cookie.match(/(?:^|;\s*)vento_csrf=([^;]+)/);
    if (match) {
        evt.detail.headers['X-CSRF-Token'] = match[1];
    }
});
</script>
```

`htmx:configRequest` fires before every HTMX request, so every
`hx-post`/`hx-put`/`hx-delete` on any page picks up the token
automatically — no hidden field per form. An HTMX endpoint therefore needs
no CSRF exemption: the initial `GET` sets the cookie, and every HTMX
request echoes it back.

## Adding your own HTMX-driven endpoint

1. Add the `hx-*` attributes to the triggering element in a view:
   `hx-post` (or `-get`/`-put`/`-delete`), `hx-target` (a CSS selector),
   and `hx-swap` (`outerHTML` to replace the target, `beforeend` to append
   into it, etc.).
2. Create a partial under `views/partials/` — a
   `{{define "content"}}...{{end}}` block containing just the fragment to
   swap in.
3. In the controller: do the work, then `c.Partial("partials/<name>", data)`
   when `c.IsHTMX()`, with a sensible non-HTMX fallback (JSON, redirect,
   or a full `c.View`).
4. Register the route in `routes/web.go` like any other. HTMX requests go
   through the same compiled middleware chain — rate limiting, body
   limits, CSRF (once bridged), security headers — as everything else.

A deletion needs no partial at all: respond with an empty body and let
`hx-swap="outerHTML"` on the target replace the element with nothing.

The [Todo tutorial](tutorial-todo.md) puts all of this together into a
working add/toggle/delete list.
