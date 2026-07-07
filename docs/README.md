<p align="center">
  <img src="../assets/logo.svg" alt="Vento — a high-performance Go web framework" width="420">
</p>

# Vento Documentation

Vento is a lightweight, high-performance web framework for Go, built directly
on the standard library (`net/http`, `html/template`) with GORM/MySQL as its
only external integration. It borrows the developer experience of frameworks
like Laravel or Express — a single `Context` object, chainable middleware, an
MVC-style project layout, and an Artisan-style CLI — while staying small
enough to read end to end in an afternoon.

That last point is the whole design goal: **Vento is meant to be understood,
not just used.** Every guide below explains not only *how* to use a feature
but *how it is built* and *why it was built that way*, with pointers into the
actual source (everything lives in the seven files under `vento/`).

## How to read these docs

**New to the project?** Read in this order:

1. [Getting Started](getting-started.md) — clone to running server in five minutes.
2. [Project Structure](project-structure.md) — what every file is for, and the one-way import graph.
3. [Bootstrapping](bootstrapping.md) — what happens, line by line, between `go run .` and "listening on :8080".
4. [Architecture](architecture.md) — how `Engine`, `Context`, and the router fit together, and the performance model.

After that, jump to whichever topic you're working with.

## Contents

| Guide | What it covers |
|---|---|
| [Getting Started](getting-started.md) | Prerequisites, installing the CLI, environment setup, first run, a tour of every demo route |
| [Project Structure](project-structure.md) | Every file and folder, the one-way dependency graph, adding a feature end to end |
| [Bootstrapping](bootstrapping.md) | The complete startup sequence: env → DB → templates → routes → static → server |
| [Architecture](architecture.md) | `Engine`, `Context`, the router, the request lifecycle, and the zero-allocation design |
| [Routing](routing.md) | Registering routes, dynamic `:params`, Trie internals, matching precedence |
| [The Context API](context.md) | Full reference for every method on `*vento.Context` — request in, response out |
| [Middleware](middleware.md) | The chain model, execution order, every built-in, writing your own |
| [Views & Templates](views-templates.md) | Layout stitching at startup, rendering pages, passing data |
| [Reactive UIs with HTMX](htmx.md) | `IsHTMX()`, `Partial()`, the CSRF bridge |
| [Front-end: Tailwind & Static Assets](frontend-tailwind.md) | `Engine.Static`, the Tailwind build pipeline |
| [Database](database.md) | Models, the migration registry, seeders, querying with GORM |
| [Configuration](configuration.md) | `.env` parsing, environment variables, MySQL DSN assembly |
| [CLI Reference](cli-reference.md) | Every `vento` command, `setup.go`, hot reload via air |
| [Security](security.md) | Every built-in protection, the threat model, deliberate scope boundaries |
| [Tutorial: Todo CRUD](tutorial-todo.md) | Hands-on: build a complete create/read/update/delete feature, touching every layer |

## The one-paragraph mental model

An `Engine` holds one **Trie router per HTTP method**. At **registration
time** (startup), every route's full handler chain — global middlewares +
route middlewares + controller — is compiled into a single `[]HandlerFunc`
slice and stored on the route's Trie node. At **request time**, `ServeHTTP`
looks up that pre-built slice, takes a recycled `*Context` from a
`sync.Pool`, points it at the slice, and runs it. Templates get the same
treatment: every page is stitched into the shared layout **once**, at
startup, so rendering is a single `ExecuteTemplate` call. Nothing on the hot
path parses, compiles, or allocates what could have been prepared earlier.

## Design philosophy

Vento makes a small number of deliberate, opinionated choices instead of
trying to be everything to everyone:

- **One handler signature.** Every middleware and controller is
  `func(*vento.Context)`. There is no separate "middleware type" to learn;
  a middleware is just a handler that calls `c.Next()`.
- **One database.** MySQL via GORM is the only supported provider. This
  keeps `vento/config.go` and the CLI simple, at the cost of flexibility
  other frameworks offer. If you need a different database, fork
  `ConnectDB` — it's ten lines.
- **Compile once, serve fast.** Middleware chains and view templates are
  both assembled at startup, never per request. The corollary: **order of
  startup calls matters** (`Use` before routes, routes before `Static`),
  and the [Bootstrapping](bootstrapping.md) guide explains exactly why.
- **MVC by convention, not by magic.** `controllers/`, `models/`,
  `routes/` are plain Go packages with a one-way import graph
  (`main → routes → controllers → models/vento`). There is no
  reflection-based auto-wiring to reverse-engineer.
- **Security on by default.** Rate limiting, CSRF, body limits, server
  timeouts, and hardening headers ship enabled, and the threat model is
  written down in [`SECURITY_AUDIT.md`](../SECURITY_AUDIT.md) instead of
  left implicit.

## Where to go next

- Building a full feature, hands-on: [Tutorial: Todo CRUD](tutorial-todo.md)
- Building your first endpoint: [Routing](routing.md) + [The Context API](context.md)
- Adding a database table: [Database](database.md)
- Understanding a panic/500: [Middleware](middleware.md#recovery)
- Making part of a page update without a reload: [HTMX](htmx.md)
- Auditing what's exposed by default: [Security](security.md)
