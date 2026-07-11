package vento

import "golang.org/x/crypto/bcrypt"

// HashPassword hashes a plaintext password with bcrypt (cost 12 - higher
// than bcrypt's default of 10, cheap enough for an interactive login but
// expensive enough to slow down offline brute-forcing of a leaked hash
// database) - Laravel's Hash::make(), adapted to Go. The result already
// encodes the salt and cost, so it's the one value to store in the
// database; never store the plaintext password anywhere, even briefly.
//
//	hash, err := vento.HashPassword(form.Password)
//	if err != nil {
//	    c.InternalError("could not hash password")
//	    return
//	}
//	user := models.User{Email: form.Email, PasswordHash: hash}
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword reports whether password matches hash (as produced by
// HashPassword). The comparison is constant-time with respect to the
// password's content (bcrypt's design), so timing can't leak how much of a
// guess was correct - Laravel's Hash::check():
//
//	if !vento.CheckPassword(user.PasswordHash, form.Password) {
//	    c.Unauthorized("invalid credentials")
//	    return
//	}
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
