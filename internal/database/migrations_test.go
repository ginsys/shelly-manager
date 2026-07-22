package database

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/logging"
)

// legacyConfigTemplatesDDL is the schema of a database created before the scope
// column existed, copied verbatim from the report in issue #275. Note what it
// lacks: scope, and any NOT NULL on config.
const legacyConfigTemplatesDDL = "CREATE TABLE `config_templates` (" +
	"`id` integer PRIMARY KEY AUTOINCREMENT, `name` text NOT NULL, `description` text, " +
	"`device_type` text, `generation` integer, `config` text, `variables` text, " +
	"`is_default` numeric, `created_at` datetime, `updated_at` datetime)"

// legacyTemplate is one row in a legacy fixture. Pointer fields distinguish
// "column absent/NULL" from "empty string", which is the whole point of several
// of these cases.
type legacyTemplate struct {
	name       string
	deviceType *string
	config     *string
	scope      *string // only used by fixtures that already have a scope column
	isDefault  int
}

func strPtr(s string) *string { return &s }

func testLogger(t *testing.T) *logging.Logger {
	t.Helper()
	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)
	return logger
}

// openRawSQLite opens the database file directly, bypassing Manager, so tests
// can inspect or build a schema without triggering any migration.
func openRawSQLite(t *testing.T, path string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	require.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

// writeLegacyDB creates a pre-scope database at a fresh path and returns it.
func writeLegacyDB(t *testing.T, templates ...legacyTemplate) string {
	t.Helper()
	return writeLegacyDBWithDDL(t, legacyConfigTemplatesDDL, templates...)
}

func writeLegacyDBWithDDL(t *testing.T, ddl string, templates ...legacyTemplate) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "legacy.db")
	db := openRawSQLite(t, path)
	require.NoError(t, db.Exec(ddl).Error)

	hasScopeColumn := strings.Contains(ddl, "`scope`")
	for _, tmpl := range templates {
		columns := []string{"name", "device_type", "config", "is_default"}
		values := []any{tmpl.name, tmpl.deviceType, tmpl.config, tmpl.isDefault}
		if hasScopeColumn {
			columns = append(columns, "scope")
			values = append(values, tmpl.scope)
		}

		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(columns)), ",")
		stmt := fmt.Sprintf("INSERT INTO config_templates (%s) VALUES (%s)",
			strings.Join(columns, ","), placeholders)
		require.NoError(t, db.Exec(stmt, values...).Error)
	}

	closeRawSQLite(t, db)
	return path
}

func closeRawSQLite(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
}

// startManager runs the full production startup path — pre-migration fixups
// followed by AutoMigrate — against an existing database file.
func startManager(t *testing.T, path string) (*Manager, error) {
	t.Helper()
	manager, err := NewManagerFromPathWithLogger(path, testLogger(t))
	if manager != nil {
		t.Cleanup(func() { _ = manager.Close() })
	}
	return manager, err
}

func mustStartManager(t *testing.T, path string) *Manager {
	t.Helper()
	manager, err := startManager(t, path)
	require.NoError(t, err)
	require.NotNil(t, manager)
	return manager
}

type templateState struct {
	ID         uint
	Name       string
	Scope      *string
	DeviceType *string
	Config     *string
}

func readTemplates(t *testing.T, db *gorm.DB) []templateState {
	t.Helper()
	var rows []templateState
	require.NoError(t, db.Raw(
		"SELECT id, name, scope, device_type, config FROM config_templates ORDER BY id").
		Scan(&rows).Error)
	return rows
}

// readTemplatesLegacy reads a database that may still lack the scope column.
func readTemplatesLegacy(t *testing.T, db *gorm.DB) []templateState {
	t.Helper()
	var rows []templateState
	require.NoError(t, db.Raw(
		"SELECT id, name, NULL AS scope, device_type, config FROM config_templates ORDER BY id").
		Scan(&rows).Error)
	return rows
}

