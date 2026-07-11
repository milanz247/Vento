// Command vento is the framework's Artisan-style developer tool. Run it
// from the project root (it reads ./.env and writes under the app packages):
//
//	vento run                       start the app (hot-reload via air when available)
//	vento route:list                list every registered route
//	vento env:check                 validate .env and test the MySQL connection
//	vento db:migrate                apply every pending migration (offers to create
//	                               the database first if it doesn't exist yet)
//	vento db:rollback               revert the most recently applied migration
//	vento db:status                 show which migrations have applied vs are pending
//	vento db:fresh                  drop every table, then re-run every migration (destructive)
//	vento db:automigrate            GORM AutoMigrate every model in models.All() (dev)
//	vento db:seed                   run all registered seeders (idempotent)
//	vento make:controller Name      scaffold app/controllers/name_controller.go (single stub handler)
//	vento make:resource Name        scaffold a full CRUD controller + model + test + route hints
//	vento make:model Name           scaffold app/models/name.go
//	vento make:middleware Name      scaffold app/middleware/name.go
//	vento make:migration name       scaffold migrations/<timestamp>_name.go
//	vento make:seeder Name          scaffold app/seeders/name_seeder.go
//	vento make:test Name            scaffold a controller test file (see vento/vtest)
//	vento version                   print the CLI version
package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"vento-app/app/middleware"
	"vento-app/app/models"
	"vento-app/app/seeders"
	"vento-app/migrations"
	"vento-app/routes"
	"vento-app/vento"
	"vento-app/vento/config"
	"vento-app/vento/migrate"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jinzhu/inflection"
	"gorm.io/gorm"
)

// ANSI escape sequences for colored terminal output.
const (
	reset  = "\033[0m"
	red    = "\033[31m"
	green  = "\033[32m"
	yellow = "\033[33m"
	cyan   = "\033[36m"
	bold   = "\033[1m"
)

const banner = cyan + bold + `
__     __  _____   _   _  _____    ___
\ \   / / | ____| | \ | ||_   _|  / _ \
 \ \ / /  |  _|   |  \| |  | |   | | | |
  \ V /   | |___  | |\  |  | |   | |_| |
   \_/    |_____| |_| \_|  |_|    \___/
` + reset + `        high-performance Go web framework
`

// cliVersion is the CLI's own version, printed by `vento version`. Bump it
// alongside notable framework changes.
const cliVersion = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		fmt.Print(banner)
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "run", "serve":
		fmt.Print(banner)
		runApp()
	case "route:list":
		routeList()
	case "env:check", "doctor":
		envCheck()
	case "db:migrate":
		runMigrations(openDB(true))
	case "db:rollback":
		rollback(openDB(false))
	case "db:status":
		dbStatus(openDB(false))
	case "db:fresh":
		dbFresh(openDB(false))
	case "db:automigrate":
		autoMigrate(openDB(true))
	case "db:seed":
		seed(openDB(false))
	case "make:controller":
		requireName("make:controller", "Post")
		makeController(os.Args[2])
	case "make:resource":
		requireName("make:resource", "Post")
		makeResource(os.Args[2])
	case "make:model":
		requireName("make:model", "Post")
		makeModel(os.Args[2])
	case "make:middleware":
		requireName("make:middleware", "RequireAuth")
		makeMiddleware(os.Args[2])
	case "make:migration":
		requireName("make:migration", "create_posts_table")
		makeMigration(os.Args[2])
	case "make:seeder":
		requireName("make:seeder", "Products")
		makeSeeder(os.Args[2])
	case "make:test":
		requireName("make:test", "Post")
		makeTest(os.Args[2])
	case "version", "--version", "-v":
		fmt.Println("vento " + cliVersion)
	case "help", "--help", "-h":
		fmt.Print(banner)
		usage()
	default:
		fmt.Print(banner)
		fmt.Fprintln(os.Stderr, red+"unknown command:"+reset+" "+os.Args[1])
		usage()
		os.Exit(1)
	}
}

// requireName aborts with a usage hint when a make: command was invoked
// without its trailing name argument.
func requireName(cmd, example string) {
	if len(os.Args) < 3 {
		fail(cmd + " requires a name, e.g. vento " + cmd + " " + example)
	}
}

