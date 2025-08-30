package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	securityNonceKey contextKey = "security_nonce"
)

// SecurityConfig holds configuration for security middleware
type SecurityConfig struct {
	// Content Security Policy
	CSP string

	// Rate limiting
	RateLimit       int            // requests per window
	RateLimitWindow time.Duration  // time window for rate limiting
	RateLimitByPath map[string]int // path-specific rate limits

	// Request limits
	MaxRequestSize int64         // maximum request body size in bytes
	RequestTimeout time.Duration // maximum request processing time

	// Security headers
	EnableHSTS        bool   // enable Strict-Transport-Security
	HSTSMaxAge        int    // HSTS max-age in seconds
	PermissionsPolicy string // permissions policy header
	// CORS
	CORSAllowedOrigins []string // allowed origins; if empty, uses "*" (dev)
	CORSAllowedMethods []string // allowed methods
	CORSAllowedHeaders []string // allowed headers
	CORSMaxAge         int      // preflight cache seconds

	// Logging and monitoring
	LogSecurityEvents bool // enable security event logging
	LogAllRequests    bool // enable request/response logging
	EnableMonitoring  bool // enable security monitoring and alerting

	// Attack detection
	EnableIPBlocking bool          // enable automatic IP blocking
	BlockDuration    time.Duration // how long to block suspicious IPs

	// Proxy handling
	UseProxyHeaders bool     // trust proxy headers for client IP extraction
	TrustedProxies  []string // list of trusted proxy IPs or CIDR ranges
}

// DefaultSecurityConfig returns a secure default configuration
func DefaultSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		CSP:             "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self';",
		RateLimit:       1000,
		RateLimitWindow: time.Hour,
		RateLimitByPath: map[string]int{
			"/api/v1/devices/{id}/control": 100, // device control endpoints
			"/api/v1/provisioning":         50,  // provisioning endpoints
			"/api/v1/config/bulk":          20,  // bulk operations
		},
		MaxRequestSize:     10 * 1024 * 1024, // 10MB
		RequestTimeout:     30 * time.Second,
		EnableHSTS:         false,    // disabled by default, enable for HTTPS
		HSTSMaxAge:         31536000, // 1 year
		PermissionsPolicy:  "geolocation=(), camera=(), microphone=(), payment=()",
		CORSAllowedOrigins: nil,
		CORSAllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		CORSAllowedHeaders: []string{"Content-Type", "Authorization", "X-Requested-With"},
		CORSMaxAge:         86400,
		LogSecurityEvents:  true,
		LogAllRequests:     false,     // enable for debugging
		EnableMonitoring:   true,      // enable security monitoring
		EnableIPBlocking:   true,      // enable automatic IP blocking
		BlockDuration:      time.Hour, // block for 1 hour
		UseProxyHeaders:    false,
		TrustedProxies:     nil,
	}
}

// RateLimiter implements a simple in-memory rate limiter
type RateLimiter struct {
	mutex           sync.RWMutex
	clients         map[string]*clientInfo
	config          *SecurityConfig
	logger          *logging.Logger
	cleanupInterval time.Duration
}

type clientInfo struct {
	requests  int
	window    time.Time
	blocked   bool
	blockTime time.Time
}

// NewRateLimiter creates a new rate limiter instance
func NewRateLimiter(config *SecurityConfig, logger *logging.Logger) *RateLimiter {
	rl := &RateLimiter{
		clients:         make(map[string]*clientInfo),
		config:          config,
		logger:          logger,
		cleanupInterval: time.Minute * 5,
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// Allow checks if a request should be allowed based on rate limiting
func (rl *RateLimiter) Allow(clientIP, path string) bool {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	now := time.Now()
	client, exists := rl.clients[clientIP]

	if !exists {
		rl.clients[clientIP] = &clientInfo{
			requests: 1,
			window:   now,
			blocked:  false,
		}
		return true
	}

	// Check if client is currently blocked
	if client.blocked && now.Sub(client.blockTime) < time.Minute*15 {
		return false
	}

	// Reset window if expired
	if now.Sub(client.window) > rl.config.RateLimitWindow {
		client.requests = 1
		client.window = now
		client.blocked = false
		return true
	}

	// Check path-specific limits
	limit := rl.config.RateLimit
	for pathPrefix, pathLimit := range rl.config.RateLimitByPath {
		if strings.Contains(path, strings.ReplaceAll(pathPrefix, "{id}", "")) {
			if pathLimit < limit {
				limit = pathLimit
			}
			break
		}
	}

	// Increment request count
	client.requests++

	// Check if limit exceeded
	if client.requests > limit {
		client.blocked = true
		client.blockTime = now

		if rl.logger != nil && rl.config.LogSecurityEvents {
			rl.logger.WithFields(map[string]any{
				"client_ip":      clientIP,
				"path":           path,
				"requests":       client.requests,
				"limit":          limit,
				"component":      "rate_limiter",
				"security_event": "rate_limit_exceeded",
			}).Warn("Rate limit exceeded")
		}

		return false
	}

	return true
}

// cleanup removes old client entries
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		rl.mutex.Lock()
		now := time.Now()
		for ip, client := range rl.clients {
			// Remove entries older than 2 hours
			if now.Sub(client.window) > time.Hour*2 {
				delete(rl.clients, ip)
			}
		}
		rl.mutex.Unlock()
	}
}

