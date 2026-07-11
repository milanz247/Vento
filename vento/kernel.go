package vento

// DefaultMiddleware is Vento's production-ready global middleware stack,
// installed automatically by New() so a fresh app is secure out of the box
// with zero setup - no app has to hand-assemble logging, recovery, or
// security headers itself. The order is deliberate - cheap, broad
// protections run before expensive or request-specific ones:
//
//   - Logger wraps everything so even recovered panics get a timed line.
//   - Recovery converts downstream panics into clean 500s.
//   - SecurityHeaders stamps hardening headers before any body is written.
//   - BodyLimit (1 MiB) caps request bodies before any handler reads them.
//   - RateLimiter (10 req/s, burst 20, per IP) rejects floods early.
//   - CSRFProtection guards every cookie/session-authenticated route,
//     which by default is everything: only "/api/auth" is exempted, since
//     those are the two or three routes (register/login) that run before a
//     session exists at all, so there's nothing yet for CSRF to protect.
//     Nothing else is exempt by default - a session-authenticated route
//     (see RequireAuth) left exempt has no CSRF protection at all, which
//     is exactly the mistake this default used to make by blanket-exempting
//     all of "/api" on the assumption every API route uses token auth.
//     A genuinely token-authenticated route (bearer/API-key, never reads a
//     cookie) is fine to exempt explicitly, per-app, in main.go - CSRF only
//     matters for requests a browser attaches credentials to automatically.
//
// An application adds its own middleware (e.g. request-ID tracing, auth) on
// top via app.Use(...) in main.go, before mapping its route tables, rather
// than editing this list - which keeps framework-level and
// application-level concerns separate.
func DefaultMiddleware() []HandlerFunc {
	return []HandlerFunc{
		Logger,
		Recovery,
		SecurityHeaders,
		BodyLimit(1 << 20),
		RateLimiter(10, 20),
		CSRFProtection("/api/auth"),
	}
}
