package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/api/middleware"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// A bulk-apply writes devices one after another, so it outlasts the 30 s that every protected route
// is given and, cut there, ended in a 408 with no result after some devices were written
// (ginsys/shelly-manager#725). These tests give the whole chain a short default deadline and
// devices that are slow to take a write.

// slowFleet adds n devices that each keep the name they are given and take delay for every write.
func slowFleet(t *testing.T, db *database.Manager, n int, delay time.Duration, onWrite func(i int)) []uint {
	t.Helper()
	ids := make([]uint, n)
	for i := range ids {
		// A record has one MAC, and the device at its address must report the same one.
		addr := settingsDevice(t, fmt.Sprintf("A4CF123456%02X", i), func() {
			if onWrite != nil {
				onWrite(i)
			}
			time.Sleep(delay)
		})
		ids[i] = deviceAt(t, db, addr, fmt.Sprintf("A4:CF:12:34:56:%02X", i)).ID
	}
	return ids
}

// postApplyWith sends the request to the handler on ctx, which the test may end at any moment.
func postApplyWith(ctx context.Context, handler *Handler, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, bulkApplyPath, strings.NewReader(body)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.BulkApplyConfigs(w, req)
	return w
}

func overridesNameTheBulkName(t *testing.T, db *database.Manager, id uint) bool {
	t.Helper()
	return strings.Contains(storedOverrides(t, db, id), "bulk-name")
}

