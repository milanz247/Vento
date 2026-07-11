package vento

import "testing"

func TestRandomStringLength(t *testing.T) {
	s := RandomString(16)
	if len(s) != 32 { // hex-encoded: 2 chars per byte
		t.Fatalf("expected a 32-char string for 16 random bytes, got %d chars (%q)", len(s), s)
	}
}

func TestRandomStringIsRandom(t *testing.T) {
	a := RandomString(32)
	b := RandomString(32)
	if a == b {
		t.Fatal("expected two calls to produce different values")
	}
}
