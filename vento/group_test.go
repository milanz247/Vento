package vento

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGroupPrefixesRoutes(t *testing.T) {
	e := newTestEngine()
	api := e.Group("/api")
	api.GET("/health", func(c *Context) { c.OK(H{"ok": true}) })

	if node, _ := e.router.getRoute(http.MethodGet, "/api/health"); node == nil {
		t.Fatal("expected group route registered at /api/health")
	}
	if node, _ := e.router.getRoute(http.MethodGet, "/health"); node != nil {
		t.Fatal("did not expect the route at /health without the prefix")
	}
}

func TestGroupNormalizesSlashes(t *testing.T) {
	e := newTestEngine()
	// Messy prefix and path should still resolve to /api/users.
	api := e.Group("api/")
	api.GET("users", func(c *Context) {})

	if node, _ := e.router.getRoute(http.MethodGet, "/api/users"); node == nil {
		t.Fatal("expected /api/users regardless of slash sloppiness")
	}
}

func TestGroupMiddlewareRunsInOrder(t *testing.T) {
	e := newTestEngine() // bare engine: no global middleware
	var order []string

	groupMW := func(c *Context) { order = append(order, "group"); c.Next() }
	routeMW := func(c *Context) { order = append(order, "route"); c.Next() }
	handler := func(c *Context) { order = append(order, "handler") }

	api := e.Group("/api", groupMW)
	api.GET("/x", handler, routeMW)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/x", nil)
	e.ServeHTTP(rec, req)

	want := []string{"group", "route", "handler"}
	if len(order) != len(want) {
		t.Fatalf("expected order %v, got %v", want, order)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("expected order %v, got %v", want, order)
		}
	}
}

func TestNestedGroupsAccumulate(t *testing.T) {
	e := newTestEngine()
	var hits []string

	outer := e.Group("/api", func(c *Context) { hits = append(hits, "outer"); c.Next() })
	inner := outer.Group("/v1", func(c *Context) { hits = append(hits, "inner"); c.Next() })
	inner.GET("/ping", func(c *Context) { hits = append(hits, "handler") })

	node, _ := e.router.getRoute(http.MethodGet, "/api/v1/ping")
	if node == nil {
		t.Fatal("expected nested route at /api/v1/ping")
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil)
	e.ServeHTTP(rec, req)

	want := []string{"outer", "inner", "handler"}
	if len(hits) != len(want) {
		t.Fatalf("expected %v, got %v", want, hits)
	}
	for i := range want {
		if hits[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, hits)
		}
	}
}

func TestParentGroupMiddlewareNotAliased(t *testing.T) {
	// Registering two child routes off one group must not let the first
	// route's middleware leak into the second via a shared backing array.
	e := newTestEngine()
	base := e.Group("/api", func(c *Context) { c.Next() })

	base.GET("/a", func(c *Context) {}, func(c *Context) { c.Next() })
	base.GET("/b", func(c *Context) {})

	nodeA, _ := e.router.getRoute(http.MethodGet, "/api/a")
	nodeB, _ := e.router.getRoute(http.MethodGet, "/api/b")
	// /a: group mw + route mw + handler = 3; /b: group mw + handler = 2.
	if len(nodeA.handlers) != 3 {
		t.Fatalf("expected 3 handlers on /api/a, got %d", len(nodeA.handlers))
	}
	if len(nodeB.handlers) != 2 {
		t.Fatalf("expected 2 handlers on /api/b (no aliasing), got %d", len(nodeB.handlers))
	}
}

func TestEnginePATCH(t *testing.T) {
	e := newTestEngine()
	e.PATCH("/users/:id", func(c *Context) { c.NoContent() })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/users/1", nil)
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected PATCH route to respond 204, got %d", rec.Code)
	}
}
