package vento

import (
	"net/http"
	"strings"
)

// staticMount pairs a URL prefix with the pre-compiled handler chain that
// serves files beneath it.
type staticMount struct {
	prefix   string
	handlers []HandlerFunc
}

// Static serves the files under dir at urlPrefix, e.g.
// Static("/public", "./public") serves ./public/css/app.css at
// /public/css/app.css. Static mounts are checked before the route Trie in
// ServeHTTP, so pick a urlPrefix (like "/public") that doesn't collide with
// an application route.
//
// Like GET/POST/etc., Static compiles its handler chain (global middlewares
// + the file server) once, at registration time - so, exactly like routes,
// Static must be called after Use() to be covered by Logger, Recovery,
// SecurityHeaders, and any other global middleware.
func (e *Engine) Static(urlPrefix, dir string) {
	urlPrefix = "/" + strings.Trim(urlPrefix, "/")
	fileServer := http.StripPrefix(urlPrefix, http.FileServer(http.Dir(dir)))

	chain := make([]HandlerFunc, 0, len(e.middlewares)+1)
	chain = append(chain, e.middlewares...)
	chain = append(chain, func(c *Context) {
		fileServer.ServeHTTP(c.Writer, c.Request)
	})

	e.statics = append(e.statics, staticMount{prefix: urlPrefix + "/", handlers: chain})
}

// matchStatic returns the compiled handler chain for the first static mount
// whose prefix covers path, or nil if none matches.
func (e *Engine) matchStatic(path string) []HandlerFunc {
	for _, s := range e.statics {
		if strings.HasPrefix(path, s.prefix) {
			return s.handlers
		}
	}
	return nil
}
