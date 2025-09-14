package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"gorm.io/gorm"

	apiresp "github.com/ginsys/shelly-manager/internal/api/response"
	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/metrics"
	"github.com/ginsys/shelly-manager/internal/notification"
	"github.com/ginsys/shelly-manager/internal/service"
)

// Handler contains dependencies for API handlers
type Handler struct {
	DB                  database.DatabaseInterface
	Service             *service.ShellyService
	NotificationHandler *notification.Handler
	MetricsHandler      *metrics.Handler
	ConfigService       *configuration.Service
	ExportHandlers      *ExportHandlers
	ImportHandlers      *ImportHandlers
	logger              *logging.Logger
	securityMonitor     interface{} // Security monitor for metrics (using interface{} to avoid circular imports)
	// AdminAPIKey provides simple guard for sensitive endpoints until full auth is implemented
	AdminAPIKey string
}

// NewHandler creates a new API handler
func NewHandler(db database.DatabaseInterface, svc *service.ShellyService, notificationHandler *notification.Handler) *Handler {
	return NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())
}

// NewHandlerWithLogger creates a new API handler with custom logger
func NewHandlerWithLogger(db database.DatabaseInterface, svc *service.ShellyService, notificationHandler *notification.Handler, metricsHandler *metrics.Handler, logger *logging.Logger) *Handler {
	// Create configuration service
	configService := configuration.NewService(db.GetDB(), logger)

	return &Handler{
		DB:                  db,
		Service:             svc,
		NotificationHandler: notificationHandler,
		MetricsHandler:      metricsHandler,
		ConfigService:       configService,
		logger:              logger,
	}
}

// SetAdminAPIKey sets the in-memory admin key for guarding sensitive operations.
func (h *Handler) SetAdminAPIKey(key string) { h.AdminAPIKey = key }

// requireAdmin checks Authorization or X-API-Key against AdminAPIKey.
func (h *Handler) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if h.AdminAPIKey == "" {
		return true
	}
	auth := r.Header.Get("Authorization")
	xKey := r.Header.Get("X-API-Key")
	ok := false
	if strings.HasPrefix(auth, "Bearer ") && strings.TrimPrefix(auth, "Bearer ") == h.AdminAPIKey {
		ok = true
	}
	if !ok && xKey != "" && xKey == h.AdminAPIKey {
		ok = true
	}
	if !ok {
		h.responseWriter().WriteError(w, r, http.StatusUnauthorized, apiresp.ErrCodeUnauthorized, "Admin authorization required", nil)
		return false
	}
	return true
}

