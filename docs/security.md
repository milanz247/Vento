# Security

Vento ships hardened defaults and writes its threat model down instead of
leaving it implicit. The full audit — every gap identified, the mitigation
implemented, and what deliberately remains out of scope — lives in
[`SECURITY_AUDIT.md`](../SECURITY_AUDIT.md) at the project root; treat that
as the source of truth. This page is the practical summary, organized by
attack surface.

## The default protection stack

Wired in `routes/web.go`, outermost first:

```go
app.Use(
    vento.Logger,
    vento.Recovery,
    vento.SecurityHeaders,
    vento.BodyLimit(1<<20),
    vento.RateLimiter(10, 20),
    vento.CSRFProtection("/users"),
)
```

Plus two protections that aren't middleware: hardened server timeouts in
`Engine.Run`, and `html/template`'s contextual auto-escaping in every view.

## Attack surface → defense

| Threat | Defense | Where |
|---|---|---|
| Application-layer DoS (request floods) | Per-IP token bucket → `429` + `Retry-After`; bounded memory even under spoofed-IP floods | `vento.RateLimiter` — [details](middleware.md#ratelimiter) |
| Slow-loris connection exhaustion | `http.Server` with `ReadHeaderTimeout` 5s, `ReadTimeout`/`WriteTimeout` 30s, `IdleTimeout` 120s — never the standard library's unlimited defaults | `Engine.Run` (`vento/engine.go`) |
| Memory exhaustion via huge request bodies | `http.MaxBytesReader` cap (1 MiB global default); oversized reads fail instead of buffering | `vento.BodyLimit` — [details](middleware.md#bodylimit) |
| Cross-site request forgery | Double-submit cookie: 32-byte `crypto/rand` token, `SameSite=Lax`, `Secure` auto-set over TLS, constant-time comparison, `403` on mismatch | `vento.CSRFProtection` — [details](middleware.md#csrfprotection) |
| Clickjacking / MIME sniffing / referrer leakage | `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: strict-origin-when-cross-origin`, `X-XSS-Protection: 0` (per current OWASP guidance — the legacy auditor is gone from modern browsers and was itself exploitable) | `vento.SecurityHeaders` |
| XSS via rendered views | `html/template` contextual auto-escaping — on by default, defeated only by an explicit `template.HTML` cast | every `c.View`/`c.Partial` |
| CDN compromise of the htmx script | Subresource Integrity: when you load htmx, pin the artifact's SHA-384 hash + `crossorigin="anonymous"` on the `<script>` tag | [HTMX guide](htmx.md#wiring-htmx-into-your-layout) |
| Mass assignment | Request JSON binds to dedicated input structs (e.g. `createUserInput`), never directly onto GORM models — clients can't set `ID`, `CreatedAt`, or future privileged columns | `controllers/user_controller.go` |
| Schema/driver information disclosure | Real GORM/SQL errors are logged server-side only; clients get generic messages | controller convention — [details](database.md#querying-from-a-controller) |
| SQL injection | GORM parameterized queries throughout; no string-built SQL anywhere | all queries |
| A panicking request killing the process | `Recovery` catches, logs the stack server-side, returns a clean JSON `500` | `vento.Recovery` |
| Path traversal via static files | `http.Dir` rejects `..` escapes; static requests also run the full middleware chain | `Engine.Static` (`vento/static.go`) |
| Credential leakage through VCS | No DSN is ever hand-written; config lives as discrete `DB_*` keys in a gitignored `.env` | `vento/config.go` |

## Design notes worth understanding

**Why the CSRF cookie is readable by JavaScript.** The double-submit
pattern *requires* the front end to read the cookie and echo it in a
header — that's the proof the request originates from your page (a
cross-site attacker can make the browser *send* the cookie, but can't
*read* it thanks to the same-origin policy). `HttpOnly` would break this
by design. The layout's [CSRF bridge](htmx.md#htmx-and-csrf) does the echo
automatically for every HTMX request.

**Why CSRF exempts `/users`.** `POST /users` is a JSON API meant for
non-browser clients. CSRF attacks work by riding *ambient browser
credentials* (cookies) attached automatically to cross-site requests;
an API driven by explicit tokens has no such ambient credential to ride.
Exempting it is standard practice — the exemption list is the visible,
auditable record of that decision.

**Why chains being compiled at startup matters for security.** Middleware
coverage is determined by registration order and nothing else — you can
audit exactly what protects every route by reading `routes/web.go` top to
bottom. There is no runtime hook that can silently remove a protection.

**Why startup failures are fatal.** Half-configured deployments (missing
DB config, unparseable template) abort the boot loudly rather than serving
a degraded, unpredictable app. See
[Bootstrapping](bootstrapping.md#boot-order-rules-summarized).

## What's deliberately out of scope

From the audit's residual-risks section — read the full entries in
[`SECURITY_AUDIT.md`](../SECURITY_AUDIT.md#3-known-residual-risks--deliberate-scope-boundaries)
before relying on any of these in production:

- **TLS is not terminated by the framework.** Deploy behind a
  TLS-terminating reverse proxy, or build your own `http.Server` with TLS
  around the Engine (it's a plain `http.Handler`).
- **The rate limiter keys on `RemoteAddr`.** Behind a reverse proxy, every
  client shares the proxy's IP — rate-limit at the proxy, or extend the
  limiter to trust `X-Forwarded-For` from known proxy addresses only.
- **`/secret`'s token rides the query string.** Demo-only pattern; query
  strings land in access logs and referrer headers. Use an
  `Authorization` header in real code.
- **No `Content-Security-Policy` header.** A useful CSP requires
  per-application tuning of script/style sources, so Vento doesn't set a
  generic one. Add it via your own header middleware once your asset
  sources are stable.
- **`.env` hygiene is on you.** The repo's copy holds only local dev
  values; never commit production credentials.

## Adding your own security middleware

Any `vento.HandlerFunc` (or a factory returning one — see
[Middleware](middleware.md#writing-your-own-middleware)) can join the
global chain via `app.Use(...)` or guard a single route. Keep the ordering
convention: broad, cheap checks (headers, rate limiting) before expensive
or request-specific ones (CSRF, auth), so obviously-bad traffic is
rejected with minimum work — and remember `Use` only covers routes
registered *after* it.

## Reporting a vulnerability

This is a starter/reference framework rather than a maintained security
product — treat an issue you find like any other bug in your fork: fix it,
and record it in [`SECURITY_AUDIT.md`](../SECURITY_AUDIT.md) so the threat
model stays honest.