func columnNames(t *testing.T, db *gorm.DB) []string {
	t.Helper()
	var info []struct {
		Name string
	}
	require.NoError(t, db.Raw("PRAGMA table_info(config_templates)").Scan(&info).Error)

	names := make([]string, 0, len(info))
	for _, column := range info {
		names = append(names, column.Name)
	}
	return names
}

// columnNotNull reads the constraint straight out of SQLite rather than trusting
// the driver's view of it.
func columnNotNull(t *testing.T, db *gorm.DB, column string) bool {
	t.Helper()
	var info []struct {
		Name    string
		Notnull int
	}
	require.NoError(t, db.Raw("PRAGMA table_info(config_templates)").Scan(&info).Error)

	for _, c := range info {
		if strings.EqualFold(c.Name, column) {
			return c.Notnull == 1
		}
	}
	t.Fatalf("column %q not found", column)
	return false
}

func indexNames(t *testing.T, db *gorm.DB) []string {
	t.Helper()
	var names []string
	require.NoError(t, db.Raw(
		"SELECT name FROM sqlite_master WHERE type='index' AND tbl_name='config_templates' AND name IS NOT NULL").
		Scan(&names).Error)
	return names
}

// assertMigratedSchema checks the end state a legacy database must reach: both
// constraints enforced, every model-declared index restored, and the table still
// usable for inserts.
func assertMigratedSchema(t *testing.T, manager *Manager) {
	t.Helper()
	db := manager.GetDB()

	assert.True(t, columnNotNull(t, db, "scope"), "scope must be NOT NULL after migration")
	assert.True(t, columnNotNull(t, db, "config"), "config must be NOT NULL after migration")

	// The driver must also be able to report it — an unknown answer would let
	// the fixup skip work it should have done.
	for _, column := range []string{"scope", "config"} {
		nullable, ok, err := configTemplateColumnNullable(db, column)
		require.NoError(t, err)
		assert.True(t, ok, "driver must report nullability of %s", column)
		assert.False(t, nullable, "%s must not be nullable", column)
	}

	// AlterColumn rebuilds the SQLite table and drops separate indexes with it;
	// these assertions are what prove the following AutoMigrate restored them.
	migrator := db.Migrator()
	assert.True(t, migrator.HasIndex(&ConfigTemplate{}, "Name"), "unique name index missing")
	assert.True(t, migrator.HasIndex(&ConfigTemplate{}, "Scope"), "scope index missing")
	assert.True(t, migrator.HasIndex(&ConfigTemplate{}, "DeviceType"), "device_type index missing")

	// A rebuilt table must still accept writes, autoincrement included.
	inserted := ConfigTemplate{Name: "post-migration-insert", Scope: "global", Config: []byte(`{}`)}
	require.NoError(t, db.Create(&inserted).Error)
	assert.NotZero(t, inserted.ID, "autoincrement must still assign ids")

	// ...and the name index must still be unique, not merely present under some
	// unexpected name. Every other column is valid so uniqueness is the only
	// possible cause of failure.
	duplicate := ConfigTemplate{Name: "post-migration-insert", Scope: "global", Config: []byte(`{}`)}
	assert.Error(t, db.Create(&duplicate).Error, "duplicate template name must be rejected")

	require.NoError(t, db.Delete(&inserted).Error)
}

