# Security Audit — Vento Framework

**Scope:** framework core (`vento/`), application layer (`controllers/`, `routes/`, `models/`, `cmd/vento/`), configuration (`.env`), and the demo application.
**Date:** 2026-07-05

---

## 1. Threat-model gaps identified during design review

| # | Finding | Severity | Description |
|---|---------|----------|-------------|
| 1 | No rate limiting | High | Without throttling, any client can issue unlimited requests: a single host can exhaust database connections or CPU (application-layer DoS). |
| 2 | No CSRF protection | High | Browser form endpoints (e.g. `POST /submit`) would accept any cross-origin request carrying the victim's ambient cookies. |
| 3 | Missing security headers | Medium | Responses without `X-Frame-Options` (clickjacking), `X-Content-Type-Options` (MIME sniffing), `Referrer-Policy` (referrer leakage), and `X-XSS-Protection` leave browser-side attack surface open. |
| 4 | Database error leakage | Medium | Echoing raw GORM/SQL error strings to clients disclosss schema and driver internals — reconnaissance fuel for injection attempts. |
| 5 | Mass assignment | Medium | Binding request JSON directly onto GORM models lets clients set `ID`, `CreatedAt`, `DeletedAt`, and any future privileged column. |
| 6 | No input validation | Low | Records could be created with empty required fields. |
| 7 | Configuration in code | Low | Hard-coded DSNs leak credentials through VCS history; credentials must live in `.env`, never committed with production values. |
| 8 | Per-request allocation pressure | Info (perf/availability) | Heap-allocating per-request state adds GC pauses under sustained load — an availability concern under DoS conditions. |

## 2. Mitigations implemented

| Gap | Mitigation | Where |
|-----|-----------|-------|
| 1 | Per-IP token-bucket rate limiter over `sync.Map` (wired at 10 req/s, burst 20 → HTTP 429 + `Retry-After`). Idle buckets are purged at most once a minute so the map stays bounded under spoofed-address floods. | `vento/security.go` → `RateLimiter`, wired in `routes/web.go` |
| 2 | Double-submit-cookie CSRF middleware: safe methods are issued a 32-byte `crypto/rand` token cookie (`vento_csrf`, `SameSite=Lax`); non-idempotent methods must echo it in `X-CSRF-Token` or `_csrf`, compared with `crypto/subtle.ConstantTimeCompare` → 403 otherwise. Explicit exemption list for token-driven JSON APIs. | `vento/security.go` → `CSRFProtection`, wired in `routes/web.go` |
| 3 | `SecurityHeaders` middleware stamps `X-Frame-Options: DENY`, `X-XSS-Protection: 1; mode=block`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: strict-origin-when-cross-origin` on every response. | `vento/security.go` → `SecurityHeaders` |
| 4 | Database errors are logged server-side only; clients receive generic messages ("could not create user"). | `controllers/user_controller.go` |
| 5 | JSON binds to a dedicated `createUserInput` struct; only `name`/`email` ever reach the model. | `controllers/user_controller.go` |
| 6 | Required-field validation → 422 on empty name/email. | `controllers/user_controller.go` |
| 7 | All DB settings live in `.env` as discrete `DB_*` keys; `BuildMySQLDSN` assembles the DSN at runtime and startup aborts loudly when configuration is incomplete. | `vento/config.go`, `main.go` |
| 8 | `sync.Pool`-backed Context recycling plus registration-time chain compilation: zero per-request Context/chain allocations once warm. `Context.Reset` clears all per-request state, so a recycled Context can never leak one client's data to another. | `vento/engine.go`, `vento/context.go`, `vento/router.go` |

Additional standing controls: panic recovery (`vento.Recovery`) keeps the process alive and returns clean 500s with server-side stack traces; request logging (`vento.Logger`) provides an audit trail with latency and status; `html/template` gives contextual auto-escaping (XSS-safe output encoding) on every rendered view.

### Addendum (2026-07-05): server hardening pass

- **Server timeouts** — `Engine.Run` now builds an `http.Server` with
  `ReadHeaderTimeout: 5s`, `ReadTimeout: 30s`, `WriteTimeout: 30s`,
  `IdleTimeout: 120s` instead of calling bare `http.ListenAndServe` (which
  has no limits), closing the slow-loris hole. Applications needing custom
  values can construct their own `http.Server` around the Engine, which
  implements `http.Handler` directly. (`vento/engine.go`)
- **Request body limits** — new `vento.BodyLimit(maxBytes)` middleware wraps
  the body in `http.MaxBytesReader`; wired globally at 1 MiB in
  `routes/web.go`, so no handler can be streamed an unbounded body.
  (`vento/security.go`)
- **CSRF cookie `Secure` flag** — now set automatically whenever the request
  arrived over TLS (`c.Request.TLS != nil`), so HTTPS deployments never send
  the token in clear text. Behind a TLS-terminating proxy, set the flag at
  the proxy. (`vento/security.go`)
- **`X-XSS-Protection` modernized** — changed from `1; mode=block` to `0`
  per current OWASP guidance: the legacy browser XSS auditor has been
  removed from modern browsers and was itself an information-leak vector in
  older ones. XSS defense rests on `html/template` contextual auto-escaping.
  (`vento/security.go`)
- **Subresource Integrity** — the htmx `<script>` tag in
  `views/layouts/base.html` now carries an `integrity` hash (verified
  against the published 1.9.12 artifact) plus `crossorigin="anonymous"`, so
  a compromised or hijacked CDN response is rejected by the browser instead
  of executed.

### Addendum (2026-07-05): static file serving

`Engine.Static` (`vento/static.go`) was added to serve the compiled Tailwind CSS bundle at `/public`. It wraps `http.FileServer(http.Dir(dir))`, which already rejects `..` path-traversal outside `dir` — no additional sanitization was needed. Static requests are compiled with the same global middleware chain as routes (`Logger`, `Recovery`, `SecurityHeaders`, `RateLimiter`, ...), so they receive identical hardening and are equally subject to the rate limiter. Only mount directories intended for full public exposure — everything under the mounted `dir` is servable, so `.env`, `.git`, or other project directories must never be passed to `Static`.

## 3. Known residual risks / deliberate scope boundaries

- **TLS** is not terminated by the framework; deploy behind a TLS-terminating proxy or extend `Run` to `http.ListenAndServeTLS`.
- **`/secret` token in query string** — demo-only pattern; query strings land in access logs and referrer headers. Real deployments should use an `Authorization` header.
- **Rate limiter behind proxies** keys on `RemoteAddr`; behind a reverse proxy all clients share the proxy's IP. Trust `X-Forwarded-For` only from known proxy addresses if needed.
- **No `Content-Security-Policy`** header yet; requires per-application tuning of script/style sources.
- **`.env` hygiene** — this repo's copy holds only local development values; never commit a production `.env`.
