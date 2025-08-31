package metrics

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// WebSocketHub manages WebSocket connections for real-time metrics
type WebSocketHub struct {
	clients        map[*WebSocketClient]bool
	register       chan *WebSocketClient
	unregister     chan *WebSocketClient
	broadcast      chan *MetricsUpdate
	service        *Service
	logger         *logging.Logger
	mu             sync.RWMutex
	allowedOrigins []string

	// Connection limiting per client IP
	connCounts     map[string]int
	connLimitPerIP int
}

// WebSocketClient represents a connected WebSocket client
type WebSocketClient struct {
	hub  *WebSocketHub
	conn *websocket.Conn
	send chan *MetricsUpdate
	ip   string
}

// MetricsUpdate represents a real-time metrics update
type MetricsUpdate struct {
	Type      string      `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// DashboardMetrics represents the complete dashboard metrics
type DashboardMetrics struct {
	SystemStatus        SystemStatus        `json:"system_status"`
	DeviceMetrics       []DeviceMetric      `json:"device_metrics"`
	DriftMetrics        DriftMetrics        `json:"drift_metrics"`
	NotificationMetrics NotificationMetrics `json:"notification_metrics"`
	ResolutionMetrics   ResolutionMetrics   `json:"resolution_metrics"`
}

// SystemStatus represents overall system health
type SystemStatus struct {
	Uptime             float64   `json:"uptime_seconds"`
	MetricsEnabled     bool      `json:"metrics_enabled"`
	LastCollectionTime time.Time `json:"last_collection_time"`
	TotalDevices       int       `json:"total_devices"`
	OnlineDevices      int       `json:"online_devices"`
	DevicesWithDrift   int       `json:"devices_with_drift"`
}

// DeviceMetric represents individual device status
type DeviceMetric struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	Status       string    `json:"status"`
	ConfigSynced bool      `json:"config_synced"`
	LastSeen     time.Time `json:"last_seen"`
}

// DriftMetrics represents configuration drift statistics
type DriftMetrics struct {
	TotalDriftIssues     int            `json:"total_drift_issues"`
	SeverityDistribution map[string]int `json:"severity_distribution"`
	CategoryDistribution map[string]int `json:"category_distribution"`
	TrendAnalysis        []DriftTrend   `json:"trend_analysis"`
}

// DriftTrend represents drift trend data
type DriftTrend struct {
	Date     time.Time `json:"date"`
	Count    int       `json:"count"`
	Severity string    `json:"severity"`
	Category string    `json:"category"`
}

// NotificationMetrics represents notification system statistics
type NotificationMetrics struct {
	TotalSent           int            `json:"total_sent"`
	TotalFailed         int            `json:"total_failed"`
	ChannelBreakdown    map[string]int `json:"channel_breakdown"`
	AlertLevelBreakdown map[string]int `json:"alert_level_breakdown"`
	AverageLatency      float64        `json:"average_latency_seconds"`
}

// ResolutionMetrics represents resolution system statistics
type ResolutionMetrics struct {
	TotalResolutions      int                `json:"total_resolutions"`
	AutoFixSuccessRate    map[string]float64 `json:"auto_fix_success_rate"`
	ResolutionsByCategory map[string]int     `json:"resolutions_by_category"`
	AverageReviewTime     float64            `json:"average_review_time_seconds"`
}

// historical global upgrader removed; each handler builds its own with proper CheckOrigin.

// NewWebSocketHub creates a new WebSocket hub
func NewWebSocketHub(service *Service, logger *logging.Logger) *WebSocketHub {
	return &WebSocketHub{
		clients:        make(map[*WebSocketClient]bool),
		register:       make(chan *WebSocketClient),
		unregister:     make(chan *WebSocketClient),
		broadcast:      make(chan *MetricsUpdate),
		service:        service,
		logger:         logger,
		connCounts:     make(map[string]int),
		connLimitPerIP: 5,
	}
}

// Run starts the WebSocket hub
func (h *WebSocketHub) Run(ctx context.Context) {
	// Start background metrics collection and broadcasting
	go h.startMetricsCollection(ctx)

	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			if client.ip != "" {
				h.connCounts[client.ip]++
			}
			h.mu.Unlock()

			h.logger.WithFields(map[string]any{
				"component": "websocket",
				"clients":   len(h.clients),
			}).Info("New WebSocket client connected")

			// Send initial metrics to new client
			go h.sendInitialMetrics(client)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			if client.ip != "" && h.connCounts[client.ip] > 0 {
				h.connCounts[client.ip]--
				if h.connCounts[client.ip] == 0 {
					delete(h.connCounts, client.ip)
				}
			}
			h.mu.Unlock()

			h.logger.WithFields(map[string]any{
				"component": "websocket",
				"clients":   len(h.clients),
			}).Info("WebSocket client disconnected")

		case update := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- update:
				default:
					delete(h.clients, client)
					close(client.send)
				}
			}
			h.mu.RUnlock()

		case <-ctx.Done():
			h.logger.WithFields(map[string]any{
				"component": "websocket",
			}).Info("WebSocket hub shutting down")
			return
		}
	}
}

// startMetricsCollection starts periodic metrics collection and broadcasting
func (h *WebSocketHub) startMetricsCollection(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second) // Update every 5 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			metrics, err := h.collectDashboardMetrics(ctx)
			if err != nil {
				h.logger.WithFields(map[string]any{
					"error":     err.Error(),
					"component": "websocket",
				}).Error("Failed to collect dashboard metrics")
				continue
			}

			update := &MetricsUpdate{
				Type:      "metrics_update",
				Timestamp: time.Now(),
				Data:      metrics,
			}

			select {
			case h.broadcast <- update:
			default:
				// Channel full, skip this update
			}

		case <-ctx.Done():
			return
		}
	}
}

// collectDashboardMetrics collects all metrics for the dashboard
func (h *WebSocketHub) collectDashboardMetrics(ctx context.Context) (*DashboardMetrics, error) {
	// Trigger metrics collection
	if err := h.service.CollectMetrics(ctx); err != nil {
		return nil, fmt.Errorf("failed to collect metrics: %w", err)
	}

	// Get system status
	systemStatus, err := h.getSystemStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get system status: %w", err)
	}

	// Get device metrics
	deviceMetrics, err := h.getDeviceMetrics(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get device metrics: %w", err)
	}

	// Get drift metrics
	driftMetrics, err := h.getDriftMetrics(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get drift metrics: %w", err)
	}

	// Get notification metrics
	notificationMetrics, err := h.getNotificationMetrics(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get notification metrics: %w", err)
	}

	// Get resolution metrics
	resolutionMetrics, err := h.getResolutionMetrics(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get resolution metrics: %w", err)
	}

	return &DashboardMetrics{
		SystemStatus:        *systemStatus,
		DeviceMetrics:       deviceMetrics,
		DriftMetrics:        *driftMetrics,
		NotificationMetrics: *notificationMetrics,
		ResolutionMetrics:   *resolutionMetrics,
	}, nil
}

// sendInitialMetrics sends initial metrics to a new client
func (h *WebSocketHub) sendInitialMetrics(client *WebSocketClient) {
	ctx := context.Background()
	metrics, err := h.collectDashboardMetrics(ctx)
	if err != nil {
		h.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "websocket",
		}).Error("Failed to collect initial metrics")
		return
	}

	update := &MetricsUpdate{
		Type:      "initial_metrics",
		Timestamp: time.Now(),
		Data:      metrics,
	}

	select {
	case client.send <- update:
	default:
		// Client channel full or closed
	}
}

// HandleWebSocket handles WebSocket upgrade requests
func (h *WebSocketHub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Build an upgrader that enforces allowed origins if configured
	localUpgrader := websocket.Upgrader{
		ReadBufferSize:  0,
		WriteBufferSize: 0,
		CheckOrigin: func(r *http.Request) bool {
			if len(h.allowedOrigins) == 0 {
				return true
			}
			origin := r.Header.Get("Origin")
			if origin == "" {
				return false
			}
			for _, ao := range h.allowedOrigins {
				if ao == "*" || ao == origin {
					return true
				}
			}
			return false
		},
	}
	// Enforce per-IP connection limit
	ip := getClientIP(r)
	if h.connLimitPerIP > 0 {
		h.mu.RLock()
		current := h.connCounts[ip]
		h.mu.RUnlock()
		if current >= h.connLimitPerIP {
			http.Error(w, "Too many WebSocket connections from this IP", http.StatusTooManyRequests)
			return
		}
	}

	conn, err := localUpgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "websocket",
		}).Error("Failed to upgrade WebSocket connection")
		return
	}

	client := &WebSocketClient{
		hub:  h,
		conn: conn,
		send: make(chan *MetricsUpdate, 256),
		ip:   ip,
	}

	client.hub.register <- client

	// Start goroutines for handling the connection
	go client.writePump()
	go client.readPump()
}

// SetAllowedOrigins configures the allowed origins for WebSocket connections
func (h *WebSocketHub) SetAllowedOrigins(origins []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.allowedOrigins = origins
}

// SetConnectionLimitPerIP configures the maximum concurrent WebSocket connections per client IP
func (h *WebSocketHub) SetConnectionLimitPerIP(n int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.connLimitPerIP = n
}

// getClientIP extracts client IP from headers or remote addr
func getClientIP(r *http.Request) string {
	if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
		// Use first IP in the list
		for i := 0; i < len(xf); i++ {
			if xf[i] == ',' {
				return xf[:i]
			}
		}
		return xf
	}
	if xr := r.Header.Get("X-Real-IP"); xr != "" {
		return xr
	}
	// Fallback to RemoteAddr (host:port)
	addr := r.RemoteAddr
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i]
		}
	}
	return addr
}

// readPump pumps messages from the WebSocket connection
func (c *WebSocketClient) readPump() {
	defer func() {
		c.hub.unregister <- c
		if err := c.conn.Close(); err != nil {
			// Log error if possible but continue
			_ = err
		}
	}()

	c.conn.SetReadLimit(512)
	if err := c.conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
		c.hub.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "websocket",
		}).Error("Failed to set read deadline")
		return
	}
	c.conn.SetPongHandler(func(string) error {
		if err := c.conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
			c.hub.logger.WithFields(map[string]any{
				"error":     err.Error(),
				"component": "websocket",
			}).Error("Failed to set read deadline in pong handler")
		}
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.hub.logger.WithFields(map[string]any{
					"error":     err.Error(),
					"component": "websocket",
				}).Error("WebSocket error")
			}
			break
		}
	}
}

// writePump pumps messages to the WebSocket connection
func (c *WebSocketClient) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		if err := c.conn.Close(); err != nil {
			// Log error if possible but continue
			_ = err
		}
	}()

	for {
		select {
		case update, ok := <-c.send:
			if err := c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
				c.hub.logger.WithFields(map[string]any{
					"error":     err.Error(),
					"component": "websocket",
				}).Error("Failed to set write deadline")
				return
			}
			if !ok {
				if err := c.conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil {
					c.hub.logger.WithFields(map[string]any{
						"error":     err.Error(),
						"component": "websocket",
					}).Error("Failed to write close message")
				}
				return
			}

			if err := c.conn.WriteJSON(update); err != nil {
				c.hub.logger.WithFields(map[string]any{
					"error":     err.Error(),
					"component": "websocket",
				}).Error("Failed to write WebSocket message")
				return
			}

		case <-ticker.C:
			if err := c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
				c.hub.logger.WithFields(map[string]any{
					"error":     err.Error(),
					"component": "websocket",
				}).Error("Failed to set write deadline for ping")
				return
			}
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// BroadcastAlert broadcasts an alert to all connected clients
func (h *WebSocketHub) BroadcastAlert(alertType, message string, severity string) {
	alert := map[string]interface{}{
		"alert_type": alertType,
		"message":    message,
		"severity":   severity,
	}

	update := &MetricsUpdate{
		Type:      "alert",
		Timestamp: time.Now(),
		Data:      alert,
	}

	select {
	case h.broadcast <- update:
		h.logger.WithFields(map[string]any{
			"alert_type": alertType,
			"severity":   severity,
			"component":  "websocket",
		}).Debug("Broadcasted alert to dashboard clients")
	default:
		// Channel full, skip this alert
		h.logger.WithFields(map[string]any{
			"alert_type": alertType,
			"severity":   severity,
			"component":  "websocket",
		}).Warn("Failed to broadcast alert - channel full")
	}
}

// BroadcastDeviceStatusChange broadcasts device status changes
func (h *WebSocketHub) BroadcastDeviceStatusChange(deviceID, deviceName, oldStatus, newStatus string) {
	statusChange := map[string]interface{}{
		"device_id":   deviceID,
		"device_name": deviceName,
		"old_status":  oldStatus,
		"new_status":  newStatus,
		"timestamp":   time.Now(),
	}

	update := &MetricsUpdate{
		Type:      "device_status_change",
		Timestamp: time.Now(),
		Data:      statusChange,
	}

	severity := "info"
	if newStatus == "offline" {
		severity = "warning"
	} else if newStatus == "online" && oldStatus == "offline" {
		severity = "info"
	}

	// Also send as an alert for immediate visibility
	alertMessage := fmt.Sprintf("Device %s went %s", deviceName, newStatus)
	h.BroadcastAlert("device_status", alertMessage, severity)

	select {
	case h.broadcast <- update:
	default:
		// Channel full, skip this update
	}
}

// BroadcastDriftDetected broadcasts configuration drift detection
func (h *WebSocketHub) BroadcastDriftDetected(deviceID, deviceName string, driftCount int, severity string) {
	driftAlert := map[string]interface{}{
		"device_id":   deviceID,
		"device_name": deviceName,
		"drift_count": driftCount,
		"severity":    severity,
		"timestamp":   time.Now(),
	}

	update := &MetricsUpdate{
		Type:      "drift_detected",
		Timestamp: time.Now(),
		Data:      driftAlert,
	}

	// Send alert
	alertMessage := fmt.Sprintf("Configuration drift detected on %s (%d issues)", deviceName, driftCount)
	h.BroadcastAlert("drift_detected", alertMessage, severity)

	select {
	case h.broadcast <- update:
	default:
		// Channel full, skip this update
	}
}

// GetWebSocketHub returns a function to get the current hub instance
// This allows external services to broadcast alerts
func GetWebSocketHubBroadcaster(hub *WebSocketHub) func(alertType, message, severity string) {
	return func(alertType, message, severity string) {
		if hub != nil {
			hub.BroadcastAlert(alertType, message, severity)
		}
	}
}

// Helper methods for collecting specific metrics

func (h *WebSocketHub) getSystemStatus(ctx context.Context) (*SystemStatus, error) {
	// Query device counts
	var totalDevices, onlineDevices, devicesWithDrift int64

	if err := h.service.db.WithContext(ctx).Table("devices").Count(&totalDevices).Error; err != nil {
		return nil, fmt.Errorf("failed to count total devices: %w", err)
	}

	if err := h.service.db.WithContext(ctx).Table("devices").Where("status = ?", "online").Count(&onlineDevices).Error; err != nil {
		return nil, fmt.Errorf("failed to count online devices: %w", err)
	}

	// Count devices with drift (simplified query)
	if err := h.service.db.WithContext(ctx).Table("drift_trends").Where("resolved = ?", false).Distinct("device_id").Count(&devicesWithDrift).Error; err != nil {
		// If drift_trends table doesn't exist yet, set to 0
		devicesWithDrift = 0
	}

	return &SystemStatus{
		Uptime:             time.Since(time.Now().Add(-24 * time.Hour)).Seconds(), // Placeholder
		MetricsEnabled:     h.service.IsEnabled(),
		LastCollectionTime: h.service.GetLastCollectionTime(),
		TotalDevices:       int(totalDevices),
		OnlineDevices:      int(onlineDevices),
		DevicesWithDrift:   int(devicesWithDrift),
	}, nil
}

func (h *WebSocketHub) getDeviceMetrics(ctx context.Context) ([]DeviceMetric, error) {
	var devices []struct {
		ID     uint   `json:"id"`
		Name   string `json:"name"`
		Type   string `json:"type"`
		Status string `json:"status"`
	}

	if err := h.service.db.WithContext(ctx).Table("devices").Select("id, name, type, status").Scan(&devices).Error; err != nil {
		return nil, fmt.Errorf("failed to query devices: %w", err)
	}

	metrics := make([]DeviceMetric, len(devices))
	for i, device := range devices {
		metrics[i] = DeviceMetric{
			ID:           fmt.Sprintf("%d", device.ID),
			Name:         device.Name,
			Type:         device.Type,
			Status:       device.Status,
			ConfigSynced: device.Status == "online", // Simplified
			LastSeen:     time.Now(),                // Placeholder
		}
	}

	return metrics, nil
}

func (h *WebSocketHub) getDriftMetrics(ctx context.Context) (*DriftMetrics, error) {
	// Query drift statistics
	var totalDrift int64
	severityDist := make(map[string]int)
	categoryDist := make(map[string]int)

	// Check if drift_trends table exists
	var count int
	err := h.service.db.WithContext(ctx).Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='drift_trends'").Scan(&count).Error
	if err != nil || count == 0 {
		// Table doesn't exist, return empty metrics
		return &DriftMetrics{
			TotalDriftIssues:     0,
			SeverityDistribution: severityDist,
			CategoryDistribution: categoryDist,
			TrendAnalysis:        []DriftTrend{},
		}, nil
	}

	if err := h.service.db.WithContext(ctx).Table("drift_trends").Where("resolved = ?", false).Count(&totalDrift).Error; err != nil {
		totalDrift = 0
	}

	// Get severity distribution
	var severityResults []struct {
		Severity string
		Count    int
	}
	if err := h.service.db.WithContext(ctx).Table("drift_trends").
		Select("severity, COUNT(*) as count").
		Where("resolved = ?", false).
		Group("severity").
		Scan(&severityResults).Error; err == nil {
		for _, result := range severityResults {
			severityDist[result.Severity] = result.Count
		}
	}

	// Get category distribution
	var categoryResults []struct {
		Category string
		Count    int
	}
	if err := h.service.db.WithContext(ctx).Table("drift_trends").
		Select("category, COUNT(*) as count").
		Where("resolved = ?", false).
		Group("category").
		Scan(&categoryResults).Error; err == nil {
		for _, result := range categoryResults {
			categoryDist[result.Category] = result.Count
		}
	}

	return &DriftMetrics{
		TotalDriftIssues:     int(totalDrift),
		SeverityDistribution: severityDist,
		CategoryDistribution: categoryDist,
		TrendAnalysis:        []DriftTrend{}, // Simplified for now
	}, nil
}

func (h *WebSocketHub) getNotificationMetrics(ctx context.Context) (*NotificationMetrics, error) {
	// Check if notification_histories table exists
	var count int
	err := h.service.db.WithContext(ctx).Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='notification_histories'").Scan(&count).Error
	if err != nil || count == 0 {
		// Table doesn't exist, return empty metrics
		return &NotificationMetrics{
			TotalSent:           0,
			TotalFailed:         0,
			ChannelBreakdown:    make(map[string]int),
			AlertLevelBreakdown: make(map[string]int),
			AverageLatency:      0,
		}, nil
	}

	var totalSent, totalFailed int64
	channelBreakdown := make(map[string]int)
	alertLevelBreakdown := make(map[string]int)

	// Count successful notifications (status = 'sent')
	if err := h.service.db.WithContext(ctx).Table("notification_histories").Where("status = ?", "sent").Count(&totalSent).Error; err != nil {
		totalSent = 0
	}

	// Count failed notifications (status = 'failed')
	if err := h.service.db.WithContext(ctx).Table("notification_histories").Where("status = ?", "failed").Count(&totalFailed).Error; err != nil {
		totalFailed = 0
	}

	return &NotificationMetrics{
		TotalSent:           int(totalSent),
		TotalFailed:         int(totalFailed),
		ChannelBreakdown:    channelBreakdown,
		AlertLevelBreakdown: alertLevelBreakdown,
		AverageLatency:      0, // Simplified
	}, nil
}

func (h *WebSocketHub) getResolutionMetrics(ctx context.Context) (*ResolutionMetrics, error) {
	// Check if resolution_histories table exists
	var count int
	err := h.service.db.WithContext(ctx).Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='resolution_histories'").Scan(&count).Error
	if err != nil || count == 0 {
		// Table doesn't exist, return empty metrics
		return &ResolutionMetrics{
			TotalResolutions:      0,
			AutoFixSuccessRate:    make(map[string]float64),
			ResolutionsByCategory: make(map[string]int),
			AverageReviewTime:     0,
		}, nil
	}

	var totalResolutions int64
	autoFixSuccessRate := make(map[string]float64)
	resolutionsByCategory := make(map[string]int)

	// Count total resolutions
	if err := h.service.db.WithContext(ctx).Table("resolution_histories").Count(&totalResolutions).Error; err != nil {
		totalResolutions = 0
	}

	return &ResolutionMetrics{
		TotalResolutions:      int(totalResolutions),
		AutoFixSuccessRate:    autoFixSuccessRate,
		ResolutionsByCategory: resolutionsByCategory,
		AverageReviewTime:     0, // Simplified
	}, nil
}
