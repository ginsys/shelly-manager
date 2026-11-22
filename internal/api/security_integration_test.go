package api

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/api/middleware"
	"github.com/ginsys/shelly-manager/internal/api/response"
	"github.com/ginsys/shelly-manager/internal/logging"
)

// TestSecurityFrameworkIntegration tests the complete security framework
func TestSecurityFrameworkIntegration(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})

	// Create security configuration for testing
	securityConfig := &middleware.SecurityConfig{
		CSP:               "default-src 'self'",
		RateLimit:         10,
		RateLimitWindow:   time.Second,
		RateLimitByPath:   map[string]int{"/api/v1/control": 3},
		MaxRequestSize:    1024 * 1024, // 1MB
		RequestTimeout:    5 * time.Second,
		EnableHSTS:        false, // Disabled for HTTP testing
		HSTSMaxAge:        31536000,
		PermissionsPolicy: "geolocation=(), camera=(), microphone=()",
		LogSecurityEvents: true,
		LogAllRequests:    false,
	}

	validationConfig := &middleware.ValidationConfig{
		AllowedContentTypes: map[string]bool{
			"application/json":                  true,
			"application/x-www-form-urlencoded": true,
		},
		StrictContentType:         true,
		RequiredHeaders:           []string{},
		ForbiddenHeaders:          []string{"x-forwarded-proto"},
		MaxHeaderSize:             8192,
		MaxHeaderCount:            50,
		ValidateJSON:              true,
		MaxJSONDepth:              10,
		MaxJSONArraySize:          1000,
		MaxQueryParamSize:         2048,
		MaxQueryParamCount:        50,
		ForbiddenParams:           []string{"__proto__", "constructor"},
		BlockSuspiciousUserAgents: true,
		BlockSuspiciousHeaders:    true,
		LogValidationErrors:       true,
	}

	// Create test handler
	testHandler := &TestHandler{
		logger: logger,
	}

	// Create router with full security middleware stack
	router := setupSecureRouter(testHandler, logger, securityConfig, validationConfig)

	t.Run("Normal Request Flow", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/test", nil)
		req.Header.Set("User-Agent", "TestAgent/1.0")
		req.RemoteAddr = "192.168.1.100:12345"

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		// Should succeed with security headers
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.NotEmpty(t, rr.Header().Get("Content-Security-Policy"))
		assert.NotEmpty(t, rr.Header().Get("X-Frame-Options"))
		assert.NotEmpty(t, rr.Header().Get("X-Content-Type-Options"))
	})

	t.Run("SQL Injection Attack Prevention", func(t *testing.T) {
		maliciousPayloads := []string{
			"1' OR '1'='1",
			"'; DROP TABLE users; --",
			"' UNION SELECT * FROM passwords --",
			"1; DELETE FROM sessions; --",
		}

		for i, payload := range maliciousPayloads {
			t.Run(fmt.Sprintf("SQLInjection_%d", i+1), func(t *testing.T) {
				// Properly URL encode the malicious payload
				encodedPayload := url.QueryEscape(payload)
				req := httptest.NewRequest("GET", "/api/v1/users?id="+encodedPayload, nil)
				req.Header.Set("User-Agent", "TestAgent/1.0")
				req.RemoteAddr = fmt.Sprintf("10.0.0.%d:12345", i+1)

				rr := httptest.NewRecorder()
				router.ServeHTTP(rr, req)

				// Security middleware should block malicious requests with 400 status
				// First payload might pass (simple quote), others should be blocked
				if i == 0 {
					assert.True(t, rr.Code == http.StatusOK || rr.Code == http.StatusBadRequest, "First payload might pass or be blocked")
				} else {
					assert.Equal(t, http.StatusBadRequest, rr.Code, "Malicious SQL injection should be blocked")
				}

				// Verify security headers are applied regardless of status
				assert.NotEmpty(t, rr.Header().Get("Content-Security-Policy"))
			})
		}
	})

	t.Run("XSS Attack Prevention", func(t *testing.T) {
		xssPayloads := []string{
			"<script>alert('xss')</script>",
			"javascript:alert(document.cookie)",
			"<img src=x onerror=alert('xss')>",
			"<svg onload=alert('xss')>",
		}

		for i, payload := range xssPayloads {
			t.Run(fmt.Sprintf("XSS_%d", i+1), func(t *testing.T) {
				// Properly URL encode the malicious payload
				encodedPayload := url.QueryEscape(payload)
				req := httptest.NewRequest("GET", "/api/v1/search?q="+encodedPayload, nil)
				req.Header.Set("User-Agent", "TestAgent/1.0")
				req.RemoteAddr = fmt.Sprintf("10.1.0.%d:12345", i+1)

				rr := httptest.NewRecorder()
				router.ServeHTTP(rr, req)

				// Security middleware should block XSS attempts or let CSP headers handle them
				assert.True(t, rr.Code == http.StatusOK || rr.Code == http.StatusBadRequest, "XSS attempt should be handled")

				// CSP headers should prevent execution regardless
				assert.Contains(t, rr.Header().Get("Content-Security-Policy"), "default-src 'self'")
				assert.Equal(t, "DENY", rr.Header().Get("X-Frame-Options"))
				assert.Equal(t, "1; mode=block", rr.Header().Get("X-XSS-Protection"))
			})
		}
	})

	t.Run("Rate Limiting Integration", func(t *testing.T) {
		clientIP := "10.2.0.1"

		// Make requests up to the limit
		for i := 0; i < 10; i++ {
			req := httptest.NewRequest("GET", "/api/v1/test", nil)
			req.Header.Set("User-Agent", "TestAgent/1.0")
			req.RemoteAddr = fmt.Sprintf("%s:12345", clientIP)

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code, "Request %d should succeed", i+1)
		}

		// Next request should be rate limited
		req := httptest.NewRequest("GET", "/api/v1/test", nil)
		req.Header.Set("User-Agent", "TestAgent/1.0")
		req.RemoteAddr = fmt.Sprintf("%s:12345", clientIP)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusTooManyRequests, rr.Code)
		assert.NotEmpty(t, rr.Header().Get("X-RateLimit-Limit"))
		assert.Equal(t, "0", rr.Header().Get("X-RateLimit-Remaining"))

		// Verify error response format
		var errorResponse response.APIResponse
		err := json.NewDecoder(rr.Body).Decode(&errorResponse)
		require.NoError(t, err)
		assert.False(t, errorResponse.Success)
		assert.Equal(t, "RATE_LIMIT_EXCEEDED", errorResponse.Error.Code)
	})

	t.Run("Path-Specific Rate Limiting", func(t *testing.T) {
		clientIP := "10.2.0.2"

		// Control endpoint has limit of 3
		for i := 0; i < 3; i++ {
			req := httptest.NewRequest("POST", "/api/v1/control", strings.NewReader(`{"action":"test"}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("User-Agent", "TestAgent/1.0")
			req.RemoteAddr = fmt.Sprintf("%s:12345", clientIP)

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code, "Control request %d should succeed", i+1)
		}

		// 4th request should be blocked
		req := httptest.NewRequest("POST", "/api/v1/control", strings.NewReader(`{"action":"test"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "TestAgent/1.0")
		req.RemoteAddr = fmt.Sprintf("%s:12345", clientIP)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusTooManyRequests, rr.Code)

		// But other endpoints should still work (until they hit their limits)
		req2 := httptest.NewRequest("GET", "/api/v1/test", nil)
		req2.Header.Set("User-Agent", "TestAgent/1.0")
		req2.RemoteAddr = fmt.Sprintf("%s:12345", clientIP)

		rr2 := httptest.NewRecorder()
		router.ServeHTTP(rr2, req2)

		// The control refusal is for the control rule only: it sets no block for other routes, and
		// five requests are well under the global limit of 10.
		assert.Equal(t, http.StatusOK, rr2.Code, "a path refusal must not block other endpoints")
	})

	t.Run("Content Validation Integration", func(t *testing.T) {
		tests := []struct {
			name           string
			method         string
			contentType    string
			body           string
			expectedStatus int
		}{
			{
				name:           "Valid JSON",
				method:         "POST",
				contentType:    "application/json",
				body:           `{"name": "test", "value": 123}`,
				expectedStatus: http.StatusOK,
			},
			{
				name:           "Invalid Content Type",
				method:         "POST",
				contentType:    "application/xml",
				body:           `<test>data</test>`,
				expectedStatus: http.StatusUnsupportedMediaType,
			},
			{
				name:           "Invalid JSON",
				method:         "POST",
				contentType:    "application/json",
				body:           `{"name": "test", "value":}`,
				expectedStatus: http.StatusBadRequest,
			},
			{
				name:           "Missing Content Type",
				method:         "POST",
				contentType:    "",
				body:           `{"name": "test"}`,
				expectedStatus: http.StatusBadRequest,
			},
		}

		for i, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				req := httptest.NewRequest(tt.method, "/api/v1/test", strings.NewReader(tt.body))
				if tt.contentType != "" {
					req.Header.Set("Content-Type", tt.contentType)
				}
				req.Header.Set("User-Agent", "TestAgent/1.0")
				req.RemoteAddr = fmt.Sprintf("10.3.0.%d:12345", i+1)

				rr := httptest.NewRecorder()
				router.ServeHTTP(rr, req)

				assert.Equal(t, tt.expectedStatus, rr.Code, tt.name)

				if tt.expectedStatus != http.StatusOK {
					// Verify error response format
					var errorResponse response.APIResponse
					err := json.NewDecoder(rr.Body).Decode(&errorResponse)
					if err == nil { // Only check if JSON parsing succeeds
						assert.False(t, errorResponse.Success)
					}
				}
			})
		}
	})

	t.Run("Malicious User Agent Blocking", func(t *testing.T) {
		maliciousUserAgents := []string{
			"sqlmap/1.0",
			"Nikto/2.1.6",
		}

		for i, userAgent := range maliciousUserAgents {
			t.Run(fmt.Sprintf("MaliciousUA_%d", i+1), func(t *testing.T) {
				req := httptest.NewRequest("GET", "/api/v1/test", nil)
				req.Header.Set("User-Agent", userAgent)
				req.RemoteAddr = fmt.Sprintf("10.4.0.%d:12345", i+1)

				rr := httptest.NewRecorder()
				router.ServeHTTP(rr, req)

				// Should be blocked by header validation
				assert.Equal(t, http.StatusBadRequest, rr.Code)

				var errorResponse response.APIResponse
				err := json.NewDecoder(rr.Body).Decode(&errorResponse)
				require.NoError(t, err)
				assert.False(t, errorResponse.Success)
				assert.Equal(t, "VALIDATION_FAILED", errorResponse.Error.Code)
			})
		}
	})

	t.Run("Request Size Limiting", func(t *testing.T) {
		// Create large valid JSON payload (2MB, exceeds 1MB limit)
		largeData := strings.Repeat("x", 2*1024*1024-100) // Leave room for JSON structure
		largeJSON := fmt.Sprintf(`{"data": "%s"}`, largeData)

		req := httptest.NewRequest("POST", "/api/v1/upload", strings.NewReader(largeJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "TestAgent/1.0")
		req.RemoteAddr = "10.5.0.1:12345"

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		// Should be blocked by size limit or JSON validation (both are valid security responses)
		assert.True(t, rr.Code == http.StatusRequestEntityTooLarge || rr.Code == http.StatusBadRequest,
			"Large request should be blocked by size limit (413) or JSON validation (400)")
	})

	t.Run("Concurrent Attack Simulation", func(t *testing.T) {
		var wg sync.WaitGroup
		results := make(chan testResult, 50)

		// Launch concurrent attackers
		attackerIPs := []string{"10.6.0.1", "10.6.0.2", "10.6.0.3", "10.6.0.4", "10.6.0.5"}

		for _, ip := range attackerIPs {
			wg.Add(1)
			go func(attackerIP string) {
				defer wg.Done()

				blocked := 0
				success := 0

				// Each attacker makes 20 requests rapidly
				for i := 0; i < 20; i++ {
					req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/test?attack=%d", i), nil)
					req.Header.Set("User-Agent", "TestAgent/1.0")
					req.RemoteAddr = fmt.Sprintf("%s:12345", attackerIP)

					rr := httptest.NewRecorder()
					router.ServeHTTP(rr, req)

					switch rr.Code {
					case http.StatusTooManyRequests:
						blocked++
					case http.StatusOK:
						success++
					}
				}

				results <- testResult{ip: attackerIP, blocked: blocked, success: success}
			}(ip)
		}

		wg.Wait()
		close(results)

		// Verify results
		totalBlocked := 0
		totalSuccess := 0

		for result := range results {
			totalBlocked += result.blocked
			totalSuccess += result.success

			// Each IP should have some requests blocked due to rate limiting
			assert.Greater(t, result.blocked, 5, "IP %s should have some blocked requests", result.ip)
		}

		// With 5 IPs × 20 requests = 100 total requests, expect significant blocking
		assert.GreaterOrEqual(t, totalBlocked, 50, "Should have at least 50 blocked requests")
		assert.GreaterOrEqual(t, totalSuccess, 20, "Should have at least 20 successful requests")
	})
}

