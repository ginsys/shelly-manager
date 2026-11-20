package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// A bulk read stops starting devices before the request deadline, so the answer is written and names the
// devices it did not read, instead of a 408 with no result for a run that went on (ginsys/shelly-manager#730).
// Every device is a fake that counts its requests; the first request of a device takes `delay`, so one
// device costs about `delay` in all.

func slowDevice(t *testing.T, mac string, delay time.Duration) (addr string, hits *atomic.Int32) {
	t.Helper()
	hits = new(atomic.Int32)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			time.Sleep(delay)
		}
		if r.URL.Path == "/shelly" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"type": "SHPLG-S", "mac": mac, "auth": false, "fw": "1.14.0"})
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)
	return strings.TrimPrefix(server.URL, "http://"), hits
}

func delayedFleet(t *testing.T, db *database.Manager, n int, delay time.Duration) []fleetDevice {
	t.Helper()
	fleet := make([]fleetDevice, 0, n)
	for i := 0; i < n; i++ {
		addr, hits := slowDevice(t, fmt.Sprintf("A4CF123456%02X", i), delay)
		device := deviceAt(t, db, addr, fmt.Sprintf("A4:CF:12:34:56:%02X", i))
		require.NoError(t, db.GetDB().Model(&database.Device{}).Where("id = ?", device.ID).
			Update("desired_config", `{"system":{"device":{"name":"fleet"}}}`).Error)
		fleet = append(fleet, fleetDevice{device: device, hits: hits})
	}
	return fleet
}

func fleetIDs(fleet []fleetDevice) []uint {
	ids := make([]uint, len(fleet))
	for i, f := range fleet {
		ids[i] = f.device.ID
	}
	return ids
}

// postBulkWithin is postBulk with a request that carries the given context.
func postBulkWithin(ctx context.Context, handler *Handler, call func(*Handler, http.ResponseWriter, *http.Request), path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", path, strings.NewReader(body)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	call(handler, w, req)
	return w
}

// shortBudget gives bulk reads a margin of 1 s, so a request deadline of 1.5 s leaves a budget of 0.5 s.
func shortBudget(t *testing.T) {
	t.Helper()
	old := bulkReadMargin
	bulkReadMargin = time.Second
	t.Cleanup(func() { bulkReadMargin = old })
}

func entryStatuses(t *testing.T, data map[string]any) (statuses []string, reasons []string) {
	t.Helper()
	list, ok := data["results"].([]any)
	require.True(t, ok, "results is not a list: %v", data["results"])
	for _, item := range list {
		entry := item.(map[string]any)
		statuses = append(statuses, fmt.Sprint(entry["status"]))
		reasons = append(reasons, fmt.Sprint(entry["error"]))
	}
	return statuses, reasons
}

func TestBulkRoutes_StopStartingDevicesBeforeTheRequestDeadline(t *testing.T) {
	for _, route := range bulkRoutes {
		for _, stopOnError := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stop_on_error=%v", route.name, stopOnError), func(t *testing.T) {
				shortBudget(t)
				db, cleanup := testutil.TestDatabase(t)
				defer cleanup()
				handler := exportHandler(t, db)
				fleet := delayedFleet(t, db, 6, 300*time.Millisecond)
				body := fmt.Sprintf(`{"device_ids":[%s],"options":{"stop_on_error":%v}}`,
					strings.Trim(strings.Join(strings.Fields(fmt.Sprint(fleetIDs(fleet))), ","), "[]"), stopOnError)

				deadline := 1500 * time.Millisecond
				ctx, cancel := context.WithTimeout(context.Background(), deadline)
				defer cancel()
				start := time.Now()
				w := postBulkWithin(ctx, handler, route.call, route.path, body)
				elapsed := time.Since(start)

				require.Equal(t, http.StatusOK, w.Code, w.Body.String())
				assert.Less(t, elapsed, deadline, "the answer was written after the request deadline")
				data := decodeData(t, w)
				assert.NotContains(t, data, "summary", "a time-limited run answers the basic result, not the report")

				statuses, reasons := entryStatuses(t, data)
				require.Len(t, statuses, 6, "every named device has an entry")
				ran, skipped := 0, 0
				for i, status := range statuses {
					switch status {
					case "skipped":
						skipped++
						assert.Zero(t, fleet[i].hits.Load(), "a device reported skipped was contacted")
						assert.Equal(t, configuration.ReasonCallEnded, reasons[i], "a skipped device says why")
					case "success", "synced", "drift": // a device that was read
						ran++
						assert.NotZero(t, fleet[i].hits.Load(), "a device reported read was not contacted")
					default:
						t.Errorf("device %d: unexpected status %q (%s)", i, status, reasons[i])
					}
				}
				assert.GreaterOrEqual(t, ran, 1, "the budget leaves room for the first device")
				assert.GreaterOrEqual(t, skipped, 3, "the devices past the budget are not started")
			})
		}
	}
}

func TestBulkRoutes_SkipEveryDeviceWhenTheClientHasLeft(t *testing.T) {
	for _, route := range bulkRoutes {
		t.Run(route.name, func(t *testing.T) {
			db, cleanup := testutil.TestDatabase(t)
			defer cleanup()
			handler := exportHandler(t, db)
			fleet := bulkFleet(t, db, 3)

			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			w := postBulkWithin(ctx, handler, route.call, route.path, idsBody(fleetIDs(fleet)...))

			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			statuses, reasons := entryStatuses(t, decodeData(t, w))
			assert.Equal(t, []string{"skipped", "skipped", "skipped"}, statuses)
			for i := range reasons {
				assert.Equal(t, configuration.ReasonCallEnded, reasons[i])
			}
			assert.Zero(t, totalHits(fleet), "a device was contacted for a client that had left")
		})
	}
}

func TestBulkRoutes_ReadEveryDeviceWhenTheRequestHasNoDeadline(t *testing.T) {
	for _, route := range bulkRoutes {
		t.Run(route.name, func(t *testing.T) {
			shortBudget(t)
			db, cleanup := testutil.TestDatabase(t)
			defer cleanup()
			handler := exportHandler(t, db)
			fleet := delayedFleet(t, db, 3, 100*time.Millisecond)

			w := postBulkWithin(context.Background(), handler, route.call, route.path, idsBody(fleetIDs(fleet)...))

			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			for i, f := range fleet {
				assert.NotZero(t, f.hits.Load(), "device %d was not contacted", i)
			}
			if route.results {
				statuses, _ := entryStatuses(t, decodeData(t, w))
				assert.NotContains(t, statuses, "skipped")
			}
		})
	}
}

// The enhanced route answers the report when no device was skipped for time (the stop_on_error skip
// already leaves the skipped devices out of the report, by the operator's own choice).
func TestEnhancedBulkDrift_AnswersTheReportWhenNothingWasSkippedForTime(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	handler := exportHandler(t, db)
	fleet := bulkFleet(t, db, 2)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	w := postBulkWithin(ctx, handler, (*Handler).EnhancedBulkDetectConfigDrift, "/api/v1/config/bulk-drift-detect-enhanced", idsBody(fleetIDs(fleet)...))

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, decodeData(t, w), "summary")
}
