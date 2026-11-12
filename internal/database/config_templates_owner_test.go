package database

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/configuration"
)

// The config_templates table has one owner, configuration.ConfigTemplate, and one
// AutoMigrate site, the Manager's migration. These tests start the real startup
// path on the table shapes that exist on disk and prove it keeps every row and
// rewrites nothing it does not have to.

// templatesShapeB is the table an older release made from the 8-column model:
// the three extra columns of the wider model were never added.
const templatesShapeB = "CREATE TABLE `config_templates` (`id` integer PRIMARY KEY AUTOINCREMENT," +
	"`name` text NOT NULL,`description` text,`scope` text NOT NULL,`device_type` text," +
	"`config` text NOT NULL,`created_at` datetime,`updated_at` datetime)"

// templatesShapeC is the table that three AutoMigrate passes left on a current
// install, read from a fresh database made by the pre-ginsys/shelly-manager#280
// code: the 8 columns, then the three extras added in place. This is the shape on
// the operator's live database.
const templatesShapeC = "CREATE TABLE `config_templates` (`id` integer PRIMARY KEY AUTOINCREMENT," +
	"`name` text NOT NULL,`description` text,`scope` text NOT NULL,`device_type` text," +
	"`config` text NOT NULL,`created_at` datetime,`updated_at` datetime, " +
	"`generation` integer, `variables` text, `is_default` numeric)"

var templateIndexDDL = []string{
	"CREATE UNIQUE INDEX `idx_config_templates_name` ON `config_templates`(`name`)",
	"CREATE INDEX `idx_config_templates_scope` ON `config_templates`(`scope`)",
	"CREATE INDEX `idx_config_templates_device_type` ON `config_templates`(`device_type`)",
	// An index the operator added. GORM does not know it, so a table rebuild drops it.
	"CREATE INDEX `operator_templates_created_at` ON `config_templates`(`created_at`)",
}

// writeTemplatesDB builds a database file with the given table shape, the model's
// indexes, the operator's extra index and the given rows.
func writeTemplatesDB(t *testing.T, ddl string, inserts ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "templates.db")
	db := openRawSQLite(t, path)
	require.NoError(t, db.Exec(ddl).Error)
	for _, index := range templateIndexDDL {
		require.NoError(t, db.Exec(index).Error)
	}
	for _, insert := range inserts {
		require.NoError(t, db.Exec(insert).Error)
	}
	closeRawSQLite(t, db)
	return path
}

// templatesSchema is every schema statement the database holds for the table,
// the table's own and its indexes', in a stable order. Any ALTER or rebuild
// changes it.
func templatesSchema(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var rows []string
	require.NoError(t, db.Raw(
		"SELECT name || ' | ' || COALESCE(sql, '') FROM sqlite_master "+
			"WHERE tbl_name = 'config_templates' ORDER BY name").Scan(&rows).Error)
	require.NotEmpty(t, rows, "config_templates has no schema at all")
	return strings.Join(rows, "\n")
}

func schemaVersion(t *testing.T, path string) int {
	t.Helper()
	db := openRawSQLite(t, path)
	var version int
	require.NoError(t, db.Raw("PRAGMA schema_version").Scan(&version).Error)
	closeRawSQLite(t, db)
	return version
}

// TestConfigTemplatesOwner_ManagerAloneCreatesTheWholeTable: the Manager is the one
// migration site, so a fresh database gets every column of the owner from it alone,
// with no configuration service started.
func TestConfigTemplatesOwner_ManagerAloneCreatesTheWholeTable(t *testing.T) {
	manager := mustStartManager(t, filepath.Join(t.TempDir(), "fresh.db"))

	assert.ElementsMatch(t,
		[]string{"id", "name", "description", "scope", "device_type", "config",
			"generation", "variables", "is_default", "created_at", "updated_at"},
		columnNames(t, manager.GetDB()))
	assert.True(t, columnNotNull(t, manager.GetDB(), "scope"))
	assert.True(t, columnNotNull(t, manager.GetDB(), "config"))
	assertMigratedSchema(t, manager)
}