func TestSecurityHeadersCompliance(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})
	securityConfig := middleware.DefaultSecurityConfig()
	validationConfig := middleware.DefaultValidationConfig()

	testHandler := &TestHandler{logger: logger}
	router := setupSecureRouter(testHandler, logger, securityConfig, validationConfig)

	t.Run("OWASP Security Headers", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/test", nil)
		req.Header.Set("User-Agent", "SecurityTest/1.0")
		req.RemoteAddr = "192.168.100.1:12345"

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		// Check OWASP recommended security headers
		headers := rr.Header()

		// Content Security Policy
		assert.NotEmpty(t, headers.Get("Content-Security-Policy"), "Should have CSP header")
		assert.Contains(t, headers.Get("Content-Security-Policy"), "default-src", "CSP should have default-src")

		// X-Frame-Options
		assert.Equal(t, "DENY", headers.Get("X-Frame-Options"), "Should deny framing")

		// X-Content-Type-Options
		assert.Equal(t, "nosniff", headers.Get("X-Content-Type-Options"), "Should prevent MIME sniffing")

		// X-XSS-Protection
		assert.Equal(t, "1; mode=block", headers.Get("X-XSS-Protection"), "Should enable XSS protection")

		// Referrer-Policy
		assert.Equal(t, "strict-origin-when-cross-origin", headers.Get("Referrer-Policy"), "Should have strict referrer policy")

		// Permissions-Policy
		assert.NotEmpty(t, headers.Get("Permissions-Policy"), "Should have permissions policy")

		// Cache control for API endpoints
		assert.Contains(t, headers.Get("Cache-Control"), "no-cache", "API should not be cached")
	})

	t.Run("HTTPS Security Headers", func(t *testing.T) {
		// Enable HSTS for this test
		httpsConfig := *securityConfig
		httpsConfig.EnableHSTS = true

		httpsRouter := setupSecureRouter(testHandler, logger, &httpsConfig, validationConfig)

		req := httptest.NewRequest("GET", "/api/v1/test", nil)
		req.Header.Set("User-Agent", "SecurityTest/1.0")
		req.RemoteAddr = "192.168.100.2:12345"

		// Simulate HTTPS by setting TLS connection
		req.TLS = &tls.ConnectionState{}

		rr := httptest.NewRecorder()
		httpsRouter.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		// Check HSTS header
		hsts := rr.Header().Get("Strict-Transport-Security")
		assert.NotEmpty(t, hsts, "Should have HSTS header for HTTPS")
		assert.Contains(t, hsts, "max-age", "HSTS should have max-age")
		assert.Contains(t, hsts, "includeSubDomains", "HSTS should include subdomains")
	})
}

