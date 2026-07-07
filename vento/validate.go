package vento

import (
	"fmt"
	"net/mail"
	"reflect"
	"strconv"
	"strings"
)

// ValidationErrors collects every rule that failed, so a handler can report
// all of them at once instead of stopping at the first.
type ValidationErrors []string

func (e ValidationErrors) Error() string {
	return strings.Join(e, "; ")
}

// Validate checks v's fields against their `validate` struct tag - a
// comma-separated list of rules:
//
//   - required   the field must not be its zero value
//   - email      a non-empty string field must parse as an email address
//   - min=N      string length, or numeric value, must be >= N
//   - max=N      string length, or numeric value, must be <= N
//     (N may itself be a decimal, e.g. min=2.5, for float fields)
//
// It's called automatically by Bind; call it directly to validate a value
// that didn't come from a request body. v must be a struct or pointer to
// one.
func Validate(v any) error {
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("vento: Validate target must be a struct or pointer to one")
	}
	rt := rv.Type()

	var errs ValidationErrors
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		tag := field.Tag.Get("validate")
		if tag == "" || !field.IsExported() {
			continue
		}
		for rule := range strings.SplitSeq(tag, ",") {
			if msg := checkRule(field.Name, rv.Field(i), rule); msg != "" {
				errs = append(errs, msg)
			}
		}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

func checkRule(name string, v reflect.Value, rule string) string {
	rule, param, _ := strings.Cut(rule, "=")

	switch rule {
	case "required":
		if v.IsZero() {
			return name + " is required"
		}
	case "email":
		if v.Kind() == reflect.String && v.String() != "" {
			if _, err := mail.ParseAddress(v.String()); err != nil {
				return name + " must be a valid email address"
			}
		}
	case "min":
		n, _ := strconv.ParseFloat(param, 64)
		if !meetsBound(v, n, minOK) {
			return fmt.Sprintf("%s must be at least %s", name, formatBound(n))
		}
	case "max":
		n, _ := strconv.ParseFloat(param, 64)
		if !meetsBound(v, n, maxOK) {
			return fmt.Sprintf("%s must be at most %s", name, formatBound(n))
		}
	}
	return ""
}

func minOK(got, n float64) bool { return got >= n }
func maxOK(got, n float64) bool { return got <= n }

// meetsBound applies cmp to v against n: string length for string fields,
// the numeric value itself for numeric fields. Everything compares as
// float64 - comparing a float64 field's exact value against int64(v.Float())
// would silently truncate it (e.g. validate:"max=2" would let 2.9 through) -
// so float fields are never downcast. Any other kind passes unconditionally;
// min/max only make sense for those two families.
func meetsBound(v reflect.Value, n float64, cmp func(got, n float64) bool) bool {
	switch v.Kind() {
	case reflect.String:
		return cmp(float64(len(v.String())), n)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return cmp(float64(v.Int()), n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return cmp(float64(v.Uint()), n)
	case reflect.Float32, reflect.Float64:
		return cmp(v.Float(), n)
	default:
		return true
	}
}

// formatBound renders a min=/max= bound without a spurious ".0" for the
// common whole-number case (validate:"min=8") while still showing decimals
// when the tag actually specified one (validate:"min=2.5").
func formatBound(n float64) string {
	return strconv.FormatFloat(n, 'f', -1, 64)
}
