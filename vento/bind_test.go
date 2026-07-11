package vento

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type loginForm struct {
	Email    string `json:"email" form:"email" validate:"required,email"`
	Password string `json:"password" form:"password" validate:"required,min=8"`
}

func TestBindJSONDecodesAndValidates(t *testing.T) {
	body := `{"email":"user@example.com","password":"supersecret"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	c := &Context{index: -1}
	c.Reset(httptest.NewRecorder(), req)

	var form loginForm
	if err := c.Bind(&form); err != nil {
		t.Fatalf("expected valid payload to bind cleanly, got %v", err)
	}
	if form.Email != "user@example.com" || form.Password != "supersecret" {
		t.Fatalf("unexpected bound values: %+v", form)
	}
}

func TestBindJSONRejectsUnknownFields(t *testing.T) {
	body := `{"email":"user@example.com","password":"supersecret","admin":true}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	c := &Context{index: -1}
	c.Reset(httptest.NewRecorder(), req)

	var form loginForm
	if err := c.Bind(&form); err == nil {
		t.Fatal("expected an unknown field to be rejected, not silently dropped")
	}
}

func TestBindJSONFailsValidation(t *testing.T) {
	body := `{"email":"not-an-email","password":"short"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	c := &Context{index: -1}
	c.Reset(httptest.NewRecorder(), req)

	var form loginForm
	err := c.Bind(&form)
	if err == nil {
		t.Fatal("expected validation to fail for invalid email and short password")
	}
	if !strings.Contains(err.Error(), "email") || !strings.Contains(err.Error(), "Password") {
		t.Fatalf("expected both field errors reported, got %v", err)
	}
}

func TestBindFormDecodesFields(t *testing.T) {
	form := url.Values{"email": {"user@example.com"}, "password": {"supersecret"}}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	c := &Context{index: -1}
	c.Reset(httptest.NewRecorder(), req)

	var lf loginForm
	if err := c.Bind(&lf); err != nil {
		t.Fatalf("expected form binding to succeed, got %v", err)
	}
	if lf.Email != "user@example.com" || lf.Password != "supersecret" {
		t.Fatalf("unexpected bound values: %+v", lf)
	}
}

type numericForm struct {
	Count int64   `form:"count"`
	Ratio float64 `form:"ratio"`
	Ok    bool    `form:"ok"`
}

func TestBindFormConvertsNumericAndBoolFields(t *testing.T) {
	form := url.Values{"count": {"42"}, "ratio": {"3.5"}, "ok": {"true"}}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	c := &Context{index: -1}
	c.Reset(httptest.NewRecorder(), req)

	var nf numericForm
	if err := c.bindForm(&nf); err != nil {
		t.Fatalf("expected numeric form fields to convert, got %v", err)
	}
	if nf.Count != 42 || nf.Ratio != 3.5 || !nf.Ok {
		t.Fatalf("unexpected bound values: %+v", nf)
	}
}