// TestConfigTemplateScopeMigration_Backfill covers the mapping rule: 'all' is the
// only wildcard, matched exactly and case-sensitively, so anything else concrete
// narrows to device_type scope.
func TestConfigTemplateScopeMigration_Backfill(t *testing.T) {
	tests := []struct {
		name       string
		template   legacyTemplate
		wantScope  string
		wantGlobal int
		wantDevice int
	}{
		{
			name:       "concrete device type narrows to device_type scope",
			template:   legacyTemplate{name: "shsw1", deviceType: strPtr("SHSW-1"), config: strPtr(`{"a":1}`)},
			wantScope:  "device_type",
			wantDevice: 1,
		},
		{
			name:       "exact all becomes global",
			template:   legacyTemplate{name: "everything", deviceType: strPtr("all"), config: strPtr(`{"a":1}`)},
			wantScope:  "global",
			wantGlobal: 1,
		},
		{
			name:       "All is a concrete type, not the wildcard",
			template:   legacyTemplate{name: "cased", deviceType: strPtr("All"), config: strPtr(`{"a":1}`)},
			wantScope:  "device_type",
			wantDevice: 1,
		},
		{
			name: "is_default with a concrete type is not ambiguous",
			template: legacyTemplate{
				name: "default-shplg", deviceType: strPtr("SHPLG-S"), config: strPtr(`{"a":1}`), isDefault: 1,
			},
			wantScope:  "device_type",
			wantDevice: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeLegacyDB(t, tt.template)

			// Assert the emitted result, not just the data: it is the only
			// evidence of which phase did what.
			raw := openRawSQLite(t, path)
			result, err := fixupConfigTemplateScope(raw, testLogger(t))
			require.NoError(t, err)
			assert.True(t, result.ScopeColumnAdded)
			assert.False(t, result.ConfigColumnAdded)
			assert.Equal(t, tt.wantGlobal, result.RowsBackfilledGlobal)
			assert.Equal(t, tt.wantDevice, result.RowsBackfilledDeviceType)
			assert.True(t, result.ScopeConstraintTightened)
			assert.True(t, result.ConfigConstraintTightened)
			closeRawSQLite(t, raw)

			manager := mustStartManager(t, path)
			rows := readTemplates(t, manager.GetDB())
			require.Len(t, rows, 1)
			require.NotNil(t, rows[0].Scope)
			assert.Equal(t, tt.wantScope, *rows[0].Scope)
			assertMigratedSchema(t, manager)
		})
	}
}

// TestConfigTemplateScopeMigration_MixedRows proves a single pass classifies
// every row independently.
func TestConfigTemplateScopeMigration_MixedRows(t *testing.T) {
	path := writeLegacyDB(t,
		legacyTemplate{name: "wildcard", deviceType: strPtr("all"), config: strPtr(`{}`)},
		legacyTemplate{name: "concrete", deviceType: strPtr("SHSW-1"), config: strPtr(`{}`)},
		legacyTemplate{name: "another", deviceType: strPtr("SHPLG-S"), config: strPtr(`{}`)},
	)

	manager := mustStartManager(t, path)
	rows := readTemplates(t, manager.GetDB())
	require.Len(t, rows, 3)

	got := map[string]string{}
	for _, row := range rows {
		require.NotNil(t, row.Scope, "row %q kept a NULL scope", row.Name)
		got[row.Name] = *row.Scope
	}
	assert.Equal(t, map[string]string{
		"wildcard": "global",
		"concrete": "device_type",
		"another":  "device_type",
	}, got)

	assertMigratedSchema(t, manager)
}

// TestConfigTemplateScopeMigration_Aborts covers every row the migration refuses
// to interpret. Each case asserts the database is untouched, the provider was
// closed, and that repairing the row lets the server start.
func TestConfigTemplateScopeMigration_Aborts(t *testing.T) {
	tests := []struct {
		name        string
		ddl         string
		templates   []legacyTemplate
		wantMessage string
		repair      string
	}{
		{
			name:        "empty device type is ambiguous",
			templates:   []legacyTemplate{{name: "orphan", deviceType: strPtr(""), config: strPtr(`{}`)}},
			wantMessage: "device_type is empty",
			repair:      "UPDATE config_templates SET device_type = 'all'",
		},
		{
			name:        "null device type is ambiguous",
			templates:   []legacyTemplate{{name: "orphan", deviceType: nil, config: strPtr(`{}`)}},
			wantMessage: "device_type is empty",
			repair:      "UPDATE config_templates SET device_type = 'SHSW-1'",
		},
		{
			name:        "null config cannot be fabricated",
			templates:   []legacyTemplate{{name: "bodyless", deviceType: strPtr("all"), config: nil}},
			wantMessage: "config is NULL",
			repair:      "UPDATE config_templates SET config = '{}'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ddl := tt.ddl
			if ddl == "" {
				ddl = legacyConfigTemplatesDDL
			}
			path := writeLegacyDBWithDDL(t, ddl, tt.templates...)

			before := func() []templateState {
				db := openRawSQLite(t, path)
				defer closeRawSQLite(t, db)
				return readTemplatesLegacy(t, db)
			}()

			_, err := startManager(t, path)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantMessage)
			assert.Contains(t, err.Error(), "docs/guides/database-upgrade.md")

			// The provider was closed on failure, so the file must be
			// immediately reopenable for repair — and unchanged.
			db := openRawSQLite(t, path)
			assert.NotContains(t, columnNames(t, db), "scope", "aborted migration must not add columns")
			assert.Equal(t, before, readTemplatesLegacy(t, db), "aborted migration must not modify rows")

			require.NoError(t, db.Exec(tt.repair).Error)
			closeRawSQLite(t, db)

			manager := mustStartManager(t, path)
			assertMigratedSchema(t, manager)
		})
	}
}

