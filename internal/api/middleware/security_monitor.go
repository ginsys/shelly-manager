package middleware

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/ginsys/shelly-manager/internal/api/response"
	"github.com/ginsys/shelly-manager/internal/logging"
)

// SecurityMonitor tracks security events and suspicious activities
type SecurityMonitor struct {
	logger     *logging.Logger
	config     *SecurityConfig
	attackMap  sync.Map // map[string]*AttackInfo - tracks attacks by IP
	alerts     chan SecurityAlert
	statistics *SecurityStatistics
	done       chan struct{} // channel to signal shutdown
}

// AttackInfo tracks attack patterns from specific IPs
type AttackInfo struct {
	IP                  string
	FirstSeen           time.Time
	LastSeen            time.Time
	TotalRequests       int
	SuspiciousRequests  int
	RateLimitViolations int
	ValidationFailures  int
	AttackTypes         map[string]int // attack type -> count
	Blocked             bool
	BlockedUntil        time.Time
}

// SecurityAlert represents a security alert
type SecurityAlert struct {
	Level     string                 `json:"level"`      // CRITICAL, HIGH, MEDIUM, LOW
	Type      string                 `json:"type"`       // Attack type
	Message   string                 `json:"message"`    // Alert message
	ClientIP  string                 `json:"client_ip"`  // Source IP
	Path      string                 `json:"path"`       // Requested path
	UserAgent string                 `json:"user_agent"` // User agent
	Timestamp time.Time              `json:"timestamp"`  // When it occurred
	RequestID string                 `json:"request_id"` // Request ID for correlation
	Details   map[string]interface{} `json:"details"`    // Additional context
}

// SecurityStatistics tracks overall security metrics
type SecurityStatistics struct {
	mutex               sync.RWMutex
	TotalRequests       int64
	BlockedRequests     int64
	SuspiciousRequests  int64
	RateLimitViolations int64
	ValidationFailures  int64
	AttacksByType       map[string]int64
	TopAttackerIPs      map[string]int64
	AlertsByLevel       map[string]int64
	LastUpdated         time.Time
}

// SecurityMetrics provides security metrics for monitoring
type SecurityMetrics struct {
	TotalRequests       int64             `json:"total_requests"`
	BlockedRequests     int64             `json:"blocked_requests"`
	SuspiciousRequests  int64             `json:"suspicious_requests"`
	RateLimitViolations int64             `json:"rate_limit_violations"`
	ValidationFailures  int64             `json:"validation_failures"`
	AttacksByType       map[string]int64  `json:"attacks_by_type"`
	TopAttackerIPs      []IPAttackSummary `json:"top_attacker_ips"`
	AlertsByLevel       map[string]int64  `json:"alerts_by_level"`
	LastUpdated         time.Time         `json:"last_updated"`
	BlockedIPs          int               `json:"blocked_ips"`
	ActiveThreats       int               `json:"active_threats"`
}

// IPAttackSummary provides attack summary for an IP
type IPAttackSummary struct {
	IP                 string         `json:"ip"`
	TotalRequests      int            `json:"total_requests"`
	SuspiciousRequests int            `json:"suspicious_requests"`
	AttackTypes        map[string]int `json:"attack_types"`
	FirstSeen          time.Time      `json:"first_seen"`
	LastSeen           time.Time      `json:"last_seen"`
	Blocked            bool           `json:"blocked"`
}

// NewSecurityMonitor creates a new security monitor
func NewSecurityMonitor(config *SecurityConfig, logger *logging.Logger) *SecurityMonitor {
	sm := &SecurityMonitor{
		logger: logger,
		config: config,
		alerts: make(chan SecurityAlert, 100),
		done:   make(chan struct{}),
		statistics: &SecurityStatistics{
			AttacksByType:  make(map[string]int64),
			TopAttackerIPs: make(map[string]int64),
			AlertsByLevel:  make(map[string]int64),
			LastUpdated:    time.Now(),
		},
	}

	// Start alert processor
	go sm.processAlerts()

	// Start cleanup routine
	go sm.cleanupOldData()

	return sm
}

