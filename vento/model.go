package vento

import (
	"errors"

	"gorm.io/gorm"
)

// FindOrNotFound looks up a record by primary key into dest and handles
// the two ways that can fail: a 404 response if no such record exists, or
// a 500 if the lookup itself errors (a dropped connection, a malformed
// query) - the find-or-fail pattern nearly every Show/Update/Delete
// handler needs, without hand-rolling the
// errors.Is(err, gorm.ErrRecordNotFound) check on every one:
//
//	var user models.User
//	if !vento.FindOrNotFound(c, &user, id) {
//	    return
//	}
//	// user is loaded past this point
//
// The lookup uses an explicit "id = ?" condition rather than GORM's
// First(dest, id) inline-condition shortcut - deliberately: when id is a
// string, that shortcut treats anything that isn't purely numeric as a raw
// SQL WHERE fragment rather than a parameterized value, which is exactly
// the shape of bug that turns Model's route-parameter lookup into a SQL
// injection vector. "id = ?" always binds id as a parameter, whatever its
// type.
//
// It's a package-level function, not a Context method, because Go doesn't
// allow a method to carry its own type parameters - dest's type is exactly
// what makes this generic over any model, so it has to be a plain function
// taking *Context as its first argument instead of c.FindOrNotFound(...).
func FindOrNotFound[T any](c *Context, dest *T, id any) bool {
	err := c.DB().First(dest, "id = ?", id).Error
	if err == nil {
		return true
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.NotFound("resource not found")
	} else {
		c.InternalError("database error")
	}
	return false
}

// Model is route model binding in its simplest form: read the route
// parameter named param (e.g. "id" for a route "/users/:id"), look up a T
// by it, and write the 404/500 response itself on failure - collapsing
// what used to be ParamUint followed by FindOrNotFound into one call:
//
//	func UserShow(c *vento.Context) {
//	    user, ok := vento.Model[models.User](c, "id")
//	    if !ok {
//	        return
//	    }
//	    c.OK(user)
//	}
//
// The route parameter is passed to the database as-is (a string, safely
// parameterized - see FindOrNotFound) rather than parsed as an integer
// first, so a malformed ID (e.g. "abc") simply matches no row and becomes
// the same 404 a well-formed-but-nonexistent ID would. That's a deliberate
// simplification: if an API needs to distinguish "malformed ID" (400) from
// "no such record" (404), use ParamUint followed by FindOrNotFound
// instead, which keeps that distinction.
func Model[T any](c *Context, param string) (*T, bool) {
	id := c.Param(param)
	if id == "" {
		c.BadRequest("missing route parameter: " + param)
		return nil, false
	}
	var v T
	if !FindOrNotFound(c, &v, id) {
		return nil, false
	}
	return &v, true
}
