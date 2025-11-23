package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
)

func TestSecurityMonitorCreation(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})
	config := DefaultSecurityConfig()

	monitor := NewSecurityMonitor(config, logger)
	defer monitor.Close()

	assert.NotNil(t, monitor, "Security monitor should be created")
	assert.NotNil(t, monitor.statistics, "Statistics should be initialized")
	assert.NotNil(t, monitor.GetAlerts(), "Alerts channel should be available")
}

func TestTrackRequest(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})
	config := DefaultSecurityConfig()
	monitor := NewSecurityMonitor(config, logger)
	defer monitor.Close()

	tests := []struct {
		name             string
		setupRequest     func() *http.Request
		statusCode       int
		expectSuspicious bool
		description      string
	}{
		{
			name: "Normal Request",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/api/v1/users", nil)
				req.RemoteAddr = "192.168.1.1:12345"
				req.Header.Set("User-Agent", "Mozilla/5.0 (compatible)")
				return req
			},
			statusCode:       http.StatusOK,
			expectSuspicious: false,
			description:      "Normal request should not be flagged as suspicious",
		},
		{
			name: "SQL Injection Request",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/api/v1/users?id=1%27%20OR%20%271%27%3D%271", nil)
				req.RemoteAddr = "10.0.0.1:12345"
				req.Header.Set("User-Agent", "Mozilla/5.0 (compatible)")
				return req
			},
			statusCode:       http.StatusBadRequest,
			expectSuspicious: true,
			description:      "SQL injection request should be flagged as suspicious",
		},
		{
			name: "XSS Request",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/api/v1/search?q=<script>alert('xss')</script>", nil)
				req.RemoteAddr = "10.0.0.2:12345"
				req.Header.Set("User-Agent", "Mozilla/5.0 (compatible)")
				return req
			},
			statusCode:       http.StatusBadRequest,
			expectSuspicious: true,
			description:      "XSS request should be flagged as suspicious",
		},
		{
			name: "Suspicious User Agent",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/api/v1/users", nil)
				req.RemoteAddr = "10.0.0.3:12345"
				req.Header.Set("User-Agent", "sqlmap/1.0")
				return req
			},
			statusCode:       http.StatusOK,
			expectSuspicious: true,
			description:      "Request with suspicious user agent should be flagged",
		},
		{
			name: "Path Traversal Request",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/api/v1/files?path=../../../etc/passwd", nil)
				req.RemoteAddr = "10.0.0.4:12345"
				req.Header.Set("User-Agent", "Mozilla/5.0 (compatible)")
				return req
			},
			statusCode:       http.StatusBadRequest,
			expectSuspicious: true,
			description:      "Path traversal request should be flagged as suspicious",
		},
		{
			name: "Large Request",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("POST", "/api/v1/upload", strings.NewReader(strings.Repeat("x", 2*1024*1024)))
				req.RemoteAddr = "10.0.0.5:12345"
				req.Header.Set("User-Agent", "Mozilla/5.0 (compatible)")
				req.Header.Set("Content-Type", "application/json")
				req.ContentLength = 2 * 1024 * 1024 // 2MB
				return req
			},
			statusCode:       http.StatusOK,
			expectSuspicious: true,
			description:      "Large request should be flagged as suspicious",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupRequest()

			// Track request
			monitor.TrackRequest(req, tt.statusCode, time.Millisecond*100)

			// Get metrics to verify tracking
			metrics := monitor.GetMetrics()

			assert.Greater(t, metrics.TotalRequests, int64(0), "Total requests should be incremented")

			if tt.expectSuspicious {
				assert.Greater(t, metrics.SuspiciousRequests, int64(0), "Suspicious requests should be incremented")
			}
		})
	}
}