// TrackRequest records a request for security monitoring
func (sm *SecurityMonitor) TrackRequest(r *http.Request, statusCode int, duration time.Duration) {
	clientIP := getClientIP(r)

	sm.statistics.mutex.Lock()
	sm.statistics.TotalRequests++
	sm.statistics.LastUpdated = time.Now()
	sm.statistics.mutex.Unlock()

	// Load or create attack info
	var attackInfo *AttackInfo
	if value, exists := sm.attackMap.Load(clientIP); exists {
		attackInfo = value.(*AttackInfo)
	} else {
		attackInfo = &AttackInfo{
			IP:          clientIP,
			FirstSeen:   time.Now(),
			AttackTypes: make(map[string]int),
		}
		sm.attackMap.Store(clientIP, attackInfo)
	}

	attackInfo.LastSeen = time.Now()
	attackInfo.TotalRequests++

	// Detect suspicious patterns
	if sm.isSuspiciousRequest(r, statusCode) {
		attackInfo.SuspiciousRequests++
		sm.statistics.mutex.Lock()
		sm.statistics.SuspiciousRequests++
		sm.statistics.mutex.Unlock()

		attackType := sm.detectAttackType(r)
		attackInfo.AttackTypes[attackType]++

		sm.statistics.mutex.Lock()
		sm.statistics.AttacksByType[attackType]++
		sm.statistics.TopAttackerIPs[clientIP]++
		sm.statistics.mutex.Unlock()

		// Generate security alert
		sm.generateAlert(r, attackType, clientIP, statusCode)
	}

	// Check if IP should be blocked
	if sm.shouldBlockIP(attackInfo) && !attackInfo.Blocked {
		sm.blockIP(attackInfo, clientIP)
	}
}

// TrackRateLimitViolation records a rate limit violation
func (sm *SecurityMonitor) TrackRateLimitViolation(r *http.Request) {
	clientIP := getClientIP(r)

	if value, exists := sm.attackMap.Load(clientIP); exists {
		attackInfo := value.(*AttackInfo)
		attackInfo.RateLimitViolations++
	}

	sm.statistics.mutex.Lock()
	sm.statistics.RateLimitViolations++
	sm.statistics.mutex.Unlock()

	sm.generateAlert(r, "rate_limit_violation", clientIP, http.StatusTooManyRequests)
}

// TrackValidationFailure records a validation failure
func (sm *SecurityMonitor) TrackValidationFailure(r *http.Request, validationType string) {
	clientIP := getClientIP(r)

	if value, exists := sm.attackMap.Load(clientIP); exists {
		attackInfo := value.(*AttackInfo)
		attackInfo.ValidationFailures++
	}

	sm.statistics.mutex.Lock()
	sm.statistics.ValidationFailures++
	sm.statistics.mutex.Unlock()

	sm.generateAlert(r, fmt.Sprintf("validation_failure_%s", validationType), clientIP, http.StatusBadRequest)
}

// IsIPBlocked checks if an IP is currently blocked
func (sm *SecurityMonitor) IsIPBlocked(ip string) bool {
	if value, exists := sm.attackMap.Load(ip); exists {
		attackInfo := value.(*AttackInfo)
		if attackInfo.Blocked {
			if time.Now().Before(attackInfo.BlockedUntil) {
				return true
			} else {
				// Unblock if time has passed
				attackInfo.Blocked = false
				attackInfo.BlockedUntil = time.Time{}
			}
		}
	}
	return false
}

// GetMetrics returns current security metrics
func (sm *SecurityMonitor) GetMetrics() SecurityMetrics {
	sm.statistics.mutex.RLock()
	defer sm.statistics.mutex.RUnlock()

	// Build top attacker IPs
	topAttackers := make([]IPAttackSummary, 0)
	sm.attackMap.Range(func(key, value interface{}) bool {
		ip := key.(string)
		info := value.(*AttackInfo)

		if info.SuspiciousRequests > 0 {
			topAttackers = append(topAttackers, IPAttackSummary{
				IP:                 ip,
				TotalRequests:      info.TotalRequests,
				SuspiciousRequests: info.SuspiciousRequests,
				AttackTypes:        info.AttackTypes,
				FirstSeen:          info.FirstSeen,
				LastSeen:           info.LastSeen,
				Blocked:            info.Blocked,
			})
		}
		return len(topAttackers) < 10 // Limit to top 10
	})

	// Count blocked IPs and active threats
	blockedCount := 0
	activeThreats := 0
	sm.attackMap.Range(func(key, value interface{}) bool {
		info := value.(*AttackInfo)
		if info.Blocked {
			blockedCount++
		}
		if info.SuspiciousRequests > 0 && time.Since(info.LastSeen) < time.Hour {
			activeThreats++
		}
		return true
	})

	return SecurityMetrics{
		TotalRequests:       sm.statistics.TotalRequests,
		BlockedRequests:     sm.statistics.BlockedRequests,
		SuspiciousRequests:  sm.statistics.SuspiciousRequests,
		RateLimitViolations: sm.statistics.RateLimitViolations,
		ValidationFailures:  sm.statistics.ValidationFailures,
		AttacksByType:       sm.statistics.AttacksByType,
		TopAttackerIPs:      topAttackers,
		AlertsByLevel:       sm.statistics.AlertsByLevel,
		LastUpdated:         sm.statistics.LastUpdated,
		BlockedIPs:          blockedCount,
		ActiveThreats:       activeThreats,
	}
}

