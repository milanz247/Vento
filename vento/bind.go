package vento

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// Bind decodes the request body into v, then runs Validate against v's
// `validate` struct tags: JSON if Content-Type is application/json (or
// unset, since that's Vento's default API format), otherwise POST/PUT form
// values. v must be a pointer to a struct.
//
//	type LoginForm struct {
//	    Email    string `json:"email" form:"email" validate:"required,email"`
//	    Password string `json:"password" form:"password" validate:"required,min=8"`
//	}
//
//	var form LoginForm
//	if err := c.Bind(&form); err != nil {
//	    c.Abort(http.StatusUnprocessableEntity, err.Error())
//	    return
//	}
func (c *Context) Bind(v any) error {
	if ct := c.Request.Header.Get("Content-Type"); ct != "" && !strings.HasPrefix(ct, "application/json") {
		if err := c.bindForm(v); err != nil {
			return err
		}
	} else if err := c.BindJSON(v); err != nil {
		return err
	}
	return Validate(v)
}

// BindJSON decodes the request body as JSON into v, without running
// Validate - use Bind for the common decode-then-validate case. Unknown
// fields in the body are rejected, catching typos in client payloads
// instead of silently dropping them.
func (c *Context) BindJSON(v any) error {
	dec := json.NewDecoder(c.Request.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("vento: decoding JSON body: %w", err)
	}
	return nil
}

// bindForm reads parsed POST/PUT form values into v's exported fields by
// name - a `form:"name"` tag if present, otherwise the field's Go name -
// converting each value to the field's type. Only string, bool, and the
// numeric kinds are supported; anything else is left at its zero value.
func (c *Context) bindForm(v any) error {
	if err := c.Request.ParseForm(); err != nil {
		return fmt.Errorf("vento: parsing form: %w", err)
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("vento: Bind target must be a pointer to a struct")
	}
	rv = rv.Elem()
	rt := rv.Type()

	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if !field.IsExported() {
			continue
		}
		name := field.Tag.Get("form")
		if name == "" {
			name = field.Name
		}
		raw := c.Request.FormValue(name)
		if raw == "" {
			continue
		}
		if err := setField(rv.Field(i), raw); err != nil {
			return fmt.Errorf("vento: field %s: %w", field.Name, err)
		}
	}
	return nil
}

func setField(f reflect.Value, raw string) error {
	switch f.Kind() {
	case reflect.String:
		f.SetString(raw)
	case reflect.Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return err
		}
		f.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return err
		}
		f.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return err
		}
		f.SetUint(n)
	case reflect.Float32, reflect.Float64:
		n, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return err
		}
		f.SetFloat(n)
	}
	return nil
}
