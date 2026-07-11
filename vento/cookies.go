package vento

import "net/http"

// This file wraps cookie reading and writing so handlers never touch
// c.Request.Cookie / http.SetCookie(c.Writer, ...) directly - and, more
// importantly, so every cookie an app sets inherits the same hardened
// defaults as Vento's own session and CSRF cookies (HttpOnly, SameSite=Lax,
// and Secure whenever the request is served over TLS). A framework that
// makes the safe thing the easy thing is one fewer place to get a cookie's
// security flags subtly wrong.

// Cookie returns the value of the named request cookie, or an error if it
// isn't present.
func (c *Context) Cookie(name string) (string, error) {
	ck, err := c.Request.Cookie(name)
	if err != nil {
		return "", err
	}
	return ck.Value, nil
}

// SetCookie sets a cookie with secure defaults: Path "/", HttpOnly,
// SameSite=Lax, and Secure when the request is served over TLS (see
// isSecure). maxAge is in seconds - positive sets an expiry, 0 makes it a
// session cookie (cleared when the browser closes), negative deletes it
// (prefer ClearCookie for that).
//
// HttpOnly means client-side JavaScript can't read the cookie, which is
// the right default for anything security-sensitive; for a value the
// front-end genuinely needs to read, set it another way (Vento's own CSRF
// cookie is the deliberate non-HttpOnly exception, handled internally).
func (c *Context) SetCookie(name, value string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecure(c.Request),
	})
}

// ClearCookie deletes the named cookie by sending it back expired - e.g. on
// logout. It matches the Path and flags SetCookie uses so the browser
// reliably drops it.
func (c *Context) ClearCookie(name string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecure(c.Request),
	})
}
