//go:build linux

package provisioning

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// nmcliStandIn puts a stand-in nmcli first on PATH. It answers from the environment, so no value is put
// into the script text, and it logs every call. No real Wi-Fi device is read or changed. It returns the
// path of the call log.
func nmcliStandIn(t *testing.T, deviceStatus, rescanError, networkList string) string {
	t.Helper()

	dir := t.TempDir()
	callLog := filepath.Join(dir, "calls.log")
	script := `#!/bin/sh
echo "$@" >> "$NMCLI_CALL_LOG"
case "$*" in
  *"device status") printf '%s\n' "$NMCLI_DEVICE_STATUS" ;;
  *"device wifi rescan")
    if [ -n "$NMCLI_RESCAN_ERROR" ]; then echo "$NMCLI_RESCAN_ERROR" >&2; exit 1; fi ;;
  *"device wifi list") printf '%s\n' "$NMCLI_NETWORK_LIST" ;;
esac
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "nmcli"), []byte(script), 0o755))

	t.Setenv("PATH", dir)
	t.Setenv("NMCLI_CALL_LOG", callLog)
	t.Setenv("NMCLI_DEVICE_STATUS", deviceStatus)
	t.Setenv("NMCLI_RESCAN_ERROR", rescanError)
	t.Setenv("NMCLI_NETWORK_LIST", networkList)
	return callLog
}

func standInCalls(t *testing.T, callLog string) string {
	t.Helper()
	calls, err := os.ReadFile(callLog)
	require.NoError(t, err, "the stand-in nmcli was never called, so the case proves nothing")
	return string(calls)
}

func newTestLinuxInterface(t *testing.T) *LinuxNetworkInterface {
	t.Helper()
	logger, err := logging.New(logging.Config{Level: "error", Format: "text", Output: "stdout"})
	require.NoError(t, err)
	return NewLinuxNetworkInterface(logger)
}

const scanRefused = "Error: Scanning not allowed while unavailable"

// A device NetworkManager cannot scan with is an error, not an empty list. The scan is not even tried.
func TestGetAvailableNetworks_RefusesADeviceThatCannotScan(t *testing.T) {
	cases := []struct {
		name         string
		deviceStatus string
		wantState    string
	}{
		{"unavailable", "wlo1:wifi:unavailable", "wlo1"},
		{"unmanaged", "wlo1:wifi:unmanaged", "wlo1"},
		{"no Wi-Fi device", "eno1:ethernet:connected", "no Wi-Fi device"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			callLog := nmcliStandIn(t, tc.deviceStatus, scanRefused, "")

			networks, err := newTestLinuxInterface(t).GetAvailableNetworks(context.Background())

			require.Error(t, err)
			assert.Nil(t, networks)
			assert.Contains(t, err.Error(), tc.wantState)
			if tc.name == "unmanaged" {
				// The radio can be on: the hint is about NetworkManager ignoring the device.
				assert.Contains(t, err.Error(), "nmcli device set wlo1 managed yes")
				assert.NotContains(t, err.Error(), "rfkill")
			} else {
				assert.Contains(t, err.Error(), "rfkill list")
				assert.Contains(t, err.Error(), "nmcli radio wifi")
			}
			if tc.name != "no Wi-Fi device" {
				assert.Contains(t, err.Error(), tc.name)
			}

			calls := standInCalls(t, callLog)
			assert.NotContains(t, calls, "rescan", "a device that cannot scan must not be asked to")
			assert.NotContains(t, calls, "wifi list")
		})
	}
}

// "disconnected" is the radio on and joined to nothing: the normal state of a host that scans.
func TestGetAvailableNetworks_ScansADisconnectedDevice(t *testing.T) {
	callLog := nmcliStandIn(t, "wlo1:wifi:disconnected", "", "ShellyPlug-AABBCC:WPA2:70:6:2437")

	networks, err := newTestLinuxInterface(t).GetAvailableNetworks(context.Background())

	require.NoError(t, err)
	require.Len(t, networks, 1)
	assert.Equal(t, "ShellyPlug-AABBCC", networks[0].SSID)
	assert.Contains(t, standInCalls(t, callLog), "device wifi rescan", "control: the scan ran")
}

// A rescan that nmcli refuses right after another one is not a dead radio: the list is still read.
func TestGetAvailableNetworks_AFailedRescanOnAUsableDeviceStillReadsTheList(t *testing.T) {
	nmcliStandIn(t, "wlo1:wifi:connected", "Error: Scanning not allowed immediately following previous scan", "HomeNet:WPA2:80:1:2412")

	networks, err := newTestLinuxInterface(t).GetAvailableNetworks(context.Background())

	require.NoError(t, err)
	require.Len(t, networks, 1)
}

// A good scan that sees nothing is an empty list and no error; the caller decides how to say it.
func TestGetAvailableNetworks_AGoodScanWithNoNetworksIsEmpty(t *testing.T) {
	nmcliStandIn(t, "wlo1:wifi:disconnected", "", "")

	networks, err := newTestLinuxInterface(t).GetAvailableNetworks(context.Background())

	require.NoError(t, err)
	assert.Empty(t, networks)
}

// nmcli prints its states in the caller's language unless it is told otherwise. The stand-in answers in
// Dutch when LC_ALL is not C.UTF-8, so a read that does not set it sees "niet beschikbaar" and calls the device
// usable.
func TestGetInterfaceInfo_ReadsTheStateInTheCLocale(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\nif [ \"$LC_ALL\" = C.UTF-8 ]; then printf 'wlo1:wifi:unavailable\\n'; else printf 'wlo1:wifi:niet beschikbaar\\n'; fi\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "nmcli"), []byte(script), 0o755))
	t.Setenv("PATH", dir)
	t.Setenv("LC_ALL", "nl_BE.UTF-8")

	info := newTestLinuxInterface(t).GetInterfaceInfo()

	assert.Equal(t, "unavailable", info.Status)
	assert.NotEmpty(t, info.Problem())

	_, err := newTestLinuxInterface(t).GetAvailableNetworks(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unavailable")
}

// dutchNmcli puts a stand-in nmcli on PATH that answers in English when LC_ALL is C or C.UTF-8 and in
// Dutch otherwise ("geactiveerd", "niet beschikbaar"). Under plain C it also prints a non-ASCII profile
// name with "?" for each such character, as nmcli does. It logs LC_ALL with the arguments of every call.
// The parent environment is Dutch. It returns the call log path.
func dutchNmcli(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	callLog := filepath.Join(dir, "calls.log")
	script := `#!/bin/sh
echo "LC_ALL=$LC_ALL $*" >> "$NMCLI_CALL_LOG"
name=${NMCLI_ACTIVE_NAME-HomeNet}
case "$LC_ALL" in
  C|C.UTF-8) active=activated; dev=disconnected ;;
  *) active=geactiveerd; dev='niet beschikbaar' ;;
