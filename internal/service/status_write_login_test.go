package service

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ginsys/shelly-manager/internal/database"
)

// flakyReload is a database whose device read fails once the test turns the failure on.
type flakyReload struct {
	database.DatabaseInterface
	fail atomic.Bool
}

func (f *flakyReload) GetDevice(id uint) (*database.Device, error) {
	if f.fail.Load() {
		return nil, errors.New("database unavailable")
	}
	return f.DatabaseInterface.GetDevice(id)
}

// failedReloadCase saves a new login while the device answers and then makes the reload fail. The
// status read must still answer, write nothing, and leave the new login and the old status alone.
func failedReloadCase(t *testing.T, status string) {
	t.Helper()
	gate := &gatedDevice{accept: map[string]bool{"old": true, "new": true}}
	service, device := retryFixture(t, gate)
	lastSeen := time.Now().Add(-time.Minute).Truncate(time.Second)
	device.Status = status
	device.LastSeen = lastSeen
	if err := service.DB.UpdateDevice(device); err != nil {
		t.Fatal(err)
	}
	db := &flakyReload{DatabaseInterface: service.DB}
	service.DB = db
	gate.onServe = func() {
		if err := service.UpdateDeviceAuth(device.ID, "new", "newpw"); err != nil {
			t.Errorf("UpdateDeviceAuth failed: %v", err)
		}
		db.fail.Store(true)
	}

	if _, err := service.GetDeviceStatus(device.ID); err != nil {
		t.Fatalf("GetDeviceStatus failed although the device answered: %v", err)
	}
	db.fail.Store(false)

	if user, _ := storedLogin(t, service, device.ID); user != "new" {
		t.Errorf("the stored login is %q after a failed reload, want new", user)
	}
	stored, err := service.DB.GetDevice(device.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != status || !stored.LastSeen.Equal(lastSeen) {
		t.Errorf("a failed reload still wrote the status: %q, last seen %v (want %q, %v)",
			stored.Status, stored.LastSeen, status, lastSeen)
	}
}

func TestGetDeviceStatus_OnlineWriteWithAFailedReloadWritesNothing(t *testing.T) {
	failedReloadCase(t, "online")
}

func TestGetDeviceStatus_OfflineRecoveryWithAFailedReloadWritesNothing(t *testing.T) {
	failedReloadCase(t, "offline")
}

// A status read loads the device, asks it for its status, and then records the device as online.
// A credential update lands while the device answers. The write must not put the old login back.
func TestGetDeviceStatus_OnlineWriteKeepsALoginSavedDuringTheRequest(t *testing.T) {
	gate := &gatedDevice{accept: map[string]bool{"old": true, "new": true}}
	service, device := retryFixture(t, gate)
	device.Status = "online"
	if err := service.DB.UpdateDevice(device); err != nil {
		t.Fatal(err)
	}
	gate.onServe = func() {
		if err := service.UpdateDeviceAuth(device.ID, "new", "newpw"); err != nil {
			t.Errorf("UpdateDeviceAuth failed: %v", err)
		}
	}

	if _, err := service.GetDeviceStatus(device.ID); err != nil {
		t.Fatalf("GetDeviceStatus failed: %v", err)
	}

	if user, _ := storedLogin(t, service, device.ID); user != "new" {
		t.Errorf("the stored login is %q after the status write, want new", user)
	}
	stored, err := service.DB.GetDevice(device.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != "online" || time.Since(stored.LastSeen) > time.Minute {
		t.Errorf("the status write did not land: status %q, last seen %v", stored.Status, stored.LastSeen)
	}
}

// The recovery branch: the device was marked offline a moment ago, the probe succeeds, and a
// credential update lands during the probe. The write that marks it online must keep the new login.
func TestGetDeviceStatus_OfflineRecoveryWriteKeepsALoginSavedDuringTheProbe(t *testing.T) {
	gate := &gatedDevice{accept: map[string]bool{"old": true, "new": true}}
	service, device := retryFixture(t, gate)
	device.Status = "offline"
	device.LastSeen = time.Now().Add(-time.Minute)
	if err := service.DB.UpdateDevice(device); err != nil {
		t.Fatal(err)
	}
	gate.onServe = func() {
		if err := service.UpdateDeviceAuth(device.ID, "new", "newpw"); err != nil {
			t.Errorf("UpdateDeviceAuth failed: %v", err)
		}
	}

	if _, err := service.GetDeviceStatus(device.ID); err != nil {
		t.Fatalf("GetDeviceStatus failed: %v", err)
	}

	if user, _ := storedLogin(t, service, device.ID); user != "new" {
		t.Errorf("the stored login is %q after the recovery write, want new", user)
	}
	stored, err := service.DB.GetDevice(device.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != "online" {
		t.Errorf("the recovery write did not mark the device online: status %q", stored.Status)
	}
}
