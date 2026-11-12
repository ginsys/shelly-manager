package api

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/database"
)

// These tests pin the version check of the plain device PUT (ginsys/shelly-manager#502). The
// client sends back the updated_at it read. A PUT whose updated_at is not the stored one is
// refused with 409, so a client never saves old data over a newer save.

func putDevice(t *testing.T, handler *Handler, body map[string]any) (int, string) {
	t.Helper()
	text, err := json.Marshal(body)
	require.NoError(t, err)
	rec := serve(handler.UpdateDevice, "PUT", "/api/v1/devices/1", "1", string(text))
	return rec.Code, rec.Body.String()
}

// The updated_at that a GET returns is accepted by the PUT as it stands. It crosses a JSON round
// trip and the database, and must lose no precision. A second PUT with the same value is stale,
// because the first PUT moved updated_at.
func TestUpdateDevice_AcceptsTheVersionItRead(t *testing.T) {
	handler, db, device := loginDeviceHandler(t)
	read := readForPut(t, handler, device)
	require.NotEmpty(t, read["updated_at"], "the GET must return the version")

	read["name"] = "First"
	code, body := putDevice(t, handler, read)
	require.Equal(t, http.StatusOK, code, body)

	read["name"] = "Second"
	code, body = putDevice(t, handler, read)
	assert.Equal(t, http.StatusConflict, code, body)

	stored, err := db.GetDevice(device.ID)
	require.NoError(t, err)
	assert.Equal(t, "First", stored.Name, "the stale PUT must save nothing")
}

// The PUT response carries the new updated_at, so the client can save again without a new GET.
func TestUpdateDevice_ResponseCarriesTheNewVersion(t *testing.T) {
	handler, _, device := loginDeviceHandler(t)
	read := readForPut(t, handler, device)

	read["name"] = "First"
	text, err := json.Marshal(read)
	require.NoError(t, err)
	rec := serve(handler.UpdateDevice, "PUT", "/api/v1/devices/1", "1", string(text))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var saved map[string]any
	require.NoError(t, json.Unmarshal(dataOf(t, rec.Body.Bytes()), &saved))

	saved["name"] = "Second"
	code, body := putDevice(t, handler, saved)
	assert.Equal(t, http.StatusOK, code, body)
}

// A database can store updated_at with less precision than the clock gives (PostgreSQL keeps
// microseconds). The PUT answer must carry the stored value, or a client that sends the answer
// back gets a false 409. The callback below makes the test database round to microseconds after
// each update, as PostgreSQL does.
func TestUpdateDevice_ResponseVersionIsTheStoredValueWhenTheDatabaseRounds(t *testing.T) {
	handler, db, device := loginDeviceHandler(t)
	const name = "test:round-updated-at-to-microseconds"
	require.NoError(t, db.GetDB().Callback().Update().After("gorm:update").Register(name, func(tx *gorm.DB) {
		saved, ok := tx.Statement.Dest.(*database.Device)
		if !ok || tx.Error != nil {
			return
		}
		rounded := saved.UpdatedAt.Truncate(time.Microsecond)
		tx.Session(&gorm.Session{NewDB: true}).Exec("UPDATE devices SET updated_at = ? WHERE id = ?", rounded, saved.ID)
	}))
	t.Cleanup(func() { _ = db.GetDB().Callback().Update().Remove(name) })

	read := readForPut(t, handler, device)
	read["name"] = "First"
	text, err := json.Marshal(read)
	require.NoError(t, err)
	rec := serve(handler.UpdateDevice, "PUT", "/api/v1/devices/1", "1", string(text))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var saved map[string]any
	require.NoError(t, json.Unmarshal(dataOf(t, rec.Body.Bytes()), &saved))

	stored, err := db.GetDevice(device.ID)
	require.NoError(t, err)
	answered, err := time.Parse(time.RFC3339Nano, saved["updated_at"].(string))
	require.NoError(t, err)
	require.True(t, answered.Equal(stored.UpdatedAt), "the answer carries %v and the row holds %v", answered, stored.UpdatedAt)

	saved["name"] = "Second"
	code, body := putDevice(t, handler, saved)
	assert.Equal(t, http.StatusOK, code, body)
}

// Two spellings of one instant are one version. The compare uses time.Equal and not a text match.
func TestUpdateDevice_AcceptsTheSameInstantInAnotherZone(t *testing.T) {
	handler, _, device := loginDeviceHandler(t)
	read := readForPut(t, handler, device)
	instant, err := time.Parse(time.RFC3339Nano, read["updated_at"].(string))
	require.NoError(t, err)
	read["updated_at"] = instant.In(time.FixedZone("test", 5*3600+1800)).Format(time.RFC3339Nano)

	read["name"] = "Renamed"
	code, body := putDevice(t, handler, read)
	assert.Equal(t, http.StatusOK, code, body)
}

// A save made after the client's read makes the client's version stale, a login save included.
func TestUpdateDevice_RefusesAVersionThatAnotherSaveMoved(t *testing.T) {
	handler, db, device := loginDeviceHandler(t)
	read := readForPut(t, handler, device)

	saveLogin(t, db, device.ID, "bee", "beepw")

	read["name"] = "Renamed"
	code, body := putDevice(t, handler, read)
	assert.Equal(t, http.StatusConflict, code, body)

	stored, err := db.GetDevice(device.ID)
	require.NoError(t, err)
	assert.Equal(t, "Before", stored.Name, "the refused PUT must save nothing")
	requireStoredLogin(t, db, device.ID, "bee", "beepw")
}

// A body with no updated_at is an older client or a script. It saves as before.
func TestUpdateDevice_WithoutAVersionStillSaves(t *testing.T) {
	handler, db, device := loginDeviceHandler(t)
	read := readForPut(t, handler, device)
	delete(read, "updated_at")
	saveLogin(t, db, device.ID, "bee", "beepw")

	read["name"] = "Renamed"
	code, body := putDevice(t, handler, read)
	assert.Equal(t, http.StatusOK, code, body)

	stored, err := db.GetDevice(device.ID)
	require.NoError(t, err)
	assert.Equal(t, "Renamed", stored.Name)
}
