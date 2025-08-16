package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/logging"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create test tables
	err = db.Exec(`
		CREATE TABLE IF NOT EXISTS drift_trends (
			id INTEGER PRIMARY KEY,
			severity TEXT,
			category TEXT,
			resolved BOOLEAN DEFAULT FALSE
		)
	`).Error
	if err != nil {
		t.Fatalf("Failed to create drift_trends table: %v", err)
	}

	err = db.Exec(`
		CREATE TABLE IF NOT EXISTS resolution_histories (
			id INTEGER PRIMARY KEY,
			category TEXT,
			method TEXT,
			type TEXT,
			success BOOLEAN,
			executed_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`).Error
	if err != nil {
		t.Fatalf("Failed to create resolution_histories table: %v", err)
	}

	err = db.Exec(`
		CREATE TABLE IF NOT EXISTS devices (
			id INTEGER PRIMARY KEY,
			name TEXT,
			type TEXT,
			status TEXT
		)
	`).Error
	if err != nil {
		t.Fatalf("Failed to create devices table: %v", err)
	}

	return db
}

func setupTestService(t *testing.T) (*Service, prometheus.Gatherer) {
	t.Helper()

	db := setupTestDB(t)
	logger := logging.GetDefault()
	registry := prometheus.NewRegistry()

	service := NewService(db, logger, registry)

	return service, registry
}

func TestNewService(t *testing.T) {
	db := setupTestDB(t)
	logger := logging.GetDefault()

	service := NewService(db, logger, nil)

	if service == nil {
		t.Fatal("NewService returned nil")
	}

	if !service.IsEnabled() {
		t.Error("Service should be enabled by default")
	}

	if service.db != db {
		t.Error("Service database not set correctly")
	}

	if service.logger != logger {
		t.Error("Service logger not set correctly")
	}
}

func TestServiceEnableDisable(t *testing.T) {
	service, _ := setupTestService(t)

	// Should be enabled by default
	if !service.IsEnabled() {
		t.Error("Service should be enabled by default")
	}

	// Test disable
	service.Disable()
	if service.IsEnabled() {
		t.Error("Service should be disabled after Disable()")
	}

	// Test enable
	service.Enable()
	if !service.IsEnabled() {
		t.Error("Service should be enabled after Enable()")
	}
}

func TestRecordDriftDetection(t *testing.T) {
	service, _ := setupTestService(t)

	// Record some drift detections
	service.RecordDriftDetection("drift", "SHSW-1", 100*time.Millisecond)
	service.RecordDriftDetection("synced", "SHSW-25", 150*time.Millisecond)
	service.RecordDriftDetection("error", "SHSW-1", 50*time.Millisecond)

	// Check counter values
	driftCount := testutil.ToFloat64(service.driftDetectionTotal.WithLabelValues("drift", "SHSW-1"))
	if driftCount != 1 {
		t.Errorf("Expected drift count 1, got %f", driftCount)
	}

	syncedCount := testutil.ToFloat64(service.driftDetectionTotal.WithLabelValues("synced", "SHSW-25"))
	if syncedCount != 1 {
		t.Errorf("Expected synced count 1, got %f", syncedCount)
	}

	errorCount := testutil.ToFloat64(service.driftDetectionTotal.WithLabelValues("error", "SHSW-1"))
	if errorCount != 1 {
		t.Errorf("Expected error count 1, got %f", errorCount)
	}

	// For histograms, we can check the count metric which tracks number of observations
	// Use a histogram directly rather than testutil.ToFloat64
	metric := &dto.Metric{}
	if err := service.driftDetectionDuration.WithLabelValues("single").(prometheus.Histogram).Write(metric); err == nil {
		if metric.GetHistogram().GetSampleCount() != 3 {
			t.Errorf("Expected 3 histogram samples, got %d", metric.GetHistogram().GetSampleCount())
		}
	}
}

func TestRecordDriftDetectionDisabled(t *testing.T) {
	service, _ := setupTestService(t)

	// Disable service
	service.Disable()

	// Record drift detection
	service.RecordDriftDetection("drift", "SHSW-1", 100*time.Millisecond)

	// Should not record when disabled
	driftCount := testutil.ToFloat64(service.driftDetectionTotal.WithLabelValues("drift", "SHSW-1"))
	if driftCount != 0 {
		t.Errorf("Expected no drift count when disabled, got %f", driftCount)
	}
}

