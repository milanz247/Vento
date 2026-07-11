package vento

import (
	"fmt"
	"strconv"
)

// This file adds typed readers for the two request inputs handlers parse
// most - route parameters (nearly always numeric IDs) and query strings
// (nearly always optional, with a default) - so controllers stop importing
// strconv and hand-writing the same parse-and-fallback dance:
//
//	id, err := c.ParamInt("id")           // vs strconv.Atoi(c.Param("id"))
//	if err != nil {
//	    c.BadRequest("invalid id")
//	    return
//	}
//	page := c.QueryInt("page", 1)         // ?page=2, defaulting to 1

// ParamInt returns a route parameter parsed as an int, e.g. c.ParamInt("id")
// for a route "/users/:id". The error is descriptive (naming the parameter)
// so it can be surfaced straight to the client via c.BadRequest.
func (c *Context) ParamInt(key string) (int, error) {
	raw := c.Param(key)
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("vento: route parameter %q is not an integer: %q", key, raw)
	}
	return n, nil
}

// ParamUint returns a route parameter parsed as an unsigned int - the right
// choice for database IDs, which are never negative. It errors on a
// negative or non-numeric value.
func (c *Context) ParamUint(key string) (uint, error) {
	raw := c.Param(key)
	n, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("vento: route parameter %q is not a non-negative integer: %q", key, raw)
	}
	return uint(n), nil
}

// Query returns a URL query parameter (defined in context.go). QueryDefault
// returns def instead when the parameter is absent or empty - so an
// optional filter can collapse to a sensible fallback in one line.
func (c *Context) QueryDefault(key, def string) string {
	if v := c.Query(key); v != "" {
		return v
	}
	return def
}

// QueryInt returns a query parameter parsed as an int, falling back to def
// when it's absent, empty, or not a valid integer. Query parameters are
// optional by nature (pagination, limits, filters), so - unlike ParamInt -
// this never errors: a garbage ?page=abc simply yields the default.
func (c *Context) QueryInt(key string, def int) int {
	if n, err := strconv.Atoi(c.Query(key)); err == nil {
		return n
	}
	return def
}

// Header returns the value of a request header, e.g.
// c.Header("Authorization") - a shorthand for c.Request.Header.Get, handy
// in auth and content-negotiation middleware.
func (c *Context) Header(key string) string {
	return c.Request.Header.Get(key)
}
