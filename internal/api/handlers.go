package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/metrics"
	"github.com/ginsys/shelly-manager/internal/notification"
	"github.com/ginsys/shelly-manager/internal/service"
)

// Handler contains dependencies for API handlers
type Handler struct {
	DB                  *database.Manager
	Service             *service.ShellyService
	NotificationHandler *notification.Handler
	MetricsHandler      *metrics.Handler
	ConfigService       *configuration.Service
	logger              *logging.Logger
}

// NewHandler creates a new API handler
func NewHandler(db *database.Manager, svc *service.ShellyService, notificationHandler *notification.Handler) *Handler {
	return NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())
}

// NewHandlerWithLogger creates a new API handler with custom logger
func NewHandlerWithLogger(db *database.Manager, svc *service.ShellyService, notificationHandler *notification.Handler, metricsHandler *metrics.Handler, logger *logging.Logger) *Handler {
	// Create configuration service
	configService := configuration.NewService(db.DB, logger)

	return &Handler{
		DB:                  db,
		Service:             svc,
		NotificationHandler: notificationHandler,
		MetricsHandler:      metricsHandler,
		ConfigService:       configService,
		logger:              logger,
	}
}

// GetDevices handles GET /api/v1/devices
func (h *Handler) GetDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := h.DB.GetDevices()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(devices)
}

// AddDevice handles POST /api/v1/devices
func (h *Handler) AddDevice(w http.ResponseWriter, r *http.Request) {
	var device database.Device
	if err := json.NewDecoder(r.Body).Decode(&device); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate and normalize device settings
	if err := h.validateDeviceSettings(&device); err != nil {
		http.Error(w, fmt.Sprintf("Invalid device settings: %v", err), http.StatusBadRequest)
		return
	}

	if err := h.DB.AddDevice(&device); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(device)
}

// GetDevice handles GET /api/v1/devices/{id}
func (h *Handler) GetDevice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}

	device, err := h.DB.GetDevice(uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "Device not found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(device)
}

// UpdateDevice handles PUT /api/v1/devices/{id}
func (h *Handler) UpdateDevice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}

	// Check if device exists
	existingDevice, err := h.DB.GetDevice(uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "Device not found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Decode updated device data
	var updatedDevice database.Device
	if err := json.NewDecoder(r.Body).Decode(&updatedDevice); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate and normalize device settings
	if err := h.validateDeviceSettings(&updatedDevice); err != nil {
		http.Error(w, fmt.Sprintf("Invalid device settings: %v", err), http.StatusBadRequest)
		return
	}

	// Update existing device with new data
	updatedDevice.ID = existingDevice.ID
	if err := h.DB.UpdateDevice(&updatedDevice); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updatedDevice)
}

