package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
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
	TrustedIPs       []string      // IPs exempt from suspicious activity tracking and blocking

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
		TrustedIPs:         []string{"127.0.0.1", "::1", "localhost"},
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
	requests  int // every request of the client in the window: the global limit counts these
	window    time.Time
	blocked   bool
	blockTime time.Time
	// paths holds one counter per path rule the client has used, keyed by the matched prefix of the
	// path (see matchRule). A path limit counts only these, never the client's other traffic.
	paths map[string]*pathCount
}

type pathCount struct {
	requests int
	window   time.Time
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

// rateRefusal says which limit refused a request and when to come back: the limit that refused, the
// end of that limit's window for the client (a whole second, rounded up, so a client that returns at
// that second is not refused again) and the wait until then in whole seconds.
type rateRefusal struct {
	limit      int
	reset      time.Time
	retryAfter time.Duration
}

// Allow checks if a request should be allowed based on rate limiting
func (rl *RateLimiter) Allow(clientIP, path string) bool {
	return rl.check(clientIP, path) == nil
}

// check applies the rate limits to a request: nil when it is allowed, else the refusal. A refusal by the
// global limit reports the global limit and a reset one window from now, as the middleware always did.
func (rl *RateLimiter) check(clientIP, path string) *rateRefusal {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	now := time.Now()
	client, exists := rl.clients[clientIP]

	if !exists {
		client = &clientInfo{window: now}
		rl.clients[clientIP] = client
	}

	// Check if client is currently blocked
	if client.blocked && now.Sub(client.blockTime) < time.Minute*15 {
		return rl.globalRefusal(now)
	}

	// Reset window if expired
	if now.Sub(client.window) > rl.config.RateLimitWindow {
		client.requests = 0
		client.window = now
		client.blocked = false
	}

	// The global limit counts every request of the client and blocks it on every route.
	client.requests++
	if client.requests > rl.config.RateLimit {
		client.blocked = true
		client.blockTime = now

		if rl.logger != nil && rl.config.LogSecurityEvents {
			rl.logger.WithFields(map[string]any{
				"client_ip":      clientIP,
				"path":           path,
				"requests":       client.requests,
				"limit":          rl.config.RateLimit,
				"component":      "rate_limiter",
				"security_event": "rate_limit_exceeded",
			}).Warn("Rate limit exceeded")
		}

		return rl.globalRefusal(now)
	}

	return rl.allowPath(client, clientIP, path, now)
}

// globalRefusal is the refusal by the global limit.
func (rl *RateLimiter) globalRefusal(now time.Time) *rateRefusal {
	return &rateRefusal{
		limit:      rl.config.RateLimit,
		reset:      now.Add(rl.config.RateLimitWindow),
		retryAfter: rl.config.RateLimitWindow,
	}
}

// allowPath applies the path rule that matches path, if any: nil when the request is allowed. The rule
// has its own counter and window per client. Going over it refuses this rule's paths until its window
// resets, and blocks nothing else. The request has already been counted by the global limit.
func (rl *RateLimiter) allowPath(client *clientInfo, clientIP, path string, now time.Time) *rateRefusal {
	limit, key, ok := rl.matchRule(path)
	if !ok {
		return nil
	}

	if client.paths == nil {
		client.paths = make(map[string]*pathCount)
	}
	counter := client.paths[key]
	if counter == nil || now.Sub(counter.window) > rl.config.RateLimitWindow {
		counter = &pathCount{window: now}
		client.paths[key] = counter
	}

	counter.requests++
	if counter.requests <= limit {
		return nil
	}

	if rl.logger != nil && rl.config.LogSecurityEvents {
		rl.logger.WithFields(map[string]any{
			"client_ip":      clientIP,
			"path":           path,
			"rule":           key,
			"requests":       counter.requests,
			"limit":          limit,
			"component":      "rate_limiter",
			"security_event": "rate_limit_exceeded",
		}).Warn("Path rate limit exceeded")
	}

	// The counter resets once more than a window has passed since it began, so its window ends at
	// counter.window+RateLimitWindow. Round that instant up to the second: the Reset header is whole
	// seconds, and a client that returns at the second named must not be refused again.
	reset := counter.window.Add(rl.config.RateLimitWindow)
	if reset.Nanosecond() != 0 {
		reset = reset.Truncate(time.Second).Add(time.Second)
	}
	wait := reset.Sub(now)
	if wait < 0 {
		wait = 0
	}
	if rest := wait % time.Second; rest != 0 {
		wait += time.Second - rest
	}
	return &rateRefusal{limit: limit, reset: reset, retryAfter: wait}
}

// matchRule finds the RateLimitByPath rule for path. A rule matches when the path starts with its
// key, where {id} stands for one non-empty path segment. The longest matching key wins (ties go to
// the smaller key), so the result never depends on map order. counterKey is the matched part of the
// path with the id filled in, for example /api/v1/devices/7/control: a rule with {id} counts per
// device.
func (rl *RateLimiter) matchRule(path string) (limit int, counterKey string, ok bool) {
	bestKey := ""
	for key, keyLimit := range rl.config.RateLimitByPath {
		matched, found := matchPathKey(key, path)
		if !found {
			continue
		}
		if !ok || len(key) > len(bestKey) || (len(key) == len(bestKey) && key < bestKey) {
			bestKey, limit, counterKey, ok = key, keyLimit, matched, true
		}
	}
	return limit, counterKey, ok
}

// matchPathKey reports whether path starts with key, with {id} matching one non-empty segment, and
// returns the matched part of path. A numeric id loses its leading zeros, so 7 and 07 give one
// counter: the routes parse the id as a decimal number.
func matchPathKey(key, path string) (string, bool) {
	var matched strings.Builder
	pos := 0
	for i, part := range strings.Split(key, "{id}") {
		if i > 0 {
			segment := strings.IndexByte(path[pos:], '/')
			if segment < 0 {
				segment = len(path) - pos
			}
			if segment == 0 {
				return "", false
			}
			matched.WriteString(canonicalID(path[pos : pos+segment]))
			pos += segment
		}
		if !strings.HasPrefix(path[pos:], part) {
			return "", false
		}
		matched.WriteString(part)
		pos += len(part)
	}
	return matched.String(), true
}

// canonicalID drops the leading zeros of an all-digit id and leaves any other id as it is.
func canonicalID(id string) string {
	for _, c := range id {
		if c < '0' || c > '9' {
			return id
		}
	}
	if trimmed := strings.TrimLeft(id, "0"); trimmed != "" {
		return trimmed
	}
	return "0"
}

// cleanup removes old client entries
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		rl.mutex.Lock()
		rl.prune(time.Now())
		rl.mutex.Unlock()
	}
}

