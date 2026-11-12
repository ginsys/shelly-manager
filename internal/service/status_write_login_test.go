package service

import (
	"testing"
	"time"
)

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
