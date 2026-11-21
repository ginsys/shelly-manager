package database

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/configuration"
)

// The desired-configuration write and its history row on each server. MySQL without clientFoundRows
// reports changed rows only, so a write that changes no column reports none: it must read as done, not as
// a lost race. The same cases run with the option, and on PostgreSQL. Each subtest skips when its server
// is not configured; the Database Provider Migration job of the CI supplies both.
func TestDesiredHistoryProviders(t *testing.T) {
	fixtures := []struct {
		name  string
		setup func(t *testing.T) providerFixture
		found bool
	}{
		{name: "postgresql", setup: setupPostgreSQLFixture},
		{name: "mysql", setup: setupMySQLFixture},
		{name: "mysql with clientFoundRows", setup: setupMySQLFixture, found: true},
	}
	for _, spec := range fixtures {
		t.Run(spec.name, func(t *testing.T) {
			fixture := spec.setup(t)
			if spec.found {
				fixture.config.DSN += "&clientFoundRows=true"
			}
			checkDesiredHistory(t, startProviderManager(t, fixture))
		})
	}
}

func checkDesiredHistory(t *testing.T, manager *Manager) {
	t.Helper()
	db := manager.GetDB()
	repo := configuration.NewGormConfigRepository(db, nil)
	device := &Device{MAC: "history:provider", IP: "192.0.2.77", Type: "switch", Name: "provider", Settings: "{}",
		LastSeen: time.Now().UTC(), TemplateIDs: "[]", Overrides: "{}", DesiredConfig: `{"a":1}`}
	require.NoError(t, manager.AddDevice(device))
	require.NoError(t, db.Model(&Device{}).Where("id = ?", device.ID).Update("config_applied", false).Error)
	read := configuration.DesiredRead{TemplateIDs: "[]", Overrides: "{}", Desired: `{"a":1}`}
	rows := func() (n int64) {
		require.NoError(t, db.Model(&configuration.DesiredConfigHistory{}).Where("device_id = ?", device.ID).Count(&n).Error)
		return n
	}

	t.Run("an unchanged pending row is done, not a miss", func(t *testing.T) {
		require.NoError(t, repo.SetDeviceDesiredConfigPending(device.ID, read, json.RawMessage(`{"a":1}`)))
		assert.Zero(t, rows())
	})

	t.Run("a stale read is a miss and writes nothing", func(t *testing.T) {
		stale := read
		stale.Overrides = `{"x":1}`
		assert.ErrorIs(t, repo.SetDeviceDesiredConfigPending(device.ID, stale, json.RawMessage(`{"a":2}`)), configuration.ErrDesiredChanged)
		assert.Zero(t, rows())
	})

	t.Run("a real change writes the row", func(t *testing.T) {
		require.NoError(t, repo.SetDeviceDesiredConfigPending(device.ID, read, json.RawMessage(`{"a":2}`)))
		assert.EqualValues(t, 1, rows())
	})

	t.Run("a deleted device is done and leaves no row", func(t *testing.T) {
		require.NoError(t, manager.DeleteDevice(device.ID))
		require.NoError(t, repo.SetDeviceDesiredConfigPending(device.ID, read, json.RawMessage(`{"a":3}`)))
		assert.Zero(t, rows())
	})

	t.Run("a long masked text round-trips", func(t *testing.T) {
		big := `{"name":"` + strings.Repeat("x", 70000) + `"}`
		require.NoError(t, configuration.RecordDesiredChange(db, 4242, "recompute", `{"a":1}`, big))
		var row configuration.DesiredConfigHistory
		require.NoError(t, db.Where("device_id = ?", 4242).Take(&row).Error)
		require.NotNil(t, row.NewDesired)
		assert.Equal(t, len(big), len(*row.NewDesired))
	})
}