esac
if [ "$LC_ALL" != C.UTF-8 ]; then case "$name" in *é*) name="${name%é*}?${name#*é}" ;; esac; fi
case "$*" in
  *"device status") printf 'wlo1:wifi:%s\n' "$dev" ;;
  *"device wifi list") printf 'HomeNet:WPA2:70:6:2437\n' ;;
  *"-f NAME connection show") printf '%s\n' "${NMCLI_PROFILES-HomeNet}" ;;
  *"connection show --active") printf '%s:802-11-wireless:%s\n' "$name" "$active" ;;
  *"connection show HomeNet") printf '802-11-wireless.ssid:HomeNet\n802-11-wireless.security:wpa-psk\n' ;;
esac
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "nmcli"), []byte(script), 0o755))
	t.Setenv("PATH", dir)
	t.Setenv("NMCLI_CALL_LOG", callLog)
	t.Setenv("LC_ALL", "nl_BE.UTF-8")
	return callLog
}

func TestGetCurrentNetwork_ReadsAnActiveConnectionUnderANonEnglishLocale(t *testing.T) {
	dutchNmcli(t)

	network, err := newTestLinuxInterface(t).GetCurrentNetwork(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "HomeNet", network.SSID)
	assert.Equal(t, "wpa-psk", network.Security)
}

func TestDisconnectFromNetwork_BringsDownTheActiveConnectionUnderANonEnglishLocale(t *testing.T) {
	callLog := dutchNmcli(t)

	require.NoError(t, newTestLinuxInterface(t).DisconnectFromNetwork(context.Background()))

	assert.Contains(t, standInCalls(t, callLog), "connection down HomeNet")
}