// TestConfigTemplateScopeMigration_ReportsEveryOffender proves operators get the
// whole list, not the first failure.
func TestConfigTemplateScopeMigration_ReportsEveryOffender(t *testing.T) {
	path := writeLegacyDB(t,
		legacyTemplate{name: "fine", deviceType: strPtr("all"), config: strPtr(`{}`)},
		legacyTemplate{name: "ambiguous-one", deviceType: strPtr(""), config: strPtr(`{}`)},
		legacyTemplate{name: "ambiguous-two", deviceType: nil, config: strPtr(`{}`)},
		legacyTemplate{name: "bodyless", deviceType: strPtr("SHSW-1"), config: nil},
	)

	_, err := startManager(t, path)
	require.Error(t, err)
	for _, name := range []string{"ambiguous-one", "ambiguous-two", "bodyless"} {
		assert.Contains(t, err.Error(), name)
	}
	assert.NotContains(t, err.Error(), `name="fine"`, "valid rows must not be reported as offenders")

	// Mixed valid/invalid input must still leave everything untouched.
	db := openRawSQLite(t, path)
	assert.NotContains(t, columnNames(t, db), "scope")
	for _, row := range readTemplatesLegacy(t, db) {
		assert.Nil(t, row.Scope)
	}
}

// TestConfigTemplateScopeMigration_ExistingScopes covers databases that already
// have a scope column: valid values are authoritative, invalid ones abort.
func TestConfigTemplateScopeMigration_ExistingScopes(t *testing.T) {
	const ddlWithScope = "CREATE TABLE `config_templates` (" +
		"`id` integer PRIMARY KEY AUTOINCREMENT, `name` text NOT NULL, `description` text, " +
		"`scope` text, `device_type` text, `generation` integer, `config` text, `variables` text, " +
		"`is_default` numeric, `created_at` datetime, `updated_at` datetime)"

	t.Run("valid explicit scopes are never reinterpreted", func(t *testing.T) {
		path := writeLegacyDBWithDDL(t, ddlWithScope,
			// A group-scoped template with a concrete device type would be
			// rewritten to device_type if the migration second-guessed it.
			legacyTemplate{name: "grouped", scope: strPtr("group"), deviceType: strPtr("SHSW-1"), config: strPtr(`{}`)},
			legacyTemplate{name: "worldwide", scope: strPtr("global"), deviceType: strPtr("SHPLG-S"), config: strPtr(`{}`)},
		)

		manager := mustStartManager(t, path)
		rows := readTemplates(t, manager.GetDB())
		require.Len(t, rows, 2)
		assert.Equal(t, "group", *rows[0].Scope)
		assert.Equal(t, "global", *rows[1].Scope)
		assertMigratedSchema(t, manager)
	})

	t.Run("nullable scope column with holes is backfilled", func(t *testing.T) {
		path := writeLegacyDBWithDDL(t, ddlWithScope,
			legacyTemplate{name: "keeps-scope", scope: strPtr("group"), deviceType: strPtr("SHSW-1"), config: strPtr(`{}`)},
			legacyTemplate{name: "null-scope", scope: nil, deviceType: strPtr("all"), config: strPtr(`{}`)},
			legacyTemplate{name: "empty-scope", scope: strPtr(""), deviceType: strPtr("SHPLG-S"), config: strPtr(`{}`)},
		)

		raw := openRawSQLite(t, path)
		result, err := fixupConfigTemplateScope(raw, testLogger(t))
		require.NoError(t, err)
		assert.False(t, result.ScopeColumnAdded, "column already existed")
		assert.Equal(t, 1, result.RowsBackfilledGlobal)
		assert.Equal(t, 1, result.RowsBackfilledDeviceType)
		closeRawSQLite(t, raw)

		manager := mustStartManager(t, path)
		rows := readTemplates(t, manager.GetDB())
		require.Len(t, rows, 3)
		assert.Equal(t, "group", *rows[0].Scope)
		assert.Equal(t, "global", *rows[1].Scope)
		assert.Equal(t, "device_type", *rows[2].Scope)
		assertMigratedSchema(t, manager)
	})

	t.Run("invalid non-empty scope aborts", func(t *testing.T) {
		path := writeLegacyDBWithDDL(t, ddlWithScope,
			legacyTemplate{name: "garbage-scope", scope: strPtr("garbage"), deviceType: strPtr("all"), config: strPtr(`{}`)},
		)

		_, err := startManager(t, path)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `scope "garbage" is invalid`)
	})

	t.Run("device_type scope without a device type aborts", func(t *testing.T) {
		path := writeLegacyDBWithDDL(t, ddlWithScope,
			legacyTemplate{name: "unscoped", scope: strPtr("device_type"), deviceType: strPtr(""), config: strPtr(`{}`)},
		)

		_, err := startManager(t, path)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "device_type required")
	})
}

