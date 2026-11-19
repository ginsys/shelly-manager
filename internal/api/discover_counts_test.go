package api

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/discovery"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// ginsys/shelly-manager#707: the discover handler counted a new device only when its own insert
// worked. The service had stored the device already, so the insert failed on the unique MAC and
// "new_devices" read 0 for every device. The count now says what the pass added.
func TestDiscoverHandler_CountsNewDevices(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	svc := testShellyService(t, db)
	svc.SetDiscoverFunc(func(context.Context, []string, time.Duration) ([]discovery.ShellyDevice, error) {
		return []discovery.ShellyDevice{{
			ID: "shelly1-707", MAC: "68C63A000707", Model: "SHSW-25", Generation: 1, Version: "1.14.0",
			// The handler imports the configuration of each device it finds. A closed loopback port
			// refuses at once and sends nothing off this host.
			IP: "127.0.0.1:1", Discovered: time.Now(),
		}}, nil
	})

	logFile := filepath.Join(t.TempDir(), "api.log")
	logger, err := logging.New(logging.Config{Level: "info", Format: "json", Output: logFile})
	require.NoError(t, err)
	handler := NewHandlerWithLogger(db, svc, testNotificationHandler(t, db), nil, logger)
	defer handler.WaitBackground()

	for range 2 {
		handler.DiscoverHandler(httptest.NewRecorder(), httptest.NewRequest("POST", "/api/v1/discover", nil))
		handler.WaitBackground()
	}

	require.Equal(t, []float64{1, 0}, newDevicesLogged(t, logFile),
		"new_devices of the two passes: the first adds the device, the second finds it known")
}

// newDevicesLogged returns the new_devices member of each "Discovery processing completed" line.
func newDevicesLogged(t *testing.T, path string) []float64 {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	var counts []float64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if !strings.Contains(scanner.Text(), "Discovery processing completed") {
			continue
		}
		var line map[string]any
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &line), scanner.Text())
		n, ok := line["new_devices"].(float64)
		require.True(t, ok, "the line has no numeric new_devices: %s", scanner.Text())
		counts = append(counts, n)
	}
	require.NoError(t, scanner.Err())
	return counts
}
