package sync

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An export hands the saved device configuration to a plugin. A Gen1 action URL can hold a
// password or a token, so the export masks every URL of every action (ginsys/shelly-manager#381).
func TestMaskExportSecrets_MasksActionURLs(t *testing.T) {
	build := func() *ExportData {
		return &ExportData{Configurations: []ConfigurationData{{
			DeviceID: 1,
			Config: map[string]interface{}{"actions": []interface{}{
				map[string]interface{}{
					"name": "btn_on_url", "index": float64(0),
					"secret_urls": []interface{}{"http://admin:hunter2@192.168.1.4/on?token=s3cr3t", "http://a/2"},
				},
				map[string]interface{}{
					"name": "out_on_url", "index": float64(0), "enabled": true,
					"secret_urls": []interface{}{"http://x:pw2@192.168.1.4/off"},
				},
			}},
		}}}
	}
	unmasked, err := json.Marshal(build())
	require.NoError(t, err)
	for _, secret := range []string{"hunter2", "s3cr3t", "pw2"} {
		require.Contains(t, string(unmasked), secret, "the fixture must hold the secret")
	}

	data := build()
	maskExportSecrets(data, map[string]interface{}{})

	masked, err := json.Marshal(data)
	require.NoError(t, err)
	for _, secret := range []string{"hunter2", "s3cr3t", "pw2"} {
		assert.NotContains(t, string(masked), secret)
	}
	assert.Contains(t, string(masked), "btn_on_url", "the action itself is kept")
	assert.Contains(t, string(masked), RedactedValue)
}