func usage() {
	fmt.Println(bold + "usage:" + reset + ` vento <command>

  ` + bold + `Serving` + reset + `
  ` + green + `run | serve` + reset + `             start the app (hot-reload via air when installed)
  ` + green + `route:list` + reset + `              list every registered route
  ` + green + `env:check` + reset + `               validate .env and test the MySQL connection

  ` + bold + `Database` + reset + `
  ` + green + `db:migrate` + reset + `              apply every pending migration
  ` + green + `db:rollback` + reset + `             revert the most recently applied migration
  ` + green + `db:status` + reset + `               show which migrations have applied vs are pending
  ` + green + `db:fresh` + reset + `                drop every table, then re-run every migration (destructive)
  ` + green + `db:automigrate` + reset + `          GORM AutoMigrate every model in models.All() (dev shortcut)
  ` + green + `db:seed` + reset + `                 run all registered seeders (safe to re-run)

  ` + bold + `Code generation` + reset + `
  ` + green + `make:controller <Name>` + reset + `  scaffold app/controllers/<name>_controller.go (single stub handler)
  ` + green + `make:resource <Name>` + reset + `    scaffold a full CRUD controller + model + test + route hints
  ` + green + `make:model <Name>` + reset + `       scaffold app/models/<name>.go
  ` + green + `make:middleware <Name>` + reset + `  scaffold app/middleware/<name>.go
  ` + green + `make:migration <name>` + reset + `   scaffold migrations/<timestamp>_<name>.go
  ` + green + `make:seeder <Name>` + reset + `      scaffold app/seeders/<name>_seeder.go
  ` + green + `make:test <Name>` + reset + `        scaffold a controller test file (see vento/vtest)

  ` + bold + `Other` + reset + `
  ` + green + `version` + reset + `                 print the CLI version`)
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, red+"error:"+reset+" "+msg)
	os.Exit(1)
}

// confirm prints prompt and reads a y/yes answer from stdin, case-
// insensitively. Every destructive command (dropping a database, dropping
// every table) routes through here rather than proceeding silently.
func confirm(prompt string) bool {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
}

// runApp starts the application, preferring air for live hot-reloading
// when it is installed, and falling back to a plain `go run .` otherwise.
func runApp() {
	ensureCSSBuilt()

	var cmd *exec.Cmd
	if airPath, err := exec.LookPath("air"); err == nil {
		fmt.Println(green + "air detected" + reset + " - starting with hot-reload (.air.toml)")
		cmd = exec.Command(airPath)
	} else {
		fmt.Println(yellow + "air not found in PATH" + reset + ` - starting without hot-reload.
For live reloading, install air:

    go install github.com/air-verse/air@latest`)
		cmd = exec.Command("go", "run", ".")
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fail(err.Error())
	}
}

// ensureCSSBuilt runs the Tailwind CSS build once at startup so the app
// never serves an unstyled page just because public/css/app.css was never
// generated. It's best-effort and silent when nothing needs doing: if npm
// isn't installed, or the build fails, vento run still proceeds - a broken
// CSS build shouldn't block starting the server.
func ensureCSSBuilt() {
	if _, err := exec.LookPath("npm"); err != nil {
		return
	}
	if _, err := os.Stat("node_modules"); os.IsNotExist(err) {
		fmt.Println("installing frontend dependencies (npm install) ...")
		install := exec.Command("npm", "install")
		install.Stdout = os.Stdout
		install.Stderr = os.Stderr
		if err := install.Run(); err != nil {
			fmt.Fprintln(os.Stderr, yellow+"warning:"+reset+" npm install failed: "+err.Error())
			return
		}
	}

	build := exec.Command("npm", "run", "build:css")
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintln(os.Stderr, yellow+"warning:"+reset+" CSS build failed: "+err.Error())
	}
}

// routeList boots an Engine exactly as main.go does - same middleware,
// same route tables, same static mount - but never connects to a database
// or starts the server, since listing routes needs nothing but the
// compiled route table. That means route:list works even when MySQL is
// unreachable, which is exactly when a developer is most likely to reach
// for it. Keep this in sync with main.go's bootstrap sequence if that
// changes, or route:list's output will quietly drift from what the app
// actually serves.
func routeList() {
	env := config.LoadEnv(".env")

	app := vento.New()
	app.Use(middleware.RequestID)
	if env["APP_KEY"] != "" {
		app.Use(vento.Sessions(env["APP_KEY"]))
	}
	routes.Web(app)
	routes.Api(app)
	app.Static("/public", "./public")

	list := app.Routes()
	staticMounts := app.StaticMounts()
	if len(list) == 0 && len(staticMounts) == 0 {
		fmt.Println(yellow + "no routes registered" + reset)
		return
	}

	methodWidth, pathWidth := len("METHOD"), len("PATH")
	for _, r := range list {
		methodWidth = max(methodWidth, len(r.Method))
		pathWidth = max(pathWidth, len(r.Path))
	}

	fmt.Printf(bold+"  %-*s  %-*s  %s"+reset+"\n", methodWidth, "METHOD", pathWidth, "PATH", "HANDLERS")
	for _, r := range list {
		fmt.Printf("  "+green+"%-*s"+reset+"  %-*s  %d\n", methodWidth, r.Method, pathWidth, r.Path, r.HandlerCount)
	}
	for _, prefix := range staticMounts {
		fmt.Printf("  "+cyan+"%-*s"+reset+"  %-*s  -\n", methodWidth, "STATIC", pathWidth, prefix+"*")
	}
	fmt.Printf("\n%d route(s), %d static mount(s)\n", len(list), len(staticMounts))
}

