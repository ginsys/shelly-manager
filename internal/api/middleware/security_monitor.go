package middleware

import (
	"fmt"
	"maps"
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
	logger       *logging.Logger
	config       *SecurityConfig
	attackMap    sync.Map           // map[string]*AttackInfo - tracks attacks by IP
	alerts       chan SecurityAlert // internal processing channel
	alertsPublic chan SecurityAlert // public fan-out for consumers/tests
	statistics   *SecurityStatistics
	done         chan struct{} // channel to signal shutdown
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

	// mu guards every field above and below against concurrent use of one IP's entry. Every reader and
	// writer takes it: TrackRequest, TrackRateLimitViolation, IsIPBlocked, GetMetrics and the cleanup.
	// The cleanup deletes the entry from attackMap while holding it, so a writer that must count into a
	// live entry takes it through lockedAttackInfo, which checks the map still holds it.
	// Lock order: mu first, then SecurityStatistics.mutex (the writers take the statistics lock while
	// holding mu), so nothing may take mu while holding the statistics lock.
	mu sync.Mutex
	// recent holds the times of the last HourlyThreshold+1 suspicious requests (a ring), for the
	// rolling-hour rule of shouldBlockIP; it is allocated on the first write. recentNext is the slot
	// the next write replaces; recentCount is how many slots are filled.
	recent      []time.Time
	recentNext  int
	recentCount int
}