// RotateAdminKey updates the in-memory admin key used by API/WS/export/import handlers.
// Body: {"new_key": "..."}
func (h *Handler) RotateAdminKey(w http.ResponseWriter, r *http.Request) {
	// Require current admin privileges
	if !h.requireAdmin(w, r) {
		return
	}
	// Parse new key
	var body struct {
		NewKey string `json:"new_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.responseWriter().WriteValidationError(w, r, "Invalid JSON request body")
		return
	}
	if strings.TrimSpace(body.NewKey) == "" {
		h.responseWriter().WriteValidationError(w, r, "new_key is required")
		return
	}

	// Update admin key across components
	h.AdminAPIKey = body.NewKey
	if h.ExportHandlers != nil {
		h.ExportHandlers.SetAdminAPIKey(body.NewKey)
	}
	if h.ImportHandlers != nil {
		h.ImportHandlers.SetAdminAPIKey(body.NewKey)
	}
	if h.MetricsHandler != nil {
		h.MetricsHandler.SetAdminAPIKey(body.NewKey)
	}

	if h.logger != nil {
		h.logger.WithFields(map[string]any{
			"component":  "admin",
			"action":     "rotate_admin_key",
			"request_id": r.Context().Value("request_id"),
		}).Info("Admin API key rotated")
	}

	h.responseWriter().WriteSuccess(w, r, map[string]any{"rotated": true})
}

// writeJSON writes a JSON response and logs any encoding errors
func (h *Handler) writeJSON(w http.ResponseWriter, data interface{}) {
	if err := json.NewEncoder(w).Encode(data); err != nil && h.logger != nil {
		h.logger.Error("Failed to encode JSON response", "error", err)
	}
}

// responseWriter returns a standardized API response writer
func (h *Handler) responseWriter() *apiresp.ResponseWriter {
	return apiresp.NewResponseWriter(h.logger)
}

// Healthz returns basic liveness: process is up and DB reachable.
func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	type health struct {
		Status string `json:"status"`
	}
	// Basic DB check via lightweight query
	one := 0
	dbErr := h.DB.GetDB().Raw("SELECT 1").Scan(&one).Error
	status := "ok"
	if dbErr != nil || one != 1 {
		status = "degraded"
	}
	resp := health{Status: status}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// FastHealthz - Optimized health endpoint for test mode
func (h *Handler) FastHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)

	// Minimal JSON response for speed
	response := fmt.Sprintf(`{"status":"ok","timestamp":"%s","mode":"test"}`,
		time.Now().Format(time.RFC3339))
	if _, err := w.Write([]byte(response)); err != nil {
		// Log error but don't change response status as headers already sent
		h.logger.Error("Failed to write FastHealthz response", "error", err)
	}
}

// Readyz returns readiness: dependencies available (currently DB).
func (h *Handler) Readyz(w http.ResponseWriter, r *http.Request) {
	// Fail if DB not reachable
	var one int
	if err := h.DB.GetDB().Raw("SELECT 1").Scan(&one).Error; err != nil || one != 1 {
		h.responseWriter().WriteError(w, r, http.StatusServiceUnavailable, apiresp.ErrCodeServiceUnavailable, "Dependency not ready (database)", nil)
		return
	}
	h.responseWriter().WriteSuccess(w, r, map[string]any{"ready": true})
}

// GetDevices handles GET /api/v1/devices
func (h *Handler) GetDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := h.DB.GetDevices()
	if err != nil {
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}

	// Pagination params (optional). If page_size not provided, return all items as single page.
	total := len(devices)
	pageSize := apiresp.GetQueryParamInt(r, "page_size", 0)
	page := apiresp.GetQueryParamInt(r, "page", 1)
	start, end := 0, total
	if pageSize > 0 {
		if page < 1 {
			page = 1
		}
		start = (page - 1) * pageSize
		if start > total {
			start = total
		}
		end = start + pageSize
		if end > total {
			end = total
		}
	} else {
		// Single-page default
		page = 1
		pageSize = total
	}

	pageDevices := devices
	if start != 0 || end != total {
		pageDevices = devices[start:end]
	}

	// Build pagination meta
	totalPages := 1
	if pageSize > 0 {
		if total%pageSize == 0 {
			totalPages = total / pageSize
		} else {
			totalPages = (total / pageSize) + 1
		}
		if total == 0 {
			totalPages = 1
		}
	}
	meta := &apiresp.Metadata{
		Page: &apiresp.PaginationMeta{
			Page:       page,
			PageSize:   pageSize,
			TotalPages: totalPages,
			HasNext:    page < totalPages,
			HasPrev:    page > 1,
		},
		Count:      intPtr(len(pageDevices)),
		TotalCount: intPtr(total),
	}

	h.responseWriter().WriteSuccessWithMeta(w, r, map[string]interface{}{"devices": pageDevices}, meta)
}

// AddDevice handles POST /api/v1/devices
func (h *Handler) AddDevice(w http.ResponseWriter, r *http.Request) {
	var device database.Device
	if err := json.NewDecoder(r.Body).Decode(&device); err != nil {
		h.responseWriter().WriteValidationError(w, r, "Invalid JSON request body")
		return
	}

	// Validate and normalize device settings
	if err := h.validateDeviceSettings(&device); err != nil {
		h.responseWriter().WriteValidationError(w, r, fmt.Sprintf("Invalid device settings: %v", err))
		return
	}

	if err := h.DB.AddDevice(&device); err != nil {
		// Enhanced error logging for debugging test issues
		h.logger.WithFields(map[string]any{
			"error":       err.Error(),
			"device_ip":   device.IP,
			"device_mac":  device.MAC,
			"device_type": device.Type,
			"settings":    device.Settings,
			"component":   "api",
			"operation":   "add_device",
			"request_id":  r.Context().Value("request_id"),
		}).Error("AddDevice operation failed with detailed context")

		// Check if it's a unique constraint violation and return appropriate error
		if strings.Contains(strings.ToLower(err.Error()), "unique constraint") ||
			strings.Contains(strings.ToLower(err.Error()), "constraint failed") {
			if strings.Contains(strings.ToLower(err.Error()), "ip") {
				h.responseWriter().WriteError(w, r, http.StatusConflict, apiresp.ErrCodeConflict, "Device with this IP address already exists", nil)
			} else if strings.Contains(strings.ToLower(err.Error()), "mac") {
				h.responseWriter().WriteError(w, r, http.StatusConflict, apiresp.ErrCodeConflict, "Device with this MAC address already exists", nil)
			} else {
				h.responseWriter().WriteError(w, r, http.StatusConflict, apiresp.ErrCodeConflict, "Device already exists", nil)
			}
			return
		}

		h.responseWriter().WriteInternalError(w, r, err)
		return
	}

	h.responseWriter().WriteCreated(w, r, device)
}

// GetDevice handles GET /api/v1/devices/{id}
func (h *Handler) GetDevice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}

	device, err := h.DB.GetDevice(uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			h.responseWriter().WriteNotFoundError(w, r, "Device")
		} else {
			h.responseWriter().WriteInternalError(w, r, err)
		}
		return
	}

	h.responseWriter().WriteSuccess(w, r, device)
}

// UpdateDevice handles PUT /api/v1/devices/{id}
func (h *Handler) UpdateDevice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}

	// Check if device exists
	existingDevice, err := h.DB.GetDevice(uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			h.responseWriter().WriteNotFoundError(w, r, "Device")
		} else {
			h.responseWriter().WriteInternalError(w, r, err)
		}
		return
	}

	// Decode updated device data
	var updatedDevice database.Device
	if err := json.NewDecoder(r.Body).Decode(&updatedDevice); err != nil {
		h.responseWriter().WriteValidationError(w, r, "Invalid JSON request body")
		return
	}

	// Validate and normalize device settings
	if err := h.validateDeviceSettings(&updatedDevice); err != nil {
		h.responseWriter().WriteValidationError(w, r, fmt.Sprintf("Invalid device settings: %v", err))
		return
	}

	// Update existing device with new data
	updatedDevice.ID = existingDevice.ID
	if err := h.DB.UpdateDevice(&updatedDevice); err != nil {
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}

	h.responseWriter().WriteSuccess(w, r, updatedDevice)
}

// DeleteDevice handles DELETE /api/v1/devices/{id}
func (h *Handler) DeleteDevice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}

	if err := h.DB.DeleteDevice(uint(id)); err != nil {
		// If device doesn't exist, still return 204 (idempotent delete)
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			h.responseWriter().WriteInternalError(w, r, err)
			return
		}
	}

	h.responseWriter().WriteNoContent(w, r)
}

func intPtr(i int) *int { return &i }

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
				if err := h.DB.UpdateDevice(existing); err != nil && h.logger != nil {
					h.logger.Error("Failed to update device during import", "error", err, "deviceID", existing.ID)
				}

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

	h.responseWriter().WriteSuccess(w, r, map[string]interface{}{
		"status":  "discovery_started",
		"message": "Device discovery has been initiated in background",
	})
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
	h.writeJSON(w, status)
}

// ProvisionDevices handles POST /api/v1/provisioning/provision
func (h *Handler) ProvisionDevices(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":  "provisioning_started",
		"message": "Device provisioning has been initiated",
	}

	w.Header().Set("Content-Type", "application/json")
	h.writeJSON(w, response)
}

// GetDHCPReservations handles GET /api/v1/dhcp/reservations
func (h *Handler) GetDHCPReservations(w http.ResponseWriter, r *http.Request) {
	reservations := []map[string]interface{}{}

	w.Header().Set("Content-Type", "application/json")
	h.writeJSON(w, reservations)
}

// ControlDevice handles POST /api/v1/devices/{id}/control
func (h *Handler) ControlDevice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}

	// Parse request body
	var req struct {
		Action string                 `json:"action"`
		Params map[string]interface{} `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.responseWriter().WriteValidationError(w, r, "Invalid JSON request body")
		return
	}

	// Validate action
	if req.Action == "" {
		h.responseWriter().WriteValidationError(w, r, "Action is required")
		return
	}

	// Execute control command
	if err := h.Service.ControlDevice(uint(id), req.Action, req.Params); err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"action":    req.Action,
			"error":     err.Error(),
		}).Error("Device control failed")
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}

	h.responseWriter().WriteSuccess(w, r, map[string]interface{}{
		"status":    "success",
		"device_id": id,
		"action":    req.Action,
	})
}

