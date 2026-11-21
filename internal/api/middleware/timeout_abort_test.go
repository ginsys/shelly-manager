package middleware

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// streamedChunk is large enough that net/http flushes the status line and the first chunk to the
// client before the handler returns, which is what makes the response started without a Flush
// (the timeout writer has none).
var streamedChunk = bytes.Repeat([]byte("z"), 8<<10)

// serveRecordingAbort serves one request on a recorder and reports whether the handler chain ended in
// the http.ErrAbortHandler panic that net/http turns into a cut connection. Any other panic is raised.
func serveRecordingAbort(next http.Handler, rr http.ResponseWriter, req *http.Request) (aborted bool) {
	defer func() {
		if rec := recover(); rec == http.ErrAbortHandler {
			aborted = true
		} else if rec != nil {
			panic(rec)
		}
	}()
	next.ServeHTTP(rr, req)
	return false
}

// stuckUntilTestEnds returns a channel that closes at the end of the test, for a handler that is still
// running when its deadline passes. A handler that returned at the deadline would race it: done and
// the deadline would both be ready, and the middleware would see a finished handler.
func stuckUntilTestEnds(t *testing.T) <-chan struct{} {
	t.Helper()
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	return release
}

// getOverTheWire sends one request to a real server around next. A client that is told the transfer
// failed returns the read error; one that is told it ended whole returns the body.
func getOverTheWire(t *testing.T, next http.Handler) (status int, body []byte, readErr error, getErr error) {
	t.Helper()
	srv := httptest.NewServer(next)
	t.Cleanup(srv.Close)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(srv.URL)
	if err != nil {
		return 0, nil, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, readErr = io.ReadAll(resp.Body)
	return resp.StatusCode, body, readErr, nil
}

// A handler that writes and then aborts with http.ErrAbortHandler, the standard way to cut a
// transfer, must reach the client as a failed transfer. The timeout layer recovers in its own
// goroutine, so it carries the abort out and raises it in the request goroutine.
func TestTimeoutMiddleware_AbortAfterWriteFailsTheTransfer(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(streamedChunk)
		panic(http.ErrAbortHandler)
	})

	status, _, readErr, getErr := getOverTheWire(t, TimeoutMiddleware(timeoutTestConfig(time.Second), nil)(handler))
	require.NoError(t, getErr)
	assert.Equal(t, http.StatusOK, status)
	assert.Error(t, readErr, "the client must see a failed transfer, not a 200 that ends whole")
}

// An abort before any byte is still an abort: the connection is cut and no answer is written.
func TestTimeoutMiddleware_AbortBeforeWriteCutsTheConnection(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	})

	_, _, _, getErr := getOverTheWire(t, TimeoutMiddleware(timeoutTestConfig(time.Second), nil)(handler))
	assert.Error(t, getErr)
}

// Any other panic after the first byte writes no status and no text into the stream, and the server
// lives: the next request is answered.
func TestTimeoutMiddleware_OrdinaryPanicAfterWriteAppendsNothing(t *testing.T) {
	var calls atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			_, _ = w.Write(streamedChunk)
			panic("boom")
		}
		_, _ = w.Write([]byte("alive"))
	})
	srv := httptest.NewServer(TimeoutMiddleware(timeoutTestConfig(time.Second), nil)(handler))
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL)
	require.NoError(t, err)
	body, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.NoError(t, readErr)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, streamedChunk, body, "nothing may be appended to a response that had started")

	resp, err = http.Get(srv.URL)
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	assert.Equal(t, "alive", string(body))
}

// A panic before any byte is still answered with the 500.
func TestTimeoutMiddleware_PanicBeforeWriteStillAnswers500(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})

	status, body, readErr, getErr := getOverTheWire(t, TimeoutMiddleware(timeoutTestConfig(time.Second), nil)(handler))
	require.NoError(t, getErr)
	require.NoError(t, readErr)
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Equal(t, "Internal Server Error\n", string(body))
}

// A deadline that expires after the response started cannot send a 408, so it cuts the transfer:
// ending the response there would read as a complete one.
func TestTimeoutMiddleware_DeadlineAfterStartAbortsTheTransfer(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(streamedChunk)
		<-stuckUntilTestEnds(t)
	})

	status, _, readErr, getErr := getOverTheWire(t, TimeoutMiddleware(timeoutTestConfig(100*time.Millisecond), nil)(handler))
	require.NoError(t, getErr)
	assert.Equal(t, http.StatusOK, status)
	assert.Error(t, readErr, "the client must see a failed transfer, not a short 200")
}

// A handler that aborts at the moment its deadline passes leaves the middleware with both the
// handler's completion and the deadline ready, and the select may take either. The abort must reach
// net/http through both. The handler cancels the request context itself and panics at once, so the
// deadline channel and done are ready together; a select that wakes on the deadline does not look
// again, so on the unrepaired code nearly every round loses the abort.
func TestTimeoutMiddleware_AbortAtTheDeadlineIsNotLost(t *testing.T) {
	const rounds = 300
	lost := 0
	for i := 0; i < rounds; i++ {
		parent, cancel := context.WithCancel(context.Background())
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("partial"))
			cancel()
			panic(http.ErrAbortHandler)
		})
		req := httptest.NewRequest("GET", "/api/v1/test", nil).WithContext(parent)
		if !serveRecordingAbort(TimeoutMiddleware(timeoutTestConfig(time.Minute), nil)(handler), httptest.NewRecorder(), req) {
			lost++
		}
		cancel()
	}
	assert.Zero(t, lost, "rounds in which the abort was swallowed, of %d", rounds)
}

// A deadline before the first byte still gets the 408 answer.
func TestTimeoutMiddleware_DeadlineBeforeStartStillAnswers408(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-stuckUntilTestEnds(t)
	})

	status, body, readErr, getErr := getOverTheWire(t, TimeoutMiddleware(timeoutTestConfig(100*time.Millisecond), nil)(handler))
	require.NoError(t, getErr)
	require.NoError(t, readErr)
	assert.Equal(t, http.StatusRequestTimeout, status)
	assertTimeoutBody(t, string(body))
}
