package vento

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// SecurityHeaders is a built-in middleware that stamps standard hardening
// headers onto every response before the rest of the chain runs.
//
// X-XSS-Protection is set to "0" per current OWASP guidance: the legacy
// browser XSS auditor it used to enable has been removed from modern
// browsers and was itself a vector for information leaks in older ones,
// so explicitly disabling it is the hardened setting. XSS defense comes
// from html/template's contextual auto-escaping instead.
func SecurityHeaders(c *Context) {
	h := c.Writer.Header()
	h.Set("X-Frame-Options", "DENY")
	h.Set("X-XSS-Protection", "0")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
	c.Next()
}

// BodyLimit returns a middleware that caps the request body at maxBytes.
// It wraps the body in http.MaxBytesReader, so a handler reading past the
// limit gets an error (and the connection is closed) instead of letting a
// client stream an unbounded body into memory - e.g. through
// json.NewDecoder or ParseForm. 1 << 20 (1 MiB) is a sensible default for
// form/JSON endpoints; raise it only on routes that genuinely accept
// large uploads.
func BodyLimit(maxBytes int64) HandlerFunc {
	return func(c *Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}

// bucket is one client's token-bucket state. Each bucket has its own
// mutex so refill/spend stays atomic per client while the sync.Map
// handles concurrent lookup/insert across clients without a global lock.
type bucket struct {
	mu     sync.Mutex
	tokens float64
	last   time.Time
}

// RateLimiter returns a token-bucket rate-limiting middleware: each
// client IP accrues rps tokens per second up to burst, and each request
// spends one token. A client with an empty bucket receives 429 Too Many
// Requests with a Retry-After hint. Buckets live in a sync.Map; entries
// idle for several minutes are purged opportunistically (at most once a
// minute) so the map cannot grow without bound under address-spoofed
// traffic.
//
// The client key is the connection's remote IP. Behind a reverse proxy,
// rate-limit at the proxy instead, or extend this to trust
// X-Forwarded-For only from known proxy addresses - trusting it blindly
// would let clients evade the limiter with a forged header.
func RateLimiter(rps float64, burst float64) HandlerFunc {
	var (
		buckets   sync.Map // ip string -> *bucket
		lastPurge atomic.Int64
	)
	lastPurge.Store(time.Now().UnixNano())

	return func(c *Context) {
		ip, _, err := net.SplitHostPort(c.Request.RemoteAddr)
		if err != nil {
			ip = c.Request.RemoteAddr
		}

		now := time.Now()

		// Opportunistic purge of idle clients, at most once a minute.
		if prev := lastPurge.Load(); now.UnixNano()-prev > int64(time.Minute) &&
			lastPurge.CompareAndSwap(prev, now.UnixNano()) {
			buckets.Range(func(key, value any) bool {
				b := value.(*bucket)
				b.mu.Lock()
				idle := now.Sub(b.last) > 3*time.Minute
				b.mu.Unlock()
				if idle {
					buckets.Delete(key)
				}
				return true
			})
		}

		value, _ := buckets.LoadOrStore(ip, &bucket{tokens: burst, last: now})
		b := value.(*bucket)

		b.mu.Lock()
		b.tokens += now.Sub(b.last).Seconds() * rps
		if b.tokens > burst {
			b.tokens = burst
		}
		b.last = now
		allowed := b.tokens >= 1
		if allowed {
			b.tokens--
		}
		b.mu.Unlock()

		if !allowed {
			c.Writer.Header().Set("Retry-After", "1")
			c.Abort(http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		c.Next()
	}
}

// CSRFCookieName is the cookie carrying the CSRF token, and
// CSRFHeaderName is the request header clients echo it back in
// (double-submit cookie pattern). Form posts may alternatively send the
// token in a "_csrf" field.
const (
	CSRFCookieName = "vento_csrf"
	CSRFHeaderName = "X-CSRF-Token"
	csrfFormField  = "_csrf"
)

// CSRFProtection returns a middleware implementing double-submit-cookie
// CSRF protection. Safe methods (GET, HEAD, OPTIONS, TRACE) pass through
// and are issued a random token cookie if they don't have one yet. Every
// non-idempotent method (POST, PUT, DELETE, ...) must echo that cookie's
// value back in the X-CSRF-Token header or a "_csrf" form field, compared
// in constant time; a missing or wrong token aborts with 403.
//
// exemptPrefixes lists path prefixes excluded from validation - typically
// token-authenticated JSON APIs, which are not CSRF-vulnerable because
// browsers never attach their credentials automatically.
func CSRFProtection(exemptPrefixes ...string) HandlerFunc {
	return func(c *Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
			if _, err := c.Request.Cookie(CSRFCookieName); err != nil {
				raw := make([]byte, 32)
				if _, err := rand.Read(raw); err != nil {
					c.Abort(http.StatusInternalServerError, "could not issue CSRF token")
					return
				}
				http.SetCookie(c.Writer, &http.Cookie{
					Name:     CSRFCookieName,
					Value:    hex.EncodeToString(raw),
					Path:     "/",
					SameSite: http.SameSiteLaxMode,
					// Secure whenever the request itself arrived over TLS, so
					// an HTTPS deployment never sends the token in clear text.
					// (Behind a TLS-terminating proxy the hop to Go is plain
					// HTTP; set the flag at the proxy in that topology.)
					Secure: c.Request.TLS != nil,
					// Not HttpOnly by design: front-end JS must read it to
					// echo it back in the X-CSRF-Token header.
				})
			}
			c.Next()
			return
		}

		for _, prefix := range exemptPrefixes {
			if strings.HasPrefix(c.Request.URL.Path, prefix) {
				c.Next()
				return
			}
		}

		cookie, err := c.Request.Cookie(CSRFCookieName)
		if err != nil || cookie.Value == "" {
			c.Abort(http.StatusForbidden, "CSRF token missing")
			return
		}

		token := c.Request.Header.Get(CSRFHeaderName)
		if token == "" {
			token = c.FormValue(csrfFormField)
		}

		if subtle.ConstantTimeCompare([]byte(token), []byte(cookie.Value)) != 1 {
			c.Abort(http.StatusForbidden, "CSRF token invalid")
			return
		}
		c.Next()
	}
}
