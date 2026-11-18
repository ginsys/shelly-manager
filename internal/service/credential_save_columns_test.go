package service

import (
	"testing"
	"time"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/database"
)

// The writers of the device row that the service owns each write their own columns and no other
// (ginsys/shelly-manager#565). Another writer, on this replica or another, sets config_applied,
// desired_config and pending_reboot without taking clientMu. These cases put that writer right
// after the read the service made, which is the instant a whole-row save would write the old
// values back over it.

const (
	racerDesired = `{"set":"by the other writer"}`
	storedBefore = `{"set":"before"}`
)

// afterEveryReadDB runs hook right after every GetDevice returns, so a row read by the service is
// already out of date when the service holds it.
type afterEveryReadDB struct {
	database.DatabaseInterface
	hook func()
}

func (r *afterEveryReadDB) GetDevice(id uint) (*database.Device, error) {
	device, err := r.DatabaseInterface.GetDevice(id)
	if r.hook != nil {
		r.hook()
	}
	return device, err
}

// setFlags writes the three columns that the credential and discovery writers do not own, with
// one UPDATE of those columns: the way the apply and the import write them.
func setFlags(t *testing.T, db database.DatabaseInterface, id uint, applied bool, desired string, pending bool) {
	t.Helper()
	err := db.GetDB().Model(&database.Device{}).Where("id = ?", id).
		UpdateColumns(map[string]any{"config_applied": applied, "desired_config": desired}).Error
	if err != nil {
		t.Fatal(err)
	}
	// pending_reboot is read-only on the Device model: only the config repository writes it.
	if err := configuration.NewGormConfigRepository(db.GetDB(), nil).SetDevicePendingReboot(id, pending); err != nil {
		t.Fatal(err)
	}
}

// otherWriter is what a second writer does: it clears config_applied, replaces desired_config and
// sets pending_reboot.
func otherWriter(t *testing.T, db database.DatabaseInterface, id uint) func() {
	return func() { setFlags(t, db, id, false, racerDesired, true) }
}

// assertOtherWriterKept fails when a column of the other writer is not as it set it.
func assertOtherWriterKept(t *testing.T, db database.DatabaseInterface, id uint) *database.Device {
	t.Helper()
	got, err := db.GetDevice(id)
	if err != nil {
		t.Fatal(err)
	}
	if got.ConfigApplied {
		t.Error("config_applied is true: a stale save wrote it back over the other writer")
	}
	if got.DesiredConfig != racerDesired {
		t.Errorf("desired_config is %q: a stale save wrote it back over the other writer", got.DesiredConfig)
	}
	if !got.PendingReboot {
		t.Error("pending_reboot is false: a stale save wrote it back over the other writer")
	}
	return got
}

// raceFixture is the retry fixture with the flags the caller will read as stored.
func raceFixture(t *testing.T) (*ShellyService, *database.Device, *gatedDevice) {
	t.Helper()
	gate := &gatedDevice{accept: map[string]bool{"prov": true, "new": true}, gateGET: true}
	service, device := retryFixture(t, gate)
	setFlags(t, service.DB, device.ID, true, storedBefore, false)
	device, err := service.DB.GetDevice(device.ID)
	if err != nil {
		t.Fatal(err)
	}
	return service, device, gate
}

func TestUpdateDeviceAuth_KeepsTheOtherWritersColumns(t *testing.T) {
	service, device, _ := raceFixture(t)
	real := service.DB
	service.DB = &afterEveryReadDB{DatabaseInterface: real, hook: otherWriter(t, real, device.ID)}

	if err := service.UpdateDeviceAuth(device.ID, "new", "newpw"); err != nil {
		t.Fatalf("UpdateDeviceAuth failed: %v", err)
	}

	assertOtherWriterKept(t, real, device.ID)
	if user, settings := storedLogin(t, service, device.ID); user != "new" || settings["note"] != "keep" {
		t.Errorf("stored login %q, note %v: want the new login and the note kept", user, settings["note"])
	}
}

func TestClearRejectedLogin_KeepsTheOtherWritersColumns(t *testing.T) {
	service, device, _ := raceFixture(t)
	real := service.DB
	service.DB = &afterEveryReadDB{DatabaseInterface: real, hook: otherWriter(t, real, device.ID)}

	snapshot := *device
	if !service.clearRejectedLogin(&snapshot, "old", "oldpw") {
		t.Fatal("clearRejectedLogin returned false, want true")
	}

	assertOtherWriterKept(t, real, device.ID)
	if user, settings := storedLogin(t, service, device.ID); user != "" || settings["note"] != "keep" {
		t.Errorf("stored login %q, note %v: want the login cleared and the note kept", user, settings["note"])
	}
}

