package vento

import "testing"

func TestHashPasswordRoundTrips(t *testing.T) {
	hash, err := HashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hash == "" || hash == "correct-horse-battery-staple" {
		t.Fatalf("expected a non-plaintext hash, got %q", hash)
	}
	if !CheckPassword(hash, "correct-horse-battery-staple") {
		t.Fatal("expected the correct password to verify")
	}
}

func TestCheckPasswordRejectsWrongPassword(t *testing.T) {
	hash, err := HashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if CheckPassword(hash, "wrong-password") {
		t.Fatal("expected an incorrect password to be rejected")
	}
}

func TestHashPasswordProducesDifferentHashesForSamePassword(t *testing.T) {
	h1, _ := HashPassword("same-password")
	h2, _ := HashPassword("same-password")
	if h1 == h2 {
		t.Fatal("expected distinct salts to produce distinct hashes for the same password")
	}
	if !CheckPassword(h1, "same-password") || !CheckPassword(h2, "same-password") {
		t.Fatal("expected both hashes to independently verify the same password")
	}
}