// envCheck validates the local .env against what the app needs to start
// (MySQL configuration, APP_KEY for sessions) and, if the required keys
// are present, attempts an actual MySQL connection - failing fast
// (DBConnectRetries=0) rather than waiting through the server's normal
// startup backoff, since this command is meant for a quick yes/no answer
// at a developer's terminal.
func envCheck() {
	fmt.Println(bold + "environment check" + reset)

	if _, err := os.Stat(".env"); err != nil {
		fmt.Println(red + "✗" + reset + " .env not found - copy .env.example to .env and fill in real values")
		os.Exit(1)
	}
	fmt.Println(green + "✓" + reset + " .env found")

	env := config.LoadEnv(".env")

	missing := false
	for _, key := range []string{"DB_HOST", "DB_USER", "DB_NAME"} {
		if env[key] == "" {
			fmt.Println(red + "✗" + reset + " " + key + " is not set")
			missing = true
		}
	}
	if !missing {
		fmt.Println(green + "✓" + reset + " DB_HOST/DB_USER/DB_NAME are set")
	}

	if env["APP_KEY"] == "" {
		fmt.Println(yellow + "!" + reset + " APP_KEY is not set - vento.Sessions (and anything using c.Session(), including auth) will not persist across requests. Generate one with: openssl rand -hex 32")
	} else {
		fmt.Println(green + "✓" + reset + " APP_KEY is set")
	}

	port := env["PORT"]
	if port == "" {
		port = "8080 (default)"
	}
	fmt.Println(green + "✓" + reset + " PORT=" + port)

	if missing {
		fmt.Println()
		fmt.Println(yellow + "skipping MySQL connectivity check - required keys are missing" + reset)
		os.Exit(1)
	}

	dsn, ok := config.BuildMySQLDSN(env)
	if !ok {
		return
	}

	origRetries := vento.DBConnectRetries
	vento.DBConnectRetries = 0 // fail fast: this is an interactive check, not a server startup
	defer func() { vento.DBConnectRetries = origRetries }()

	app := vento.New()
	if err := app.ConnectDB(dsn); err != nil {
		fmt.Println(red + "✗" + reset + " could not connect to MySQL: " + err.Error())
		os.Exit(1)
	}
	fmt.Println(green + "✓" + reset + " connected to MySQL at " + env["DB_HOST"])
}

// openDB connects to MySQL using the DB_* keys in .env. MySQL is the
// exclusive database provider: if it is unconfigured or unreachable the
// command aborts rather than silently working against a different store.
// When createIfMissing is true (db:migrate), the target database is
// created first - interactively, with the user's confirmation - if it
// doesn't already exist, so a fresh checkout can migrate straight away
// instead of failing with a cryptic "unknown database" error.
//
// Connection retries are disabled (DBConnectRetries=0): every caller here
// is an interactive command run at a developer's terminal, where a fast
// failure ("MySQL isn't running") beats the ~30s backoff Run's normal
// server-startup path uses to ride out container startup races.
func openDB(createIfMissing bool) *gorm.DB {
	env := config.LoadEnv(".env")
	if env["DB_HOST"] == "" || env["DB_USER"] == "" || env["DB_NAME"] == "" {
		fail("DB_HOST/DB_USER/DB_NAME missing from .env - MySQL configuration is required")
	}

	if createIfMissing {
		port := env["DB_PORT"]
		if port == "" {
			port = "3306"
		}
		ensureDatabaseExists(env["DB_USER"], env["DB_PASSWORD"], env["DB_HOST"], port, env["DB_NAME"])
	}

	dsn, ok := config.BuildMySQLDSN(env)
	if !ok {
		fail("DB_HOST/DB_USER/DB_NAME missing from .env - MySQL configuration is required")
	}

	vento.DBConnectRetries = 0
	app := vento.New()
	if err := app.ConnectDB(dsn); err != nil {
		fail("could not connect to MySQL: " + err.Error())
	}
	return app.DB
}

