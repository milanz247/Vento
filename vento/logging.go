package vento

import (
	"io"
	"log/slog"
	"os"
)

// asyncWriter decouples writing a log line from the goroutine that produced
// it: Write hands the bytes to a buffered channel and returns immediately,
// while a single background goroutine drains the channel and performs the
// actual (blocking) write to out. Without this, every request pays for a
// synchronous stdout syscall - and, via the standard log package's shared
// mutex, serializes against every other concurrently logging goroutine -
// directly in the hot path.
//
// If the buffer fills (the writer goroutine can't keep up with log volume),
// Write drops the line rather than blocking the request goroutine that
// produced it: a lost log line under extreme load is preferable to request
// latency scaling with logging throughput.
type asyncWriter struct {
	ch chan []byte
}

func newAsyncWriter(out io.Writer, bufSize int) *asyncWriter {
	w := &asyncWriter{ch: make(chan []byte, bufSize)}
	go func() {
		for b := range w.ch {
			out.Write(b)
		}
	}()
	return w
}

func (w *asyncWriter) Write(p []byte) (int, error) {
	b := make([]byte, len(p))
	copy(b, p)
	select {
	case w.ch <- b:
	default:
		// Buffer full: drop this line rather than block the caller.
	}
	return len(p), nil
}

// Log is Vento's structured logger, used by Logger and Recovery for
// request/panic logging. It defaults to JSON on stdout via a non-blocking
// writer (see asyncWriter); replace it before Run to change format,
// destination, or level - e.g. to send logs to a file, add attributes
// every line should carry, or lower the level in development:
//
//	vento.Log = slog.New(slog.NewTextHandler(os.Stderr, nil))
var Log = slog.New(slog.NewJSONHandler(newAsyncWriter(os.Stdout, 4096), nil))
