package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// POST /config/bulk-apply stores a partial configuration as the overrides of each named device and
// applies it through the per-device apply (ginsys/shelly-manager#594). Every device here is a fake
// server on the loopback; a device that must not be touched gets no request.

const bulkApplyPath = "/api/v1/config/bulk-apply"

const applyNamePatch = `{"system":{"device":{"name":"bulk-name"}}}`

type applyEntry struct {
	DeviceID       uint     `json:"device_id"`
	DeviceName     string   `json:"device_name"`
	DeviceIP       string   `json:"device_ip"`
	Status         string   `json:"status"`
	Stored         *bool    `json:"stored"`
	Reason         string   `json:"reason"`
	Failures       []string `json:"failures"`
	RequiresReboot bool     `json:"requires_reboot"`
}

type applyResult struct {
	Total       int          `json:"total"`
	Applied     int          `json:"applied"`
	Pending     int          `json:"pending"`
	Unconfirmed int          `json:"unconfirmed"`
	Refused     int          `json:"refused"`
	Failed      int          `json:"failed"`
	Skipped     int          `json:"skipped"`
	Results     []applyEntry `json:"results"`
}

func applyBody(config string, stopOnError bool, ids ...uint) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprint(id)
	}
	return fmt.Sprintf(`{"device_ids":[%s],"config":%s,"options":{"stop_on_error":%t}}`, strings.Join(parts, ","), config, stopOnError)
}

func postApply(handler *Handler, body string) *httptest.ResponseRecorder {
	return postBulk(handler, (*Handler).BulkApplyConfigs, bulkApplyPath, body)
}

func decodeApply(t *testing.T, w *httptest.ResponseRecorder) applyResult {
	t.Helper()
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body struct {
		Data applyResult `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), w.Body.String())
	return body.Data
}

func storedOverrides(t *testing.T, db *database.Manager, id uint) string {
	t.Helper()
	var overrides string
	require.NoError(t, db.GetDB().Raw("SELECT overrides FROM devices WHERE id = ?", id).Scan(&overrides).Error)
	return overrides
}

func storedDesired(t *testing.T, db *database.Manager, id uint) string {
	t.Helper()
	var desired string
	require.NoError(t, db.GetDB().Raw("SELECT desired_config FROM devices WHERE id = ?", id).Scan(&desired).Error)
	return desired
}

func setOverrides(t *testing.T, db *database.Manager, id uint, overrides string) {
	t.Helper()
	require.NoError(t, db.GetDB().Exec("UPDATE devices SET overrides = ? WHERE id = ?", overrides, id).Error)
}

// quietDevice is a Gen1 device that reports mac on /shelly and counts every other request: the
// requests that follow the identity check are the ones that read or write the device.
func quietDevice(t *testing.T, mac string) (addr string, other *atomic.Int32) {
	t.Helper()
	other = new(atomic.Int32)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/shelly" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"type": "SHPLG-S", "mac": mac, "auth": false, "fw": "1.14.0"})
			return
		}
		other.Add(1)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)
	return strings.TrimPrefix(server.URL, "http://"), other
}

func applyFleet(t *testing.T, n int) (*Handler, *database.Manager, []fleetDevice) {
	t.Helper()
	db, cleanup := testutil.TestDatabase(t)
	t.Cleanup(cleanup)
	handler := exportHandler(t, db)
	return handler, db, bulkFleet(t, db, n)
}

func TestBulkApply_RouteIsRegistered(t *testing.T) {
	t.Setenv("SHELLY_SECURITY_VALIDATION_TEST_MODE", "")
	router := newRouteContractRouter(t)
	for method, want := range map[string]bool{http.MethodPost: true, http.MethodGet: false} {
		req, err := http.NewRequest(method, bulkApplyPath, nil)
		require.NoError(t, err)
		var match mux.RouteMatch
		matched := router.Match(req, &match) && match.MatchErr == nil && match.Route != nil
		assert.Equal(t, want, matched, "%s %s", method, bulkApplyPath)
	}
}

// Admin first: a caller without the key reaches no parser, no database write and no device.
func TestBulkApply_RequiresTheAdminKey(t *testing.T) {
	handler, db, fleet := applyFleet(t, 1)
	handler.SetAdminAPIKey("bulk-apply-key")

	w := postApply(handler, applyBody(applyNamePatch, false, fleet[0].device.ID))
	require.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
	assert.Zero(t, totalHits(fleet))
	assert.NotContains(t, storedOverrides(t, db, fleet[0].device.ID), "bulk-name")

	// A body that would be a 400 for an admin is still a 401 here.
	w = postApply(handler, `{`)
	assert.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
}

func TestBulkApply_RefusesABadRequestBeforeAnyWrite(t *testing.T) {
	handler, db, fleet := applyFleet(t, 1)
	id := fleet[0].device.ID
	cases := map[string]string{
		"empty body":          ``,
		"no members":          `{}`,
		"no config":           fmt.Sprintf(`{"device_ids":[%d]}`, id),
		"null config":         applyBody(`null`, false, id),
		"array config":        applyBody(`[]`, false, id),
		"string config":       applyBody(`"x"`, false, id),
		"empty config":        applyBody(`{}`, false, id),
		"no devices":          `{"device_ids":[],"config":` + applyNamePatch + `}`,
		"unknown member":      fmt.Sprintf(`{"device_ids":[%d],"config":%s,"template_id":3}`, id, applyNamePatch),
		"all devices member":  fmt.Sprintf(`{"all":true,"config":%s}`, applyNamePatch),
		"misspelled section":  applyBody(`{"wfi":{"enable":true}}`, false, id),
		"unknown nested key":  applyBody(`{"system":{"devise":{"name":"x"}}}`, false, id),
		"wrong value type":    applyBody(`{"system":{"device":{"name":5}}}`, false, id),
		"two json values":     applyBody(applyNamePatch, false, id) + ` {}`,
		"non-positive id":     `{"device_ids":[0],"config":` + applyNamePatch + `}`,
		"trailing id garbage": `{"device_ids":["a"],"config":` + applyNamePatch + `}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			w := postApply(handler, body)
			assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			assert.Zero(t, totalHits(fleet), "a device was contacted")
			assert.NotContains(t, storedOverrides(t, db, id), "bulk-name")
		})
	}
}

