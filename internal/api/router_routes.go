package api

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/ginsys/shelly-manager/internal/api/middleware"
	"github.com/ginsys/shelly-manager/internal/logging"
)

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

	// Typed configuration routes
	api.HandleFunc("/devices/{id}/config/typed", handler.GetTypedDeviceConfig).Methods("GET")
	api.HandleFunc("/devices/{id}/config/typed", handler.UpdateTypedDeviceConfig).Methods("PUT")
	api.HandleFunc("/devices/{id}/capabilities", handler.GetDeviceCapabilities).Methods("GET")
	api.HandleFunc("/config/validate-typed", handler.ValidateTypedConfig).Methods("POST")
	api.HandleFunc("/config/convert-to-typed", handler.ConvertConfigToTyped).Methods("POST")
	api.HandleFunc("/config/convert-to-raw", handler.ConvertTypedToRaw).Methods("POST")
	api.HandleFunc("/config/schema", handler.GetConfigurationSchema).Methods("GET")
	api.HandleFunc("/config/bulk-validate", handler.BulkValidateConfigs).Methods("POST")

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
