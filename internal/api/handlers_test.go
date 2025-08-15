package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/notification"
	"github.com/ginsys/shelly-manager/internal/service"
	"github.com/ginsys/shelly-manager/internal/testutil"
	"github.com/gorilla/mux"
)

// testShellyService creates a test ShellyService for API handler tests
func testShellyService(t *testing.T, db *database.Manager) *service.ShellyService {
	t.Helper()
	cfg := testutil.TestConfig()
	return service.NewService(db, cfg)
}

// testNotificationHandler creates a test notification handler for API tests
func testNotificationHandler(t *testing.T, db *database.Manager) *notification.Handler {
	t.Helper()
	logger := logging.GetDefault()
	emailConfig := notification.EmailSMTPConfig{
		Host:     "localhost",
		Port:     587,
		Username: "test",
		Password: "test",
		From:     "test@example.com",
		TLS:      false,
	}
	notificationService := notification.NewService(db.DB, logger, emailConfig)
	return notification.NewHandler(notificationService, logger)
}

func TestGetDevices(t *testing.T) {
	// Setup
	db := testutil.TestDatabase(t)
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	// Add test devices
	testDevices := []*database.Device{
		testutil.TestDevice(),
		{
			IP:       "192.168.1.101",
			MAC:      "B4:CF:12:34:56:78",
			Type:     "Relay Switch",
			Name:     "Test Switch",
			Firmware: "20231219-134356",
			Status:   "online",
			LastSeen: time.Now(),
			Settings: `{"model":"SHSW-1","gen":1}`,
		},
	}

	for _, device := range testDevices {
		err := db.AddDevice(device)
		testutil.AssertNoError(t, err)
	}

	// Create request
	req := httptest.NewRequest("GET", "/api/v1/devices", nil)
	w := httptest.NewRecorder()

	// Execute
	handler.GetDevices(w, req)

	// Assert
	testutil.AssertEqual(t, http.StatusOK, w.Code)
	testutil.AssertEqual(t, "application/json", w.Header().Get("Content-Type"))

	var devices []database.Device
	err := json.Unmarshal(w.Body.Bytes(), &devices)
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, 2, len(devices))
}

func TestAddDevice(t *testing.T) {
	// Setup
	db := testutil.TestDatabase(t)
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	device := testutil.TestDevice()
	deviceJSON, err := json.Marshal(device)
	testutil.AssertNoError(t, err)

	// Create request
	req := httptest.NewRequest("POST", "/api/v1/devices", bytes.NewReader(deviceJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	handler.AddDevice(w, req)

	// Assert
	testutil.AssertEqual(t, http.StatusCreated, w.Code)
	testutil.AssertEqual(t, "application/json", w.Header().Get("Content-Type"))

	var returnedDevice database.Device
	err = json.Unmarshal(w.Body.Bytes(), &returnedDevice)
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, device.MAC, returnedDevice.MAC)
	testutil.AssertEqual(t, device.Name, returnedDevice.Name)

	// Verify device was actually added to database
	devices, err := db.GetDevices()
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, 1, len(devices))
}

func TestAddDevice_InvalidJSON(t *testing.T) {
	db := testutil.TestDatabase(t)
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	// Create request with invalid JSON
	req := httptest.NewRequest("POST", "/api/v1/devices", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	handler.AddDevice(w, req)

	// Assert
	testutil.AssertEqual(t, http.StatusBadRequest, w.Code)
}

func TestGetDevice(t *testing.T) {
	// Setup
	db := testutil.TestDatabase(t)
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	device := testutil.TestDevice()
	err := db.AddDevice(device)
	testutil.AssertNoError(t, err)

	// Create request with device ID
	req := httptest.NewRequest("GET", "/api/v1/devices/"+strconv.Itoa(int(device.ID)), nil)
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(device.ID))})
	w := httptest.NewRecorder()

	// Execute
	handler.GetDevice(w, req)

	// Assert
	testutil.AssertEqual(t, http.StatusOK, w.Code)
	testutil.AssertEqual(t, "application/json", w.Header().Get("Content-Type"))

	var returnedDevice database.Device
	err = json.Unmarshal(w.Body.Bytes(), &returnedDevice)
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, device.ID, returnedDevice.ID)
	testutil.AssertEqual(t, device.MAC, returnedDevice.MAC)
}

func TestGetDevice_NotFound(t *testing.T) {
	db := testutil.TestDatabase(t)
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	// Create request with non-existent device ID
	req := httptest.NewRequest("GET", "/api/v1/devices/999", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "999"})
	w := httptest.NewRecorder()

	// Execute
	handler.GetDevice(w, req)

	// Assert
	testutil.AssertEqual(t, http.StatusNotFound, w.Code)
}