// A misspelled key names itself and no value.
func TestBulkApply_NamesTheUnknownKeyAndNoValue(t *testing.T) {
	handler, _, fleet := applyFleet(t, 1)
	w := postApply(handler, applyBody(`{"wfi":{"pass":"hunter2-typo"}}`, false, fleet[0].device.ID))
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `wfi`)
	assert.NotContains(t, w.Body.String(), "hunter2-typo")
}

func TestBulkApply_TheCapCountsDistinctDevices(t *testing.T) {
	handler, db, fleet := applyFleet(t, 1)

	ids := make([]uint, 0, 11)
	for i := uint(9001); i < 9012; i++ {
		ids = append(ids, i)
	}
	w := postApply(handler, applyBody(applyNamePatch, false, ids...))
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "10")
	assert.Zero(t, totalHits(fleet))

	// Ten distinct ids pass, and an eleventh element that repeats one is dropped before the count.
	w = postApply(handler, applyBody(applyNamePatch, false, append(ids[:10], ids[0])...))
	res := decodeApply(t, w)
	assert.Equal(t, 10, res.Total)
	assert.Len(t, res.Results, 10)
	assert.NotContains(t, storedOverrides(t, db, fleet[0].device.ID), "bulk-name")
}

// The other bulk routes still refuse the new member: a config on them is an unknown field.
func TestBulkApply_OtherBulkRoutesRefuseAConfigMember(t *testing.T) {
	handler, _, fleet := applyFleet(t, 1)
	body := fmt.Sprintf(`{"device_ids":[%d],"config":%s}`, fleet[0].device.ID, applyNamePatch)
	for _, route := range bulkRoutes {
		w := postBulk(handler, route.call, route.path, body)
		assert.Equal(t, http.StatusBadRequest, w.Code, "%s: %s", route.name, w.Body.String())
	}
	w := postDownload(handler, body)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Zero(t, totalHits(fleet))
}

