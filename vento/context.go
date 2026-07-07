// Package vento is a lightweight, high-performance full-stack web
// framework built on Go's standard library, with GORM/MySQL as its only
// external integration.
package vento

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strings"

	"gorm.io/gorm"
)

// H is a shorthand for the map type view data is normally passed in, e.g.
// c.View("index", vento.H{"Message": "hi"}). It's just a named
// map[string]any, so it drops straight into ExecuteTemplate the same as a
// plain map literal would - it exists purely to shave the "map[string]any"
// boilerplate off every controller.
type H map[string]any

// Context wraps the standard http.ResponseWriter and *http.Request pair
// passed into every handler, so application code never touches net/http
// directly for common operations - the core of Vento's DX.
//
// Contexts are recycled through the Engine's sync.Pool: a request never
// allocates a fresh Context once the pool is warm. The corollary is that
// handlers must never retain a *Context (or anything reached through it)
// past the end of the request; copy out any values needed later.
type Context struct {
	Writer     http.ResponseWriter
	Request    *http.Request
	StatusCode int

	params   map[string]string
	handlers []HandlerFunc // compiled chain: global + route middlewares + controller
	index    int           // index of the currently executing handler; starts at -1

	db        *gorm.DB            // injected by the Engine before the chain runs
	templates map[string]*viewSet // pre-stitched layout+page sets, injected by the Engine
	viewData  H                   // values accumulated via Set, rendered by View when called with no data
}

// Reset re-initialises a pooled Context for a new request/response cycle,
// clearing every piece of per-request state so nothing can leak from the
// previous request that used this instance. The Engine calls it right
// after taking the Context out of the pool.
func (c *Context) Reset(w http.ResponseWriter, r *http.Request) {
	c.Writer = w
	c.Request = r
	c.StatusCode = http.StatusOK
	c.params = nil
	c.handlers = nil
	c.index = -1
	c.db = nil
	c.templates = nil
	c.viewData = nil
}

// Set stashes a key/value pair on the request, readable later via Get or,
// with no arguments needed, rendered straight into the view by View/HTML -
// c.View(name) with no data argument sends whatever has been Set so far.
// This lets middleware and the controller build up view data incrementally
// instead of assembling one map literal by hand. Values set earlier in the
// handler chain are visible to everything downstream, including the final
// controller.
func (c *Context) Set(key string, value any) {
	if c.viewData == nil {
		c.viewData = make(H)
	}
	c.viewData[key] = value
}

// Get returns a value previously stored with Set, and whether it was found.
func (c *Context) Get(key string) (any, bool) {
	v, ok := c.viewData[key]
	return v, ok
}

// Next advances to and runs the next handler in the chain. Middlewares
// call it to yield control downstream; any code placed after the Next()
// call runs once the rest of the chain has returned, which is what makes
// "before/after" middleware logic like latency logging possible.
func (c *Context) Next() {
	c.index++
	for c.index < len(c.handlers) {
		c.handlers[c.index](c)
		c.index++
	}
}

// Abort stops the handler chain immediately - no subsequent middleware or
// controller will run - and writes statusCode/msg as a structured JSON
// error response.
func (c *Context) Abort(statusCode int, msg string) {
	c.index = len(c.handlers)
	c.JSON(statusCode, map[string]string{"error": msg})
}

// JSON marshals data to JSON, sets the Content-Type header, writes the
// status code, and streams the encoded body to the client.
func (c *Context) JSON(statusCode int, data any) {
	c.StatusCode = statusCode
	c.Writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	c.Writer.WriteHeader(statusCode)

	if err := json.NewEncoder(c.Writer).Encode(data); err != nil {
		// Headers are already sent; a plain-text trailer is the best we can do.
		http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
	}
}

// String writes a plain-text response with the given status code.
func (c *Context) String(statusCode int, text string) {
	c.StatusCode = statusCode
	c.Writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.Writer.WriteHeader(statusCode)
	c.Writer.Write([]byte(text))
}

// Query returns the value of a URL query parameter, e.g. /user?name=Milan.
func (c *Context) Query(key string) string {
	return c.Request.URL.Query().Get(key)
}

