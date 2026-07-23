package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAllMessageTypes pins the exact set and values of WebSocket message types.
// It is the Go-side anchor of the cross-boundary contract: the frontend manifest
// (ui/src/api/metricsMessages.ts) is asserted equal to this set by
// TestMessageTypeManifestParity, so a change here forces a frontend change.
func TestAllMessageTypes(t *testing.T) {
	assert.Equal(t, []string{
		"initial_metrics",
		"metrics_update",
		"alert",
		"device_status_change",
		"drift_detected",
	}, AllMessageTypes())

	// Constant values are the wire strings and must not drift.
	assert.Equal(t, "initial_metrics", MessageTypeInitialMetrics)
	assert.Equal(t, "metrics_update", MessageTypeMetricsUpdate)
	assert.Equal(t, "alert", MessageTypeAlert)
	assert.Equal(t, "device_status_change", MessageTypeDeviceStatusChange)
	assert.Equal(t, "drift_detected", MessageTypeDriftDetected)

	// No duplicates.
	seen := map[string]bool{}
	for _, mt := range AllMessageTypes() {
		require.False(t, seen[mt], "duplicate message type %q", mt)
		seen[mt] = true
	}
}

// TestDashboardUpdateBuilder asserts initial_metrics and metrics_update carry a
// DashboardMetrics snapshot and differ only by type.
func TestDashboardUpdateBuilder(t *testing.T) {
	metrics := &DashboardMetrics{}

	initial := newDashboardUpdate(MessageTypeInitialMetrics, metrics)
	assert.Equal(t, MessageTypeInitialMetrics, initial.Type)
	assert.Same(t, metrics, initial.Data)
	assert.False(t, initial.Timestamp.IsZero())

	update := newDashboardUpdate(MessageTypeMetricsUpdate, metrics)
	assert.Equal(t, MessageTypeMetricsUpdate, update.Type)
	assert.Same(t, metrics, update.Data)
}

// TestAlertUpdateBuilder asserts the alert payload shape.
func TestAlertUpdateBuilder(t *testing.T) {
	u := newAlertUpdate("drift_detected", "Drift on Dev", "warning")
	assert.Equal(t, MessageTypeAlert, u.Type)
	assert.False(t, u.Timestamp.IsZero())

	data, ok := u.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "drift_detected", data["alert_type"])
	assert.Equal(t, "Drift on Dev", data["message"])
	assert.Equal(t, "warning", data["severity"])
	assert.ElementsMatch(t, []string{"alert_type", "message", "severity"}, keysOf(data))
}

// TestDeviceStatusChangeUpdateBuilder asserts the device_status_change payload shape.
func TestDeviceStatusChangeUpdateBuilder(t *testing.T) {
	u := newDeviceStatusChangeUpdate("42", "Living Room", "online", "offline")
	assert.Equal(t, MessageTypeDeviceStatusChange, u.Type)

	data, ok := u.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "42", data["device_id"])
	assert.Equal(t, "Living Room", data["device_name"])
	assert.Equal(t, "online", data["old_status"])
	assert.Equal(t, "offline", data["new_status"])
	assert.NotNil(t, data["timestamp"])
	assert.ElementsMatch(t,
		[]string{"device_id", "device_name", "old_status", "new_status", "timestamp"},
		keysOf(data))
}

// TestDriftDetectedUpdateBuilder asserts the drift_detected payload shape.
func TestDriftDetectedUpdateBuilder(t *testing.T) {
	u := newDriftDetectedUpdate("42", "Living Room", 3, "high")
	assert.Equal(t, MessageTypeDriftDetected, u.Type)

	data, ok := u.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "42", data["device_id"])
	assert.Equal(t, "Living Room", data["device_name"])
	assert.Equal(t, 3, data["drift_count"])
	assert.Equal(t, "high", data["severity"])
	assert.NotNil(t, data["timestamp"])
	assert.ElementsMatch(t,
		[]string{"device_id", "device_name", "drift_count", "severity", "timestamp"},
		keysOf(data))
}

func keysOf(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