// GetDeviceStatus handles GET /api/v1/devices/{id}/status
func (h *Handler) GetDeviceStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}

	// Get device status
	status, err := h.Service.GetDeviceStatus(uint(id))
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to get device status")
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}

	h.responseWriter().WriteSuccess(w, r, status)
}

// GetDeviceEnergy handles GET /api/v1/devices/{id}/energy
func (h *Handler) GetDeviceEnergy(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
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
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}

	h.responseWriter().WriteSuccess(w, r, energy)
}

// GetDeviceConfig handles GET /api/v1/devices/{id}/config
func (h *Handler) GetDeviceConfig(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}

	// Get device configuration
	config, err := h.Service.GetDeviceConfig(uint(id))
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to get device config")
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}

	h.responseWriter().WriteSuccess(w, r, config)
}

// GetCurrentDeviceConfig handles GET /api/v1/devices/{id}/config/current
// Returns the current live device configuration directly from the device
func (h *Handler) GetCurrentDeviceConfig(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}

	// Get current configuration directly from the device
	config, err := h.Service.ImportDeviceConfig(uint(id))
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to get current device config from device")
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	h.writeJSON(w, map[string]interface{}{
		"success":       true,
		"configuration": config,
	})
}

// GetCurrentDeviceConfigNormalized handles GET /api/v1/devices/{id}/config/current/normalized
// Returns the current live device configuration in normalized format for comparison
func (h *Handler) GetCurrentDeviceConfigNormalized(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		h.writeJSON(w, map[string]interface{}{
			"success": false,
			"error":   "Invalid device ID",
		})
		return
	}

	// Get current configuration directly from the device
	config, err := h.Service.ImportDeviceConfig(uint(id))
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to get current device config from device for normalization")
		w.Header().Set("Content-Type", "application/json")
		h.writeJSON(w, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Parse the raw config for normalization
	var rawConfig map[string]interface{}
	if err := json.Unmarshal(config.Config, &rawConfig); err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to parse device config for normalization")
		h.responseWriter().WriteError(w, r, http.StatusInternalServerError, apiresp.ErrCodeInternalServer, "Failed to parse device configuration", nil)
		return
	}

	// Normalize the configuration
	normalizer := NewConfigNormalizer()
	normalized := normalizer.NormalizeRawConfig(rawConfig)

	h.responseWriter().WriteSuccess(w, r, map[string]interface{}{"configuration": normalized})
}

