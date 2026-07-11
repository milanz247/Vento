package controllers

import (
	"vento-app/app/models"
	"vento-app/vento"
)

// UserForm is what UserCreate and UserUpdate bind the request body into -
// only the fields a client is allowed to set. Kept separate from
// models.User rather than binding into the model directly, so a client
// payload can never overwrite gorm.Model's ID/CreatedAt/UpdatedAt/
// DeletedAt columns by including them in the JSON body.
type UserForm struct {
	Name  string `json:"name" form:"name" validate:"required,min=2,max=100"`
	Email string `json:"email" form:"email" validate:"required,email"`
}

// UserIndex handles GET /api/users - a paginated list, e.g.
// /api/users?page=2&per_page=10.
func UserIndex(c *vento.Context) {
	page, err := vento.Query[models.User](c).Paginate(c.QueryInt("page", 1), c.QueryInt("per_page", vento.DefaultPerPage))
	if err != nil {
		c.InternalError("could not load users")
		return
	}
	c.OK(page)
}

// UserShow handles GET /api/users/:id.
func UserShow(c *vento.Context) {
	user, ok := vento.Model[models.User](c, "id")
	if !ok {
		return
	}
	c.OK(user)
}

// UserCreate handles POST /api/users.
func UserCreate(c *vento.Context) {
	var form UserForm
	if !c.BindOrAbort(&form) {
		return
	}

	user := models.User{Name: form.Name, Email: form.Email}
	if !vento.Query[models.User](c).CreateOrAbort(&user) {
		return
	}
	c.Created(user)
}

// UserUpdate handles PUT /api/users/:id.
func UserUpdate(c *vento.Context) {
	user, ok := vento.Model[models.User](c, "id")
	if !ok {
		return
	}

	var form UserForm
	if !c.BindOrAbort(&form) {
		return
	}

	user.Name = form.Name
	user.Email = form.Email
	if !vento.Query[models.User](c).SaveOrAbort(user) {
		return
	}
	c.OK(user)
}

// UserDelete handles DELETE /api/users/:id.
func UserDelete(c *vento.Context) {
	user, ok := vento.Model[models.User](c, "id")
	if !ok {
		return
	}
	if !vento.Query[models.User](c).DeleteOrAbort(user) {
		return
	}
	c.NoContent()
}
