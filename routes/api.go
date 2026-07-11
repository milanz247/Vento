package routes

import (
	"vento-app/app/controllers"
	"vento-app/vento"
)

// Api maps every JSON endpoint onto app, all served under the /api prefix -
// Vento's equivalent of Laravel's routes/api.php. Handlers here respond
// with JSON (c.OK, c.Created, c.JSON), not c.View.
//
// Only /api/auth is CSRF-exempt in vento.DefaultMiddleware (see
// vento/kernel.go) - not all of /api. Auth's own routes (register/login)
// have no session yet to protect, so CSRF doesn't apply to them; every
// other /api route uses vento.RequireAuth, which is session-cookie-based,
// so CSRF protection has to actually run for it. A JSON client (a
// same-origin frontend) reads the vento_csrf cookie's value and echoes it
// back in the X-CSRF-Token header on POST/PUT/DELETE, exactly like a
// cookie-authenticated web route would.
//
// app.Group("/api") collects these routes under one prefix so it isn't
// repeated on every line; a single route can still take its own middleware
// as trailing arguments.
func Api(app *vento.Engine) {
	api := app.Group("/api")

	api.GET("/health", controllers.Health)

	auth := api.Group("/auth")
	auth.POST("/register", controllers.AuthRegister)
	auth.POST("/login", controllers.AuthLogin)
	auth.POST("/logout", controllers.AuthLogout)
	auth.GET("/me", controllers.AuthMe)

	// UserController is a full worked CRUD example - a real reference for
	// the pattern (paginated index, find-or-404 show/update/delete,
	// bind-or-422 create/update) to copy when adding your own resources.
	// Guarded by RequireAuth: every route below needs a logged-in session
	// (see /api/auth/login above) - list/read/write/delete access to user
	// records is not public.
	users := api.Group("", vento.RequireAuth)
	users.GET("/users", controllers.UserIndex)
	users.POST("/users", controllers.UserCreate)
	users.GET("/users/:id", controllers.UserShow)
	users.PUT("/users/:id", controllers.UserUpdate)
	users.DELETE("/users/:id", controllers.UserDelete)

	// Add your own API routes here. If a new route table needs its own
	// CSRF-exempt prefix (e.g. a genuinely token-authenticated webhook
	// endpoint), add it explicitly in vento/kernel.go's CSRFProtection
	// call - don't assume "/api" covers it.
}
