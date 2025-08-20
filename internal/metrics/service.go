package metrics

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// Service handles metrics collection and export
type Service struct {
	db     *gorm.DB
	logger *logging.Logger

	// Prometheus metrics
	registry prometheus.Registerer

	// Drift detection metrics
	driftDetectionTotal       prometheus.CounterVec
	driftDetectionDuration    prometheus.HistogramVec
	devicesWithDrift          prometheus.GaugeVec
	driftSeverityDistribution prometheus.GaugeVec

	// Resolution metrics
	resolutionRequestsTotal prometheus.CounterVec
	autoFixSuccessRate      prometheus.GaugeVec
	manualReviewTime        prometheus.HistogramVec
	resolutionsByCategory   prometheus.GaugeVec

	// Notification metrics
	notificationsSent    prometheus.CounterVec
	notificationFailures prometheus.CounterVec
	notificationLatency  prometheus.HistogramVec

	// System health metrics
	deviceStatus     prometheus.GaugeVec
	configSyncStatus prometheus.GaugeVec
	systemUptime     prometheus.Counter

	// Internal state
	mu                 sync.RWMutex
	lastCollectionTime time.Time
	enabled            bool
}

// NewService creates a new metrics service
func NewService(db *gorm.DB, logger *logging.Logger, registry prometheus.Registerer) *Service {
	if registry == nil {
		registry = prometheus.DefaultRegisterer
	}

	s := &Service{
		db:       db,
		logger:   logger,
		registry: registry,
		enabled:  true,
	}

	s.initializePrometheusMetrics()

	s.logger.WithFields(map[string]any{
		"component": "metrics",
	}).Info("Metrics service initialized")

	return s
}

// initializePrometheusMetrics sets up all Prometheus metrics
func (s *Service) initializePrometheusMetrics() {
	// Drift detection metrics
	s.driftDetectionTotal = *promauto.With(s.registry).NewCounterVec(
		prometheus.CounterOpts{
			Name: "shelly_drift_detection_total",
			Help: "Total number of drift detection operations",
		},
		[]string{"status", "device_type"},
	)

	s.driftDetectionDuration = *promauto.With(s.registry).NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "shelly_drift_detection_duration_seconds",
			Help:    "Duration of drift detection operations",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation_type"},
	)

	s.devicesWithDrift = *promauto.With(s.registry).NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "shelly_devices_with_drift",
			Help: "Number of devices currently showing configuration drift",
		},
		[]string{"severity", "category"},
	)

	s.driftSeverityDistribution = *promauto.With(s.registry).NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "shelly_drift_severity_distribution",
			Help: "Distribution of drift issues by severity level",
		},
		[]string{"severity"},
	)

	// Resolution metrics
	s.resolutionRequestsTotal = *promauto.With(s.registry).NewCounterVec(
		prometheus.CounterOpts{
			Name: "shelly_resolution_requests_total",
			Help: "Total number of resolution requests",
		},
		[]string{"type", "status", "category"},
	)

	s.autoFixSuccessRate = *promauto.With(s.registry).NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "shelly_auto_fix_success_rate",
			Help: "Success rate of auto-fix operations",
		},
		[]string{"category"},
	)

	s.manualReviewTime = *promauto.With(s.registry).NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "shelly_manual_review_duration_seconds",
			Help:    "Time taken for manual review of resolution requests",
			Buckets: []float64{60, 300, 900, 1800, 3600, 7200, 21600, 86400}, // 1m to 1d
		},
		[]string{"priority", "category"},
	)

	s.resolutionsByCategory = *promauto.With(s.registry).NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "shelly_resolutions_by_category",
			Help: "Number of resolutions by category",
		},
		[]string{"category", "method"},
	)

	// Notification metrics
	s.notificationsSent = *promauto.With(s.registry).NewCounterVec(
		prometheus.CounterOpts{
			Name: "shelly_notifications_sent_total",
			Help: "Total number of notifications sent",
		},
		[]string{"channel_type", "alert_level"},
	)

	s.notificationFailures = *promauto.With(s.registry).NewCounterVec(
		prometheus.CounterOpts{
			Name: "shelly_notification_failures_total",
			Help: "Total number of notification failures",
		},
		[]string{"channel_type", "error_type"},
	)

	s.notificationLatency = *promauto.With(s.registry).NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "shelly_notification_latency_seconds",
			Help:    "Latency of notification delivery",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30},
		},
		[]string{"channel_type"},
	)

	// System health metrics
	s.deviceStatus = *promauto.With(s.registry).NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "shelly_device_status",
			Help: "Status of Shelly devices (1=online, 0=offline)",
		},
		[]string{"device_id", "device_name", "device_type"},
	)

	s.configSyncStatus = *promauto.With(s.registry).NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "shelly_config_sync_status",
			Help: "Configuration sync status (1=synced, 0=drift)",
		},
		[]string{"device_id", "device_name"},
	)

	s.systemUptime = promauto.With(s.registry).NewCounter(
		prometheus.CounterOpts{
			Name: "shelly_manager_uptime_seconds_total",
			Help: "Total uptime of the shelly-manager service",
		},
	)
}

