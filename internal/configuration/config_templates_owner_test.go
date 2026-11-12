package configuration

import (
	"encoding/json"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// config_templates is migrated in one place, the database Manager's migration, which
// runs the legacy fixups first and stops startup when it fails. The service and the
// repository never migrate it, so these two tests fail if either grows its own
// AutoMigrate for the table again.

func bareDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	return db
}

func TestNewService_DoesNotMigrateConfigTemplates(t *testing.T) {
	db := bareDB(t)
	logger, err := logging.New(logging.Config{Level: "error", Format: "text"})
	require.NoError(t, err)

	NewService(db, logger)

	assert.False(t, db.Migrator().HasTable("config_templates"),
		"NewService must not migrate config_templates; the database Manager owns that")
	assert.True(t, db.Migrator().HasTable("device_configs"), "the service still migrates its own tables")
}

func TestNewGormConfigRepository_MigratesNothing(t *testing.T) {
	db := bareDB(t)

	NewGormConfigRepository(db, nil)

	assert.False(t, db.Migrator().HasTable("config_templates"),
		"the repository must not migrate config_templates; the database Manager owns that")
	assert.False(t, db.Migrator().HasTable("device_tags"),
		"the repository must not migrate device_tags; the database Manager owns that")
}

// TestRepositoryUpdateKeepsTheExtraColumns: the repository sees only the eight
// columns of the shared view, so saving through it must leave generation,
// variables and is_default as they were.
func TestRepositoryUpdateKeepsTheExtraColumns(t *testing.T) {
	db := bareDB(t)
	require.NoError(t, db.AutoMigrate(&ConfigTemplate{}))
	repo := NewGormConfigRepositoryOn(db, nil)

	rich := ConfigTemplate{
		Name: "rich", Scope: "device_type", DeviceType: "SHSW-1", Generation: 2,
		Config: json.RawMessage(`{"a":1}`), Variables: json.RawMessage(`{"ssid":"x"}`), IsDefault: true,
	}
	require.NoError(t, db.Create(&rich).Error)

	require.NoError(t, repo.UpdateTemplate(&ServiceConfigTemplate{
		ID: rich.ID, Name: "rich", Description: "edited", Scope: "device_type", DeviceType: "SHSW-1",
		Config: json.RawMessage(`{"a":2}`),
	}))

	var got ConfigTemplate
	require.NoError(t, db.First(&got, rich.ID).Error)
	assert.Equal(t, "edited", got.Description)
	assert.JSONEq(t, `{"a":2}`, string(got.Config))
	assert.Equal(t, 2, got.Generation, "generation must survive a repository update")
	assert.JSONEq(t, `{"ssid":"x"}`, string(got.Variables), "variables must survive a repository update")
	assert.True(t, got.IsDefault, "is_default must survive a repository update")
}
