package configuration

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// device_configs.sync_status holds import and restore state. A drift check reads the device and
// stores nothing (ginsys/shelly-manager#606), so the status route must not word a stored value as
// drift (ginsys/shelly-manager#635).
func TestGetImportStatus_WordsTheStoredImportState(t *testing.T) {
	cases := []struct {
		syncStatus string
		message    string
	}{
		{"synced", "Configuration successfully imported and synced"},
		{"pending", "Configuration imported but pending sync to device"},
		{"error", "Error occurred during last import/sync operation"},
		{"something-else", "Unknown status: something-else"},
	}
	for _, tc := range cases {
		t.Run(tc.syncStatus, func(t *testing.T) {
			service, db := setupTestService(t)
			createTestDevice(t, db, 1, "Test Device", "SHSW-1")
			require.NoError(t, db.Create(&DeviceConfig{
				DeviceID: 1, Config: json.RawMessage(`{"name":"x"}`), SyncStatus: tc.syncStatus,
			}).Error)

			status, err := service.GetImportStatus(1)

			require.NoError(t, err)
			assert.Equal(t, tc.syncStatus, status.Status)
			assert.Equal(t, tc.message, status.Message)
		})
	}
}

func TestGetImportStatus_NoRow(t *testing.T) {
	service, db := setupTestService(t)
	createTestDevice(t, db, 1, "Test Device", "SHSW-1")

	status, err := service.GetImportStatus(1)

	require.NoError(t, err)
	assert.Equal(t, "not_imported", status.Status)
}

// A row marked drift was written before a drift check stopped storing its result. It is not
// evidence of drift now: the message says so and names the check that is.
func TestGetImportStatus_ALegacyDriftRowIsNotReportedAsDrift(t *testing.T) {
	service, db := setupTestService(t)
	createTestDevice(t, db, 1, "Test Device", "SHSW-1")
	require.NoError(t, db.Create(&DeviceConfig{
		DeviceID: 1, Config: json.RawMessage(`{"name":"x"}`), SyncStatus: "drift",
	}).Error)

	status, err := service.GetImportStatus(1)

	require.NoError(t, err)
	assert.NotContains(t, status.Message, "drift detected")
	assert.Contains(t, status.Message, "no longer stored here")
	assert.Contains(t, status.Message, "/config/drift")
}
