// Package routes declares the application's HTTP surface, mirroring Laravel:
// the global middleware stack lives in kernel.go (like app/Http/Kernel.php),
// and the route table lives here (like routes/web.php). It imports
// controllers, middleware, and vento only - never main - which keeps
// main -> routes -> controllers a one-way dependency graph instead of a
// cycle.
package routes

import (
	"vento-app/controllers"
	"vento-app/vento"
)

// web maps every endpoint onto app. This is the one place you add routes as
// your app grows - keep it to route declarations only; the global middleware
// stack is defined in kernel.go (GlobalMiddleware). A single route can still
// take its own middleware as trailing arguments, e.g.:
//
//	app.GET("/admin", controllers.AdminIndex, middleware.RequireAuth)
func web(app *vento.Engine) {
	app.GET("/", controllers.Index)
	// Add your routes here.
}
