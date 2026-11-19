package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ginsys/shelly-manager/internal/discovery"
)

// fakeDevice is the one device that the fake discovery and the fake probe know.
func fakeDevice() discovery.ShellyDevice {
	return discovery.ShellyDevice{
		ID: "shelly1-585", MAC: "68C63A000585", Model: "SHSW-25", Generation: 1, Version: "1.14.0",
		IP: "192.0.2.58", Discovered: time.Now(),
	}
}

// fakeDiscovery makes discovery find the fake device, and the probe answer for its address only,
// so that no network is touched.
func fakeDiscovery(t *testing.T) {
	t.Helper()
	shellyService.SetDiscoverFunc(func(context.Context, []string, time.Duration) ([]discovery.ShellyDevice, error) {
		return []discovery.ShellyDevice{fakeDevice()}, nil
	})
	shellyService.SetProbeFunc(func(_ context.Context, host string, _ time.Duration) (*discovery.ShellyDevice, error) {
		if d := fakeDevice(); host == d.IP {
			return &d, nil
		}
		return nil, nil
	})
}

// captureStdout runs fn and returns what it printed. The commands print with fmt.Printf.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()
	func() {
		defer func() { os.Stdout = orig }()
		fn()
		_ = w.Close()
	}()
	return <-done
}

func useTestEnvironment(t *testing.T) {
	t.Helper()
	origDB, origService, origCfg := dbManager, shellyService, cfg
	t.Cleanup(func() { dbManager, shellyService, cfg = origDB, origService, origCfg })
	dbManager, shellyService, cfg = createTestEnvironment(t)
	fakeDiscovery(t)
}

// ginsys/shelly-manager#585: a device that this run added printed "Already in database",
// because the lookup ran after the discovery had stored it.
func TestDiscoverCommand_SaysAddedForANewDevice(t *testing.T) {
	useTestEnvironment(t)

	first := captureStdout(t, func() { discoverCmd.Run(discoverCmd, []string{"192.0.2.0/24"}) })
	if !strings.Contains(first, "Added to database") || strings.Contains(first, "Already in database") {
		t.Errorf("a run over a new device must say it was added:\n%s", first)
	}

	second := captureStdout(t, func() { discoverCmd.Run(discoverCmd, []string{"192.0.2.0/24"}) })
	if !strings.Contains(second, "Already in database") || strings.Contains(second, "Added to database") {
		t.Errorf("a run over a known device must say it was known:\n%s", second)
	}
}

// The add command found the device through the discovery, which stored it, and then created it
// a second time, so every call failed.
func TestAddDevice_AddsADiscoveredDeviceAndAppliesTheName(t *testing.T) {
	useTestEnvironment(t)

	var out bytes.Buffer
	if err := runAdd(&out, "192.0.2.58", "Garage"); err != nil {
		t.Fatalf("runAdd: %v", err)
	}
	if !strings.Contains(out.String(), "Added device: Garage") {
		t.Errorf("output does not name the added device:\n%s", out.String())
	}
	stored, err := dbManager.GetDeviceByMAC("68C63A000585")
	if err != nil {
		t.Fatalf("the device is not stored: %v", err)
	}
	if stored.Name != "Garage" {
		t.Errorf("stored name = %q, want Garage", stored.Name)
	}
}

func TestAddDevice_AKnownDeviceIsReportedAndKeepsItsName(t *testing.T) {
	useTestEnvironment(t)

	if err := runAdd(io.Discard, "192.0.2.58", "Garage"); err != nil {
		t.Fatalf("first runAdd: %v", err)
	}
	var out bytes.Buffer
	if err := runAdd(&out, "192.0.2.58", "Other"); err != nil {
		t.Fatalf("second runAdd: %v", err)
	}
	if !strings.Contains(out.String(), "already known") || strings.Contains(out.String(), "Added device") {
		t.Errorf("a known device must be reported as known:\n%s", out.String())
	}
	stored, err := dbManager.GetDeviceByMAC("68C63A000585")
	if err != nil {
		t.Fatalf("GetDeviceByMAC: %v", err)
	}
	if stored.Name != "Garage" {
		t.Errorf("add over a known device renamed it to %q", stored.Name)
	}
}

// add stores the device at the address it was given and no other: the discovery finds every
// device on the network (mDNS) whatever address was asked for, so add must not use it.
func TestAddDevice_StoresOnlyTheDeviceAtTheAddress(t *testing.T) {
	useTestEnvironment(t)
	other := fakeDevice()
	other.MAC, other.IP = "68C63A000586", "192.0.2.59"
	discovered := 0
	shellyService.SetDiscoverFunc(func(context.Context, []string, time.Duration) ([]discovery.ShellyDevice, error) {
		discovered++
		return []discovery.ShellyDevice{fakeDevice(), other}, nil
	})

	if err := runAdd(io.Discard, "192.0.2.58", ""); err != nil {
		t.Fatalf("runAdd: %v", err)
	}
	if discovered != 0 {
		t.Errorf("add ran the network discovery %d times; it must probe the one address", discovered)
	}
	if _, err := dbManager.GetDeviceByMAC("68C63A000585"); err != nil {
		t.Errorf("the device at the address is not stored: %v", err)
	}
	if _, err := dbManager.GetDeviceByMAC("68C63A000586"); err == nil {
		t.Error("a device at another address was stored")
	}
}

func TestAddDevice_NoDeviceAtTheAddressIsAnError(t *testing.T) {
	useTestEnvironment(t)

	var out bytes.Buffer
	if err := runAdd(&out, "192.0.2.77", "Garage"); err == nil {
		t.Errorf("runAdd must fail when no device answers at the address, printed:\n%s", out.String())
	}
	if _, err := dbManager.GetDeviceByMAC("68C63A000585"); err == nil {
		t.Error("a device was stored although none answered at the address")
	}
}

func TestAddDevice_ANonAddressIsRefusedBeforeAnyProbe(t *testing.T) {
	useTestEnvironment(t)
	probed := false
	shellyService.SetProbeFunc(func(context.Context, string, time.Duration) (*discovery.ShellyDevice, error) {
		probed = true
		return nil, nil
	})
	if err := runAdd(io.Discard, "invalid-ip", ""); err == nil {
		t.Error("runAdd over text that is not an IP address must fail")
	}
	if probed {
		t.Error("a name that is not an IP address was probed")
	}
}
