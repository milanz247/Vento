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
	var users []models.User
	page := c.QueryInt("page", 1)
	perPage := c.QueryInt("per_page", vento.DefaultPerPage)

	if err := c.DB().Scopes(vento.Paginate(page, perPage)).Find(&users).Error; err != nil {
		c.InternalError("could not load users")
		return
	}
	c.OK(users)
}

// UserShow handles GET /api/users/:id.
func UserShow(c *vento.Context) {
	id, err := c.ParamUint("id")
	if err != nil {
		c.BadRequest("invalid id")
		return
	}

	var user models.User
	if !vento.FindOrNotFound(c, &user, id) {
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
	if err := c.DB().Create(&user).Error; err != nil {
		c.InternalError("could not create user")
		return
	}
	c.Created(user)
}

// UserUpdate handles PUT /api/users/:id.
func UserUpdate(c *vento.Context) {
	id, err := c.ParamUint("id")
	if err != nil {
		c.BadRequest("invalid id")
		return
	}

	var user models.User
	if !vento.FindOrNotFound(c, &user, id) {
		return
	}

	var form UserForm
	if !c.BindOrAbort(&form) {
		return
	}

	user.Name = form.Name
	user.Email = form.Email
	if err := c.DB().Save(&user).Error; err != nil {
		c.InternalError("could not update user")
		return
	}
	c.OK(user)
}

// UserDelete handles DELETE /api/users/:id.
func UserDelete(c *vento.Context) {
	id, err := c.ParamUint("id")
	if err != nil {
		c.BadRequest("invalid id")
		return
	}

	var user models.User
	if !vento.FindOrNotFound(c, &user, id) {
		return
	}

	if err := c.DB().Delete(&user).Error; err != nil {
		c.InternalError("could not delete user")
		return
	}
	c.NoContent()
}