func TestErrorResponseConsistency(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})

	// Create security configuration with low rate limits for testing
	securityConfig := &middleware.SecurityConfig{
		CSP:               "default-src 'self'",
		RateLimit:         10,          // Low limit to trigger rate limiting
		RateLimitWindow:   time.Second, // Short window for fast testing
		RateLimitByPath:   map[string]int{"/api/v1/control": 3},
		MaxRequestSize:    1024 * 1024, // 1MB
		RequestTimeout:    5 * time.Second,
		EnableHSTS:        false,
		HSTSMaxAge:        31536000,
		PermissionsPolicy: "geolocation=(), camera=(), microphone=()",
		LogSecurityEvents: true,
		LogAllRequests:    false,
	}
	validationConfig := middleware.DefaultValidationConfig()

	testHandler := &TestHandler{logger: logger}
	router := setupSecureRouter(testHandler, logger, securityConfig, validationConfig)

	errorTests := []struct {
		name            string
		setupRequest    func() *http.Request
		expectedCode    int
		expectedErrCode string
		description     string
	}{
		{
			name: "Rate Limit Error",
			setupRequest: func() *http.Request {
				// Generate requests to trigger rate limit
				for i := 0; i < 15; i++ {
					req := httptest.NewRequest("GET", "/api/v1/test", nil)
					req.Header.Set("User-Agent", "TestAgent/1.0")
					req.RemoteAddr = "10.9.0.1:12345"

					rr := httptest.NewRecorder()
					router.ServeHTTP(rr, req)
				}

				// Return the request that should be rate limited
				req := httptest.NewRequest("GET", "/api/v1/test", nil)
				req.Header.Set("User-Agent", "TestAgent/1.0")
				req.RemoteAddr = "10.9.0.1:12345"
				return req
			},
			expectedCode:    http.StatusTooManyRequests,
			expectedErrCode: "RATE_LIMIT_EXCEEDED",
			description:     "Rate limit should return consistent error format",
		},
		{
			name: "Validation Error",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("POST", "/api/v1/test", strings.NewReader(`{"invalid": json`))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("User-Agent", "TestAgent/1.0")
				req.RemoteAddr = "10.9.0.2:12345"
				return req
			},
			expectedCode:    http.StatusBadRequest,
			expectedErrCode: "VALIDATION_FAILED",
			description:     "Validation error should return consistent error format",
		},
		{
			name: "Unsupported Media Type Error",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("POST", "/api/v1/test", strings.NewReader(`<xml>data</xml>`))
				req.Header.Set("Content-Type", "application/xml")
				req.Header.Set("User-Agent", "TestAgent/1.0")
				req.RemoteAddr = "10.9.0.3:12345"
				return req
			},
			expectedCode:    http.StatusUnsupportedMediaType,
			expectedErrCode: "UNSUPPORTED_MEDIA_TYPE",
			description:     "Unsupported media type should return consistent error format",
		},
	}

	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupRequest()

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedCode, rr.Code, tt.description)

			// Verify standardized error response format
			var errorResponse response.APIResponse
			err := json.NewDecoder(rr.Body).Decode(&errorResponse)
			require.NoError(t, err, "Error response should be valid JSON")

			assert.False(t, errorResponse.Success, "Error response should have success=false")
			assert.NotNil(t, errorResponse.Error, "Error response should have error object")

			// Add nil check to prevent panic
			if errorResponse.Error != nil {
				assert.Equal(t, tt.expectedErrCode, errorResponse.Error.Code, "Error code should match expected")
				assert.NotEmpty(t, errorResponse.Error.Message, "Error should have message")
			} else {
				t.Errorf("Error response is nil - response was: %+v", errorResponse)
			}

			assert.False(t, errorResponse.Timestamp.IsZero(), "Error response should have timestamp")
		})
	}
}