// GetAlerts returns the alerts channel for monitoring
func (sm *SecurityMonitor) GetAlerts() <-chan SecurityAlert {
	return sm.alerts
}

// Close shuts down the security monitor and stops background goroutines
func (sm *SecurityMonitor) Close() {
	close(sm.done)
	close(sm.alerts)
}

// Private methods

func (sm *SecurityMonitor) isSuspiciousRequest(r *http.Request, statusCode int) bool {
	// Check for suspicious patterns
	if detectSuspiciousRequest(r) {
		return true
	}

	// Check for error responses that might indicate attacks
	if statusCode >= 400 && statusCode != 404 && statusCode != 401 {
		return true
	}

	// Check for suspicious user agents
	if isSuspiciousUserAgent(r.UserAgent()) {
		return true
	}

	// Check for suspicious headers
	for name, values := range r.Header {
		for _, value := range values {
			if containsSuspiciousContent(value) || isSuspiciousHeaderName(name) {
				return true
			}
		}
	}

	return false
}

func (sm *SecurityMonitor) detectAttackType(r *http.Request) string {
	path := strings.ToLower(r.URL.Path)
	query := strings.ToLower(r.URL.RawQuery)
	userAgent := strings.ToLower(r.UserAgent())

	// Also check URL-decoded query for better detection
	decodedQuery := ""
	if r.URL.RawQuery != "" {
		if decoded, err := url.QueryUnescape(r.URL.RawQuery); err == nil {
			decodedQuery = strings.ToLower(decoded)
		}
	}

	// SQL Injection
	sqlPatterns := []string{"' or ", " or 1=1", "union select", "drop table", "delete from", "insert into"}
	for _, pattern := range sqlPatterns {
		if strings.Contains(path, pattern) || strings.Contains(query, pattern) || strings.Contains(decodedQuery, pattern) {
			return "sql_injection"
		}
	}

	// XSS
	xssPatterns := []string{"<script>", "javascript:", "onerror=", "onload=", "eval(", "alert("}
	for _, pattern := range xssPatterns {
		if strings.Contains(path, pattern) || strings.Contains(query, pattern) || strings.Contains(decodedQuery, pattern) {
			return "xss_attempt"
		}
	}

	// Path Traversal
	if strings.Contains(path, "..") || strings.Contains(query, "..") {
		return "path_traversal"
	}

	// Scanner/Bot Detection
	scannerPatterns := []string{"sqlmap", "nikto", "nessus", "burpsuite", "nmap"}
	for _, pattern := range scannerPatterns {
		if strings.Contains(userAgent, pattern) {
			return "automated_scanner"
		}
	}

	// Suspicious user agent
	if isSuspiciousUserAgent(r.UserAgent()) {
		return "suspicious_user_agent"
	}

	// Large request (potential DoS)
	if r.ContentLength > 1024*1024 { // 1MB
		return "large_request"
	}

	return "general_suspicious"
}

func (sm *SecurityMonitor) shouldBlockIP(attackInfo *AttackInfo) bool {
	// Block if too many suspicious requests
	if attackInfo.SuspiciousRequests >= 10 {
		return true
	}

	// Block if too many rate limit violations
	if attackInfo.RateLimitViolations >= 5 {
		return true
	}

	// Block if multiple attack types
	if len(attackInfo.AttackTypes) >= 3 {
		return true
	}

	// Block if high attack rate
	timeDiff := time.Since(attackInfo.FirstSeen)
	if timeDiff > 0 && float64(attackInfo.SuspiciousRequests)/timeDiff.Hours() > 5 {
		return true
	}

	return false
}