func TestBackfillDiscoverySettings_KeepsTheOtherWritersColumns(t *testing.T) {
	service, device, _ := raceFixture(t)
	real := service.DB
	service.DB = &afterEveryReadDB{DatabaseInterface: real, hook: otherWriter(t, real, device.ID)}

	if err := service.backfillDiscoverySettings(device, "SHPLG-S", 1, true); err != nil {
		t.Fatalf("backfillDiscoverySettings failed: %v", err)
	}

	assertOtherWriterKept(t, real, device.ID)
	if user, settings := storedLogin(t, service, device.ID); user != "old" || settings["note"] != "keep" {
		t.Errorf("stored login %q, note %v: want the login and the note kept", user, settings["note"])
	}
}

func TestRefreshDiscoveredDevice_KeepsTheOtherWritersColumns(t *testing.T) {
	service, device, _ := raceFixture(t)
	real := service.DB
	service.DB = &afterEveryReadDB{DatabaseInterface: real, hook: otherWriter(t, real, device.ID)}

	seen := time.Now().Truncate(time.Second)
	if _, err := service.RefreshDiscoveredDevice(device.ID, "192.0.2.50", "online", seen, "2.0"); err != nil {
		t.Fatalf("RefreshDiscoveredDevice failed: %v", err)
	}

	got := assertOtherWriterKept(t, real, device.ID)
	if got.IP != "192.0.2.50" || got.Status != "online" || got.Firmware != "2.0" {
		t.Errorf("the discovery fields were not stored: ip %q, status %q, firmware %q", got.IP, got.Status, got.Firmware)
	}
}

// The login fallback finds that the provisioning login works and saves it. The caller's device is
// the row it read earlier, and the device answers the test request after the other writer ran.
func TestLoginFallbackSave_KeepsTheOtherWritersColumnsAndTheSettingsKeys(t *testing.T) {
	service, device, gate := raceFixture(t)
	service.Config.Provisioning.AuthEnabled = true
	service.Config.Provisioning.AuthUser = "prov"
	service.Config.Provisioning.AuthPassword = "provpw"
	device.Settings = `{"gen":1,"auth_enabled":true,"note":"keep","legacy":{"x":1}}`
	if err := service.DB.GetDB().Model(&database.Device{}).Where("id = ?", device.ID).
		UpdateColumn("settings", device.Settings).Error; err != nil {
		t.Fatal(err)
	}
	gate.onServe = otherWriter(t, service.DB, device.ID)

	if _, err := service.getClientWithRetry(device, true); err != nil {
		t.Fatalf("getClientWithRetry failed: %v", err)
	}

	assertOtherWriterKept(t, service.DB, device.ID)
	user, settings := storedLogin(t, service, device.ID)
	if user != "prov" {
		t.Errorf("the working login was not saved: stored user %q, want prov", user)
	}
	if settings["note"] != "keep" || settings["legacy"] == nil {
		t.Errorf("the save dropped settings keys: %v", settings)
	}
}

// A second settings writer stores a login after the fallback's read and before its save. The
// fallback builds its text from a fresh read at write time and leaves a login that is stored now:
// the login that someone stored on purpose is never put back to the one the fallback found.
func TestLoginFallbackSave_LeavesALoginStoredMeanwhile(t *testing.T) {
	service, device, gate := raceFixture(t)
	service.Config.Provisioning.AuthEnabled = true
	service.Config.Provisioning.AuthUser = "prov"
	service.Config.Provisioning.AuthPassword = "provpw"
	start := `{"gen":1,"auth_enabled":true,"note":"keep"}`
	if err := service.DB.GetDB().Model(&database.Device{}).Where("id = ?", device.ID).
		UpdateColumn("settings", start).Error; err != nil {
		t.Fatal(err)
	}
	device.Settings = start
	gate.onServe = func() {
		meanwhile := `{"gen":1,"auth_enabled":true,"auth_user":"new","auth_pass":"newpw","note":"changed"}`
		if err := service.DB.GetDB().Model(&database.Device{}).Where("id = ?", device.ID).
			UpdateColumn("settings", meanwhile).Error; err != nil {
			t.Error(err)
		}
	}

	if _, err := service.getClientWithRetry(device, true); err != nil {
		t.Fatalf("getClientWithRetry failed: %v", err)
	}

	user, settings := storedLogin(t, service, device.ID)
	if user != "new" {
		t.Errorf("the stored user is %q, want new: the fallback overwrote a login stored meanwhile", user)
	}
	if settings["note"] != "changed" {
		t.Errorf("the note is %v, want changed: the fallback overwrote the other writer's settings", settings["note"])
	}
}
