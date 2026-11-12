package api

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/database"
)

// These tests pin that a writer which saves a whole device row keeps a login that was saved
// after the writer read the row (ginsys/shelly-manager#496). readHookDB runs a function once,
// right after the writer's first read returns, so the test saves the new login at the exact point
// where a writer that takes no lock would later put the older login back.

// readHookDB wraps a database and calls a hook once after the next read of the named kind.
type readHookDB struct {
	database.DatabaseInterface
	afterGetDevice      func()
	afterGetDeviceByMAC func()
}

func (d *readHookDB) GetDevice(id uint) (*database.Device, error) {
	device, err := d.DatabaseInterface.GetDevice(id)
	if hook := d.afterGetDevice; hook != nil {
		d.afterGetDevice = nil
		hook()
	}
	return device, err
}

func (d *readHookDB) GetDeviceByMAC(mac string) (*database.Device, error) {
	device, err := d.DatabaseInterface.GetDeviceByMAC(mac)
	if hook := d.afterGetDeviceByMAC; hook != nil {
		d.afterGetDeviceByMAC = nil
		hook()
	}
	return device, err
}

const loginADevice = `{"gen":1,"auth_enabled":true,"auth_user":"old","auth_pass":"oldpw","note":"keep"}`

// saveLogin stores login user/pass in the device row, as UpdateDeviceAuth does.
func saveLogin(t *testing.T, db database.DatabaseInterface, id uint, user, pass string) {
	t.Helper()
	stored, err := db.GetDevice(id)
	require.NoError(t, err)
	settings := map[string]any{
		"gen": 1, "auth_enabled": true, "auth_user": user, "auth_pass": pass, "note": "keep",
		"auth": map[string]string{"username": user, "password": pass},
	}
	text, err := json.Marshal(settings)
	require.NoError(t, err)
	stored.Settings = string(text)
	require.NoError(t, db.UpdateDevice(stored))
}

func requireStoredLogin(t *testing.T, db database.DatabaseInterface, id uint, user, pass string) {
	t.Helper()
	stored, err := db.GetDevice(id)
	require.NoError(t, err)
	var settings map[string]any
	require.NoError(t, json.Unmarshal([]byte(stored.Settings), &settings))
	assert.Equal(t, user, settings["auth_user"], "stored login user")
	assert.Equal(t, pass, settings["auth_pass"], "stored login password")
}

// loginDeviceHandler is a handler with one device that stores login old/oldpw.
func loginDeviceHandler(t *testing.T) (*Handler, *database.Manager, *database.Device) {
	t.Helper()
	handler, db := newTestHandler(t)
	device := &database.Device{
		IP: "127.0.0.1:1", MAC: "A4:CF:12:34:56:78", Type: "Shelly1", Name: "Before",
		Firmware: "1.0.0", Status: "online", Settings: loginADevice,
	}
	require.NoError(t, db.AddDevice(device))
	return handler, db, device
}

// readForPut reads the device through the API, as a client does, and returns it as a map.
func readForPut(t *testing.T, handler *Handler, device *database.Device) map[string]any {
	t.Helper()
	get := serve(handler.GetDevice, "GET", "/api/v1/devices/1", idString(device), "")
	require.Equal(t, http.StatusOK, get.Code, get.Body.String())
	var read map[string]any
	require.NoError(t, json.Unmarshal(dataOf(t, get.Body.Bytes()), &read))
	return read
}

// A plain device PUT that does not change the login keeps the login saved after its read.
func TestUpdateDevice_KeepsALoginSavedAfterItsRead(t *testing.T) {
	handler, db, device := loginDeviceHandler(t)
	read := readForPut(t, handler, device)
	read["name"] = "Renamed"
	body, err := json.Marshal(read)
	require.NoError(t, err)

	handler.DB = &readHookDB{
		DatabaseInterface: db,
		afterGetDevice:    func() { saveLogin(t, db, device.ID, "bee", "beepw") },
	}
	put := serve(handler.UpdateDevice, "PUT", "/api/v1/devices/1", idString(device), string(body))
	require.Equal(t, http.StatusOK, put.Code, put.Body.String())

	stored, err := db.GetDevice(device.ID)
	require.NoError(t, err)
	assert.Equal(t, "Renamed", stored.Name, "the PUT must still save its own change")
	requireStoredLogin(t, db, device.ID, "bee", "beepw")
}

// A PUT that changes the login on purpose wins over a login saved after its read.
func TestUpdateDevice_SavesALoginTheRequestChanges(t *testing.T) {
	handler, db, device := loginDeviceHandler(t)
	read := readForPut(t, handler, device)
	read["settings"] = `{"gen":1,"auth_enabled":true,"auth_user":"carol","auth_pass":"carolpw","note":"keep"}`
	body, err := json.Marshal(read)
	require.NoError(t, err)

	handler.DB = &readHookDB{
		DatabaseInterface: db,
		afterGetDevice:    func() { saveLogin(t, db, device.ID, "bee", "beepw") },
	}
	put := serve(handler.UpdateDevice, "PUT", "/api/v1/devices/1", idString(device), string(body))
	require.Equal(t, http.StatusOK, put.Code, put.Body.String())

	requireStoredLogin(t, db, device.ID, "carol", "carolpw")
}

// A PUT keeps every other setting that the request sends, and drops none of the stored ones.
func TestUpdateDevice_KeepsTheRequestsOtherSettings(t *testing.T) {
	handler, db, device := loginDeviceHandler(t)
	read := readForPut(t, handler, device)
	read["settings"] = `{"gen":1,"auth_enabled":true,"auth_user":"old","auth_pass":"` + secretMask + `","note":"changed"}`
	body, err := json.Marshal(read)
	require.NoError(t, err)

	handler.DB = &readHookDB{
		DatabaseInterface: db,
		afterGetDevice:    func() { saveLogin(t, db, device.ID, "bee", "beepw") },
	}
	put := serve(handler.UpdateDevice, "PUT", "/api/v1/devices/1", idString(device), string(body))
	require.Equal(t, http.StatusOK, put.Code, put.Body.String())

	stored, err := db.GetDevice(device.ID)
	require.NoError(t, err)
	var settings map[string]any
	require.NoError(t, json.Unmarshal([]byte(stored.Settings), &settings))
	assert.Equal(t, "changed", settings["note"])
	assert.Equal(t, "bee", settings["auth_user"])
	assert.Equal(t, "beepw", settings["auth_pass"])
}

// The import refresh updates the address, status and firmware. It keeps a login saved after it
// read the device.
func TestRefreshKnownDevice_KeepsALoginSavedAfterItsRead(t *testing.T) {
	handler, db, device := loginDeviceHandler(t)
	handler.DB = &readHookDB{
		DatabaseInterface:   db,
		afterGetDeviceByMAC: func() { saveLogin(t, db, device.ID, "bee", "beepw") },
	}

	seen := time.Now().Add(time.Hour).Truncate(time.Second)
	_, known := handler.refreshKnownDevice(database.Device{
		MAC: device.MAC, IP: "127.0.0.1:2", Status: "offline", LastSeen: seen, Firmware: "2.0.0",
	})
	require.True(t, known)

	stored, err := db.GetDevice(device.ID)
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:2", stored.IP)
	assert.Equal(t, "offline", stored.Status)
	assert.Equal(t, "2.0.0", stored.Firmware)
	assert.True(t, stored.LastSeen.Equal(seen), "last seen %v, want %v", stored.LastSeen, seen)
	requireStoredLogin(t, db, device.ID, "bee", "beepw")
}
