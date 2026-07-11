package vento

import (
	"net/http"
	"testing"
)

func TestParamInt(t *testing.T) {
	c, _ := newTestContext(http.MethodGet, "/users/42")
	c.params = map[string]string{"id": "42"}

	n, err := c.ParamInt("id")
	if err != nil || n != 42 {
		t.Fatalf("expected 42, got %d (err=%v)", n, err)
	}
}

func TestParamIntRejectsNonNumeric(t *testing.T) {
	c, _ := newTestContext(http.MethodGet, "/users/abc")
	c.params = map[string]string{"id": "abc"}

	if _, err := c.ParamInt("id"); err == nil {
		t.Fatal("expected an error for a non-numeric param")
	}
}

func TestParamUintRejectsNegative(t *testing.T) {
	c, _ := newTestContext(http.MethodGet, "/users/-1")
	c.params = map[string]string{"id": "-1"}

	if _, err := c.ParamUint("id"); err == nil {
		t.Fatal("expected an error for a negative uint param")
	}
}

func TestQueryDefault(t *testing.T) {
	c, _ := newTestContext(http.MethodGet, "/?sort=name")
	if got := c.QueryDefault("sort", "id"); got != "name" {
		t.Fatalf("expected present value 'name', got %q", got)
	}
	if got := c.QueryDefault("missing", "id"); got != "id" {
		t.Fatalf("expected default 'id', got %q", got)
	}
}

func TestQueryInt(t *testing.T) {
	c, _ := newTestContext(http.MethodGet, "/?page=3&bad=xyz")
	if got := c.QueryInt("page", 1); got != 3 {
		t.Fatalf("expected 3, got %d", got)
	}
	if got := c.QueryInt("bad", 1); got != 1 {
		t.Fatalf("expected fallback 1 for non-numeric, got %d", got)
	}
	if got := c.QueryInt("missing", 7); got != 7 {
		t.Fatalf("expected fallback 7 for missing, got %d", got)
	}
}

func TestHeader(t *testing.T) {
	c, _ := newTestContext(http.MethodGet, "/")
	c.Request.Header.Set("Authorization", "Bearer xyz")
	if got := c.Header("Authorization"); got != "Bearer xyz" {
		t.Fatalf("expected the Authorization header, got %q", got)
	}
}