// DeleteDevice handles DELETE /api/v1/devices/{id}
func (h *Handler) DeleteDevice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}

	if err := h.DB.DeleteDevice(uint(id)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DiscoverHandler handles POST /api/v1/discover
func (h *Handler) DiscoverHandler(w http.ResponseWriter, r *http.Request) {
	// Parse optional network parameter
	var req struct {
		Network      string `json:"network"`
		ImportConfig bool   `json:"import_config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Continue with defaults if decode fails
		req = struct {
			Network      string `json:"network"`
			ImportConfig bool   `json:"import_config"`
		}{
			Network:      "auto",
			ImportConfig: true,
		}
	}

	// Default to auto-import config for new devices
	if !req.ImportConfig {
		req.ImportConfig = true
	}

	// Run discovery in background
	go func() {
		network := req.Network
		if network == "" {
			network = "auto"
		}

		h.logger.WithFields(map[string]any{
			"network":       network,
			"import_config": req.ImportConfig,
			"component":     "api",
		}).Info("Starting device discovery")

		// Discover devices
		devices, err := h.Service.DiscoverDevices(network)
		if err != nil {
			h.logger.WithFields(map[string]any{
				"error":     err.Error(),
				"component": "api",
			}).Error("Discovery failed")
			return
		}

		h.logger.WithFields(map[string]any{
			"devices_found": len(devices),
			"component":     "api",
		}).Info("Discovery completed")

		// Save discovered devices and import their configurations
		newDevices := 0
		configsImported := 0

		for _, device := range devices {
			// Check if device already exists by MAC
			existing, err := h.DB.GetDeviceByMAC(device.MAC)
			if err == nil && existing != nil {
				// Update existing device
				existing.IP = device.IP
				existing.Status = device.Status
				existing.LastSeen = device.LastSeen
				existing.Firmware = device.Firmware
				h.DB.UpdateDevice(existing)

				// Import config if requested
				if req.ImportConfig {
					if _, err := h.Service.ImportDeviceConfig(existing.ID); err == nil {
						configsImported++
					}
				}
			} else {
				// Add new device
				if err := h.DB.AddDevice(&device); err == nil {
					newDevices++

					// Import config for new device if requested
					if req.ImportConfig && device.ID > 0 {
						if _, err := h.Service.ImportDeviceConfig(device.ID); err == nil {
							configsImported++
						} else {
							h.logger.WithFields(map[string]any{
								"device_id": device.ID,
								"device_ip": device.IP,
								"error":     err.Error(),
								"component": "api",
							}).Warn("Failed to import config for new device")
						}
					}
				}
			}
		}

		h.logger.WithFields(map[string]any{
			"total_devices":    len(devices),
			"new_devices":      newDevices,
			"configs_imported": configsImported,
			"component":        "api",
		}).Info("Discovery processing completed")
	}()

	response := map[string]interface{}{
		"status":  "discovery_started",
		"message": "Device discovery has been initiated in background",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetProvisioningStatus handles GET /api/v1/provisioning/status
func (h *Handler) GetProvisioningStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"status":         "idle",
		"devices":        []string{},
		"last_run":       nil,
		"next_run":       nil,
		"auto_provision": false,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// ProvisionDevices handles POST /api/v1/provisioning/provision
func (h *Handler) ProvisionDevices(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":  "provisioning_started",
		"message": "Device provisioning has been initiated",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetDHCPReservations handles GET /api/v1/dhcp/reservations
func (h *Handler) GetDHCPReservations(w http.ResponseWriter, r *http.Request) {
	reservations := []map[string]interface{}{}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(reservations)
}

// ControlDevice handles POST /api/v1/devices/{id}/control
func (h *Handler) ControlDevice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}

	// Parse request body
	var req struct {
		Action string                 `json:"action"`
		Params map[string]interface{} `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate action
	if req.Action == "" {
		http.Error(w, "Action is required", http.StatusBadRequest)
		return
	}

	// Execute control command
	if err := h.Service.ControlDevice(uint(id), req.Action, req.Params); err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"action":    req.Action,
			"error":     err.Error(),
		}).Error("Device control failed")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status":    "success",
		"device_id": id,
		"action":    req.Action,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetDeviceStatus handles GET /api/v1/devices/{id}/status
func (h *Handler) GetDeviceStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}

	// Get device status
	status, err := h.Service.GetDeviceStatus(uint(id))
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to get device status")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// GetDeviceEnergy handles GET /api/v1/devices/{id}/energy
func (h *Handler) GetDeviceEnergy(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}

	// Get channel from query params (default to 0)
	channel := 0
	if ch := r.URL.Query().Get("channel"); ch != "" {
		if c, parseErr := strconv.Atoi(ch); parseErr == nil {
			channel = c
		}
	}

	// Get energy data
	energy, err := h.Service.GetDeviceEnergy(uint(id), channel)
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"channel":   channel,
			"error":     err.Error(),
		}).Error("Failed to get energy data")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(energy)
}

