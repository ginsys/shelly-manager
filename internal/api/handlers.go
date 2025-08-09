package api

import (
	"encoding/json"
	"net/http"
	"strconv"

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
	// This will be implemented when we integrate with the discovery service
	response := map[string]interface{}{
		"status":  "discovery_started",
		"message": "Device discovery has been initiated",
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