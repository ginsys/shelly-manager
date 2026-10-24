package api

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/mux"

	"github.com/ginsys/shelly-manager/internal/api/middleware"
	apiresp "github.com/ginsys/shelly-manager/internal/api/response"
	"github.com/ginsys/shelly-manager/internal/logging"
	imetrics "github.com/ginsys/shelly-manager/internal/metrics"
)

// httpMetricsMiddleware registers Prometheus HTTP metrics once. Tests build
// SetupRoutesWithSecurity more than once in a process, and promauto panics
// on a second registration against the default registerer. The server calls
// this once.
var (
	httpMetricsOnce sync.Once
	httpMetricsMW   func(http.Handler) http.Handler
)

func httpMetricsMiddleware() func(http.Handler) http.Handler {
	httpMetricsOnce.Do(func() {
		httpMetricsMW = imetrics.NewHTTPMetrics(nil).HTTPMiddleware()
	})
	return httpMetricsMW
}

// SetupRoutes configures all API routes
func SetupRoutes(handler *Handler) *mux.Router {
	return SetupRoutesWithLogger(handler, logging.GetDefault())
}

// SetupRoutesWithLogger configures all API routes with logging middleware
func SetupRoutesWithLogger(handler *Handler, logger *logging.Logger) *mux.Router {
	return SetupRoutesWithSecurity(handler, logger, middleware.DefaultSecurityConfig(), middleware.DefaultValidationConfig())
}

// SetupRoutesWithSecurity configures all API routes with comprehensive security middleware
func SetupRoutesWithSecurity(handler *Handler, logger *logging.Logger, securityConfig *middleware.SecurityConfig, validationConfig *middleware.ValidationConfig) *mux.Router {
	// TEST MODE: Use optimized router
	if os.Getenv("SHELLY_SECURITY_VALIDATION_TEST_MODE") == "true" {
		return SetupTestModeRoutes(handler, logger)
	}

	r := mux.NewRouter()

	// Initialize security monitor
	var securityMonitor *middleware.SecurityMonitor
	if securityConfig.EnableMonitoring {
		securityMonitor = middleware.NewSecurityMonitor(securityConfig, logger)

		// Store security monitor in handler for metrics endpoint access
		if handler != nil {
			handler.securityMonitor = securityMonitor
		}
	}

	// Health endpoints FIRST, with minimal middleware to allow curl health checks
	// These endpoints need to be accessible without user agent validation for CI/CD
	if handler != nil {
		// Create a subrouter for health endpoints with only basic middleware
		healthRouter := r.PathPrefix("/").Subrouter()
		healthRouter.Use(logging.RecoveryMiddleware(logger))
		healthRouter.Use(logging.HTTPMiddleware(logger))
		healthRouter.HandleFunc("/healthz", handler.Healthz).Methods("GET")
		healthRouter.HandleFunc("/readyz", handler.Readyz).Methods("GET")
		// Version endpoint used by UI to detect mismatches
		healthRouter.HandleFunc("/version", handler.Version).Methods("GET")
	}

	// WebSocket endpoint with minimal middleware to preserve Hijacker interface
	if handler.MetricsHandler != nil {
		// Optionally restrict WebSocket origins using security config
		if securityConfig != nil {
			if hub := handler.MetricsHandler.GetWebSocketHub(); hub != nil {
				hub.SetAllowedOrigins(securityConfig.CORSAllowedOrigins)
			}
		}
		wsRouter := r.PathPrefix("/").Subrouter()
		wsRouter.Use(logging.RecoveryMiddleware(logger))
		wsRouter.HandleFunc("/metrics/ws", handler.MetricsHandler.HandleWebSocket).Methods("GET")
	}

	// Create protected subrouter for all other routes with full security middleware
	protected := r.PathPrefix("/").Subrouter()

	// Apply security middleware in proper order:
	// 1. Recovery middleware (catch panics first)
	protected.Use(logging.RecoveryMiddleware(logger))

	// 2. IP blocking middleware (block malicious IPs early)
	if securityConfig.EnableIPBlocking && securityMonitor != nil {
		protected.Use(middleware.IPBlockingMiddleware(securityConfig, securityMonitor, logger))
	}

	// 3. Security monitoring middleware (track all requests)
	if securityMonitor != nil {
		protected.Use(middleware.MonitoringMiddleware(securityConfig, securityMonitor, logger))
	}

	// 4. Security logging middleware (log all requests for monitoring)
	protected.Use(middleware.SecurityLoggingMiddleware(securityConfig, logger))

	// 5. Security headers middleware (set security headers early)
	protected.Use(middleware.SecurityHeadersMiddleware(securityConfig, logger))

	// 6. Request timeout middleware (prevent resource exhaustion)
	protected.Use(middleware.TimeoutMiddleware(securityConfig, logger))

	// 7. Rate limiting middleware (prevent DoS attacks)
	protected.Use(middleware.RateLimitMiddleware(securityConfig, logger))

	// 8. Request size limiting middleware (prevent large payload attacks)
	protected.Use(middleware.RequestSizeMiddleware(securityConfig, logger))

	// 9. Request validation middleware (validate headers, content types, etc.)
	protected.Use(middleware.ValidateHeadersMiddleware(validationConfig, logger))
	protected.Use(middleware.ValidateContentTypeMiddleware(validationConfig, logger))
	protected.Use(middleware.ValidateQueryParamsMiddleware(validationConfig, logger))
	protected.Use(middleware.ValidateJSONMiddleware(validationConfig, logger))

	// 10. Enhanced CORS middleware (security-aware CORS handling)
	protected.Use(enhancedCORSMiddleware(logger, securityConfig))

	// 11. Standard logging middleware (existing functionality)
	protected.Use(logging.HTTPMiddleware(logger))

	// 12. Prometheus HTTP metrics middleware (baseline observability)
	if handler != nil {
		protected.Use(httpMetricsMiddleware())
	}

	// API routes. Test mode calls registerAPIRoutes on its own subrouter.
	// Middleware above is the only difference, except GET /api/v1/metrics/security,
	// which closes over securityMonitor and is omitted when that is nil.
	api := protected.PathPrefix("/api/v1").Subrouter()
	registerAPIRoutes(api, handler, logger, securityMonitor)

	// Static file serving removed (Phase 8): legacy UI is deleted; SPA will be served by the new UI build.

	return r
}

