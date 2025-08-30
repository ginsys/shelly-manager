package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/notification"
	"github.com/ginsys/shelly-manager/internal/service"
	"github.com/ginsys/shelly-manager/internal/testutil"
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
	notificationService := notification.NewService(db.GetDB(), logger, emailConfig)
	return notification.NewHandler(notificationService, logger)
}

func TestGetDevices(t *testing.T) {
	// Setup
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
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
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Expected application/json Content-Type, got %s", ct)
	}

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	testutil.AssertNoError(t, err)

	// Check standardized response structure
	testutil.AssertEqual(t, true, response["success"])
	data, ok := response["data"].(map[string]interface{})
	testutil.AssertEqual(t, true, ok)
	devicesInterface, exists := data["devices"]
	testutil.AssertEqual(t, true, exists)

	// Convert devices array to check length
	devicesArray, ok := devicesInterface.([]interface{})
	testutil.AssertEqual(t, true, ok)
	testutil.AssertEqual(t, 2, len(devicesArray))
}

func TestAddDevice(t *testing.T) {
	// Setup
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
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
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Expected application/json Content-Type, got %s", ct)
	}

	var wrap map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &wrap)
	testutil.AssertNoError(t, err)
	dataMap, ok := wrap["data"].(map[string]interface{})
	testutil.AssertEqual(t, true, ok)
	dataJSON, _ := json.Marshal(dataMap)
	var returnedDevice database.Device
	err = json.Unmarshal(dataJSON, &returnedDevice)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, device.MAC, returnedDevice.MAC)
	testutil.AssertEqual(t, device.Name, returnedDevice.Name)

	// Verify device was actually added to database
	devices, err := db.GetDevices()
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, 1, len(devices))
}

func TestAddDevice_InvalidJSON(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
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
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
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
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Expected application/json Content-Type, got %s", ct)
	}

	var wrap map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &wrap)
	testutil.AssertNoError(t, err)
	dataMap, ok := wrap["data"].(map[string]interface{})
	testutil.AssertEqual(t, true, ok)
	dataJSON, _ := json.Marshal(dataMap)
	var returnedDevice database.Device
	err = json.Unmarshal(dataJSON, &returnedDevice)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, device.ID, returnedDevice.ID)
	testutil.AssertEqual(t, device.MAC, returnedDevice.MAC)
}

func TestGetDevice_NotFound(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
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
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
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
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
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

	var wrap map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &wrap)
	testutil.AssertNoError(t, err)
	dataMap, ok := wrap["data"].(map[string]interface{})
	testutil.AssertEqual(t, true, ok)
	dataJSON, _ := json.Marshal(dataMap)
	var returnedDevice database.Device
	err = json.Unmarshal(dataJSON, &returnedDevice)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, "Updated Device Name", returnedDevice.Name)
	testutil.AssertEqual(t, "offline", returnedDevice.Status)

	// Verify device was actually updated in database
	dbDevice, err := db.GetDevice(device.ID)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, "Updated Device Name", dbDevice.Name)
}

func TestUpdateDevice_NotFound(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
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
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
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
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
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
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	req := httptest.NewRequest("POST", "/api/v1/discover", nil)
	w := httptest.NewRecorder()

	// Execute
	handler.DiscoverHandler(w, req)

	// Assert
	testutil.AssertEqual(t, http.StatusOK, w.Code)
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Expected application/json Content-Type, got %s", ct)
	}

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	testutil.AssertNoError(t, err)
	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data wrapper in response: %s", w.Body.String())
	}
	status, exists := data["status"]
	if !exists || status != "discovery_started" {
		t.Errorf("Expected status 'discovery_started', got %v", status)
	}
}

func TestGetProvisioningStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network test in short mode")
	}

	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	req := httptest.NewRequest("GET", "/api/v1/provisioning/status", nil)
	w := httptest.NewRecorder()

	// Execute
	handler.GetProvisioningStatus(w, req)

	// Assert
	testutil.AssertEqual(t, http.StatusOK, w.Code)
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Expected application/json Content-Type, got %s", ct)
	}

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	testutil.AssertNoError(t, err)

	status, exists := response["status"]
	if !exists || status != "idle" {
		t.Errorf("Expected status 'idle', got %v", status)
	}
}