// SecurityHeadersMiddleware adds comprehensive security headers to responses
func SecurityHeadersMiddleware(config *SecurityConfig, logger *logging.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Generate nonce for CSP if needed
			nonce := generateNonce()

			// Set security headers
			w.Header().Set("Content-Security-Policy", config.CSP)
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Permissions-Policy", config.PermissionsPolicy)

			// Add HSTS header for HTTPS requests
			if config.EnableHSTS && r.TLS != nil {
				hstsValue := fmt.Sprintf("max-age=%d; includeSubDomains", config.HSTSMaxAge)
				w.Header().Set("Strict-Transport-Security", hstsValue)
			}

			// Add cache control for security-sensitive endpoints
			if strings.Contains(r.URL.Path, "/api/v1/") {
				w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
				w.Header().Set("Pragma", "no-cache")
				w.Header().Set("Expires", "0")
			}

			// Add security context to request (typed key to avoid collisions)
			ctx := context.WithValue(r.Context(), securityNonceKey, nonce)
			r = r.WithContext(ctx)

			// Log security headers if enabled
			if logger != nil && config.LogSecurityEvents {
				logger.WithFields(map[string]any{
					"method":     r.Method,
					"path":       r.URL.Path,
					"user_agent": r.UserAgent(),
					"component":  "security_headers",
				}).Debug("Security headers applied")
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimitMiddleware implements rate limiting per client IP
func RateLimitMiddleware(config *SecurityConfig, logger *logging.Logger) func(http.Handler) http.Handler {
	rateLimiter := NewRateLimiter(config, logger)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get client IP
			clientIP := getClientIP(r)

			// Check rate limit
			if !rateLimiter.Allow(clientIP, r.URL.Path) {
				if logger != nil && config.LogSecurityEvents {
					logger.WithFields(map[string]any{
						"client_ip":      clientIP,
						"path":           r.URL.Path,
						"method":         r.Method,
						"user_agent":     r.UserAgent(),
						"component":      "rate_limiter",
						"security_event": "request_blocked",
					}).Warn("Request blocked by rate limiter")
				}

				// Return rate limit exceeded response
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", config.RateLimit))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(config.RateLimitWindow).Unix()))
				w.WriteHeader(http.StatusTooManyRequests)

				// Write standardized error response with timestamp
				response := map[string]interface{}{
					"success": false,
					"error": map[string]interface{}{
						"code":    "RATE_LIMIT_EXCEEDED",
						"message": "Too many requests. Please try again later.",
					},
					"meta": map[string]interface{}{
						"retry_after": config.RateLimitWindow.Seconds(),
					},
					"timestamp": time.Now().UTC(),
				}

				if err := writeJSONResponse(w, response); err != nil && logger != nil {
					logger.Error("Failed to write rate limit response", "error", err)
				}
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequestSizeMiddleware limits request body size
func RequestSizeMiddleware(config *SecurityConfig, logger *logging.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Limit request body size
			if config.MaxRequestSize > 0 {
				// If Content-Length is known and exceeds the limit, reject immediately
				if r.ContentLength > 0 && r.ContentLength > config.MaxRequestSize {
					w.WriteHeader(http.StatusRequestEntityTooLarge)
					return
				}
				// Otherwise wrap the body to enforce a hard cap during reads
				r.Body = http.MaxBytesReader(w, r.Body, config.MaxRequestSize)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SecurityLoggingMiddleware provides comprehensive request/response logging for security monitoring
func SecurityLoggingMiddleware(config *SecurityConfig, logger *logging.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if logger == nil {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			clientIP := getClientIP(r)

			// Log request details for security monitoring
			if config.LogAllRequests || config.LogSecurityEvents {
				logger.WithFields(map[string]any{
					"method":       r.Method,
					"path":         r.URL.Path,
					"client_ip":    clientIP,
					"user_agent":   r.UserAgent(),
					"content_type": r.Header.Get("Content-Type"),
					"referer":      r.Header.Get("Referer"),
					"component":    "security_monitor",
					"event_type":   "request_received",
				}).Info("API request received")
			}

			// Check for suspicious patterns
			if detectSuspiciousRequest(r) {
				logger.WithFields(map[string]any{
					"method":         r.Method,
					"path":           r.URL.Path,
					"client_ip":      clientIP,
					"user_agent":     r.UserAgent(),
					"component":      "security_monitor",
					"security_event": "suspicious_request",
					"query_params":   r.URL.RawQuery,
				}).Warn("Suspicious request detected")
			}

			// Create response wrapper for status code capture
			wrapped := &securityResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			next.ServeHTTP(wrapped, r)

			// Log response details
			duration := time.Since(start).Milliseconds()
			if config.LogAllRequests || (config.LogSecurityEvents && (wrapped.statusCode >= 400 || wrapped.statusCode == 401 || wrapped.statusCode == 403)) {
				logger.WithFields(map[string]any{
					"method":        r.Method,
					"path":          r.URL.Path,
					"client_ip":     clientIP,
					"status_code":   wrapped.statusCode,
					"duration_ms":   duration,
					"response_size": wrapped.size,
					"component":     "security_monitor",
					"event_type":    "request_completed",
				}).Info("API request completed")
			}
		})
	}
}

// TimeoutMiddleware implements request timeout protection
func TimeoutMiddleware(config *SecurityConfig, logger *logging.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if config.RequestTimeout <= 0 {
				next.ServeHTTP(w, r)
				return
			}

			ctx, cancel := context.WithTimeout(r.Context(), config.RequestTimeout)
			defer cancel()

			r = r.WithContext(ctx)

			done := make(chan struct{})
			go func() {
				next.ServeHTTP(w, r)
				close(done)
			}()

			select {
			case <-done:
				// Request completed normally
			case <-ctx.Done():
				// Request timed out
				if logger != nil && config.LogSecurityEvents {
					logger.WithFields(map[string]any{
						"method":         r.Method,
						"path":           r.URL.Path,
						"client_ip":      getClientIP(r),
						"timeout":        config.RequestTimeout,
						"component":      "security_timeout",
						"security_event": "request_timeout",
					}).Warn("Request timed out")
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusRequestTimeout)
				response := map[string]interface{}{
					"success": false,
					"error": map[string]interface{}{
						"code":    "REQUEST_TIMEOUT",
						"message": "Request processing timeout",
					},
					"timestamp": time.Now().UTC(),
				}
				if err := writeJSONResponse(w, response); err != nil {
					// Log the error but don't panic - this is a timeout response
					// We can't change the response at this point
					return
				}
			}
		})
	}
}

// Helper types and functions

type securityResponseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
}