// Through the whole production chain, a run that takes longer than the default deadline answers
// with every device's status. On main the middleware cut it at the default and answered 408.
func TestBulkApply_ARunPastTheDefaultDeadlineStillAnswersEveryDevice(t *testing.T) {
	t.Setenv(testModeEnv, "")
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	handler := exportHandler(t, db)
	ids := slowFleet(t, db, 3, 200*time.Millisecond, nil)

	secCfg := middleware.DefaultSecurityConfig()
	secCfg.RequestTimeout = 150 * time.Millisecond
	router := SetupRoutesWithSecurity(handler, handler.logger, secCfg, middleware.DefaultValidationConfig())

	req := httptest.NewRequest(http.MethodPost, bulkApplyPath, strings.NewReader(applyBody(applyNamePatch, false, ids...)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "TestAgent/1.0")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	res := decodeApply(t, rr)
	require.Len(t, res.Results, 3)
	assert.Equal(t, 3, res.Applied, "%+v", res.Results)
	for _, id := range ids {
		assert.True(t, overridesNameTheBulkName(t, db, id), "device %d", id)
	}
}

// The request ending while a device is being written does not cut that device: it ends as the
// write ended, and no later device is contacted or has its overrides stored. On main the cancelled
// context cut device 1 mid-apply and stored the overrides of the later ones before their apply failed.
func TestBulkApply_TheDeviceInFlightIsNotCutByTheRequestEnding(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	handler := exportHandler(t, db)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ids := slowFleet(t, db, 3, 100*time.Millisecond, func(i int) {
		if i == 0 {
			cancel() // the client leaves while device 1 is being written
		}
	})

	res := decodeApply(t, postApplyWith(ctx, handler, applyBody(applyNamePatch, false, ids...)))

	require.Len(t, res.Results, 3)
	assert.Equal(t, bulkApplyApplied, res.Results[0].Status, "%+v", res.Results[0])
	assert.True(t, overridesNameTheBulkName(t, db, ids[0]))
	for i := 1; i < 3; i++ {
		e := res.Results[i]
		assert.Equal(t, bulkApplySkipped, e.Status, "%+v", e)
		assert.Equal(t, reasonCallEnded, e.Reason)
		require.NotNil(t, e.Stored)
		assert.False(t, *e.Stored)
		assert.False(t, overridesNameTheBulkName(t, db, ids[i]), "device %d must have no overrides stored", ids[i])
	}
	assert.Equal(t, 1, res.Applied)
	assert.Equal(t, 2, res.Skipped)
}

// A request that has ended before the first device is reached touches no device at all.
func TestBulkApply_ARequestThatAlreadyEndedTouchesNoDevice(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	handler := exportHandler(t, db)
	ids := slowFleet(t, db, 2, 0, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res := decodeApply(t, postApplyWith(ctx, handler, applyBody(applyNamePatch, false, ids...)))

	require.Len(t, res.Results, 2)
	for i, e := range res.Results {
		assert.Equal(t, bulkApplySkipped, e.Status, "%+v", e)
		assert.Equal(t, reasonCallEnded, e.Reason)
		assert.False(t, overridesNameTheBulkName(t, db, ids[i]))
	}
	assert.Equal(t, 2, res.Skipped)
}

// The run has a budget of its own. A device in flight when it ends is cut by it and answers
// honestly, and the devices after it are not contacted.
func TestBulkApply_TheBudgetEndsTheRunAndSaysSo(t *testing.T) {
	previous := bulkApplyBudget
	bulkApplyBudget = 150 * time.Millisecond
	t.Cleanup(func() { bulkApplyBudget = previous })

	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	handler := exportHandler(t, db)
	ids := slowFleet(t, db, 3, 600*time.Millisecond, nil)

	res := decodeApply(t, postApply(handler, applyBody(applyNamePatch, false, ids...)))

	require.Len(t, res.Results, 3)
	assert.NotEqual(t, bulkApplyApplied, res.Results[0].Status, "the device in flight was cut by the budget: %+v", res.Results[0])
	for i := 1; i < 3; i++ {
		assert.Equal(t, bulkApplySkipped, res.Results[i].Status, "%+v", res.Results[i])
		assert.Equal(t, reasonCallEnded, res.Results[i].Reason)
		assert.False(t, overridesNameTheBulkName(t, db, ids[i]))
	}
}

// A request whose body arrived late has little of its deadline left, and the run may not start a
// device it cannot finish before that deadline: the budget counts from the request's deadline.
func TestBulkApply_ALateRequestLeavesTheRunLessThanTheBudget(t *testing.T) {
	prevBudget, prevDeadline := bulkApplyBudget, bulkApplyDeadline
	bulkApplyBudget, bulkApplyDeadline = 150*time.Millisecond, 300*time.Millisecond
	t.Cleanup(func() { bulkApplyBudget, bulkApplyDeadline = prevBudget, prevDeadline })

	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	handler := exportHandler(t, db)
	ids := slowFleet(t, db, 2, 0, nil)
	// 50 ms are left of the deadline, which is less than the 150 ms margin the answer needs.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	res := decodeApply(t, postApplyWith(ctx, handler, applyBody(applyNamePatch, false, ids...)))

	require.Len(t, res.Results, 2)
	for i, e := range res.Results {
		assert.Equal(t, bulkApplySkipped, e.Status, "%+v", e)
		assert.Equal(t, reasonCallEnded, e.Reason)
		assert.False(t, overridesNameTheBulkName(t, db, ids[i]))
	}
}

// A route reaches the longer deadline only by the exact name set where it is registered. The test
// reads the real route table and counts it first: a walk that saw nothing would pass for anything.
func TestRouteDeadline_OnlyTheBulkApplyRouteHasOne(t *testing.T) {
	t.Setenv("SHELLY_SECURITY_VALIDATION_TEST_MODE", "")
	router := newRouteContractRouter(t)

	var routes int
	var named []string
	require.NoError(t, router.Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
		routes++
		if route.GetName() == bulkApplyRouteName {
			path, err := route.GetPathTemplate()
			require.NoError(t, err)
			methods, _ := route.GetMethods()
			named = append(named, strings.Join(methods, ",")+" "+path)
		}
		return nil
	}))
	require.Greater(t, routes, 100, "the walk must see the whole route table")
	assert.Equal(t, []string{"POST /api/v1/config/bulk-apply"}, named)
}

// routeDeadline answers from the matched route's name alone: a named route gets the deadline, an
// unnamed one and one with another name get the default.
func TestRouteDeadline_ReadsTheMatchedRoutesName(t *testing.T) {
	r := mux.NewRouter()
	type answer struct {
		d  time.Duration
		ok bool
	}
	got := map[string]answer{}
	record := func(path string) http.HandlerFunc {
		return func(w http.ResponseWriter, req *http.Request) {
			d, ok := routeDeadline(req)
			got[path] = answer{d, ok}
		}
	}
	r.HandleFunc("/named", record("/named")).Name(bulkApplyRouteName)
	r.HandleFunc("/unnamed", record("/unnamed"))
	r.HandleFunc("/other", record("/other")).Name("something-else")
	for _, path := range []string{"/named", "/unnamed", "/other"} {
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}
	assert.Equal(t, answer{bulkApplyDeadline, true}, got["/named"])
	assert.Equal(t, answer{}, got["/unnamed"])
	assert.Equal(t, answer{}, got["/other"])

	// A request that matched no route has none to read.
	d, ok := routeDeadline(httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Zero(t, d)
	assert.False(t, ok)
}
