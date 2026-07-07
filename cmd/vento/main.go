// Command vento is the framework's Artisan-style developer tool. Run it
// from the project root (it reads ./.env and writes under the app packages):
//
//	vento run                       start the app (hot-reload via air when available)
//	vento db:migrate                apply every pending migration (offers to create
//	                               the database first if it doesn't exist yet)
//	vento db:rollback               revert the most recently applied migration
//	vento db:automigrate            GORM AutoMigrate every model in models.All() (dev)
//	vento db:seed                   run all registered seeders (idempotent)
//	vento make:controller Name      scaffold controllers/name_controller.go
//	vento make:model Name           scaffold models/name.go
//	vento make:middleware Name      scaffold middleware/name.go
//	vento make:migration name       scaffold migrations/<timestamp>_name.go
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

	"vento-app/migrations"
	"vento-app/models"
	"vento-app/vento"

	_ "github.com/go-sql-driver/mysql"
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

func main() {
	fmt.Print(banner)

	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "run", "serve":
		runApp()
	case "db:migrate":
		runMigrations(openDB(true))
	case "db:rollback":
		rollback(openDB(false))
	case "db:automigrate":
		autoMigrate(openDB(true))
	case "db:seed":
		seed(openDB(false))
	case "make:controller":
		requireName("make:controller", "Post")
		makeController(os.Args[2])
	case "make:model":
		requireName("make:model", "Post")
		makeModel(os.Args[2])
	case "make:middleware":
		requireName("make:middleware", "RequireAuth")
		makeMiddleware(os.Args[2])
	case "make:migration":
		requireName("make:migration", "create_posts_table")
		makeMigration(os.Args[2])
	default:
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

  ` + green + `run | serve` + reset + `             start the app (hot-reload via air when installed)
  ` + green + `db:migrate` + reset + `              apply every pending migration
  ` + green + `db:rollback` + reset + `             revert the most recently applied migration
  ` + green + `db:automigrate` + reset + `          GORM AutoMigrate every model in models.All() (dev shortcut)
  ` + green + `db:seed` + reset + `                 run all registered seeders (safe to re-run)
  ` + green + `make:controller <Name>` + reset + `  scaffold controllers/<name>_controller.go
  ` + green + `make:model <Name>` + reset + `       scaffold models/<name>.go
  ` + green + `make:middleware <Name>` + reset + `  scaffold middleware/<name>.go
  ` + green + `make:migration <name>` + reset + `   scaffold migrations/<timestamp>_<name>.go`)
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, red+"error:"+reset+" "+msg)
	os.Exit(1)
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

// openDB connects to MySQL using the DB_* keys in .env. MySQL is the
// exclusive database provider: if it is unconfigured or unreachable the
// command aborts rather than silently working against a different store.
// When createIfMissing is true (db:migrate), the target database is
// created first - interactively, with the user's confirmation - if it
// doesn't already exist, so a fresh checkout can migrate straight away
// instead of failing with a cryptic "unknown database" error.
func openDB(createIfMissing bool) *gorm.DB {
	env := vento.LoadEnv(".env")
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

	dsn, ok := vento.BuildMySQLDSN(env)
	if !ok {
		fail("DB_HOST/DB_USER/DB_NAME missing from .env - MySQL configuration is required")
	}

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

	fmt.Printf(yellow+"database %q does not exist."+reset+" Create it? [y/N]: ", name)
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.ToLower(strings.TrimSpace(answer))
	if answer != "y" && answer != "yes" {
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
	applied, err := vento.RunMigrations(db, migrations.All())
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
	id, err := vento.RollbackLastMigration(db, migrations.All())
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

// autoMigrate runs GORM AutoMigrate over every model in models.All() - an
// untracked, additive schema sync handy for rapid prototyping. Prefer
// versioned migrations (db:migrate) once the schema needs ordered,
// reversible history.
func autoMigrate(db *gorm.DB) {
	all := models.All()
	if err := vento.AutoMigrateModels(db, all); err != nil {
		fail(err.Error())
	}
	for _, model := range all {
		fmt.Printf(green+"auto-migrated"+reset+" %T\n", model)
	}
	fmt.Println(bold + "db:automigrate complete" + reset)
}

// seeder is one named, idempotent database seeding step.
type seeder struct {
	name string
	run  func(db *gorm.DB) error
}

// seeders is the registered seeding list. Each seeder must be safe to
// run repeatedly: use FirstOrCreate (or equivalent) so re-seeding never
// inserts duplicates.
var seeders = []seeder{
	{name: "users", run: seedUsers},
}

func seed(db *gorm.DB) {
	for _, s := range seeders {
		if err := s.run(db); err != nil {
			fail(fmt.Sprintf("seeder %q failed: %v", s.name, err))
		}
		fmt.Printf(green+"seeded"+reset+" %s\n", s.name)
	}
	fmt.Println(bold + "db:seed complete" + reset)
}

// seedUsers inserts five test users, keyed on email so re-running the
// seeder is a no-op for rows that already exist.
func seedUsers(db *gorm.DB) error {
	testUsers := []models.User{
		{Name: "Ada Lovelace", Email: "ada@example.com"},
		{Name: "Grace Hopper", Email: "grace@example.com"},
		{Name: "Alan Turing", Email: "alan@example.com"},
		{Name: "Edsger Dijkstra", Email: "edsger@example.com"},
		{Name: "Barbara Liskov", Email: "barbara@example.com"},
	}
	for i := range testUsers {
		err := db.Where(models.User{Email: testUsers[i].Email}).
			FirstOrCreate(&testUsers[i]).Error
		if err != nil {
			return err
		}
	}
	return nil
}

// controllerStub is the boilerplate written by make:controller. %[1]s is
// the StudlyCase name (e.g. Post), %[2]s its lowercase form.
const controllerStub = `package controllers

import (
	"net/http"

	"vento-app/vento"
)

// %[1]sIndex handles GET /%[2]ss: list all %[2]ss.
func %[1]sIndex(c *vento.Context) {
	c.JSON(http.StatusOK, map[string]string{"message": "%[1]sIndex: not implemented"})
}

// %[1]sShow handles GET /%[2]ss/:id: show one %[2]s.
func %[1]sShow(c *vento.Context) {
	c.JSON(http.StatusOK, map[string]string{
		"id":      c.Param("id"),
		"message": "%[1]sShow: not implemented",
	})
}

// %[1]sStore handles POST /%[2]ss: create a %[2]s.
func %[1]sStore(c *vento.Context) {
	c.JSON(http.StatusCreated, map[string]string{"message": "%[1]sStore: not implemented"})
}
`

// makeController scaffolds controllers/<snake>_controller.go containing
// stubbed Index/Show/Store handlers named after the StudlyCase name.
func makeController(rawName string) {
	studly := studlyCase(rawName)
	if studly == "" {
		fail(fmt.Sprintf("%q is not a usable controller name (letters/digits only)", rawName))
	}
	path := filepath.Join("controllers", snakeCase(studly)+"_controller.go")
	writeScaffold(path, fmt.Sprintf(controllerStub, studly, strings.ToLower(studly)))
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
	path := filepath.Join("models", snakeCase(studly)+".go")
	writeScaffold(path, fmt.Sprintf(modelStub, studly))
	fmt.Println(yellow + "next:" + reset + " add &" + studly + "{} to models.All() in models/user.go")
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
	path := filepath.Join("middleware", snakeCase(studly)+".go")
	writeScaffold(path, fmt.Sprintf(middlewareStub, studly))
	fmt.Println(yellow + "next:" + reset + " add middleware." + studly + " to GlobalMiddleware() in routes/kernel.go, or attach it to a route in routes/web.go")
}

// migrationStub is written by make:migration. %[1]s is the full migration
// ID (timestamp prefix + snake_case name).
const migrationStub = `package migrations

import (
	"gorm.io/gorm"

	"vento-app/vento"
)

// %[1]s
func init() {
	register(vento.Migration{
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
