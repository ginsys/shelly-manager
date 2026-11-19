package service

import (
	"context"
	"testing"
	"time"

	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/discovery"
)

// updated_at is the version of the device PUT (ginsys/shelly-manager#502). A discovery that found
// nothing new about a device is an observation and must leave it, so a person who loaded the device
// before the pass can still save it (ginsys/shelly-manager#583).

func storedVersion(t *testing.T, db *database.Manager, id uint) time.Time {
	t.Helper()
	device, err := db.GetDevice(id)
	if err != nil {
		t.Fatalf("GetDevice failed: %v", err)
	}
	return device.UpdatedAt
}

func TestRefreshDiscoveredDevice_VersionMovesOnlyOnAChange(t *testing.T) {
	db := createTestDB(t)
	service := NewService(db, createTestConfig())
	device := createTestDevice(t, db, "192.0.2.20")
	before := storedVersion(t, db, device.ID)
	time.Sleep(5 * time.Millisecond)

	got, err := service.RefreshDiscoveredDevice(device.ID, device.IP, "online", time.Now(), device.Firmware)
	if err != nil {
		t.Fatalf("RefreshDiscoveredDevice failed: %v", err)
	}
	if !got.UpdatedAt.Equal(before) {
		t.Errorf("an unchanged refresh moved updated_at from %v to %v", before, got.UpdatedAt)
	}
	if got.Status != "online" {
		t.Errorf("the refresh did not store the status: %q", got.Status)
	}

	got, err = service.RefreshDiscoveredDevice(device.ID, device.IP, "online", time.Now(), "1.15.0")
	if err != nil {
		t.Fatalf("RefreshDiscoveredDevice failed: %v", err)
	}
	if !got.UpdatedAt.After(before) {
		t.Errorf("a refresh with a new firmware left updated_at at %v", got.UpdatedAt)
	}
	if got.Firmware != "1.15.0" {
		t.Errorf("the refresh did not store the firmware: %q", got.Firmware)
	}
}

// A full discovery pass also backfills the settings of the device. A pass over a device that
// carries them already writes nothing that a client edits, so the version stays.
func TestDiscoverDevices_VersionMovesOnlyOnAChange(t *testing.T) {
	db := createTestDB(t)
	service := NewService(db, createTestConfig())
	firmware := "1.14.0"
	service.SetDiscoverFunc(func(context.Context, []string, time.Duration) ([]discovery.ShellyDevice, error) {
		return []discovery.ShellyDevice{{
			ID: "shelly1-2", MAC: "68C63A123457", Model: "SHSW-25", Generation: 1, Version: firmware,
			AuthEn: false, IP: "192.0.2.21", Discovered: time.Now(),
		}}, nil
	})

	// The first pass creates the device and stores the settings it discovered.
	found, err := service.DiscoverDevices("auto")
	if err != nil || len(found) != 1 {
		t.Fatalf("first DiscoverDevices: %v, %d devices", err, len(found))
	}
	id := found[0].ID
	first := storedVersion(t, db, id)
	time.Sleep(5 * time.Millisecond)

	found, err = service.DiscoverDevices("auto")
	if err != nil || len(found) != 1 {
		t.Fatalf("second DiscoverDevices: %v, %d devices", err, len(found))
	}
	if second := storedVersion(t, db, id); !second.Equal(first) {
		t.Errorf("a pass that found nothing new moved updated_at from %v to %v", first, second)
	}
	if !found[0].UpdatedAt.Equal(first) {
		t.Errorf("the returned device carries version %v, stored is %v", found[0].UpdatedAt, first)
	}

	firmware = "1.15.0"
	found, err = service.DiscoverDevices("auto")
	if err != nil || len(found) != 1 {
		t.Fatalf("third DiscoverDevices: %v, %d devices", err, len(found))
	}
	if third := storedVersion(t, db, id); !third.After(first) {
		t.Errorf("a pass that found a new firmware left updated_at at %v", third)
	}
	if found[0].Firmware != "1.15.0" {
		t.Errorf("the returned device carries firmware %q", found[0].Firmware)
	}
}

// A device whose settings do not hold what a discovery stores gets them once, and a client that
// edited the settings still meets a changed version.
func TestDiscoverDevices_BackfillMovesTheVersionOnlyWhenTheSettingsChange(t *testing.T) {
	db := createTestDB(t)
	service := NewService(db, createTestConfig())
	device := createTestDevice(t, db, "192.0.2.22")
	// A discovery names the type of a model with discovery.GetDeviceType. Store that type, so that
	// only the settings differ and the first pass moves the version through the backfill alone.
	device.Type = discovery.GetDeviceType("SHSW-25")
	if err := saveDevice(db, device); err != nil {
		t.Fatal(err)
	}
	service.SetDiscoverFunc(func(context.Context, []string, time.Duration) ([]discovery.ShellyDevice, error) {
		return []discovery.ShellyDevice{{
			ID: "shelly1-3", MAC: device.MAC, Model: "SHSW-25", Generation: 1, Version: device.Firmware,
			AuthEn: false, IP: device.IP, Discovered: time.Now(),
		}}, nil
	})
	before := storedVersion(t, db, device.ID)
	time.Sleep(5 * time.Millisecond)

	// createTestDevice has no auth_user or auth_pass, so the first backfill adds them.
	if _, err := service.DiscoverDevices("auto"); err != nil {
		t.Fatalf("first DiscoverDevices failed: %v", err)
	}
	first := storedVersion(t, db, device.ID)
	if !first.After(before) {
		t.Fatalf("a backfill that added settings left updated_at at %v", first)
	}
	if _, settings := storedLogin(t, service, device.ID); settings["auth_pass"] != "" {
		t.Fatalf("the backfill did not store the missing auth_pass: %v", settings["auth_pass"])
	}
	time.Sleep(5 * time.Millisecond)

	if _, err := service.DiscoverDevices("auto"); err != nil {
		t.Fatalf("second DiscoverDevices failed: %v", err)
	}
	if second := storedVersion(t, db, device.ID); !second.Equal(first) {
		t.Errorf("a backfill that changed nothing moved updated_at from %v to %v", first, second)
	}
}