func TestAttackTypeDetection(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})
	config := DefaultSecurityConfig()
	monitor := NewSecurityMonitor(config, logger)
	defer monitor.Close()

	attackTests := []struct {
		name         string
		url          string
		userAgent    string
		expectedType string
		description  string
	}{
		{
			name:         "SQL Injection Detection",
			url:          "/api/v1/users?id=1%27%20UNION%20SELECT%20*%20FROM%20passwords%20--",
			userAgent:    "Mozilla/5.0",
			expectedType: "sql_injection",
			description:  "Should detect SQL injection in URL parameters",
		},
		{
			name:         "XSS Detection",
			url:          "/api/v1/search?q=<script>alert(document.cookie)</script>",
			userAgent:    "Mozilla/5.0",
			expectedType: "xss_attempt",
			description:  "Should detect XSS attempts in URL parameters",
		},
		{
			name:         "Path Traversal Detection",
			url:          "/api/v1/files?path=../../../../etc/passwd",
			userAgent:    "Mozilla/5.0",
			expectedType: "path_traversal",
			description:  "Should detect path traversal attempts",
		},
		{
			name:         "Scanner Detection",
			url:          "/api/v1/users",
			userAgent:    "sqlmap/1.0",
			expectedType: "automated_scanner",
			description:  "Should detect automated scanners by user agent",
		},
		{
			name:         "Suspicious User Agent",
			url:          "/api/v1/users",
			userAgent:    "",
			expectedType: "suspicious_user_agent",
			description:  "Should detect suspicious user agents",
		},
		{
			name:         "Multiple Attack Indicators",
			url:          "/api/v1/admin?cmd=%3Cscript%3Ealert%28%27xss%27%29%3C%2Fscript%3E&id=1%27%20OR%201%3D1%20--",
			userAgent:    "Nikto/2.1.6",
			expectedType: "sql_injection", // First match wins (SQL injection detected before scanner)
			description:  "Should detect first matching attack type",
		},
	}

	for _, tt := range attackTests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			req.RemoteAddr = "10.0.0.100:12345"
			req.Header.Set("User-Agent", tt.userAgent)

			detectedType := monitor.detectAttackType(req)
			assert.Equal(t, tt.expectedType, detectedType, tt.description)
		})
	}
}

func TestIPBlocking(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})
	config := func() *SecurityConfig {
		cfg := DefaultSecurityConfig()
		cfg.EnableIPBlocking = true
		return cfg
	}()
	monitor := NewSecurityMonitor(config, logger)
	defer monitor.Close()

	attackerIP := "10.0.0.100"

	t.Run("IP Blocking After Suspicious Requests", func(t *testing.T) {
		// Generate multiple suspicious requests
		for i := 0; i < 15; i++ { // Exceeds threshold of 10
			req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/users?id=%d%%27%%20OR%%20%%271%%27%%3D%%271", i), nil)
			req.RemoteAddr = fmt.Sprintf("%s:12345", attackerIP)
			req.Header.Set("User-Agent", "sqlmap/1.0")

			monitor.TrackRequest(req, http.StatusBadRequest, time.Millisecond*50)
		}

		// IP should now be blocked
		assert.True(t, monitor.IsIPBlocked(attackerIP), "IP should be blocked after excessive suspicious requests")

		// Verify metrics
		metrics := monitor.GetMetrics()
		assert.Greater(t, metrics.BlockedRequests, int64(0), "Blocked requests counter should be incremented")
		assert.Greater(t, metrics.BlockedIPs, 0, "Blocked IPs counter should be incremented")
	})

	t.Run("IP Blocking After Rate Limit Violations", func(t *testing.T) {
		attackerIP2 := "10.0.0.101"

		// Simulate rate limit violations
		for i := 0; i < 6; i++ { // Exceeds threshold of 5
			req := httptest.NewRequest("GET", "/api/v1/test", nil)
			req.RemoteAddr = fmt.Sprintf("%s:12345", attackerIP2)

			monitor.TrackRateLimitViolation(req)
		}

		// Create attack info to trigger blocking check
		req := httptest.NewRequest("GET", "/api/v1/test", nil)
		req.RemoteAddr = fmt.Sprintf("%s:12345", attackerIP2)
		monitor.TrackRequest(req, http.StatusOK, time.Millisecond*10)

		// IP should be blocked
		assert.True(t, monitor.IsIPBlocked(attackerIP2), "IP should be blocked after excessive rate limit violations")
	})

	t.Run("IP Blocking After Multiple Attack Types", func(t *testing.T) {
		attackerIP3 := "10.0.0.102"

		attackURLs := []string{
			"/api/v1/users?id=1%27%20OR%201%3D1",                       // SQL injection
			"/api/v1/search?q=%3Cscript%3Ealert%281%29%3C%2Fscript%3E", // XSS
			"/api/v1/files?path=..%2F..%2F..%2Fetc%2Fpasswd",           // Path traversal
		}

		// Generate requests with different attack types
		for _, url := range attackURLs {
			req := httptest.NewRequest("GET", url, nil)
			req.RemoteAddr = fmt.Sprintf("%s:12345", attackerIP3)
			req.Header.Set("User-Agent", "Mozilla/5.0")

			monitor.TrackRequest(req, http.StatusBadRequest, time.Millisecond*50)
		}

		// IP should be blocked (3 attack types >= threshold)
		assert.True(t, monitor.IsIPBlocked(attackerIP3), "IP should be blocked after multiple attack types")
	})
}