// ImportDeviceConfig handles POST /api/v1/devices/{id}/config/import
func (h *Handler) ImportDeviceConfig(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}

	// Import configuration from device
	config, err := h.Service.ImportDeviceConfig(uint(id))
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to import device config")
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}

	h.responseWriter().WriteSuccess(w, r, config)
}

// GetImportStatus handles GET /api/v1/devices/{id}/config/status
func (h *Handler) GetImportStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}

	// Get import status for device
	status, err := h.Service.GetImportStatus(uint(id))
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to get import status")
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}

	h.responseWriter().WriteSuccess(w, r, status)
}

// ExportDeviceConfig handles POST /api/v1/devices/{id}/config/export
func (h *Handler) ExportDeviceConfig(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}

	// Export configuration to device
	if err := h.Service.ExportDeviceConfig(uint(id)); err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to export device config")
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}

	response := map[string]interface{}{
		"status":    "success",
		"device_id": id,
		"message":   "Configuration exported to device",
	}

	h.responseWriter().WriteSuccess(w, r, response)
}

// BulkImportConfigs handles POST /api/v1/config/bulk-import
func (h *Handler) BulkImportConfigs(w http.ResponseWriter, r *http.Request) {
	// Get all devices
	devices, err := h.Service.DB.GetDevices()
	if err != nil {
		h.logger.WithFields(map[string]any{
			"error": err.Error(),
		}).Error("Failed to get devices")
		h.responseWriter().WriteInternalError(w, r, err)
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

	h.responseWriter().WriteSuccess(w, r, response)
}

// BulkExportConfigs handles POST /api/v1/config/bulk-export
func (h *Handler) BulkExportConfigs(w http.ResponseWriter, r *http.Request) {
	// Get all devices
	devices, err := h.Service.DB.GetDevices()
	if err != nil {
		h.logger.WithFields(map[string]any{
			"error": err.Error(),
		}).Error("Failed to get devices")
		h.responseWriter().WriteInternalError(w, r, err)
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

	h.responseWriter().WriteSuccess(w, r, response)
}

// DetectConfigDrift handles GET /api/v1/devices/{id}/config/drift
func (h *Handler) DetectConfigDrift(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}

	// Detect configuration drift
	drift, err := h.Service.DetectConfigDrift(uint(id))
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to detect config drift")
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}

	if drift == nil {
		response := map[string]interface{}{
			"device_id": id,
			"drift":     false,
			"message":   "No configuration drift detected",
		}
		h.responseWriter().WriteSuccess(w, r, response)
		return
	}

	h.responseWriter().WriteSuccess(w, r, drift)
}

