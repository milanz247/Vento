// Package routes declares the application's HTTP surface as one exported
// function per table, mirroring Laravel's routes/*.php files: browser page
// routes live here in web.go (routes/web.php equivalent), JSON API routes
// live in api.go (routes/api.php equivalent), and an app can add its own
// (e.g. admin.go). Every table is just a func(*vento.Engine) - main.go
// calls each one directly after app.Use()-ing any app-specific middleware,
// so this package stays pure route declarations with no wiring logic of
// its own. It imports controllers, middleware, and vento only - never main
// - which keeps main -> routes -> controllers a one-way dependency graph
// instead of a cycle.
package routes

import (
	"vento-app/app/controllers"
	"vento-app/vento"
)

// Web maps every browser-facing page onto app - full HTML responses
// rendered via c.View, session/cookie-authenticated, CSRF-protected. This
// is the one place you add page routes as your app grows - keep it to
// route declarations only; app-specific middleware is Use()'d in main.go
// before Web is called, and Vento's own security stack is applied
// automatically by vento.New(). A single route can still take its own
// middleware as trailing arguments, e.g.:
//
//	app.GET("/admin", controllers.AdminIndex, middleware.RequireAuth)
func Web(app *vento.Engine) {
	app.GET("/", controllers.Index)
	// Add your page routes here.
}