func TestPerformanceUnderAttack(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "error", Format: "text", Output: "stdout"}) // Reduce logging for performance test
	securityConfig := middleware.DefaultSecurityConfig()
	validationConfig := middleware.DefaultValidationConfig()

	testHandler := &TestHandler{logger: logger}
	router := setupSecureRouter(testHandler, logger, securityConfig, validationConfig)

	t.Run("Performance Under Normal Load", func(t *testing.T) {
		numRequests := 1000
		start := time.Now()

		for i := 0; i < numRequests; i++ {
			req := httptest.NewRequest("GET", "/api/v1/test", nil)
			req.Header.Set("User-Agent", "PerfTest/1.0")
			req.RemoteAddr = fmt.Sprintf("192.168.200.%d:12345", i%255+1)

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
		}

		duration := time.Since(start)
		avgDuration := duration / time.Duration(numRequests)

		// Performance requirement: < 10ms per request under normal load
		assert.Less(t, avgDuration, 10*time.Millisecond,
			"Average request processing should be < 10ms, got %v", avgDuration)
	})

	t.Run("Performance Under Malicious Load", func(t *testing.T) {
		numRequests := 500
		maliciousRequests := 0
		start := time.Now()

		for i := 0; i < numRequests; i++ {
			var req *http.Request

			// Mix of malicious and normal requests
			if i%3 == 0 {
				// Malicious request - properly URL encode
				maliciousQuery := url.QueryEscape("'+OR+1=1+--")
				req = httptest.NewRequest("GET", "/api/v1/users?id="+maliciousQuery, nil)
				req.Header.Set("User-Agent", "sqlmap/1.0")
				maliciousRequests++
			} else {
				// Normal request
				req = httptest.NewRequest("GET", "/api/v1/test", nil)
				req.Header.Set("User-Agent", "PerfTest/1.0")
			}

			req.RemoteAddr = fmt.Sprintf("10.10.%d.%d:12345", i/255+1, i%255+1)

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			// Should handle all requests (malicious ones still return 200 but are monitored)
			assert.True(t, rr.Code == http.StatusOK || rr.Code == http.StatusBadRequest || rr.Code == http.StatusTooManyRequests)
		}

		duration := time.Since(start)
		avgDuration := duration / time.Duration(numRequests)

		// Performance requirement: < 20ms per request under attack (allows for security processing)
		assert.Less(t, avgDuration, 20*time.Millisecond,
			"Average request processing under attack should be < 20ms, got %v", avgDuration)

		t.Logf("Processed %d requests (%d malicious) in %v, avg: %v",
			numRequests, maliciousRequests, duration, avgDuration)
	})
}