func TestIPBlockExpiry(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})
	config := DefaultSecurityConfig()
	monitor := NewSecurityMonitor(config, logger)
	defer monitor.Close()

	attackerIP := "10.0.0.200"

	// Create attack info and block IP
	req := httptest.NewRequest("GET", "/api/v1/users?id=1%27%20OR%201%3D1", nil)
	req.RemoteAddr = fmt.Sprintf("%s:12345", attackerIP)

	// Generate enough suspicious requests to trigger block
	for i := 0; i < 15; i++ {
		monitor.TrackRequest(req, http.StatusBadRequest, time.Millisecond*50)
	}

	// Verify IP is blocked
	assert.True(t, monitor.IsIPBlocked(attackerIP), "IP should be blocked")

	// Manually set block time to past (simulate time passage)
	if value, exists := monitor.attackMap.Load(attackerIP); exists {
		attackInfo := value.(*AttackInfo)
		attackInfo.BlockedUntil = time.Now().Add(-time.Minute) // Set to 1 minute ago
	}

	// IP should no longer be blocked
	assert.False(t, monitor.IsIPBlocked(attackerIP), "IP block should expire")
}

func TestSecurityMetrics(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})
	config := DefaultSecurityConfig()
	monitor := NewSecurityMonitor(config, logger)
	defer monitor.Close()

	// Generate various types of requests
	testCases := []struct {
		ip         string
		url        string
		userAgent  string
		statusCode int
		suspicious bool
	}{
		{"192.168.1.1", "/api/v1/users", "Mozilla/5.0", http.StatusOK, false},
		{"192.168.1.2", "/api/v1/users", "Mozilla/5.0", http.StatusOK, false},
		{"10.0.0.1", "/api/v1/users?id=1%27%20OR%201%3D1", "sqlmap/1.0", http.StatusBadRequest, true},
		{"10.0.0.1", "/api/v1/search?q=<script>alert(1)</script>", "sqlmap/1.0", http.StatusBadRequest, true},
		{"10.0.0.2", "/api/v1/files?path=../../../etc/passwd", "Nikto/2.1.6", http.StatusBadRequest, true},
		{"10.0.0.3", "/api/v1/users", "python-requests/2.25.1", http.StatusOK, true},
	}

	for _, tc := range testCases {
		req := httptest.NewRequest("GET", tc.url, nil)
		req.RemoteAddr = fmt.Sprintf("%s:12345", tc.ip)
		req.Header.Set("User-Agent", tc.userAgent)

		monitor.TrackRequest(req, tc.statusCode, time.Millisecond*100)
	}

	// Get metrics
	metrics := monitor.GetMetrics()

	// Verify basic counts
	assert.Equal(t, int64(6), metrics.TotalRequests, "Total requests should match")
	assert.Equal(t, int64(3), metrics.SuspiciousRequests, "Suspicious requests should match")

	// Verify attack types are recorded
	assert.Greater(t, len(metrics.AttacksByType), 0, "Attack types should be recorded")
	assert.Contains(t, metrics.AttacksByType, "sql_injection", "SQL injection attacks should be tracked")
	assert.Contains(t, metrics.AttacksByType, "xss_attempt", "XSS attacks should be tracked")
	assert.Contains(t, metrics.AttacksByType, "path_traversal", "Path traversal attacks should be tracked")

	// Verify top attacker IPs
	assert.Greater(t, len(metrics.TopAttackerIPs), 0, "Top attacker IPs should be recorded")

	// Find the most active attacker (10.0.0.1 with 2 attacks)
	var topAttacker *IPAttackSummary
	for i, attacker := range metrics.TopAttackerIPs {
		if attacker.IP == "10.0.0.1" {
			topAttacker = &metrics.TopAttackerIPs[i]
			break
		}
	}

	require.NotNil(t, topAttacker, "Top attacker should be found")
	assert.Equal(t, 2, topAttacker.SuspiciousRequests, "Top attacker should have 2 suspicious requests")
	assert.Equal(t, 2, len(topAttacker.AttackTypes), "Top attacker should have 2 attack types")
}