func (w *securityResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *securityResponseWriter) Write(data []byte) (int, error) {
	n, err := w.ResponseWriter.Write(data)
	w.size += n
	return n, err
}

// getClientIP extracts the real client IP from request
func getClientIP(r *http.Request) string {
	// If proxy headers are trusted, try to extract the client IP considering trusted proxies
	// We infer the config from context if present; otherwise, use conservative behavior
	var trusted []string
	useProxy := false
	if cfgVal := r.Context().Value(contextKey("security_config")); cfgVal != nil {
		if cfg, ok := cfgVal.(*SecurityConfig); ok {
			trusted = cfg.TrustedProxies
			useProxy = cfg.UseProxyHeaders
		}
	}

	if useProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			// Iterate from rightmost to leftmost; pick first not in trusted set
			for i := len(parts) - 1; i >= 0; i-- {
				ip := strings.TrimSpace(parts[i])
				if ip == "" {
					continue
				}
				if !isTrustedProxyIP(ip, trusted) {
					return ip
				}
			}
		}
		if xri := r.Header.Get("X-Real-IP"); xri != "" && !isTrustedProxyIP(strings.TrimSpace(xri), trusted) {
			return strings.TrimSpace(xri)
		}
	} else {
		// Legacy behavior: take first XFF IP if present, else X-Real-IP
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if idx := strings.Index(xff, ","); idx != -1 {
				return strings.TrimSpace(xff[:idx])
			}
			return strings.TrimSpace(xff)
		}
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			return strings.TrimSpace(xri)
		}
	}

	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// isTrustedProxyIP checks if an IP string is within the trusted proxies list (IPs or CIDRs)