func TestRecordBulkDriftDetection(t *testing.T) {
	service, _ := setupTestService(t)

	service.RecordBulkDriftDetection(10, 2*time.Second)

	// Check histogram samples
	metric := &dto.Metric{}
	if err := service.driftDetectionDuration.WithLabelValues("bulk").(prometheus.Histogram).Write(metric); err == nil {
		if metric.GetHistogram().GetSampleCount() != 1 {
			t.Errorf("Expected 1 bulk histogram sample, got %d", metric.GetHistogram().GetSampleCount())
		}
	}
}

func TestUpdateDriftDistribution(t *testing.T) {
	service, _ := setupTestService(t)

	severityCount := map[string]int{
		"critical": 5,
		"warning":  10,
		"info":     2,
	}

	categoryCount := map[string]int{
		"network": 8,
		"auth":    6,
		"time":    3,
	}

	service.UpdateDriftDistribution(severityCount, categoryCount)

	// Check severity distribution
	criticalCount := testutil.ToFloat64(service.driftSeverityDistribution.WithLabelValues("critical"))
	if criticalCount != 5 {
		t.Errorf("Expected critical severity count 5, got %f", criticalCount)
	}

	warningCount := testutil.ToFloat64(service.driftSeverityDistribution.WithLabelValues("warning"))
	if warningCount != 10 {
		t.Errorf("Expected warning severity count 10, got %f", warningCount)
	}

	// Check category distribution
	networkCount := testutil.ToFloat64(service.devicesWithDrift.WithLabelValues("total", "network"))
	if networkCount != 8 {
		t.Errorf("Expected network category count 8, got %f", networkCount)
	}
}

func TestRecordResolutionRequest(t *testing.T) {
	service, _ := setupTestService(t)

	service.RecordResolutionRequest("auto_fix", "success", "network")
	service.RecordResolutionRequest("manual", "pending", "auth")
	service.RecordResolutionRequest("auto_fix", "failed", "time")

	successCount := testutil.ToFloat64(service.resolutionRequestsTotal.WithLabelValues("auto_fix", "success", "network"))
	if successCount != 1 {
		t.Errorf("Expected resolution success count 1, got %f", successCount)
	}

	pendingCount := testutil.ToFloat64(service.resolutionRequestsTotal.WithLabelValues("manual", "pending", "auth"))
	if pendingCount != 1 {
		t.Errorf("Expected resolution pending count 1, got %f", pendingCount)
	}
}

func TestUpdateAutoFixSuccessRate(t *testing.T) {
	service, _ := setupTestService(t)

	service.UpdateAutoFixSuccessRate("network", 0.85)
	service.UpdateAutoFixSuccessRate("auth", 0.92)

	networkRate := testutil.ToFloat64(service.autoFixSuccessRate.WithLabelValues("network"))
	if networkRate != 0.85 {
		t.Errorf("Expected network success rate 0.85, got %f", networkRate)
	}

	authRate := testutil.ToFloat64(service.autoFixSuccessRate.WithLabelValues("auth"))
	if authRate != 0.92 {
		t.Errorf("Expected auth success rate 0.92, got %f", authRate)
	}
}

func TestRecordManualReviewTime(t *testing.T) {
	service, _ := setupTestService(t)

	service.RecordManualReviewTime("high", "security", 30*time.Minute)
	service.RecordManualReviewTime("low", "cosmetic", 5*time.Minute)

	// Check histogram samples using metric DTO
	highPriorityMetric := &dto.Metric{}
	if err := service.manualReviewTime.WithLabelValues("high", "security").(prometheus.Histogram).Write(highPriorityMetric); err == nil {
		if highPriorityMetric.GetHistogram().GetSampleCount() != 1 {
			t.Errorf("Expected 1 high priority review sample, got %d", highPriorityMetric.GetHistogram().GetSampleCount())
		}
	}

	lowPriorityMetric := &dto.Metric{}
	if err := service.manualReviewTime.WithLabelValues("low", "cosmetic").(prometheus.Histogram).Write(lowPriorityMetric); err == nil {
		if lowPriorityMetric.GetHistogram().GetSampleCount() != 1 {
			t.Errorf("Expected 1 low priority review sample, got %d", lowPriorityMetric.GetHistogram().GetSampleCount())
		}
	}
}

