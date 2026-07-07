// Package controllers holds request handlers, mirroring Laravel's
// app/Http/Controllers. Keeping them out of main.go and out of routes is
// what avoids a routes <-> main circular import: routes only needs to
// import controllers and vento, never main.
//
// Add a handler here (or in a new file in this package), then wire it to a
// URL in routes/web.go. A handler is any func(c *vento.Context).
package controllers

import "vento-app/vento"

// Index renders the welcome page: c.View stitches views/index.html into
// views/layouts/base.html automatically. vento.H is a map[string]any
// shorthand for view data; a struct, or c.Set + c.View("index") with no
// data argument, work just as well - see vento.Context.View.
func Index(c *vento.Context) {
	c.View("index", vento.H{
		"Message": "A lightweight, high-performance Go web framework built on the standard library.",
	})
}