// RecordDriftDetection records drift detection metrics
func (s *Service) RecordDriftDetection(status, deviceType string, duration time.Duration) {
	if !s.enabled {
		return
	}

	s.driftDetectionTotal.WithLabelValues(status, deviceType).Inc()
	s.driftDetectionDuration.WithLabelValues("single").Observe(duration.Seconds())

	s.logger.WithFields(map[string]any{
		"status":      status,
		"device_type": deviceType,
		"duration":    duration,
		"component":   "metrics",
	}).Debug("Recorded drift detection metric")
}

// RecordBulkDriftDetection records bulk drift detection metrics
func (s *Service) RecordBulkDriftDetection(deviceCount int, duration time.Duration) {
	if !s.enabled {
		return
	}

	s.driftDetectionDuration.WithLabelValues("bulk").Observe(duration.Seconds())

	s.logger.WithFields(map[string]any{
		"device_count": deviceCount,
		"duration":     duration,
		"component":    "metrics",
	}).Debug("Recorded bulk drift detection metric")
}

// UpdateDriftDistribution updates the current drift distribution
func (s *Service) UpdateDriftDistribution(severityCount map[string]int, categoryCount map[string]int) {
	if !s.enabled {
		return
	}

	// Update severity distribution
	for severity, count := range severityCount {
		s.driftSeverityDistribution.WithLabelValues(severity).Set(float64(count))
	}

	// Update devices with drift by category
	for category := range categoryCount {
		for severity := range severityCount {
			// Set to 0 first to clear old values
			s.devicesWithDrift.WithLabelValues(severity, category).Set(0)
		}
	}

	// Set actual values
	for category, count := range categoryCount {
		s.devicesWithDrift.WithLabelValues("total", category).Set(float64(count))
	}

	s.logger.WithFields(map[string]any{
		"severity_count": severityCount,
		"category_count": categoryCount,
		"component":      "metrics",
	}).Debug("Updated drift distribution metrics")
}

// RecordResolutionRequest records resolution request metrics
func (s *Service) RecordResolutionRequest(requestType, status, category string) {
	if !s.enabled {
		return
	}

	s.resolutionRequestsTotal.WithLabelValues(requestType, status, category).Inc()

	s.logger.WithFields(map[string]any{
		"request_type": requestType,
		"status":       status,
		"category":     category,
		"component":    "metrics",
	}).Debug("Recorded resolution request metric")
}

// UpdateAutoFixSuccessRate updates the auto-fix success rate
func (s *Service) UpdateAutoFixSuccessRate(category string, rate float64) {
	if !s.enabled {
		return
	}

	s.autoFixSuccessRate.WithLabelValues(category).Set(rate)

	s.logger.WithFields(map[string]any{
		"category":     category,
		"success_rate": rate,
		"component":    "metrics",
	}).Debug("Updated auto-fix success rate")
}

