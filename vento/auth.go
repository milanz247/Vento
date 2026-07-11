package vento

import "fmt"

// authSessionKey is the session key Login/Logout/AuthID use to store the
// authenticated user's ID. It's unexported so "the session is
// authenticated" can only be established through Login, in one place,
// rather than by any handler that happens to Session().Set the right key.
const authSessionKey = "_auth_user_id"

// Login marks the current session as authenticated for userID - the one
// step a login handler needs after verifying credentials (see
// CheckPassword). userID is stored as its string form (via fmt.Sprint), so
// any ID type - uint, int, a UUID string - round-trips through the signed
// session cookie without JSON's usual "every number comes back as
// float64" surprise (see Session's doc comment).
//
//	if !vento.CheckPassword(user.PasswordHash, form.Password) {
//	    c.Unauthorized("invalid credentials")
//	    return
//	}
//	c.Login(user.ID)
//	c.OK(user)
//
// Login requires the Sessions middleware to be installed (see
// vento.Sessions) - without it, exactly like any other Session().Set call,
// the login "succeeds" but never persists past this response, and
// Context.Session already logs a warning the first time that happens.
func (c *Context) Login(userID any) {
	c.Session().Set(authSessionKey, fmt.Sprint(userID))
}

// Logout clears the current session's authentication - e.g. on a logout
// endpoint. It leaves the rest of the session's data untouched; call
// c.Session().Clear() instead if a logout should wipe everything.
func (c *Context) Logout() {
	c.Session().Delete(authSessionKey)
}

// AuthID returns the authenticated user's ID (as stored by Login) and
// whether one is present - the primitive Authenticated and CurrentUser are
// built on, for callers that just need the ID without a database round
// trip (e.g. to stamp an "updated_by" column).
func (c *Context) AuthID() (string, bool) {
	v, ok := c.Session().Get(authSessionKey)
	if !ok {
		return "", false
	}
	id, ok := v.(string)
	return id, ok
}

// Authenticated reports whether the current session has a logged-in user -
// Laravel's Auth::check().
func (c *Context) Authenticated() bool {
	_, ok := c.AuthID()
	return ok
}

// CurrentUser loads the authenticated user's model by primary key -
// Laravel's auth()->user(), adapted to Go's type system: a method can't
// carry its own type parameter, so this is a package-level function
// generic over the model type instead of a Context method, called with an
// explicit type argument:
//
//	user, ok := vento.CurrentUser[models.User](c)
//	if !ok {
//	    c.Unauthorized("not logged in")
//	    return
//	}
//
// ok is false both when nobody is logged in and when the logged-in user's
// ID no longer resolves to a row (e.g. the account was deleted since the
// session cookie was issued) - either way, there's no user to hand back.
func CurrentUser[T any](c *Context) (*T, bool) {
	id, ok := c.AuthID()
	if !ok {
		return nil, false
	}
	var user T
	if err := c.DB().First(&user, "id = ?", id).Error; err != nil {
		return nil, false
	}
	return &user, true
}

// RequireAuth is a middleware that aborts with 401 Unauthorized unless the
// request has an authenticated session (see Login/Authenticated) - guard
// any route or group that needs a logged-in user:
//
//	admin := app.Group("/admin", vento.RequireAuth)
//	admin.GET("/dashboard", controllers.Dashboard)
//
// It must run after Sessions in the middleware chain (Sessions is what
// populates the session Authenticated reads) - the usual app.Use ordering
// in main.go (Sessions before route tables) already guarantees this.
func RequireAuth(c *Context) {
	if !c.Authenticated() {
		c.Unauthorized("authentication required")
		return
	}
	c.Next()
}
