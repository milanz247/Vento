package vento

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
)

// SessionCookieName is the cookie Sessions stores signed session data in.
const SessionCookieName = "vento_session"

// Session is per-request key/value storage backed by a signed cookie - a
// lightweight alternative to a server-side session store, loaded and saved
// automatically by the Sessions middleware and reached via c.Session().
//
// Session data rides in the cookie itself (base64-encoded JSON, HMAC-signed
// against tampering), so it is tamper-proof but NOT confidential - the
// client can read it. Never store secrets in it (passwords, raw tokens);
// store an opaque reference instead (e.g. a user ID) and look the rest up
// from the database. Values also round-trip through JSON, so a number
// stored as int comes back as float64 on the next request - the same
// caveat as any JSON-based cache.
type Session struct {
	mu    sync.RWMutex
	data  map[string]any
	dirty bool
}

// Get returns a value previously stored with Set, and whether it was found.
func (s *Session) Get(key string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[key]
	return v, ok
}

// Set stores a value under key, marking the session dirty so the Sessions
// middleware re-signs and re-sends the cookie on this response.
func (s *Session) Set(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data == nil {
		s.data = make(map[string]any)
	}
	s.data[key] = value
	s.dirty = true
}

// Delete removes key from the session, if present.
func (s *Session) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[key]; ok {
		delete(s.data, key)
		s.dirty = true
	}
}

// Clear empties the session entirely - e.g. on logout.
func (s *Session) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.data) > 0 {
		s.data = make(map[string]any)
		s.dirty = true
	}
}

// Sessions returns a middleware that loads a signed session cookie into
// each request's Context (see c.Session()) and, if the handler chain
// changed it, transparently re-signs and re-sends it - no explicit save
// call required, even from inside c.View, which writes its own headers.
//
// secret is stretched into a 256-bit HMAC key via SHA-256, so any
// length/format works; keep it stable across restarts (e.g. an APP_KEY in
// .env) or every session breaks on deploy.
//
// Not part of DefaultMiddleware since it needs that app-specific secret;
// wire it in explicitly, typically only on routes that need login state:
//
//	app.Use(vento.Sessions(env["APP_KEY"]))
func Sessions(secret string) HandlerFunc {
	key := sha256.Sum256([]byte(secret))

	return func(c *Context) {
		c.session = readSessionCookie(c.Request, key[:])
		c.Writer = &sessionWriter{ResponseWriter: c.Writer, c: c, key: key[:]}
		c.Next()
	}
}

// readSessionCookie decodes and verifies the session cookie, returning a
// fresh empty Session if it's absent, malformed, or fails signature
// verification (e.g. the secret rotated, or tampering) - a bad cookie never
// fails the request, it just starts a new session.
func readSessionCookie(r *http.Request, key []byte) *Session {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil || cookie.Value == "" {
		return &Session{}
	}

	i := strings.LastIndex(cookie.Value, ".")
	if i < 0 {
		return &Session{}
	}
	payload, sig := cookie.Value[:i], cookie.Value[i+1:]

	rawPayload, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return &Session{}
	}
	rawSig, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return &Session{}
	}
	if !hmac.Equal(rawSig, signPayload(rawPayload, key)) {
		return &Session{}
	}

	var data map[string]any
	if err := json.Unmarshal(rawPayload, &data); err != nil {
		return &Session{}
	}
	return &Session{data: data}
}

// writeSessionCookie signs s's data and sets it as the session cookie.
// Marshal failures (a value that can't round-trip through JSON) are
// swallowed - the session simply isn't saved for this response, which is
// preferable to failing the whole request over a session value.
func writeSessionCookie(w http.ResponseWriter, r *http.Request, s *Session, key []byte) {
	s.mu.RLock()
	payload, err := json.Marshal(s.data)
	s.mu.RUnlock()
	if err != nil {
		return
	}

	value := base64.RawURLEncoding.EncodeToString(payload) + "." +
		base64.RawURLEncoding.EncodeToString(signPayload(payload, key))

	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecure(r),
	})
}

func signPayload(payload, key []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(payload)
	return mac.Sum(nil)
}

// sessionWriter defers writing the session's Set-Cookie until the instant
// headers actually go out, so a handler can call Session().Set(...) any
// time before producing its response - including inside c.View, which
// calls WriteHeader itself - without an explicit save step.
type sessionWriter struct {
	http.ResponseWriter
	c           *Context
	key         []byte
	wroteHeader bool
}

func (w *sessionWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.wroteHeader = true
		if w.c.session != nil && w.c.session.dirty {
			writeSessionCookie(w.ResponseWriter, w.c.Request, w.c.session, w.key)
		}
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *sessionWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}
