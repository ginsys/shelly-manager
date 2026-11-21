package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/mux"
	"gorm.io/gorm"

	apiresp "github.com/ginsys/shelly-manager/internal/api/response"
	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/metrics"
	"github.com/ginsys/shelly-manager/internal/notification"
	"github.com/ginsys/shelly-manager/internal/redact"
	"github.com/ginsys/shelly-manager/internal/security/apikey"
	"github.com/ginsys/shelly-manager/internal/service"
	"github.com/ginsys/shelly-manager/internal/shelly"
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
	// adminKey provides simple guard for sensitive endpoints until full auth is implemented. The export,
	// import and metrics handlers read the same key once ShareAdminKey has run, so a rotation switches
	// all of them at once.
	adminKey apikey.Key
	// AgentAPIKey is the second key, accepted next to the admin key on the provisioner agent
	// routes only. Empty means the agent routes accept the admin key alone.
	AgentAPIKey string
	// driftSchedulerEnabled is whether this process runs the drift scheduler; set once at start-up.
	driftSchedulerEnabled bool
	// Version/banner support
	serverStartedAt time.Time
	// gitSHA is the revision this binary was built from, or "" when the build carries none.
	gitSHA string
	// background tracks goroutines that handlers start after they answer, such as the cleanup
	// of expired discovered devices.
	background sync.WaitGroup
	// registryPurgedAt is when the provisioner registry purge last started, in Unix nanoseconds.
	registryPurgedAt atomic.Int64
}

// WaitBackground blocks until the background work that handlers started has finished. Call it
// before closing the database the handlers use, so that work never reads a closed database.
func (h *Handler) WaitBackground() { h.background.Wait() }

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
		serverStartedAt:     time.Now(),
		gitSHA:              buildRevision(),
	}
}

// SetAdminAPIKey sets the in-memory admin key for guarding sensitive operations.
func (h *Handler) SetAdminAPIKey(key string) { h.adminKey.Set(key) }

// AdminAPIKey returns the current admin key, or "" when none is set.
func (h *Handler) AdminAPIKey() string { return h.adminKey.Get() }

// ShareAdminKey makes the export, import and metrics handlers read this handler's admin key, so one
// change of it, RotateAdminKey, switches all of them at once. Call it when the handlers are
// assembled, after they are attached; their own earlier keys are dropped.
func (h *Handler) ShareAdminKey() {
	if h.ExportHandlers != nil {
		h.ExportHandlers.adminKey.Share(&h.adminKey)
	}
	if h.ImportHandlers != nil {
		h.ImportHandlers.adminKey.Share(&h.adminKey)
	}
	if h.MetricsHandler != nil {
		h.MetricsHandler.ShareAdminKey(&h.adminKey)
	}
}

// SetAgentAPIKey sets the in-memory key the provisioner agents send. It opens the agent routes and
// nothing else.
func (h *Handler) SetAgentAPIKey(key string) { h.AgentAPIKey = key }

// requireAdmin checks Authorization or X-API-Key against the admin key.
func (h *Handler) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	key := h.adminKey.Get()
	if key == "" {
		return true
	}
	if !apikey.Authorized(r, key) {
		h.responseWriter().WriteError(w, r, http.StatusUnauthorized, apiresp.ErrCodeUnauthorized, "Admin authorization required", nil)
		return false
	}
	return true
}

// requireAgentOrAdmin is requireAdmin for the routes the provisioner agent calls: it accepts the agent
// key as well. With no admin key set the route stays open, as every guarded route does; an unset agent
// key adds no credential.
func (h *Handler) requireAgentOrAdmin(w http.ResponseWriter, r *http.Request) bool {
	key := h.adminKey.Get()
	if key == "" {
		return true
	}
	if !apikey.Authorized(r, key, h.AgentAPIKey) {
		h.responseWriter().WriteError(w, r, http.StatusUnauthorized, apiresp.ErrCodeUnauthorized, "Authorization required", nil)
		return false
	}
	return true
}