// The route acts on the devices the request names and on no others.
func TestBulkApply_ActsOnlyOnTheNamedDevices(t *testing.T) {
	handler, db, fleet := applyFleet(t, 3)
	untouched := storedDesired(t, db, fleet[1].device.ID)

	w := postApply(handler, applyBody(applyNamePatch, false, fleet[2].device.ID, fleet[0].device.ID))
	res := decodeApply(t, w)

	require.Len(t, res.Results, 2, "one entry per named device, in request order")
	assert.Equal(t, fleet[2].device.ID, res.Results[0].DeviceID)
	assert.Equal(t, fleet[0].device.ID, res.Results[1].DeviceID)
	assert.Zero(t, fleet[1].hits.Load(), "the device that was not named was contacted")
	assert.NotZero(t, fleet[0].hits.Load())
	assert.NotZero(t, fleet[2].hits.Load())
	assert.Empty(t, strings.TrimSpace(strings.Trim(storedOverrides(t, db, fleet[1].device.ID), "{}")), "an unnamed device's overrides changed")
	assert.Equal(t, untouched, storedDesired(t, db, fleet[1].device.ID))
	assert.Contains(t, storedOverrides(t, db, fleet[0].device.ID), "bulk-name")
	assert.Contains(t, storedOverrides(t, db, fleet[2].device.ID), "bulk-name")
}

// The same id twice is one device, one patch and one apply.
func TestBulkApply_ARepeatedIDIsOneEntry(t *testing.T) {
	handler, _, fleet := applyFleet(t, 1)
	id := fleet[0].device.ID

	res := decodeApply(t, postApply(handler, applyBody(applyNamePatch, false, id, id)))

	require.Len(t, res.Results, 1)
	assert.Equal(t, 1, res.Total)
}

// One device that cannot take the JSON means nothing is written to any: the devices before it in the
// request are not written either. A version that checked and wrote device by device fails this.
func TestBulkApply_OneInvalidDeviceMeansNothingIsWrittenToAny(t *testing.T) {
	handler, db, fleet := applyFleet(t, 2)
	// Device 0 is a roller model, which has a roller section; device 1 is a plug, which has none.
	require.NoError(t, db.GetDB().Exec(`UPDATE devices SET settings = ? WHERE id = ?`,
		`{"model":"SHSW-25","gen":1}`, fleet[0].device.ID).Error)
	desired0, desired1 := storedDesired(t, db, fleet[0].device.ID), storedDesired(t, db, fleet[1].device.ID)

	w := postApply(handler, applyBody(`{"roller":{"max_open_time":10}}`, false, fleet[0].device.ID, fleet[1].device.ID))
	res := decodeApply(t, w)

	require.Len(t, res.Results, 2)
	assert.Equal(t, bulkApplySkipped, res.Results[0].Status)
	assert.Equal(t, bulkApplyRefused, res.Results[1].Status)
	for _, e := range res.Results {
		require.NotNil(t, e.Stored)
		assert.False(t, *e.Stored, "device %d: nothing may be stored", e.DeviceID)
	}
	assert.Contains(t, strings.Join(res.Results[1].Failures, " "), "roller")
	assert.Zero(t, totalHits(fleet), "a device was contacted")
	assert.NotContains(t, storedOverrides(t, db, fleet[0].device.ID), "roller", "the valid device was written")
	assert.Equal(t, desired0, storedDesired(t, db, fleet[0].device.ID))
	assert.Equal(t, desired1, storedDesired(t, db, fleet[1].device.ID))
	assert.Equal(t, 1, res.Skipped)
	assert.Equal(t, 1, res.Refused)
}

