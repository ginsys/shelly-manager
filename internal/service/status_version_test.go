package service

import (
	"testing"
	"time"
)

// updated_at is the version of the device that a client edits (ginsys/shelly-manager#502). A write
// that only records what a status read observed must not move it, or the device page goes stale on
// every status poll. Each test covers one path that records a device as online.

// versionAfter runs act on a device that is set to the given status, and fails if act moved
// updated_at or did not mark the device online.
func versionAfter(t *testing.T, status string, act func(s *ShellyService, id uint)) {
	t.Helper()
	gate := &gatedDevice{accept: map[string]bool{"old": true}}
	service, device := retryFixture(t, gate)
	device.Status = status
	device.LastSeen = time.Now().Add(-time.Minute)
	if err := service.DB.UpdateDevice(device); err != nil {
		t.Fatal(err)
	}
	before, err := service.DB.GetDevice(device.ID)
	if err != nil {
		t.Fatal(err)
	}
	// A save moves updated_at to the clock, so wait out the clock's resolution.
	time.Sleep(20 * time.Millisecond)

	act(service, device.ID)

	after, err := service.DB.GetDevice(device.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !after.UpdatedAt.Equal(before.UpdatedAt) {
		t.Errorf("updated_at moved from %v to %v", before.UpdatedAt, after.UpdatedAt)
	}
	if after.Status != "online" || !after.LastSeen.After(before.LastSeen) {
		t.Errorf("the observation did not land: status %q, last seen %v (was %v)", after.Status, after.LastSeen, before.LastSeen)
	}
}

func TestGetDeviceStatus_OnlineReadKeepsTheVersion(t *testing.T) {
	versionAfter(t, "online", func(s *ShellyService, id uint) {
		if _, err := s.GetDeviceStatus(id); err != nil {
			t.Fatalf("GetDeviceStatus failed: %v", err)
		}
	})
}

func TestGetDeviceStatus_OfflineRecoveryKeepsTheVersion(t *testing.T) {
	versionAfter(t, "offline", func(s *ShellyService, id uint) {
		if _, err := s.GetDeviceStatus(id); err != nil {
			t.Fatalf("GetDeviceStatus failed: %v", err)
		}
	})
}

func TestControlDevice_RecordingTheDeviceOnlineKeepsTheVersion(t *testing.T) {
	versionAfter(t, "online", func(s *ShellyService, id uint) {
		if err := s.ControlDevice(id, "on", nil); err != nil {
			t.Fatalf("ControlDevice failed: %v", err)
		}
	})
}
