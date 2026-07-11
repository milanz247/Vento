// Package vento is a lightweight, high-performance full-stack web
// framework built on Go's standard library, with GORM/MySQL as its only
// external integration.
//
// # Package layout
//
// Go requires every file that declares a method on Context, Engine, or
// RouterGroup to live in this same package and directory - a method can't
// be declared in a subpackage and still spell as c.Method() or app.Method().
// So unlike an application's own code (which is free to split into
// app/controllers, app/models, ...), this package stays a single flat
// directory of files for everything that's a method, organized by filename
// instead of by folder.
//
// Anything that genuinely doesn't need to be a method - it takes its
// subject as a plain argument rather than acting on *Context, *Engine, or
// *RouterGroup - has been factored out into its own subpackage instead
// (see "Subpackages" below). That's not a consolation prize: a subpackage
// is independently testable with no *Context, *http.Request, or database
// in scope, gets its own focused package doc, and is exactly as globally
// importable from application code as vento itself.
//
// Files are grouped here by concern, so this comment doubles as a table of
// contents - skim it once to know which file (or subpackage) to open for a
// given need.
//
// Bootstrap:
//   - engine.go  Engine: New, Use, route registration, ConnectDB, LoadHTMLGlob, Run, ServeHTTP
//   - kernel.go  DefaultMiddleware - the global middleware stack New() installs automatically
//
// Routing:
//   - router.go  the per-method Trie route matcher (unexported; reached only through Engine/RouterGroup)
//   - group.go   RouterGroup - app.Group(prefix, middleware...) for shared-prefix route sets
//
// The request/response core:
//   - context.go    Context itself: Reset, Set/Get, Next/Abort, JSON/String, Query/FormValue/Param, DB/SetDB/SetParams, View/HTML/Partial, Detach
//   - responses.go  c.OK/Created/NoContent/Status/Redirect and the error shorthands (BadRequest, NotFound, ...)
//   - params.go     c.ParamInt/ParamUint/QueryInt/QueryDefault/Header - typed request-input readers
//   - cookies.go    c.Cookie/SetCookie/ClearCookie, with hardened defaults baked in
//
// Binding and validation:
//   - bind.go      c.Bind/BindJSON/BindOrAbort - decode a request body into a struct
//   - validate.go  the `validate:"..."` struct-tag rule engine Bind runs automatically
//
// Database and queries:
//   - model.go       FindOrNotFound and Model[T] - the find-by-ID-or-404 patterns
//   - query.go       Query[T](c) and QueryHandle[T] - the typed CRUD/pagination handle
//   - pagination.go  the standalone Paginate GORM scope and DefaultPerPage/MaxPerPage
//
// Security (SecurityHeaders/BodyLimit/RateLimiter/CSRFProtection are all
// part of DefaultMiddleware and need no setup; CORS and Sessions are
// opt-in, wired via app.Use in main.go):
//   - security.go  SecurityHeaders, BodyLimit, RateLimiter, CSRFProtection
//   - proxy.go     TrustProxyHeaders and the isSecure/clientIP helpers those (and Sessions) use
//   - cors.go      CORS
//   - session.go   Session, Sessions middleware - signed-cookie session storage
//   - auth.go      Login/Logout/Authenticated/CurrentUser/RequireAuth - session-backed auth
//
// Serving files and logging:
//   - static.go       Static - serves a directory, directory-listing disabled
//   - middlewares.go  Logger, Recovery
//   - logging.go      Log (structured, non-blocking) that Logger/Recovery write through
//
// Cross-cutting:
//   - typed.go      Provide/Use - type-keyed, request-scoped storage (lightweight DI)
//   - background.go c.AfterResponse - safe best-effort background work
//
// # Subpackages
//
//   - vento/config   LoadEnv, BuildMySQLDSN, Env/EnvInt/EnvBool. Pure - takes
//     a file path or reads the process environment directly, no *Context
//     involved.
//   - vento/hash     Make/Check (bcrypt password hashing) and RandomString -
//     the one place crypto/rand is touched. Pure.
//   - vento/migrate  Migration, Run, RollbackLast, AutoMigrateModels - schema
//     management. Pure - takes a *gorm.DB directly; used by an
//     application's migrations package and the CLI, not by request
//     handlers.
//   - vento/support  pagination-bounds math, route-prefix math, and generic
//     Map/Filter - small pure helpers factored out of engine.go/group.go/
//     query.go rather than duplicated or left unexported.
//   - vento/vtest    unit-testing helpers (NewContext, DecodeJSON) for an
//     application's own controllers. Split out so importing vento for
//     production code never pulls in net/http/httptest.
//   - vento/cmd/vento  the `vento` CLI (project scaffolding, db:migrate, ...).
package vento
