package hash

import "testing"

func TestMakeRoundTrips(t *testing.T) {
	hashed, err := Make("correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hashed == "" || hashed == "correct-horse-battery-staple" {
		t.Fatalf("expected a non-plaintext hash, got %q", hashed)
	}
	if !Check(hashed, "correct-horse-battery-staple") {
		t.Fatal("expected the correct password to verify")
	}
}

func TestCheckRejectsWrongPassword(t *testing.T) {
	hashed, err := Make("correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if Check(hashed, "wrong-password") {
		t.Fatal("expected an incorrect password to be rejected")
	}
}

func TestMakeProducesDifferentHashesForSamePassword(t *testing.T) {
	h1, _ := Make("same-password")
	h2, _ := Make("same-password")
	if h1 == h2 {
		t.Fatal("expected distinct salts to produce distinct hashes for the same password")
	}
	if !Check(h1, "same-password") || !Check(h2, "same-password") {
		t.Fatal("expected both hashes to independently verify the same password")
	}
}

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