// GetDeviceConfig handles GET /api/v1/devices/{id}/config
func (h *Handler) GetDeviceConfig(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}

	// Get device configuration
	config, err := h.Service.GetDeviceConfig(uint(id))
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to get device config")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// ImportDeviceConfig handles POST /api/v1/devices/{id}/config/import
func (h *Handler) ImportDeviceConfig(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}

	// Import configuration from device
	config, err := h.Service.ImportDeviceConfig(uint(id))
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to import device config")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// GetImportStatus handles GET /api/v1/devices/{id}/config/status
func (h *Handler) GetImportStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}

	// Get import status for device
	status, err := h.Service.GetImportStatus(uint(id))
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to get import status")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// ExportDeviceConfig handles POST /api/v1/devices/{id}/config/export
func (h *Handler) ExportDeviceConfig(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}

	// Export configuration to device
	if err := h.Service.ExportDeviceConfig(uint(id)); err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to export device config")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status":    "success",
		"device_id": id,
		"message":   "Configuration exported to device",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// BulkImportConfigs handles POST /api/v1/config/bulk-import
func (h *Handler) BulkImportConfigs(w http.ResponseWriter, r *http.Request) {
	// Get all devices
	devices, err := h.Service.DB.GetDevices()
	if err != nil {
		h.logger.WithFields(map[string]any{
			"error": err.Error(),
		}).Error("Failed to get devices")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type ImportResult struct {
		DeviceID uint   `json:"device_id"`
		IP       string `json:"ip"`
		Status   string `json:"status"`
		Error    string `json:"error,omitempty"`
	}

	results := make([]ImportResult, 0, len(devices))
	successCount := 0
	errorCount := 0

	// Import configuration for each device
	for _, device := range devices {
		result := ImportResult{
			DeviceID: device.ID,
			IP:       device.IP,
		}

		// Attempt to import configuration
		config, err := h.Service.ImportDeviceConfig(device.ID)
		if err != nil {
			result.Status = "error"
			result.Error = err.Error()
			errorCount++
			h.logger.WithFields(map[string]any{
				"device_id": device.ID,
				"device_ip": device.IP,
				"error":     err.Error(),
			}).Warn("Failed to import device config during bulk import")
		} else {
			result.Status = "success"
			successCount++
			h.logger.WithFields(map[string]any{
				"device_id": device.ID,
				"device_ip": device.IP,
				"config_id": config.ID,
			}).Info("Successfully imported device config during bulk import")
		}

		results = append(results, result)
	}

	response := map[string]interface{}{
		"total":   len(devices),
		"success": successCount,
		"errors":  errorCount,
		"results": results,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// BulkExportConfigs handles POST /api/v1/config/bulk-export
func (h *Handler) BulkExportConfigs(w http.ResponseWriter, r *http.Request) {
	// Get all devices
	devices, err := h.Service.DB.GetDevices()
	if err != nil {
		h.logger.WithFields(map[string]any{
			"error": err.Error(),
		}).Error("Failed to get devices")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type ExportResult struct {
		DeviceID uint   `json:"device_id"`
		IP       string `json:"ip"`
		Status   string `json:"status"`
		Error    string `json:"error,omitempty"`
	}

	results := make([]ExportResult, 0, len(devices))
	successCount := 0
	errorCount := 0

	// Export configuration to each device
	for _, device := range devices {
		result := ExportResult{
			DeviceID: device.ID,
			IP:       device.IP,
		}

		// Attempt to export configuration
		err := h.Service.ExportDeviceConfig(device.ID)
		if err != nil {
			result.Status = "error"
			result.Error = err.Error()
			errorCount++
			h.logger.WithFields(map[string]any{
				"device_id": device.ID,
				"device_ip": device.IP,
				"error":     err.Error(),
			}).Warn("Failed to export device config during bulk export")
		} else {
			result.Status = "success"
			successCount++
			h.logger.WithFields(map[string]any{
				"device_id": device.ID,
				"device_ip": device.IP,
			}).Info("Successfully exported device config during bulk export")
		}

		results = append(results, result)
	}

	response := map[string]interface{}{
		"total":   len(devices),
		"success": successCount,
		"errors":  errorCount,
		"results": results,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DetectConfigDrift handles GET /api/v1/devices/{id}/config/drift
func (h *Handler) DetectConfigDrift(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}

	// Detect configuration drift
	drift, err := h.Service.DetectConfigDrift(uint(id))
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to detect config drift")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if drift == nil {
		response := map[string]interface{}{
			"device_id": id,
			"drift":     false,
			"message":   "No configuration drift detected",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(drift)
}

// BulkDetectConfigDrift handles POST /api/v1/config/bulk-drift-detect
func (h *Handler) BulkDetectConfigDrift(w http.ResponseWriter, r *http.Request) {
	// Perform bulk drift detection across all devices
	result, err := h.Service.BulkDetectConfigDrift()
	if err != nil {
		h.logger.WithFields(map[string]any{
			"error": err.Error(),
		}).Error("Failed to perform bulk drift detection")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// GetConfigTemplates handles GET /api/v1/config/templates
func (h *Handler) GetConfigTemplates(w http.ResponseWriter, r *http.Request) {
	templates, err := h.Service.ConfigSvc.GetTemplates()
	if err != nil {
		h.logger.WithFields(map[string]any{
			"error": err.Error(),
		}).Error("Failed to get config templates")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(templates)
}

// CreateConfigTemplate handles POST /api/v1/config/templates
func (h *Handler) CreateConfigTemplate(w http.ResponseWriter, r *http.Request) {
	var template configuration.ConfigTemplate
	if err := json.NewDecoder(r.Body).Decode(&template); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if err := h.Service.ConfigSvc.CreateTemplate(&template); err != nil {
		h.logger.WithFields(map[string]any{
			"error": err.Error(),
		}).Error("Failed to create config template")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(template)
}

// UpdateConfigTemplate handles PUT /api/v1/config/templates/{id}
func (h *Handler) UpdateConfigTemplate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid template ID", http.StatusBadRequest)
		return
	}

	var template configuration.ConfigTemplate
	if err := json.NewDecoder(r.Body).Decode(&template); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	template.ID = uint(id)
	if err := h.Service.ConfigSvc.UpdateTemplate(&template); err != nil {
		h.logger.WithFields(map[string]any{
			"template_id": id,
			"error":       err.Error(),
		}).Error("Failed to update config template")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(template)
}

// DeleteConfigTemplate handles DELETE /api/v1/config/templates/{id}
func (h *Handler) DeleteConfigTemplate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid template ID", http.StatusBadRequest)
		return
	}

	if err := h.Service.ConfigSvc.DeleteTemplate(uint(id)); err != nil {
		h.logger.WithFields(map[string]any{
			"template_id": id,
			"error":       err.Error(),
		}).Error("Failed to delete config template")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ApplyConfigTemplate handles POST /api/v1/devices/{id}/config/apply-template
func (h *Handler) ApplyConfigTemplate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}

	var req struct {
		TemplateID uint                   `json:"template_id"`
		Variables  map[string]interface{} `json:"variables"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if err := h.Service.ApplyConfigTemplate(uint(id), req.TemplateID, req.Variables); err != nil {
		h.logger.WithFields(map[string]any{
			"device_id":   id,
			"template_id": req.TemplateID,
			"error":       err.Error(),
		}).Error("Failed to apply config template")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status":      "success",
		"device_id":   id,
		"template_id": req.TemplateID,
		"message":     "Template applied to device",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetConfigHistory handles GET /api/v1/devices/{id}/config/history
func (h *Handler) GetConfigHistory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}

	// Get limit from query params (default to 50)
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if lim, parseErr := strconv.Atoi(l); parseErr == nil && lim > 0 {
			limit = lim
		}
	}

	history, err := h.Service.ConfigSvc.GetConfigHistory(uint(id), limit)
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to get config history")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}

// UpdateDeviceConfig handles PUT /api/v1/devices/{id}/config
func (h *Handler) UpdateDeviceConfig(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}

	// Parse request body
	var configUpdate map[string]interface{}
	if decodeErr := json.NewDecoder(r.Body).Decode(&configUpdate); decodeErr != nil {
		http.Error(w, "Invalid JSON in request body", http.StatusBadRequest)
		return
	}

	// Update device configuration
	err = h.Service.UpdateDeviceConfig(uint(id), configUpdate)
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to update device config")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return updated config
	config, err := h.Service.GetDeviceConfig(uint(id))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// UpdateRelayConfig handles PUT /api/v1/devices/{id}/config/relay
func (h *Handler) UpdateRelayConfig(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}

	// Parse relay configuration
	var relayConfig configuration.RelayConfig
	if decodeErr := json.NewDecoder(r.Body).Decode(&relayConfig); decodeErr != nil {
		http.Error(w, "Invalid relay configuration JSON", http.StatusBadRequest)
		return
	}

	// Update relay configuration
	err = h.Service.UpdateRelayConfig(uint(id), &relayConfig)
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to update relay config")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// UpdateDimmingConfig handles PUT /api/v1/devices/{id}/config/dimming
func (h *Handler) UpdateDimmingConfig(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}

	// Parse dimming configuration
	var dimmingConfig configuration.DimmingConfig
	if decodeErr := json.NewDecoder(r.Body).Decode(&dimmingConfig); decodeErr != nil {
		http.Error(w, "Invalid dimming configuration JSON", http.StatusBadRequest)
		return
	}

	// Update dimming configuration
	err = h.Service.UpdateDimmingConfig(uint(id), &dimmingConfig)
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to update dimming config")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// UpdateRollerConfig handles PUT /api/v1/devices/{id}/config/roller
func (h *Handler) UpdateRollerConfig(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}

	// Parse roller configuration
	var rollerConfig configuration.RollerConfig
	if decodeErr := json.NewDecoder(r.Body).Decode(&rollerConfig); decodeErr != nil {
		http.Error(w, "Invalid roller configuration JSON", http.StatusBadRequest)
		return
	}

	// Update roller configuration
	err = h.Service.UpdateRollerConfig(uint(id), &rollerConfig)
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to update roller config")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// UpdatePowerMeteringConfig handles PUT /api/v1/devices/{id}/config/power-metering
func (h *Handler) UpdatePowerMeteringConfig(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}

	// Parse power metering configuration
	var powerConfig configuration.PowerMeteringConfig
	if decodeErr := json.NewDecoder(r.Body).Decode(&powerConfig); decodeErr != nil {
		http.Error(w, "Invalid power metering configuration JSON", http.StatusBadRequest)
		return
	}

	// Update power metering configuration
	err = h.Service.UpdatePowerMeteringConfig(uint(id), &powerConfig)
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to update power metering config")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// UpdateDeviceAuth handles PUT /api/v1/devices/{id}/config/auth
func (h *Handler) UpdateDeviceAuth(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}

	// Parse authentication configuration
	var authConfig struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if decodeErr := json.NewDecoder(r.Body).Decode(&authConfig); decodeErr != nil {
		http.Error(w, "Invalid auth configuration JSON", http.StatusBadRequest)
		return
	}

	// Update device authentication
	err = h.Service.UpdateDeviceAuth(uint(id), authConfig.Username, authConfig.Password)
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to update device auth")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// GetDriftSchedules handles GET /api/v1/config/drift-schedules
func (h *Handler) GetDriftSchedules(w http.ResponseWriter, r *http.Request) {
	schedules, err := h.Service.GetDriftSchedules()
	if err != nil {
		h.logger.WithFields(map[string]any{
			"error": err.Error(),
		}).Error("Failed to get drift schedules")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(schedules)
}

// CreateDriftSchedule handles POST /api/v1/config/drift-schedules
func (h *Handler) CreateDriftSchedule(w http.ResponseWriter, r *http.Request) {
	var schedule configuration.DriftDetectionSchedule
	if err := json.NewDecoder(r.Body).Decode(&schedule); err != nil {
		http.Error(w, "Invalid schedule JSON", http.StatusBadRequest)
		return
	}

	created, err := h.Service.CreateDriftSchedule(schedule)
	if err != nil {
		h.logger.WithFields(map[string]any{
			"schedule_name": schedule.Name,
			"error":         err.Error(),
		}).Error("Failed to create drift schedule")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(created)
}

// GetDriftSchedule handles GET /api/v1/config/drift-schedules/{id}
func (h *Handler) GetDriftSchedule(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid schedule ID", http.StatusBadRequest)
		return
	}

	schedule, err := h.Service.GetDriftSchedule(uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "Schedule not found", http.StatusNotFound)
			return
		}
		h.logger.WithFields(map[string]any{
			"schedule_id": id,
			"error":       err.Error(),
		}).Error("Failed to get drift schedule")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(schedule)
}

// UpdateDriftSchedule handles PUT /api/v1/config/drift-schedules/{id}
func (h *Handler) UpdateDriftSchedule(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid schedule ID", http.StatusBadRequest)
		return
	}

	var updates configuration.DriftDetectionSchedule
	if decodeErr := json.NewDecoder(r.Body).Decode(&updates); decodeErr != nil {
		http.Error(w, "Invalid schedule JSON", http.StatusBadRequest)
		return
	}

	updated, err := h.Service.UpdateDriftSchedule(uint(id), updates)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "Schedule not found", http.StatusNotFound)
			return
		}
		h.logger.WithFields(map[string]any{
			"schedule_id": id,
			"error":       err.Error(),
		}).Error("Failed to update drift schedule")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}

// DeleteDriftSchedule handles DELETE /api/v1/config/drift-schedules/{id}
func (h *Handler) DeleteDriftSchedule(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid schedule ID", http.StatusBadRequest)
		return
	}

	err = h.Service.DeleteDriftSchedule(uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "Schedule not found", http.StatusNotFound)
			return
		}
		h.logger.WithFields(map[string]any{
			"schedule_id": id,
			"error":       err.Error(),
		}).Error("Failed to delete drift schedule")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// ToggleDriftSchedule handles POST /api/v1/config/drift-schedules/{id}/toggle
func (h *Handler) ToggleDriftSchedule(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid schedule ID", http.StatusBadRequest)
		return
	}

	updated, err := h.Service.ToggleDriftSchedule(uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "Schedule not found", http.StatusNotFound)
			return
		}
		h.logger.WithFields(map[string]any{
			"schedule_id": id,
			"error":       err.Error(),
		}).Error("Failed to toggle drift schedule")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}

// GetDriftScheduleRuns handles GET /api/v1/config/drift-schedules/{id}/runs
func (h *Handler) GetDriftScheduleRuns(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid schedule ID", http.StatusBadRequest)
		return
	}

	// Parse optional limit parameter
	limit := 50 // Default limit
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsedLimit, parseErr := strconv.Atoi(limitStr); parseErr == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	runs, err := h.Service.GetDriftScheduleRuns(uint(id), limit)
	if err != nil {
		h.logger.WithFields(map[string]any{
			"schedule_id": id,
			"error":       err.Error(),
		}).Error("Failed to get drift schedule runs")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(runs)
}

// GetDriftReports handles GET /api/v1/config/drift-reports
func (h *Handler) GetDriftReports(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	reportType := r.URL.Query().Get("type")
	deviceIDStr := r.URL.Query().Get("device_id")
	limitStr := r.URL.Query().Get("limit")

	var deviceID *uint
	if deviceIDStr != "" {
		if id, err := strconv.ParseUint(deviceIDStr, 10, 32); err == nil {
			deviceIDUint := uint(id)
			deviceID = &deviceIDUint
		}
	}

	limit := 50 // Default limit
	if limitStr != "" {
		if parsedLimit, parseErr := strconv.Atoi(limitStr); parseErr == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	reports, err := h.Service.GetDriftReports(reportType, deviceID, limit)
	if err != nil {
		h.logger.WithFields(map[string]any{
			"report_type": reportType,
			"device_id":   deviceID,
			"error":       err.Error(),
		}).Error("Failed to get drift reports")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(reports)
}

// GenerateDeviceDriftReport handles POST /api/v1/devices/{id}/drift-report
func (h *Handler) GenerateDeviceDriftReport(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid device ID", http.StatusBadRequest)
		return
	}

	report, err := h.Service.GenerateDeviceDriftReport(uint(id))
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to generate device drift report")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(report)
}

// GetDriftTrends handles GET /api/v1/config/drift-trends
func (h *Handler) GetDriftTrends(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	deviceIDStr := r.URL.Query().Get("device_id")
	resolvedStr := r.URL.Query().Get("resolved")
	limitStr := r.URL.Query().Get("limit")

	var deviceID *uint
	if deviceIDStr != "" {
		if id, err := strconv.ParseUint(deviceIDStr, 10, 32); err == nil {
			deviceIDUint := uint(id)
			deviceID = &deviceIDUint
		}
	}

	var resolved *bool
	if resolvedStr != "" {
		if resolvedBool, err := strconv.ParseBool(resolvedStr); err == nil {
			resolved = &resolvedBool
		}
	}

	limit := 100 // Default limit
	if limitStr != "" {
		if parsedLimit, parseErr := strconv.Atoi(limitStr); parseErr == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	trends, err := h.Service.GetDriftTrends(deviceID, resolved, limit)
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": deviceID,
			"resolved":  resolved,
			"error":     err.Error(),
		}).Error("Failed to get drift trends")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(trends)
}

// MarkTrendResolved handles POST /api/v1/config/drift-trends/{id}/resolve
func (h *Handler) MarkTrendResolved(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid trend ID", http.StatusBadRequest)
		return
	}

	err = h.Service.MarkTrendResolved(uint(id))
	if err != nil {
		h.logger.WithFields(map[string]any{
			"trend_id": id,
			"error":    err.Error(),
		}).Error("Failed to mark trend as resolved")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "resolved"})
}

// EnhancedBulkDetectConfigDrift handles POST /api/v1/config/bulk-drift-detect-enhanced
func (h *Handler) EnhancedBulkDetectConfigDrift(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("Starting enhanced bulk drift detection with comprehensive reporting")

	result, err := h.Service.BulkDetectConfigDrift()
	if err != nil {
		h.logger.WithFields(map[string]any{
			"error": err.Error(),
		}).Error("Failed to perform bulk drift detection")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Generate comprehensive report
	report, err := h.Service.EnhanceBulkDriftResult(result, nil)
	if err != nil {
		h.logger.WithFields(map[string]any{
			"error": err.Error(),
		}).Warn("Failed to generate comprehensive report, returning basic result")

		// Fall back to basic result if reporting fails
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
		return
	}

	h.logger.WithFields(map[string]any{
		"report_id":       report.ID,
		"devices_drifted": result.Drifted,
		"critical_issues": report.Summary.CriticalIssues,
		"recommendations": len(report.Recommendations),
	}).Info("Enhanced bulk drift detection completed")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(report)
}

// PreviewTemplate handles POST /api/v1/configuration/preview-template
func (h *Handler) PreviewTemplate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Template  string                 `json:"template"`
		Variables map[string]interface{} `json:"variables"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.WithFields(map[string]any{
			"error": err.Error(),
		}).Warn("Failed to decode template preview request")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	// Process custom template
	result := h.ConfigService.SubstituteVariables(json.RawMessage(req.Template), req.Variables)

	h.logger.WithFields(map[string]any{
		"template_size": len(req.Template),
		"result_size":   len(result),
	}).Info("Template preview completed successfully")

	// Parse result to return as JSON object
	var resultObj interface{}
	if err := json.Unmarshal(result, &resultObj); err != nil {
		// If parsing fails, return as raw string
		resultObj = string(result)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"result": resultObj,
	})
}

// ValidateTemplate handles POST /api/v1/configuration/validate-template
func (h *Handler) ValidateTemplate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Template string `json:"template"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.WithFields(map[string]any{
			"error": err.Error(),
		}).Warn("Failed to decode template validation request")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"valid": false,
			"error": "Invalid request body",
		})
		return
	}

	// Create a template engine to validate
	templateEngine := configuration.NewTemplateEngine(h.logger)
	err := templateEngine.ValidateTemplate(req.Template)

	if err != nil {
		h.logger.WithFields(map[string]any{
			"error": err.Error(),
		}).Info("Template validation failed")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"valid": false,
			"error": err.Error(),
		})
		return
	}

	h.logger.Info("Template validation succeeded")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"valid": true,
	})
}

// SaveTemplate handles POST /api/v1/configuration/templates
func (h *Handler) SaveTemplate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string                 `json:"name"`
		Template  string                 `json:"template"`
		Variables map[string]interface{} `json:"variables"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.WithFields(map[string]any{
			"error": err.Error(),
		}).Warn("Failed to decode save template request")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	if req.Name == "" || req.Template == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Template name and content are required",
		})
		return
	}

	// Validate template before saving
	templateEngine := configuration.NewTemplateEngine(h.logger)
	if err := templateEngine.ValidateTemplate(req.Template); err != nil {
		h.logger.WithFields(map[string]any{
			"name":  req.Name,
			"error": err.Error(),
		}).Warn("Cannot save invalid template")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Template validation failed: " + err.Error(),
		})
		return
	}

	// Convert variables to JSON
	variablesJSON, err := json.Marshal(req.Variables)
	if err != nil {
		h.logger.WithFields(map[string]any{
			"error": err.Error(),
		}).Warn("Failed to marshal template variables")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Failed to process variables",
		})
		return
	}

	// Create template record
	template := &configuration.ConfigTemplate{
		Name:        req.Name,
		Description: "User-created template via web interface",
		DeviceType:  "all",
		Generation:  0, // 0 means applies to all generations
		Config:      json.RawMessage(req.Template),
		Variables:   json.RawMessage(variablesJSON),
		IsDefault:   false,
	}

	// Save to database using the config service
	if err := h.ConfigService.SaveTemplate(template); err != nil {
		h.logger.WithFields(map[string]any{
			"name":  req.Name,
			"error": err.Error(),
		}).Error("Failed to save template to database")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Failed to save template",
		})
		return
	}

	h.logger.WithFields(map[string]any{
		"name":        req.Name,
		"template_id": template.ID,
	}).Info("Template saved successfully")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"id":      template.ID,
	})
}

