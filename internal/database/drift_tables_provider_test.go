package database

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/database/provider"
)

// The drift schedule tables belong to the Manager. drift_detection_runs.results holds a result
// document of a whole fleet, so it must be a long text column on MySQL (TEXT stops at 64 KiB) and must
// stay the text it was on PostgreSQL and SQLite: a column that GORM types as bytes would turn the
// existing rows into bytea or a blob. The proof starts from the table as the previous release made
// it, with rows in it, and reads the column type from each server's own catalog.

const (
	driftRunsLegacySQLite = `CREATE TABLE drift_detection_runs (
		id integer PRIMARY KEY AUTOINCREMENT,
		schedule_id integer NOT NULL,
		status text,
		started_at datetime,
		completed_at datetime,
		duration integer,
		results text,
		error text,
		created_at datetime)`

	driftRunsLegacyPostgres = `CREATE TABLE drift_detection_runs (
		id bigserial PRIMARY KEY,
		schedule_id bigint NOT NULL,
		status text,
		started_at timestamptz,
		completed_at timestamptz,
		duration bigint,
		results text,
		error text,
		created_at timestamptz)`

	driftRunsLegacyMySQL = "CREATE TABLE drift_detection_runs (" +
		"id bigint unsigned AUTO_INCREMENT PRIMARY KEY," +
		"schedule_id bigint unsigned NOT NULL," +
		"status longtext," +
		"started_at datetime(3)," +
		"completed_at datetime(3)," +
		"duration bigint," +
		"results text," +
		"error longtext," +
		"created_at datetime(3))"

	// A document with backslashes and quotes: a column that re-encodes text changes it.
	driftLegacyResults = `{"total":1,"results":[{"device_id":7,"path":"C:\\temp\\x","note":"a \"quoted\" word"}]}`
)

func TestDriftTablesMigrationProviders(t *testing.T) {
	fixtures := []struct {
		name     string
		setup    func(t *testing.T) providerFixture
		wantType string
		// columnType reads the type of drift_detection_runs.results from the server's own catalog.
		columnType func(t *testing.T, db *gorm.DB) string
	}{
		{name: "sqlite", setup: setupSQLiteDriftFixture, wantType: "text", columnType: sqliteResultsType},
		{name: "postgresql", setup: setupPostgreSQLFixture, wantType: "text", columnType: postgresResultsType},
		{name: "mysql", setup: setupMySQLFixture, wantType: "longtext", columnType: mysqlResultsType},
	}
	for _, spec := range fixtures {
		t.Run(spec.name, func(t *testing.T) {
			fixture := spec.setup(t) // skips this provider when its server is not configured
			legacyDDL := map[string]string{
				"sqlite": driftRunsLegacySQLite, "postgresql": driftRunsLegacyPostgres, "mysql": driftRunsLegacyMySQL,
			}[spec.name]

			raw := openProviderDB(t, fixture)
			require.NoError(t, raw.Exec(legacyDDL).Error)
			require.NoError(t, raw.Exec(
				"INSERT INTO drift_detection_runs (schedule_id, status, results) VALUES (?, ?, ?)",
				1, "completed", driftLegacyResults).Error)
			require.NoError(t, raw.Exec(
				"INSERT INTO drift_detection_runs (schedule_id, status) VALUES (?, ?)", 1, "running").Error)

			check := func(t *testing.T, manager *Manager) {
				t.Helper()
				db := manager.GetDB()

				var stored string
				require.NoError(t, db.Raw(
					"SELECT results FROM drift_detection_runs WHERE status = 'completed'").Scan(&stored).Error)
				assert.Equal(t, driftLegacyResults, stored, "an existing result was changed by the migration")

				var runs []configuration.DriftDetectionRun
				require.NoError(t, db.Order("id").Find(&runs).Error)
				require.Len(t, runs, 2, "an existing row was lost")
				assert.JSONEq(t, driftLegacyResults, string(runs[0].Results))
				assert.Empty(t, runs[1].Results, "a running row has no result")

				assert.Equal(t, spec.wantType, spec.columnType(t, db), "type of drift_detection_runs.results")
			}

			manager := startProviderManager(t, fixture)
			check(t, manager)

			t.Run("a result of 200 KiB round-trips", func(t *testing.T) {
				big := `{"pad":"` + strings.Repeat("x", 200<<10) + `"}`
				// The scheduler always sets StartedAt; MySQL refuses the zero time (error 1292).
				run := configuration.DriftDetectionRun{ScheduleID: 1, Status: "completed", StartedAt: time.Now().UTC(), Results: []byte(big)}
				require.NoError(t, manager.GetDB().Create(&run).Error)
				var back configuration.DriftDetectionRun
				require.NoError(t, manager.GetDB().First(&back, run.ID).Error)
				assert.Equal(t, len(big), len(back.Results))
				require.NoError(t, manager.GetDB().Delete(&back).Error)
			})

			t.Run("a device is listed once per schedule", func(t *testing.T) {
				db := manager.GetDB()
				require.NoError(t, db.Create(&configuration.DriftScheduleDevice{ScheduleID: 1, DeviceID: 2}).Error)
				require.NoError(t, db.Create(&configuration.DriftScheduleDevice{ScheduleID: 1, DeviceID: 3}).Error)
				require.NoError(t, db.Create(&configuration.DriftScheduleDevice{ScheduleID: 2, DeviceID: 2}).Error)
				assert.Error(t, db.Create(&configuration.DriftScheduleDevice{ScheduleID: 1, DeviceID: 2}).Error,
					"the same device twice in one schedule")
			})

			t.Run("a restart over the populated tables changes nothing", func(t *testing.T) {
				reopened := startProviderManager(t, fixture)
				check(t, reopened)
				var devices int64
				require.NoError(t, reopened.GetDB().Model(&configuration.DriftScheduleDevice{}).Count(&devices).Error)
				assert.Equal(t, int64(3), devices)
			})
		})
	}
}

// setupSQLiteDriftFixture is a file database, so a second manager reopens the same rows.
func setupSQLiteDriftFixture(t *testing.T) providerFixture {
	t.Helper()
	return providerFixture{
		name: "sqlite",
		config: provider.DatabaseConfig{
			Provider: "sqlite",
			DSN:      filepath.Join(t.TempDir(), "drift.db"),
			LogLevel: "error",
		},
	}
}

func sqliteResultsType(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var typ string
	require.NoError(t, db.Raw(
		"SELECT type FROM pragma_table_info('drift_detection_runs') WHERE name = 'results'").Scan(&typ).Error)
	return strings.ToLower(typ)
}

func postgresResultsType(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var typ string
	require.NoError(t, db.Raw(
		"SELECT data_type FROM information_schema.columns WHERE table_name = 'drift_detection_runs' AND column_name = 'results'").Scan(&typ).Error)
	return strings.ToLower(typ)
}

func mysqlResultsType(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var typ string
	require.NoError(t, db.Raw(
		"SELECT data_type FROM information_schema.columns "+
			"WHERE table_schema = DATABASE() AND table_name = 'drift_detection_runs' AND column_name = 'results'").Scan(&typ).Error)
	return strings.ToLower(typ)
}