// TestConfigTemplatesOwner_ShapeBGainsTheExtrasAndKeepsRows: a table made by the
// 8-column model gains the three extra columns in place. Nothing is added NOT NULL,
// so no row is touched, and an old row reads generation 0, no variables, not default.
func TestConfigTemplatesOwner_ShapeBGainsTheExtrasAndKeepsRows(t *testing.T) {
	path := writeTemplatesDB(t, templatesShapeB,
		"INSERT INTO config_templates (name, description, scope, device_type, config) "+
			"VALUES ('fleet', 'all devices', 'global', 'all', CAST('{\"a\":1}' AS BLOB))",
		"INSERT INTO config_templates (name, scope, device_type, config) "+
			"VALUES ('plug', 'device_type', 'SHPLG-S', CAST('{\"b\":2}' AS BLOB))")

	manager := mustStartManager(t, path)
	db := manager.GetDB()

	assert.Subset(t, columnNames(t, db), []string{"generation", "variables", "is_default"})
	assert.False(t, columnNotNull(t, db, "generation"), "an added column must not be NOT NULL")
	assert.False(t, columnNotNull(t, db, "variables"), "an added column must not be NOT NULL")
	assert.False(t, columnNotNull(t, db, "is_default"), "an added column must not be NOT NULL")
	assert.Contains(t, indexNames(t, db), "operator_templates_created_at",
		"adding columns must not rebuild the table")

	var templates []configuration.ConfigTemplate
	require.NoError(t, db.Order("id").Find(&templates).Error)
	require.Len(t, templates, 2)
	assert.Equal(t, "fleet", templates[0].Name)
	assert.Equal(t, "all devices", templates[0].Description)
	assert.Equal(t, "global", templates[0].Scope)
	assert.Equal(t, "all", templates[0].DeviceType)
	assert.JSONEq(t, `{"a":1}`, string(templates[0].Config))
	assert.Equal(t, "plug", templates[1].Name)
	assert.Equal(t, "SHPLG-S", templates[1].DeviceType)
	for _, template := range templates {
		assert.Zero(t, template.Generation, "%s: an old row reads generation 0", template.Name)
		assert.Empty(t, template.Variables, "%s: an old row has no variables", template.Name)
		assert.False(t, template.IsDefault, "%s: an old row is not a default", template.Name)
	}
}

// TestConfigTemplatesOwner_ShapeCIssuesNoDDL: on the shape the live database has,
// the startup changes nothing in the table's schema (no ALTER, no rebuild: the
// operator's index survives), keeps every value, and a second startup issues no
// schema statement at all.
func TestConfigTemplatesOwner_ShapeCIssuesNoDDL(t *testing.T) {
	path := writeTemplatesDB(t, templatesShapeC,
		"INSERT INTO config_templates (name, description, scope, device_type, generation, config, variables, is_default) "+
			"VALUES ('rich', 'with extras', 'device_type', 'SHSW-1', 2, CAST('{\"a\":1}' AS BLOB), CAST('{\"ssid\":\"x\"}' AS BLOB), 1)",
		"INSERT INTO config_templates (name, scope, device_type, config) "+
			"VALUES ('plain', 'global', 'all', CAST('{}' AS BLOB))")

	raw := openRawSQLite(t, path)
	before := templatesSchema(t, raw)
	closeRawSQLite(t, raw)

	first := mustStartManager(t, path)
	assert.Equal(t, before, templatesSchema(t, first.GetDB()),
		"the first startup must not ALTER or rebuild config_templates")

	var rich configuration.ConfigTemplate
	require.NoError(t, first.GetDB().Where("name = ?", "rich").First(&rich).Error)
	assert.Equal(t, 2, rich.Generation)
	assert.True(t, rich.IsDefault)
	assert.JSONEq(t, `{"ssid":"x"}`, string(rich.Variables))
	assert.Equal(t, "with extras", rich.Description)
	assert.Contains(t, indexNames(t, first.GetDB()), "operator_templates_created_at")

	var count int64
	require.NoError(t, first.GetDB().Model(&configuration.ConfigTemplate{}).Count(&count).Error)
	assert.EqualValues(t, 2, count)
	require.NoError(t, first.Close())

	// The first startup may create other tables. From here the database is settled,
	// and a startup must not touch its schema at all.
	settled := schemaVersion(t, path)
	second := mustStartManager(t, path)
	require.NoError(t, second.Close())
	assert.Equal(t, settled, schemaVersion(t, path),
		"a startup on a settled database must issue no schema statement")
}
