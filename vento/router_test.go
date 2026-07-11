package vento

import "testing"

func handler(*Context) {}

func TestRouterStaticMatch(t *testing.T) {
	r := newRouter()
	r.addRoute("GET", "/users", []HandlerFunc{handler})

	node, params := r.getRoute("GET", "/users")
	if node == nil {
		t.Fatal("expected route to match")
	}
	if len(params) != 0 {
		t.Fatalf("expected no params, got %v", params)
	}
}

func TestRouterWildcardCapture(t *testing.T) {
	r := newRouter()
	r.addRoute("GET", "/users/:id", []HandlerFunc{handler})

	node, params := r.getRoute("GET", "/users/42")
	if node == nil {
		t.Fatal("expected route to match")
	}
	if params["id"] != "42" {
		t.Fatalf("expected id=42, got %q", params["id"])
	}
}

func TestRouterStaticPreferredOverWildcard(t *testing.T) {
	r := newRouter()
	var staticHit, wildHit bool
	r.addRoute("GET", "/users/me", []HandlerFunc{func(*Context) { staticHit = true }})
	r.addRoute("GET", "/users/:id", []HandlerFunc{func(*Context) { wildHit = true }})

	node, params := r.getRoute("GET", "/users/me")
	if node == nil {
		t.Fatal("expected route to match")
	}
	node.handlers[0](nil)
	if !staticHit || wildHit {
		t.Fatalf("expected the literal /users/me route to win, static=%v wild=%v", staticHit, wildHit)
	}
	if len(params) != 0 {
		t.Fatalf("literal match should not capture params, got %v", params)
	}
}

func TestRouterBacktracksToWildcard(t *testing.T) {
	r := newRouter()
	r.addRoute("GET", "/users/me", []HandlerFunc{handler})
	r.addRoute("GET", "/users/:id/profile", []HandlerFunc{handler})

	node, params := r.getRoute("GET", "/users/42/profile")
	if node == nil {
		t.Fatal("expected wildcard route to match after static dead-end")
	}
	if params["id"] != "42" {
		t.Fatalf("expected id=42, got %v", params)
	}
}

func TestRouterNoMatch(t *testing.T) {
	r := newRouter()
	r.addRoute("GET", "/users", []HandlerFunc{handler})

	if node, _ := r.getRoute("GET", "/nope"); node != nil {
		t.Fatal("expected no match for unregistered path")
	}
	if node, _ := r.getRoute("POST", "/users"); node != nil {
		t.Fatal("expected no match for unregistered method")
	}
}

func TestRouterIntermediateNodeIsNotARoute(t *testing.T) {
	r := newRouter()
	r.addRoute("GET", "/users/:id/profile", []HandlerFunc{handler})

	// "/users/42" is an intermediate node (has children) but was never
	// registered as its own route, so it must not match.
	if node, _ := r.getRoute("GET", "/users/42"); node != nil {
		t.Fatal("expected intermediate node to not match as a route")
	}
}

func TestSplitPathCollapsesSlashes(t *testing.T) {
	got := splitPath("//users//42/")
	want := []string{"users", "42"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