// enhancedCORSMiddleware provides security-aware CORS handling
func enhancedCORSMiddleware(logger *logging.Logger, config *middleware.SecurityConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Determine allowed origin
			allowedOrigin := "*"
			if config != nil && len(config.CORSAllowedOrigins) > 0 {
				// If a specific list is configured, only echo back when matched
				for _, ao := range config.CORSAllowedOrigins {
					if ao == "*" || ao == origin {
						allowedOrigin = origin
						break
					}
				}
				if origin == "" {
					allowedOrigin = "*"
				}
			}

			// Security-aware CORS headers
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			w.Header().Set("Vary", "Origin")

			methods := "GET, POST, PUT, DELETE, OPTIONS"
			if config != nil && len(config.CORSAllowedMethods) > 0 {
				methods = strings.Join(config.CORSAllowedMethods, ", ")
			}
			headers := "Content-Type, Authorization, X-Requested-With"
			if config != nil && len(config.CORSAllowedHeaders) > 0 {
				headers = strings.Join(config.CORSAllowedHeaders, ", ")
			}
			maxAge := "86400"
			if config != nil && config.CORSMaxAge > 0 {
				maxAge = strconv.Itoa(config.CORSMaxAge)
			}
			w.Header().Set("Access-Control-Allow-Methods", methods)
			w.Header().Set("Access-Control-Allow-Headers", headers)
			w.Header().Set("Access-Control-Expose-Headers", "X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset")
			w.Header().Set("Access-Control-Max-Age", maxAge)

			// Log CORS requests for security monitoring
			if origin != "" {
				logger.WithFields(map[string]any{
					"method":    r.Method,
					"path":      r.URL.Path,
					"origin":    origin,
					"component": "cors",
				}).Debug("CORS request processed")

				// TODO: Add origin validation for production
				// For now, accept all origins but log them for monitoring
			}

			// Handle preflight requests
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// createSecurityMetricsHandler creates a handler for security metrics
func createSecurityMetricsHandler(monitor *middleware.SecurityMonitor, logger *logging.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rw := apiresp.NewResponseWriter(logger)
		if monitor == nil {
			rw.WriteError(w, r, http.StatusServiceUnavailable, apiresp.ErrCodeServiceUnavailable, "Security monitoring is not enabled", nil)
			return
		}

		metrics := monitor.GetMetrics()
		rw.WriteSuccess(w, r, metrics)
	}
}

// SetupTestModeRoutes creates an optimized router with minimal middleware stack for testing
func SetupTestModeRoutes(handler *Handler, logger *logging.Logger) *mux.Router {
	r := mux.NewRouter()

	// Health endpoints with ZERO middleware for maximum speed
	if handler != nil {
		r.HandleFunc("/healthz", handler.FastHealthz).Methods("GET")
		r.HandleFunc("/readyz", handler.FastHealthz).Methods("GET")
		// Version endpoint for UI parity
		r.HandleFunc("/version", handler.Version).Methods("GET")
	}

	// WebSocket with minimal middleware (if metrics enabled)
	if handler.MetricsHandler != nil {
		r.HandleFunc("/metrics/ws", handler.MetricsHandler.HandleWebSocket).Methods("GET")
	}

	// Same /api/v1 table as production. Middleware stays minimal.
	// securityMonitor is nil, so GET /api/v1/metrics/security is not registered.
	api := r.PathPrefix("/api/v1").Subrouter()
	api.Use(logging.RecoveryMiddleware(logger)) // Only recovery for error handling
	api.Use(testModeCORSMiddleware(logger))     // Minimal CORS for browser tests

	if handler != nil {
		registerAPIRoutes(api, handler, logger, nil)
	}

	logger.Info("Configured test mode router with the production API table")
	return r
}

// testModeCORSMiddleware provides minimal CORS middleware for test mode
func testModeCORSMiddleware(logger *logging.Logger) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
