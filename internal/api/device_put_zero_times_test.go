package api

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A PUT whose body leaves out last_seen and created_at keeps the stored values
// (ginsys/shelly-manager#515). A script or an older client sends such a body. The values used to
// decode to zero times, which MySQL rejects with a 500 and the other providers stored.
func TestUpdateDevice_KeepsTheStoredTimesWhenTheBodyLeavesThemOut(t *testing.T) {
	handler, db, device := loginDeviceHandler(t)
	require.NoError(t, db.RecordDeviceSeen(device.ID, "online", time.Now().Add(-time.Hour)))
	before, err := db.GetDevice(device.ID)
	require.NoError(t, err)
	require.False(t, before.CreatedAt.IsZero())
	require.False(t, before.LastSeen.IsZero())

	read := readForPut(t, handler, device)
	delete(read, "last_seen")
	delete(read, "created_at")
	delete(read, "updated_at")
	read["name"] = "Renamed by a script"

	code, body := putDevice(t, handler, read)
	require.Equal(t, http.StatusOK, code, body)

	after, err := db.GetDevice(device.ID)
	require.NoError(t, err)
	assert.Equal(t, "Renamed by a script", after.Name)
	assert.True(t, after.CreatedAt.Equal(before.CreatedAt), "created_at: stored %v, now %v", before.CreatedAt, after.CreatedAt)
	assert.True(t, after.LastSeen.Equal(before.LastSeen), "last_seen: stored %v, now %v", before.LastSeen, after.LastSeen)
}
