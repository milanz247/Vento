package vento

import (
	"errors"
	"net/http"
)

// This file adds the response shorthands a handler reaches for on almost
// every request, so day-to-day controller code never has to import
// net/http just to name a status code. Each one is a thin, named wrapper
// over the primitives in context.go (JSON, String, Abort) - they add no
// new behavior, only a clearer, shorter spelling of the common cases.
//
//	func Show(c *vento.Context) {
//	    user, err := findUser(c)
//	    if err != nil {
//	        c.NotFound("user not found") // vs c.Abort(http.StatusNotFound, ...)
//	        return
//	    }
//	    c.OK(user)                       // vs c.JSON(http.StatusOK, user)
//	}

// OK writes data as JSON with 200 OK - the default success response for a
// handler that returns a resource.
func (c *Context) OK(data any) { c.JSON(http.StatusOK, data) }

// Created writes data as JSON with 201 Created - the conventional response
// after a POST that creates a new resource.
func (c *Context) Created(data any) { c.JSON(http.StatusCreated, data) }

// NoContent writes 204 No Content with an empty body - e.g. after a DELETE,
// or a PUT/PATCH that returns nothing.
func (c *Context) NoContent() { c.Status(http.StatusNoContent) }

// Status writes an empty response with just the given status code. Use it
// when the status line alone is the response; use JSON/String/View when
// there's a body.
func (c *Context) Status(code int) {
	c.StatusCode = code
	c.Writer.WriteHeader(code)
}

// Redirect sends a 302 Found redirect to url, the general-purpose "go here
// instead" response - e.g. after a successful login or form submission.
func (c *Context) Redirect(url string) {
	c.StatusCode = http.StatusFound
	http.Redirect(c.Writer, c.Request, url, http.StatusFound)
}

// The error shorthands below all defer to Abort, so - exactly like Abort -
// they stop the handler chain immediately (no later middleware or
// controller runs) and write {"error": msg} as JSON with the named status.
// That makes them equally at home ending a controller or rejecting a
// request from inside a guard middleware:
//
//	func RequireAuth(c *vento.Context) {
//	    if c.Header("Authorization") == "" {
//	        c.Unauthorized("missing token")
//	        return
//	    }
//	    c.Next()
//	}

// BadRequest aborts with 400 - the client sent something malformed or
// invalid (a bad ID, a failed Bind, an out-of-range parameter).
func (c *Context) BadRequest(msg string) { c.Abort(http.StatusBadRequest, msg) }

// Unauthorized aborts with 401 - the request lacks valid authentication.
func (c *Context) Unauthorized(msg string) { c.Abort(http.StatusUnauthorized, msg) }

// Forbidden aborts with 403 - the caller is authenticated but not allowed
// to do this.
func (c *Context) Forbidden(msg string) { c.Abort(http.StatusForbidden, msg) }

// NotFound aborts with 404 - the requested resource does not exist.
func (c *Context) NotFound(msg string) { c.Abort(http.StatusNotFound, msg) }

// InternalError aborts with 500 - an unexpected server-side failure. Prefer
// letting a genuine panic reach Recovery; use this for errors you handle
// explicitly but still want reported as a 500.
func (c *Context) InternalError(msg string) { c.Abort(http.StatusInternalServerError, msg) }

// AbortWithError stops the handler chain and writes err as a JSON error
// response with the given status code - the general-purpose version of
// Abort for when you already have an error value instead of a message
// string.
//
// A ValidationErrors (as returned by Bind/Validate - see BindOrAbort, which
// calls this automatically) is rendered as {"errors": ["...", "..."]} -
// one entry per failed rule. Those messages are safe to expose as-is (they
// name only the field and rule, e.g. "Email is required" - see
// vento/validate.go), so a client can display every problem at once
// instead of just the first.
//
// Any other error is logged server-side with its full detail via Log, and
// rendered to the client as a generic {"error": "..."} - the error's own
// text (err.Error()) is deliberately not sent to the client: for a decode
// failure (see BindJSON), that text embeds Go-internal detail (struct
// field names, package-qualified wrapping) that's not a client's business
// and shouldn't be relied on by one either.
func (c *Context) AbortWithError(statusCode int, err error) {
	var verrs ValidationErrors
	if errors.As(err, &verrs) {
		c.index = len(c.handlers)
		c.JSON(statusCode, map[string]any{"errors": []string(verrs)})
		return
	}
	Log.Error("request rejected", "status", statusCode, "error", err.Error())
	c.Abort(statusCode, "the request could not be processed")
}
