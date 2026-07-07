package vento

import (
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// HandlerFunc is the signature every middleware and controller satisfies.
// Unlike net/http's ServeHTTP(w, r), it takes a single *Context, which is
// the core DX simplification Vento offers.
type HandlerFunc func(*Context)

// Engine is Vento's coordinator. It owns route registration and, by
// implementing http.Handler, plugs directly into Go's standard server
// with zero adapter code.
//
// Two zero-allocation patterns keep the hot path lean:
//   - Contexts are recycled through a sync.Pool instead of allocated per
//     request, removing that allocation from the GC's workload entirely.
//   - Handler chains (global middlewares + route middlewares + controller)
//     are compiled once at registration time and stored on the route's
//     Trie node, so serving a request never rebuilds a chain.
//
// The registration-time compilation means Use() must be called before the
// routes it should apply to - the idiomatic layout (routes/web.go calls
// Use first, then maps endpoints) does this naturally.
type Engine struct {
	router      *router
	middlewares []HandlerFunc // global middlewares, prefixed onto every route's chain
	pool        sync.Pool     // recycled *Context instances

	DB        *gorm.DB            // GORM connection pool, injected into every Context
	templates map[string]*viewSet // view name -> pre-stitched layout+page template set
	statics   []staticMount       // URL-prefix -> http.Handler mounts registered via Static
}

// New instantiates a ready-to-use Engine, pre-loaded with DefaultMiddleware
// so it is secure out of the box, and whose pool generates clean Context
// instances on demand (until recycling makes that unnecessary).
func New() *Engine {
	e := &Engine{
		router: newRouter(),
	}
	e.pool.New = func() any { return &Context{} }
	e.Use(DefaultMiddleware()...)
	return e
}

// Use registers global middlewares. They run, in order, ahead of every
// route registered after this call.
func (e *Engine) Use(middlewares ...HandlerFunc) {
	e.middlewares = append(e.middlewares, middlewares...)
}

// addRoute compiles the full handler chain for one endpoint - global
// middlewares, then route-specific middlewares, then the controller - and
// stores it on the route's terminal Trie node.
func (e *Engine) addRoute(method, path string, handler HandlerFunc, middlewares []HandlerFunc) {
	chain := make([]HandlerFunc, 0, len(e.middlewares)+len(middlewares)+1)
	chain = append(chain, e.middlewares...)
	chain = append(chain, middlewares...)
	chain = append(chain, handler)
	e.router.addRoute(method, path, chain)
}

// GET registers a handler for GET requests to path, optionally guarded by
// route-specific middlewares.
func (e *Engine) GET(path string, handler HandlerFunc, middlewares ...HandlerFunc) {
	e.addRoute(http.MethodGet, path, handler, middlewares)
}

// POST registers a handler for POST requests to path, optionally guarded
// by route-specific middlewares.
func (e *Engine) POST(path string, handler HandlerFunc, middlewares ...HandlerFunc) {
	e.addRoute(http.MethodPost, path, handler, middlewares)
}

// PUT registers a handler for PUT requests to path, optionally guarded by
// route-specific middlewares.
func (e *Engine) PUT(path string, handler HandlerFunc, middlewares ...HandlerFunc) {
	e.addRoute(http.MethodPut, path, handler, middlewares)
}

// DELETE registers a handler for DELETE requests to path, optionally
// guarded by route-specific middlewares.
func (e *Engine) DELETE(path string, handler HandlerFunc, middlewares ...HandlerFunc) {
	e.addRoute(http.MethodDelete, path, handler, middlewares)
}

// ConnectDB opens a GORM connection pool against MySQL using dsn and
// stores it on Engine.DB for every Context to reach via c.DB(). MySQL is
// Vento's sole database provider; a failure here should abort startup.
func (e *Engine) ConnectDB(dsn string) error {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return err
	}
	e.DB = db
	return nil
}