// RotateAdminKey updates the in-memory admin key used by API/WS/export/import handlers, all at once
// (they read one shared key, see ShareAdminKey).
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
	// Header handling drops trailing whitespace, and leading whitespace in
	// X-API-Key, so such a key cannot be presented reliably. Refuse it; never
	// trim, so the stored key is the one the operator typed.
	if body.NewKey != strings.TrimSpace(body.NewKey) {
		h.responseWriter().WriteValidationError(w, r, "new_key must not start or end with whitespace")
		return
	}

	// One store switches every handler that shares the key (ShareAdminKey).
	h.adminKey.Set(body.NewKey)

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
	body, err := json.Marshal(data)
	if err != nil {
		if h.logger != nil {
			h.logger.Error("Failed to marshal JSON response", "error", err)
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	body = append(body, '\n')
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	if _, writeErr := w.Write(body); writeErr != nil && h.logger != nil {
		h.logger.Error("Failed to write JSON response", "error", writeErr)
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
	h.writeJSON(w, resp)
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

// Version returns minimal API version/build info for UI mismatch banner. git_sha is the revision
// the server binary was built from; it is empty when the build carries none, and the UI then
// shows no banner.
func (h *Handler) Version(w http.ResponseWriter, r *http.Request) {
	resp := map[string]any{
		"api_version":       "dev",
		"server_started_at": h.serverStartedAt.Format(time.RFC3339),
		"git_sha":           h.gitSHA,
	}
	// Try to include database provider info if available
	if mgr, ok := h.DB.(interface{ GetProviderInfo() (string, string) }); ok {
		name, ver := mgr.GetProviderInfo()
		resp["database_provider_name"] = name
		resp["database_provider_version"] = ver
	}
	h.writeJSON(w, apiresp.Success(resp))
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
	// The saved secrets inside Settings, Overrides and DesiredConfig never leave the API.
	masked := make([]database.Device, len(pageDevices))
	for i, d := range pageDevices {
		masked[i] = maskDeviceSecrets(d)
	}
	pageDevices = masked

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

	// A mask in the body has nothing stored to take its place: it is dropped, never stored.
	// With nothing stored no entry can be ambiguous, but a text that repeats a member name is refused.
	if err := restoreDeviceSecrets(&device, nil); err != nil {
		h.responseWriter().WriteValidationError(w, r, err.Error())
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
			"settings":    maskJSONText(device.Settings),
			"component":   "api",
			"operation":   "add_device",
			"request_id":  r.Context().Value("request_id"),
		}).Error("AddDevice operation failed with detailed context")

		// A unique constraint violation is a conflict. The database seals driver text,
		// so the violation is read by type and the column is no longer known.
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			h.responseWriter().WriteError(w, r, http.StatusConflict, apiresp.ErrCodeConflict, "Device with this IP or MAC address already exists", nil)
			return
		}

		h.responseWriter().WriteInternalError(w, r, err)
		return
	}

	h.responseWriter().WriteCreated(w, r, maskDeviceSecrets(device))
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

	h.responseWriter().WriteSuccess(w, r, maskDeviceSecrets(*device))
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

	// The client read the device with its secrets masked: a mask that comes back takes the stored
	// value, so the mask is never saved over a secret.
	if err := restoreDeviceSecrets(&updatedDevice, existingDevice); err != nil {
		h.responseWriter().WriteValidationError(w, r, err.Error())
		return
	}

	// Validate and normalize device settings
	if err := h.validateDeviceSettings(&updatedDevice); err != nil {
		h.responseWriter().WriteValidationError(w, r, fmt.Sprintf("Invalid device settings: %v", err))
		return
	}

	// Update existing device with new data
	updatedDevice.ID = existingDevice.ID
	// The write keeps a login that was saved after the read above, unless the request changes it.
	if err := h.Service.UpdateDeviceKeepingLogin(existingDevice, &updatedDevice); err != nil {
		if errors.Is(err, service.ErrDeviceChanged) {
			h.responseWriter().WriteError(w, r, http.StatusConflict, apiresp.ErrCodeConflict,
				"The device changed after you read it. Reload it and try again.", nil)
			return
		}
		// The device was deleted after the read above.
		if errors.Is(err, gorm.ErrRecordNotFound) {
			h.responseWriter().WriteNotFoundError(w, r, "Device")
			return
		}
		h.logger.WithFields(map[string]any{
			"error":      err.Error(),
			"device_id":  updatedDevice.ID,
			"device_ip":  updatedDevice.IP,
			"device_mac": updatedDevice.MAC,
			"component":  "api",
			"operation":  "update_device",
			"request_id": r.Context().Value("request_id"),
		}).Error("UpdateDevice operation failed with detailed context")

		if errors.Is(err, gorm.ErrDuplicatedKey) {
			h.responseWriter().WriteError(w, r, http.StatusConflict, apiresp.ErrCodeConflict, "Device with this IP or MAC address already exists", nil)
			return
		}

		h.responseWriter().WriteInternalError(w, r, err)
		return
	}

	h.responseWriter().WriteSuccess(w, r, maskDeviceSecrets(updatedDevice))
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

	// Run discovery in background. WaitBackground waits for it, so a caller that closes the
	// database never leaves a discovery running.
	h.background.Add(1)
	go func() {
		defer h.background.Done()
		network := req.Network
		if network == "" {
			network = "auto"
		}

		h.logger.WithFields(map[string]any{
			"network":       network,
			"import_config": req.ImportConfig,
			"component":     "api",
		}).Info("Starting device discovery")

		// Discover devices. The service stores each one, and says whether this pass added it.
		devices, err := h.Service.DiscoverDevicesReport(network)
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

		// Count the devices this pass added and import the configuration of each device found
		newDevices := 0
		configsImported := 0

		for _, device := range devices {
			if device.Added {
				newDevices++
			}
			if !req.ImportConfig {
				continue
			}
			if _, err := h.Service.ImportDeviceConfig(device.ID); err == nil {
				configsImported++
			} else if device.Added {
				h.logger.WithFields(map[string]any{
					"device_id": device.ID,
					"device_ip": device.IP,
					"error":     err.Error(),
					"component": "api",
				}).Warn("Failed to import config for new device")
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

	h.responseWriter().WriteSuccess(w, r, status)
}

// ProvisionDevices handles POST /api/v1/provisioning/provision
func (h *Handler) ProvisionDevices(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":  "provisioning_started",
		"message": "Device provisioning has been initiated",
	}

	h.responseWriter().WriteSuccess(w, r, response)
}

// GetDHCPReservations handles GET /api/v1/dhcp/reservations
func (h *Handler) GetDHCPReservations(w http.ResponseWriter, r *http.Request) {
	reservations := []map[string]interface{}{}

	h.responseWriter().WriteSuccess(w, r, reservations)
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
		Force  bool                   `json:"force"`
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

	// Pass force flag into params
	if req.Force {
		if req.Params == nil {
			req.Params = make(map[string]interface{})
		}
		req.Params["force"] = true
	}

	// Execute control command
	if err := h.Service.ControlDevice(uint(id), req.Action, req.Params); err != nil {
		if errors.Is(err, service.ErrDeviceOffline) {
			h.responseWriter().WriteError(w, r, http.StatusServiceUnavailable, apiresp.ErrCodeDeviceOffline,
				"Device is offline. Set \"force\": true to attempt anyway.", nil)
			return
		}
		if errors.Is(err, service.ErrInvalidChannel) {
			h.responseWriter().WriteValidationError(w, r, map[string]string{"channel": err.Error()})
			return
		}
		if errors.Is(err, configuration.ErrDeviceIdentity) {
			h.responseWriter().WriteError(w, r, http.StatusConflict, apiresp.ErrCodeConflict, err.Error(), nil)
			return
		}
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
		if errors.Is(err, service.ErrDeviceOffline) {
			h.responseWriter().WriteError(w, r, http.StatusServiceUnavailable, apiresp.ErrCodeDeviceOffline, "Device is offline", nil)
			return
		}
		if h.writeDeviceLoginRefused(w, r, err) || h.writeDeviceUnanswered(w, r, err) {
			return
		}
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
		if errors.Is(err, service.ErrDeviceOffline) {
			h.responseWriter().WriteError(w, r, http.StatusServiceUnavailable, apiresp.ErrCodeDeviceOffline, "Device is offline", nil)
			return
		}
		if h.writeDeviceLoginRefused(w, r, err) || h.writeDeviceUnanswered(w, r, err) {
			return
		}
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
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// A device that was never imported has no stored configuration.
		h.responseWriter().WriteNotFoundError(w, r, "Imported device configuration")
		return
	}
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to get device config")
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}

	// The stored record keeps the real values; only the answer is masked.
	h.responseWriter().WriteSuccess(w, r, maskDeviceConfig(config))
}

// writeDeviceUnanswered answers a device that did not answer in time with 504 and the error's text,
// which names the device and its address (ginsys/shelly-manager#755), and reports whether it did. It
// matches the service's own DeviceUnansweredError and not the timeout itself, so only a read that the
// service wrapped as one reads "did not answer": an apply that may have written is never told so.
func (h *Handler) writeDeviceUnanswered(w http.ResponseWriter, r *http.Request, err error) bool {
	var unanswered *service.DeviceUnansweredError
	if !errors.As(err, &unanswered) {
		return false
	}
	h.logger.WithFields(map[string]any{
		"device_id": unanswered.DeviceID,
		"device_ip": unanswered.IP,
	}).Warn("Device did not answer")
	h.responseWriter().WriteError(w, r, http.StatusGatewayTimeout, apiresp.ErrCodeDeviceUnreachable, unanswered.Error(), nil)
	return true
}

// writeDeviceLoginRefused answers a device that rejected the stored login with 502 and the message
// that names the fix, and reports whether it did. It is not 401, which would read as the caller's
// own login to this API failing. It runs before writeDeviceUnanswered: a refused login whose
// identity read then timed out is still a refused login (ginsys/shelly-manager#759).
func (h *Handler) writeDeviceLoginRefused(w http.ResponseWriter, r *http.Request, err error) bool {
	if !shelly.IsAuthError(err) {
		return false
	}
	h.responseWriter().WriteError(w, r, http.StatusBadGateway, apiresp.ErrCodeExternalServiceError,
		"The device rejected the stored credentials; update them with PUT /devices/{id}/config/auth", nil)
	return true
}

// writeDeviceReadError reports a failed read from a physical device. A device
// that rejects our login is an upstream problem the caller can act on (fix the
// stored credentials), so it gets 502 with a specific message instead of a
// generic 500. It is not 401, which would read as the caller's own login to
// this API failing. A device that did not answer gets 504 (writeDeviceUnanswered).
func (h *Handler) writeDeviceReadError(w http.ResponseWriter, r *http.Request, err error) {
	if h.writeDeviceLoginRefused(w, r, err) {
		return
	}
	if h.writeDeviceUnanswered(w, r, err) {
		return
	}
	h.responseWriter().WriteInternalError(w, r, err)
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

	// Read the current configuration from the device without storing it: the stored
	// record is the baseline that drift detection compares against.
	config, err := h.Service.ReadDeviceConfig(uint(id))
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to get current device config from device")
		h.writeDeviceReadError(w, r, err)
		return
	}

	h.responseWriter().WriteSuccess(w, r, maskDeviceConfig(config))
}