// isValidDBName restricts database names accepted by ensureDatabaseExists
// to letters, digits, and underscores, so name can be interpolated into a
// CREATE DATABASE statement (which cannot use placeholder args) without
// risking SQL injection from a hand-edited .env file.
func isValidDBName(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return true
}

// ensureDatabaseExists checks whether the configured MySQL database
// already exists and, if not, asks the developer for confirmation before
// creating it - connecting to the MySQL server itself (no database
// selected) rather than the target DSN, since that would fail before the
// database exists.
func ensureDatabaseExists(user, password, host, port, name string) {
	if !isValidDBName(name) {
		fail("DB_NAME " + name + " is not a valid database name (letters, digits, underscore only)")
	}

	serverDSN := fmt.Sprintf("%s:%s@tcp(%s:%s)/", user, password, host, port)
	conn, err := sql.Open("mysql", serverDSN)
	if err != nil {
		fail("could not open MySQL connection: " + err.Error())
	}
	defer conn.Close()

	var found string
	err = conn.QueryRow(
		"SELECT SCHEMA_NAME FROM INFORMATION_SCHEMA.SCHEMATA WHERE SCHEMA_NAME = ?", name,
	).Scan(&found)
	if err == nil {
		return // database already exists
	}
	if err != sql.ErrNoRows {
		fail("could not check for database " + name + ": " + err.Error())
	}

	if !confirm(fmt.Sprintf(yellow+"database %q does not exist."+reset+" Create it? [y/N]: ", name)) {
		fail("aborted - database " + name + " does not exist")
	}

	_, err = conn.Exec(fmt.Sprintf(
		"CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", name,
	))
	if err != nil {
		fail("creating database " + name + ": " + err.Error())
	}
	fmt.Println(green + "created" + reset + " database " + name)
}

// runMigrations applies every pending migration in migrations.All(),
// recording each in the schema_migrations table so it never runs twice.
func runMigrations(db *gorm.DB) {
	applied, err := migrate.Run(db, migrations.All())
	if err != nil {
		fail(err.Error())
	}
	if len(applied) == 0 {
		fmt.Println(green + "nothing to migrate" + reset + " - database is up to date")
		return
	}
	for _, id := range applied {
		fmt.Printf(green+"migrated"+reset+" %s\n", id)
	}
	fmt.Println(bold + "db:migrate complete" + reset)
}

// rollback reverts the most recently applied migration by running its Down
// function and deleting its schema_migrations row.
func rollback(db *gorm.DB) {
	id, err := migrate.RollbackLast(db, migrations.All())
	if err != nil {
		fail(err.Error())
	}
	if id == "" {
		fmt.Println(green + "nothing to roll back" + reset + " - no migrations applied")
		return
	}
	fmt.Printf(green+"rolled back"+reset+" %s\n", id)
	fmt.Println(bold + "db:rollback complete" + reset)
}

// dbStatus prints every registered migration's applied/pending state, so a
// developer can see what db:migrate would do without running it.
func dbStatus(db *gorm.DB) {
	status, err := migrate.Status(db, migrations.All())
	if err != nil {
		fail(err.Error())
	}
	if len(status) == 0 {
		fmt.Println(yellow + "no migrations registered" + reset)
		return
	}

	pending := 0
	for _, s := range status {
		if s.Applied {
			fmt.Printf("  "+green+"applied"+reset+"   %s "+yellow+"(%s)"+reset+"\n",
				s.ID, s.AppliedAt.Local().Format("2006-01-02 15:04:05"))
		} else {
			fmt.Printf("  "+red+"pending"+reset+"   %s\n", s.ID)
			pending++
		}
	}
	fmt.Println()
	if pending == 0 {
		fmt.Println(green + "database is up to date" + reset)
	} else {
		fmt.Printf(yellow+"%d migration(s) pending"+reset+" - run vento db:migrate\n", pending)
	}
}

