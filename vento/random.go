package vento

import (
	"crypto/rand"
	"encoding/hex"
)

// RandomString returns a cryptographically random, hex-encoded string built
// from n random bytes (so the returned string is 2*n characters). It's the
// one place Vento generates a random token - CSRF tokens (security.go) use
// it internally, and application code reaches for the same primitive for
// anything else that needs an unguessable identifier: a request ID, a
// password-reset token, an API key.
//
//	token := vento.RandomString(32) // 64 hex chars, 256 bits of entropy
//
// n should be at least 16 (128 bits) for anything security-sensitive; 32 is
// a comfortable default. Returns "" only if the system CSPRNG is
// unavailable, which in practice never happens on a real OS.
func RandomString(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}
