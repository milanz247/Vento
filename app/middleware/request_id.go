// Package middleware holds the application's own middleware, mirroring
// Laravel's app/Http/Middleware. It is the home for cross-cutting request
// logic you write yourself - the framework's built-in middleware (Logger,
// Recovery, SecurityHeaders, BodyLimit, RateLimiter, CSRFProtection) lives
// in the vento package and is documented in docs/middleware.md.
//
// A middleware is any vento.HandlerFunc - func(*vento.Context) - that calls
// c.Next() to pass control down the chain. Register the ones here in
// routes/web.go: globally via app.Use(...), or per-route as an extra
// argument to app.GET/POST/... The dependency direction stays one-way:
// this package imports vento only, never routes or controllers.
package middleware

import "vento-app/vento"

// RequestID attaches a unique X-Request-ID to every request/response so a
// single request can be traced across logs and, behind a proxy, across
// services. An inbound X-Request-ID (e.g. set by a load balancer) is
// honored; otherwise a fresh random one is generated via vento.RandomString
// - the same random-token primitive Vento's own CSRF protection uses, so
// there's exactly one place in the whole app that touches crypto/rand. The
// value is written onto both the request headers - so downstream handlers
// can read it via c.Request.Header.Get("X-Request-ID") - and the response
// headers.
//
// It is wired into the global chain in routes/web.go. Use it as the pattern
// for your own plain (non-configurable) middleware.
func RequestID(c *vento.Context) {
	id := c.Request.Header.Get("X-Request-ID")
	if id == "" {
		id = vento.RandomString(16) // 32 hex chars
		c.Request.Header.Set("X-Request-ID", id)
	}
	c.Writer.Header().Set("X-Request-ID", id)
	c.Next()
}