// dbFresh drops every table in the configured database, then re-runs every
// migration from scratch - a dev-only "start over" command (Laravel's
// migrate:fresh). It always asks for confirmation first: this is the most
// destructive command in the CLI, and unlike db:rollback (which reverts
// one known, deliberate step) there's no equivalent "undo" for it.
func dbFresh(db *gorm.DB) {
	if !confirm(yellow + "db:fresh drops every table in the configured database, then re-runs every migration.\nThis cannot be undone. Continue? [y/N]: " + reset) {
		fail("aborted")
	}

	var tables []string
	if err := db.Raw("SHOW TABLES").Scan(&tables).Error; err != nil {
		fail("listing tables: " + err.Error())
	}
	if len(tables) == 0 {
		fmt.Println(green + "no tables to drop" + reset)
	} else {
		if err := db.Exec("SET FOREIGN_KEY_CHECKS = 0").Error; err != nil {
			fail("disabling foreign key checks: " + err.Error())
		}
		for _, t := range tables {
			if err := db.Exec(fmt.Sprintf("DROP TABLE `%s`", t)).Error; err != nil {
				fail("dropping table " + t + ": " + err.Error())
			}
			fmt.Printf(green+"dropped"+reset+" %s\n", t)
		}
		if err := db.Exec("SET FOREIGN_KEY_CHECKS = 1").Error; err != nil {
			fail("re-enabling foreign key checks: " + err.Error())
		}
	}

	runMigrations(db)
}

// autoMigrate runs GORM AutoMigrate over every model in models.All() - an
// untracked, additive schema sync handy for rapid prototyping. Prefer
// versioned migrations (db:migrate) once the schema needs ordered,
// reversible history.
func autoMigrate(db *gorm.DB) {
	all := models.All()
	if err := migrate.AutoMigrateModels(db, all); err != nil {
		fail(err.Error())
	}
	for _, model := range all {
		fmt.Printf(green+"auto-migrated"+reset+" %T\n", model)
	}
	fmt.Println(bold + "db:automigrate complete" + reset)
}

// seed runs every seeder registered in app/seeders (see that package's
// doc comment - each seeder self-registers via an init(), so make:seeder
// is all a developer needs to add one).
func seed(db *gorm.DB) {
	all := seeders.All()
	if len(all) == 0 {
		fmt.Println(yellow + "no seeders registered" + reset + " - vento make:seeder <Name> to add one")
		return
	}
	for _, s := range all {
		if err := s.Run(db); err != nil {
			fail(fmt.Sprintf("seeder %q failed: %v", s.Name, err))
		}
		fmt.Printf(green+"seeded"+reset+" %s\n", s.Name)
	}
	fmt.Println(bold + "db:seed complete" + reset)
}

// controllerStub is the boilerplate written by make:controller. %[1]s is
// the StudlyCase name (e.g. Post), %[2]s its lowercase plural form.
const controllerStub = `package controllers

import (
	"net/http"

	"vento-app/vento"
)

// %[1]sIndex handles GET /%[2]s: list all %[2]s.
func %[1]sIndex(c *vento.Context) {
	c.JSON(http.StatusOK, map[string]string{"message": "%[1]sIndex: not implemented"})
}

// %[1]sShow handles GET /%[2]s/:id: show one %[1]s.
func %[1]sShow(c *vento.Context) {
	c.JSON(http.StatusOK, map[string]string{
		"id":      c.Param("id"),
		"message": "%[1]sShow: not implemented",
	})
}

// %[1]sStore handles POST /%[2]s: create a %[1]s.
func %[1]sStore(c *vento.Context) {
	c.JSON(http.StatusCreated, map[string]string{"message": "%[1]sStore: not implemented"})
}
`

// makeController scaffolds controllers/<snake>_controller.go containing
// stubbed Index/Show/Store handlers named after the StudlyCase name. For a
// full working CRUD controller wired to a model, use make:resource
// instead - this command is for a single ad-hoc handler group you intend
// to fill in by hand.
func makeController(rawName string) {
	studly := studlyCase(rawName)
	if studly == "" {
		fail(fmt.Sprintf("%q is not a usable controller name (letters/digits only)", rawName))
	}
	path := filepath.Join("app", "controllers", snakeCase(studly)+"_controller.go")
	writeScaffold(path, fmt.Sprintf(controllerStub, studly, inflection.Plural(strings.ToLower(studly))))
}

