package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// ginsys/shelly-manager#707: a request whose Content-Length is over the limit answered 413 with no
// body. The rate-limit, timeout and IP-blocked answers of the same file send the envelope.
func TestRequestSizeMiddleware_AnswersTheEnvelope(t *testing.T) {
	logger, err := logging.New(logging.Config{Level: "error", Format: "text", Output: "stderr"})
	require.NoError(t, err)
	cfg := DefaultSecurityConfig()
	cfg.MaxRequestSize = 1024

	reached := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true })

	req := httptest.NewRequest("POST", "/api/v1/upload", strings.NewReader(strings.Repeat("x", 4096)))
	rr := httptest.NewRecorder()
	RequestSizeMiddleware(cfg, logger)(next).ServeHTTP(rr, req)

	require.Equal(t, http.StatusRequestEntityTooLarge, rr.Code)
	assert.False(t, reached, "the handler must not run for a body over the limit")
	assert.Contains(t, rr.Header().Get("Content-Type"), "application/json")

	var body struct {
		Success   bool             `json:"success"`
		Timestamp *json.RawMessage `json:"timestamp"`
		Error     *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body), "the 413 body is not JSON: %q", rr.Body.String())
	assert.False(t, body.Success)
	assert.NotNil(t, body.Timestamp, "the answer has no timestamp: %s", rr.Body.String())
	require.NotNil(t, body.Error, "the answer has no error: %s", rr.Body.String())
	assert.Equal(t, "REQUEST_TOO_LARGE", body.Error.Code)
	assert.NotEmpty(t, body.Error.Message)
}
