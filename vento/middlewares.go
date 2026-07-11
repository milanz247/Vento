package vento

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"time"
)

// Logger is a built-in middleware that times the rest of the handler
// chain and logs the request method, path, resulting status code, and
// latency once it completes, as structured fields via Log. Register it
// first so it wraps everything - including Recovery, so even a recovered
// panic still gets a timed line.
//
// If an X-Request-ID header is present on the request by the time Logger's
// deferred line runs (e.g. set by the app's own request-ID middleware
// registered ahead of Logger, or an upstream proxy), it's included as the
// request_id field so a single request can be traced through the log
// stream even when Logger itself ran before that ID was assigned.
func Logger(c *Context) {
	start := time.Now()

	c.Next()

	attrs := []any{
		"method", c.Request.Method,
		"path", c.Request.URL.Path,
		"status", c.StatusCode,
		"latency_ms", time.Since(start).Milliseconds(),
	}
	if id := c.Request.Header.Get("X-Request-ID"); id != "" {
		attrs = append(attrs, "request_id", id)
	}
	Log.Info("request", attrs...)
}

// Recovery is a built-in middleware that recovers from any panic raised
// further down the handler chain, logs it with a stack trace via Log, and
// responds with a clean 500 Internal Server Error JSON body instead of
// letting the panic crash the whole server process.
func Recovery(c *Context) {
	defer func() {
		if err := recover(); err != nil {
			Log.Error("panic recovered", "error", fmt.Sprint(err), "stack", string(debug.Stack()))
			c.Abort(http.StatusInternalServerError, "Internal Server Error")
		}
	}()

	c.Next()
}