// resourceControllerStub is written by make:resource: a complete,
// working CRUD controller built on vento.Query[T]/vento.Model[T]/
// c.BindOrAbort - the same pattern app/controllers/user_controller.go
// uses, not a set of "not implemented" stubs. %[1]s is the StudlyCase
// name, %[2]s its lowercase plural form (used in doc comments/route
// hints only), %[3]s a lowerCamel variable name.
const resourceControllerStub = `package controllers

import (
	"vento-app/app/models"
	"vento-app/vento"
)

// %[1]sForm is what %[1]sCreate and %[1]sUpdate bind the request body into -
// only the fields a client is allowed to set. Add validate:"..." tags as
// needed (see vento.Validate), and keep it separate from models.%[1]s so a
// client payload can never overwrite gorm.Model's ID/CreatedAt/UpdatedAt/
// DeletedAt columns.
//
//	Name string ` + "`" + `json:"name" form:"name" validate:"required,min=2,max=100"` + "`" + `
type %[1]sForm struct {
}

// %[1]sIndex handles GET /%[2]s - a paginated list, e.g. /%[2]s?page=2&per_page=10.
func %[1]sIndex(c *vento.Context) {
	page, err := vento.Query[models.%[1]s](c).Paginate(c.QueryInt("page", 1), c.QueryInt("per_page", vento.DefaultPerPage))
	if err != nil {
		c.InternalError("could not load %[2]s")
		return
	}
	c.OK(page)
}

// %[1]sShow handles GET /%[2]s/:id.
func %[1]sShow(c *vento.Context) {
	%[3]s, ok := vento.Model[models.%[1]s](c, "id")
	if !ok {
		return
	}
	c.OK(%[3]s)
}

// %[1]sCreate handles POST /%[2]s.
func %[1]sCreate(c *vento.Context) {
	var form %[1]sForm
	if !c.BindOrAbort(&form) {
		return
	}

	%[3]s := models.%[1]s{
		// Map form fields onto the model here, e.g. Name: form.Name.
	}
	if !vento.Query[models.%[1]s](c).CreateOrAbort(&%[3]s) {
		return
	}
	c.Created(%[3]s)
}

// %[1]sUpdate handles PUT /%[2]s/:id.
func %[1]sUpdate(c *vento.Context) {
	%[3]s, ok := vento.Model[models.%[1]s](c, "id")
	if !ok {
		return
	}

	var form %[1]sForm
	if !c.BindOrAbort(&form) {
		return
	}

	// Apply form fields onto %[3]s here before saving, e.g. %[3]s.Name = form.Name.
	if !vento.Query[models.%[1]s](c).SaveOrAbort(%[3]s) {
		return
	}
	c.OK(%[3]s)
}

// %[1]sDelete handles DELETE /%[2]s/:id.
func %[1]sDelete(c *vento.Context) {
	%[3]s, ok := vento.Model[models.%[1]s](c, "id")
	if !ok {
		return
	}
	if !vento.Query[models.%[1]s](c).DeleteOrAbort(%[3]s) {
		return
	}
	c.NoContent()
}
`

// resourceTestStub is written by make:resource (as
// <name>_controller_test.go alongside the controller) and by the
// standalone make:test - a working test file exercising the generated
// controller via vento/vtest and an in-memory sqlite database, following
// exactly the pattern vento/vtest/vtest_test.go itself uses against
// app/controllers/user_controller.go.
const resourceTestStub = `package controllers_test

import (
	"net/http"
	"testing"

	"vento-app/app/controllers"
	"vento-app/app/models"
	"vento-app/vento/vtest"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func new%[1]sTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("opening in-memory sqlite: %%v", err)
	}
	if err := db.AutoMigrate(&models.%[1]s{}); err != nil {
		t.Fatalf("migrating %[1]s: %%v", err)
	}
	return db
}

func Test%[1]sIndex(t *testing.T) {
	db := new%[1]sTestDB(t)
	c, rec := vtest.NewContext(http.MethodGet, "/%[2]s", nil, nil)
	c.SetDB(db)

	controllers.%[1]sIndex(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %%d: %%s", rec.Code, rec.Body)
	}
}

func Test%[1]sShowNotFound(t *testing.T) {
	db := new%[1]sTestDB(t)
	c, rec := vtest.NewContext(http.MethodGet, "/%[2]s/999", nil, map[string]string{"id": "999"})
	c.SetDB(db)

	controllers.%[1]sShow(c)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %%d: %%s", rec.Code, rec.Body)
	}
}

func Test%[1]sCreate(t *testing.T) {
	db := new%[1]sTestDB(t)
	// TODO: %[1]sForm{} is sent empty here. If it has validate:"required,..."
	// tags, fill in valid values for them, then tighten the assertion below
	// to http.StatusCreated - this generated version only checks that
	// Create handles the request without crashing, since it can't know
	// what your form's required fields are.
	c, rec := vtest.NewContext(http.MethodPost, "/%[2]s", controllers.%[1]sForm{}, nil)
	c.SetDB(db)

	controllers.%[1]sCreate(c)

	if rec.Code == http.StatusInternalServerError {
		t.Fatalf("expected %[1]sCreate to handle the request without a server error, got 500: %%s", rec.Body)
	}
}
`

