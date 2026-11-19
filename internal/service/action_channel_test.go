package service

import (
	"errors"
	"net"
	"testing"

	"github.com/ginsys/shelly-manager/internal/database"
)

// The channel of an on, off or toggle action picks the relay the device switches, so a value that is
// not a whole number of 0 or more is refused instead of switching a neighbouring relay.
func TestActionChannel(t *testing.T) {
	tests := []struct {
		name    string
		params  map[string]interface{}
		want    int
		wantErr bool
	}{
		{"absent", map[string]interface{}{}, 0, false},
		{"nil params", nil, 0, false},
		{"null", map[string]interface{}{"channel": nil}, 0, false},
		{"whole float", map[string]interface{}{"channel": float64(2)}, 2, false},
		{"whole zero", map[string]interface{}{"channel": float64(0)}, 0, false},
		{"go int", map[string]interface{}{"channel": 1}, 1, false},
		{"go int zero", map[string]interface{}{"channel": 0}, 0, false},
		{"fraction", map[string]interface{}{"channel": 1.5}, 0, true},
		{"fraction below one", map[string]interface{}{"channel": 0.5}, 0, true},
		{"out of range high", map[string]interface{}{"channel": 1e300}, 0, true},
		{"out of range low", map[string]interface{}{"channel": -1e300}, 0, true},
		{"one past the exact integers", map[string]interface{}{"channel": float64(9007199254740992)}, 0, true},
		{"negative float", map[string]interface{}{"channel": float64(-1)}, 0, true},
		{"negative int", map[string]interface{}{"channel": -1}, 0, true},
		{"string", map[string]interface{}{"channel": "1"}, 0, true},
		{"bool", map[string]interface{}{"channel": true}, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := actionChannel(tt.params)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidChannel) {
					t.Fatalf("want ErrInvalidChannel, got channel %d and error %v", got, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got channel %d, want %d", got, tt.want)
			}
		})
	}
}

// A bad channel is refused before any device call: the device here has no reachable address and the
// force flag skips the offline pre-check, so a request that got as far as the device would fail with a
// connection error, never with ErrInvalidChannel.
func TestShellyService_ControlDevice_BadChannelIsRefusedBeforeAnyDeviceCall(t *testing.T) {
	if ln, err := net.Listen("tcp4", "127.0.0.1:0"); err != nil {
		t.Skipf("Skipping due to restricted socket permissions: %v", err)
	} else {
		_ = ln.Close()
	}

	db := createTestDB(t)
	svc := NewService(db, createTestConfigBusiness())
	device := &database.Device{
		IP:       "127.0.0.1:1",
		MAC:      "68C63A123470",
		Type:     "SHSW-25",
		Name:     "Unreachable",
		Status:   "offline",
		Settings: `{"model":"SHSW-25","gen":1,"auth_enabled":false}`,
	}
	if err := db.AddDevice(device); err != nil {
		t.Fatalf("Failed to create test device: %v", err)
	}

	for _, action := range []string{"on", "off", "toggle"} {
		for _, channel := range []interface{}{1.5, float64(-1), -1, "1", 1e300} {
			err := svc.ControlDevice(device.ID, action, map[string]interface{}{"channel": channel, "force": true})
			if !errors.Is(err, ErrInvalidChannel) {
				t.Errorf("%s with channel %v: want ErrInvalidChannel, got %v", action, channel, err)
			}
		}
	}

	// A good channel gets past the check and fails later, at the device.
	err := svc.ControlDevice(device.ID, "on", map[string]interface{}{"channel": float64(1), "force": true})
	if err == nil || errors.Is(err, ErrInvalidChannel) {
		t.Errorf("a whole channel should reach the device and fail there, got %v", err)
	}

	// The offline pre-check still comes first, and reboot has no channel to check.
	err = svc.ControlDevice(device.ID, "on", map[string]interface{}{"channel": 1.5})
	if !errors.Is(err, ErrDeviceOffline) {
		t.Errorf("an offline device answers ErrDeviceOffline before the channel is read, got %v", err)
	}
	err = svc.ControlDevice(device.ID, "reboot", map[string]interface{}{"channel": 1.5, "force": true})
	if errors.Is(err, ErrInvalidChannel) {
		t.Errorf("reboot takes no channel, got %v", err)
	}
}