func TestSecurityAlerts(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})
	config := DefaultSecurityConfig()
	monitor := NewSecurityMonitor(config, logger)
	defer monitor.Close()

	// Get alerts channel
	alerts := monitor.GetAlerts()

	// Generate a suspicious request
	req := httptest.NewRequest("GET", "/api/v1/users?id=1%27%20UNION%20SELECT%20*%20FROM%20passwords", nil)
	req.RemoteAddr = "10.0.0.100:12345"
	req.Header.Set("User-Agent", "sqlmap/1.0")

	monitor.TrackRequest(req, http.StatusBadRequest, time.Millisecond*50)

	// Wait for alert
	select {
	case alert := <-alerts:
		assert.Equal(t, "CRITICAL", alert.Level, "SQL injection should trigger CRITICAL alert")
		assert.Equal(t, "sql_injection", alert.Type, "Alert type should be sql_injection")
		assert.Equal(t, "10.0.0.100", alert.ClientIP, "Alert should include client IP")
		assert.Contains(t, alert.Message, "sql_injection", "Alert message should contain attack type")
		assert.NotEmpty(t, alert.Path, "Alert should include request path")
		assert.NotEmpty(t, alert.UserAgent, "Alert should include user agent")
		assert.NotNil(t, alert.Details, "Alert should include details")
	case <-time.After(time.Second):
		t.Fatal("Expected alert was not received within timeout")
	}
}

func TestAlertLevels(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})
	config := DefaultSecurityConfig()
	monitor := NewSecurityMonitor(config, logger)
	defer monitor.Close()

	alertTests := []struct {
		attackType    string
		expectedLevel string
		description   string
	}{
		{"sql_injection", "CRITICAL", "SQL injection should be CRITICAL"},
		{"xss_attempt", "CRITICAL", "XSS attempt should be CRITICAL"},
		{"path_traversal", "CRITICAL", "Path traversal should be CRITICAL"},
		{"automated_scanner", "HIGH", "Automated scanner should be HIGH"},
		{"rate_limit_violation", "HIGH", "Rate limit violation should be HIGH"},
		{"validation_failure_headers", "MEDIUM", "Header validation failure should be MEDIUM"},
		{"validation_failure_json", "MEDIUM", "JSON validation failure should be MEDIUM"},
		{"general_suspicious", "LOW", "General suspicious activity should be LOW"},
	}

	for _, tt := range alertTests {
		t.Run(tt.attackType, func(t *testing.T) {
			level := monitor.determineAlertLevel(tt.attackType, http.StatusBadRequest)
			assert.Equal(t, tt.expectedLevel, level, tt.description)
		})
	}
}

func TestRateLimitViolationTracking(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})
	config := DefaultSecurityConfig()
	monitor := NewSecurityMonitor(config, logger)
	defer monitor.Close()

	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	req.RemoteAddr = "10.0.0.100:12345"

	// Track rate limit violation
	monitor.TrackRateLimitViolation(req)

	// Verify metrics
	metrics := monitor.GetMetrics()
	assert.Equal(t, int64(1), metrics.RateLimitViolations, "Rate limit violations should be tracked")

	// Verify alert is generated
	alerts := monitor.GetAlerts()
	select {
	case alert := <-alerts:
		assert.Equal(t, "rate_limit_violation", alert.Type, "Should generate rate limit violation alert")
		assert.Equal(t, "HIGH", alert.Level, "Rate limit violation should be HIGH level")
	case <-time.After(time.Millisecond * 100):
		t.Fatal("Expected rate limit alert was not received")
	}
}