// An id that names no device is that entry's refusal; the others still run.
func TestBulkApply_AnUnknownIDIsRefusedAndDoesNotBlockTheOthers(t *testing.T) {
	handler, db, fleet := applyFleet(t, 2)

	res := decodeApply(t, postApply(handler, applyBody(applyNamePatch, false, fleet[0].device.ID, 9999, fleet[1].device.ID)))

	require.Len(t, res.Results, 3)
	assert.Equal(t, uint(9999), res.Results[1].DeviceID)
	assert.Equal(t, bulkApplyRefused, res.Results[1].Status)
	assert.Equal(t, "device not found", res.Results[1].Reason)
	require.NotNil(t, res.Results[1].Stored)
	assert.False(t, *res.Results[1].Stored)
	assert.NotZero(t, fleet[1].hits.Load(), "the device after the unknown id was not processed")
	assert.Contains(t, storedOverrides(t, db, fleet[1].device.ID), "bulk-name")
}

// A device at the stored address that is another device is refused before any write to it, and the
// entry says what did change: the new settings are stored and the device stays pending.
func TestBulkApply_AWrongDeviceAtTheAddressIsRefusedAndTheEntryNamesWhatIsStored(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	handler := exportHandler(t, db)
	addr, other := quietDevice(t, otherDeviceID)
	device := deviceAt(t, db, addr, storedMAC)

	res := decodeApply(t, postApply(handler, applyBody(applyNamePatch, false, device.ID)))

	require.Len(t, res.Results, 1)
	e := res.Results[0]
	assert.Equal(t, bulkApplyRefused, e.Status)
	require.NotNil(t, e.Stored)
	assert.True(t, *e.Stored)
	assert.Contains(t, e.Reason, "not the stored device")
	assert.Contains(t, e.Reason, "nothing was written to the device")
	assert.Contains(t, e.Reason, "the new settings are stored and the device stays pending")
	assert.Zero(t, other.Load(), "the device that answered was read or written after the identity check")
	assert.Contains(t, storedOverrides(t, db, device.ID), "bulk-name")
}

// stop_on_error: the first device that does not end applied stops the run; the rest are not contacted.
func TestBulkApply_StopOnErrorSkipsTheRest(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	handler := exportHandler(t, db)
	wrongAddr, _ := quietDevice(t, otherDeviceID)
	wrong := deviceAt(t, db, wrongAddr, storedMAC)
	fleet := bulkFleet(t, db, 1)

	stopped := decodeApply(t, postApply(handler, applyBody(applyNamePatch, true, wrong.ID, fleet[0].device.ID)))
	require.Len(t, stopped.Results, 2)
	assert.Equal(t, bulkApplyRefused, stopped.Results[0].Status)
	assert.Equal(t, bulkApplySkipped, stopped.Results[1].Status)
	require.NotNil(t, stopped.Results[1].Stored)
	assert.False(t, *stopped.Results[1].Stored)
	assert.Zero(t, fleet[0].hits.Load(), "a skipped device was contacted")
	assert.NotContains(t, storedOverrides(t, db, fleet[0].device.ID), "bulk-name")
	assert.Equal(t, 1, stopped.Skipped)

	continued := decodeApply(t, postApply(handler, applyBody(applyNamePatch, false, wrong.ID, fleet[0].device.ID)))
	require.Len(t, continued.Results, 2)
	assert.NotEqual(t, bulkApplySkipped, continued.Results[1].Status)
	assert.NotZero(t, fleet[0].hits.Load())
}

// A write the device does not take, or a read-back that differs, is a failed entry that names the
// setting and holds no value.
func TestBulkApply_AFailedWriteNamesTheSettingAndNoValue(t *testing.T) {
	handler, db, fleet := applyFleet(t, 1)
	const secret = "hunter2-bulk"
	patch := `{"system":{"device":{"name":"` + secret + `"}}}`

	w := postApply(handler, applyBody(patch, false, fleet[0].device.ID))
	res := decodeApply(t, w)

	require.Len(t, res.Results, 1)
	e := res.Results[0]
	assert.Equal(t, bulkApplyFailed, e.Status, w.Body.String())
	require.NotNil(t, e.Stored)
	assert.True(t, *e.Stored)
	assert.NotEmpty(t, e.Failures)
	assert.NotContains(t, w.Body.String(), secret, "a value reached the answer")
	assert.Contains(t, storedOverrides(t, db, fleet[0].device.ID), secret, "the overrides stay stored when the apply fails")
}

