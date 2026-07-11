package vento

import (
	"errors"
	"net/http"

	"vento-app/vento/support"

	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

// QueryHandle is a typed handle for CRUD operations against a single model
// type, obtained via Query[T](c). Its methods aren't themselves generic -
// Go doesn't allow that - so this is what makes the fluent, ordinary
// method calls in vento.Query[models.User](c).FindOrAbort(id) possible:
// the type parameter is fixed once, at construction, by Query.
//
// QueryHandle intentionally stays a thin layer over the boilerplate this
// framework can safely standardize - find-or-404, create/save/delete with
// error classification, pagination - not a replacement query builder. For
// anything it doesn't cover (joins, raw SQL, complex conditions), c.DB()
// remains the full-power escape hatch, and is exactly what QueryHandle
// itself is built on.
type QueryHandle[T any] struct {
	c *Context
}

// Query returns a QueryHandle for model type T bound to c - the entry
// point for the CreateOrAbort/SaveOrAbort/DeleteOrAbort/FindOrAbort/
// Paginate/All family:
//
//	vento.Query[models.User](c).FindOrAbort(id)
func Query[T any](c *Context) *QueryHandle[T] {
	return &QueryHandle[T]{c: c}
}

// FindOrAbort loads a record by primary key, writing the response itself
// (404 or 500) on failure. It's QueryHandle's version of the package-level
// FindOrNotFound, for use inside a chain already started from
// vento.Query[T](c).
func (q *QueryHandle[T]) FindOrAbort(id any) (*T, bool) {
	var v T
	if !FindOrNotFound(q.c, &v, id) {
		return nil, false
	}
	return &v, true
}

// All loads every row of T - fine for small, unbounded-by-design tables
// (lookup/reference data); prefer Paginate for anything a client-facing
// list endpoint returns, so response size can't grow unbounded with the
// table.
func (q *QueryHandle[T]) All() ([]T, error) {
	var items []T
	err := q.c.DB().Find(&items).Error
	return items, err
}

// Page is a paginated result: the page's rows plus enough metadata for a
// client to render pagination controls or fetch the next page, returned by
// QueryHandle.Paginate.
type Page[T any] struct {
	Data     []T `json:"data"`
	Page     int `json:"page"`
	PerPage  int `json:"per_page"`
	Total    int `json:"total"`
	LastPage int `json:"last_page"`
}

// Paginate runs a COUNT plus a LIMIT/OFFSET SELECT for a 1-based page
// number, folding the pattern every list endpoint repeats - count total,
// clamp page/perPage, compute the offset, run the query, compute the last
// page - into one call:
//
//	func UserIndex(c *vento.Context) {
//	    page, err := vento.Query[models.User](c).Paginate(c.QueryInt("page", 1), c.QueryInt("per_page", vento.DefaultPerPage))
//	    if err != nil {
//	        c.InternalError("could not load users")
//	        return
//	    }
//	    c.OK(page)
//	}
//
// page/perPage clamping (page < 1, perPage < 1 or over MaxPerPage) uses the
// same support.PaginationBounds as the standalone Paginate GORM scope.
func (q *QueryHandle[T]) Paginate(page, perPage int) (*Page[T], error) {
	limit, offset := support.PaginationBounds(page, perPage, DefaultPerPage, MaxPerPage)
	normalizedPage := offset/limit + 1

	var total int64
	if err := q.c.DB().Model(new(T)).Count(&total).Error; err != nil {
		return nil, err
	}

	var items []T
	if err := q.c.DB().Offset(offset).Limit(limit).Find(&items).Error; err != nil {
		return nil, err
	}

	lastPage := max((int(total)+limit-1)/limit, 1)

	return &Page[T]{
		Data:     items,
		Page:     normalizedPage,
		PerPage:  limit,
		Total:    int(total),
		LastPage: lastPage,
	}, nil
}

// CreateOrAbort inserts v, writing the response itself on failure and
// returning false: a duplicate-key violation becomes 409 Conflict (rather
// than the generic 500 a raw c.DB().Create(&v).Error would produce),
// anything else is logged and reported as 500.
func (q *QueryHandle[T]) CreateOrAbort(v *T) bool {
	return classifyWriteError(q.c, q.c.DB().Create(v).Error)
}

// SaveOrAbort updates v (an existing record, e.g. loaded via FindOrAbort),
// writing the response itself on failure and returning false.
func (q *QueryHandle[T]) SaveOrAbort(v *T) bool {
	return classifyWriteError(q.c, q.c.DB().Save(v).Error)
}

// DeleteOrAbort deletes v, writing the response itself on failure and
// returning false. It does not itself write a success response - follow it
// with c.NoContent() (or whatever the endpoint's convention is).
func (q *QueryHandle[T]) DeleteOrAbort(v *T) bool {
	return classifyWriteError(q.c, q.c.DB().Delete(v).Error)
}

// classifyWriteError inspects a GORM write error and writes the
// appropriate HTTP response: a MySQL duplicate-key error (1062) becomes
// 409 Conflict, gorm.ErrRecordNotFound (the row disappeared between load
// and write) becomes 404, anything else is logged and reported as a
// generic 500 - the driver/SQL detail is never leaked to the client.
// Returns true (nothing written) when err is nil.
func classifyWriteError(c *Context, err error) bool {
	if err == nil {
		return true
	}
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.NotFound("resource not found")
	case isDuplicateKeyError(err):
		c.Abort(http.StatusConflict, "a record with this value already exists")
	default:
		Log.Error("database write failed", "error", err.Error())
		c.InternalError("database error")
	}
	return false
}

// isDuplicateKeyError reports whether err is a MySQL duplicate-key
// violation (error 1062) - best-effort, driver-specific detection, not a
// guarantee for every constraint-violation shape (composite keys, foreign
// key violations fall through to the generic 500 rather than a wrong 409).
func isDuplicateKeyError(err error) bool {
	var mysqlErr *mysqldriver.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