func TestGetDevice_InvalidID(t *testing.T) {
	db := testutil.TestDatabase(t)
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	// Create request with invalid device ID
	req := httptest.NewRequest("GET", "/api/v1/devices/invalid", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "invalid"})
	w := httptest.NewRecorder()

	// Execute
	handler.GetDevice(w, req)

	// Assert
	testutil.AssertEqual(t, http.StatusBadRequest, w.Code)
}

func TestUpdateDevice(t *testing.T) {
	// Setup
	db := testutil.TestDatabase(t)
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	device := testutil.TestDevice()
	err := db.AddDevice(device)
	testutil.AssertNoError(t, err)

	// Prepare updated device
	updatedDevice := *device
	updatedDevice.Name = "Updated Device Name"
	updatedDevice.Status = "offline"

	deviceJSON, err := json.Marshal(updatedDevice)
	testutil.AssertNoError(t, err)

	// Create request
	req := httptest.NewRequest("PUT", "/api/v1/devices/"+strconv.Itoa(int(device.ID)), bytes.NewReader(deviceJSON))
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(device.ID))})
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	handler.UpdateDevice(w, req)

	// Assert
	testutil.AssertEqual(t, http.StatusOK, w.Code)

	var returnedDevice database.Device
	err = json.Unmarshal(w.Body.Bytes(), &returnedDevice)
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, "Updated Device Name", returnedDevice.Name)
	testutil.AssertEqual(t, "offline", returnedDevice.Status)

	// Verify device was actually updated in database
	dbDevice, err := db.GetDevice(device.ID)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, "Updated Device Name", dbDevice.Name)
}

func TestUpdateDevice_NotFound(t *testing.T) {
	db := testutil.TestDatabase(t)
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	device := testutil.TestDevice()
	deviceJSON, err := json.Marshal(device)
	testutil.AssertNoError(t, err)

	// Create request with non-existent device ID
	req := httptest.NewRequest("PUT", "/api/v1/devices/999", bytes.NewReader(deviceJSON))
	req = mux.SetURLVars(req, map[string]string{"id": "999"})
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	handler.UpdateDevice(w, req)

	// Assert
	testutil.AssertEqual(t, http.StatusNotFound, w.Code)
}

func TestDeleteDevice(t *testing.T) {
	// Setup
	db := testutil.TestDatabase(t)
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	device := testutil.TestDevice()
	err := db.AddDevice(device)
	testutil.AssertNoError(t, err)

	// Create request
	req := httptest.NewRequest("DELETE", "/api/v1/devices/"+strconv.Itoa(int(device.ID)), nil)
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(device.ID))})
	w := httptest.NewRecorder()

	// Execute
	handler.DeleteDevice(w, req)

	// Assert
	testutil.AssertEqual(t, http.StatusNoContent, w.Code)

	// Verify device was actually deleted from database
	devices, err := db.GetDevices()
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, 0, len(devices))
}

func TestDeleteDevice_NotFound(t *testing.T) {
	db := testutil.TestDatabase(t)
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	// Create request with non-existent device ID
	req := httptest.NewRequest("DELETE", "/api/v1/devices/999", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "999"})
	w := httptest.NewRecorder()

	// Execute
	handler.DeleteDevice(w, req)

	// Assert - Note: GORM Delete doesn't return error for non-existent records
	testutil.AssertEqual(t, http.StatusNoContent, w.Code)
}

