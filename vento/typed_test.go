package vento

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type testTenant struct{ Name string }

func TestProvideAndUseRoundTrip(t *testing.T) {
	c, _ := newTestContext(http.MethodGet, "/")

	if _, ok := Use[*testTenant](c); ok {
		t.Fatal("expected Use to fail before anything was Provided")
	}

	Provide(c, &testTenant{Name: "acme"})

	got, ok := Use[*testTenant](c)
	if !ok {
		t.Fatal("expected Use to find the provided value")
	}
	if got.Name != "acme" {
		t.Fatalf("expected Name=acme, got %q", got.Name)
	}
}

func TestUseDistinguishesTypes(t *testing.T) {
	c, _ := newTestContext(http.MethodGet, "/")
	Provide(c, "a string value")

	if _, ok := Use[*testTenant](c); ok {
		t.Fatal("expected Use[*testTenant] to not match a provided string")
	}
	s, ok := Use[string](c)
	if !ok || s != "a string value" {
		t.Fatalf("expected the string to round-trip, got %q (ok=%v)", s, ok)
	}
}

func TestProvideOverwritesSameType(t *testing.T) {
	c, _ := newTestContext(http.MethodGet, "/")
	Provide(c, &testTenant{Name: "first"})
	Provide(c, &testTenant{Name: "second"})

	got, ok := Use[*testTenant](c)
	if !ok || got.Name != "second" {
		t.Fatalf("expected the second Provide to win, got %+v (ok=%v)", got, ok)
	}
}

func TestResetClearsTypedStore(t *testing.T) {
	c, _ := newTestContext(http.MethodGet, "/")
	Provide(c, &testTenant{Name: "acme"})

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	c.Reset(rec2, req2)

	if _, ok := Use[*testTenant](c); ok {
		t.Fatal("expected Reset to clear the typed store")
	}
}
