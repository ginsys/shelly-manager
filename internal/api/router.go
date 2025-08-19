package api

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// SetupRoutes configures all API routes
func SetupRoutes(handler *Handler) *mux.Router {
	return SetupRoutesWithLogger(handler, logging.GetDefault())
}

// SetupRoutesWithLogger configures all API routes with logging middleware
func SetupRoutesWithLogger(handler *Handler, logger *logging.Logger) *mux.Router {
	r := mux.NewRouter()

	// WebSocket endpoint FIRST, without any middleware to preserve Hijacker interface
	if handler.MetricsHandler != nil {
		r.HandleFunc("/metrics/ws", handler.MetricsHandler.HandleWebSocket).Methods("GET")
	}

	// Add logging middleware to all other routes
	r.Use(logging.HTTPMiddleware(logger))
	r.Use(logging.RecoveryMiddleware(logger))
	r.Use(logging.CORSMiddleware(logger))

	// API routes
	api := r.PathPrefix("/api/v1").Subrouter()

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

	// Notification routes
	if handler.NotificationHandler != nil {
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

	// Static file serving
	r.PathPrefix("/web/").Handler(http.StripPrefix("/web/", http.FileServer(http.Dir("./web/static/"))))
	r.PathPrefix("/").Handler(http.StripPrefix("/", http.FileServer(http.Dir("./web/static/"))))

	return r
}