func TestProvisionDevices(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
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
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
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
	if testing.Short() {
		t.Skip("Skipping network test in short mode")
	}

	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
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
	if testing.Short() {
		t.Skip("Skipping network test in short mode")
	}

	// Setup
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
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
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
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
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
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
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
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
	if testing.Short() {
		t.Skip("Skipping network test in short mode")
	}

	// Setup
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
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
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
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
	if testing.Short() {
		t.Skip("Skipping network test in short mode")
	}

	// Setup
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
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
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
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

func TestGetCurrentDeviceConfig(t *testing.T) {
	// Setup
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	// Add a test device
	device := testutil.TestDevice()
	err := db.AddDevice(device)
	testutil.AssertNoError(t, err)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/devices/%d/config/current", device.ID), nil)
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(device.ID))})

	w := httptest.NewRecorder()
	handler.GetCurrentDeviceConfig(w, req)

	// Should get JSON response (allow charset suffix)
	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Expected Content-Type to start with application/json, got %s", ct)
	}

	// Decode the response to check structure
	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	testutil.AssertNoError(t, err)

	// Should have success field (will be false due to service error, but that's expected)
	_, hasSuccess := response["success"]
	testutil.AssertEqual(t, true, hasSuccess)
}

func TestGetConfigTemplates(t *testing.T) {
	// Setup
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	req := httptest.NewRequest("GET", "/api/v1/config/templates", nil)

	w := httptest.NewRecorder()
	handler.GetConfigTemplates(w, req)

	testutil.AssertEqual(t, http.StatusOK, w.Code)

	// Verify standardized response wrapper with data array
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	testutil.AssertNoError(t, err)
	if _, ok := resp["success"].(bool); !ok {
		t.Fatalf("Expected success field in response")
	}
	// data should be an array (possibly empty)
	if _, ok := resp["data"].([]interface{}); !ok {
		t.Fatalf("Expected data to be an array, got %T", resp["data"])
	}
}

func TestCreateConfigTemplate(t *testing.T) {
	// Setup
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
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
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
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
	if testing.Short() {
		t.Skip("Skipping network test in short mode")
	}

	// Setup
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
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
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	req := httptest.NewRequest("POST", "/api/v1/config/drift/detect/bulk", nil)

	w := httptest.NewRecorder()
	handler.BulkDetectConfigDrift(w, req)

	// Should succeed even without devices since it just tries to process all devices
	testutil.AssertEqual(t, http.StatusOK, w.Code)
}

func TestNewHandler(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)

	handler := NewHandler(db, svc, notificationHandler)

	// Verify handler is properly initialized
	testutil.AssertNotNil(t, handler)
	testutil.AssertNotNil(t, handler.DB)
	testutil.AssertNotNil(t, handler.Service)
	testutil.AssertNotNil(t, handler.ConfigService)
}

func TestGetProvisioningStatusNew(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	req := httptest.NewRequest("GET", "/api/v1/provisioning/status", nil)
	w := httptest.NewRecorder()

	handler.GetProvisioningStatus(w, req)

	testutil.AssertEqual(t, http.StatusOK, w.Code)
	testutil.AssertEqual(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	testutil.AssertNoError(t, err)

	status, exists := response["status"]
	testutil.AssertTrue(t, exists)
	testutil.AssertEqual(t, "idle", status)
}

func TestGetDeviceEnergy_InvalidID(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	req := httptest.NewRequest("GET", "/api/v1/devices/invalid/energy", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "invalid"})

	w := httptest.NewRecorder()
	handler.GetDeviceEnergy(w, req)

	testutil.AssertEqual(t, http.StatusBadRequest, w.Code)
}

func TestImportDeviceConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network test in short mode")
	}

	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	// Add a test device
	device := testutil.TestDevice()
	err := db.AddDevice(device)
	testutil.AssertNoError(t, err)

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/devices/%d/config/import", device.ID), nil)
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(device.ID))})

	w := httptest.NewRecorder()
	handler.ImportDeviceConfig(w, req)

	// Should get an error since we're not actually connecting to a device
	testutil.AssertEqual(t, http.StatusInternalServerError, w.Code)
}

func TestImportDeviceConfig_InvalidID(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	req := httptest.NewRequest("POST", "/api/v1/devices/invalid/config/import", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "invalid"})

	w := httptest.NewRecorder()
	handler.ImportDeviceConfig(w, req)

	testutil.AssertEqual(t, http.StatusBadRequest, w.Code)
}

func TestGetImportStatus(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	// Add a test device
	device := testutil.TestDevice()
	err := db.AddDevice(device)
	testutil.AssertNoError(t, err)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/devices/%d/config/import/status", device.ID), nil)
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(device.ID))})

	w := httptest.NewRecorder()
	handler.GetImportStatus(w, req)

	testutil.AssertEqual(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	testutil.AssertNoError(t, err)
	data, ok := response["data"].(map[string]interface{})
	testutil.AssertTrue(t, ok)
	status, exists := data["status"]
	testutil.AssertTrue(t, exists)
	testutil.AssertEqual(t, "not_imported", status) // Device has no config yet, so "not_imported" is correct
}

func TestExportDeviceConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network test in short mode")
	}

	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	// Add a test device
	device := testutil.TestDevice()
	err := db.AddDevice(device)
	testutil.AssertNoError(t, err)

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/devices/%d/config/export", device.ID), nil)
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(device.ID))})

	w := httptest.NewRecorder()
	handler.ExportDeviceConfig(w, req)

	// Should get an error since we're not actually connecting to a device
	testutil.AssertEqual(t, http.StatusInternalServerError, w.Code)
}

func TestBulkImportConfigs(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	req := httptest.NewRequest("POST", "/api/v1/config/import/bulk", nil)
	w := httptest.NewRecorder()

	handler.BulkImportConfigs(w, req)

	testutil.AssertEqual(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	testutil.AssertNoError(t, err)
	data, ok := response["data"].(map[string]interface{})
	testutil.AssertEqual(t, true, ok)

	// Check that response has expected structure inside data
	total, totalExists := data["total"]
	testutil.AssertTrue(t, totalExists)
	testutil.AssertEqual(t, 0, int(total.(float64))) // No devices, so total should be 0
	successCount, successExists := data["success"]
	testutil.AssertTrue(t, successExists)
	testutil.AssertEqual(t, 0, int(successCount.(float64)))

	errors, errorsExists := data["errors"]
	testutil.AssertTrue(t, errorsExists)
	testutil.AssertEqual(t, 0, int(errors.(float64)))

	results, resultsExists := data["results"]
	testutil.AssertTrue(t, resultsExists)
	testutil.AssertEqual(t, 0, len(results.([]interface{})))
}

func TestBulkExportConfigs(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	req := httptest.NewRequest("POST", "/api/v1/config/export/bulk", nil)
	w := httptest.NewRecorder()

	handler.BulkExportConfigs(w, req)

	testutil.AssertEqual(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	testutil.AssertNoError(t, err)
	data, ok := response["data"].(map[string]interface{})
	testutil.AssertEqual(t, true, ok)

	// Check that response has expected structure inside data
	total, totalExists := data["total"]
	testutil.AssertTrue(t, totalExists)
	testutil.AssertEqual(t, 0, int(total.(float64))) // No devices, so total should be 0

	successCount, successExists := data["success"]
	testutil.AssertTrue(t, successExists)
	testutil.AssertEqual(t, 0, int(successCount.(float64)))

	errors, errorsExists := data["errors"]
	testutil.AssertTrue(t, errorsExists)
	testutil.AssertEqual(t, 0, int(errors.(float64)))

	results, resultsExists := data["results"]
	testutil.AssertTrue(t, resultsExists)
	testutil.AssertEqual(t, 0, len(results.([]interface{})))
}

func TestDetectConfigDrift_InvalidID(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	req := httptest.NewRequest("POST", "/api/v1/devices/invalid/drift/detect", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "invalid"})

	w := httptest.NewRecorder()
	handler.DetectConfigDrift(w, req)

	testutil.AssertEqual(t, http.StatusBadRequest, w.Code)
}

func TestUpdateConfigTemplate(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	// First create a template
	template := &configuration.ConfigTemplate{
		Name:        "Test Template",
		Description: "Test Description",
		DeviceType:  "shelly-1",
		Config:      []byte(`{"relay": {"auto_on": true}}`),
	}
	err := db.GetDB().Create(template).Error
	testutil.AssertNoError(t, err)

	// Update the template
	updateReq := map[string]interface{}{
		"name":        "Updated Template",
		"description": "Updated Description",
		"deviceType":  "shelly-1",
		"config":      map[string]interface{}{"relay": map[string]interface{}{"auto_on": false}},
	}
	body, _ := json.Marshal(updateReq)

	req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/config/templates/%d", template.ID), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(template.ID))})

	w := httptest.NewRecorder()
	handler.UpdateConfigTemplate(w, req)

	testutil.AssertEqual(t, http.StatusOK, w.Code)

	// Verify template was updated
	var updatedTemplate configuration.ConfigTemplate
	err = db.GetDB().First(&updatedTemplate, template.ID).Error
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, "Updated Template", updatedTemplate.Name)
}

