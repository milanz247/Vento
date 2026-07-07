# Front-end: Tailwind CSS & Static Assets

Vento's Go core has no opinion about CSS. What the framework provides is
static file serving (`Engine.Static`); what the starter app adds on top is
a local Tailwind CSS build that produces one servable stylesheet. This page
covers both layers and how they meet.

## Static file serving (`Engine.Static`)

```go
func (e *Engine) Static(urlPrefix, dir string)

app.Static("/public", "./public")  // ./public/css/app.css -> /public/css/app.css
```

`vento/static.go` implements this in ~40 lines on top of the standard
library: `http.StripPrefix` + `http.FileServer(http.Dir(dir))`. The design
decisions worth knowing:

- **Chains compile at registration, like routes.** The mount's handler
  chain is `[current global middlewares..., fileServer]`, snapshotted when
  `Static` is called. So **`Static` must come after
  `routes.RegisterRoutes`** (which calls `Use` first) — earlier, and static
  responses would silently skip `Logger`, `SecurityHeaders`, the rate
  limiter, everything. `main.go` gets this right; keep it that way. (Full
  reasoning: [Bootstrapping § step 7](bootstrapping.md#step-7--appstaticpublic-public).)
- **Mounts are checked before the route Trie** in `ServeHTTP` — a matching
  prefix short-circuits routing. Pick a prefix (like `/public`) that no
  route uses.
- **Both paths converge on `dispatch`**, so a static request still gets a
  pooled `Context` and identical middleware coverage to a routed one —
  static files are rate-limited and header-hardened like everything else.
- **Path traversal is already handled**: `http.Dir` rejects `..` escapes
  outside `dir`. The flip side: *everything* under the mounted directory
  is servable. Only mount directories meant for full public exposure —
  never the project root, never anywhere `.env` lives.

Any new static asset just goes under `public/` — `public/img/logo.png` is
served at `/public/img/logo.png` with no per-file code.

## The Tailwind build

The build lives entirely in `package.json`/`node_modules` and never touches
the Go binary — Node is a build-time tool here, not a runtime dependency.

```
assets/css/input.css    source   →   @import "tailwindcss";
public/css/app.css      output   →   compiled + minified (gitignored)
```

```json
"scripts": {
  "build:css": "tailwindcss -i ./assets/css/input.css -o ./public/css/app.css --minify",
  "watch:css": "tailwindcss -i ./assets/css/input.css -o ./public/css/app.css --watch"
}
```

Tailwind v4 scans the project's templates for class names automatically, so
`views/**/*.html` drives what ends up in the bundle — unused utilities are
never emitted.

### First-time setup

```bash
npm install
npm run build:css
```

A fresh clone renders unstyled until this runs — `public/css/app.css` is
generated, not committed.

### During development

Two terminals:

```bash
npm run watch:css    # rebuilds the CSS when a template's classes change
./bin/vento run       # air rebuilds/restarts the Go server on .go/.html edits
```

Editing a Tailwind class in a view triggers both watchers — refresh the
browser to see the combined result.

### Why there is no `tailwind.config.js`

Tailwind CSS v4 configures itself from the CSS entry point. Customize
design tokens with an `@theme` block in `assets/css/input.css`:

```css
@import "tailwindcss";

@theme {
  --color-brand: #16213e;
  --font-sans: "Inter", sans-serif;
}
```

...then use them as utilities: `bg-brand`, `font-sans`.

## Wiring into the layout

`views/layouts/base.html` links the compiled bundle:

```html
<link rel="stylesheet" href="/public/css/app.css">
```

The same layout loads the only runtime CDN asset in the project — the htmx
script — pinned with a Subresource Integrity hash so a tampered CDN
response is rejected by the browser ([Security](security.md)). Everything
else is served locally.

## Committed vs. generated

| | Paths |
|---|---|
| **Committed** | `package.json`, `package-lock.json`, `assets/css/input.css`, `assets/*.svg` |
| **Generated (gitignored)** | `node_modules/`, `public/css/app.css` |
