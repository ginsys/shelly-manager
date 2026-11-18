package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gorilla/mux"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// storedMAC is the MAC of every device record in these tests, in the colon spelling a record may
// hold; a device reports 12 hex digits.
const (
	storedMAC     = "A4:CF:12:34:56:78"
	reportedMAC   = "A4CF12345678"
	otherDeviceID = "112233445566"
)

// gen1Device is a Gen1 device at a local address. It answers GET /shelly with the given MAC (or
// the given status when status is not 200) and counts every other request that is a write.
func gen1Device(t *testing.T, mac string, status int) (addr string, writes *atomic.Int32) {
	t.Helper()
	writes = new(atomic.Int32)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/shelly" {
			if status != http.StatusOK {
				http.Error(w, "unauthorized", status)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"type": "SHPLG-S", "mac": mac, "auth": false, "fw": "1.14.0"})
			return
		}
		if r.Method == http.MethodPost {
			writes.Add(1)
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)
	return strings.TrimPrefix(server.URL, "http://"), writes
}

// deviceAt stores a device record at addr, with a stored configuration.
func deviceAt(t *testing.T, db *database.Manager, addr, mac string) *database.Device {
	t.Helper()
	device := testutil.TestDevice()
	device.IP = addr
	device.MAC = mac
	testutil.AssertNoError(t, db.AddDevice(device))
	testutil.AssertNoError(t, db.GetDB().Create(&configuration.DeviceConfig{
		DeviceID: device.ID, Config: json.RawMessage(`{"name":"exported"}`), SyncStatus: "pending",
	}).Error)
	return device
}

func exportHandler(t *testing.T, db *database.Manager) *Handler {
	t.Helper()
	return NewHandlerWithLogger(db, testShellyService(t, db), testNotificationHandler(t, db), nil, logging.GetDefault())
}

func postControl(handler *Handler, id uint, action string) *httptest.ResponseRecorder {
	body := strings.NewReader(`{"action":"` + action + `","force":true}`)
	req := httptest.NewRequest("POST", "/api/v1/devices/"+strconv.Itoa(int(id))+"/control", body)
	req = mux.SetURLVars(req, map[string]string{"id": strconv.Itoa(int(id))})
	w := httptest.NewRecorder()
	handler.ControlDevice(w, req)
	return w
}

// A switch or reboot for the device at a stale address does not reach the device that answers
// there: 409 with the reason, not a 200 and not a hidden 500.
func TestControlDevice_RefusesADeviceWithAnotherMAC(t *testing.T) {
	for _, action := range []string{"on", "off", "reboot"} {
		t.Run(action, func(t *testing.T) {
			db, cleanup := testutil.TestDatabase(t)
			defer cleanup()
			handler := exportHandler(t, db)
			addr, writes := gen1Device(t, otherDeviceID, http.StatusOK)
			device := deviceAt(t, db, addr, storedMAC)

			w := postControl(handler, device.ID, action)

			testutil.AssertEqual(t, http.StatusConflict, w.Code)
			if !strings.Contains(w.Body.String(), "stored MAC") {
				t.Errorf("the 409 should say what differs, got %s", w.Body.String())
			}
			testutil.AssertEqual(t, int32(0), writes.Load())
		})
	}
}

func TestControlDevice_ControlsTheStoredDevice(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	handler := exportHandler(t, db)
	addr, writes := gen1Device(t, reportedMAC, http.StatusOK)
	device := deviceAt(t, db, addr, storedMAC)

	w := postControl(handler, device.ID, "on")

	testutil.AssertEqual(t, http.StatusOK, w.Code)
	testutil.AssertEqual(t, int32(1), writes.Load())
}