// blockedAt reports whether a block of this IP is still running at now. A Blocked flag whose time has
// passed is not a block: IsIPBlocked clears it only when somebody asks. The caller holds mu.
func (a *AttackInfo) blockedAt(now time.Time) bool {
	return a.Blocked && now.Before(a.BlockedUntil)
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
		logger:       logger,
		config:       config,
		alerts:       make(chan SecurityAlert, 100),
		alertsPublic: make(chan SecurityAlert, 100),
		done:         make(chan struct{}),
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

// isTrustedIP checks if an IP is in the trusted IPs list (e.g. localhost).
// Trusted IPs are exempt from suspicious activity tracking and blocking.
func isTrustedIP(ip string, trustedIPs []string) bool {
	for _, trusted := range trustedIPs {
		if ip == trusted {
			return true
		}
	}
	return false
}

// TrackRequest records a request for security monitoring
func (sm *SecurityMonitor) TrackRequest(r *http.Request, statusCode int, duration time.Duration) {
	clientIP := getClientIP(r)

	sm.statistics.mutex.Lock()
	sm.statistics.TotalRequests++
	sm.statistics.LastUpdated = time.Now()
	sm.statistics.mutex.Unlock()

	// Skip suspicious activity tracking for trusted IPs (e.g. localhost)
	if isTrustedIP(clientIP, sm.config.TrustedIPs) {
		return
	}

	attackInfo := sm.lockedAttackInfo(clientIP)
	defer attackInfo.mu.Unlock()

	now := time.Now()
	attackInfo.LastSeen = now
	attackInfo.TotalRequests++

	// A request admitted before the block and finished during it still counts in the monitor's own
	// statistics, which are history, but not in this IP's counters: the block reset them, and this
	// request must not start the next block's count.
	inBlock := attackInfo.blockedAt(now)

	// A 429 is a rate limit violation and counts once, as that: isSuspiciousRequest leaves its status
	// out. The limiter's 429 reaches the monitor here, through MonitoringMiddleware.
	if statusCode == http.StatusTooManyRequests {
		sm.recordRateLimitViolation(attackInfo, r, clientIP, inBlock)
	}

	// Detect suspicious patterns
	if sm.isSuspiciousRequest(r, statusCode) {
		if !inBlock {
			attackInfo.SuspiciousRequests++
			attackInfo.recordSuspiciousTime(now, sm.config.HourlyThreshold)
		}
		sm.statistics.mutex.Lock()
		sm.statistics.SuspiciousRequests++
		sm.statistics.mutex.Unlock()

		attackType := sm.detectAttackType(r)
		if attackType == "general_suspicious" {
			// Fallback heuristics for SQL injection patterns in params
			q := r.URL.Query()
			for _, vs := range q {
				for _, v := range vs {
					lv := strings.ToLower(v)
					if (strings.Contains(lv, " or ") && strings.Contains(lv, "1=1")) || strings.Contains(lv, "' or ") {
						attackType = "sql_injection"
						break
					}
				}
			}
		}
		if !inBlock {
			attackInfo.AttackTypes[attackType]++
		}

		sm.statistics.mutex.Lock()
		sm.statistics.AttacksByType[attackType]++
		sm.statistics.TopAttackerIPs[clientIP]++
		sm.statistics.mutex.Unlock()

		// Generate security alert
		sm.generateAlert(r, attackType, clientIP, statusCode)
	}

	sm.blockIfDue(attackInfo, clientIP, now)
}

// TrackRateLimitViolation records one rate limit violation for a refusal that MonitoringMiddleware
// does not see. A 429 that passes MonitoringMiddleware is already counted by TrackRequest: calling
// this for it as well would count the refusal twice and block the IP at the 3rd one. Nothing in
// production calls it today.
func (sm *SecurityMonitor) TrackRateLimitViolation(r *http.Request) {
	clientIP := getClientIP(r)

	if isTrustedIP(clientIP, sm.config.TrustedIPs) {
		return
	}

	attackInfo := sm.lockedAttackInfo(clientIP)
	defer attackInfo.mu.Unlock()

	now := time.Now()
	attackInfo.LastSeen = now // the cleanup drops an entry by LastSeen
	sm.recordRateLimitViolation(attackInfo, r, clientIP, attackInfo.blockedAt(now))
	sm.blockIfDue(attackInfo, clientIP, now)
}

// recordRateLimitViolation counts one rate limit violation in the monitor's statistics, and in the
// IP's counters unless a block is running (inBlock, see TrackRequest). The caller holds attackInfo.mu.
func (sm *SecurityMonitor) recordRateLimitViolation(attackInfo *AttackInfo, r *http.Request, clientIP string, inBlock bool) {
	if !inBlock {
		attackInfo.RateLimitViolations++
	}

	sm.statistics.mutex.Lock()
	sm.statistics.RateLimitViolations++
	sm.statistics.mutex.Unlock()

	sm.generateAlert(r, "rate_limit_violation", clientIP, http.StatusTooManyRequests)
}

// lockedAttackInfo returns the live tracking entry of an IP with its mu held; the caller unlocks it.
func (sm *SecurityMonitor) lockedAttackInfo(clientIP string) *AttackInfo {
	return sm.lockLive(sm.attackInfoFor(clientIP), clientIP)
}

// lockLive locks info and returns it if the map still holds it for the IP. The cleanup deletes an
// entry only while holding its mu, so once the lock is held the answer cannot change until unlock. An
// entry the cleanup deleted between the caller's load and this lock is replaced by the live one, so a
// request is never counted into an entry nobody reads again.
func (sm *SecurityMonitor) lockLive(info *AttackInfo, clientIP string) *AttackInfo {
	for {
		info.mu.Lock()
		if value, ok := sm.attackMap.Load(clientIP); ok && value == info {
			return info
		}
		info.mu.Unlock()
		info = sm.attackInfoFor(clientIP)
	}
}

// attackInfoFor returns the tracking entry of an IP and creates it on first sight. The entry can be
// deleted by the cleanup at any moment: take mu through lockedAttackInfo, never by locking what this
// returns.
func (sm *SecurityMonitor) attackInfoFor(clientIP string) *AttackInfo {
	if value, exists := sm.attackMap.Load(clientIP); exists {
		return value.(*AttackInfo)
	}
	now := time.Now()
	// LastSeen is set before the entry is published: the cleanup drops an entry by LastSeen and may
	// run the moment the entry is visible.
	value, _ := sm.attackMap.LoadOrStore(clientIP, &AttackInfo{
		IP:          clientIP,
		FirstSeen:   now,
		LastSeen:    now,
		AttackTypes: make(map[string]int),
	})
	return value.(*AttackInfo)
}

// recordSuspiciousTime notes when a suspicious request came, keeping the last limit+1. A limit of zero
// or less switches the hourly rule off, so nothing is kept. The caller holds attackInfo.mu.
func (a *AttackInfo) recordSuspiciousTime(now time.Time, limit int) {
	if limit <= 0 {
		return
	}
	if a.recent == nil {
		a.recent = make([]time.Time, limit+1)
	}
	a.recent[a.recentNext] = now
	a.recentNext = (a.recentNext + 1) % len(a.recent)
	if a.recentCount < len(a.recent) {
		a.recentCount++
	}
}

// suspiciousInLastHour reports whether more than limit suspicious requests came inside the hour up to
// now. The caller holds attackInfo.mu.
func (a *AttackInfo) suspiciousInLastHour(now time.Time, limit int) bool {
	if limit <= 0 || a.recentCount < limit+1 {
		return false
	}
	// The ring is full, so recentNext is the slot of the oldest of the last limit+1 times.
	return now.Sub(a.recent[a.recentNext]) <= time.Hour
}

// IsIPBlocked checks if an IP is currently blocked
func (sm *SecurityMonitor) IsIPBlocked(ip string) bool {
	if value, exists := sm.attackMap.Load(ip); exists {
		attackInfo := value.(*AttackInfo)
		attackInfo.mu.Lock()
		defer attackInfo.mu.Unlock()
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

// GetMetrics returns a snapshot of the security metrics. Every map in it is a copy, because the caller
// encodes the result after GetMetrics returns, while TrackRequest keeps writing the originals.
func (sm *SecurityMonitor) GetMetrics() SecurityMetrics {
	// The per-IP pass takes each entry's lock and so must not run under the statistics lock: the
	// writers take the entry's lock first and the statistics lock second.
	topAttackers := make([]IPAttackSummary, 0)
	blockedCount := 0
	activeThreats := 0
	sm.attackMap.Range(func(key, value interface{}) bool {
		info := value.(*AttackInfo)
		info.mu.Lock()
		defer info.mu.Unlock()

		// A block resets the counters, so a blocked IP is listed for as long as the block runs, with
		// the counters as reset, and counts as an active threat whatever its LastSeen.
		blocked := info.blockedAt(time.Now())
		if blocked {
			blockedCount++
		}
		if info.SuspiciousRequests > 0 || blocked {
			if blocked || time.Since(info.LastSeen) < time.Hour {
				activeThreats++
			}
			if len(topAttackers) < 10 { // Limit to top 10
				topAttackers = append(topAttackers, IPAttackSummary{
					IP:                 key.(string),
					TotalRequests:      info.TotalRequests,
					SuspiciousRequests: info.SuspiciousRequests,
					AttackTypes:        maps.Clone(info.AttackTypes),
					FirstSeen:          info.FirstSeen,
					LastSeen:           info.LastSeen,
					Blocked:            blocked,
				})
			}
		}
		return true
	})

	sm.statistics.mutex.RLock()
	defer sm.statistics.mutex.RUnlock()
	return SecurityMetrics{
		TotalRequests:       sm.statistics.TotalRequests,
		BlockedRequests:     sm.statistics.BlockedRequests,
		SuspiciousRequests:  sm.statistics.SuspiciousRequests,
		RateLimitViolations: sm.statistics.RateLimitViolations,
		ValidationFailures:  sm.statistics.ValidationFailures,
		AttacksByType:       maps.Clone(sm.statistics.AttacksByType),
		TopAttackerIPs:      topAttackers,
		AlertsByLevel:       maps.Clone(sm.statistics.AlertsByLevel),
		LastUpdated:         sm.statistics.LastUpdated,
		BlockedIPs:          blockedCount,
		ActiveThreats:       activeThreats,
	}
}

// GetAlerts returns the alerts channel for monitoring
func (sm *SecurityMonitor) GetAlerts() <-chan SecurityAlert {
	return sm.alertsPublic
}

// Close shuts down the security monitor and stops background goroutines
func (sm *SecurityMonitor) Close() {
	close(sm.done)
	close(sm.alerts)
	close(sm.alertsPublic)
}

// Private methods

func (sm *SecurityMonitor) isSuspiciousRequest(r *http.Request, statusCode int) bool {
	// Check for suspicious patterns
	if detectSuspiciousRequest(r) {
		return true
	}

	// Check for error responses that might indicate attacks.
	// Exclude 404 (not found), 401 (unauthorized), and 408 (request timeout)
	// as these are normal operational responses, not attack indicators.
	// Timeouts (408) commonly occur when communicating with offline devices.
	// Exclude 429 too: TrackRequest counts it once, as a rate limit violation.
	if statusCode >= 400 && statusCode != 404 && statusCode != 401 && statusCode != 408 &&
		statusCode != http.StatusTooManyRequests {
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

	// Extract parsed query params for robust matching on decoded values
	parsed := r.URL.Query()

	// SQL Injection (robust checks including decoded param values)
	// Quick heuristic: encoded or decoded quote plus boolean OR in query
	if (strings.Contains(query, "%27") || strings.Contains(decodedQuery, "'")) && (strings.Contains(query, "%20or%20") || strings.Contains(decodedQuery, " or ")) {
		return "sql_injection"
	}
	// Heuristic: look for classic boolean-based patterns within any param value
	for _, vs := range parsed {
		for _, v := range vs {
			lv := strings.ToLower(v)
			if (strings.Contains(lv, " or ") && strings.Contains(lv, "1=1")) || strings.Contains(lv, "' or ") {
				return "sql_injection"
			}
		}
	}

	// Pattern-based matching (raw/decoded)
	sqlPatterns := []string{
		"' or ", " or 1=1", "union select", "drop table", "delete from", "insert into",
		// common encoded variants seen in tests/inputs
		"%27%20or%201%3d1", "%27%20or%201=1", "or%201%3d1",
	}
	for _, pattern := range sqlPatterns {
		if strings.Contains(path, pattern) || strings.Contains(query, pattern) || strings.Contains(decodedQuery, pattern) {
			return "sql_injection"
		}
		for k, vs := range parsed {
			lk := strings.ToLower(k)
			if strings.Contains(lk, pattern) {
				return "sql_injection"
			}
			for _, v := range vs {
				lv := strings.ToLower(v)
				if strings.Contains(lv, pattern) {
					return "sql_injection"
				}
			}
		}
	}
	// Heuristic: common boolean-based SQLi even if not matched above
	if (strings.Contains(decodedQuery, " or 1=1") || strings.Contains(query, " or 1=1")) ||
		(strings.Contains(decodedQuery, "' or ") || strings.Contains(query, "' or ")) {
		return "sql_injection"
	}
	ustr := strings.ToLower(r.URL.String())
	if strings.Contains(ustr, "%27%20or%201%3d1") || strings.Contains(ustr, "' or ") || strings.Contains(ustr, " or 1=1") {
		return "sql_injection"
	}

	// XSS
	xssPatterns := []string{"<script>", "javascript:", "onerror=", "onload=", "eval(", "alert("}
	for _, pattern := range xssPatterns {
		if strings.Contains(path, pattern) || strings.Contains(query, pattern) || strings.Contains(decodedQuery, pattern) {
			return "xss_attempt"
		}
		for _, vs := range parsed {
			for _, v := range vs {
				if strings.Contains(strings.ToLower(v), pattern) {
					return "xss_attempt"
				}
			}
		}
	}

	// Path Traversal
	if strings.Contains(path, "..") || strings.Contains(query, "..") || strings.Contains(decodedQuery, "..") {
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

// shouldBlockIP applies the block rules, each at its configured threshold. The caller holds
// attackInfo.mu.
func (sm *SecurityMonitor) shouldBlockIP(attackInfo *AttackInfo, now time.Time) bool {
	c := sm.config
	if c.SuspiciousThreshold > 0 && attackInfo.SuspiciousRequests >= c.SuspiciousThreshold {
		return true
	}
	if c.RateViolationThreshold > 0 && attackInfo.RateLimitViolations >= c.RateViolationThreshold {
		return true
	}
	if c.AttackTypeThreshold > 0 && len(attackInfo.AttackTypes) >= c.AttackTypeThreshold {
		return true
	}
	return attackInfo.suspiciousInLastHour(now, c.HourlyThreshold)
}

// blockIfDue blocks the IP when blocking is on, no block is running and a rule fires. With blocking
// off the monitor only counts: nothing is blocked, logged as blocked, or reset. The caller holds
// attackInfo.mu.
func (sm *SecurityMonitor) blockIfDue(attackInfo *AttackInfo, ip string, now time.Time) {
	if sm.config.EnableIPBlocking && !attackInfo.blockedAt(now) && sm.shouldBlockIP(attackInfo, now) {
		sm.blockIP(attackInfo, ip, now)
	}
}

// blockIP starts a block of the configured length and starts the IP's counting over, so the block
// does not leave a counter at the limit for the first request after it. Statistics of the whole
// monitor are history and are not reset. The caller holds attackInfo.mu.
func (sm *SecurityMonitor) blockIP(attackInfo *AttackInfo, ip string, now time.Time) {
	// The log reads the counters that caused the block, so it goes before the reset.
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

	attackInfo.Blocked = true
	attackInfo.BlockedUntil = now.Add(sm.config.BlockDuration)
	attackInfo.SuspiciousRequests = 0
	attackInfo.RateLimitViolations = 0
	attackInfo.AttackTypes = make(map[string]int)
	attackInfo.recent, attackInfo.recentNext, attackInfo.recentCount = nil, 0, 0

	sm.statistics.mutex.Lock()
	sm.statistics.BlockedRequests++
	sm.statistics.mutex.Unlock()
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

	// Send alert to internal processor (non-blocking)
	select {
	case sm.alerts <- alert:
	default:
		if sm.logger != nil {
			sm.logger.Warn("Security alert channel is full, dropping alert", "alert_type", attackType)
		}
	}

	// Fan-out to public channel (non-blocking) for external consumers/tests
	select {
	case sm.alertsPublic <- alert:
	default:
		// drop if full to avoid blocking
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
			sm.cleanupOldDataOnce(time.Now().Add(-24 * time.Hour))
		case <-sm.done:
			return // Shutdown signal received
		}
	}
}

// cleanupOldDataOnce performs a single-pass cleanup using the provided cutoff.
// This is separated for testability to avoid blocking ticker loops in tests.
func (sm *SecurityMonitor) cleanupOldDataOnce(cutoff time.Time) {
	sm.attackMap.Range(func(key, value interface{}) bool {
		ip := key.(string)
		info := value.(*AttackInfo)

		// Remove any entry whose last activity is before the cutoff, except one whose block is still
		// running: a blocked client's requests stop before the monitor, so its LastSeen does not move
		// and a block longer than the cutoff would end early.
		info.mu.Lock()
		defer info.mu.Unlock()
		if info.LastSeen.Before(cutoff) && !info.blockedAt(time.Now()) {
			sm.attackMap.Delete(ip)
		}
		return true
	})
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