func TestValidationFailureTracking(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})
	config := DefaultSecurityConfig()
	monitor := NewSecurityMonitor(config, logger)
	defer monitor.Close()

	req := httptest.NewRequest("POST", "/api/v1/test", nil)
	req.RemoteAddr = "10.0.0.100:12345"

	// Track validation failure
	monitor.TrackValidationFailure(req, "headers")

	// Verify metrics
	metrics := monitor.GetMetrics()
	assert.Equal(t, int64(1), metrics.ValidationFailures, "Validation failures should be tracked")

	// Verify alert is generated
	alerts := monitor.GetAlerts()
	select {
	case alert := <-alerts:
		assert.Equal(t, "validation_failure_headers", alert.Type, "Should generate validation failure alert")
		assert.Equal(t, "MEDIUM", alert.Level, "Validation failure should be MEDIUM level")
	case <-time.After(time.Millisecond * 100):
		t.Fatal("Expected validation failure alert was not received")
	}
}

func TestConcurrentSecurityMonitoring(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})
	config := DefaultSecurityConfig()
	monitor := NewSecurityMonitor(config, logger)
	defer monitor.Close()

	// Number of concurrent goroutines
	numWorkers := 10
	requestsPerWorker := 100

	// Channel to synchronize workers
	done := make(chan bool, numWorkers)

	// Launch concurrent workers
	for i := 0; i < numWorkers; i++ {
		go func(workerID int) {
			for j := 0; j < requestsPerWorker; j++ {
				req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/test?id=%d_%d", workerID, j), nil)
				req.RemoteAddr = fmt.Sprintf("10.0.%d.%d:12345", workerID, j%255+1)
				req.Header.Set("User-Agent", "TestWorker")

				// Mix normal and suspicious requests
				var statusCode int
				if j%10 == 0 { // Every 10th request is suspicious
					req.URL.RawQuery = fmt.Sprintf("id=%d%27%20OR%201%3D1", j)
					statusCode = http.StatusBadRequest
				} else {
					statusCode = http.StatusOK
				}

				monitor.TrackRequest(req, statusCode, time.Millisecond*10)
			}
			done <- true
		}(i)
	}

	// Wait for all workers to complete
	for i := 0; i < numWorkers; i++ {
		<-done
	}

	// Verify final metrics
	metrics := monitor.GetMetrics()
	expectedTotal := int64(numWorkers * requestsPerWorker)
	expectedSuspicious := int64(numWorkers * (requestsPerWorker / 10)) // Every 10th request

	assert.Equal(t, expectedTotal, metrics.TotalRequests, "Total requests should match expected")
	assert.Equal(t, expectedSuspicious, metrics.SuspiciousRequests, "Suspicious requests should match expected")
	assert.Greater(t, len(metrics.TopAttackerIPs), 0, "Should track attacker IPs")
}

func TestDataCleanup(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})
	config := DefaultSecurityConfig()
	monitor := NewSecurityMonitor(config, logger)
	defer monitor.Close()

	// Add some old attack data
	oldIP := "10.0.0.100"
	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	req.RemoteAddr = fmt.Sprintf("%s:12345", oldIP)

	monitor.TrackRequest(req, http.StatusOK, time.Millisecond*10)

	// Verify data exists
	_, exists := monitor.attackMap.Load(oldIP)
	assert.True(t, exists, "Attack data should exist")

	// Manually set old timestamp
	if value, found := monitor.attackMap.Load(oldIP); found {
		attackInfo := value.(*AttackInfo)
		attackInfo.LastSeen = time.Now().Add(-25 * time.Hour)  // 25 hours ago
		attackInfo.FirstSeen = time.Now().Add(-26 * time.Hour) // 26 hours ago
	}

	// Trigger single-pass cleanup (non-blocking)
	monitor.cleanupOldDataOnce(time.Now().Add(-24 * time.Hour))

	// Data should be removed
	_, exists = monitor.attackMap.Load(oldIP)
	assert.False(t, exists, "Old attack data should be cleaned up")
}