// LoadHTMLGlob walks the directory tree rooted at the portion of pattern
// before its first wildcard and pre-stitches every page template into the
// shared layout at startup. Files under a "layouts" directory form the
// layout set; every other .html file is a page. Each page is compiled as
// its own clone of the layout set, so two pages may both {{define
// "content"}} without colliding, and c.View("index", data) renders
// views/index.html inside views/layouts/base.html with zero boilerplate
// in the controller. Pages are keyed by their path relative to the root,
// without the .html extension (e.g. "index", "users/show").
func (e *Engine) LoadHTMLGlob(pattern string) {
	root, _, _ := strings.Cut(pattern, "*")
	root = strings.TrimSuffix(root, "/")
	if root == "" {
		root = "."
	}

	var layouts, pages []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if strings.HasPrefix(rel, "layouts"+string(filepath.Separator)) {
			layouts = append(layouts, path)
		} else {
			pages = append(pages, path)
		}
		return nil
	})
	if err != nil {
		panic(fmt.Sprintf("vento: LoadHTMLGlob(%q): %v", pattern, err))
	}

	var layoutSet *template.Template
	layoutEntry := ""
	if len(layouts) > 0 {
		layoutSet, err = template.ParseFiles(layouts...)
		if err != nil {
			panic(fmt.Sprintf("vento: LoadHTMLGlob(%q): %v", pattern, err))
		}
		// Prefer base.html as the document entry point; otherwise use the
		// first layout parsed.
		layoutEntry = filepath.Base(layouts[0])
		for _, l := range layouts {
			if filepath.Base(l) == "base.html" {
				layoutEntry = "base.html"
				break
			}
		}
	}

	e.templates = make(map[string]*viewSet, len(pages))
	for _, page := range pages {
		var set *template.Template
		entry := layoutEntry
		if layoutSet != nil {
			set, err = layoutSet.Clone()
			if err == nil {
				set, err = set.ParseFiles(page)
			}
		} else {
			set, err = template.ParseFiles(page)
			entry = filepath.Base(page)
		}
		if err != nil {
			panic(fmt.Sprintf("vento: LoadHTMLGlob(%q): parsing %s: %v", pattern, page, err))
		}

		rel, _ := filepath.Rel(root, page)
		name := strings.TrimSuffix(filepath.ToSlash(rel), ".html")
		e.templates[name] = &viewSet{tmpl: set, entry: entry}
	}
}

// ServeHTTP satisfies http.Handler, which is what allows an *Engine to be
// passed straight to http.ListenAndServe (or wrapped by httptest.Server,
// or mounted under another mux) with no glue code.
//
// Static mounts (registered via Static) are checked before the route Trie;
// both paths converge on dispatch, so static requests get the same pooled
// Context and pre-compiled global-middleware coverage (Logger, Recovery,
// SecurityHeaders, ...) that routed requests do.
func (e *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if handlers := e.matchStatic(r.URL.Path); handlers != nil {
		e.dispatch(w, r, handlers, nil)
		return
	}

	if matched, params := e.router.getRoute(r.Method, r.URL.Path); matched != nil {
		e.dispatch(w, r, matched.handlers, params)
		return
	}

	// A CORS preflight targets a route that (almost always) is never
	// registered under OPTIONS itself, so it would otherwise 404 before
	// the global middleware chain - and CORS - ever runs. Run the global
	// chain for it specifically, falling through to the normal 404 if
	// nothing in the chain (i.e. CORS, for a disallowed origin) handles it.
	if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
		chain := make([]HandlerFunc, 0, len(e.middlewares)+1)
		chain = append(chain, e.middlewares...)
		chain = append(chain, func(c *Context) { http.NotFound(c.Writer, c.Request) })
		e.dispatch(w, r, chain, nil)
		return
	}

	http.NotFound(w, r)
}

// dispatch acquires a Context from the pool, points it at handlers (a
// pre-compiled chain - see addRoute and Static), runs it via Next(), and
// returns the Context to the pool. If a panic escapes the whole chain (i.e.
// Recovery is not installed), the Context is deliberately not repooled and
// is left to the GC.
func (e *Engine) dispatch(w http.ResponseWriter, r *http.Request, handlers []HandlerFunc, params map[string]string) {
	ctx := e.pool.Get().(*Context)
	ctx.Reset(w, r)
	ctx.params = params
	ctx.handlers = handlers
	ctx.db = e.DB
	ctx.templates = e.templates

	ctx.Next()

	e.pool.Put(ctx)
}

// Run starts the HTTP server on addr (e.g. ":8080"), using the Engine
// itself as the root http.Handler, and blocks until it stops - either
// because ListenAndServe failed (e.g. the port is already in use) or
// because the process received SIGINT/SIGTERM, in which case Run drains
// in-flight requests via graceful shutdown before returning nil. A second
// signal, or requests still open after ShutdownTimeout, does not force an
// immediate exit - callers wanting that should also install their own
// os/signal handling if it matters for their deployment.
//
// The server is configured with conservative timeouts rather than the
// standard library's unlimited defaults, so a client that connects and
// then stalls (slow-loris style) cannot pin a goroutine and its
// connection forever. The values are deliberately generous for normal
// traffic; an application needing custom ones (e.g. long-polling) can
// build its own http.Server and pass the Engine as Handler, since Engine
// implements http.Handler directly.
func (e *Engine) Run(addr string) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           e,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		fmt.Printf("vento: listening on %s\n", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serveErr <- err
			return
		}
		close(serveErr)
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)

	select {
	case err := <-serveErr:
		return err
	case <-stop:
		fmt.Println("vento: shutting down (waiting up to " + ShutdownTimeout.String() + " for in-flight requests)...")
		ctx, cancel := context.WithTimeout(context.Background(), ShutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			return err
		}
		fmt.Println("vento: shutdown complete")
		return nil
	}
}

// ShutdownTimeout is how long Run waits for in-flight requests to finish
// draining after SIGINT/SIGTERM before giving up. It's a package-level var
// rather than a Run parameter so it stays out of the common call
// (app.Run(":8080")) - override it before calling Run if 10s doesn't suit
// a deployment (e.g. long-running uploads).
var ShutdownTimeout = 10 * time.Second
