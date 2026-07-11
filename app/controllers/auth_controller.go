package controllers

import (
	"errors"

	"vento-app/app/models"
	"vento-app/vento"
	"vento-app/vento/hash"

	"gorm.io/gorm"
)

// RegisterForm is what AuthRegister binds the request body into.
type RegisterForm struct {
	Name     string `json:"name" form:"name" validate:"required,min=2,max=100"`
	Email    string `json:"email" form:"email" validate:"required,email,max=255"`
	Password string `json:"password" form:"password" validate:"required,min=8,max=72"`
}

// LoginForm is what AuthLogin binds the request body into.
type LoginForm struct {
	Email    string `json:"email" form:"email" validate:"required,email"`
	Password string `json:"password" form:"password" validate:"required"`
}

// AuthRegister handles POST /api/auth/register: creates a new user with a
// bcrypt-hashed password (see vento/hash) and immediately logs them in, the
// same way AuthLogin does. A duplicate email is rejected with 409 Conflict
// by CreateOrAbort's existing duplicate-key handling (models.User.Email has
// a unique index - see migrations/20260711_145840_add_password_to_users.go).
func AuthRegister(c *vento.Context) {
	var form RegisterForm
	if !c.BindOrAbort(&form) {
		return
	}

	hashed, err := hash.Make(form.Password)
	if err != nil {
		c.InternalError("could not create account")
		return
	}

	user := models.User{Name: form.Name, Email: form.Email, PasswordHash: hashed}
	if !vento.Query[models.User](c).CreateOrAbort(&user) {
		return
	}

	c.Login(user.ID)
	c.Created(user)
}

// AuthLogin handles POST /api/auth/login: verifies email+password and, on
// success, marks the session authenticated (see Context.Login) so
// RequireAuth-guarded routes accept subsequent requests carrying the
// resulting session cookie.
//
// A wrong email and a wrong password both get the same generic "invalid
// credentials" response - reporting "no such email" specifically would let
// an attacker enumerate registered accounts one guess at a time.
func AuthLogin(c *vento.Context) {
	var form LoginForm
	if !c.BindOrAbort(&form) {
		return
	}

	var user models.User
	err := c.DB().Where("email = ?", form.Email).First(&user).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			c.InternalError("could not process login")
			return
		}
		c.Unauthorized("invalid credentials")
		return
	}

	if !hash.Check(user.PasswordHash, form.Password) {
		c.Unauthorized("invalid credentials")
		return
	}

	c.Login(user.ID)
	c.OK(user)
}

// AuthLogout handles POST /api/auth/logout: clears the session's
// authentication (see Context.Logout). Safe to call whether or not a
// session was actually authenticated.
func AuthLogout(c *vento.Context) {
	c.Logout()
	c.NoContent()
}

// AuthMe handles GET /api/auth/me: returns the currently authenticated
// user, or 401 if the request has no valid session - the way a frontend
// checks "am I logged in" on load without hitting a specific resource.
func AuthMe(c *vento.Context) {
	user, ok := vento.CurrentUser[models.User](c)
	if !ok {
		c.Unauthorized("not logged in")
		return
	}
	c.OK(user)
}
