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

// The device_tags table has one owner, configuration.DbDeviceTag, and one AutoMigrate
// site, the Manager's migration. These tests start the real startup path on the shape
// a current install has on disk and prove it rewrites nothing.

// deviceTagsCurrent is the table both earlier models made (they carried identical
// tags), read from a database created by the startup path.
const deviceTagsCurrent = "CREATE TABLE `device_tags` (`id` integer PRIMARY KEY AUTOINCREMENT," +
	"`device_id` integer NOT NULL,`tag` text NOT NULL,`created_at` datetime)"

var deviceTagsIndexDDL = []string{
	"CREATE INDEX `idx_device_tags_device_id` ON `device_tags`(`device_id`)",
	"CREATE INDEX `idx_device_tags_tag` ON `device_tags`(`tag`)",
	// An index the operator added. GORM does not know it, so a table rebuild drops it.
	"CREATE INDEX `operator_device_tags_created_at` ON `device_tags`(`created_at`)",
}

// deviceTagsSchema is every schema statement the database holds for the table, in a
// stable order. Any ALTER or rebuild changes it.
func deviceTagsSchema(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var rows []string
	require.NoError(t, db.Raw(
		"SELECT name || ' | ' || COALESCE(sql, '') FROM sqlite_master "+
			"WHERE tbl_name = 'device_tags' ORDER BY name").Scan(&rows).Error)
	require.NotEmpty(t, rows, "device_tags has no schema at all")
	return strings.Join(rows, "\n")
}

// TestDeviceTagsOwner_ManagerAloneCreatesTheTable: the Manager is the one migration
// site, so a fresh database gets the table from it with no configuration service started.
func TestDeviceTagsOwner_ManagerAloneCreatesTheTable(t *testing.T) {
	manager := mustStartManager(t, filepath.Join(t.TempDir(), "fresh.db"))

	// The whole definition of the table and of each index, not their names: a model
	// that turns an index unique would otherwise differ on a fresh database only.
	want := []string{
		"device_tags | " + deviceTagsCurrent,
		"idx_device_tags_device_id | " + deviceTagsIndexDDL[0],
		"idx_device_tags_tag | " + deviceTagsIndexDDL[1],
	}
	assert.Equal(t, strings.Join(want, "\n"), deviceTagsSchema(t, manager.GetDB()))
}

// TestDeviceTagsOwner_CurrentShapeIssuesNoDDL: on the shape a current install has, the
// startup changes nothing in the table's schema (no ALTER, no rebuild: the operator's
// index survives), keeps every row, and a second startup issues no schema statement.
func TestDeviceTagsOwner_CurrentShapeIssuesNoDDL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tags.db")
	raw := openRawSQLite(t, path)
	require.NoError(t, raw.Exec(deviceTagsCurrent).Error)
	for _, index := range deviceTagsIndexDDL {
		require.NoError(t, raw.Exec(index).Error)
	}
	require.NoError(t, raw.Exec(
		"INSERT INTO device_tags (device_id, tag, created_at) VALUES (1, 'kitchen', '2026-01-02 03:04:05'), (2, 'kitchen', NULL)").Error)
	before := deviceTagsSchema(t, raw)
	closeRawSQLite(t, raw)

	first := mustStartManager(t, path)
	assert.Equal(t, before, deviceTagsSchema(t, first.GetDB()),
		"the first startup must not ALTER or rebuild device_tags")

	var tags []configuration.DbDeviceTag
	require.NoError(t, first.GetDB().Order("id").Find(&tags).Error)
	require.Len(t, tags, 2)
	assert.Equal(t, uint(1), tags[0].DeviceID)
	assert.Equal(t, "kitchen", tags[0].Tag)
	assert.Equal(t, uint(2), tags[1].DeviceID)
	require.NoError(t, first.Close())

	// The first startup may create other tables. From here the database is settled,
	// and a startup must not touch its schema at all.
	settled := schemaVersion(t, path)
	second := mustStartManager(t, path)
	require.NoError(t, second.Close())
	assert.Equal(t, settled, schemaVersion(t, path),
		"a startup on a settled database must issue no schema statement")
}