// TestConfigTemplateScopeMigration_EmptyConfigIsNotNull guards the boundary: an
// empty config is a value, not the schema defect NULL represents.
func TestConfigTemplateScopeMigration_EmptyConfigIsNotNull(t *testing.T) {
	path := writeLegacyDB(t, legacyTemplate{name: "empty-body", deviceType: strPtr("all"), config: strPtr("")})

	manager := mustStartManager(t, path)
	rows := readTemplates(t, manager.GetDB())
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].Config)
	assert.Equal(t, "", *rows[0].Config)
	assert.Equal(t, "global", *rows[0].Scope)
}

// TestConfigTemplateScopeMigration_Paths covers the schema shapes that are not
// simply "populated legacy table".
func TestConfigTemplateScopeMigration_Paths(t *testing.T) {
	t.Run("legacy table with zero rows", func(t *testing.T) {
		// Distinct from a fresh database: the table exists, so AutoMigrate would
		// still issue the ALTER that SQLite refuses.
		path := writeLegacyDB(t)

		manager := mustStartManager(t, path)
		assert.Empty(t, readTemplates(t, manager.GetDB()))
		assertMigratedSchema(t, manager)
	})

	t.Run("legacy table missing config and device_type on an empty table", func(t *testing.T) {
		const minimalDDL = "CREATE TABLE `config_templates` (" +
			"`id` integer PRIMARY KEY AUTOINCREMENT, `name` text NOT NULL, `description` text, " +
			"`created_at` datetime, `updated_at` datetime)"
		path := writeLegacyDBWithDDL(t, minimalDDL)

		raw := openRawSQLite(t, path)
		result, err := fixupConfigTemplateScope(raw, testLogger(t))
		require.NoError(t, err)
		assert.True(t, result.ScopeColumnAdded)
		assert.True(t, result.ConfigColumnAdded)
		closeRawSQLite(t, raw)

		manager := mustStartManager(t, path)
		assert.Contains(t, columnNames(t, manager.GetDB()), "device_type",
			"AutoMigrate must add the nullable device_type column")
		assertMigratedSchema(t, manager)
	})

	t.Run("missing config with rows aborts", func(t *testing.T) {
		const noConfigDDL = "CREATE TABLE `config_templates` (" +
			"`id` integer PRIMARY KEY AUTOINCREMENT, `name` text NOT NULL, `device_type` text, " +
			"`created_at` datetime, `updated_at` datetime)"
		path := filepath.Join(t.TempDir(), "legacy.db")
		db := openRawSQLite(t, path)
		require.NoError(t, db.Exec(noConfigDDL).Error)
		require.NoError(t, db.Exec("INSERT INTO config_templates (name, device_type) VALUES ('orphan','all')").Error)
		closeRawSQLite(t, db)

		_, err := startManager(t, path)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "config column is missing")
	})

	t.Run("fresh database", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "fresh.db")

		manager := mustStartManager(t, path)
		assert.Contains(t, columnNames(t, manager.GetDB()), "scope")
		assertMigratedSchema(t, manager)
	})

	t.Run("idempotent across restarts", func(t *testing.T) {
		path := writeLegacyDB(t,
			legacyTemplate{name: "wildcard", deviceType: strPtr("all"), config: strPtr(`{}`)},
			legacyTemplate{name: "concrete", deviceType: strPtr("SHSW-1"), config: strPtr(`{}`)},
		)

		first := mustStartManager(t, path)
		rowsAfterFirst := readTemplates(t, first.GetDB())
		indexesAfterFirst := indexNames(t, first.GetDB())
		require.NoError(t, first.Close())

		// A fully migrated database must produce a result with nothing set.
		raw := openRawSQLite(t, path)
		result, err := fixupConfigTemplateScope(raw, testLogger(t))
		require.NoError(t, err)
		assert.False(t, result.changedAnything(), "second run must be a no-op, got %+v", result)
		closeRawSQLite(t, raw)

		second := mustStartManager(t, path)
		assert.Equal(t, rowsAfterFirst, readTemplates(t, second.GetDB()))
		assert.ElementsMatch(t, indexesAfterFirst, indexNames(t, second.GetDB()))
		assertMigratedSchema(t, second)
	})
}

