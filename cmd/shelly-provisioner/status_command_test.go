//go:build linux

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The real `status` command, built and run with a stand-in nmcli on PATH, never says the device is
// available when NetworkManager lists it as unavailable. Start-up prints its own lines before checkStatus
// runs, so this case reads the whole output. No real Wi-Fi device is read or changed.
func TestStatusCommand_NeverSaysAvailableForAnUnavailableDevice(t *testing.T) {
	goBin, err := exec.LookPath("go")
	require.NoError(t, err)
	binary := filepath.Join(t.TempDir(), "shelly-provisioner")
	build := exec.Command(goBin, "build", "-o", binary, ".")
	buildOut, err := build.CombinedOutput()
	require.NoError(t, err, string(buildOut))
	config, err := filepath.Abs(filepath.Join("..", "..", "configs", "shelly-provisioner.yaml"))
	require.NoError(t, err)

	shimDir := t.TempDir()
	callLog := filepath.Join(shimDir, "calls.log")
	// The values go in through the environment: a value put into the script text would be read as shell code.
	shim := "#!/bin/sh\necho \"$@\" >> \"$NMCLI_CALL_LOG\"\ncase \"$*\" in\n  *\"device status\") printf '%s\\n' \"$NMCLI_DEVICE_STATUS\" ;;\n  *) exit 1 ;;\nesac\n"
	require.NoError(t, os.WriteFile(filepath.Join(shimDir, "nmcli"), []byte(shim), 0o755))

	cmd := exec.Command(binary, "--config", config, "status")
	cmd.Env = []string{"PATH=" + shimDir, "NMCLI_CALL_LOG=" + callLog, "NMCLI_DEVICE_STATUS=wlo1:wifi:unavailable"}
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))

	assert.Contains(t, string(out), "wlo1 is unavailable")
	assert.NotContains(t, string(out), "Available")
	assert.NotContains(t, string(out), "Working")

	calls, err := os.ReadFile(callLog)
	require.NoError(t, err, "the stand-in nmcli was never called, so the case proves nothing")
	assert.NotContains(t, string(calls), "rescan")
}