// makeResource scaffolds a complete, working CRUD resource in one command:
// a controller built on vento.Query[T]/vento.Model[T] (resourceControllerStub),
// a model (modelStub, skipped if one already exists at that path rather
// than failing the whole command), and a test file (resourceTestStub) -
// then prints the route:: lines to paste into routes/api.go, since
// registering routes is app-specific wiring the CLI shouldn't guess at or
// silently rewrite routes/api.go to inject.
func makeResource(rawName string) {
	studly := studlyCase(rawName)
	if studly == "" {
		fail(fmt.Sprintf("%q is not a usable resource name (letters/digits only)", rawName))
	}
	lower := strings.ToLower(studly)
	plural := inflection.Plural(lower)
	varName := lowerCamel(studly)

	writeScaffold(
		filepath.Join("app", "controllers", snakeCase(studly)+"_controller.go"),
		fmt.Sprintf(resourceControllerStub, studly, plural, varName),
	)

	modelPath := filepath.Join("app", "models", snakeCase(studly)+".go")
	if _, err := os.Stat(modelPath); err != nil {
		writeScaffold(modelPath, fmt.Sprintf(modelStub, studly))
		fmt.Println(yellow + "next:" + reset + " add &" + studly + "{} to models.All() in app/models/user.go")
	} else {
		fmt.Println(yellow + "skipped:" + reset + " " + modelPath + " already exists")
	}

	writeScaffold(
		filepath.Join("app", "controllers", snakeCase(studly)+"_controller_test.go"),
		fmt.Sprintf(resourceTestStub, studly, plural, varName),
	)

	fmt.Println(yellow + "next:" + reset + " wire the routes in routes/api.go:")
	fmt.Printf("    api.GET(\"/%s\", controllers.%sIndex)\n", plural, studly)
	fmt.Printf("    api.POST(\"/%s\", controllers.%sCreate)\n", plural, studly)
	fmt.Printf("    api.GET(\"/%s/:id\", controllers.%sShow)\n", plural, studly)
	fmt.Printf("    api.PUT(\"/%s/:id\", controllers.%sUpdate)\n", plural, studly)
	fmt.Printf("    api.DELETE(\"/%s/:id\", controllers.%sDelete)\n", plural, studly)
}

// makeTest scaffolds a standalone controller test file (resourceTestStub)
// for a controller that already exists - the make:resource split-out for
// when a developer wrote (or hand-edited) a controller and wants the
// matching test skeleton without regenerating the controller itself. It
// assumes the resource-style handler names (%[1]sIndex/%[1]sShow/
// %[1]sCreate) make:resource and make:controller both produce; adjust the
// generated file if a controller's handlers are named differently.
func makeTest(rawName string) {
	studly := studlyCase(rawName)
	if studly == "" {
		fail(fmt.Sprintf("%q is not a usable name (letters/digits only)", rawName))
	}
	lower := strings.ToLower(studly)
	plural := inflection.Plural(lower)
	varName := lowerCamel(studly)

	path := filepath.Join("app", "controllers", snakeCase(studly)+"_controller_test.go")
	writeScaffold(path, fmt.Sprintf(resourceTestStub, studly, plural, varName))
	fmt.Println(yellow + "note:" + reset + " this test assumes " + studly + "Index/" + studly + "Show/" + studly + "Create exist (as make:resource generates) - adjust if your controller's handlers differ")
}

// modelStub is written by make:model. %[1]s is the StudlyCase name.
const modelStub = `package models

import "gorm.io/gorm"

// %[1]s is a GORM model. Add fields below, then register it in All()
// (models/user.go) so db:automigrate and the seeders can see it - or define
// its table in a migration instead (vento make:migration).
type %[1]s struct {
	gorm.Model
}
`

// makeModel scaffolds models/<snake>.go with a GORM model stub.
func makeModel(rawName string) {
	studly := studlyCase(rawName)
	if studly == "" {
		fail(fmt.Sprintf("%q is not a usable model name (letters/digits only)", rawName))
	}
	path := filepath.Join("app", "models", snakeCase(studly)+".go")
	writeScaffold(path, fmt.Sprintf(modelStub, studly))
	fmt.Println(yellow + "next:" + reset + " add &" + studly + "{} to models.All() in app/models/user.go")
}

// middlewareStub is written by make:middleware. %[1]s is the StudlyCase name.
const middlewareStub = `package middleware

import "vento-app/vento"

// %[1]s is a middleware: it runs on each request, then calls c.Next() to
// hand control to the rest of the chain. Wire it in routes/web.go - globally
// with app.Use(middleware.%[1]s), or per route as a trailing argument to
// app.GET/POST/... To gatekeep, call c.Abort(code, msg) and return instead
// of calling c.Next().
func %[1]s(c *vento.Context) {
	// ... your logic here ...
	c.Next()
}
`

