package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// The bulk routes act on the devices the request names, in the direction of the per-device route of
// the same name, and refuse a request that names none (ginsys/shelly-manager#248). Every device here
// is a fake server that counts the requests it gets: a device that must not be touched gets none.

// bulkRoutes are the four routes. The enhanced drift route answers a report, so its device list is
// not compared; every route is held to "touches only the named devices".
var bulkRoutes = []struct {
	name    string
	path    string
	call    func(*Handler, http.ResponseWriter, *http.Request)
	results bool // the answer lists one result per named device
}{
	{"import", "/api/v1/config/bulk-import", (*Handler).BulkImportConfigs, true},
	{"export", "/api/v1/config/bulk-export", (*Handler).BulkExportConfigs, true},
	{"drift", "/api/v1/config/bulk-drift-detect", (*Handler).BulkDetectConfigDrift, true},
	{"drift-enhanced", "/api/v1/config/bulk-drift-detect-enhanced", (*Handler).EnhancedBulkDetectConfigDrift, false},
}

type fleetDevice struct {
	device *database.Device
	hits   *atomic.Int32
}

// countingDevice is a Gen1 device that answers with the given MAC digits and counts every request.
func countingDevice(t *testing.T, mac string) (addr string, hits *atomic.Int32) {
	t.Helper()
	hits = new(atomic.Int32)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path == "/shelly" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"type": "SHPLG-S", "mac": mac, "auth": false, "fw": "1.14.0"})
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)
	return strings.TrimPrefix(server.URL, "http://"), hits
}

// bulkFleet stores n devices, each at its own counting fake.
func bulkFleet(t *testing.T, db *database.Manager, n int) []fleetDevice {
	t.Helper()
	fleet := make([]fleetDevice, 0, n)
	for i := 0; i < n; i++ {
		addr, hits := countingDevice(t, fmt.Sprintf("A4CF123456%02X", i))
		device := deviceAt(t, db, addr, fmt.Sprintf("A4:CF:12:34:56:%02X", i))
		fleet = append(fleet, fleetDevice{device: device, hits: hits})
	}
	return fleet
}

func postBulk(handler *Handler, call func(*Handler, http.ResponseWriter, *http.Request), path, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest("POST", path, nil)
	} else {
		req = httptest.NewRequest("POST", path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	call(handler, w, req)
	return w
}

func idsBody(ids ...uint) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprint(id)
	}
	return `{"device_ids":[` + strings.Join(parts, ",") + `]}`
}

// decodeData returns the data member of a success answer.
func decodeData(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), w.Body.String())
	return body.Data
}

func resultIDs(t *testing.T, data map[string]any) []uint {
	t.Helper()
	list, ok := data["results"].([]any)
	require.True(t, ok, "results is not a list: %v", data["results"])
	ids := make([]uint, 0, len(list))
	for _, item := range list {
		entry := item.(map[string]any)
		ids = append(ids, uint(entry["device_id"].(float64)))
	}
	return ids
}

func totalHits(fleet []fleetDevice) (total int32) {
	for _, f := range fleet {
		total += f.hits.Load()
	}
	return total
}

func TestBulkRoutes_ActOnlyOnTheNamedDevices(t *testing.T) {
	for _, route := range bulkRoutes {
		t.Run(route.name, func(t *testing.T) {
			db, cleanup := testutil.TestDatabase(t)
			defer cleanup()
			handler := exportHandler(t, db)
			fleet := bulkFleet(t, db, 3)

			w := postBulk(handler, route.call, route.path, idsBody(fleet[0].device.ID, fleet[2].device.ID))

			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			assert.Zero(t, fleet[1].hits.Load(), "the device that was not named was contacted")
			assert.NotZero(t, fleet[0].hits.Load(), "a named device was not contacted")
			assert.NotZero(t, fleet[2].hits.Load(), "a named device was not contacted")
			if route.results {
				assert.Equal(t, []uint{fleet[0].device.ID, fleet[2].device.ID}, resultIDs(t, decodeData(t, w)))
			}
		})
	}
}

func TestBulkRoutes_RefuseARequestThatNamesNoDevice(t *testing.T) {
	bodies := map[string]string{
		"no body":      "",
		"empty object": `{}`,
		"empty list":   `{"device_ids":[]}`,
		"null list":    `{"device_ids":null}`,
		"zero id":      `{"device_ids":[0]}`,
		"negative id":  `{"device_ids":[-1]}`,
		"not json":     `not json`,
		"two values":   `{"device_ids":[1]}{"device_ids":[2]}`,
	}
	for _, route := range bulkRoutes {
		for name, body := range bodies {
			t.Run(route.name+"/"+name, func(t *testing.T) {
				db, cleanup := testutil.TestDatabase(t)
				defer cleanup()
				handler := exportHandler(t, db)
				fleet := bulkFleet(t, db, 2)

				w := postBulk(handler, route.call, route.path, body)

				assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
				assert.Contains(t, w.Body.String(), "VALIDATION_FAILED")
				assert.Zero(t, totalHits(fleet), "a request that names no device touched the fleet")
			})
		}
	}
}