// prune drops the clients whose window is older than 2 hours and, for the others, the path counters
// whose window has passed. An active client keeps its entry for ever, so without this a client that
// asks for a new device id every time would grow its path counters without bound. The caller holds
// the mutex.
func (rl *RateLimiter) prune(now time.Time) {
	for ip, client := range rl.clients {
		// Remove entries older than 2 hours
		if now.Sub(client.window) > time.Hour*2 {
			delete(rl.clients, ip)
			continue
		}
		for key, counter := range client.paths {
			if now.Sub(counter.window) > rl.config.RateLimitWindow {
				delete(client.paths, key)
			}
		}
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
			if refusal := rateLimiter.check(clientIP, r.URL.Path); refusal != nil {
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
				// The limit that refused the request, not always the global one.
				w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", refusal.limit))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", refusal.reset.Unix()))

				// Write standardized error response with timestamp
				response := map[string]interface{}{
					"success": false,
					"error": map[string]interface{}{
						"code":    "RATE_LIMIT_EXCEEDED",
						"message": "Too many requests. Please try again later.",
					},
					"meta": map[string]interface{}{
						"retry_after": refusal.retryAfter.Seconds(),
					},
					"timestamp": time.Now().UTC(),
				}

				body, err := json.Marshal(response)
				if err != nil {
					http.Error(w, "Internal server error", http.StatusInternalServerError)
					return
				}
				body = append(body, '\n')
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Content-Length", strconv.Itoa(len(body)))
				w.WriteHeader(http.StatusTooManyRequests)
				if _, writeErr := w.Write(body); writeErr != nil && logger != nil {
					logger.Error("Failed to write rate limit response", "error", writeErr)
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

			// The handler runs in its own goroutine against a guarded writer, so
			// that after a timeout exactly one party writes the response.
			tw := newTimeoutWriter(w)
			done := make(chan struct{})
			go func() {
				// One deferred func, in this order: recover, answer, then close
				// done. Closing done first would let this request return and
				// net/http send an empty 200 before the 500 is written.
				defer func() {
					if rec := recover(); rec != nil {
						answered := tw.fail()
						if logger != nil {
							logger.WithFields(map[string]any{
								"method":    r.Method,
								"path":      r.URL.Path,
								"remote":    r.RemoteAddr,
								"panic":     rec,
								"answered":  answered,
								"component": "security_timeout",
							}).Error("HTTP request panicked")
						}
					}
					close(done)
				}()
				next.ServeHTTP(tw, r)
			}()

			select {
			case <-done:
				tw.finish()
			case <-ctx.Done():
				expired, started := tw.expire(done, writeTimeoutAnswer)
				if expired && logger != nil && config.LogSecurityEvents {
					logger.WithFields(map[string]any{
						"method":           r.Method,
						"path":             r.URL.Path,
						"client_ip":        getClientIP(r),
						"timeout":          config.RequestTimeout,
						"response_started": started,
						"component":        "security_timeout",
						"security_event":   "request_timeout",
					}).Warn("Request timed out")
				}
			}
		})
	}
}

