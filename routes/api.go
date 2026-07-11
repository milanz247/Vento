package routes

import (
	"vento-app/app/controllers"
	"vento-app/vento"
)

// Api maps every JSON endpoint onto app, all served under the /api prefix -
// Vento's equivalent of Laravel's routes/api.php. Handlers here respond
// with JSON (c.OK, c.Created, c.JSON), not c.View.
//
// app.Group("/api") collects these routes under one prefix so it isn't
// repeated on every line; a single route can still take its own middleware
// as trailing arguments.
func Api(app *vento.Engine) {
	api := app.Group("/api")

	api.GET("/health", controllers.Health)

	// Add your own API routes here. vento.RequireAuth (session-cookie
	// based) is available to guard a group once you add authenticated
	// routes - see vento/auth.go - and CSRF protection already runs
	// globally via vento.DefaultMiddleware (see vento/kernel.go).
}
