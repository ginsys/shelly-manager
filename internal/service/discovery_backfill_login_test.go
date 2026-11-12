package service

import (
	"context"
	"testing"
	"time"

	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/discovery"
)

// upsertHookDB calls a hook once, right after the discovery upsert returns. The test saves a new
// login there: the point where a settings backfill that takes no lock would later put the older
// login back (ginsys/shelly-manager#496).
type upsertHookDB struct {
	database.DatabaseInterface
	afterUpsert func()
}

func (d *upsertHookDB) UpsertDeviceFromDiscovery(mac string, update database.DiscoveryUpdate, initialName string) (*database.Device, error) {
	device, err := d.DatabaseInterface.UpsertDeviceFromDiscovery(mac, update, initialName)
	if hook := d.afterUpsert; hook != nil {
		d.afterUpsert = nil
		hook()
	}
	return device, err
}

// A discovery backfills model, generation and the auth flag in the settings. It keeps a login
// that was saved after the discovery read the device.
func TestDiscoverDevices_BackfillKeepsALoginSavedAfterItsRead(t *testing.T) {
	db := createTestDB(t)
	service := NewService(db, createTestConfig())
	device := createTestDevice(t, db, "192.0.2.10")
	device.Settings = `{"gen":1,"auth_enabled":true,"auth_user":"old","auth_pass":"oldpw","note":"keep"}`
	if err := db.UpdateDevice(device); err != nil {
		t.Fatal(err)
	}
	service.SetDiscoverFunc(func(context.Context, []string, time.Duration) ([]discovery.ShellyDevice, error) {
		return []discovery.ShellyDevice{{
			ID: "shelly1-1", MAC: device.MAC, Model: "SHSW-NEW", Generation: 1, Version: "1.14.0",
			AuthEn: true, IP: device.IP, Discovered: time.Now(),
		}}, nil
	})
	service.DB = &upsertHookDB{
		DatabaseInterface: db,
		afterUpsert: func() {
			stored, err := db.GetDevice(device.ID)
			if err != nil {
				t.Errorf("GetDevice failed: %v", err)
				return
			}
			stored.Settings = `{"gen":1,"auth_enabled":true,"auth_user":"bee","auth_pass":"beepw","note":"keep"}`
			if err := db.UpdateDevice(stored); err != nil {
				t.Errorf("UpdateDevice failed: %v", err)
			}
		},
	}

	if _, err := service.DiscoverDevices("auto"); err != nil {
		t.Fatalf("DiscoverDevices failed: %v", err)
	}

	user, settings := storedLogin(t, service, device.ID)
	if user != "bee" || settings["auth_pass"] != "beepw" {
		t.Errorf("the stored login is %q/%v, want bee/beepw", user, settings["auth_pass"])
	}
	if settings["model"] != "SHSW-NEW" {
		t.Errorf("the backfill did not store the model: %v", settings["model"])
	}
	if settings["note"] != "keep" {
		t.Errorf("the backfill dropped a stored setting: %v", settings["note"])
	}
}