// writeTimeoutAnswer writes the 408 REQUEST_TIMEOUT answer to w.
func writeTimeoutAnswer(w http.ResponseWriter) {
	response := map[string]interface{}{
		"success": false,
		"error": map[string]interface{}{
			"code":    "REQUEST_TIMEOUT",
			"message": "Request processing timeout",
		},
		"timestamp": time.Now().UTC(),
	}
	body, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	body = append(body, '\n')
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusRequestTimeout)
	_, _ = w.Write(body)
}

// timeoutWriter is the only writer the handler goroutine sees. The real writer
// is touched under mu alone, so the handler and the timeout path never write
// at once, and once the timeout answered, the handler's writes are dropped.
type timeoutWriter struct {
	mu          sync.Mutex
	w           http.ResponseWriter // the real writer; touched only under mu
	h           http.Header         // the handler's own header map; see Header
	up          http.Header         // the headers upstream middleware set; a 408 or 500 answer starts from them
	wroteHeader bool                // the handler's response has started (or the 500 went out)
	timedOut    bool                // the timeout answer went out or was skipped for good
	finished    bool                // the handler returned; net/http has taken the writer back
	failed      bool                // the 500 after a handler panic went out: the response is ours, not the handler's
}

// errWriteAfterFinish is returned to a write that arrives after the handler
// returned, for example from a goroutine the handler started.
var errWriteAfterFinish = errors.New("write after the handler returned")

// newTimeoutWriter starts the handler's header map as a copy of the headers
// set so far by the middleware upstream.
func newTimeoutWriter(w http.ResponseWriter) *timeoutWriter {
	return &timeoutWriter{w: w, h: w.Header().Clone(), up: w.Header().Clone()}
}

// Header returns the handler's own map. Only the handler goroutine uses it
// until the handler has returned, so it needs no lock; it reaches the real
// writer at the first write.
func (tw *timeoutWriter) Header() http.Header { return tw.h }

// WriteHeader starts the response, unless it has started or timed out.
func (tw *timeoutWriter) WriteHeader(code int) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.timedOut || tw.finished || tw.wroteHeader {
		return
	}
	tw.copyHeadersLocked()
	// An informational status (100 to 199, not 101) is not the final one: the
	// handler may still send its own, so the response has not started.
	informational := code >= 100 && code <= 199 && code != http.StatusSwitchingProtocols
	// The real WriteHeader panics on an invalid code. Mark the response as
	// started only after it returned, so that the panic can still be answered.
	tw.w.WriteHeader(code)
	if !informational {
		tw.wroteHeader = true
	}
}

