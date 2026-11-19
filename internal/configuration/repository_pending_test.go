package configuration

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The desired configuration and the applied flag change in one statement (ginsys/shelly-manager#716),
// so a read never sees the new configuration with the flag still true, and an apply's compare-and-set
// against the old configuration cannot mark the new one applied.
func TestSetDeviceDesiredConfigPending_WritesBothColumnsInOneStatement(t *testing.T) {
	db := bareDB(t)
	require.NoError(t, db.AutoMigrate(&DbDevice{}))
	repo := NewGormConfigRepositoryOn(db, nil)
	require.NoError(t, db.Create(&DbDevice{ID: 1, DesiredConfig: `{"a":1}`, ConfigApplied: true}).Error)

	require.NoError(t, repo.SetDeviceDesiredConfigPending(1, json.RawMessage(`{"a":2}`)))

	var row DbDevice
	require.NoError(t, db.First(&row, 1).Error)
	assert.Equal(t, `{"a":2}`, row.DesiredConfig)
	assert.False(t, row.ConfigApplied, "the flag is cleared with the write: false is a zero value a struct update would skip")

	// An apply that pushed the old configuration cannot mark the new one applied.
	set, err := repo.SetDeviceConfigAppliedIfDesired(1, `{"a":1}`)
	require.NoError(t, err)
	assert.False(t, set)
	require.NoError(t, db.First(&row, 1).Error)
	assert.False(t, row.ConfigApplied)

	// An apply of the new configuration still can.
	set, err = repo.SetDeviceConfigAppliedIfDesired(1, `{"a":2}`)
	require.NoError(t, err)
	assert.True(t, set)
}
