// Package hash holds cryptographic primitives with no dependency on
// *vento.Context - password hashing and random token generation. It's a
// separate package (rather than living in vento directly) because none of
// it needs to be a method: mirroring Laravel's distinct Hash facade, this
// gets its own clearly-named folder instead of sitting in vento's flat,
// method-constrained directory (see vento's package doc for why most of
// vento can't do this).
package hash

import (
	"crypto/rand"
	"encoding/hex"

	"golang.org/x/crypto/bcrypt"
)

// Make hashes a plaintext password with bcrypt (cost 12 - higher than
// bcrypt's default of 10, cheap enough for an interactive login but
// expensive enough to slow down offline brute-forcing of a leaked hash
// database) - Laravel's Hash::make(). The result already encodes the salt
// and cost, so it's the one value to store in the database; never store
// the plaintext password anywhere, even briefly.
//
//	hashed, err := hash.Make(form.Password)
//	if err != nil {
//	    c.InternalError("could not hash password")
//	    return
//	}
//	user := models.User{Email: form.Email, PasswordHash: hashed}
func Make(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

// Check reports whether password matches hashed (as produced by Make). The
// comparison is constant-time with respect to the password's content
// (bcrypt's design), so timing can't leak how much of a guess was correct -
// Laravel's Hash::check():
//
//	if !hash.Check(user.PasswordHash, form.Password) {
//	    c.Unauthorized("invalid credentials")
//	    return
//	}
func Check(hashed, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password)) == nil
}

// RandomString returns a cryptographically random, hex-encoded string
// built from n random bytes (so the returned string is 2*n characters) -
// the one place crypto/rand is touched anywhere in this codebase. Vento's
// own CSRF token generation and request-ID middleware both use it; reach
// for the same primitive for anything else that needs an unguessable
// identifier: a password-reset token, an API key.
//
//	token := hash.RandomString(32) // 64 hex chars, 256 bits of entropy
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