// The patch keeps the PATCH rules: null removes an override, and a secret sent back as the mask keeps
// the stored one.
func TestBulkApply_KeepsThePatchRules(t *testing.T) {
	handler, db, fleet := applyFleet(t, 1)
	id := fleet[0].device.ID
	setOverrides(t, db, id, `{"system":{"device":{"name":"bulk-old"}},"mqtt":{"user":"iot","pass":"keep-me"}}`)

	decodeApply(t, postApply(handler, applyBody(`{"system":{"device":{"name":null}},"mqtt":{"pass":"`+secretMask+`"}}`, false, id)))

	overrides := storedOverrides(t, db, id)
	assert.NotContains(t, overrides, "bulk-old", "null did not remove the override")
	assert.Contains(t, overrides, "keep-me", "the mask replaced the stored secret")
}

// bulkApplyOutcome maps each result of the per-device apply to one status, the stored flag and a fixed
// reason. The reason of a refusal after the overrides were stored says so.
func TestBulkApplyOutcome_OneRowPerResult(t *testing.T) {
	applied := &configuration.ApplyVerifyResult{
		ConfigApplied: true,
		ApplyResult:   &configuration.ApplyResult{Success: true},
		VerifyResult:  &configuration.VerifyResult{Match: true},
	}
	pending := &configuration.ApplyVerifyResult{
		ApplyResult:  &configuration.ApplyResult{Success: true},
		VerifyResult: &configuration.VerifyResult{Match: true},
	}
	unconfirmed := &configuration.ApplyVerifyResult{
		ApplyResult: &configuration.ApplyResult{Success: true, Unconfirmed: true},
	}
	refusedWrite := &configuration.ApplyVerifyResult{
		ApplyResult: &configuration.ApplyResult{
			FailedCount: 1,
			Failures:    []configuration.ApplyFailure{{Path: "mqtt.pass", Value: "hunter2", Error: "device said: bad pass hunter2"}},
		},
	}
	readBack := &configuration.ApplyVerifyResult{
		ApplyResult: &configuration.ApplyResult{Success: true},
		VerifyResult: &configuration.VerifyResult{Differences: []configuration.ConfigDifference{
			{Path: "mqtt.server", Expected: "wanted-value", Actual: "device-value"},
		}},
	}
	const storedTail = "nothing was written to the device, the new settings are stored and the device stays pending"

	rows := []struct {
		name         string
		result       *configuration.ApplyVerifyResult
		err          error
		status       string
		reasonHas    string
		failuresHave []string
	}{
		{"applied", applied, nil, bulkApplyApplied, "", nil},
		{"pending", pending, nil, bulkApplyPending, "stays pending", nil},
		{"unconfirmed", unconfirmed, nil, bulkApplyUnconfirmed, "not verified", nil},
		{"refused write", refusedWrite, nil, bulkApplyFailed, "refused a write", []string{"mqtt.pass: the device refused the write"}},
		{"read-back difference", readBack, nil, bulkApplyFailed, "different value", []string{"mqtt.server: the device holds a different value"}},
		{"unsupported generation", nil, fmt.Errorf("x: %w", configuration.ErrUnsupportedGeneration), bulkApplyRefused, storedTail, nil},
		{"unsupported settings", nil, fmt.Errorf("x: %w", configuration.ErrUnsupportedSettings), bulkApplyRefused, storedTail, nil},
		{"wrong device", nil, fmt.Errorf("x: %w", configuration.ErrDeviceIdentity), bulkApplyRefused, storedTail, nil},
		{"apply running", nil, fmt.Errorf("x: %w", configuration.ErrApplyInProgress), bulkApplyRefused, storedTail, nil},
		{"other error", nil, errors.New("verify failed: password hunter2 rejected"), bulkApplyFailed, "did not end in a verified, recorded write", nil},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			e := bulkApplyOutcome(bulkApplyEntry{DeviceID: 7}, row.result, row.err)

			assert.Equal(t, uint(7), e.DeviceID)
			assert.Equal(t, row.status, e.Status)
			require.NotNil(t, e.Stored, "the overrides were stored before the apply")
			assert.True(t, *e.Stored)
			assert.Contains(t, e.Reason, row.reasonHas)
			assert.Equal(t, row.failuresHave, e.Failures)
			encoded, err := json.Marshal(e)
			require.NoError(t, err)
			for _, leak := range []string{"hunter2", "wanted-value", "device-value", "device said"} {
				assert.NotContains(t, string(encoded), leak)
			}
		})
	}
}