// RecordManualReviewTime records the time taken for manual review
func (s *Service) RecordManualReviewTime(priority, category string, duration time.Duration) {
	if !s.enabled {
		return
	}

	s.manualReviewTime.WithLabelValues(priority, category).Observe(duration.Seconds())

	s.logger.WithFields(map[string]any{
		"priority":  priority,
		"category":  category,
		"duration":  duration,
		"component": "metrics",
	}).Debug("Recorded manual review time")
}

// RecordNotificationSent records successful notification delivery
func (s *Service) RecordNotificationSent(channelType, alertLevel string, latency time.Duration) {
	if !s.enabled {
		return
	}

	s.notificationsSent.WithLabelValues(channelType, alertLevel).Inc()
	s.notificationLatency.WithLabelValues(channelType).Observe(latency.Seconds())

	s.logger.WithFields(map[string]any{
		"channel_type": channelType,
		"alert_level":  alertLevel,
		"latency":      latency,
		"component":    "metrics",
	}).Debug("Recorded notification sent")
}

// RecordNotificationFailure records notification failure
func (s *Service) RecordNotificationFailure(channelType, errorType string) {
	if !s.enabled {
		return
	}

	s.notificationFailures.WithLabelValues(channelType, errorType).Inc()

	s.logger.WithFields(map[string]any{
		"channel_type": channelType,
		"error_type":   errorType,
		"component":    "metrics",
	}).Debug("Recorded notification failure")
}

// UpdateDeviceStatus updates device online/offline status
func (s *Service) UpdateDeviceStatus(deviceID, deviceName, deviceType string, online bool) {
	if !s.enabled {
		return
	}

	status := 0.0
	if online {
		status = 1.0
	}

	s.deviceStatus.WithLabelValues(deviceID, deviceName, deviceType).Set(status)
}

// UpdateConfigSyncStatus updates configuration sync status
func (s *Service) UpdateConfigSyncStatus(deviceID, deviceName string, synced bool) {
	if !s.enabled {
		return
	}

	status := 0.0
	if synced {
		status = 1.0
	}

	s.configSyncStatus.WithLabelValues(deviceID, deviceName).Set(status)
}

// StartUptimeCounter starts the uptime counter
func (s *Service) StartUptimeCounter() {
	if !s.enabled {
		return
	}

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for range ticker.C {
			s.systemUptime.Inc()
		}
	}()
}

// CollectMetrics performs periodic metrics collection from database
func (s *Service) CollectMetrics(ctx context.Context) error {
	if !s.enabled {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	start := time.Now()

	s.logger.WithFields(map[string]any{
		"component": "metrics",
	}).Debug("Starting metrics collection")

	// Collect drift metrics
	if err := s.collectDriftMetrics(ctx); err != nil {
		s.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "metrics",
		}).Error("Failed to collect drift metrics")
	}

	// Collect resolution metrics
	if err := s.collectResolutionMetrics(ctx); err != nil {
		s.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "metrics",
		}).Error("Failed to collect resolution metrics")
	}

	// Collect device status metrics
	if err := s.collectDeviceMetrics(ctx); err != nil {
		s.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "metrics",
		}).Error("Failed to collect device metrics")
	}

	s.lastCollectionTime = time.Now()
	duration := time.Since(start)

	s.logger.WithFields(map[string]any{
		"duration":  duration,
		"component": "metrics",
	}).Debug("Completed metrics collection")

	return nil
}

