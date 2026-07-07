# Tutorial: Building a Todo List (Full CRUD)

This tutorial builds a complete, working todo list with Vento — create, read,
update (toggle done), and delete — rendered server-side and updated in place
with HTMX, no page reloads and no client-side framework. Along the way it
touches every layer of the framework in the order a real feature grows:
**model → migration → controller → routes → views → verify**.

You should have the app running first ([Getting Started](getting-started.md)).
Everything here follows the conventions from the other guides, which are
linked at each step if you want the deeper "why".

## What you'll build

A `/todos` page with:

- a form that **adds** a todo (the new row appears instantly),
- a checkbox on each row that **toggles** done/not-done (the row re-renders
  in place, with strikethrough styling),
- a delete button that **removes** the row from the table and the database.

Behind it: one model, four handlers, three routes + one page route, one page
template, and one partial. That's the entire feature.

## Step 1 — The model

Create `models/todo.go`:

```go
package models

import "gorm.io/gorm"

// Todo is one item on the todo list (GET /todos, POST /todos,
// POST /todos/:id/toggle, DELETE /todos/:id).
type Todo struct {
	gorm.Model
	Title string
	Done  bool
}
```

`gorm.Model` provides `ID`, `CreatedAt`, `UpdatedAt`, and soft-delete
`DeletedAt` ([Database § Models](database.md#models)).

Register it in the model registry — append one line to `models.All()`
in `models/user.go`:

```go
func All() []any {
	return []any{
		&User{},
		&Todo{},   // ← new
	}
}
```

That list is what `db:automigrate` and the seeders read
([Database § The model registry](database.md#the-model-registry)).

## Step 2 — Migrate

Scaffold a migration for the new table:

```bash
./bin/vento make:migration create_todos_table
```

Open the generated `migrations/<timestamp>_create_todos_table.go`, add
`"vento-app/models"` to its imports, and fill in `Up`/`Down`:

```go
Up: func(tx *gorm.DB) error {
	return tx.AutoMigrate(&models.Todo{})
},
Down: func(tx *gorm.DB) error {
	return tx.Migrator().DropTable(&models.Todo{})
},
```

Then apply it:

```bash
./bin/vento db:migrate
```

You should see `migrated <timestamp>_create_todos_table` — the `todos` table
now exists. `db:migrate` records each applied migration in
`schema_migrations`, so re-running only ever runs new ones (and
`./bin/vento db:rollback` reverts this one via its `Down`).

## Step 3 — The controller

Create `controllers/todo_controller.go`. Four handlers: one page, three
mutations. (You can scaffold a starting point with
`./bin/vento make:controller Todo` and reshape it, or just write the file.)

```go
package controllers

import (
	"log"
	"net/http"

	"vento-app/vento"
	"vento-app/models"
)

// TodoIndex handles GET /todos: the full page, seeded with every todo so
// the first paint already matches what later HTMX swaps will produce.
func TodoIndex(c *vento.Context) {
	var todos []models.Todo
	if err := c.DB().Order("id").Find(&todos).Error; err != nil {
		log.Printf("controllers: listing todos failed: %v", err)
		c.Abort(http.StatusInternalServerError, "could not list todos")
		return
	}
	c.View("todos", map[string]any{"Todos": todos})
}

// CreateTodo handles POST /todos: binds the "title" form field, inserts a
// new (incomplete) todo, and responds with just that row's markup
// (views/partials/todo_row.html) so HTMX can append it via
// hx-target="#todo-list-body" hx-swap="beforeend".
func CreateTodo(c *vento.Context) {
	title := c.FormValue("title")
	if title == "" {
		c.Abort(http.StatusUnprocessableEntity, "title is required")
		return
	}

	todo := models.Todo{Title: title}
	if err := c.DB().Create(&todo).Error; err != nil {
		log.Printf("controllers: creating todo failed: %v", err)
		c.Abort(http.StatusInternalServerError, "could not create todo")
		return
	}

	c.Partial("partials/todo_row", todo)
}

// ToggleTodo handles POST /todos/:id/toggle: flips Done on the todo
// identified by the :id route segment and responds with the row's updated
// markup, so HTMX can swap it in place via hx-swap="outerHTML".
func ToggleTodo(c *vento.Context) {
	var todo models.Todo
	if err := c.DB().First(&todo, c.Param("id")).Error; err != nil {
		c.Abort(http.StatusNotFound, "todo not found")
		return
	}

	todo.Done = !todo.Done
	if err := c.DB().Save(&todo).Error; err != nil {
		log.Printf("controllers: toggling todo failed: %v", err)
		c.Abort(http.StatusInternalServerError, "could not update todo")
		return
	}

	c.Partial("partials/todo_row", todo)
}

// DeleteTodo handles DELETE /todos/:id: removes the todo and responds with
// an empty body. Paired with hx-swap="outerHTML" on the client, swapping an
// element's outerHTML with nothing removes it from the DOM — no partial
// template needed for a deletion.
func DeleteTodo(c *vento.Context) {
	if err := c.DB().Delete(&models.Todo{}, c.Param("id")).Error; err != nil {
		log.Printf("controllers: deleting todo failed: %v", err)
		c.Abort(http.StatusInternalServerError, "could not delete todo")
		return
	}

	c.String(http.StatusOK, "")
}
```

Conventions at work here, all covered elsewhere in the docs:

- **Real errors to the log, generic messages to the client**
  ([Database § Querying](database.md#querying-from-a-controller)).
- **`c.Param("id")`** reads the dynamic route segment
  ([Routing § Dynamic parameters](routing.md#dynamic-parameters)).
- **`c.Partial`** renders just the row fragment, no layout
  ([The Context API](context.md#cpartialname-string-data-any)).
- **422 for validation, 404 for missing rows** — `Abort` writes the JSON
  error and stops the chain.
- Form fields arrive through `c.FormValue`; bodies are already capped by
  the global `BodyLimit` middleware.

## Step 4 — Routes

In `routes/web.go`, inside `RegisterRoutes` (after the `app.Use(...)`
block, like every route):

```go
// Todo list: full CRUD, HTMX-driven. CSRF-protected the normal way —
// the bridge added to the layout below sends X-CSRF-Token automatically.
app.GET("/todos", controllers.TodoIndex)
app.POST("/todos", controllers.CreateTodo)
app.POST("/todos/:id/toggle", controllers.ToggleTodo)
app.DELETE("/todos/:id", controllers.DeleteTodo)
```

Note what you did **not** do: no CSRF exemption. These are browser-driven
endpoints, so they *should* be CSRF-checked — and they'll pass, because the
[CSRF bridge](htmx.md#htmx-and-csrf) you add to the layout in Step 5 attaches
the token to every HTMX request automatically. Every route also inherits the
full global chain (logging, recovery, headers, body limit, rate limiting) by
virtue of being registered after `Use`
([Middleware](middleware.md#global-vs-route-middleware)).

## Step 5 — The views

### Wire HTMX into the layout

HTMX is opt-in, so first load it (and the CSRF bridge) in
`views/layouts/base.html` — add this inside `<head>`, before the stylesheet
link ([HTMX § Wiring HTMX into your layout](htmx.md#wiring-htmx-into-your-layout)):

```html
<script src="https://unpkg.com/htmx.org@1.9.12"
        integrity="sha384-ujb1lZYygJmzgSwoxRggbCHcjc0rB2XoQrxeTUQyRjrOnlCoYta87iKBWq3EsdM2"
        crossorigin="anonymous"></script>
<script>
document.addEventListener('htmx:configRequest', function (evt) {
    var match = document.cookie.match(/(?:^|;\s*)vento_csrf=([^;]+)/);
    if (match) { evt.detail.headers['X-CSRF-Token'] = match[1]; }
});
</script>
```

### The page: `views/todos.html`

A page is a `{{define "content"}}` block; the layout provides everything
else ([Views & Templates](views-templates.md)). View name = path relative
to `views/` minus `.html`, so this page renders with `c.View("todos", ...)`.

```html
{{define "content"}}
<section class="mx-auto max-w-xl px-6 py-24">
    <h1 class="text-3xl font-bold tracking-tight text-slate-900">Todos</h1>
    <p class="mt-2 text-sm text-slate-500">
        Every action below updates in place via HTMX &mdash; no page reloads.
    </p>

    <form
        hx-post="/todos"
        hx-target="#todo-list-body"
        hx-swap="beforeend"
        hx-on::after-request="if(event.detail.successful) this.reset()"
        class="mt-8 flex gap-2"
    >
        <input type="text" name="title" placeholder="What needs doing?" required
               class="min-w-0 flex-1 rounded-lg border border-slate-200 px-3 py-2 text-sm focus:border-slate-400 focus:outline-none">
        <button type="submit"
                class="rounded-lg bg-slate-900 px-4 py-2 text-sm font-semibold text-white transition hover:bg-slate-700">
            Add
        </button>
    </form>

    <table class="mt-8 w-full text-left">
        <tbody id="todo-list-body">
            {{range .Todos}}
            <tr id="todo-row-{{.ID}}">
                <td class="w-8 border-t border-slate-100 py-2">
                    <input type="checkbox" {{if .Done}}checked{{end}}
                           hx-post="/todos/{{.ID}}/toggle"
                           hx-target="#todo-row-{{.ID}}"
                           hx-swap="outerHTML"
                           class="h-4 w-4 rounded border-slate-300">
                </td>
                <td class="border-t border-slate-100 py-2 pr-2 text-sm {{if .Done}}text-slate-400 line-through{{else}}text-slate-700{{end}}">
                    {{.Title}}
                </td>
                <td class="w-8 border-t border-slate-100 py-2 text-right">
                    <button
                        hx-delete="/todos/{{.ID}}"
                        hx-target="#todo-row-{{.ID}}"
                        hx-swap="outerHTML"
                        class="text-slate-400 transition hover:text-red-500"
                        aria-label="Delete"
                    >&times;</button>
                </td>
            </tr>
            {{end}}
        </tbody>
    </table>
</section>
{{end}}
```

### The partial: `views/partials/todo_row.html`

The single-row fragment the mutation handlers respond with. It **must be
identical markup to one `<tr>` of the page above** — this duplication is a
consequence of pages being independent template clones, explained in
[Views & Templates](views-templates.md#how-startup-compilation-works).

```html
{{define "content"}}
<tr id="todo-row-{{.ID}}">
    <td class="w-8 border-t border-slate-100 py-2">
        <input type="checkbox" {{if .Done}}checked{{end}}
               hx-post="/todos/{{.ID}}/toggle"
               hx-target="#todo-row-{{.ID}}"
               hx-swap="outerHTML"
               class="h-4 w-4 rounded border-slate-300">
    </td>
    <td class="border-t border-slate-100 py-2 pr-2 text-sm {{if .Done}}text-slate-400 line-through{{else}}text-slate-700{{end}}">
        {{.Title}}
    </td>
    <td class="w-8 border-t border-slate-100 py-2 text-right">
        <button
            hx-delete="/todos/{{.ID}}"
            hx-target="#todo-row-{{.ID}}"
            hx-swap="outerHTML"
            class="text-slate-400 transition hover:text-red-500"
            aria-label="Delete"
        >&times;</button>
    </td>
</tr>
{{end}}
```

Here the partial renders a `models.Todo` value directly (not a map), so the
template reads fields off the dot: `{{.ID}}`, `{{.Title}}`, `{{.Done}}`.

### How each interaction maps to HTMX

| Action | Request | Response | Swap |
|---|---|---|---|
| Add | `hx-post="/todos"` (form) | new row's `<tr>` | `beforeend` into `#todo-list-body` — appends |
| Toggle | `hx-post="/todos/{id}/toggle"` (checkbox) | updated `<tr>` | `outerHTML` of `#todo-row-{id}` — replaces in place |
| Delete | `hx-delete="/todos/{id}"` (button) | empty body | `outerHTML` with nothing — removes the row |

Details worth noticing:

- **`id="todo-row-{{.ID}}"`** gives every row a stable target so toggle
  and delete can address exactly one `<tr>`.
- **`hx-on::after-request="...this.reset()"`** clears the input after a
  *successful* add — a failed validation (422) leaves your typing intact.
- **The swapped-in row carries its own `hx-*` attributes**, so a toggled
  row can be toggled again and deleted — each swap re-arms the next
  interaction ([HTMX](htmx.md) explains the building blocks).
- The Tailwind classes above are picked up automatically by the CSS build —
  run `npm run build:css` (or have `watch:css` running) if you introduced
  classes not used elsewhere.

## Step 6 — Run and verify

Restart the server (or let air do it — template and Go changes both
trigger it):

```bash
./bin/vento run
```

Open **http://localhost:8080/todos** and exercise all four operations: add
a couple of items, toggle one (strikethrough appears), delete one. Watch
the server log — each action produces one line, e.g.
`[vento] POST /todos 200 3.1ms`.

### Verifying from the command line

The mutation endpoints are CSRF-protected, so a bare curl gets rejected —
prove that first (this failing is the security working):

```bash
curl -i -X POST localhost:8080/todos -d 'title=from curl'
# HTTP/1.1 403 ... {"error":"CSRF token missing"}
```

To call them like a browser would, do the cookie/header dance:

```bash
JAR=/tmp/vento-cookies.txt
curl -s -c "$JAR" localhost:8080/todos -o /dev/null          # GET issues the cookie
TOKEN=$(grep vento_csrf "$JAR" | awk '{print $NF}')

# Create
curl -s -b "$JAR" -H "X-CSRF-Token: $TOKEN" -X POST \
     -d 'title=from curl' localhost:8080/todos
# → <tr id="todo-row-N">...</tr>

# Toggle (use the N from the row you just created)
curl -s -b "$JAR" -H "X-CSRF-Token: $TOKEN" -X POST \
     localhost:8080/todos/N/toggle

# Delete
curl -s -b "$JAR" -H "X-CSRF-Token: $TOKEN" -X DELETE \
     localhost:8080/todos/N
```

And confirm validation:

```bash
curl -s -b "$JAR" -H "X-CSRF-Token: $TOKEN" -X POST -d 'title=' localhost:8080/todos
# → {"error":"title is required"}   (HTTP 422)
```

## What you exercised

| Layer | What this feature used | Guide |
|---|---|---|
| Model | `Todo` struct + `models.All()` registration | [Database](database.md) |
| Migration | `make:migration` → `Up`/`Down` → `db:migrate` | [Database § Migrations](database.md#migrations) |
| Routing | static + `:id` routes, `GET`/`POST`/`DELETE` trees | [Routing](routing.md) |
| Context | `Param`, `FormValue`, `View`, `Partial`, `String`, `Abort`, `DB` | [The Context API](context.md) |
| Views | a page, a partial, per-page template namespaces | [Views & Templates](views-templates.md) |
| HTMX | `beforeend` append, `outerHTML` swap, empty-body delete, form reset | [HTMX](htmx.md) |
| Security | CSRF (via the bridge), body limit, validation, generic errors | [Security](security.md) |

## Where to take it next

- **A JSON API alongside the HTML.** Branch on `c.IsHTMX()` in the
  mutation handlers — fragment for HTMX, `c.JSON` for API clients. Bind
  JSON bodies to a dedicated
  input struct (`type createTodoInput struct { Title string }`), never the
  GORM model ([mass assignment](database.md#querying-from-a-controller)).
- **Seed data.** Add a `todos` seeder in `cmd/vento/main.go` with
  `FirstOrCreate` keyed on `Title` ([Database § Seeders](database.md#seeders)).
- **An edit form.** A `GET /todos/:id/edit` partial that swaps the row for
  an inline form, and a `PUT /todos/:id` that swaps back the updated row —
  the same three-piece pattern (route, handler, partial) as toggle.
- **Validation beyond "not empty".** Length caps, trimming, duplicate
  checks — validation lives in the handler (or a route middleware),
  returning 422 with a useful message.