// TestConfigTemplateScopeMigration_PartialConstraintRecovery is the regression
// for a crash landing between the two independent constraint operations. No
// fault injection needed: a database where scope is already NOT NULL while
// config is not is exactly that state.
func TestConfigTemplateScopeMigration_PartialConstraintRecovery(t *testing.T) {
	const partialDDL = "CREATE TABLE `config_templates` (" +
		"`id` integer PRIMARY KEY AUTOINCREMENT, `name` text NOT NULL, `description` text, " +
		"`scope` text NOT NULL, `device_type` text, `generation` integer, `config` text, " +
		"`variables` text, `is_default` numeric, `created_at` datetime, `updated_at` datetime)"

	path := writeLegacyDBWithDDL(t, partialDDL,
		legacyTemplate{name: "already-scoped", scope: strPtr("device_type"), deviceType: strPtr("SHSW-1"), config: strPtr(`{}`)},
	)

	raw := openRawSQLite(t, path)
	require.True(t, columnNotNull(t, raw, "scope"))
	require.False(t, columnNotNull(t, raw, "config"))

	result, err := fixupConfigTemplateScope(raw, testLogger(t))
	require.NoError(t, err)
	assert.False(t, result.ScopeColumnAdded)
	assert.Zero(t, result.RowsBackfilledGlobal)
	assert.Zero(t, result.RowsBackfilledDeviceType)
	assert.False(t, result.ScopeConstraintTightened, "scope was already NOT NULL; it must not be altered again")
	assert.True(t, result.ConfigConstraintTightened, "config must be tightened to complete the migration")
	closeRawSQLite(t, raw)

	manager := mustStartManager(t, path)
	rows := readTemplates(t, manager.GetDB())
	require.Len(t, rows, 1)
	assert.Equal(t, "device_type", *rows[0].Scope)
	assert.Equal(t, "SHSW-1", *rows[0].DeviceType)
	assertMigratedSchema(t, manager)

	// And the completed state stays put.
	require.NoError(t, manager.Close())
	again := openRawSQLite(t, path)
	repeat, err := fixupConfigTemplateScope(again, testLogger(t))
	require.NoError(t, err)
	assert.False(t, repeat.changedAnything(), "a completed migration must be a no-op, got %+v", repeat)
}

