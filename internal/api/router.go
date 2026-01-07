package api

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gorilla/mux"

	"github.com/ginsys/shelly-manager/internal/api/middleware"
	apiresp "github.com/ginsys/shelly-manager/internal/api/response"
	"github.com/ginsys/shelly-manager/internal/logging"
	imetrics "github.com/ginsys/shelly-manager/internal/metrics"
)

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
		hm := imetrics.NewHTTPMetrics(nil)
		protected.Use(hm.HTTPMiddleware())
	}

	// API routes - use protected subrouter for full security middleware
	api := protected.PathPrefix("/api/v1").Subrouter()

	// Handle CORS preflight for any API path with a generic OPTIONS responder.
	// This ensures cross-origin clients receive proper CORS headers instead of 404.
	api.PathPrefix("/").Methods("OPTIONS").Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The enhanced CORS middleware already set headers; just return 200 OK.
		w.WriteHeader(http.StatusOK)
	}))

	// Admin routes (guarded by simple admin key if configured)
	api.HandleFunc("/admin/rotate-admin-key", handler.RotateAdminKey).Methods("POST")

	// Device routes
	api.HandleFunc("/devices", handler.GetDevices).Methods("GET")
	api.HandleFunc("/devices", handler.AddDevice).Methods("POST")
	api.HandleFunc("/devices/{id}", handler.GetDevice).Methods("GET")
	api.HandleFunc("/devices/{id}", handler.UpdateDevice).Methods("PUT")
	api.HandleFunc("/devices/{id}", handler.DeleteDevice).Methods("DELETE")

	// Device control routes
	api.HandleFunc("/devices/{id}/control", handler.ControlDevice).Methods("POST")
	api.HandleFunc("/devices/{id}/status", handler.GetDeviceStatus).Methods("GET")
	api.HandleFunc("/devices/{id}/energy", handler.GetDeviceEnergy).Methods("GET")

	// Device configuration routes
	api.HandleFunc("/devices/{id}/config", handler.GetDeviceConfig).Methods("GET")
	api.HandleFunc("/devices/{id}/config", handler.UpdateDeviceConfig).Methods("PUT")
	api.HandleFunc("/devices/{id}/config/current", handler.GetCurrentDeviceConfig).Methods("GET")
	api.HandleFunc("/devices/{id}/config/current/normalized", handler.GetCurrentDeviceConfigNormalized).Methods("GET")
	api.HandleFunc("/devices/{id}/config/typed/normalized", handler.GetTypedDeviceConfigNormalized).Methods("GET")
	api.HandleFunc("/devices/{id}/config/import", handler.ImportDeviceConfig).Methods("POST")
	api.HandleFunc("/devices/{id}/config/status", handler.GetImportStatus).Methods("GET")
	api.HandleFunc("/devices/{id}/config/export", handler.ExportDeviceConfig).Methods("POST")
	api.HandleFunc("/devices/{id}/config/drift", handler.DetectConfigDrift).Methods("GET")
	api.HandleFunc("/devices/{id}/config/apply-template", handler.ApplyConfigTemplate).Methods("POST")
	api.HandleFunc("/devices/{id}/config/history", handler.GetConfigHistory).Methods("GET")

	// Device capability-specific configuration routes
	api.HandleFunc("/devices/{id}/config/relay", handler.UpdateRelayConfig).Methods("PUT")
	api.HandleFunc("/devices/{id}/config/dimming", handler.UpdateDimmingConfig).Methods("PUT")
	api.HandleFunc("/devices/{id}/config/roller", handler.UpdateRollerConfig).Methods("PUT")
	api.HandleFunc("/devices/{id}/config/power-metering", handler.UpdatePowerMeteringConfig).Methods("PUT")
	api.HandleFunc("/devices/{id}/config/auth", handler.UpdateDeviceAuth).Methods("PUT")

	// Configuration template routes
	api.HandleFunc("/config/templates", handler.GetConfigTemplates).Methods("GET")
	api.HandleFunc("/config/templates", handler.CreateConfigTemplate).Methods("POST")
	api.HandleFunc("/config/templates/{id}", handler.UpdateConfigTemplate).Methods("PUT")
	api.HandleFunc("/config/templates/{id}", handler.DeleteConfigTemplate).Methods("DELETE")

	// Template variable substitution routes
	api.HandleFunc("/configuration/preview-template", handler.PreviewTemplate).Methods("POST")
	api.HandleFunc("/configuration/validate-template", handler.ValidateTemplate).Methods("POST")
	api.HandleFunc("/configuration/templates", handler.SaveTemplate).Methods("POST")
	api.HandleFunc("/configuration/template-examples", handler.GetTemplateExamples).Methods("GET")

	// Typed configuration routes
	api.HandleFunc("/devices/{id}/config/typed", handler.GetTypedDeviceConfig).Methods("GET")
	api.HandleFunc("/devices/{id}/config/typed", handler.UpdateTypedDeviceConfig).Methods("PUT")
	api.HandleFunc("/devices/{id}/capabilities", handler.GetDeviceCapabilities).Methods("GET")
	api.HandleFunc("/configuration/validate-typed", handler.ValidateTypedConfig).Methods("POST")
	api.HandleFunc("/configuration/convert-to-typed", handler.ConvertConfigToTyped).Methods("POST")
	api.HandleFunc("/configuration/convert-to-raw", handler.ConvertTypedToRaw).Methods("POST")
	api.HandleFunc("/configuration/schema", handler.GetConfigurationSchema).Methods("GET")
	api.HandleFunc("/configuration/bulk-validate", handler.BulkValidateConfigs).Methods("POST")

	// Bulk configuration operations
	api.HandleFunc("/config/bulk-import", handler.BulkImportConfigs).Methods("POST")
	api.HandleFunc("/config/bulk-export", handler.BulkExportConfigs).Methods("POST")
	api.HandleFunc("/config/bulk-drift-detect", handler.BulkDetectConfigDrift).Methods("POST")
	api.HandleFunc("/config/bulk-drift-detect-enhanced", handler.EnhancedBulkDetectConfigDrift).Methods("POST")

	// Drift detection schedule routes
	api.HandleFunc("/config/drift-schedules", handler.GetDriftSchedules).Methods("GET")
	api.HandleFunc("/config/drift-schedules", handler.CreateDriftSchedule).Methods("POST")
	api.HandleFunc("/config/drift-schedules/{id}", handler.GetDriftSchedule).Methods("GET")
	api.HandleFunc("/config/drift-schedules/{id}", handler.UpdateDriftSchedule).Methods("PUT")
	api.HandleFunc("/config/drift-schedules/{id}", handler.DeleteDriftSchedule).Methods("DELETE")
	api.HandleFunc("/config/drift-schedules/{id}/toggle", handler.ToggleDriftSchedule).Methods("POST")
	api.HandleFunc("/config/drift-schedules/{id}/runs", handler.GetDriftScheduleRuns).Methods("GET")

	// Comprehensive drift reporting routes
	api.HandleFunc("/config/drift-reports", handler.GetDriftReports).Methods("GET")
	api.HandleFunc("/config/drift-trends", handler.GetDriftTrends).Methods("GET")
	api.HandleFunc("/config/drift-trends/{id}/resolve", handler.MarkTrendResolved).Methods("POST")

	// Device-specific drift reporting
	api.HandleFunc("/devices/{id}/drift-report", handler.GenerateDeviceDriftReport).Methods("POST")

	// New template management routes (pointer-based config system)
	api.HandleFunc("/config/templates/new", handler.GetNewConfigTemplates).Methods("GET")
	api.HandleFunc("/config/templates/new", handler.CreateNewConfigTemplate).Methods("POST")
	api.HandleFunc("/config/templates/new/{id}", handler.GetNewConfigTemplate).Methods("GET")
	api.HandleFunc("/config/templates/new/{id}", handler.UpdateNewConfigTemplate).Methods("PUT")
	api.HandleFunc("/config/templates/new/{id}", handler.DeleteNewConfigTemplate).Methods("DELETE")

	// Device template assignment routes
	api.HandleFunc("/devices/{id}/templates/new", handler.GetDeviceNewTemplates).Methods("GET")
	api.HandleFunc("/devices/{id}/templates/new", handler.SetDeviceNewTemplates).Methods("PUT")
	api.HandleFunc("/devices/{id}/templates/new/{templateId}", handler.AddDeviceNewTemplate).Methods("POST")
	api.HandleFunc("/devices/{id}/templates/new/{templateId}", handler.RemoveDeviceNewTemplate).Methods("DELETE")

	// Device tag routes
	api.HandleFunc("/devices/{id}/tags/new", handler.GetDeviceNewTags).Methods("GET")
	api.HandleFunc("/devices/{id}/tags/new", handler.AddDeviceNewTag).Methods("POST")
	api.HandleFunc("/devices/{id}/tags/new/{tag}", handler.RemoveDeviceNewTag).Methods("DELETE")
	api.HandleFunc("/tags/new", handler.ListAllNewTags).Methods("GET")
	api.HandleFunc("/tags/new/{tag}/devices", handler.GetDevicesByNewTag).Methods("GET")

	// Device override routes
	api.HandleFunc("/devices/{id}/overrides/new", handler.GetDeviceNewOverrides).Methods("GET")
	api.HandleFunc("/devices/{id}/overrides/new", handler.SetDeviceNewOverrides).Methods("PUT")
	api.HandleFunc("/devices/{id}/overrides/new", handler.PatchDeviceNewOverrides).Methods("PATCH")
	api.HandleFunc("/devices/{id}/overrides/new", handler.DeleteDeviceNewOverrides).Methods("DELETE")

	// Desired config and apply routes
	api.HandleFunc("/devices/{id}/desired-config", handler.GetDeviceDesiredConfig).Methods("GET")
	api.HandleFunc("/devices/{id}/config/new/apply", handler.ApplyDeviceNewConfig).Methods("POST")
	api.HandleFunc("/devices/{id}/config/new/status", handler.GetDeviceNewConfigStatus).Methods("GET")
	api.HandleFunc("/devices/{id}/config/new/verify", handler.VerifyDeviceNewConfig).Methods("POST")

	// Notification routes
	if handler != nil && handler.NotificationHandler != nil {
		api.HandleFunc("/notifications/channels", handler.NotificationHandler.CreateChannel).Methods("POST")
		api.HandleFunc("/notifications/channels", handler.NotificationHandler.GetChannels).Methods("GET")
		api.HandleFunc("/notifications/channels/{id}", handler.NotificationHandler.UpdateChannel).Methods("PUT")
		api.HandleFunc("/notifications/channels/{id}", handler.NotificationHandler.DeleteChannel).Methods("DELETE")
		api.HandleFunc("/notifications/channels/{id}/test", handler.NotificationHandler.TestChannel).Methods("POST")
		api.HandleFunc("/notifications/rules", handler.NotificationHandler.CreateRule).Methods("POST")
		api.HandleFunc("/notifications/rules", handler.NotificationHandler.GetRules).Methods("GET")
		api.HandleFunc("/notifications/history", handler.NotificationHandler.GetHistory).Methods("GET")
	}

	// Metrics routes (non-WebSocket)
	if handler.MetricsHandler != nil {
		metricsAPI := r.PathPrefix("/metrics").Subrouter()

		// Prometheus metrics endpoint
		metricsAPI.Handle("/prometheus", handler.MetricsHandler.PrometheusHandler()).Methods("GET")

		// Control endpoints
		metricsAPI.HandleFunc("/status", handler.MetricsHandler.GetMetricsStatus).Methods("GET")
		metricsAPI.HandleFunc("/enable", handler.MetricsHandler.EnableMetrics).Methods("POST")
		metricsAPI.HandleFunc("/disable", handler.MetricsHandler.DisableMetrics).Methods("POST")
		metricsAPI.HandleFunc("/collect", handler.MetricsHandler.CollectMetrics).Methods("POST")

		// Dashboard endpoints
		metricsAPI.HandleFunc("/dashboard", handler.MetricsHandler.GetDashboardMetrics).Methods("GET")
		metricsAPI.HandleFunc("/test-alert", handler.MetricsHandler.SendTestAlert).Methods("POST")

		// Health and summary endpoints (admin-key protected if configured)
		metricsAPI.HandleFunc("/health", handler.MetricsHandler.GetHealth).Methods("GET")
		metricsAPI.HandleFunc("/system", handler.MetricsHandler.GetSystemMetrics).Methods("GET")
		metricsAPI.HandleFunc("/devices", handler.MetricsHandler.GetDevicesMetrics).Methods("GET")
		metricsAPI.HandleFunc("/drift", handler.MetricsHandler.GetDriftSummary).Methods("GET")
		metricsAPI.HandleFunc("/notifications", handler.MetricsHandler.GetNotificationSummary).Methods("GET")
		metricsAPI.HandleFunc("/resolution", handler.MetricsHandler.GetResolutionSummary).Methods("GET")

		// Security metrics endpoint
		if securityMonitor != nil {
			metricsAPI.HandleFunc("/security", createSecurityMetricsHandler(securityMonitor, logger)).Methods("GET")
		}
	}

	// Discovery route
	api.HandleFunc("/discover", handler.DiscoverHandler).Methods("POST")

	// Provisioning routes
	api.HandleFunc("/provisioning/status", handler.GetProvisioningStatus).Methods("GET")
	api.HandleFunc("/provisioning/provision", handler.ProvisionDevices).Methods("POST")

	// Provisioner agent management routes
	api.HandleFunc("/provisioner/agents/register", handler.RegisterAgent).Methods("POST")
	api.HandleFunc("/provisioner/agents", handler.GetProvisionerAgents).Methods("GET")
	api.HandleFunc("/provisioner/agents/{id}/tasks", handler.PollTasks).Methods("GET")
	api.HandleFunc("/provisioner/tasks", handler.CreateProvisioningTask).Methods("POST")
	api.HandleFunc("/provisioner/tasks", handler.GetProvisioningTasks).Methods("GET")
	api.HandleFunc("/provisioner/tasks/{id}/status", handler.UpdateTaskStatus).Methods("PUT")
	api.HandleFunc("/provisioner/discovered-devices", handler.ReportDiscoveredDevices).Methods("POST")
	api.HandleFunc("/provisioner/discovered-devices", handler.GetDiscoveredDevices).Methods("GET")
	api.HandleFunc("/provisioner/health", handler.ProvisionerHealthCheck).Methods("GET")

	// DHCP routes
	api.HandleFunc("/dhcp/reservations", handler.GetDHCPReservations).Methods("GET")

	// Export/Import routes (if handlers are configured)
	if handler.ExportHandlers != nil {
		handler.ExportHandlers.AddExportRoutes(api)
	}
	if handler.ImportHandlers != nil {
		handler.ImportHandlers.AddImportRoutes(api)
	}

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

	// API routes with only essential middleware
	api := r.PathPrefix("/api/v1").Subrouter()
	api.Use(logging.RecoveryMiddleware(logger)) // Only recovery for error handling
	api.Use(testModeCORSMiddleware(logger))     // Minimal CORS for browser tests

	// Register core API routes WITHOUT security middleware stack
	if handler != nil {
		// Essential device management routes
		api.HandleFunc("/devices", handler.GetDevices).Methods("GET", "OPTIONS")
		api.HandleFunc("/devices", handler.AddDevice).Methods("POST", "OPTIONS")
		api.HandleFunc("/devices/{id}", handler.GetDevice).Methods("GET", "OPTIONS")
		api.HandleFunc("/devices/{id}/control", handler.ControlDevice).Methods("POST", "OPTIONS")

		// Discovery
		api.HandleFunc("/discover", handler.DiscoverHandler).Methods("POST", "OPTIONS")

		// OPTIONS handler for all API paths
		api.PathPrefix("/").Methods("OPTIONS").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
	}

	logger.Info("Configured test mode router with minimal middleware")
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