// Write sends body bytes, starting the response with an implicit 200 as
// net/http does. After a timeout it writes nothing and returns
// http.ErrHandlerTimeout; after the handler returned, errWriteAfterFinish.
func (tw *timeoutWriter) Write(b []byte) (int, error) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.timedOut {
		return 0, http.ErrHandlerTimeout
	}
	if tw.finished {
		return 0, errWriteAfterFinish
	}
	if !tw.wroteHeader {
		tw.copyHeadersLocked()
		tw.wroteHeader = true
	}
	return tw.w.Write(b)
}

// copyHeadersLocked makes the real writer's headers equal to the handler's:
// keys the handler deleted go, the rest are set. The caller holds mu.
func (tw *timeoutWriter) copyHeadersLocked() {
	dst := tw.w.Header()
	for k := range dst {
		if _, ok := tw.h[k]; !ok {
			delete(dst, k)
		}
	}
	for k, v := range tw.h {
		dst[k] = slices.Clone(v) // never share a value slice with the handler
	}
}

// resetHeadersLocked puts the real writer's headers back to what upstream
// middleware had set. An answer of ours (408, 500) starts from there, so it
// does not inherit handler headers that an informational status already
// copied over. The caller holds mu.
func (tw *timeoutWriter) resetHeadersLocked() {
	dst := tw.w.Header()
	clear(dst)
	for k, v := range tw.up {
		dst[k] = slices.Clone(v)
	}
}

// finish runs after the handler returned. A handler that set headers and
// wrote nothing still gets them sent with net/http's implicit 200, and
// trailer values it set after its response started reach net/http, which reads
// them from the real writer's header map when the handler returns.
func (tw *timeoutWriter) finish() {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	tw.finishLocked()
}

// finishLocked is finish for a caller that holds mu. From here on the writer
// belongs to net/http again, so every later write or answer is dropped.
func (tw *timeoutWriter) finishLocked() {
	// The 500 after a panic is ours: the handler's headers and trailers do not apply.
	if !tw.failed {
		tw.copyHeadersLocked()
	}
	tw.finished = true
}

// expire handles the deadline. If the handler has already finished (done is
// closed) it keeps that response and sends no timeout answer. Otherwise it
// stops the handler's writes for good and, if the handler has not started its
// response, sends answer on the real writer. It reports whether the request
// timed out and whether the handler had already started its response.
func (tw *timeoutWriter) expire(done <-chan struct{}, answer func(http.ResponseWriter)) (expired, started bool) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	select {
	case <-done:
		tw.finishLocked()
		return false, false
	default:
	}
	tw.timedOut = true
	if !tw.wroteHeader {
		tw.resetHeadersLocked()
		answer(tw.w)
	}
	return true, tw.wroteHeader
}

// fail answers 500 after a panic in the handler, once only and only if
// nothing else has answered. It writes on the real writer, never through
// WriteHeader or Write: those would append the text to a response that has
// already started, or deadlock on mu.
func (tw *timeoutWriter) fail() bool {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.wroteHeader || tw.timedOut || tw.finished {
		return false
	}
	tw.wroteHeader = true
	tw.failed = true
	tw.resetHeadersLocked()
	http.Error(tw.w, "Internal Server Error", http.StatusInternalServerError)
	return true
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

			// Check if IP is blocked (skip trusted IPs like localhost)
			if config.EnableIPBlocking && monitor != nil && !isTrustedIP(clientIP, config.TrustedIPs) && monitor.IsIPBlocked(clientIP) {
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

				response := map[string]interface{}{
					"success": false,
					"error": map[string]interface{}{
						"code":    "IP_BLOCKED",
						"message": "Your IP has been temporarily blocked due to suspicious activity.",
					},
					"timestamp": time.Now().UTC(),
				}

				body, err := json.Marshal(response)
				if err != nil {
					http.Error(w, "Internal server error", http.StatusInternalServerError)
					return
				}
				body = append(body, '\n')
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Content-Length", strconv.Itoa(len(body)))
				w.WriteHeader(http.StatusForbidden)
				if _, writeErr := w.Write(body); writeErr != nil && logger != nil {
					logger.Error("Failed to write IP blocked response", "error", writeErr)
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