// An option or a field the server does not read is refused by name, never ignored.
func TestBulkRoutes_RefuseAFieldTheServerDoesNotRead(t *testing.T) {
	cases := []struct{ body, named string }{
		{`{"device_ids":[%d],"configurations":[{}]}`, "configurations"},
		{`{"deviceIds":[%d]}`, "deviceIds"},
		{`{"device_ids":[%d],"options":{"validate_only":true}}`, "validate_only"},
		{`{"device_ids":[%d],"options":{"stopOnError":true}}`, "stopOnError"},
	}
	for _, route := range bulkRoutes {
		for _, c := range cases {
			t.Run(route.name+"/"+c.named, func(t *testing.T) {
				db, cleanup := testutil.TestDatabase(t)
				defer cleanup()
				handler := exportHandler(t, db)
				fleet := bulkFleet(t, db, 2)

				w := postBulk(handler, route.call, route.path, fmt.Sprintf(c.body, fleet[0].device.ID))

				assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
				assert.Contains(t, w.Body.String(), c.named)
				assert.Zero(t, totalHits(fleet))
			})
		}
	}
}

// stop_on_error: after the first failed device the rest are reported skipped and are not contacted.
// An id that names no device is a failed device. Without the option the later devices still run.
func TestBulkRoutes_StopOnErrorSkipsTheRest(t *testing.T) {
	const unknownID = 9999
	failed := map[string]string{"import": "failed", "export": "failed", "drift": "error"}
	for _, route := range bulkRoutes {
		if !route.results {
			continue
		}
		t.Run(route.name, func(t *testing.T) {
			db, cleanup := testutil.TestDatabase(t)
			defer cleanup()
			handler := exportHandler(t, db)
			fleet := bulkFleet(t, db, 1)

			body := fmt.Sprintf(`{"device_ids":[%d,%d],"options":{"stop_on_error":true}}`, unknownID, fleet[0].device.ID)
			w := postBulk(handler, route.call, route.path, body)

			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			data := decodeData(t, w)
			entries := data["results"].([]any)
			require.Len(t, entries, 2)
			assert.Equal(t, failed[route.name], entries[0].(map[string]any)["status"])
			assert.Equal(t, "skipped", entries[1].(map[string]any)["status"])
			assert.EqualValues(t, 1, data["skipped"])
			assert.EqualValues(t, 2, data["total"])
			assert.Zero(t, fleet[0].hits.Load(), "a skipped device was contacted")

			// The same request without the option runs the later device.
			w = postBulk(handler, route.call, route.path, idsBody(unknownID, fleet[0].device.ID))
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			assert.NotZero(t, fleet[0].hits.Load(), "without stop_on_error the later device must run")
			assert.EqualValues(t, 0, decodeData(t, w)["skipped"])
		})
	}
}

// The enhanced route answers a report of the devices that ran. With stop_on_error the devices after
// the first error are not contacted and are not in the report, so its totals add up.
func TestBulkDriftEnhanced_StopOnErrorReportsOnlyTheDevicesThatRan(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	handler := exportHandler(t, db)
	fleet := bulkFleet(t, db, 1)

	body := fmt.Sprintf(`{"device_ids":[9999,%d],"options":{"stop_on_error":true}}`, fleet[0].device.ID)
	w := postBulk(handler, (*Handler).EnhancedBulkDetectConfigDrift, "/api/v1/config/bulk-drift-detect-enhanced", body)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	data := decodeData(t, w)
	summary, ok := data["summary"].(map[string]any)
	require.True(t, ok, "the answer is not the comprehensive report: %v", data)
	assert.EqualValues(t, 1, summary["total_devices"])
	assert.EqualValues(t, 1, summary["devices_errored"])
	assert.EqualValues(t, 0, summary["devices_in_sync"])
	assert.EqualValues(t, 0, summary["devices_drifted"])
	assert.Zero(t, fleet[0].hits.Load(), "a skipped device was contacted")
}

func TestBulkRoutes_ADuplicateIDRunsOnce(t *testing.T) {
	for _, route := range bulkRoutes {
		if !route.results {
			continue
		}
		t.Run(route.name, func(t *testing.T) {
			db, cleanup := testutil.TestDatabase(t)
			defer cleanup()
			handler := exportHandler(t, db)
			fleet := bulkFleet(t, db, 1)
			id := fleet[0].device.ID

			w := postBulk(handler, route.call, route.path, idsBody(id, id))

			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			data := decodeData(t, w)
			assert.EqualValues(t, 1, data["total"])
			assert.Equal(t, []uint{id}, resultIDs(t, data))
		})
	}
}

// The wire shape of the import and export answer: the names are the contract the UI reads.
func TestBulkImportExport_AnswerShape(t *testing.T) {
	for _, route := range bulkRoutes[:2] {
		t.Run(route.name, func(t *testing.T) {
			db, cleanup := testutil.TestDatabase(t)
			defer cleanup()
			handler := exportHandler(t, db)
			fleet := bulkFleet(t, db, 1)

			w := postBulk(handler, route.call, route.path, idsBody(fleet[0].device.ID, 9999))

			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			data := decodeData(t, w)
			for _, key := range []string{"total", "success", "failed", "skipped", "results"} {
				assert.Contains(t, data, key)
			}
			assert.NotContains(t, data, "errors", "the old name of failed")
			entries := data["results"].([]any)
			require.Len(t, entries, 2)
			known := entries[0].(map[string]any)
			for _, key := range []string{"device_id", "device_name", "device_ip", "status"} {
				assert.Contains(t, known, key)
			}
			unknown := entries[1].(map[string]any)
			assert.Equal(t, "failed", unknown["status"])
			assert.Contains(t, unknown["error"], "not found")
		})
	}
}
