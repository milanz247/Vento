package vento

import (
	"io/fs"
	"net/http"
	"strconv"
	"strings"
)

// staticMount pairs a URL prefix with the pre-compiled handler chain that
// serves files beneath it.
type staticMount struct {
	prefix   string
	handlers []HandlerFunc
}

// StaticMaxAge is the Cache-Control max-age (in seconds) Static stamps onto
// every response, so browsers stop re-fetching unchanged assets on every
// load. http.FileServer already handles conditional requests on top of
// this (Last-Modified / If-Modified-Since), so a stale cache still
// revalidates cheaply instead of silently serving old content forever.
// Lower it, per mount, if assets change more often than once an hour by
// setting it before calling Static - it's read at registration time, not
// per request.
var StaticMaxAge = 3600

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
	fileServer := http.StripPrefix(urlPrefix, http.FileServer(noDirListing{http.Dir(dir)}))
	cacheControl := "public, max-age=" + strconv.Itoa(StaticMaxAge)

	chain := make([]HandlerFunc, 0, len(e.middlewares)+1)
	chain = append(chain, e.middlewares...)
	chain = append(chain, func(c *Context) {
		c.Writer.Header().Set("Cache-Control", cacheControl)
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

// noDirListing wraps an http.FileSystem so a request for a directory with
// no index.html 404s instead of falling back to http.FileServer's default
// behavior of rendering an HTML listing of the directory's contents - which
// would otherwise leak the on-disk file tree (backups, dotfiles, anything
// accidentally placed under the mount) to any client that asks.
type noDirListing struct {
	fs http.FileSystem
}

func (n noDirListing) Open(name string) (http.File, error) {
	f, err := n.fs.Open(name)
	if err != nil {
		return nil, err
	}
	if stat, statErr := f.Stat(); statErr == nil && stat.IsDir() {
		index := strings.TrimSuffix(name, "/") + "/index.html"
		idx, err := n.fs.Open(index)
		if err != nil {
			f.Close()
			return nil, fs.ErrNotExist
		}
		idx.Close()
	}
	return f, nil
}
