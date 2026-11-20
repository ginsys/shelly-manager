package main

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/provisioning"
)

// statusInterface answers the two calls `status` makes. The embedded interface is nil on purpose: any
// other call panics, which would show `status` doing more than it should.
type statusInterface struct {
	provisioning.NetworkInterface
	info     provisioning.NetworkInterfaceInfo
	networks []provisioning.WiFiNetwork
	scanErr  error
	scans    int
}

func (s *statusInterface) GetInterfaceInfo() provisioning.NetworkInterfaceInfo { return s.info }

func (s *statusInterface) GetAvailableNetworks(context.Context) ([]provisioning.WiFiNetwork, error) {
	s.scans++
	return s.networks, s.scanErr
}

// runStatus runs checkStatus against a fake interface and returns what it printed.
func runStatus(t *testing.T, iface *statusInterface) string {
	t.Helper()

	l, err := logging.New(logging.Config{Level: "error", Format: "text", Output: "stderr"})
	require.NoError(t, err)
	savedIface, savedLogger, savedURL := netInterface, logger, apiURL
	netInterface, logger, apiURL = iface, l, ""
	t.Cleanup(func() { netInterface, logger, apiURL = savedIface, savedLogger, savedURL })

	r, w, err := os.Pipe()
	require.NoError(t, err)
	savedStdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = savedStdout })

	checkStatus()

	require.NoError(t, w.Close())
	os.Stdout = savedStdout
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(out)
}

func wifiInfo(name, status string) provisioning.NetworkInterfaceInfo {
	return provisioning.NetworkInterfaceInfo{Name: name, Type: "wireless", Status: status}
}

// A device that cannot scan is a failure line with the device, its state and the likely cause, and the
// scan line never says "Working".
func TestCheckStatus_ADeviceThatCannotScanIsAFailure(t *testing.T) {
	cases := []struct {
		name string
		info provisioning.NetworkInterfaceInfo
		want string
	}{
		{"unavailable", wifiInfo("wlo1", "unavailable"), "wlo1 is unavailable"},
		{"unmanaged", wifiInfo("wlo1", "unmanaged"), "wlo1 is unmanaged"},
		{"no Wi-Fi device", wifiInfo(provisioning.NoWiFiDevice, "unavailable"), "no Wi-Fi device"},
		{"state unreadable", wifiInfo("unknown", "error"), "cannot be read"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			iface := &statusInterface{info: tc.info}

			out := runStatus(t, iface)

			assert.Contains(t, out, "❌ Network Interface:")
			assert.Contains(t, out, tc.want)
			assert.NotContains(t, out, "Available")
			assert.NotContains(t, out, "Working")
			assert.Zero(t, iface.scans, "a device that cannot scan is not asked to scan")
			if tc.info.Status != "error" {
				assert.Contains(t, out, "nmcli radio wifi")
				assert.Contains(t, out, "rfkill list")
			}
		})
	}
}

// "disconnected" is the radio on and joined to nothing: the line is a tick with the state, and the scan
// decides the rest. A good scan that sees nothing is a warning, not a tick.
func TestCheckStatus_ZeroNetworksAfterAGoodScanIsAWarning(t *testing.T) {
	iface := &statusInterface{info: wifiInfo("wlo1", "disconnected")}

	out := runStatus(t, iface)

	assert.Contains(t, out, "✅ Network Interface: wlo1 (disconnected)")
	assert.Contains(t, out, "⚠️  WiFi Scanning: No networks listed")
	assert.NotContains(t, out, "Working")
	assert.Equal(t, 1, iface.scans, "control: the scan ran")
}

func TestCheckStatus_ANetworkListIsAWorkingScan(t *testing.T) {
	iface := &statusInterface{
		info:     wifiInfo("wlo1", "connected"),
		networks: []provisioning.WiFiNetwork{{SSID: "a"}, {SSID: "b"}},
	}

	out := runStatus(t, iface)

	assert.Contains(t, out, "✅ Network Interface: wlo1 (connected)")
	assert.Contains(t, out, "✅ WiFi Scanning: Working (2 networks found)")
}

func TestCheckStatus_AFailedScanOnAUsableDeviceIsAFailure(t *testing.T) {
	iface := &statusInterface{info: wifiInfo("wlo1", "connected"), scanErr: errors.New("list failed")}

	out := runStatus(t, iface)

	assert.Contains(t, out, "❌ WiFi Scanning: Failed (list failed)")
	assert.NotContains(t, out, "Working")
}
