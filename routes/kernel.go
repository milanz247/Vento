package routes

import (
	"vento-app/middleware"
	"vento-app/vento"
)

// GlobalMiddleware is the stack applied to every route, in order (outermost
// first) - Vento's equivalent of the middleware array in Laravel's
// app/Http/Kernel.php. This is the single, global place the chain is
// declared; add, remove, or reorder it here and every route is affected.
//
// The order is deliberate - cheap, broad protections run before expensive or
// request-specific ones:
//   - Logger wraps everything so even recovered panics get a timed line.
//   - Recovery converts downstream panics into clean 500s.
//   - RequestID stamps an X-Request-ID for tracing (app-level middleware).
//   - SecurityHeaders stamps hardening headers before any body is written.
//   - BodyLimit (1 MiB) caps request bodies before any handler reads them.
//   - RateLimiter (10 req/s, burst 20, per IP) rejects floods early.
//   - CSRFProtection guards browser form posts; pass path prefixes to
//     exempt (e.g. a JSON API: vento.CSRFProtection("/api")).
func GlobalMiddleware() []vento.HandlerFunc {
	return []vento.HandlerFunc{
		vento.Logger,
		vento.Recovery,
		middleware.RequestID,
		vento.SecurityHeaders,
		vento.BodyLimit(1 << 20),
		vento.RateLimiter(10, 20),
		vento.CSRFProtection(),
	}
}

// RegisterRoutes wires the whole HTTP surface onto app: it installs the
// global middleware stack, then maps every route. main.go calls this once,
// after templates are compiled.
//
// The ordering is handled here so you never have to think about it: Vento
// compiles each route's handler chain at registration time, so Use must run
// before any route is mapped. GlobalMiddleware goes on first; web() adds the
// routes after. To add a route, edit web() in web.go - not this function.
func RegisterRoutes(app *vento.Engine) {
	app.Use(GlobalMiddleware()...)
	web(app)
}