// makeMiddleware scaffolds middleware/<snake>.go with a HandlerFunc stub.
func makeMiddleware(rawName string) {
	studly := studlyCase(rawName)
	if studly == "" {
		fail(fmt.Sprintf("%q is not a usable middleware name (letters/digits only)", rawName))
	}
	path := filepath.Join("app", "middleware", snakeCase(studly)+".go")
	writeScaffold(path, fmt.Sprintf(middlewareStub, studly))
	fmt.Println(yellow + "next:" + reset + " add middleware." + studly + " to app.Use(...) in main.go, or attach it to a route in routes/web.go")
}

// migrationStub is written by make:migration. %[1]s is the full migration
// ID (timestamp prefix + snake_case name).
const migrationStub = `package migrations

import (
	"gorm.io/gorm"

	"vento-app/vento/migrate"
)

// %[1]s
func init() {
	register(migrate.Migration{
		ID: "%[1]s",
		Up: func(tx *gorm.DB) error {
			// Apply the change, e.g.:
			//   return tx.AutoMigrate(&models.Post{})
			return nil
		},
		Down: func(tx *gorm.DB) error {
			// Reverse the change, e.g.:
			//   return tx.Migrator().DropTable(&models.Post{})
			return nil
		},
	})
}
`

// makeMigration scaffolds a timestamped, self-registering migration under
// migrations/. The name is normalized to snake_case and prefixed with a UTC
// timestamp so files sort chronologically - which is the order db:migrate
// applies them in.
func makeMigration(rawName string) {
	slug := snakeCase(studlyCase(rawName))
	if slug == "" {
		fail(fmt.Sprintf("%q is not a usable migration name (letters/digits only)", rawName))
	}
	id := time.Now().UTC().Format("20060102_150405") + "_" + slug
	path := filepath.Join("migrations", id+".go")
	writeScaffold(path, fmt.Sprintf(migrationStub, id))
}

// seederStub is written by make:seeder. %[1]s is the StudlyCase name,
// %[2]s its lowercase form used as the seeder's registered Name.
const seederStub = `package seeders

import "gorm.io/gorm"

func init() {
	register(Seeder{Name: "%[2]s", Run: seed%[1]s})
}

// seed%[1]s inserts starter data, keyed so re-running db:seed is a no-op
// for rows that already exist (FirstOrCreate or equivalent).
func seed%[1]s(db *gorm.DB) error {
	// Add "vento-app/app/models" to the imports above, then e.g.:
	//   rows := []models.%[1]s{{ /* ... */ }}
	//   for i := range rows {
	//       if err := db.Where(models.%[1]s{ /* unique field */ }).FirstOrCreate(&rows[i]).Error; err != nil {
	//           return err
	//       }
	//   }
	return nil
}
`

// makeSeeder scaffolds a self-registering seeder under app/seeders/,
// following the same init()-based registration pattern as make:migration.
func makeSeeder(rawName string) {
	studly := studlyCase(rawName)
	if studly == "" {
		fail(fmt.Sprintf("%q is not a usable seeder name (letters/digits only)", rawName))
	}
	lower := strings.ToLower(studly)
	path := filepath.Join("app", "seeders", snakeCase(studly)+"_seeder.go")
	writeScaffold(path, fmt.Sprintf(seederStub, studly, lower))
}

// writeScaffold writes content to path, creating the parent directory if
// needed and refusing to clobber an existing file, then prints a success
// line. Every make: command routes its file write through here.
func writeScaffold(path, content string) {
	if _, err := os.Stat(path); err == nil {
		fail(path + " already exists - refusing to overwrite")
	}
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fail("creating " + dir + ": " + err.Error())
		}
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		fail("writing " + path + ": " + err.Error())
	}
	fmt.Println(green + "created" + reset + " " + path)
}

// studlyCase turns "blog_post", "blog-post", or "blogPost" into
// "BlogPost", dropping any character that isn't a letter or digit.
func studlyCase(s string) string {
	var b strings.Builder
	upperNext := true
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if upperNext {
				b.WriteRune(unicode.ToUpper(r))
				upperNext = false
			} else {
				b.WriteRune(r)
			}
		default:
			upperNext = true
		}
	}
	return b.String()
}

// snakeCase turns "BlogPost" into "blog_post" for the file name.
func snakeCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// lowerCamel turns "BlogPost" into "blogPost" - StudlyCase with its first
// rune lowercased, used for a scaffolded resource's local variable name
// (e.g. "post" or "blogPost" holding a *models.Post).
func lowerCamel(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}
