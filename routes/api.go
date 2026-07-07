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
// Handlers here should respond with c.JSON, not c.View. A single route can
// still take its own middleware as trailing arguments, e.g.:
//
//	app.GET("/api/users", controllers.UserIndex, middleware.RequireAPIToken)
func Api(app *vento.Engine) {
	app.GET("/api/health", controllers.Health)
	// Add your API routes here.
}
