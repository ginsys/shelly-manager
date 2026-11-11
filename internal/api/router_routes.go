package api

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/ginsys/shelly-manager/internal/api/middleware"
	"github.com/ginsys/shelly-manager/internal/logging"
)

// adminOnly wraps a handler so it answers only a request that carries the admin key, when one is
// set. With no key set the route stays open, as with every handler that calls requireAdmin.
// Guarding a route here, in the table, means a new handler cannot forget the call.
func (h *Handler) adminOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.requireAdmin(w, r) {
			return
		}
		next(w, r)
	}
}

// registerAPIRoutes registers the production /api/v1 table on api.
// securityMonitor nil omits GET /metrics/security, which closes over it.
// handler must be non-nil. Optional notification, metrics, export, and import
// routes follow the handler fields, same as before this was shared.
func registerAPIRoutes(api *mux.Router, handler *Handler, logger *logging.Logger, securityMonitor *middleware.SecurityMonitor) {
	// Handle CORS preflight for any API path with a generic OPTIONS responder.
	// This ensures cross-origin clients receive proper CORS headers instead of 404.
	api.PathPrefix("/").Methods("OPTIONS").Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The CORS middleware already set headers; just return 200 OK.
		w.WriteHeader(http.StatusOK)
	}))

	// Version info for the UI, whose client prefixes every call with /api/v1. The root
	// GET /version stays for health tooling.
	api.HandleFunc("/version", handler.Version).Methods("GET")

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

	// admin guards a route with the admin key. Every configuration route uses it, except the
	// bulk import, export and drift routes, whose handlers call requireAdmin themselves.
	admin := handler.adminOnly

	// Device configuration routes
	api.HandleFunc("/devices/{id}/config", admin(handler.GetDeviceConfig)).Methods("GET")
	api.HandleFunc("/devices/{id}/config", admin(handler.UpdateDeviceConfig)).Methods("PUT")
	api.HandleFunc("/devices/{id}/config/current", admin(handler.GetCurrentDeviceConfig)).Methods("GET")
	api.HandleFunc("/devices/{id}/config/current/normalized", admin(handler.GetCurrentDeviceConfigNormalized)).Methods("GET")
	api.HandleFunc("/devices/{id}/config/typed/normalized", admin(handler.GetTypedDeviceConfigNormalized)).Methods("GET")
	api.HandleFunc("/devices/{id}/config/import", admin(handler.ImportDeviceConfig)).Methods("POST")
	api.HandleFunc("/devices/{id}/config/status", admin(handler.GetImportStatus)).Methods("GET")
	api.HandleFunc("/devices/{id}/config/export", admin(handler.ExportDeviceConfig)).Methods("POST")
	api.HandleFunc("/devices/{id}/config/drift", admin(handler.DetectConfigDrift)).Methods("GET")
	api.HandleFunc("/devices/{id}/config/apply-template", admin(handler.ApplyConfigTemplate)).Methods("POST")
	api.HandleFunc("/devices/{id}/config/history", admin(handler.GetConfigHistory)).Methods("GET")

	// Device capability-specific configuration routes
	api.HandleFunc("/devices/{id}/config/relay", admin(handler.UpdateRelayConfig)).Methods("PUT")
	api.HandleFunc("/devices/{id}/config/dimming", admin(handler.UpdateDimmingConfig)).Methods("PUT")
	api.HandleFunc("/devices/{id}/config/roller", admin(handler.UpdateRollerConfig)).Methods("PUT")
	api.HandleFunc("/devices/{id}/config/power-metering", admin(handler.UpdatePowerMeteringConfig)).Methods("PUT")
	api.HandleFunc("/devices/{id}/config/auth", admin(handler.UpdateDeviceAuth)).Methods("PUT")

	// Configuration template routes
	api.HandleFunc("/config/templates", admin(handler.GetConfigTemplates)).Methods("GET")
	api.HandleFunc("/config/templates", admin(handler.CreateConfigTemplate)).Methods("POST")
	api.HandleFunc("/config/templates/{id}", admin(handler.UpdateConfigTemplate)).Methods("PUT")
	api.HandleFunc("/config/templates/{id}", admin(handler.DeleteConfigTemplate)).Methods("DELETE")

	// Typed configuration routes
	api.HandleFunc("/devices/{id}/config/typed", admin(handler.GetTypedDeviceConfig)).Methods("GET")
	api.HandleFunc("/devices/{id}/config/typed", admin(handler.UpdateTypedDeviceConfig)).Methods("PUT")
	api.HandleFunc("/devices/{id}/capabilities", handler.GetDeviceCapabilities).Methods("GET")
	api.HandleFunc("/config/validate-typed", admin(handler.ValidateTypedConfig)).Methods("POST")
	api.HandleFunc("/config/convert-to-typed", admin(handler.ConvertConfigToTyped)).Methods("POST")
	api.HandleFunc("/config/convert-to-raw", admin(handler.ConvertTypedToRaw)).Methods("POST")
	api.HandleFunc("/config/schema", admin(handler.GetConfigurationSchema)).Methods("GET")
	api.HandleFunc("/config/device-types", handler.GetDeviceTypes).Methods("GET")
	api.HandleFunc("/config/bulk-validate", admin(handler.BulkValidateConfigs)).Methods("POST")

	// Bulk configuration operations
	api.HandleFunc("/config/bulk-import", handler.BulkImportConfigs).Methods("POST")
	api.HandleFunc("/config/bulk-export", handler.BulkExportConfigs).Methods("POST")
	api.HandleFunc("/config/bulk-drift-detect", handler.BulkDetectConfigDrift).Methods("POST")
	api.HandleFunc("/config/bulk-drift-detect-enhanced", handler.EnhancedBulkDetectConfigDrift).Methods("POST")

	// Drift detection schedule routes
	api.HandleFunc("/config/drift-schedules", admin(handler.GetDriftSchedules)).Methods("GET")
	api.HandleFunc("/config/drift-schedules", admin(handler.CreateDriftSchedule)).Methods("POST")
	api.HandleFunc("/config/drift-schedules/{id}", admin(handler.GetDriftSchedule)).Methods("GET")
	api.HandleFunc("/config/drift-schedules/{id}", admin(handler.UpdateDriftSchedule)).Methods("PUT")
	api.HandleFunc("/config/drift-schedules/{id}", admin(handler.DeleteDriftSchedule)).Methods("DELETE")
	api.HandleFunc("/config/drift-schedules/{id}/toggle", admin(handler.ToggleDriftSchedule)).Methods("POST")
	api.HandleFunc("/config/drift-schedules/{id}/runs", admin(handler.GetDriftScheduleRuns)).Methods("GET")

	// Comprehensive drift reporting routes
	api.HandleFunc("/config/drift-reports", admin(handler.GetDriftReports)).Methods("GET")
	api.HandleFunc("/config/drift-trends", admin(handler.GetDriftTrends)).Methods("GET")
	api.HandleFunc("/config/drift-trends/{id}/resolve", admin(handler.MarkTrendResolved)).Methods("POST")

	// Device-specific drift reporting
	api.HandleFunc("/devices/{id}/drift-report", admin(handler.GenerateDeviceDriftReport)).Methods("POST")

	// New template management routes (pointer-based config system)
	api.HandleFunc("/config/templates/new", admin(handler.GetNewConfigTemplates)).Methods("GET")
	api.HandleFunc("/config/templates/new", admin(handler.CreateNewConfigTemplate)).Methods("POST")
	api.HandleFunc("/config/templates/new/{id}", admin(handler.GetNewConfigTemplate)).Methods("GET")
	api.HandleFunc("/config/templates/new/{id}", admin(handler.UpdateNewConfigTemplate)).Methods("PUT")
	api.HandleFunc("/config/templates/new/{id}", admin(handler.DeleteNewConfigTemplate)).Methods("DELETE")

	// Device template assignment routes
	api.HandleFunc("/devices/{id}/templates/new", admin(handler.GetDeviceNewTemplates)).Methods("GET")
	api.HandleFunc("/devices/{id}/templates/new", admin(handler.SetDeviceNewTemplates)).Methods("PUT")
	api.HandleFunc("/devices/{id}/templates/new/{templateId}", admin(handler.AddDeviceNewTemplate)).Methods("POST")
	api.HandleFunc("/devices/{id}/templates/new/{templateId}", admin(handler.RemoveDeviceNewTemplate)).Methods("DELETE")

	// Device tag routes
	api.HandleFunc("/devices/{id}/tags/new", handler.GetDeviceNewTags).Methods("GET")
	api.HandleFunc("/devices/{id}/tags/new", handler.AddDeviceNewTag).Methods("POST")
	api.HandleFunc("/devices/{id}/tags/new/{tag}", handler.RemoveDeviceNewTag).Methods("DELETE")
	api.HandleFunc("/tags/new", handler.ListAllNewTags).Methods("GET")
	api.HandleFunc("/tags/new/{tag}/devices", handler.GetDevicesByNewTag).Methods("GET")

	// Device override routes
	api.HandleFunc("/devices/{id}/overrides/new", admin(handler.GetDeviceNewOverrides)).Methods("GET")
	api.HandleFunc("/devices/{id}/overrides/new", admin(handler.SetDeviceNewOverrides)).Methods("PUT")
	api.HandleFunc("/devices/{id}/overrides/new", admin(handler.PatchDeviceNewOverrides)).Methods("PATCH")
	api.HandleFunc("/devices/{id}/overrides/new", admin(handler.DeleteDeviceNewOverrides)).Methods("DELETE")

	// Desired config and apply routes. Apply also checks the key in its own handler.
	api.HandleFunc("/devices/{id}/desired-config", admin(handler.GetDeviceDesiredConfig)).Methods("GET")
	api.HandleFunc("/devices/{id}/config/new/apply", handler.ApplyDeviceNewConfig).Methods("POST")
	api.HandleFunc("/devices/{id}/config/new/status", admin(handler.GetDeviceNewConfigStatus)).Methods("GET")
	api.HandleFunc("/devices/{id}/config/new/verify", admin(handler.VerifyDeviceNewConfig)).Methods("POST")

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

	// Metrics routes (non-WebSocket) — under /api/v1 so the frontend's axios baseURL works
	if handler.MetricsHandler != nil {
		metricsAPI := api.PathPrefix("/metrics").Subrouter()

		// Admin-key policy for /metrics/* (enforced per-handler via requireAdmin,
		// which is a no-op when no admin key is configured):
		//   Gated (mutating): /enable /disable /collect /test-alert
		//   Gated (data reads, consistent with /health /system /...): /status /dashboard
		//     plus /health /system /devices /drift /notifications /resolution /security
		//   Public: /prometheus — standard scrapers do not send the admin bearer, and
		//     it exposes only aggregate counters; secure it at the network layer instead.
		//   The /metrics/ws real-time stream is a separate concern (see #247).

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

		// Security metrics endpoint — exposes attacker IPs and activity, so gate it
		// with the same admin key as the other protected metrics reads.
		// Absent when securityMonitor is nil (test mode does not construct one).
		if securityMonitor != nil {
			securityHandler := createSecurityMetricsHandler(securityMonitor, logger)
			metricsAPI.HandleFunc("/security", func(w http.ResponseWriter, r *http.Request) {
				if !handler.requireAdmin(w, r) {
					return
				}
				securityHandler(w, r)
			}).Methods("GET")
		}

		// Prometheus endpoint
		metricsAPI.Handle("/prometheus", handler.MetricsHandler.PrometheusHandler()).Methods("GET")
	}

	// Discovery route
	api.HandleFunc("/discover", handler.DiscoverHandler).Methods("POST")

	// Provisioning routes
	api.HandleFunc("/provisioning/status", handler.GetProvisioningStatus).Methods("GET")
	api.HandleFunc("/provisioning/provision", handler.ProvisionDevices).Methods("POST")

	// UI-facing provisioning task/agent routes (admin-key auth)
	api.HandleFunc("/provisioning/tasks", handler.ListProvisioningTasksUI).Methods("GET")
	api.HandleFunc("/provisioning/tasks", handler.CreateProvisioningTaskUI).Methods("POST")
	api.HandleFunc("/provisioning/tasks/{id}", handler.GetProvisioningTaskUI).Methods("GET")
	api.HandleFunc("/provisioning/tasks/{id}/cancel", handler.CancelProvisioningTaskUI).Methods("POST")
	api.HandleFunc("/provisioning/bulk", handler.BulkProvisionUI).Methods("POST")
	api.HandleFunc("/provisioning/agents", handler.ListProvisioningAgentsUI).Methods("GET")
	api.HandleFunc("/provisioning/agents/{id}/status", handler.GetProvisioningAgentStatusUI).Methods("GET")

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
}