// BulkDetectConfigDrift handles POST /api/v1/config/bulk-drift-detect
func (h *Handler) BulkDetectConfigDrift(w http.ResponseWriter, r *http.Request) {
	// Perform bulk drift detection across all devices
	result, err := h.Service.BulkDetectConfigDrift()
	if err != nil {
		h.logger.WithFields(map[string]any{
			"error": err.Error(),
		}).Error("Failed to perform bulk drift detection")
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}
	h.responseWriter().WriteSuccess(w, r, result)
}

// GetConfigTemplates handles GET /api/v1/config/templates
func (h *Handler) GetConfigTemplates(w http.ResponseWriter, r *http.Request) {
	templates, err := h.Service.ConfigSvc.GetTemplates()
	if err != nil {
		h.logger.WithFields(map[string]any{
			"error": err.Error(),
		}).Error("Failed to get config templates")
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}
	h.responseWriter().WriteSuccess(w, r, templates)
}

// CreateConfigTemplate handles POST /api/v1/config/templates
func (h *Handler) CreateConfigTemplate(w http.ResponseWriter, r *http.Request) {
	var template configuration.ConfigTemplate
	if err := json.NewDecoder(r.Body).Decode(&template); err != nil {
		h.responseWriter().WriteValidationError(w, r, "Invalid JSON request body")
		return
	}

	if err := h.Service.ConfigSvc.CreateTemplate(&template); err != nil {
		h.logger.WithFields(map[string]any{
			"error": err.Error(),
		}).Error("Failed to create config template")
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}
	h.responseWriter().WriteCreated(w, r, template)
}

// UpdateConfigTemplate handles PUT /api/v1/config/templates/{id}
func (h *Handler) UpdateConfigTemplate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid template ID", nil)
		return
	}

	var template configuration.ConfigTemplate
	if err := json.NewDecoder(r.Body).Decode(&template); err != nil {
		h.responseWriter().WriteValidationError(w, r, "Invalid JSON request body")
		return
	}

	template.ID = uint(id)
	if err := h.Service.ConfigSvc.UpdateTemplate(&template); err != nil {
		h.logger.WithFields(map[string]any{
			"template_id": id,
			"error":       err.Error(),
		}).Error("Failed to update config template")
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}
	h.responseWriter().WriteSuccess(w, r, template)
}

// DeleteConfigTemplate handles DELETE /api/v1/config/templates/{id}
func (h *Handler) DeleteConfigTemplate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid template ID", nil)
		return
	}

	if err := h.Service.ConfigSvc.DeleteTemplate(uint(id)); err != nil {
		h.logger.WithFields(map[string]any{
			"template_id": id,
			"error":       err.Error(),
		}).Error("Failed to delete config template")
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}
	h.responseWriter().WriteNoContent(w, r)
}