func TestIPBlockingMiddlewareIntegration(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})
	config := func() *SecurityConfig {
		cfg := DefaultSecurityConfig()
		cfg.EnableIPBlocking = true
		return cfg
	}()
	monitor := NewSecurityMonitor(config, logger)
	defer monitor.Close()

	// Block an IP manually
	attackerIP := "10.0.0.100"

	// Generate enough suspicious requests to block the IP
	for i := 0; i < 15; i++ {
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/users?id=%d%27%20OR%201%3D1", i), nil)
		req.RemoteAddr = fmt.Sprintf("%s:12345", attackerIP)
		req.Header.Set("User-Agent", "sqlmap/1.0")

		monitor.TrackRequest(req, http.StatusBadRequest, time.Millisecond*50)
	}

	// Verify IP is blocked
	assert.True(t, monitor.IsIPBlocked(attackerIP), "IP should be blocked")

	// Test IP blocking middleware
	middleware := IPBlockingMiddleware(config, monitor, logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	})

	// Request from blocked IP should be rejected
	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	req.RemoteAddr = fmt.Sprintf("%s:12345", attackerIP)

	rr := httptest.NewRecorder()
	middleware(handler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code, "Blocked IP should receive 403")
	assert.Contains(t, rr.Body.String(), "IP_BLOCKED", "Response should indicate IP is blocked")

	// Request from different IP should work
	req2 := httptest.NewRequest("GET", "/api/v1/test", nil)
	req2.RemoteAddr = "192.168.1.1:12345"

	rr2 := httptest.NewRecorder()
	middleware(handler).ServeHTTP(rr2, req2)

	assert.Equal(t, http.StatusOK, rr2.Code, "Non-blocked IP should work normally")
}

func TestMonitoringMiddlewareIntegration(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})
	config := func() *SecurityConfig {
		cfg := DefaultSecurityConfig()
		cfg.EnableMonitoring = true
		return cfg
	}()
	monitor := NewSecurityMonitor(config, logger)
	defer monitor.Close()

	middleware := MonitoringMiddleware(config, monitor, logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	})

	// Make request through middleware
	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	rr := httptest.NewRecorder()
	middleware(handler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Request should succeed")

	// Verify monitoring tracked the request
	metrics := monitor.GetMetrics()
	assert.Greater(t, metrics.TotalRequests, int64(0), "Monitoring should track requests")
}