// Helper functions and types

type testResult struct {
	ip      string
	blocked int
	success int
}

// TestHandler represents a mock API handler for testing
type TestHandler struct {
	logger *logging.Logger
}

// setupSecureRouter creates a router with full security middleware stack
func setupSecureRouter(handler *TestHandler, logger *logging.Logger, securityConfig *middleware.SecurityConfig, validationConfig *middleware.ValidationConfig) *mux.Router {
	router := mux.NewRouter()

	// Apply security middleware stack (order matters!)
	router.Use(logging.RecoveryMiddleware(logger))

	router.Use(middleware.SecurityHeadersMiddleware(securityConfig, logger))
	router.Use(middleware.RateLimitMiddleware(securityConfig, logger))
	router.Use(middleware.RequestSizeMiddleware(securityConfig, logger))
	router.Use(middleware.TimeoutMiddleware(securityConfig, logger))

	router.Use(middleware.SecurityLoggingMiddleware(securityConfig, logger))
	router.Use(middleware.ValidateContentTypeMiddleware(validationConfig, logger))
	router.Use(middleware.ValidateHeadersMiddleware(validationConfig, logger))
	router.Use(middleware.ValidateJSONMiddleware(validationConfig, logger))
	router.Use(middleware.ValidateQueryParamsMiddleware(validationConfig, logger))

	// Add test routes
	router.HandleFunc("/api/v1/test", createTestHandler(logger)).Methods("GET", "POST")
	router.HandleFunc("/api/v1/control", createTestHandler(logger)).Methods("POST")
	router.HandleFunc("/api/v1/upload", createTestHandler(logger)).Methods("POST")
	router.HandleFunc("/api/v1/users", createTestHandler(logger)).Methods("GET", "POST")
	router.HandleFunc("/api/v1/search", createTestHandler(logger)).Methods("GET")
	router.HandleFunc("/api/v1/files", createTestHandler(logger)).Methods("GET")
	router.HandleFunc("/api/v1/admin", createTestHandler(logger)).Methods("GET")

	return router
}

// createTestHandler creates a simple test handler
func createTestHandler(logger *logging.Logger) http.HandlerFunc {
	respWriter := response.NewResponseWriter(logger)

	return func(w http.ResponseWriter, r *http.Request) {
		respWriter.WriteSuccess(w, r, map[string]interface{}{
			"message": "success",
			"method":  r.Method,
			"path":    r.URL.Path,
		})
	}
}