// collectDriftMetrics collects drift-related metrics from database
func (s *Service) collectDriftMetrics(ctx context.Context) error {
	// Query drift trends for current distribution
	var trends []struct {
		Severity string
		Category string
		Count    int
	}

	if err := s.db.WithContext(ctx).
		Table("drift_trends").
		Select("severity, category, COUNT(*) as count").
		Where("resolved = ?", false).
		Group("severity, category").
		Scan(&trends).Error; err != nil {
		return fmt.Errorf("failed to query drift trends: %w", err)
	}

	// Update severity and category distributions
	severityCount := make(map[string]int)
	categoryCount := make(map[string]int)

	for _, trend := range trends {
		severityCount[trend.Severity] += trend.Count
		categoryCount[trend.Category] += trend.Count

		s.devicesWithDrift.WithLabelValues(trend.Severity, trend.Category).Set(float64(trend.Count))
	}

	// Update total distributions
	for severity, count := range severityCount {
		s.driftSeverityDistribution.WithLabelValues(severity).Set(float64(count))
	}

	return nil
}

// collectResolutionMetrics collects resolution-related metrics from database
func (s *Service) collectResolutionMetrics(ctx context.Context) error {
	// Calculate auto-fix success rates by category
	var autoFixStats []struct {
		Category    string
		Attempts    int
		Successes   int
		SuccessRate float64
	}

	if err := s.db.WithContext(ctx).Raw(`
		SELECT 
			category,
			COUNT(*) as attempts,
			SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END) as successes,
			CAST(SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END) AS FLOAT) / COUNT(*) as success_rate
		FROM resolution_histories 
		WHERE type = 'auto_fix' 
		  AND executed_at >= datetime('now', '-24 hours')
		GROUP BY category
	`).Scan(&autoFixStats).Error; err != nil {
		return fmt.Errorf("failed to query auto-fix stats: %w", err)
	}

	for _, stat := range autoFixStats {
		s.UpdateAutoFixSuccessRate(stat.Category, stat.SuccessRate)
	}

	// Count resolutions by category and method
	var resolutionCounts []struct {
		Category string
		Method   string
		Count    int
	}

	if err := s.db.WithContext(ctx).
		Table("resolution_histories").
		Select("category, method, COUNT(*) as count").
		Where("executed_at >= datetime('now', '-24 hours')").
		Group("category, method").
		Scan(&resolutionCounts).Error; err != nil {
		return fmt.Errorf("failed to query resolution counts: %w", err)
	}

	for _, count := range resolutionCounts {
		s.resolutionsByCategory.WithLabelValues(count.Category, count.Method).Set(float64(count.Count))
	}

	return nil
}

// collectDeviceMetrics collects device status and configuration sync metrics
func (s *Service) collectDeviceMetrics(ctx context.Context) error {
	// Query device status and config sync information
	type DeviceMetric struct {
		ID     uint
		Name   string
		Type   string
		Status string // from discovery/last_seen
		Synced bool   // from drift detection
	}

	var devices []DeviceMetric

	// This is a simplified query - in production you'd join with config sync status
	if err := s.db.WithContext(ctx).
		Table("devices").
		Select("id, name, type, status").
		Scan(&devices).Error; err != nil {
		return fmt.Errorf("failed to query device metrics: %w", err)
	}

	for _, device := range devices {
		deviceID := fmt.Sprintf("%d", device.ID)
		online := device.Status == "online"

		s.UpdateDeviceStatus(deviceID, device.Name, device.Type, online)

		// For now, assume synced if online - in production you'd check actual drift status
		s.UpdateConfigSyncStatus(deviceID, device.Name, online)
	}

	return nil
}

// Enable enables metrics collection
func (s *Service) Enable() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.enabled = true

	s.logger.WithFields(map[string]any{
		"component": "metrics",
	}).Info("Metrics collection enabled")
}

// Disable disables metrics collection
func (s *Service) Disable() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.enabled = false

	s.logger.WithFields(map[string]any{
		"component": "metrics",
	}).Info("Metrics collection disabled")
}

// IsEnabled returns whether metrics collection is enabled
func (s *Service) IsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.enabled
}

// GetLastCollectionTime returns the last metrics collection time
func (s *Service) GetLastCollectionTime() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.lastCollectionTime
}

// GetRegistry returns the Prometheus registry
func (s *Service) GetRegistry() prometheus.Registerer {
	return s.registry
}
