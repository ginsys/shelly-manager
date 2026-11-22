package provisioning

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// A pin, not new behaviour (ginsys/shelly-manager#638, slice 3, check 1): an agent with no key registers as it
// always did. The body holds no public_key and no signature, the id is the one it was given, and the token it
// gets is kept. It uses nothing from the keyed code, so it also runs on the code before that code existed.
func TestKeylessRegister_SendsNoKeyAndNoSignature(t *testing.T) {
	var sent map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&sent))
		writeEnvelope(t, w, http.StatusCreated, map[string]any{"agent_id": "provisioner-h-1", "agent_token": "t", "status": "registered"})
	}))
	defer srv.Close()
	logger, err := logging.New(logging.Config{Level: "error", Format: "text", Output: "stdout"})
	require.NoError(t, err)
	client := NewAPIClient(srv.URL, "k", "provisioner-h-1", logger)

	require.NoError(t, client.RegisterAgent("host", []string{"wifi-provisioning"}, nil))

	assert.NotContains(t, sent, "public_key")
	assert.NotContains(t, sent, "signature")
	assert.Equal(t, "provisioner-h-1", sent["id"])
	assert.True(t, client.IsRegistered())
}
