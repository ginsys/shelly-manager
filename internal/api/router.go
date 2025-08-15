package api

import (
	"net/http"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/gorilla/mux"
)

// SetupRoutes configures all API routes
func SetupRoutes(handler *Handler) *mux.Router {
	return SetupRoutesWithLogger(handler, logging.GetDefault())
}

// SetupRoutesWithLogger configures all API routes with logging middleware
func SetupRoutesWithLogger(handler *Handler, logger *logging.Logger) *mux.Router {
	r := mux.NewRouter()
	
	// Add logging middleware
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
	
	// Bulk configuration operations
	api.HandleFunc("/config/bulk-import", handler.BulkImportConfigs).Methods("POST")
	api.HandleFunc("/config/bulk-export", handler.BulkExportConfigs).Methods("POST")
	api.HandleFunc("/config/bulk-drift-detect", handler.BulkDetectConfigDrift).Methods("POST")
	
	// Drift detection schedule routes
	api.HandleFunc("/config/drift-schedules", handler.GetDriftSchedules).Methods("GET")
	api.HandleFunc("/config/drift-schedules", handler.CreateDriftSchedule).Methods("POST")
	api.HandleFunc("/config/drift-schedules/{id}", handler.GetDriftSchedule).Methods("GET")
	api.HandleFunc("/config/drift-schedules/{id}", handler.UpdateDriftSchedule).Methods("PUT")
	api.HandleFunc("/config/drift-schedules/{id}", handler.DeleteDriftSchedule).Methods("DELETE")
	api.HandleFunc("/config/drift-schedules/{id}/toggle", handler.ToggleDriftSchedule).Methods("POST")
	api.HandleFunc("/config/drift-schedules/{id}/runs", handler.GetDriftScheduleRuns).Methods("GET")
	
	// Discovery route
	api.HandleFunc("/discover", handler.DiscoverHandler).Methods("POST")
	
	// Provisioning routes
	api.HandleFunc("/provisioning/status", handler.GetProvisioningStatus).Methods("GET")
	api.HandleFunc("/provisioning/provision", handler.ProvisionDevices).Methods("POST")
	
	// DHCP routes
	api.HandleFunc("/dhcp/reservations", handler.GetDHCPReservations).Methods("GET")

	// Static file serving
	r.PathPrefix("/web/").Handler(http.StripPrefix("/web/", http.FileServer(http.Dir("./web/static/"))))
	r.PathPrefix("/").Handler(http.StripPrefix("/", http.FileServer(http.Dir("./web/static/"))))

	return r
}