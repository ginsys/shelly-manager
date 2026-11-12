package database

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/database/provider"
	"github.com/ginsys/shelly-manager/internal/logging"
)

// The database logger at info level prints every SQL statement. Without
// parameterized queries it prints the bound values too, and devices.settings
// holds the clear-text device login (ginsys/shelly-manager#489).
func TestDeviceLoginSaveKeepsSecretsOutOfSQLLog(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")

	logger, err := logging.New(logging.Config{Level: "debug", Format: "text", Output: logPath})
	require.NoError(t, err)

	manager, err := NewManagerWithLogger(provider.DatabaseConfig{
		Provider: "sqlite",
		DSN:      filepath.Join(dir, "test.db"),
		LogLevel: "info",
	}, logger)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = manager.Close()
		_ = logger.Close()
	})

	const (
		userSecret   = "login-user-9f3a1c"
		passSecret   = "login-pass-7d2e4b"
		nestedSecret = "nested-pass-5c8a0e"
	)
	settings := `{"auth_user":"` + userSecret + `","auth_pass":"` + passSecret +
		`","auth":{"password":"` + nestedSecret + `"}}`

	device := &Device{
		IP:       "192.0.2.10",
		MAC:      "AA:BB:CC:00:00:01",
		Type:     "SHSW-1",
		Name:     "secret-test",
		Settings: settings,
	}
	require.NoError(t, manager.AddDevice(device))

	device.Settings = strings.Replace(settings, userSecret, userSecret+"-2", 1)
	require.NoError(t, manager.UpdateDevice(device))

	raw, err := os.ReadFile(logPath)
	require.NoError(t, err)
	text := string(raw)

	// Control: the SQL trace is on, so a missing secret means redaction and
	// not a silent logger.
	require.Contains(t, text, "devices", "the info-level SQL trace did not run")

	for _, secret := range []string{userSecret, passSecret, nestedSecret} {
		// Boolean form: a failure must not print the whole log back.
		assert.False(t, strings.Contains(text, secret), "the SQL log holds a device login value")
	}
}
