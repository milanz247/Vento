package vento

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestContext(method, target string) (*Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, target, nil)
	c := &Context{index: -1}
	c.Reset(rec, req)
	return c, rec
}

func TestOKWritesJSON200(t *testing.T) {
	c, rec := newTestContext(http.MethodGet, "/")
	c.OK(H{"hello": "world"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected JSON body, got %q", rec.Body.String())
	}
	if body["hello"] != "world" {
		t.Fatalf("unexpected body: %v", body)
	}
}

func TestCreatedWritesJSON201(t *testing.T) {
	c, rec := newTestContext(http.MethodPost, "/")
	c.Created(H{"id": "1"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
}

func TestNoContentWrites204Empty(t *testing.T) {
	c, rec := newTestContext(http.MethodDelete, "/")
	c.NoContent()
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %q", rec.Body.String())
	}
}

func TestRedirectSends302WithLocation(t *testing.T) {
	c, rec := newTestContext(http.MethodGet, "/")
	c.Redirect("/login")
	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/login" {
		t.Fatalf("expected Location /login, got %q", loc)
	}
	if c.StatusCode != http.StatusFound {
		t.Fatalf("expected c.StatusCode to record 302 for the logger, got %d", c.StatusCode)
	}
}

func TestErrorHelpersSetStatusAndStopChain(t *testing.T) {
	cases := []struct {
		name string
		call func(*Context)
		want int
	}{
		{"BadRequest", func(c *Context) { c.BadRequest("x") }, http.StatusBadRequest},
		{"Unauthorized", func(c *Context) { c.Unauthorized("x") }, http.StatusUnauthorized},
		{"Forbidden", func(c *Context) { c.Forbidden("x") }, http.StatusForbidden},
		{"NotFound", func(c *Context) { c.NotFound("x") }, http.StatusNotFound},
		{"InternalError", func(c *Context) { c.InternalError("x") }, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			ran := false
			c := &Context{index: -1}
			c.Reset(rec, req)
			c.handlers = []HandlerFunc{
				tc.call,
				func(c *Context) { ran = true }, // must NOT run - the error aborts
			}
			c.Next()

			if rec.Code != tc.want {
				t.Fatalf("expected status %d, got %d", tc.want, rec.Code)
			}
			if ran {
				t.Fatal("expected the error helper to abort the chain, but a later handler ran")
			}
			var body map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["error"] != "x" {
				t.Fatalf("expected {\"error\":\"x\"} body, got %q", rec.Body.String())
			}
		})
	}
}