func TestRecordNotificationSent(t *testing.T) {
	service, _ := setupTestService(t)

	service.RecordNotificationSent("email", "critical", 250*time.Millisecond)
	service.RecordNotificationSent("webhook", "warning", 100*time.Millisecond)

	emailCount := testutil.ToFloat64(service.notificationsSent.WithLabelValues("email", "critical"))
	if emailCount != 1 {
		t.Errorf("Expected email notification count 1, got %f", emailCount)
	}

	webhookCount := testutil.ToFloat64(service.notificationsSent.WithLabelValues("webhook", "warning"))
	if webhookCount != 1 {
		t.Errorf("Expected webhook notification count 1, got %f", webhookCount)
	}

	// Check latency histogram
	latencyMetric := &dto.Metric{}
	if err := service.notificationLatency.WithLabelValues("email").(prometheus.Histogram).Write(latencyMetric); err == nil {
		if latencyMetric.GetHistogram().GetSampleCount() != 1 {
			t.Errorf("Expected 1 email latency sample, got %d", latencyMetric.GetHistogram().GetSampleCount())
		}
	}
}

func TestRecordNotificationFailure(t *testing.T) {
	service, _ := setupTestService(t)

	service.RecordNotificationFailure("email", "smtp_error")
	service.RecordNotificationFailure("webhook", "timeout")

	emailFailureCount := testutil.ToFloat64(service.notificationFailures.WithLabelValues("email", "smtp_error"))
	if emailFailureCount != 1 {
		t.Errorf("Expected email failure count 1, got %f", emailFailureCount)
	}

	webhookFailureCount := testutil.ToFloat64(service.notificationFailures.WithLabelValues("webhook", "timeout"))
	if webhookFailureCount != 1 {
		t.Errorf("Expected webhook failure count 1, got %f", webhookFailureCount)
	}
}

func TestUpdateDeviceStatus(t *testing.T) {
	service, _ := setupTestService(t)

	service.UpdateDeviceStatus("1", "Living Room Switch", "SHSW-1", true)
	service.UpdateDeviceStatus("2", "Bedroom Dimmer", "SHDM-1", false)

	onlineStatus := testutil.ToFloat64(service.deviceStatus.WithLabelValues("1", "Living Room Switch", "SHSW-1"))
	if onlineStatus != 1.0 {
		t.Errorf("Expected online device status 1.0, got %f", onlineStatus)
	}

	offlineStatus := testutil.ToFloat64(service.deviceStatus.WithLabelValues("2", "Bedroom Dimmer", "SHDM-1"))
	if offlineStatus != 0.0 {
		t.Errorf("Expected offline device status 0.0, got %f", offlineStatus)
	}
}

func TestUpdateConfigSyncStatus(t *testing.T) {
	service, _ := setupTestService(t)

	service.UpdateConfigSyncStatus("1", "Living Room Switch", true)
	service.UpdateConfigSyncStatus("2", "Bedroom Dimmer", false)

	syncedStatus := testutil.ToFloat64(service.configSyncStatus.WithLabelValues("1", "Living Room Switch"))
	if syncedStatus != 1.0 {
		t.Errorf("Expected synced config status 1.0, got %f", syncedStatus)
	}

	driftStatus := testutil.ToFloat64(service.configSyncStatus.WithLabelValues("2", "Bedroom Dimmer"))
	if driftStatus != 0.0 {
		t.Errorf("Expected drift config status 0.0, got %f", driftStatus)
	}
}

func TestStartTimer(t *testing.T) {
	service, _ := setupTestService(t)

	timer := service.StartTimer()

	if timer == nil {
		t.Fatal("StartTimer returned nil")
	}

	if timer.service != service {
		t.Error("Timer service reference not set correctly")
	}

	if timer.start.IsZero() {
		t.Error("Timer start time not set")
	}
}