func isTrustedProxyIP(ipStr string, trusted []string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	for _, entry := range trusted {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if _, ipnet, err := net.ParseCIDR(entry); err == nil {
			if ipnet.Contains(ip) {
				return true
			}
			continue
		}
		// Exact IP match
		if ip.Equal(net.ParseIP(entry)) {
			return true
		}
	}
	return false
}

// generateNonce creates a cryptographically secure nonce for CSP
func generateNonce() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based nonce
		return fmt.Sprintf("nonce-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

// detectSuspiciousRequest checks for common attack patterns
func detectSuspiciousRequest(r *http.Request) bool {
	// Check for common injection patterns in URL
	path := strings.ToLower(r.URL.Path)
	query := strings.ToLower(r.URL.RawQuery)
	// Decode query for better pattern matching (handles %xx encodings)
	decodedQuery := ""
	if r.URL.RawQuery != "" {
		if dq, err := url.QueryUnescape(r.URL.RawQuery); err == nil {
			decodedQuery = strings.ToLower(dq)
		}
	}
	// Decode path as well in case encoded traversal is present
	decodedPath := path
	if dp, err := url.PathUnescape(r.URL.Path); err == nil {
		decodedPath = strings.ToLower(dp)
	}

	// SQL injection patterns
	sqlPatterns := []string{"' or ", " or 1=1", "union select", "drop table", "delete from", "insert into"}
	for _, pattern := range sqlPatterns {
		if strings.Contains(path, pattern) || strings.Contains(query, pattern) || strings.Contains(decodedQuery, pattern) || strings.Contains(decodedPath, pattern) {
			return true
		}
	}

	// XSS patterns
	xssPatterns := []string{"<script>", "javascript:", "onerror=", "onload=", "eval(", "alert("}
	for _, pattern := range xssPatterns {
		if strings.Contains(path, pattern) || strings.Contains(query, pattern) || strings.Contains(decodedQuery, pattern) || strings.Contains(decodedPath, pattern) {
			return true
		}
	}

	// Path traversal patterns
	if strings.Contains(path, "..") || strings.Contains(query, "..") || strings.Contains(decodedQuery, "..") || strings.Contains(decodedPath, "..") {
		return true
	}

	// Unusually long parameters (potential buffer overflow attempts)
	if len(query) > 2000 {
		return true
	}

	return false
}

// IPBlockingMiddleware blocks requests from suspicious IPs
func IPBlockingMiddleware(config *SecurityConfig, monitor *SecurityMonitor, logger *logging.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientIP := getClientIP(r)

			// Check if IP is blocked
			if config.EnableIPBlocking && monitor != nil && monitor.IsIPBlocked(clientIP) {
				if logger != nil && config.LogSecurityEvents {
					logger.WithFields(map[string]any{
						"client_ip":      clientIP,
						"path":           r.URL.Path,
						"method":         r.Method,
						"user_agent":     r.UserAgent(),
						"component":      "ip_blocking",
						"security_event": "blocked_ip_request",
					}).Warn("Request blocked from blocked IP")
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)

				response := map[string]interface{}{
					"success": false,
					"error": map[string]interface{}{
						"code":    "IP_BLOCKED",
						"message": "Your IP has been temporarily blocked due to suspicious activity.",
					},
					"timestamp": time.Now().UTC(),
				}

				if err := writeJSONResponse(w, response); err != nil && logger != nil {
					logger.Error("Failed to write IP blocked response", "error", err)
				}
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// MonitoringMiddleware integrates with security monitor for comprehensive tracking
func MonitoringMiddleware(config *SecurityConfig, monitor *SecurityMonitor, logger *logging.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !config.EnableMonitoring || monitor == nil {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()

			// Create response wrapper for status code capture
			wrapped := &securityResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			next.ServeHTTP(wrapped, r)

			// Track request in security monitor
			duration := time.Since(start)
			monitor.TrackRequest(r, wrapped.statusCode, duration)
		})
	}
}

// writeJSONResponse is a helper function to write JSON responses
func writeJSONResponse(w http.ResponseWriter, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(data)
}