// FormValue returns the value of a POST/PUT form field, transparently
// parsing urlencoded and multipart bodies on first access.
func (c *Context) FormValue(key string) string {
	return c.Request.FormValue(key)
}

// Param returns the value captured for a dynamic route segment: for a
// route "/users/:id" and a request to "/users/42", c.Param("id") returns
// "42". It returns "" if key was not captured.
func (c *Context) Param(key string) string {
	return c.params[key]
}

// DB returns the Engine's GORM connection pool, letting handlers write
// queries directly, e.g. c.DB().Find(&users).
func (c *Context) DB() *gorm.DB {
	return c.db
}

// View renders the named page (e.g. "index" for views/index.html) with
// status 200, automatically stitched into the shared layout
// (views/layouts/base.html). The layout/page composition was pre-compiled
// at LoadHTMLGlob time, so controllers carry zero template boilerplate.
//
// data is optional and accepts anything a template can range/index/print
// over - a vento.H{...} or map[string]any literal, a struct, a slice, or
// nothing at all:
//
//	c.View("index", vento.H{"Message": "hi"})  // map shorthand
//	c.View("index", user)                      // any struct or value
//	c.View("index")                            // renders values set via c.Set
func (c *Context) View(name string, data ...any) {
	c.HTML(http.StatusOK, name, data...)
}

// HTML renders the named page from the pre-stitched view cache with an
// explicit status code, setting Content-Type: text/html. name may be
// given with or without the .html extension. data is optional; see View
// for the accepted shapes.
func (c *Context) HTML(statusCode int, name string, data ...any) {
	view := c.templates[strings.TrimSuffix(name, ".html")]
	if view == nil {
		http.Error(c.Writer, "vento: unknown view "+name+" (is LoadHTMLGlob configured?)", http.StatusInternalServerError)
		return
	}

	c.StatusCode = statusCode
	c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	c.Writer.WriteHeader(statusCode)

	if err := view.tmpl.ExecuteTemplate(c.Writer, view.entry, c.viewPayload(data...)); err != nil {
		http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
	}
}

// viewPayload resolves what View/HTML/Partial hand to the template: the
// explicit data argument when one was passed, otherwise whatever has been
// accumulated via Set (or nil if neither was used).
func (c *Context) viewPayload(data ...any) any {
	if len(data) > 0 {
		return data[0]
	}
	if c.viewData == nil {
		return nil
	}
	return c.viewData
}

// IsHTMX reports whether the current request was issued by HTMX rather than
// a normal browser navigation - i.e. it carries the "HX-Request: true"
// header, which htmx.org sets automatically on every request it makes.
// Handlers use this to branch between a full-page render (c.View) and a
// partial swap (c.Partial).
func (c *Context) IsHTMX() bool {
	return c.Request.Header.Get("HX-Request") == "true"
}

// Partial renders the named view's "content" block only, skipping the
// shared layout (views/layouts/base.html) entirely. Where c.View/c.HTML
// produce a full HTML document, c.Partial produces just the DOM fragment a
// tool like HTMX needs to swap into an existing page via
// hx-target/hx-swap - the building block for Livewire/Hotwire-style
// partial-page updates. name may be given with or without the .html
// extension, and is looked up in the same pre-stitched view cache as
// View/HTML (so any file under views/, not just ones under a "partials/"
// directory, can be rendered this way). data is optional; see View for the
// accepted shapes.
func (c *Context) Partial(name string, data ...any) {
	view := c.templates[strings.TrimSuffix(name, ".html")]
	if view == nil {
		http.Error(c.Writer, "vento: unknown view "+name+" (is LoadHTMLGlob configured?)", http.StatusInternalServerError)
		return
	}

	c.StatusCode = http.StatusOK
	c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	c.Writer.WriteHeader(http.StatusOK)

	if err := view.tmpl.ExecuteTemplate(c.Writer, "content", c.viewPayload(data...)); err != nil {
		http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
	}
}

// viewSet is one renderable page: the layout templates plus that page's
// own template, pre-stitched at LoadHTMLGlob time. entry is the template
// name to execute (the layout's basename, e.g. "base.html", or the page's
// own basename when no layout exists).
type viewSet struct {
	tmpl  *template.Template
	entry string
}
