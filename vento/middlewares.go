package vento

import (
	"log"
	"net/http"
	"runtime/debug"
	"time"
)

// Logger is a built-in middleware that times the rest of the handler
// chain and logs the request method, path, resulting status code, and
// latency once it completes. Register it first so it wraps everything -
// including Recovery, so even a recovered panic still gets a timed line.
func Logger(c *Context) {
	start := time.Now()

	c.Next()

	latency := time.Since(start)
	log.Printf("[vento] %s %s %d %s", c.Request.Method, c.Request.URL.Path, c.StatusCode, latency)
}

// Recovery is a built-in middleware that recovers from any panic raised
// further down the handler chain, logs the stack trace, and responds with
// a clean 500 Internal Server Error JSON body instead of letting the
// panic crash the whole server process.
func Recovery(c *Context) {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("[vento] panic recovered: %v\n%s", err, debug.Stack())
			c.Abort(http.StatusInternalServerError, "Internal Server Error")
		}
	}()

	c.Next()
}
