package vento

import (
	"net/http"
	"testing"
)

func TestEngineRoutesListsRegisteredRoutes(t *testing.T) {
	e := newTestEngine()
	e.GET("/users", func(*Context) {})
	e.POST("/users", func(*Context) {})
	e.GET("/users/:id", func(*Context) {})

	routes := e.Routes()
	if len(routes) != 3 {
		t.Fatalf("expected 3 routes, got %d: %+v", len(routes), routes)
	}

	want := map[string]bool{
		"GET /users":     false,
		"POST /users":    false,
		"GET /users/:id": false,
	}
	for _, r := range routes {
		key := r.Method + " " + r.Path
		if _, ok := want[key]; !ok {
			t.Fatalf("unexpected route %s", key)
		}
		want[key] = true
	}
	for key, seen := range want {
		if !seen {
			t.Fatalf("expected route %s to be present", key)
		}
	}
}

func TestEngineRoutesSortedByPathThenMethod(t *testing.T) {
	e := newTestEngine()
	e.POST("/b", func(*Context) {})
	e.GET("/b", func(*Context) {})
	e.GET("/a", func(*Context) {})

	routes := e.Routes()
	if len(routes) != 3 {
		t.Fatalf("expected 3 routes, got %d", len(routes))
	}
	if routes[0].Path != "/a" {
		t.Fatalf("expected /a first, got %+v", routes)
	}
	if routes[1].Path != "/b" || routes[1].Method != http.MethodGet {
		t.Fatalf("expected GET /b second (method-sorted before POST), got %+v", routes[1])
	}
	if routes[2].Path != "/b" || routes[2].Method != http.MethodPost {
		t.Fatalf("expected POST /b third, got %+v", routes[2])
	}
}

func TestEngineRoutesReflectsGroupPrefix(t *testing.T) {
	e := newTestEngine()
	api := e.Group("/api")
	api.GET("/health", func(*Context) {})

	routes := e.Routes()
	if len(routes) != 1 || routes[0].Path != "/api/health" {
		t.Fatalf("expected /api/health, got %+v", routes)
	}
}

func TestEngineRoutesHandlerCountIncludesMiddleware(t *testing.T) {
	e := newTestEngine()
	e.Use(func(c *Context) { c.Next() })
	e.GET("/x", func(*Context) {}, func(c *Context) { c.Next() })

	routes := e.Routes()
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}
	// 1 global middleware + 1 route middleware + 1 handler = 3
	if routes[0].HandlerCount != 3 {
		t.Fatalf("expected HandlerCount=3, got %d", routes[0].HandlerCount)
	}
}

func TestEngineRoutesEmptyWhenNoneRegistered(t *testing.T) {
	e := newTestEngine()
	if routes := e.Routes(); len(routes) != 0 {
		t.Fatalf("expected no routes, got %+v", routes)
	}
}

func TestEngineStaticMounts(t *testing.T) {
	e := newTestEngine()
	e.Static("/public", ".")

	mounts := e.StaticMounts()
	if len(mounts) != 1 || mounts[0] != "/public/" {
		t.Fatalf("expected [\"/public/\"], got %+v", mounts)
	}
}