func TestDeleteConfigTemplate(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	// First create a template
	template := &configuration.ConfigTemplate{
		Name:        "Test Template",
		Description: "Test Description",
		DeviceType:  "shelly-1",
		Config:      []byte(`{"relay": {"auto_on": true}}`),
	}
	err := db.GetDB().Create(template).Error
	testutil.AssertNoError(t, err)

	req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/config/templates/%d", template.ID), nil)
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(template.ID))})

	w := httptest.NewRecorder()
	handler.DeleteConfigTemplate(w, req)

	testutil.AssertEqual(t, http.StatusNoContent, w.Code)

	// Verify template was deleted
	var deletedTemplate configuration.ConfigTemplate
	err = db.GetDB().First(&deletedTemplate, template.ID).Error
	testutil.AssertTrue(t, err != nil) // Should not be found
}

func TestApplyConfigTemplate(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	// Create a template
	template := &configuration.ConfigTemplate{
		Name:        "Test Template",
		Description: "Test Description",
		DeviceType:  "shelly-1",
		Config:      []byte(`{"relay": {"auto_on": true}}`),
	}
	err := db.GetDB().Create(template).Error
	testutil.AssertNoError(t, err)

	// Add a test device
	device := testutil.TestDevice()
	err = db.AddDevice(device)
	testutil.AssertNoError(t, err)

	// Use the correct request format for the handler
	applyReq := map[string]interface{}{
		"template_id": template.ID,
		"variables":   map[string]interface{}{},
	}
	body, _ := json.Marshal(applyReq)

	// Use the correct endpoint - devices/{id}/config/apply-template
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/devices/%d/config/apply-template", device.ID), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(device.ID))})

	w := httptest.NewRecorder()
	handler.ApplyConfigTemplate(w, req)

	// Should get an error since we're not actually connecting to a device, but 500 instead of success
	testutil.AssertEqual(t, http.StatusInternalServerError, w.Code)
}

func TestGetConfigHistory(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	svc := testShellyService(t, db)
	notificationHandler := testNotificationHandler(t, db)
	handler := NewHandlerWithLogger(db, svc, notificationHandler, nil, logging.GetDefault())

	// Add a test device
	device := testutil.TestDevice()
	err := db.AddDevice(device)
	testutil.AssertNoError(t, err)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/devices/%d/config/history", device.ID), nil)
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(device.ID))})

	w := httptest.NewRecorder()
	handler.GetConfigHistory(w, req)

	testutil.AssertEqual(t, http.StatusOK, w.Code)

	var wrap map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &wrap)
	testutil.AssertNoError(t, err)
	_, ok := wrap["data"].([]interface{})
	testutil.AssertTrue(t, ok)
}