func (sm *SecurityMonitor) blockIP(attackInfo *AttackInfo, ip string) {
	attackInfo.Blocked = true
	attackInfo.BlockedUntil = time.Now().Add(time.Hour) // Block for 1 hour

	sm.statistics.mutex.Lock()
	sm.statistics.BlockedRequests++
	sm.statistics.mutex.Unlock()

	if sm.logger != nil && sm.config.LogSecurityEvents {
		sm.logger.WithFields(map[string]any{
			"client_ip":           ip,
			"suspicious_requests": attackInfo.SuspiciousRequests,
			"rate_violations":     attackInfo.RateLimitViolations,
			"attack_types":        attackInfo.AttackTypes,
			"component":           "security_monitor",
			"security_event":      "ip_blocked",
		}).Warn("IP address blocked due to suspicious activity")
	}
}

func (sm *SecurityMonitor) generateAlert(r *http.Request, attackType, clientIP string, statusCode int) {
	level := sm.determineAlertLevel(attackType, statusCode)

	alert := SecurityAlert{
		Level:     level,
		Type:      attackType,
		Message:   fmt.Sprintf("%s detected from %s", attackType, clientIP),
		ClientIP:  clientIP,
		Path:      r.URL.Path,
		UserAgent: r.UserAgent(),
		Timestamp: time.Now(),
		RequestID: getRequestIDFromContext(r),
		Details: map[string]interface{}{
			"method":      r.Method,
			"status_code": statusCode,
			"query":       r.URL.RawQuery,
			"referer":     r.Header.Get("Referer"),
		},
	}

	sm.statistics.mutex.Lock()
	sm.statistics.AlertsByLevel[level]++
	sm.statistics.mutex.Unlock()

	// Send alert (non-blocking)
	select {
	case sm.alerts <- alert:
		// Alert sent successfully
	default:
		// Alert channel is full, log warning
		if sm.logger != nil {
			sm.logger.Warn("Security alert channel is full, dropping alert", "alert_type", attackType)
		}
	}
}

func (sm *SecurityMonitor) determineAlertLevel(attackType string, statusCode int) string {
	switch attackType {
	case "sql_injection", "xss_attempt", "path_traversal":
		return "CRITICAL"
	case "automated_scanner", "rate_limit_violation":
		return "HIGH"
	case "validation_failure_headers", "validation_failure_json":
		return "MEDIUM"
	default:
		return "LOW"
	}
}

func (sm *SecurityMonitor) processAlerts() {
	for {
		select {
		case alert, ok := <-sm.alerts:
			if !ok {
				return // Channel is closed
			}
			if sm.logger != nil && sm.config.LogSecurityEvents {
				sm.logger.WithFields(map[string]any{
					"alert_level":    alert.Level,
					"alert_type":     alert.Type,
					"client_ip":      alert.ClientIP,
					"path":           alert.Path,
					"user_agent":     alert.UserAgent,
					"request_id":     alert.RequestID,
					"component":      "security_alert",
					"security_event": "alert_generated",
				}).Warn(alert.Message)
			}
		case <-sm.done:
			return // Shutdown signal received
		}
	}
}

func (sm *SecurityMonitor) cleanupOldData() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cutoff := time.Now().Add(-24 * time.Hour) // Remove data older than 24 hours

			sm.attackMap.Range(func(key, value interface{}) bool {
				ip := key.(string)
				info := value.(*AttackInfo)

				// Remove old entries that are not blocked and have no recent activity
				if !info.Blocked && info.LastSeen.Before(cutoff) {
					sm.attackMap.Delete(ip)
				}
				return true
			})
		case <-sm.done:
			return // Shutdown signal received
		}
	}
}

func isSuspiciousHeaderName(name string) bool {
	suspicious := []string{
		"x-forwarded-proto", "x-forwarded-host", "x-original-url",
		"x-rewrite-url", "x-real-ip", "client-ip",
	}

	lowerName := strings.ToLower(name)
	for _, pattern := range suspicious {
		if strings.Contains(lowerName, pattern) {
			return true
		}
	}
	return false
}

// getRequestIDFromContext extracts request ID from request context
func getRequestIDFromContext(r *http.Request) string {
	if ctx := r.Context(); ctx != nil {
		if requestID, ok := ctx.Value(response.RequestIDKey).(string); ok {
			return requestID
		}
	}
	return ""
}
