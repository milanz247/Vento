package vento

import (
	"net"
	"net/http"
	"strings"
)

// TrustProxyHeaders controls whether Vento trusts X-Forwarded-Proto (to
// decide if a request is "secure", e.g. for the Secure cookie flag on
// session/CSRF cookies) and X-Forwarded-For (to decide a client's IP for
// RateLimiter) from the immediate upstream connection.
//
// Leave this false (the default) when Go talks directly to the internet.
// Enable it only when Vento sits behind a reverse proxy you control that
// overwrites or strips these headers from untrusted clients before they
// reach Go - otherwise any client can forge them: sending
// "X-Forwarded-Proto: https" would mark plaintext traffic as secure, and
// forging "X-Forwarded-For" would let a client evade RateLimiter entirely
// by claiming a new IP on every request.
//
// This is a package-level var, in the same spirit as ShutdownTimeout: set
// it once at startup, before Run, rather than threading a parameter through
// every function that needs it.
var TrustProxyHeaders = false

// isSecure reports whether r should be treated as having arrived over TLS -
// either directly (r.TLS != nil) or, when TrustProxyHeaders is enabled, via
// a proxy that terminated TLS and forwarded X-Forwarded-Proto: https.
func isSecure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return TrustProxyHeaders && strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// clientIP returns the IP RateLimiter (and anything else that needs a
// per-client key) should bucket on: the leftmost address in
// X-Forwarded-For when TrustProxyHeaders is enabled, otherwise the TCP
// connection's own remote address.
func clientIP(r *http.Request) string {
	if TrustProxyHeaders {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if i := strings.IndexByte(xff, ','); i >= 0 {
				xff = xff[:i]
			}
			if ip := strings.TrimSpace(xff); ip != "" {
				return ip
			}
		}
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