// settingsDevice is a Gen1 device that keeps the name it is given: a write to /settings is read back
// by the next read, so an apply that verifies ends applied. onWrite runs after each write.
func settingsDevice(t *testing.T, mac string, onWrite func()) (addr string) {
	t.Helper()
	var name atomic.Value
	name.Store("")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/shelly":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"type": "SHPLG-S", "mac": mac, "auth": false, "fw": "1.14.0"})
		case r.URL.Path == "/settings" && r.Method == http.MethodPost:
			_ = r.ParseForm()
			if v := r.Form.Get("name"); v != "" {
				name.Store(v)
			}
			if onWrite != nil {
				onWrite()
			}
			_, _ = w.Write([]byte(`{}`))
		case r.URL.Path == "/settings":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"name": name.Load().(string)})
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(server.Close)
	return strings.TrimPrefix(server.URL, "http://")
}

// A device that takes the write and reads it back equal is applied, and the entry says what was stored.
func TestBulkApply_ADeviceThatReadsBackEqualIsApplied(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	handler := exportHandler(t, db)
	device := deviceAt(t, db, settingsDevice(t, reportedMAC, nil), storedMAC)

	res := decodeApply(t, postApply(handler, applyBody(applyNamePatch, false, device.ID)))

	require.Len(t, res.Results, 1)
	e := res.Results[0]
	assert.Equal(t, bulkApplyApplied, e.Status, "%+v", e)
	require.NotNil(t, e.Stored)
	assert.True(t, *e.Stored)
	assert.Empty(t, e.Reason)
	assert.Empty(t, e.Failures)
	assert.Equal(t, 1, res.Applied)
}

// A desired configuration that moves while the apply runs leaves the device pending, though what
// was written read back equal.
func TestBulkApply_ADesiredConfigurationThatMovesMeanwhileIsPending(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	handler := exportHandler(t, db)
	var id uint
	addr := settingsDevice(t, reportedMAC, func() {
		assert.NoError(t, db.GetDB().Exec(`UPDATE devices SET desired_config = ? WHERE id = ?`,
			`{"system":{"device":{"name":"edited-meanwhile"}}}`, id).Error)
	})
	device := deviceAt(t, db, addr, storedMAC)
	id = device.ID

	res := decodeApply(t, postApply(handler, applyBody(applyNamePatch, false, device.ID)))

	require.Len(t, res.Results, 1)
	e := res.Results[0]
	assert.Equal(t, bulkApplyPending, e.Status, "%+v", e)
	require.NotNil(t, e.Stored)
	assert.True(t, *e.Stored)
	assert.Contains(t, e.Reason, "stays pending")
	assert.Equal(t, 1, res.Pending)
}

// After the stop nothing is contacted, a device that was not found included: it is skipped like the
// others, so the totals say what the run did.
func TestBulkApply_StopOnErrorSkipsALaterMissingDeviceToo(t *testing.T) {
	handler, _, fleet := applyFleet(t, 1)

	res := decodeApply(t, postApply(handler, applyBody(applyNamePatch, true, 9001, 9002, fleet[0].device.ID)))

	require.Len(t, res.Results, 3)
	assert.Equal(t, bulkApplyRefused, res.Results[0].Status)
	assert.Equal(t, bulkApplySkipped, res.Results[1].Status)
	assert.Equal(t, bulkApplySkipped, res.Results[2].Status)
	assert.Equal(t, 1, res.Refused)
	assert.Equal(t, 2, res.Skipped)
	assert.Zero(t, fleet[0].hits.Load())
}
