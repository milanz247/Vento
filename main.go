// Command main bootstraps the Vento application: load config, connect
// MySQL, compile views, wire app-specific middleware, map every route
// table, and start the server. Handlers live in controllers, route tables
// in routes, and schema management in the CLI (vento db:migrate) - main.go
// is the one place that assembles them, so it stays a thin entry point.
package main

import (
	"log"

	"vento-app/app/middleware"
	"vento-app/routes"
	"vento-app/vento"
)

func main() {
	env := vento.LoadEnv(".env")

	// MySQL is the exclusive database provider: refuse to start without a
	// complete configuration and a live connection.
	dsn, ok := vento.BuildMySQLDSN(env)
	if !ok {
		log.Fatal("vento: DB_HOST/DB_USER/DB_NAME missing from .env - MySQL configuration is required")
	}

	app := vento.New() // pre-loaded with vento.DefaultMiddleware - see vento/kernel.go

	if err := app.ConnectDB(dsn); err != nil {
		log.Fatalf("vento: could not connect to MySQL: %v", err)
	}

	app.LoadHTMLGlob("views/**/*")

	// App-specific middleware, then every route table. Vento compiles each
	// route's handler chain at registration time, so Use must run first -
	// which the order below does naturally.
	app.Use(middleware.RequestID)
	if appKey := env["APP_KEY"]; appKey != "" {
		// Sessions is opt-in: it needs a signing secret, so it's only wired
		// in once APP_KEY is set in .env. See vento.Sessions and c.Session().
		app.Use(vento.Sessions(appKey))
	}
	routes.Web(app)
	routes.Api(app)

	// Static must be called after Use() so static asset requests get the
	// same global middleware coverage - Logger, Recovery, SecurityHeaders -
	// as routed requests.
	app.Static("/public", "./public")

	port := env["PORT"]
	if port == "" {
		port = "8080"
	}

	if err := app.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
