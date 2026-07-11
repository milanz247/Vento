package vento

import (
	"net/http"

	"vento-app/vento/support"
)

// RouterGroup is a set of routes that share a common path prefix and a
// common stack of middleware, so neither has to be repeated on every route
// in the set. It's the natural way to express "everything under /api", "the
// admin section behind an auth guard", or an API version:
//
//	api := app.Group("/api")
//	api.GET("/users", controllers.UserIndex)     // -> GET /api/users
//	api.POST("/users", controllers.UserCreate)   // -> POST /api/users
//
//	admin := app.Group("/admin", middleware.RequireAuth)
//	admin.GET("/dashboard", controllers.Dashboard) // auth runs first
//
// A group's middleware runs after the engine's global middleware (installed
// via Use) and before any route-specific middleware, in the same
// registration-time-compiled chain as everything else - a group adds no
// per-request cost over a plain route. Groups nest, accumulating prefix and
// middleware from each level.
//
// The prefix/path arithmetic (NormalizePrefix, JoinPath) and the
// no-aliasing slice-append (JoinChains) live in vento/support: they're pure
// string/slice logic with no dependency on Context or *Engine, so they're
// factored out where they can be tested and reused in isolation. What has
// to stay here is the Group/GET/POST/... surface itself - Go requires a
// method to be declared in the same package as its receiver type, so
// RouterGroup's methods can't move out of package vento.
type RouterGroup struct {
	engine      *Engine
	prefix      string
	middlewares []HandlerFunc
}

// Group returns a RouterGroup rooted at prefix, with middlewares applied to
// every route registered on it. prefix is normalized to a single leading
// slash and no trailing slash, so "api", "/api", and "/api/" are
// equivalent.
func (e *Engine) Group(prefix string, middlewares ...HandlerFunc) *RouterGroup {
	return &RouterGroup{
		engine:      e,
		prefix:      support.NormalizePrefix(prefix),
		middlewares: middlewares,
	}
}

// Group returns a nested RouterGroup, combining this group's prefix and
// middleware with the child's - so app.Group("/api").Group("/v1", auth)
// serves routes under "/api/v1" with auth applied.
func (g *RouterGroup) Group(prefix string, middlewares ...HandlerFunc) *RouterGroup {
	return &RouterGroup{
		engine:      g.engine,
		prefix:      g.prefix + support.NormalizePrefix(prefix),
		middlewares: support.JoinChains(g.middlewares, middlewares),
	}
}

// GET registers a handler for GET requests to prefix+path on this group.
func (g *RouterGroup) GET(path string, handler HandlerFunc, middlewares ...HandlerFunc) {
	g.handle(http.MethodGet, path, handler, middlewares)
}

// POST registers a handler for POST requests to prefix+path on this group.
func (g *RouterGroup) POST(path string, handler HandlerFunc, middlewares ...HandlerFunc) {
	g.handle(http.MethodPost, path, handler, middlewares)
}

// PUT registers a handler for PUT requests to prefix+path on this group.
func (g *RouterGroup) PUT(path string, handler HandlerFunc, middlewares ...HandlerFunc) {
	g.handle(http.MethodPut, path, handler, middlewares)
}

// PATCH registers a handler for PATCH requests to prefix+path on this group.
func (g *RouterGroup) PATCH(path string, handler HandlerFunc, middlewares ...HandlerFunc) {
	g.handle(http.MethodPatch, path, handler, middlewares)
}

// DELETE registers a handler for DELETE requests to prefix+path on this
// group.
func (g *RouterGroup) DELETE(path string, handler HandlerFunc, middlewares ...HandlerFunc) {
	g.handle(http.MethodDelete, path, handler, middlewares)
}

// handle prepends the group's middleware to the route's own and registers
// the full path (group prefix + route path) on the engine - which then
// compiles the global + group + route + handler chain exactly as it does
// for a top-level route.
func (g *RouterGroup) handle(method, path string, handler HandlerFunc, routeMiddlewares []HandlerFunc) {
	g.engine.addRoute(method, support.JoinPath(g.prefix, path), handler, support.JoinChains(g.middlewares, routeMiddlewares))
}
