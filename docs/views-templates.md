# Views & Templates

Vento renders server-side HTML with Go's standard `html/template`, plus a
layout-composition layer that runs **once, at startup** — so controllers
never touch `template.Template`, and no template is ever parsed at request
time. This page explains the composition model, the rendering API, and the
template-namespace rules that follow from the design.

## The three kinds of template files

```
views/
├── layouts/
│   └── base.html          # the shared document shell
└── index.html             # a page (rendered via c.View)
```

Fragment-only views for HTMX swaps conventionally live in a `views/partials/`
directory (rendered via `c.Partial`); the starter ships without one — the
[Todo tutorial](tutorial-todo.md) adds it when it builds an HTMX feature.

- **Layouts** (`views/layouts/*.html`) define the document: `<!DOCTYPE>`,
  `<head>`, any shared chrome (nav, footer) — and a hole for the page:

  ```html
  <body>
      {{template "content" .}}
  </body>
  ```

- **Pages** (any other `.html` under `views/`) fill that hole by defining
  the `content` block:

  ```html
  {{define "content"}}
  <section>
      <h1>Welcome to Vento</h1>
      <p>{{.Message}}</p>
  </section>
  {{end}}
  ```

- **Partials** are technically just pages — same discovery, same `content`
  block — but they hold a *fragment* rather than a full page body, and are
  rendered without the layout via `c.Partial`. Convention puts them under
  `views/partials/`.

## How startup compilation works

`app.LoadHTMLGlob("views/**/*")` in `main.go` does all the work
(`vento/engine.go`), and understanding it explains every rule below:

1. Walk `views/` once. `.html` files under `layouts/` form the **layout
   set**; every other `.html` file is a **page**.
2. Parse the layout set. The entry template — the one executed to produce
   a document — is `base.html` if present, else the first layout parsed.
3. For each page: **clone the layout set**, then parse the page into the
   clone. The clone is the trick — every page gets a private template
   namespace, so all pages can define a block with the same name
   (`content`) without colliding.
4. Store the result in a map keyed by the page's path relative to
   `views/`, minus the extension:

   | File | View name |
   |---|---|
   | `views/index.html` | `"index"` |
   | `views/partials/todo_row.html` | `"partials/todo_row"` |
   | `views/users/show.html` | `"users/show"` |

Parse errors **panic at startup**, naming the file — a template typo kills
the boot instead of becoming a runtime 500 later.

Two consequences of per-page cloning:

- **Pages cannot include other pages.** They share *layouts*, not each
  other. A snippet needed by several pages belongs in `views/layouts/`
  (any file there joins the shared set), or duplicate it deliberately —
  see the [HTMX guide](htmx.md) for when a fragment is rendered both
  inside a page and standalone via `c.Partial`.
- **Template edits need a restart** — the cache is built once. With
  `air` (`./bin/vento run`), `.html` edits trigger the rebuild/restart
  automatically.

## Rendering from a controller

```go
func Index(c *vento.Context) {
    c.View("index", map[string]any{
        "Message": "Welcome to Vento - a high-performance Go web framework!",
    })
}
```

| Call | Renders | Status |
|---|---|---|
| `c.View(name, data)` | page inside the layout | 200 |
| `c.HTML(code, name, data)` | page inside the layout | explicit |
| `c.Partial(name, data)` | the page's `content` block only — no layout | 200 |

`name` accepts the view key with or without the `.html` suffix. An unknown
name writes a 500 that names the missing view. `data` is anything the
template can address; a `map[string]any` is idiomatic for pages, a single
model value for partials (`{{.Title}}` reads a field off it directly).

## Passing data and template syntax

The `data` argument is the dot (`.`) inside the template:

```html
<p>{{.Message}}</p>                 <!-- map key or struct field -->

{{range .Users}}                    <!-- iterate a slice -->
    <td>{{.Name}}</td>              <!-- dot rebinds to the element -->
{{end}}

{{if .Done}}checked{{end}}          <!-- conditionals -->
```

This is standard `html/template` — anything in its documentation works.

## XSS safety comes free

`html/template` escapes **contextually**: a value interpolated into HTML
body text is HTML-escaped, into an attribute attribute-escaped, into a URL
context URL-escaped. `{{.Message}}` containing
`<script>alert(1)</script>` renders as harmless text. Don't circumvent
this with `template.HTML` unless the value is genuinely trusted markup you
built yourself — that cast is the single way to reintroduce XSS here.

## Where partials fit

`c.Partial` executes only the view's `content` block — no `<html>`, no
nav, just the fragment — which is exactly what an HTMX `hx-swap` needs.
The mechanics and the CSRF bridge are covered in
[Reactive UIs with HTMX](htmx.md).

## Adding a page, end to end

1. Create `views/about.html`:

   ```html
   {{define "content"}}
   <section class="mx-auto max-w-2xl px-6 py-24">
       <h1 class="text-3xl font-bold">{{.Title}}</h1>
   </section>
   {{end}}
   ```

2. Add a controller:

   ```go
   func About(c *vento.Context) {
       c.View("about", map[string]any{"Title": "About"})
   }
   ```

3. Register the route in `routes/web.go`:

   ```go
   app.GET("/about", controllers.About)
   ```

4. Restart (or let air do it). The new page was compiled into the cache at
   boot; rendering it costs one `ExecuteTemplate` call.
