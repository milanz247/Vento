// Command main bootstraps the Vento application: load config, connect
// MySQL, compile views, register routes, and start the server. Handlers
// live in controllers, route registration in routes, and schema
// management in the CLI (vento db:migrate) - main.go stays a thin entry
// point.
package main

import (
	"log"

	"vento-app/vento"
	"vento-app/routes"
)

func main() {
	env := vento.LoadEnv(".env")

	// MySQL is the exclusive database provider: refuse to start without a
	// complete configuration and a live connection.
	dsn, ok := vento.BuildMySQLDSN(env)
	if !ok {
		log.Fatal("vento: DB_HOST/DB_USER/DB_NAME missing from .env - MySQL configuration is required")
	}

	app := vento.New()

	if err := app.ConnectDB(dsn); err != nil {
		log.Fatalf("vento: could not connect to MySQL: %v", err)
	}

	app.LoadHTMLGlob("views/**/*")

	routes.RegisterRoutes(app)

	// Static must be called after RegisterRoutes (which calls Use first) so
	// static asset requests get the same global middleware coverage -
	// Logger, Recovery, SecurityHeaders - as routed requests.
	app.Static("/public", "./public")

	port := env["PORT"]
	if port == "" {
		port = "8080"
	}

	if err := app.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
