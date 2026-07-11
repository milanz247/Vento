package vento

import "testing"

type validateSubject struct {
	Name  string  `validate:"required,min=2,max=10"`
	Email string  `validate:"email"`
	Score float64 `validate:"min=2.5,max=9.5"`
}

func TestValidatePasses(t *testing.T) {
	v := validateSubject{Name: "Milan", Email: "user@example.com", Score: 5}
	if err := Validate(&v); err != nil {
		t.Fatalf("expected valid struct to pass, got %v", err)
	}
}

func TestValidateRequiredCatchesZeroValue(t *testing.T) {
	v := validateSubject{Email: "user@example.com", Score: 5}
	err := Validate(&v)
	if err == nil {
		t.Fatal("expected missing required field to fail")
	}
}

func TestValidateEmailRejectsMalformed(t *testing.T) {
	v := validateSubject{Name: "Milan", Email: "not-an-email", Score: 5}
	if err := Validate(&v); err == nil {
		t.Fatal("expected malformed email to fail")
	}
}

func TestValidateMinMaxOnStrings(t *testing.T) {
	v := validateSubject{Name: "M", Email: "user@example.com", Score: 5}
	if err := Validate(&v); err == nil {
		t.Fatal("expected name shorter than min=2 to fail")
	}
}

func TestValidateFloatBoundNotTruncated(t *testing.T) {
	// Score=2.9 must satisfy min=2.5 without being downcast to int(2).
	v := validateSubject{Name: "Milan", Email: "user@example.com", Score: 2.9}
	if err := Validate(&v); err != nil {
		t.Fatalf("expected 2.9 to satisfy min=2.5, got %v", err)
	}

	v2 := validateSubject{Name: "Milan", Email: "user@example.com", Score: 2.4}
	if err := Validate(&v2); err == nil {
		t.Fatal("expected 2.4 to fail min=2.5")
	}
}
