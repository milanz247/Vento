package vento

import (
	"context"
	"fmt"
)

// backgroundWorkers bounds how many goroutines AfterResponse can have in
// flight at once, so a traffic spike can't turn "send an email after every
// signup" into unbounded goroutine growth - the same class of safety cap
// as MaxConnections. A slot is held for the duration of the background
// work, not just its scheduling, so this is a genuine concurrency limit,
// not just a submission-rate limit. Work submitted beyond the cap blocks
// the calling goroutine briefly (backpressure) rather than being dropped -
// unlike a log line, background work submitted via AfterResponse is
// usually not safe to silently lose.
var backgroundWorkers = make(chan struct{}, 256)

// AfterResponse safely runs fn in a background goroutine once the current
// handler returns, for best-effort side effects that shouldn't block the
// response: sending a welcome email, pinging an analytics endpoint,
// warming a cache entry.
//
//	func UserCreate(c *vento.Context) {
//	    // ...
//	    c.AfterResponse(func(ctx context.Context) { mailer.SendWelcome(ctx, user.Email) })
//	    c.Created(user)
//	}
//
// fn receives a context derived from the request's via Detach - it carries
// the request's values but not its cancellation, so it isn't canceled the
// instant the response is written; background work runs to completion
// instead of being cut off because the client already got its response. A
// panic inside fn is recovered and logged, exactly like Recovery does for
// the main handler chain - one failing background task must not crash the
// process.
//
// This is best-effort, not a durable job queue: a process crash between
// "response sent" and "goroutine runs" loses the work silently. Fine for
// side effects that can be missed occasionally; wrong for anything that
// must survive a restart or needs retries - reach for a real job queue
// (e.g. asynq, river) for those instead.
func (c *Context) AfterResponse(fn func(ctx context.Context)) {
	bg := c.Detach()
	go func() {
		backgroundWorkers <- struct{}{}
		defer func() { <-backgroundWorkers }()
		defer func() {
			if r := recover(); r != nil {
				Log.Error("panic in AfterResponse", "panic", fmt.Sprint(r))
			}
		}()
		fn(bg.Context())
	}()
}
