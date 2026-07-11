package vento

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestAfterResponseRuns(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users/7", nil)
	c := &Context{index: -1}
	c.Reset(rec, req)
	c.params = map[string]string{"id": "7"}

	var wg sync.WaitGroup
	wg.Add(1)
	var gotCanceled bool

	c.AfterResponse(func(ctx context.Context) {
		defer wg.Done()
		gotCanceled = ctx.Err() != nil
	})

	if !waitTimeout(&wg, time.Second) {
		t.Fatal("expected AfterResponse's function to run within 1s")
	}
	if gotCanceled {
		t.Fatal("expected the background context to not be canceled")
	}
}

func TestAfterResponsePanicIsRecovered(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := &Context{index: -1}
	c.Reset(rec, req)

	var wg sync.WaitGroup
	wg.Add(1)

	c.AfterResponse(func(ctx context.Context) {
		defer wg.Done()
		panic("boom")
	})

	if !waitTimeout(&wg, time.Second) {
		t.Fatal("expected the panicking function to still run (and be recovered) within 1s")
	}
	// If the panic weren't recovered, this test binary would crash before
	// reaching this line at all - the sole assertion is "we got here."
}

func TestAfterResponseDoesNotBlockCaller(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := &Context{index: -1}
	c.Reset(rec, req)

	block := make(chan struct{})
	start := time.Now()
	c.AfterResponse(func(ctx context.Context) {
		<-block // would hang the calling goroutine if AfterResponse were synchronous
	})
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("expected AfterResponse to return immediately, took %s", elapsed)
	}
	close(block)
}

func waitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}