// ApplyConfigTemplate handles POST /api/v1/devices/{id}/config/apply-template
func (h *Handler) ApplyConfigTemplate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}

	var req struct {
		TemplateID uint                   `json:"template_id"`
		Variables  map[string]interface{} `json:"variables"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.responseWriter().WriteValidationError(w, r, "Invalid JSON request body")
		return
	}

	if err := h.Service.ApplyConfigTemplate(uint(id), req.TemplateID, req.Variables); err != nil {
		h.logger.WithFields(map[string]any{
			"device_id":   id,
			"template_id": req.TemplateID,
			"error":       err.Error(),
		}).Error("Failed to apply config template")
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}

	response := map[string]interface{}{
		"status":      "success",
		"device_id":   id,
		"template_id": req.TemplateID,
		"message":     "Template applied to device",
	}

	h.responseWriter().WriteSuccess(w, r, response)
}

// GetConfigHistory handles GET /api/v1/devices/{id}/config/history
func (h *Handler) GetConfigHistory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
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
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}

	h.responseWriter().WriteSuccess(w, r, history)
}

// UpdateDeviceConfig handles PUT /api/v1/devices/{id}/config
func (h *Handler) UpdateDeviceConfig(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}

	// Parse request body
	var configUpdate map[string]interface{}
	if decodeErr := json.NewDecoder(r.Body).Decode(&configUpdate); decodeErr != nil {
		h.responseWriter().WriteValidationError(w, r, "Invalid JSON in request body")
		return
	}

	// Update device configuration
	err = h.Service.UpdateDeviceConfig(uint(id), configUpdate)
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to update device config")
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}

	// Return updated config
	config, err := h.Service.GetDeviceConfig(uint(id))
	if err != nil {
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}

	h.responseWriter().WriteSuccess(w, r, config)
}

// UpdateRelayConfig handles PUT /api/v1/devices/{id}/config/relay
func (h *Handler) UpdateRelayConfig(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}

	// Parse relay configuration
	var relayConfig configuration.RelayConfig
	if decodeErr := json.NewDecoder(r.Body).Decode(&relayConfig); decodeErr != nil {
		h.responseWriter().WriteValidationError(w, r, "Invalid relay configuration JSON")
		return
	}

	// Update relay configuration
	err = h.Service.UpdateRelayConfig(uint(id), &relayConfig)
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to update relay config")
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}

	h.responseWriter().WriteSuccess(w, r, map[string]string{"status": "success"})
}

// UpdateDimmingConfig handles PUT /api/v1/devices/{id}/config/dimming
func (h *Handler) UpdateDimmingConfig(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}

	// Parse dimming configuration
	var dimmingConfig configuration.DimmingConfig
	if decodeErr := json.NewDecoder(r.Body).Decode(&dimmingConfig); decodeErr != nil {
		h.responseWriter().WriteValidationError(w, r, "Invalid dimming configuration JSON")
		return
	}

	// Update dimming configuration
	err = h.Service.UpdateDimmingConfig(uint(id), &dimmingConfig)
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to update dimming config")
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}

	h.responseWriter().WriteSuccess(w, r, map[string]string{"status": "success"})
}

// UpdateRollerConfig handles PUT /api/v1/devices/{id}/config/roller
func (h *Handler) UpdateRollerConfig(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}

	// Parse roller configuration
	var rollerConfig configuration.RollerConfig
	if decodeErr := json.NewDecoder(r.Body).Decode(&rollerConfig); decodeErr != nil {
		h.responseWriter().WriteValidationError(w, r, "Invalid roller configuration JSON")
		return
	}

	// Update roller configuration
	err = h.Service.UpdateRollerConfig(uint(id), &rollerConfig)
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to update roller config")
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}

	h.responseWriter().WriteSuccess(w, r, map[string]string{"status": "success"})
}

// UpdatePowerMeteringConfig handles PUT /api/v1/devices/{id}/config/power-metering
func (h *Handler) UpdatePowerMeteringConfig(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}

	// Parse power metering configuration
	var powerConfig configuration.PowerMeteringConfig
	if decodeErr := json.NewDecoder(r.Body).Decode(&powerConfig); decodeErr != nil {
		h.responseWriter().WriteValidationError(w, r, "Invalid power metering configuration JSON")
		return
	}

	// Update power metering configuration
	err = h.Service.UpdatePowerMeteringConfig(uint(id), &powerConfig)
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to update power metering config")
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}

	h.responseWriter().WriteSuccess(w, r, map[string]string{"status": "success"})
}

// UpdateDeviceAuth handles PUT /api/v1/devices/{id}/config/auth
func (h *Handler) UpdateDeviceAuth(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}

	// Parse authentication configuration
	var authConfig struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if decodeErr := json.NewDecoder(r.Body).Decode(&authConfig); decodeErr != nil {
		h.responseWriter().WriteValidationError(w, r, "Invalid auth configuration JSON")
		return
	}

	// Update device authentication
	err = h.Service.UpdateDeviceAuth(uint(id), authConfig.Username, authConfig.Password)
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to update device auth")
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}

	h.responseWriter().WriteSuccess(w, r, map[string]string{"status": "success"})
}

// GetDriftSchedules handles GET /api/v1/config/drift-schedules
func (h *Handler) GetDriftSchedules(w http.ResponseWriter, r *http.Request) {
	schedules, err := h.Service.GetDriftSchedules()
	if err != nil {
		h.logger.WithFields(map[string]any{
			"error": err.Error(),
		}).Error("Failed to get drift schedules")
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}
	h.responseWriter().WriteSuccess(w, r, schedules)
}

// CreateDriftSchedule handles POST /api/v1/config/drift-schedules
func (h *Handler) CreateDriftSchedule(w http.ResponseWriter, r *http.Request) {
	var schedule configuration.DriftDetectionSchedule
	if err := json.NewDecoder(r.Body).Decode(&schedule); err != nil {
		h.responseWriter().WriteValidationError(w, r, "Invalid schedule JSON")
		return
	}

	created, err := h.Service.CreateDriftSchedule(schedule)
	if err != nil {
		h.logger.WithFields(map[string]any{
			"schedule_name": schedule.Name,
			"error":         err.Error(),
		}).Error("Failed to create drift schedule")
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}
	h.responseWriter().WriteCreated(w, r, created)
}

// GetDriftSchedule handles GET /api/v1/config/drift-schedules/{id}
func (h *Handler) GetDriftSchedule(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid schedule ID", nil)
		return
	}

	schedule, err := h.Service.GetDriftSchedule(uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			h.responseWriter().WriteNotFoundError(w, r, "Schedule")
			return
		}
		h.logger.WithFields(map[string]any{
			"schedule_id": id,
			"error":       err.Error(),
		}).Error("Failed to get drift schedule")
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}
	h.responseWriter().WriteSuccess(w, r, schedule)
}

// UpdateDriftSchedule handles PUT /api/v1/config/drift-schedules/{id}
func (h *Handler) UpdateDriftSchedule(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid schedule ID", nil)
		return
	}

	var updates configuration.DriftDetectionSchedule
	if decodeErr := json.NewDecoder(r.Body).Decode(&updates); decodeErr != nil {
		h.responseWriter().WriteValidationError(w, r, "Invalid schedule JSON")
		return
	}

	updated, err := h.Service.UpdateDriftSchedule(uint(id), updates)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			h.responseWriter().WriteNotFoundError(w, r, "Schedule")
			return
		}
		h.logger.WithFields(map[string]any{
			"schedule_id": id,
			"error":       err.Error(),
		}).Error("Failed to update drift schedule")
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}
	h.responseWriter().WriteSuccess(w, r, updated)
}

// DeleteDriftSchedule handles DELETE /api/v1/config/drift-schedules/{id}
func (h *Handler) DeleteDriftSchedule(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid schedule ID", nil)
		return
	}

	err = h.Service.DeleteDriftSchedule(uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			h.responseWriter().WriteNotFoundError(w, r, "Schedule")
			return
		}
		h.logger.WithFields(map[string]any{
			"schedule_id": id,
			"error":       err.Error(),
		}).Error("Failed to delete drift schedule")
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}
	h.responseWriter().WriteSuccess(w, r, map[string]string{"status": "deleted"})
}

// ToggleDriftSchedule handles POST /api/v1/config/drift-schedules/{id}/toggle
func (h *Handler) ToggleDriftSchedule(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid schedule ID", nil)
		return
	}

	updated, err := h.Service.ToggleDriftSchedule(uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			h.responseWriter().WriteNotFoundError(w, r, "Schedule")
			return
		}
		h.logger.WithFields(map[string]any{
			"schedule_id": id,
			"error":       err.Error(),
		}).Error("Failed to toggle drift schedule")
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}
	h.responseWriter().WriteSuccess(w, r, updated)
}

// GetDriftScheduleRuns handles GET /api/v1/config/drift-schedules/{id}/runs
func (h *Handler) GetDriftScheduleRuns(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid schedule ID", nil)
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
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}
	h.responseWriter().WriteSuccess(w, r, runs)
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
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}
	h.responseWriter().WriteSuccess(w, r, reports)
}

// GenerateDeviceDriftReport handles POST /api/v1/devices/{id}/drift-report
func (h *Handler) GenerateDeviceDriftReport(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}

	report, err := h.Service.GenerateDeviceDriftReport(uint(id))
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to generate device drift report")
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}

	h.responseWriter().WriteCreated(w, r, report)
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
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}

	h.responseWriter().WriteSuccess(w, r, trends)
}

// MarkTrendResolved handles POST /api/v1/config/drift-trends/{id}/resolve
func (h *Handler) MarkTrendResolved(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid trend ID", nil)
		return
	}

	err = h.Service.MarkTrendResolved(uint(id))
	if err != nil {
		h.logger.WithFields(map[string]any{
			"trend_id": id,
			"error":    err.Error(),
		}).Error("Failed to mark trend as resolved")
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}

	h.responseWriter().WriteSuccess(w, r, map[string]string{"status": "resolved"})
}

// EnhancedBulkDetectConfigDrift handles POST /api/v1/config/bulk-drift-detect-enhanced
func (h *Handler) EnhancedBulkDetectConfigDrift(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("Starting enhanced bulk drift detection with comprehensive reporting")

	result, err := h.Service.BulkDetectConfigDrift()
	if err != nil {
		h.logger.WithFields(map[string]any{
			"error": err.Error(),
		}).Error("Failed to perform bulk drift detection")
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}

	// Generate comprehensive report
	report, err := h.Service.EnhanceBulkDriftResult(result, nil)
	if err != nil {
		h.logger.WithFields(map[string]any{
			"error": err.Error(),
		}).Warn("Failed to generate comprehensive report, returning basic result")

		// Fall back to basic result if reporting fails
		h.responseWriter().WriteSuccess(w, r, result)
		return
	}

	h.logger.WithFields(map[string]any{
		"report_id":       report.ID,
		"devices_drifted": result.Drifted,
		"critical_issues": report.Summary.CriticalIssues,
		"recommendations": len(report.Recommendations),
	}).Info("Enhanced bulk drift detection completed")

	h.responseWriter().WriteSuccess(w, r, report)
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
		h.responseWriter().WriteValidationError(w, r, "Invalid request body")
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

	h.responseWriter().WriteSuccess(w, r, map[string]interface{}{"result": resultObj})
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
		h.responseWriter().WriteValidationError(w, r, "Invalid request body")
		return
	}

	// Create a template engine to validate
	templateEngine := configuration.NewTemplateEngine(h.logger)
	err := templateEngine.ValidateTemplate(req.Template)

	if err != nil {
		h.logger.WithFields(map[string]any{
			"error": err.Error(),
		}).Info("Template validation failed")

		h.responseWriter().WriteSuccess(w, r, map[string]interface{}{"valid": false, "error": err.Error()})
		return
	}

	h.logger.Info("Template validation succeeded")

	h.responseWriter().WriteSuccess(w, r, map[string]interface{}{"valid": true})
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
		h.writeJSON(w, map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	if req.Name == "" || req.Template == "" {
		h.responseWriter().WriteValidationError(w, r, "Template name and content are required")
		return
	}

	// Validate template before saving
	templateEngine := configuration.NewTemplateEngine(h.logger)
	if err := templateEngine.ValidateTemplate(req.Template); err != nil {
		h.logger.WithFields(map[string]any{
			"name":  req.Name,
			"error": err.Error(),
		}).Warn("Cannot save invalid template")

		h.responseWriter().WriteValidationError(w, r, "Template validation failed: "+err.Error())
		return
	}

	// Convert variables to JSON
	variablesJSON, err := json.Marshal(req.Variables)
	if err != nil {
		h.logger.WithFields(map[string]any{
			"error": err.Error(),
		}).Warn("Failed to marshal template variables")

		h.responseWriter().WriteInternalError(w, r, fmt.Errorf("failed to process variables"))
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

		h.responseWriter().WriteInternalError(w, r, fmt.Errorf("failed to save template"))
		return
	}

	h.logger.WithFields(map[string]any{
		"name":        req.Name,
		"template_id": template.ID,
	}).Info("Template saved successfully")

	h.responseWriter().WriteSuccess(w, r, map[string]interface{}{"id": template.ID})
}

// GetTemplateExamples handles GET /api/v1/configuration/template-examples
func (h *Handler) GetTemplateExamples(w http.ResponseWriter, r *http.Request) {
	examples := configuration.GetTemplateExamples()
	documentation := configuration.GetTemplateDocumentation()

	h.responseWriter().WriteSuccess(w, r, map[string]interface{}{"examples": examples, "documentation": documentation})
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