func TestDiscoverHandler(t *testing.T) {
	db := testutil.TestDatabase(t)
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	req := httptest.NewRequest("POST", "/api/v1/discover", nil)
	w := httptest.NewRecorder()

	// Execute
	handler.DiscoverHandler(w, req)

	// Assert
	testutil.AssertEqual(t, http.StatusOK, w.Code)
	testutil.AssertEqual(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	testutil.AssertNoError(t, err)

	status, exists := response["status"]
	if !exists || status != "discovery_started" {
		t.Errorf("Expected status 'discovery_started', got %v", status)
	}
}

func TestGetProvisioningStatus(t *testing.T) {
	db := testutil.TestDatabase(t)
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	req := httptest.NewRequest("GET", "/api/v1/provisioning/status", nil)
	w := httptest.NewRecorder()

	// Execute
	handler.GetProvisioningStatus(w, req)

	// Assert
	testutil.AssertEqual(t, http.StatusOK, w.Code)
	testutil.AssertEqual(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	testutil.AssertNoError(t, err)

	status, exists := response["status"]
	if !exists || status != "idle" {
		t.Errorf("Expected status 'idle', got %v", status)
	}
}

func TestProvisionDevices(t *testing.T) {
	db := testutil.TestDatabase(t)
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	req := httptest.NewRequest("POST", "/api/v1/provisioning/provision", nil)
	w := httptest.NewRecorder()

	// Execute
	handler.ProvisionDevices(w, req)

	// Assert
	testutil.AssertEqual(t, http.StatusOK, w.Code)
	testutil.AssertEqual(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	testutil.AssertNoError(t, err)

	status, exists := response["status"]
	if !exists || status != "provisioning_started" {
		t.Errorf("Expected status 'provisioning_started', got %v", status)
	}
}

func TestGetDHCPReservations(t *testing.T) {
	db := testutil.TestDatabase(t)
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	req := httptest.NewRequest("GET", "/api/v1/dhcp/reservations", nil)
	w := httptest.NewRecorder()

	// Execute
	handler.GetDHCPReservations(w, req)

	// Assert
	testutil.AssertEqual(t, http.StatusOK, w.Code)
	testutil.AssertEqual(t, "application/json", w.Header().Get("Content-Type"))

	var response []map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	testutil.AssertNoError(t, err)

	// Should return empty array for now
	testutil.AssertEqual(t, 0, len(response))
}

// Integration test for the full API router
func TestAPIRouter(t *testing.T) {
	db := testutil.TestDatabase(t)
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())
	router := SetupRoutes(handler)

	// Add a test device
	device := testutil.TestDevice()
	err := db.AddDevice(device)
	testutil.AssertNoError(t, err)

	// Test GET /api/v1/devices
	req := httptest.NewRequest("GET", "/api/v1/devices", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	testutil.AssertEqual(t, http.StatusOK, w.Code)

	// Test GET /api/v1/devices/{id}
	req = httptest.NewRequest("GET", fmt.Sprintf("/api/v1/devices/%d", device.ID), nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	testutil.AssertEqual(t, http.StatusOK, w.Code)

	// Test POST /api/v1/discover
	req = httptest.NewRequest("POST", "/api/v1/discover", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	testutil.AssertEqual(t, http.StatusOK, w.Code)

	// Test GET /api/v1/provisioning/status
	req = httptest.NewRequest("GET", "/api/v1/provisioning/status", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	testutil.AssertEqual(t, http.StatusOK, w.Code)

	// Test GET /api/v1/dhcp/reservations
	req = httptest.NewRequest("GET", "/api/v1/dhcp/reservations", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	testutil.AssertEqual(t, http.StatusOK, w.Code)
}

func TestControlDevice(t *testing.T) {
	// Setup
	db := testutil.TestDatabase(t)
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	// Add a test device
	device := testutil.TestDevice()
	err := db.AddDevice(device)
	testutil.AssertNoError(t, err)

	// Test valid control request
	controlReq := map[string]interface{}{
		"action": "toggle",
		"params": map[string]interface{}{"output": 0},
	}
	body, _ := json.Marshal(controlReq)

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/devices/%d/control", device.ID), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(device.ID))})

	w := httptest.NewRecorder()
	handler.ControlDevice(w, req)

	// Should get an error since we're not actually connecting to a device
	testutil.AssertEqual(t, http.StatusInternalServerError, w.Code)
}

func TestControlDevice_InvalidID(t *testing.T) {
	// Setup
	db := testutil.TestDatabase(t)
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	controlReq := map[string]interface{}{
		"action": "toggle",
	}
	body, _ := json.Marshal(controlReq)

	req := httptest.NewRequest("POST", "/api/v1/devices/invalid/control", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = mux.SetURLVars(req, map[string]string{"id": "invalid"})

	w := httptest.NewRecorder()
	handler.ControlDevice(w, req)

	testutil.AssertEqual(t, http.StatusBadRequest, w.Code)
}

func TestControlDevice_InvalidJSON(t *testing.T) {
	// Setup
	db := testutil.TestDatabase(t)
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	req := httptest.NewRequest("POST", "/api/v1/devices/1/control", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	req = mux.SetURLVars(req, map[string]string{"id": "1"})

	w := httptest.NewRecorder()
	handler.ControlDevice(w, req)

	testutil.AssertEqual(t, http.StatusBadRequest, w.Code)
}

func TestControlDevice_MissingAction(t *testing.T) {
	// Setup
	db := testutil.TestDatabase(t)
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	controlReq := map[string]interface{}{
		"params": map[string]interface{}{"output": 0},
	}
	body, _ := json.Marshal(controlReq)

	req := httptest.NewRequest("POST", "/api/v1/devices/1/control", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = mux.SetURLVars(req, map[string]string{"id": "1"})

	w := httptest.NewRecorder()
	handler.ControlDevice(w, req)

	testutil.AssertEqual(t, http.StatusBadRequest, w.Code)
}

func TestGetDeviceStatus(t *testing.T) {
	// Setup
	db := testutil.TestDatabase(t)
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	// Add a test device
	device := testutil.TestDevice()
	err := db.AddDevice(device)
	testutil.AssertNoError(t, err)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/devices/%d/status", device.ID), nil)
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(device.ID))})

	w := httptest.NewRecorder()
	handler.GetDeviceStatus(w, req)

	// Should get an error since we're not actually connecting to a device
	testutil.AssertEqual(t, http.StatusInternalServerError, w.Code)
}

func TestGetDeviceStatus_InvalidID(t *testing.T) {
	// Setup
	db := testutil.TestDatabase(t)
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	req := httptest.NewRequest("GET", "/api/v1/devices/invalid/status", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "invalid"})

	w := httptest.NewRecorder()
	handler.GetDeviceStatus(w, req)

	testutil.AssertEqual(t, http.StatusBadRequest, w.Code)
}

func TestGetDeviceEnergy(t *testing.T) {
	// Setup
	db := testutil.TestDatabase(t)
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	// Add a test device
	device := testutil.TestDevice()
	err := db.AddDevice(device)
	testutil.AssertNoError(t, err)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/devices/%d/energy", device.ID), nil)
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(device.ID))})

	w := httptest.NewRecorder()
	handler.GetDeviceEnergy(w, req)

	// Should get an error since we're not actually connecting to a device
	testutil.AssertEqual(t, http.StatusInternalServerError, w.Code)
}

func TestGetDeviceConfig(t *testing.T) {
	// Setup
	db := testutil.TestDatabase(t)
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	// Add a test device
	device := testutil.TestDevice()
	err := db.AddDevice(device)
	testutil.AssertNoError(t, err)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/devices/%d/config", device.ID), nil)
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(device.ID))})

	w := httptest.NewRecorder()
	handler.GetDeviceConfig(w, req)

	// Should get an error since we're not actually connecting to a device
	testutil.AssertEqual(t, http.StatusInternalServerError, w.Code)
}