// LC_ALL=C makes nmcli print every non-ASCII character of a profile name as "?", so the disconnect would
// bring down a profile that does not exist and report success. The locale must keep UTF-8.
func TestDisconnectFromNetwork_BringsDownAProfileWithANonASCIIName(t *testing.T) {
	callLog := dutchNmcli(t)
	t.Setenv("NMCLI_ACTIVE_NAME", "Café")

	require.NoError(t, newTestLinuxInterface(t).DisconnectFromNetwork(context.Background()))

	assert.Contains(t, standInCalls(t, callLog), "connection down Café")
}

// Every nmcli call of the file runs in the C.UTF-8 locale, because the file compares nmcli's words in English.
// The case calls every method that runs nmcli and reads the locale of each call from the log.
func TestEveryNmcliCallRunsInTheCLocale(t *testing.T) {
	callLog := dutchNmcli(t)
	ni := newTestLinuxInterface(t)
	ctx := context.Background()

	ni.GetInterfaceInfo()
	_, _ = ni.GetAvailableNetworks(ctx)
	_, _ = ni.GetCurrentNetwork(ctx)
	_, _ = ni.IsConnected(ctx, "HomeNet")
	_ = ni.DisconnectFromNetwork(ctx)
	// All four connect branches: an open or a secured network, with and without a stored profile.
	for _, profiles := range []string{"HomeNet", "OtherNet"} {
		t.Setenv("NMCLI_PROFILES", profiles)
		for _, password := range []string{"", "secret"} {
			_ = ni.ConnectToNetwork(ctx, "HomeNet", password)
		}
	}

	calls := standInCalls(t, callLog)
	// Control: the calls that parse a translated word, and every connect path, all ran.
	for _, want := range []string{"device status", "device wifi rescan", "device wifi list", "-f NAME connection show", "connection show --active", "802-11-wireless.ssid", "connection down", "connection up HomeNet", "device wifi connect HomeNet\n", "device wifi connect HomeNet password secret"} {
		assert.Contains(t, calls, want, "control: the case did not reach %q", want)
	}
	for _, line := range strings.Split(strings.TrimSpace(calls), "\n") {
		assert.True(t, strings.HasPrefix(line, "LC_ALL=C.UTF-8 "), "nmcli ran in the caller's locale: %q", line)
	}
}

// A hung nmcli must not hang the state read: it ends in the "error" state.
func TestGetInterfaceInfo_AHungNmcliEndsInTheErrorState(t *testing.T) {
	dir := t.TempDir()
	// A busy loop, not sleep: PATH holds only this directory.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "nmcli"), []byte("#!/bin/sh\nwhile :; do :; done\n"), 0o755))
	t.Setenv("PATH", dir)
	saved := interfaceInfoTimeout
	interfaceInfoTimeout = 300 * time.Millisecond
	t.Cleanup(func() { interfaceInfoTimeout = saved })

	done := make(chan NetworkInterfaceInfo, 1)
	go func() { done <- newTestLinuxInterface(t).GetInterfaceInfo() }()

	select {
	case info := <-done:
		assert.Equal(t, "error", info.Status)
		assert.Contains(t, info.Problem(), "cannot be read")
	case <-time.After(10 * time.Second):
		t.Fatal("GetInterfaceInfo is still waiting for a hung nmcli")
	}
}

// Discovery uses the same read, so an unusable radio fails it with the reason and never reports "found 0
// networks".
func TestDiscoverUnprovisionedDevices_FailsWithTheReasonWhenTheDeviceCannotScan(t *testing.T) {
	nmcliStandIn(t, "wlo1:wifi:unavailable", scanRefused, "")
	logger, err := logging.New(logging.Config{Level: "error", Format: "text", Output: "stdout"})
	require.NoError(t, err)
	sp := NewShellyProvisioner(logger, NewLinuxNetworkInterface(logger))

	devices, err := sp.DiscoverUnprovisionedDevices(context.Background())

	require.Error(t, err)
	assert.Nil(t, devices)
	assert.True(t, strings.Contains(err.Error(), "unavailable"), "the error must carry the device state: %v", err)
}
