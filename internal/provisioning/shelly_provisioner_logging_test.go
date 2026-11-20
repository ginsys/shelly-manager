package provisioning

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
)

func TestIdentifyShellyDevice_LogsNoPasswordField(t *testing.T) {
	// The default password is today an empty string for every model, so a value match proves nothing:
	// the proof is that no log line carries a password field other than the literal "[hidden]".
	logFile := filepath.Join(t.TempDir(), "provisioner.log")
	logger, err := logging.New(logging.Config{Level: logging.LevelDebug, Format: "json", Output: logFile})
	require.NoError(t, err)
	sp := NewShellyProvisioner(logger, testMockInterface(logger))

	dev, identErr := sp.identifyShellyDevice(context.Background(), WiFiNetwork{SSID: "shelly1-AABBCC", Signal: -50})

	require.NoError(t, identErr)
	require.NotNil(t, dev)
	logged, readErr := os.ReadFile(logFile)
	require.NoError(t, readErr)

	sawCompleted := false
	for _, line := range strings.Split(strings.TrimSpace(string(logged)), "\n") {
		var rec map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &rec), line)
		if rec["msg"] == "Device identification completed" {
			sawCompleted = true
		}
		for key, val := range rec {
			if strings.Contains(strings.ToLower(key), "password") {
				assert.Equal(t, "[hidden]", val, "log line %q carries a password field", line)
			}
		}
	}
	assert.True(t, sawCompleted, "the control: the identification step logged its completion")
}
