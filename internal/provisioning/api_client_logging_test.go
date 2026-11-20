package provisioning

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

func TestAPIClient_UndecodableRegistrationAnswerLogsNoToken(t *testing.T) {
	// The registration answer carries the agent token. When its data cannot be decoded (here a bad
	// registered_at) the client logs the failure, and the answer must not go into that log.
	testutil.SkipIfNoSocketPermissions(t)
	logFile := filepath.Join(t.TempDir(), "client.log")
	logger, err := logging.New(logging.Config{Level: logging.LevelDebug, Format: "json", Output: logFile})
	require.NoError(t, err)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeEnvelope(t, w, http.StatusCreated, map[string]interface{}{
			"agent_id": "a", "agent_token": "tok-marker-1", "registered_at": "not a time",
		})
	}))
	defer server.Close()

	regErr := NewAPIClient(server.URL, "k", "a", logger).RegisterAgent("h", nil, nil)

	require.Error(t, regErr)
	assert.NotContains(t, regErr.Error(), "tok-marker-1")
	logged, readErr := os.ReadFile(logFile)
	require.NoError(t, readErr)
	require.Contains(t, string(logged), "Failed to parse JSON response", "the control: the failure was logged at all")
	assert.NotContains(t, string(logged), "tok-marker-1")
}
