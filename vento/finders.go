package vento

import (
	"errors"

	"gorm.io/gorm"
)

// FindOrNotFound looks up a record by primary key into dest (via
// c.DB().First) and handles the two ways that can fail: a 404 response if
// no such record exists, or a 500 if the lookup itself errors (a dropped
// connection, a malformed query) - the find-or-fail pattern nearly every
// Show/Update/Delete handler needs, without hand-rolling the
// errors.Is(err, gorm.ErrRecordNotFound) check on every one:
//
//	var user models.User
//	if !vento.FindOrNotFound(c, &user, id) {
//	    return
//	}
//	// user is loaded past this point
//
// It's a package-level function, not a Context method, because Go doesn't
// allow a method to carry its own type parameters - dest's type is exactly
// what makes this generic over any model, so it has to be a plain function
// taking *Context as its first argument instead of c.FindOrNotFound(...).
func FindOrNotFound[T any](c *Context, dest *T, id any) bool {
	err := c.DB().First(dest, id).Error
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
