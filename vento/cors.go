package vento

import (
	"net/http"
	"strconv"
)

// CORS returns a middleware that adds Cross-Origin Resource Sharing headers
// for the given allowed origins, and short-circuits preflight OPTIONS
// requests with a 204. It is not part of DefaultMiddleware since it needs
// app-specific configuration (which origins to trust) - add it explicitly,
// typically only to the API table:
//
//	app.Use(vento.CORS("https://app.example.com"))
//
// Pass "*" to allow any origin; this cannot be combined with credentialed
// requests (browsers reject a wildcard Access-Control-Allow-Origin
// alongside Access-Control-Allow-Credentials), so CORS never sets the
// credentials header when "*" is configured.
func CORS(origins ...string) HandlerFunc {
	allowAll := false
	allowed := make(map[string]bool, len(origins))
	for _, o := range origins {
		if o == "*" {
			allowAll = true
			continue
		}
		allowed[o] = true
	}

	return func(c *Context) {
		origin := c.Request.Header.Get("Origin")
		if origin == "" {
			// Not a cross-origin request (or not a browser) - nothing to do.
			c.Next()
			return
		}

		switch {
		case allowAll:
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		case allowed[origin]:
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Add("Vary", "Origin")
		default:
			c.Next()
			return
		}

		if c.Request.Method != http.MethodOptions || c.Request.Header.Get("Access-Control-Request-Method") == "" {
			c.Next()
			return
		}

		// A CORS preflight: answer it directly rather than running the
		// handler chain, per the fetch/CORS spec.
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if reqHeaders := c.Request.Header.Get("Access-Control-Request-Headers"); reqHeaders != "" {
			c.Writer.Header().Set("Access-Control-Allow-Headers", reqHeaders)
		}
		c.Writer.Header().Set("Access-Control-Max-Age", strconv.Itoa(600))
		c.StatusCode = http.StatusNoContent
		c.Writer.WriteHeader(http.StatusNoContent)
	}
}