// TestDeviceJSONColumnDefaults pins the behaviour that replaced the column
// DEFAULTs on the device JSON columns. MySQL forbids defaults on TEXT, so the
// seeding moved into BeforeSave — an unset field must still land as an empty
// document, never as an empty string that later fails to parse.
func TestDeviceJSONColumnDefaults(t *testing.T) {
	manager := mustStartManager(t, filepath.Join(t.TempDir(), "devices.db"))

	device := &Device{IP: "192.168.1.50", MAC: "AA:BB:CC:DD:EE:01", Type: "SHSW-1"}
	require.NoError(t, manager.AddDevice(device))

	stored, err := manager.GetDevice(device.ID)
	require.NoError(t, err)
	assert.Equal(t, "[]", stored.TemplateIDs)
	assert.Equal(t, "{}", stored.Overrides)
	assert.Equal(t, "{}", stored.DesiredConfig)

	// An explicit value is never overwritten.
	stored.TemplateIDs = "[1,2]"
	require.NoError(t, manager.UpdateDevice(stored))

	reloaded, err := manager.GetDevice(device.ID)
	require.NoError(t, err)
	assert.Equal(t, "[1,2]", reloaded.TemplateIDs)
}

// TestDevicesTableStabilizes guards the other half of that change: retagging the
// device columns makes AutoMigrate rewrite the table once, and it must then
// settle. A sentinel index GORM does not know about proves the second startup
// leaves the table alone instead of rebuilding it on every boot.
func TestDevicesTableStabilizes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "devices.db")

	first := mustStartManager(t, path)
	require.NoError(t, first.GetDB().Exec(
		"CREATE INDEX sentinel_devices_index ON devices(firmware)").Error)
	require.NoError(t, first.Close())

	second := mustStartManager(t, path)
	var names []string
	require.NoError(t, second.GetDB().Raw(
		"SELECT name FROM sqlite_master WHERE type='index' AND tbl_name='devices' AND name IS NOT NULL").
		Scan(&names).Error)
	assert.Contains(t, names, "sentinel_devices_index",
		"devices must not be rebuilt on every startup")
}

// TestConfigTemplateScopeMigration_NoRebuildOnCleanStartup proves a compliant
// startup rebuilds nothing. A sentinel index GORM knows nothing about survives a
// no-op but would be dropped by a table rebuild.
//
// The full production sequence is reproduced deliberately: the Manager is only
// the first of three AutoMigrate passes over config_templates (see #280), and
// testing the Manager alone hid a real defect — configuration.ConfigTemplate
// declared config nullable, so each boot relaxed the constraint and the next
// fixup re-tightened it, rebuilding the table twice per start.
func TestConfigTemplateScopeMigration_NoRebuildOnCleanStartup(t *testing.T) {
	path := writeLegacyDB(t, legacyTemplate{name: "wildcard", deviceType: strPtr("all"), config: strPtr(`{}`)})

	first := startFullStack(t, path)
	require.NoError(t, first.GetDB().Exec(
		"CREATE INDEX sentinel_not_known_to_gorm ON config_templates(created_at)").Error)
	require.NoError(t, first.Close())

	second := startFullStack(t, path)
	assert.Contains(t, indexNames(t, second.GetDB()), "sentinel_not_known_to_gorm",
		"an idempotent startup must not rebuild the table")

	// A third pass, to catch a constraint that oscillates rather than settles.
	require.NoError(t, second.Close())
	third := startFullStack(t, path)
	assert.Contains(t, indexNames(t, third.GetDB()), "sentinel_not_known_to_gorm",
		"the schema must settle, not flip back and forth between startups")
	assertMigratedSchema(t, third)
}

// startFullStack mirrors what the server does at boot: the Manager migrates,
// then the configuration service and its repository migrate their own views of
// the same tables.
func startFullStack(t *testing.T, path string) *Manager {
	t.Helper()

	manager := mustStartManager(t, path)
	configuration.NewService(manager.GetDB(), testLogger(t))
	return manager
}