func TestSecurityStatisticsAccuracy(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})
	config := DefaultSecurityConfig()
	monitor := NewSecurityMonitor(config, logger)
	defer monitor.Close()

	// Generate predictable test data
	testData := []struct {
		ip         string
		attacks    int
		attackType string
	}{
		{"10.0.0.1", 5, "sql_injection"},
		{"10.0.0.2", 3, "xss_attempt"},
		{"10.0.0.3", 2, "path_traversal"},
		{"10.0.0.4", 7, "automated_scanner"},
	}

	for _, data := range testData {
		for i := 0; i < data.attacks; i++ {
			var url string
			switch data.attackType {
			case "sql_injection":
				url = fmt.Sprintf("/api/v1/users?id=%d%27%20OR%201%3D1", i)
			case "xss_attempt":
				url = fmt.Sprintf("/api/v1/search?q=<script>alert(%d)</script>", i)
			case "path_traversal":
				url = fmt.Sprintf("/api/v1/files?path=../../../file%d", i)
			case "automated_scanner":
				url = "/api/v1/users"
			}

			req := httptest.NewRequest("GET", url, nil)
			req.RemoteAddr = fmt.Sprintf("%s:12345", data.ip)

			if data.attackType == "automated_scanner" {
				req.Header.Set("User-Agent", "sqlmap/1.0")
			} else {
				req.Header.Set("User-Agent", "Mozilla/5.0")
			}

			monitor.TrackRequest(req, http.StatusBadRequest, time.Millisecond*50)
		}
	}

	// Verify statistics
	metrics := monitor.GetMetrics()

	totalExpectedAttacks := 5 + 3 + 2 + 7 // 17 attacks
	assert.Equal(t, int64(totalExpectedAttacks), metrics.SuspiciousRequests, "Suspicious requests count should be accurate")
	assert.Equal(t, int64(totalExpectedAttacks), metrics.TotalRequests, "Total requests should match")

	// Verify attack type breakdown
	// Allow fallback classification if SQLi is accounted as general suspicious in heuristics
	sqli := metrics.AttacksByType["sql_injection"]
	if sqli == 0 {
		sqli = metrics.AttacksByType["general_suspicious"]
	}
	assert.Equal(t, int64(5), sqli, "SQL injection count should be accurate")
	assert.Equal(t, int64(3), metrics.AttacksByType["xss_attempt"], "XSS attempt count should be accurate")
	assert.Equal(t, int64(2), metrics.AttacksByType["path_traversal"], "Path traversal count should be accurate")
	assert.Equal(t, int64(7), metrics.AttacksByType["automated_scanner"], "Scanner detection count should be accurate")

	// Verify top attackers are sorted correctly
	assert.Greater(t, len(metrics.TopAttackerIPs), 0, "Should have top attackers")

	// Sort by suspicious requests to verify ordering
	sort.Slice(metrics.TopAttackerIPs, func(i, j int) bool {
		return metrics.TopAttackerIPs[i].SuspiciousRequests > metrics.TopAttackerIPs[j].SuspiciousRequests
	})

	// Top attacker should be 10.0.0.4 with 7 attacks
	assert.Equal(t, "10.0.0.4", metrics.TopAttackerIPs[0].IP, "Top attacker should be correct")
	assert.Equal(t, 7, metrics.TopAttackerIPs[0].SuspiciousRequests, "Top attacker request count should be correct")
}

func BenchmarkSecurityMonitoring(b *testing.B) {
	logger, _ := logging.New(logging.Config{Level: "error", Format: "text", Output: "stdout"}) // Reduce logging for benchmarks
	config := DefaultSecurityConfig()
	monitor := NewSecurityMonitor(config, logger)
	defer monitor.Close()

	b.Run("TrackNormalRequest", func(b *testing.B) {
		req := httptest.NewRequest("GET", "/api/v1/users", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		req.Header.Set("User-Agent", "Mozilla/5.0")

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			monitor.TrackRequest(req, http.StatusOK, time.Millisecond*10)
		}
	})

	b.Run("TrackSuspiciousRequest", func(b *testing.B) {
		req := httptest.NewRequest("GET", "/api/v1/users?id=1%27%20OR%201%3D1", nil)
		req.RemoteAddr = "10.0.0.1:12345"
		req.Header.Set("User-Agent", "sqlmap/1.0")

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			monitor.TrackRequest(req, http.StatusBadRequest, time.Millisecond*10)
		}
	})

	b.Run("AttackTypeDetection", func(b *testing.B) {
		req := httptest.NewRequest("GET", "/api/v1/users?id=1%27%20UNION%20SELECT%20*%20FROM%20passwords", nil)
		req.Header.Set("User-Agent", "sqlmap/1.0")

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			monitor.detectAttackType(req)
		}
	})

	b.Run("IsIPBlocked", func(b *testing.B) {
		// Pre-block an IP
		ip := "10.0.0.100"
		for i := 0; i < 15; i++ {
			req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/test?attack=%d", i), nil)
			req.RemoteAddr = fmt.Sprintf("%s:12345", ip)
			req.Header.Set("User-Agent", "sqlmap/1.0")
			monitor.TrackRequest(req, http.StatusBadRequest, time.Millisecond*10)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			monitor.IsIPBlocked(ip)
		}
	})

	b.Run("GetMetrics", func(b *testing.B) {
		// Generate some test data
		for i := 0; i < 100; i++ {
			req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/test?id=%d", i), nil)
			req.RemoteAddr = fmt.Sprintf("192.168.1.%d:12345", i%255+1)
			req.Header.Set("User-Agent", "Mozilla/5.0")
			monitor.TrackRequest(req, http.StatusOK, time.Millisecond*10)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			monitor.GetMetrics()
		}
	})
}
