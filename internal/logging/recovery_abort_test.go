package logging

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A handler that writes and then aborts with http.ErrAbortHandler must reach the client as a failed
// transfer. 8 KiB is large enough that net/http flushes the status line and the first chunk before
// the handler returns, so the response has started when the abort comes.
func TestRecoveryMiddleware_AbortAfterWriteFailsTheTransfer(t *testing.T) {
	logger, err := New(Config{Level: LevelError, Format: "text"})
	require.NoError(t, err)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte("z"), 8<<10))
		panic(http.ErrAbortHandler)
	})
	srv := httptest.NewServer(RecoveryMiddleware(logger)(handler))
	t.Cleanup(srv.Close)

	resp, err := (&http.Client{Timeout: 5 * time.Second}).Get(srv.URL)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	_, readErr := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Error(t, readErr, "the client must see a failed transfer, not a 200 that ends whole")
}
