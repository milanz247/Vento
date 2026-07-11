// Package vtest provides unit-testing helpers for Vento controllers - it's
// a separate package (rather than living in vento itself) so importing the
// main vento package for production code never pulls in net/http/httptest,
// the same reasoning net/http/httptest itself is split out from net/http.
package vtest

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"

	"vento-app/vento"
)

// NewContext builds a *vento.Context and its backing
// httptest.ResponseRecorder for unit-testing a controller directly, with
// no running server, no middleware chain, and no database unless one is
// wired in via c.SetDB.
//
// body is JSON-encoded as the request body when non-nil (and Content-Type
// is set to application/json, so c.Bind/BindOrAbort behave exactly as they
// would for a real client); pass nil for a body-less request (a GET, a
// DELETE by ID).
//
// params simulates the route parameters a real request would have had
// captured by the router (e.g. {"id": "5"} for a route "/users/:id") -
// pass nil for a route with none. There's no router involved here, so
// target's path is not parsed for parameters; params is the only way a
// controller's c.Param/c.ParamInt/vento.Model[T] calls see anything.
//
//	c, rec := vtest.NewContext(http.MethodPost, "/", UserForm{Name: "Ann", Email: "a@x.com"}, nil)
//	c.SetDB(testDB) // only needed if the controller under test touches the DB
//	controllers.UserCreate(c)
//	if rec.Code != http.StatusCreated {
//	    t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body)
//	}
//
//	c, rec := vtest.NewContext(http.MethodGet, "/users/5", nil, map[string]string{"id": "5"})
//	c.SetDB(testDB)
//	controllers.UserShow(c)
//
// No middleware runs - CSRF, sessions, and rate limiting are not applied.
// That's deliberate: this is a unit-test tool for controller logic. For a
// test that needs the real chain, build the *vento.Engine as main.go does
// and drive it with httptest.NewServer.
func NewContext(method, target string, body any, params map[string]string) (*vento.Context, *httptest.ResponseRecorder) {
	var reqBody io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewReader(b)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, target, reqBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	c := &vento.Context{}
	c.Reset(rec, req)
	if params != nil {
		c.SetParams(params)
	}
	return c, rec
}

// DecodeJSON decodes a recorded response body as T - the counterpart to
// NewContext for asserting on what a controller wrote:
//
//	got, err := vtest.DecodeJSON[models.User](rec)
func DecodeJSON[T any](rec *httptest.ResponseRecorder) (T, error) {
	var v T
	err := json.Unmarshal(rec.Body.Bytes(), &v)
	return v, err
}