// GetTemplateExamples handles GET /api/v1/configuration/template-examples
func (h *Handler) GetTemplateExamples(w http.ResponseWriter, r *http.Request) {
	examples := configuration.GetTemplateExamples()
	documentation := configuration.GetTemplateDocumentation()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"examples":      examples,
		"documentation": documentation,
	})
}

// validateDeviceSettings ensures device settings are valid JSON or sets defaults
func (h *Handler) validateDeviceSettings(device *database.Device) error {
	// If settings are empty, provide minimal valid JSON
	if device.Settings == "" {
		device.Settings = `{"model":"Unknown","gen":1,"auth_enabled":false}`
		return nil
	}

	// Validate that settings is valid JSON
	var settings map[string]interface{}
	if err := json.Unmarshal([]byte(device.Settings), &settings); err != nil {
		return fmt.Errorf("settings must be valid JSON: %w", err)
	}

	// Ensure minimum required fields exist
	if _, exists := settings["model"]; !exists {
		settings["model"] = "Unknown"
	}
	if _, exists := settings["gen"]; !exists {
		settings["gen"] = 1
	}
	if _, exists := settings["auth_enabled"]; !exists {
		settings["auth_enabled"] = false
	}

	// Re-serialize the normalized settings
	normalizedSettings, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("failed to normalize settings: %w", err)
	}
	device.Settings = string(normalizedSettings)

	return nil
}