// GetCurrentDeviceConfigNormalized handles GET /api/v1/devices/{id}/config/current/normalized
// Returns the current live device configuration in normalized format for comparison
func (h *Handler) GetCurrentDeviceConfigNormalized(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}

	// Read the current configuration from the device without storing it: the stored
	// record is the baseline that drift detection compares against.
	config, err := h.Service.ReadDeviceConfig(uint(id))
	if err != nil {
		h.logger.WithFields(map[string]any{
			"device_id": id,
			"error":     err.Error(),
		}).Error("Failed to get current device config from device for normalization")
		h.writeDeviceReadError(w, r, err)
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

	h.responseWriter().WriteSuccess(w, r, maskDeviceConfig(config))
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

// BulkImportConfigs handles POST /api/v1/config/bulk-import. It reads the live configuration of
// each named device into the manager, as POST /devices/{id}/config/import does for one device.
func (h *Handler) BulkImportConfigs(w http.ResponseWriter, r *http.Request) {
	// Admin before any parsing or device contact.
	if !h.requireAdmin(w, r) {
		return
	}
	selection, ok := h.decodeBulkRequest(w, r)
	if !ok {
		return
	}

	ctx, cancel := bulkReadContext(r)
	defer cancel()
	result := h.runBulkConfig(ctx, selection, "import", func(id uint) (uint, error) {
		config, err := h.Service.ImportDeviceConfig(id)
		if err != nil {
			return 0, err
		}
		return config.ID, nil
	})
	h.responseWriter().WriteSuccess(w, r, result)
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
		if errors.Is(err, service.ErrDeviceOffline) {
			h.responseWriter().WriteError(w, r, http.StatusServiceUnavailable, apiresp.ErrCodeDeviceOffline, "Device is offline", nil)
			return
		}
		if errors.Is(err, configuration.ErrNoDesiredConfig) {
			h.logger.WithFields(map[string]any{
				"device_id": id,
			}).Warn("No desired configuration for drift detection")
			h.responseWriter().WriteError(w, r, http.StatusNotFound, apiresp.ErrCodeNotFound, err.Error(), nil)
			return
		}
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

// BulkDetectConfigDrift handles POST /api/v1/config/bulk-drift-detect for the named devices.
func (h *Handler) BulkDetectConfigDrift(w http.ResponseWriter, r *http.Request) {
	// Admin before any parsing or device contact.
	if !h.requireAdmin(w, r) {
		return
	}
	selection, ok := h.decodeBulkRequest(w, r)
	if !ok {
		return
	}
	ctx, cancel := bulkReadContext(r)
	defer cancel()
	result, err := h.Service.BulkDetectConfigDrift(ctx, selection.IDs, selection.StopOnError)
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
// writeTemplateScopeError maps a violation of the template scope invariant to a
// 400. These handlers previously funnelled every error into a 500, which let a
// client's malformed scope look like a server fault — and, worse, left the
// invariant unenforced on this write path (see #275). Matching with errors.Is
// keeps the mapping intact if the services later wrap these errors with context.
// Reports whether the error was handled.
func (h *Handler) writeTemplateScopeError(w http.ResponseWriter, r *http.Request, err error) bool {
	if !errors.Is(err, configuration.ErrInvalidScope) && !errors.Is(err, configuration.ErrDeviceTypeRequired) {
		return false
	}
	h.responseWriter().WriteValidationError(w, r, err.Error())
	return true
}

func (h *Handler) CreateConfigTemplate(w http.ResponseWriter, r *http.Request) {
	var template configuration.ConfigTemplate
	if err := json.NewDecoder(r.Body).Decode(&template); err != nil {
		h.responseWriter().WriteValidationError(w, r, "Invalid JSON request body")
		return
	}

	if err := h.Service.ConfigSvc.CreateTemplate(&template); err != nil {
		if h.writeTemplateScopeError(w, r, err) {
			return
		}
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
		if h.writeTemplateScopeError(w, r, err) {
			return
		}
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
		if errors.Is(err, configuration.ErrTemplateNotFound) {
			h.responseWriter().WriteNotFoundError(w, r, "Template")
			return
		}
		if errors.Is(err, configuration.ErrDeviceNotFound) {
			h.responseWriter().WriteNotFoundError(w, r, "Device")
			return
		}
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

	// The stored rows keep the real values; only the answer is masked.
	h.responseWriter().WriteSuccess(w, r, maskConfigHistory(history))
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

	// A client that read the config sends the mask back: a masked secret takes the stored value,
	// and the mask is never stored. A missing stored config is left to the service to report.
	if stored, getErr := h.Service.GetDeviceConfig(uint(id)); getErr == nil {
		old, _ := decodeDocument(string(stored.Config))
		if restoreErr := redact.RestoreTreeInner(configUpdate, old, nil); restoreErr != nil {
			h.responseWriter().WriteValidationError(w, r, restoreErr.Error())
			return
		}
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

	h.responseWriter().WriteSuccess(w, r, maskDeviceConfig(config))
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

	// A client that read the device sends the mask back as the password: it keeps the stored one,
	// and the mask is never stored. With none stored there is nothing to keep.
	if authConfig.Password == secretMask {
		device, getErr := h.DB.GetDevice(uint(id))
		if getErr != nil {
			if errors.Is(getErr, gorm.ErrRecordNotFound) {
				h.responseWriter().WriteNotFoundError(w, r, "Device")
			} else {
				h.responseWriter().WriteInternalError(w, r, getErr)
			}
			return
		}
		authConfig.Password = storedAuthPassword(device.Settings)
	}

	// Both client builders need a user and a password, so a user without a password would be
	// stored as enabled auth that is never used. An empty user is how auth is turned off.
	if authConfig.Username != "" && authConfig.Password == "" {
		h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeValidationFailed,
			"A device login needs a password; send an empty username to turn device auth off", nil)
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

// decodeScheduleInput reads a schedule write body. An unknown field is refused, so a filter key no
// column has (device_type) fails at write time and not as a failed run later.
func (h *Handler) decodeScheduleInput(w http.ResponseWriter, r *http.Request) (configuration.ScheduleInput, bool) {
	var in configuration.ScheduleInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&in); err != nil {
		h.responseWriter().WriteValidationError(w, r, "Invalid JSON request body: "+err.Error())
		return in, false
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) { // one JSON value, nothing after it
		h.responseWriter().WriteValidationError(w, r, "Invalid JSON request body: unexpected text after the JSON value")
		return in, false
	}
	return in, true
}

// writeScheduleError answers the failure of a schedule write: a refused input is a 400, a missing
// schedule a 404, anything else a 500.
func (h *Handler) writeScheduleError(w http.ResponseWriter, r *http.Request, what string, id uint, err error) {
	switch {
	case errors.Is(err, configuration.ErrInvalidDriftSchedule):
		h.responseWriter().WriteValidationError(w, r, err.Error())
	case errors.Is(err, gorm.ErrRecordNotFound):
		h.responseWriter().WriteNotFoundError(w, r, "Schedule")
	default:
		h.logger.WithFields(map[string]any{"schedule_id": id, "error": err.Error()}).Error(what)
		h.responseWriter().WriteInternalError(w, r, err)
	}
}

// scheduleID reads the {id} of a schedule route, answering 400 when it is not a number.
func (h *Handler) scheduleID(w http.ResponseWriter, r *http.Request) (uint, bool) {
	id, err := strconv.ParseUint(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		h.responseWriter().WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid schedule ID", nil)
		return 0, false
	}
	return uint(id), true
}

// CreateDriftSchedule handles POST /api/v1/config/drift-schedules.
func (h *Handler) CreateDriftSchedule(w http.ResponseWriter, r *http.Request) {
	in, ok := h.decodeScheduleInput(w, r)
	if !ok {
		return
	}
	schedule, err := h.Service.CreateDriftSchedule(in)
	if err != nil {
		h.writeScheduleError(w, r, "Failed to create drift schedule", 0, err)
		return
	}
	h.responseWriter().WriteCreated(w, r, schedule)
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
		// The service wraps the GORM error with %w, so match with errors.Is;
		// a bare == comparison would miss it and mask a missing row as a 500.
		if errors.Is(err, gorm.ErrRecordNotFound) {
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

// UpdateDriftSchedule handles PUT /api/v1/config/drift-schedules/{id}.
func (h *Handler) UpdateDriftSchedule(w http.ResponseWriter, r *http.Request) {
	id, ok := h.scheduleID(w, r)
	if !ok {
		return
	}
	in, ok := h.decodeScheduleInput(w, r)
	if !ok {
		return
	}
	schedule, err := h.Service.UpdateDriftSchedule(id, in)
	if err != nil {
		h.writeScheduleError(w, r, "Failed to update drift schedule", id, err)
		return
	}
	h.responseWriter().WriteSuccess(w, r, schedule)
}

// DeleteDriftSchedule handles DELETE /api/v1/config/drift-schedules/{id}
func (h *Handler) DeleteDriftSchedule(w http.ResponseWriter, r *http.Request) {
	id, ok := h.scheduleID(w, r)
	if !ok {
		return
	}
	if err := h.Service.DeleteDriftSchedule(id); err != nil {
		h.writeScheduleError(w, r, "Failed to delete drift schedule", id, err)
		return
	}
	h.responseWriter().WriteSuccess(w, r, map[string]string{"status": "deleted"})
}

// ToggleDriftSchedule handles POST /api/v1/config/drift-schedules/{id}/toggle.
func (h *Handler) ToggleDriftSchedule(w http.ResponseWriter, r *http.Request) {
	id, ok := h.scheduleID(w, r)
	if !ok {
		return
	}
	schedule, err := h.Service.ToggleDriftSchedule(id)
	if err != nil {
		h.writeScheduleError(w, r, "Failed to toggle drift schedule", id, err)
		return
	}
	h.responseWriter().WriteSuccess(w, r, schedule)
}

// Run list bounds: the scheduler keeps 100 runs per schedule, so a larger limit asks for nothing more.
const (
	driftRunsDefaultLimit = 20
	driftRunsMaxLimit     = 100
)

// GetDriftScheduleRuns handles GET /api/v1/config/drift-schedules/{id}/runs?limit=N: the newest runs
// first, without their results.
func (h *Handler) GetDriftScheduleRuns(w http.ResponseWriter, r *http.Request) {
	id, ok := h.scheduleID(w, r)
	if !ok {
		return
	}
	limit := driftRunsDefaultLimit
	if r.URL.Query().Has("limit") { // an empty value is a bad limit, not an absent one
		n, err := strconv.Atoi(r.URL.Query().Get("limit"))
		if err != nil || n < 1 || n > driftRunsMaxLimit {
			h.responseWriter().WriteValidationError(w, r, fmt.Sprintf("limit must be a whole number from 1 to %d", driftRunsMaxLimit))
			return
		}
		limit = n
	}
	runs, err := h.Service.GetDriftScheduleRuns(id, limit)
	if err != nil {
		h.writeScheduleError(w, r, "Failed to get drift schedule runs", id, err)
		return
	}
	h.responseWriter().WriteSuccess(w, r, runs)
}

// SetDriftSchedulerEnabled tells the handler whether this process runs the drift scheduler
// (drift.scheduler_enabled). A handler never told answers false.
func (h *Handler) SetDriftSchedulerEnabled(enabled bool) { h.driftSchedulerEnabled = enabled }

// GetDriftSchedulerStatus handles GET /api/v1/config/drift-scheduler. It reports the configuration of
// the replica that answers, so replicas configured differently answer differently.
func (h *Handler) GetDriftSchedulerStatus(w http.ResponseWriter, r *http.Request) {
	h.responseWriter().WriteSuccess(w, r, map[string]bool{"enabled": h.driftSchedulerEnabled})
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
		if errors.Is(err, configuration.ErrNoDesiredConfig) {
			h.responseWriter().WriteError(w, r, http.StatusNotFound, apiresp.ErrCodeNotFound, err.Error(), nil)
			return
		}
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

// EnhancedBulkDetectConfigDrift handles POST /api/v1/config/bulk-drift-detect-enhanced for the
// named devices, and answers with the comprehensive report.
func (h *Handler) EnhancedBulkDetectConfigDrift(w http.ResponseWriter, r *http.Request) {
	// Admin before any parsing or device contact.
	if !h.requireAdmin(w, r) {
		return
	}
	selection, ok := h.decodeBulkRequest(w, r)
	if !ok {
		return
	}
	h.logger.Info("Starting enhanced bulk drift detection with comprehensive reporting")

	ctx, cancel := bulkReadContext(r)
	defer cancel()
	result, err := h.Service.BulkDetectConfigDrift(ctx, selection.IDs, selection.StopOnError)
	if err != nil {
		h.logger.WithFields(map[string]any{
			"error": err.Error(),
		}).Error("Failed to perform bulk drift detection")
		h.responseWriter().WriteInternalError(w, r, err)
		return
	}

	// The report leaves out the devices that were skipped, so a run that stopped at its time limit
	// answers the basic result, which names every device and why it was not read. No report of the
	// devices that did run is stored then. (A stop_on_error skip is the operator's own choice and
	// still answers the report.)
	if bulkReadTimeLimited(result) {
		h.responseWriter().WriteSuccess(w, r, result)
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
