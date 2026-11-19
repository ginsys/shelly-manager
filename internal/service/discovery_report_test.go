package service

import (
	"context"
	"testing"
	"time"

	"github.com/ginsys/shelly-manager/internal/discovery"
)

// ginsys/shelly-manager#585: a pass reports for each device whether it created the row, so a
// caller does not look the device up after the pass stored it.
func TestDiscoverDevicesReport_SaysWhichDevicesThePassAdded(t *testing.T) {
	db := createTestDB(t)
	service := NewService(db, createTestConfig())
	found := []discovery.ShellyDevice{{
		ID: "shelly1-585a", MAC: "68C63A000585", Model: "SHSW-25", Generation: 1, Version: "1.14.0",
		IP: "192.0.2.31", Discovered: time.Now(),
	}}
	service.SetDiscoverFunc(func(context.Context, []string, time.Duration) ([]discovery.ShellyDevice, error) {
		return found, nil
	})

	report, err := service.DiscoverDevicesReport("auto")
	if err != nil || len(report) != 1 {
		t.Fatalf("first pass: %v, %d devices", err, len(report))
	}
	if !report[0].Added {
		t.Error("a device the pass created must be reported as added")
	}

	// A second device joins the known one: only the new one is added.
	found = append(found, discovery.ShellyDevice{
		ID: "shelly1-585b", MAC: "68C63A000586", Model: "SHSW-25", Generation: 1, Version: "1.14.0",
		IP: "192.0.2.32", Discovered: time.Now(),
	})
	report, err = service.DiscoverDevicesReport("auto")
	if err != nil || len(report) != 2 {
		t.Fatalf("second pass: %v, %d devices", err, len(report))
	}
	added := map[string]bool{}
	for _, d := range report {
		added[d.MAC] = d.Added
	}
	if added["68C63A000585"] {
		t.Error("a device the first pass stored must be reported as known")
	}
	if !added["68C63A000586"] {
		t.Error("a device the second pass created must be reported as added")
	}

	// DiscoverDevices answers with the same devices and no flag.
	plain, err := service.DiscoverDevices("auto")
	if err != nil || len(plain) != 2 {
		t.Fatalf("DiscoverDevices: %v, %d devices", err, len(plain))
	}
}
