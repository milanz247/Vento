package routes

import (
	"vento-app/app/controllers"
	"vento-app/vento"
)

// Api maps every JSON endpoint onto app, all served under the /api prefix -
// Vento's equivalent of Laravel's routes/api.php. Kept separate from Web
// because API routes have different defaults: they're expected to be
// token-authenticated rather than cookie/session-authenticated, which is
// why "/api" is exempted from CSRFProtection in vento.DefaultMiddleware.
// Handlers here respond with JSON (c.OK, c.Created, c.JSON), not c.View.
//
// app.Group("/api") collects these routes under one prefix so it isn't
// repeated on every line; shared middleware for the whole group would go in
// the Group call (e.g. app.Group("/api", middleware.RequireAPIToken)), and
// a single route can still take its own middleware as trailing arguments.
func Api(app *vento.Engine) {
	api := app.Group("/api")

	api.GET("/health", controllers.Health)

	// UserController is a full worked CRUD example - a real reference for
	// the pattern (paginated index, find-or-404 show/update/delete,
	// bind-or-422 create/update) to copy when adding your own resources.
	api.GET("/users", controllers.UserIndex)
	api.POST("/users", controllers.UserCreate)
	api.GET("/users/:id", controllers.UserShow)
	api.PUT("/users/:id", controllers.UserUpdate)
	api.DELETE("/users/:id", controllers.UserDelete)

	// Add your own API routes here.
}
