package sync

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/database/provider"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// A zero last_seen in an archive is a device the archive never saw. MySQL rejects a zero datetime
// (error 1292), so the import dates a new device by the import time and keeps the stored value of
// an overwritten one. A value the archive carries is saved as before.
func checkImportZeroLastSeen(t *testing.T, db *database.Manager) {
	t.Helper()
	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)
	require.NoError(t, db.GetDB().AutoMigrate(&configuration.DeviceConfig{}))
	engine := NewSyncEngine(db, logger)
	ctx := context.Background()

	archive := func(lastSeen time.Time) *ExportData {
		return &ExportData{Devices: []DeviceData{{
			MAC: "aabbccdd0518", IP: "192.0.2.18", Type: "switch", Name: "archived", LastSeen: lastSeen,
		}}}
	}
	stored := func() time.Time {
		var device database.Device
		require.NoError(t, db.GetDB().Where("mac = ?", "aabbccdd0518").First(&device).Error)
		return device.LastSeen
	}

	// Every case starts from an empty devices table, so each can run alone.
	reset := func(t *testing.T) {
		t.Helper()
		require.NoError(t, db.GetDB().Where("1 = 1").Delete(&database.Device{}).Error)
	}
	// existing stores the device the overwrite cases import over, last seen at the given time.
	existing := func(t *testing.T, lastSeen time.Time) {
		t.Helper()
		reset(t)
		require.NoError(t, db.AddDevice(&database.Device{
			IP: "192.0.2.19", MAC: "aabbccdd0518", Type: "switch", Name: "stored", LastSeen: lastSeen,
		}))
	}
	past := func(age time.Duration) time.Time { return time.Now().UTC().Add(-age).Truncate(time.Second) }

	t.Run("a new device is dated by the import", func(t *testing.T) {
		reset(t)
		before := time.Now().Add(-time.Minute)
		_, err := engine.ApplyImport(ctx, archive(time.Time{}), ImportOptions{})
		require.NoError(t, err)
		assert.True(t, stored().After(before), "last_seen was %v", stored())
	})

	t.Run("a new device keeps the last_seen the archive carries", func(t *testing.T) {
		reset(t)
		carried := past(24 * time.Hour)
		_, err := engine.ApplyImport(ctx, archive(carried), ImportOptions{})
		require.NoError(t, err)
		assert.WithinDuration(t, carried, stored(), time.Second)
	})

	t.Run("an overwritten device keeps its stored last_seen", func(t *testing.T) {
		kept := past(48 * time.Hour)
		existing(t, kept)
		_, err := engine.ApplyImport(ctx, archive(time.Time{}), ImportOptions{ForceOverwrite: true})
		require.NoError(t, err)
		assert.WithinDuration(t, kept, stored(), time.Second)
	})

	t.Run("an overwritten device takes the last_seen the archive carries", func(t *testing.T) {
		existing(t, past(48*time.Hour))
		carried := past(24 * time.Hour)
		_, err := engine.ApplyImport(ctx, archive(carried), ImportOptions{ForceOverwrite: true})
		require.NoError(t, err)
		assert.WithinDuration(t, carried, stored(), time.Second)
	})

	// The same defect in its second place: a discovered device whose archive time is zero.
	t.Run("a discovered device without a time is dated by the import", func(t *testing.T) {
		require.NoError(t, db.GetDB().Where("1 = 1").Delete(&database.DiscoveredDevice{}).Error)
		data := &ExportData{DiscoveredDevices: []DiscoveredDeviceData{{
			MAC: "112233440518", AgentID: "agent-518", Model: "SHSW-1",
		}}}
		storedDiscovered := func() time.Time {
			var device database.DiscoveredDevice
			require.NoError(t, db.GetDB().Where("mac = ?", "112233440518").First(&device).Error)
			return device.Discovered
		}

		before := time.Now().Add(-time.Minute)
		_, err := engine.ApplyImport(ctx, data, ImportOptions{})
		require.NoError(t, err)
		assert.True(t, storedDiscovered().After(before), "created with %v", storedDiscovered())

		old := past(48 * time.Hour)
		require.NoError(t, db.GetDB().Model(&database.DiscoveredDevice{}).Where("mac = ?", "112233440518").
			UpdateColumn("discovered", old).Error)
		_, err = engine.ApplyImport(ctx, data, ImportOptions{ForceOverwrite: true})
		require.NoError(t, err)
		assert.True(t, storedDiscovered().After(before), "overwritten with %v", storedDiscovered())
	})
}

func TestImportZeroLastSeen_SQLite(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	t.Cleanup(cleanup)
	checkImportZeroLastSeen(t, db)
}

// The provider cases run in the Database Provider Migration job, where the servers exist, and skip
// where they do not.
func TestImportZeroLastSeenProviders(t *testing.T) {
	for _, name := range []string{"postgresql", "mysql"} {
		t.Run(name, func(t *testing.T) {
			checkImportZeroLastSeen(t, providerImportManager(t, name))
		})
	}
}

// providerImportManager opens a manager on a fresh database of the named server.
func providerImportManager(t *testing.T, name string) *database.Manager {
	t.Helper()
	const dbName = "shelly_import_zero_test"
	var (
		config            provider.DatabaseConfig
		driver, adminDSN  string
		host, port, user  string
		password, tlsName string
	)
	switch name {
	case "postgresql":
		host = os.Getenv("POSTGRES_TEST_HOST")
		if host == "" {
			t.Skip("PostgreSQL import case requires POSTGRES_TEST_HOST")
		}
		port, user, password = envOr("POSTGRES_TEST_PORT", "5432"), envOr("POSTGRES_TEST_USER", "postgres"), envOr("POSTGRES_TEST_PASSWORD", "postgres")
		driver = "pgx"
		adminDSN = fmt.Sprintf("postgres://%s:%s@%s:%s/postgres?sslmode=disable", user, password, host, port)
		config = provider.DatabaseConfig{
			Provider: "postgresql", DSN: fmt.Sprintf("postgres://%s:%s@%s:%s/%s", user, password, host, port, dbName),
			MaxOpenConns: 5, MaxIdleConns: 2, LogLevel: "error", Options: map[string]string{"sslmode": "disable"},
		}
	default:
		host = os.Getenv("MYSQL_TEST_HOST")
		if host == "" {
			t.Skip("MySQL import case requires MYSQL_TEST_HOST")
		}
		port, user, password = envOr("MYSQL_TEST_PORT", "3306"), envOr("MYSQL_TEST_USER", "root"), envOr("MYSQL_TEST_PASSWORD", "root")
		tlsName = "false"
		driver = "mysql"
		adminDSN = fmt.Sprintf("%s:%s@tcp(%s:%s)/", user, password, host, port)
		config = provider.DatabaseConfig{
			Provider: "mysql", DSN: fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", user, password, host, port, dbName),
			MaxOpenConns: 5, MaxIdleConns: 2, LogLevel: "error", Options: map[string]string{"tls": tlsName},
		}
	}

	admin, err := sql.Open(driver, adminDSN)
	require.NoError(t, err)
	t.Cleanup(func() { _ = admin.Close() })
	if pingErr := admin.Ping(); pingErr != nil {
		t.Skipf("%s not reachable: %v", name, pingErr)
	}
	_, _ = admin.Exec("DROP DATABASE IF EXISTS " + dbName)
	_, err = admin.Exec("CREATE DATABASE " + dbName)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = admin.Exec("DROP DATABASE IF EXISTS " + dbName) })

	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)
	manager, err := database.NewManagerWithLogger(config, logger)
	require.NoError(t, err)
	t.Cleanup(func() { _ = manager.Close() })
	return manager
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
