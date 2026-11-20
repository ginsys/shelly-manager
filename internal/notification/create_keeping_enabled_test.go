package notification

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// A create with enabled:false is two writes (the insert, then the false that the column default replaced).
// They run in one transaction: when the second fails, no row is left behind that would read as enabled.
func TestCreateKeepingEnabled_SecondWriteFailureLeavesNoRow(t *testing.T) {
	_, db, cleanup := setupSimpleTestService(t)
	defer cleanup()

	failUpdate := false
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("test:fail-update", func(tx *gorm.DB) {
		if failUpdate {
			_ = tx.AddError(errors.New("injected update failure"))
		}
	}))

	channel := &NotificationChannel{Name: "off", Type: "webhook", Enabled: false}

	// The control: without the injected failure the row exists and is disabled, so the failure below is
	// what removes it.
	require.NoError(t, createKeepingEnabled(db, channel, false))
	var stored NotificationChannel
	require.NoError(t, db.First(&stored, channel.ID).Error)
	assert.False(t, stored.Enabled)

	failUpdate = true
	err := createKeepingEnabled(db, &NotificationChannel{Name: "off-2", Type: "webhook", Enabled: false}, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "injected update failure")

	var rows int64
	require.NoError(t, db.Model(&NotificationChannel{}).Where("name = ?", "off-2").Count(&rows).Error)
	assert.EqualValues(t, 0, rows, "the insert was rolled back with the failed second write")
}
