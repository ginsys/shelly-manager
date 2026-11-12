package service

import (
	"errors"
	"testing"
	"time"

	"github.com/ginsys/shelly-manager/internal/database"
)

// A device PUT compares updated_at and then saves. A writer that does not take clientMu can commit
// between the two (ginsys/shelly-manager#509). The save itself must refuse in that case.

// racingDB runs hook once, right after the first GetDevice returns. That is the instant between
// the compare of the PUT and its save.
type racingDB struct {
	database.DatabaseInterface
	hook  func()
	fired bool
}

func (r *racingDB) GetDevice(id uint) (*database.Device, error) {
	device, err := r.DatabaseInterface.GetDevice(id)
	if !r.fired && r.hook != nil {
		r.fired = true
		r.hook()
	}
	return device, err
}

func TestUpdateDeviceKeepingLogin_RefusesAWriterThatCommitsBetweenCompareAndSave(t *testing.T) {
	gate := &gatedDevice{accept: map[string]bool{"old": true}}
	service, _ := retryFixture(t, gate)
	real := service.DB

	devices, err := real.GetDevices()
	if err != nil || len(devices) != 1 {
		t.Fatalf("expected one device, got %d (%v)", len(devices), err)
	}
	read, err := real.GetDevice(devices[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	// The client read this row. Its updated_at is the version of the PUT.
	basis := *read
	incoming := *read
	incoming.Name = "renamed by the PUT"

	racer := &racingDB{DatabaseInterface: real}
	racer.hook = func() {
		// Another whole-row writer saves after the compare reads the row.
		time.Sleep(20 * time.Millisecond)
		other, getErr := real.GetDevice(read.ID)
		if getErr != nil {
			t.Error(getErr)
			return
		}
		other.Name = "renamed by the other writer"
		if updateErr := real.UpdateDevice(other); updateErr != nil {
			t.Error(updateErr)
		}
	}
	service.DB = racer

	err = service.UpdateDeviceKeepingLogin(&basis, &incoming)
	if !errors.Is(err, ErrDeviceChanged) {
		t.Fatalf("the save answered %v, want ErrDeviceChanged", err)
	}
	stored, err := real.GetDevice(read.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Name != "renamed by the other writer" {
		t.Errorf("the PUT overwrote the other writer: name is %q", stored.Name)
	}
}
