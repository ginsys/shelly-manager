package service

import (
	"errors"
	"testing"
	"time"

	"github.com/ginsys/shelly-manager/internal/database"
)

// The device PUT writes the device's own fields. template_ids, overrides, desired_config and
// config_applied each have an owner and an endpoint (the template and overrides routes, the apply),
// and desired_config is derived, so a PUT body never sets them (ginsys/shelly-manager#572).

// saveDevice stores a whole row to set a fixture up. No production writer does this any more: the
// tests seed rows with it (ginsys/shelly-manager#572).
func saveDevice(db database.DatabaseInterface, device *database.Device) error {
	return db.GetDB().Save(device).Error
}

// putFixture stores the four config columns, reads the row as a client would and returns a body
// that carries other values for all four.
func putFixture(t *testing.T) (service *ShellyService, basis, incoming *database.Device) {
	t.Helper()
	gate := &gatedDevice{accept: map[string]bool{"old": true}}
	service, device := retryFixture(t, gate)
	err := service.DB.GetDB().Model(&database.Device{}).Where("id = ?", device.ID).UpdateColumns(map[string]any{
		"template_ids": `[7]`, "overrides": `{"o":1}`, "desired_config": `{"d":1}`, "config_applied": true,
	}).Error
	if err != nil {
		t.Fatal(err)
	}
	read, err := service.DB.GetDevice(device.ID)
	if err != nil {
		t.Fatal(err)
	}
	basisRow := *read
	body := *read
	body.Name = "renamed by the PUT"
	body.TemplateIDs, body.Overrides, body.DesiredConfig, body.ConfigApplied = `[8,9]`, `{"o":2}`, `{"d":2}`, false
	return service, &basisRow, &body
}

func assertConfigColumnsAsStored(t *testing.T, db database.DatabaseInterface, id uint) *database.Device {
	t.Helper()
	got, err := db.GetDevice(id)
	if err != nil {
		t.Fatal(err)
	}
	if got.TemplateIDs != `[7]` {
		t.Errorf("template_ids is %q: the PUT wrote it", got.TemplateIDs)
	}
	if got.Overrides != `{"o":1}` {
		t.Errorf("overrides is %q: the PUT wrote it", got.Overrides)
	}
	if got.DesiredConfig != `{"d":1}` {
		t.Errorf("desired_config is %q: the PUT wrote it", got.DesiredConfig)
	}
	if !got.ConfigApplied {
		t.Error("config_applied is false: the PUT wrote it")
	}
	return got
}

func TestDevicePut_LeavesTheConfigColumnsAsStored_Versioned(t *testing.T) {
	service, basis, incoming := putFixture(t)

	if err := service.UpdateDeviceKeepingLogin(basis, incoming); err != nil {
		t.Fatalf("the PUT failed: %v", err)
	}

	got := assertConfigColumnsAsStored(t, service.DB, basis.ID)
	if got.Name != "renamed by the PUT" {
		t.Errorf("the PUT did not save the device's own field: name is %q", got.Name)
	}
}

func TestDevicePut_LeavesTheConfigColumnsAsStored_Unversioned(t *testing.T) {
	service, basis, incoming := putFixture(t)
	incoming.UpdatedAt = time.Time{}

	if err := service.UpdateDeviceKeepingLogin(basis, incoming); err != nil {
		t.Fatalf("the PUT failed: %v", err)
	}

	got := assertConfigColumnsAsStored(t, service.DB, basis.ID)
	if got.Name != "renamed by the PUT" {
		t.Errorf("the PUT did not save the device's own field: name is %q", got.Name)
	}
}

// A body with no updated_at is accepted, and its save is still versioned: a writer that commits
// between the service's read and the save makes it fail, as it does for a body with a version.
func TestDevicePut_UnversionedBodyRefusesAWriterThatCommitsBeforeTheSave(t *testing.T) {
	service, basis, incoming := putFixture(t)
	incoming.UpdatedAt = time.Time{}
	real := service.DB

	racer := &racingDB{DatabaseInterface: real}
	racer.hook = func() {
		time.Sleep(20 * time.Millisecond)
		err := real.GetDB().Model(&database.Device{}).Where("id = ?", basis.ID).
			Update("name", "renamed by the other writer").Error // Update moves updated_at; UpdateColumn does not
		if err != nil {
			t.Error(err)
		}
	}
	service.DB = racer

	err := service.UpdateDeviceKeepingLogin(basis, incoming)
	if !errors.Is(err, ErrDeviceChanged) {
		t.Fatalf("the save answered %v, want ErrDeviceChanged", err)
	}
	stored, err := real.GetDevice(basis.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Name != "renamed by the other writer" {
		t.Errorf("the PUT overwrote the other writer: name is %q", stored.Name)
	}
}
