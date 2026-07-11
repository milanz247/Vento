package support

import "strings"

// NormalizePrefix reduces a route-group prefix to a single leading slash
// with no trailing slash, so callers don't have to be careful about
// slashes: "", "/", "api", "/api/", and "//api//" all normalize to ""  or
// "/api".
func NormalizePrefix(prefix string) string {
	trimmed := strings.Trim(prefix, "/")
	if trimmed == "" {
		return ""
	}
	return "/" + trimmed
}

// JoinPath appends a route path to a (already-normalized) group prefix,
// tolerating a route path given with or without a leading slash, and an
// empty path (the group's own root).
func JoinPath(prefix, path string) string {
	if path == "" {
		if prefix == "" {
			return "/"
		}
		return prefix
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return prefix + path
}

// JoinChains returns a fresh slice of a followed by b, never aliasing or
// mutating either input's backing array - so a parent's handler chain (a
// route-group's middleware, an engine's global middleware) can be safely
// reused as the base for many children without one child's appended
// middleware leaking into a sibling.
func JoinChains[T any](a, b []T) []T {
	out := make([]T, 0, len(a)+len(b))
	out = append(out, a...)
	out = append(out, b...)
	return out
}
