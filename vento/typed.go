package vento

import "reflect"

// Provide and Use are a type-safe, request-scoped "stash this, retrieve it
// later" store, keyed by the value's own type rather than a string a
// caller could typo - the foundation a future route-model-binding
// middleware or similar feature can build on. Available today for app
// code to use directly for lightweight request-scoped dependency injection
// (a resolved tenant, a per-request logger) without a DI container
// framework or global state:
//
//	app.Use(func(c *vento.Context) {
//	    vento.Provide(c, tenant.FromHost(c.Request.Host))
//	    c.Next()
//	})
//
//	func Handler(c *vento.Context) {
//	    t, ok := vento.Use[*Tenant](c)
//	}
//
// Both are package-level functions rather than Context methods because Go
// doesn't allow a method to carry its own type parameter.
//
// The store is keyed by reflect.TypeOf(v), so two Provide calls with the
// same concrete type collide - the second silently overwrites the first.
// Always provide named types (type TenantID string), never bare
// primitives (string, int), to avoid an accidental collision between two
// unrelated values that happen to share a built-in type.
//
// T must be a concrete type (a struct, a pointer, a named type) for Use to
// find it; an interface type parameter looks up as untyped nil and never
// matches, since a nil interface carries no type to key on.

// Provide stashes v on c, retrievable later via Use[T] for the rest of
// this request.
func Provide[T any](c *Context, v T) {
	if c.typed == nil {
		c.typed = make(map[reflect.Type]any)
	}
	c.typed[reflect.TypeOf(v)] = v
}

// Use retrieves a value previously stashed on c via Provide[T], and
// whether one was found.
func Use[T any](c *Context) (T, bool) {
	var zero T
	if c.typed == nil {
		return zero, false
	}
	v, ok := c.typed[reflect.TypeOf(zero)]
	if !ok {
		return zero, false
	}
	typed, ok := v.(T)
	return typed, ok
}
