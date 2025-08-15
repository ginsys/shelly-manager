package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/service"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// Handler contains dependencies for API handlers
type Handler struct {
	DB      *database.Manager
	Service *service.ShellyService
	logger  *logging.Logger
}

// NewHandler creates a new API handler
func NewHandler(db *database.Manager, svc *service.ShellyService) *Handler {
	return NewHandlerWithLogger(db, svc, logging.GetDefault())
}

// NewHandlerWithLogger creates a new API handler with custom logger
func NewHandlerWithLogger(db *database.Manager, svc *service.ShellyService, logger *logging.Logger) *Handler {
	return &Handler{
		DB:      db,
		Service: svc,
		logger:  logger,
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
		Network string `json:"network"`
		ImportConfig bool `json:"import_config"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	
	// Default to auto-import config for new devices
	if req.ImportConfig == false {
		req.ImportConfig = true
	}
	
	// Run discovery in background
	go func() {
		network := req.Network
		if network == "" {
			network = "auto"
		}
		
		h.logger.WithFields(map[string]any{
			"network": network,
			"import_config": req.ImportConfig,
			"component": "api",
		}).Info("Starting device discovery")
		
		// Discover devices
		devices, err := h.Service.DiscoverDevices(network)
		if err != nil {
			h.logger.WithFields(map[string]any{
				"error": err.Error(),
				"component": "api",
			}).Error("Discovery failed")
			return
		}
		
		h.logger.WithFields(map[string]any{
			"devices_found": len(devices),
			"component": "api",
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
								"error": err.Error(),
								"component": "api",
							}).Warn("Failed to import config for new device")
						}
					}
				}
			}
		}
		
		h.logger.WithFields(map[string]any{
			"total_devices": len(devices),
			"new_devices": newDevices,
			"configs_imported": configsImported,
			"component": "api",
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
		"status":      "idle",
		"devices":     []string{},
		"last_run":    nil,
		"next_run":    nil,
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
		if c, err := strconv.Atoi(ch); err == nil {
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
		"total":    len(devices),
		"success":  successCount,
		"errors":   errorCount,
		"results":  results,
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
		if lim, err := strconv.Atoi(l); err == nil && lim > 0 {
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
	if err := json.NewDecoder(r.Body).Decode(&configUpdate); err != nil {
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
	if err := json.NewDecoder(r.Body).Decode(&relayConfig); err != nil {
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
	if err := json.NewDecoder(r.Body).Decode(&dimmingConfig); err != nil {
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
	if err := json.NewDecoder(r.Body).Decode(&rollerConfig); err != nil {
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
	if err := json.NewDecoder(r.Body).Decode(&powerConfig); err != nil {
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
	if err := json.NewDecoder(r.Body).Decode(&authConfig); err != nil {
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