func TestGetLastCollectionTime(t *testing.T) {
	service, _ := setupTestService(t)

	// Initially should be zero
	lastTime := service.GetLastCollectionTime()
	if !lastTime.IsZero() {
		t.Error("Initial last collection time should be zero")
	}

	// After collection, should be updated
	ctx := context.Background()
	err := service.CollectMetrics(ctx)
	if err != nil {
		t.Fatalf("CollectMetrics failed: %v", err)
	}

	newLastTime := service.GetLastCollectionTime()
	if newLastTime.IsZero() {
		t.Error("Last collection time should be updated after collection")
	}

	if !newLastTime.After(lastTime) {
		t.Error("Last collection time should be after initial time")
	}
}

func TestCollectMetricsWithData(t *testing.T) {
	service, _ := setupTestService(t)

	// Insert test data
	err := service.db.Exec(`
		INSERT INTO drift_trends (severity, category, resolved) VALUES 
		('critical', 'network', false),
		('warning', 'auth', false),
		('info', 'time', true)
	`).Error
	if err != nil {
		t.Fatalf("Failed to insert test drift trends: %v", err)
	}

	err = service.db.Exec(`
		INSERT INTO resolution_histories (category, method, type, success) VALUES 
		('network', 'auto', 'auto_fix', true),
		('auth', 'manual', 'auto_fix', false),
		('time', 'auto', 'auto_fix', true)
	`).Error
	if err != nil {
		t.Fatalf("Failed to insert test resolution histories: %v", err)
	}

	err = service.db.Exec(`
		INSERT INTO devices (name, type, status) VALUES 
		('Switch 1', 'SHSW-1', 'online'),
		('Dimmer 1', 'SHDM-1', 'offline')
	`).Error
	if err != nil {
		t.Fatalf("Failed to insert test devices: %v", err)
	}

	// Collect metrics
	ctx := context.Background()
	err = service.CollectMetrics(ctx)
	if err != nil {
		t.Fatalf("CollectMetrics failed: %v", err)
	}

	// Verify metrics were updated
	criticalCount := testutil.ToFloat64(service.driftSeverityDistribution.WithLabelValues("critical"))
	if criticalCount != 1 {
		t.Errorf("Expected critical severity count 1, got %f", criticalCount)
	}

	warningCount := testutil.ToFloat64(service.driftSeverityDistribution.WithLabelValues("warning"))
	if warningCount != 1 {
		t.Errorf("Expected warning severity count 1, got %f", warningCount)
	}

	// Check auto-fix success rate for network category
	networkSuccessRate := testutil.ToFloat64(service.autoFixSuccessRate.WithLabelValues("network"))
	if networkSuccessRate != 1.0 {
		t.Errorf("Expected network success rate 1.0, got %f", networkSuccessRate)
	}
}

func TestCollectMetricsDisabled(t *testing.T) {
	service, _ := setupTestService(t)

	service.Disable()

	ctx := context.Background()
	err := service.CollectMetrics(ctx)
	if err != nil {
		t.Errorf("CollectMetrics should not fail when disabled: %v", err)
	}
}

func TestStartUptimeCounter(t *testing.T) {
	service, _ := setupTestService(t)

	// Get initial value
	initialUptime := testutil.ToFloat64(service.systemUptime)

	service.StartUptimeCounter()

	// Wait a bit for the counter to increment
	time.Sleep(1100 * time.Millisecond)

	newUptime := testutil.ToFloat64(service.systemUptime)

	// Should have incremented (at least 1 second)
	if newUptime <= initialUptime {
		t.Errorf("Uptime should have incremented, was %f, now %f", initialUptime, newUptime)
	}
}

func TestStartUptimeCounterDisabled(t *testing.T) {
	service, _ := setupTestService(t)

	service.Disable()

	// Get initial value
	initialUptime := testutil.ToFloat64(service.systemUptime)

	service.StartUptimeCounter()

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	newUptime := testutil.ToFloat64(service.systemUptime)

	// Should not have incremented when disabled
	if newUptime != initialUptime {
		t.Errorf("Uptime should not increment when disabled, was %f, now %f", initialUptime, newUptime)
	}
}