func TestGetConfigTemplates(t *testing.T) {
	// Setup
	db := testutil.TestDatabase(t)
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	req := httptest.NewRequest("GET", "/api/v1/config/templates", nil)

	w := httptest.NewRecorder()
	handler.GetConfigTemplates(w, req)

	testutil.AssertEqual(t, http.StatusOK, w.Code)

	// Verify response is valid JSON
	var templates []interface{}
	err := json.Unmarshal(w.Body.Bytes(), &templates)
	testutil.AssertNoError(t, err)
}

func TestCreateConfigTemplate(t *testing.T) {
	// Setup
	db := testutil.TestDatabase(t)
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	templateReq := map[string]interface{}{
		"name":        "Test Template",
		"description": "Test template description",
		"deviceType":  "shelly-1",
		"config":      map[string]interface{}{"relay": map[string]interface{}{"auto_on": true}},
	}
	body, _ := json.Marshal(templateReq)

	req := httptest.NewRequest("POST", "/api/v1/config/templates", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.CreateConfigTemplate(w, req)

	testutil.AssertEqual(t, http.StatusCreated, w.Code)
}

func TestCreateConfigTemplate_InvalidJSON(t *testing.T) {
	// Setup
	db := testutil.TestDatabase(t)
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	req := httptest.NewRequest("POST", "/api/v1/config/templates", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.CreateConfigTemplate(w, req)

	testutil.AssertEqual(t, http.StatusBadRequest, w.Code)
}

func TestDetectConfigDrift(t *testing.T) {
	// Setup
	db := testutil.TestDatabase(t)
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	// Add a test device
	device := testutil.TestDevice()
	err := db.AddDevice(device)
	testutil.AssertNoError(t, err)

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/devices/%d/drift/detect", device.ID), nil)
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(device.ID))})

	w := httptest.NewRecorder()
	handler.DetectConfigDrift(w, req)

	// Should get an error since we're not actually connecting to a device
	testutil.AssertEqual(t, http.StatusInternalServerError, w.Code)
}

func TestBulkDetectConfigDrift(t *testing.T) {
	// Setup
	db := testutil.TestDatabase(t)
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	req := httptest.NewRequest("POST", "/api/v1/config/drift/detect/bulk", nil)

	w := httptest.NewRecorder()
	handler.BulkDetectConfigDrift(w, req)

	// Should succeed even without devices since it just tries to process all devices
	testutil.AssertEqual(t, http.StatusOK, w.Code)
}
