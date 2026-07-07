// Package routes declares every application endpoint, mirroring
// Laravel's routes/web.php. It imports controllers and vento only - never
// main - which keeps main -> routes -> controllers a one-way dependency
// graph instead of a cycle.
package routes

import (
	"vento-app/controllers"
	"vento-app/vento"
)

// RegisterRoutes wires the global middleware stack and every route onto
// app. This is the one place you add routes as your app grows - think of
// it as Laravel's routes/web.php.
//
// Vento compiles each route's chain at registration time, so Use must run
// before the route mappings - middleware order reads outermost first:
//   - Logger wraps everything so even recovered panics get a timed line.
//   - Recovery converts downstream panics into clean 500s.
//   - SecurityHeaders stamps hardening headers before any body is written.
//   - BodyLimit (1 MiB) caps request bodies before any handler reads them.
//   - RateLimiter (10 req/s, burst 20, per IP) rejects floods early.
//   - CSRFProtection guards browser-facing form posts; pass path prefixes
//     to exempt (e.g. a JSON API: vento.CSRFProtection("/api")).
func RegisterRoutes(app *vento.Engine) {
	app.Use(
		vento.Logger,
		vento.Recovery,
		vento.SecurityHeaders,
		vento.BodyLimit(1<<20),
		vento.RateLimiter(10, 20),
		vento.CSRFProtection(),
	)

	// Routes. Add yours here.
	app.GET("/", controllers.Index)
}
