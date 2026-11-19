package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// POST /config/bulk-download answers one JSON file with the masked desired configuration of each
// named device (ginsys/shelly-manager#595). It reads the database and never contacts a device.

const bulkDownloadPath = "/api/v1/config/bulk-download"

type downloadEntry struct {
	DeviceID   uint            `json:"device_id"`
	DeviceName string          `json:"device_name"`
	MAC        string          `json:"mac"`
	Status     string          `json:"status"`
	Config     json.RawMessage `json:"config"`
	Error      string          `json:"error"`
}

type downloadFile struct {
	GeneratedAt string          `json:"generated_at"`
	Devices     []downloadEntry `json:"devices"`
}

func postDownload(handler *Handler, body string) *httptest.ResponseRecorder {
	return postBulk(handler, (*Handler).BulkDownloadConfigs, bulkDownloadPath, body)
}

func decodeDownload(t *testing.T, w *httptest.ResponseRecorder) downloadFile {
	t.Helper()
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var file downloadFile
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &file), w.Body.String())
	return file
}

// The route exists in the production table, as a POST.
func TestBulkDownload_RouteIsRegistered(t *testing.T) {
	t.Setenv("SHELLY_SECURITY_VALIDATION_TEST_MODE", "")
	router := newRouteContractRouter(t)
	for method, want := range map[string]bool{http.MethodPost: true, http.MethodGet: false} {
		req, err := http.NewRequest(method, bulkDownloadPath, nil)
		require.NoError(t, err)
		var match mux.RouteMatch
		matched := router.Match(req, &match) && match.MatchErr == nil && match.Route != nil
		assert.Equal(t, want, matched, "%s %s", method, bulkDownloadPath)
	}
}

func TestBulkDownload_AnswersAFileWithTheMaskedDesiredConfiguration(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	handler := exportHandler(t, db)
	fleet := bulkFleet(t, db, 3)
	require.NoError(t, db.GetDB().Exec("UPDATE devices SET desired_config = ? WHERE id = ?",
		`{"mqtt":{"user":"iot","pass":"hunter2-secret"},"system":{"device":{"name":"kept"}}}`, fleet[2].device.ID).Error)

	w := postDownload(handler, idsBody(fleet[2].device.ID, fleet[0].device.ID))
	file := decodeDownload(t, w)

	assert.True(t, strings.HasPrefix(w.Header().Get("Content-Disposition"), "attachment; filename="), w.Header().Get("Content-Disposition"))
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	assert.NotEmpty(t, file.GeneratedAt)
	require.Len(t, file.Devices, 2, "one entry per named device, in request order")
	assert.Equal(t, fleet[2].device.ID, file.Devices[0].DeviceID)
	assert.Equal(t, fleet[0].device.ID, file.Devices[1].DeviceID)
	assert.Equal(t, "ok", file.Devices[0].Status)
	assert.Equal(t, fleet[2].device.MAC, file.Devices[0].MAC)
	assert.Empty(t, file.Devices[0].Error)

	// The secret is masked, not omitted and not returned; the rest of the configuration is kept.
	body := string(file.Devices[0].Config)
	assert.NotContains(t, w.Body.String(), "hunter2-secret")
	assert.Contains(t, body, `"pass":"`+secretMask+`"`)
	assert.Contains(t, body, `"name":"kept"`)

	// A file, not the success envelope.
	var envelope map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	assert.NotContains(t, envelope, "success")
	assert.NotContains(t, envelope, "data")

	assert.Zero(t, totalHits(fleet), "the download read the database and contacted a device")
}

// An unknown id and a stored configuration that cannot be read are entries with an error and a
// fixed text; the call answers 200 and the other devices are in the file.
func TestBulkDownload_AnUnreadableDeviceIsAnEntryNotAFailure(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	handler := exportHandler(t, db)
	fleet := bulkFleet(t, db, 2)
	const storedBytes = "STORED-BYTES-MUST-NOT-LEAK"
	require.NoError(t, db.GetDB().Exec("UPDATE devices SET desired_config = ? WHERE id = ?",
		`{"mqtt":{"pass":"`+storedBytes+`"`, fleet[1].device.ID).Error) // truncated JSON

	w := postDownload(handler, idsBody(fleet[0].device.ID, 9999, fleet[1].device.ID))
	file := decodeDownload(t, w)

	require.Len(t, file.Devices, 3)
	assert.Equal(t, "ok", file.Devices[0].Status)

	assert.Equal(t, uint(9999), file.Devices[1].DeviceID)
	assert.Equal(t, "error", file.Devices[1].Status)
	assert.Equal(t, "device not found", file.Devices[1].Error)
	assert.Empty(t, file.Devices[1].Config)

	assert.Equal(t, "error", file.Devices[2].Status)
	assert.Equal(t, "the configuration could not be read", file.Devices[2].Error)
	assert.Empty(t, file.Devices[2].Config)
	assert.NotContains(t, w.Body.String(), storedBytes, "the stored bytes reached the file through an error text")
}

// An empty desired configuration is "none", never an entry with no settings.
func TestBulkDownload_AnEmptyDesiredConfigurationIsNotNoSettings(t *testing.T) {
	for _, stored := range []string{"", "{}", "null"} {
		t.Run("stored="+stored, func(t *testing.T) {
			db, cleanup := testutil.TestDatabase(t)
			defer cleanup()
			handler := exportHandler(t, db)
			fleet := bulkFleet(t, db, 1)
			require.NoError(t, db.GetDB().Exec("UPDATE devices SET desired_config = ? WHERE id = ?", stored, fleet[0].device.ID).Error)

			file := decodeDownload(t, postDownload(handler, idsBody(fleet[0].device.ID)))

			require.Len(t, file.Devices, 1)
			assert.Equal(t, "error", file.Devices[0].Status)
			assert.Equal(t, "no desired configuration is set for this device", file.Devices[0].Error)
			assert.Empty(t, file.Devices[0].Config)
		})
	}
}

func TestBulkDownload_RefusesMoreThanTheCapBeforeAnyRead(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	handler := exportHandler(t, db)
	fleet := bulkFleet(t, db, maxBulkDownloadDevices+1)
	ids := make([]uint, 0, len(fleet))
	for _, f := range fleet {
		ids = append(ids, f.device.ID)
	}

	over := postDownload(handler, idsBody(ids...))
	assert.Equal(t, http.StatusBadRequest, over.Code, over.Body.String())
	assert.Contains(t, over.Body.String(), "VALIDATION_FAILED")
	assert.Contains(t, over.Body.String(), strconv.Itoa(maxBulkDownloadDevices))
	assert.Zero(t, totalHits(fleet))

	// Exactly the cap is accepted.
	at := postDownload(handler, idsBody(ids[:maxBulkDownloadDevices]...))
	assert.Len(t, decodeDownload(t, at).Devices, maxBulkDownloadDevices)
}

// The request contract of the other bulk routes holds, and stop_on_error, which a read has no use
// for, is refused rather than ignored.
func TestBulkDownload_RefusesABadRequest(t *testing.T) {
	bodies := map[string]string{
		"no body":       "",
		"empty object":  `{}`,
		"empty list":    `{"device_ids":[]}`,
		"zero id":       `{"device_ids":[0]}`,
		"negative id":   `{"device_ids":[-1]}`,
		"not json":      `not json`,
		"unknown field": `{"device_ids":[1],"configurations":[{}]}`,
		"stop on error": `{"device_ids":[1],"options":{"stop_on_error":true}}`,
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			db, cleanup := testutil.TestDatabase(t)
			defer cleanup()
			handler := exportHandler(t, db)
			fleet := bulkFleet(t, db, 1)

			w := postDownload(handler, body)

			assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			assert.Contains(t, w.Body.String(), "VALIDATION_FAILED")
			assert.Empty(t, w.Header().Get("Content-Disposition"))
			assert.Zero(t, totalHits(fleet))
		})
	}
}

// A repeated id is one entry, as on the other bulk routes.
func TestBulkDownload_ARepeatedIDIsOneEntry(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	handler := exportHandler(t, db)
	fleet := bulkFleet(t, db, 1)
	id := fleet[0].device.ID

	file := decodeDownload(t, postDownload(handler, fmt.Sprintf(`{"device_ids":[%d,%d]}`, id, id)))

	assert.Len(t, file.Devices, 1)
}

// With an admin key set, a request with no key is 401 before the body is read or the database is.
func TestBulkDownload_RequiresTheAdminKey(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	handler := exportHandler(t, db)
	handler.SetAdminAPIKey("bulk-download-key")
	fleet := bulkFleet(t, db, 1)
	body := idsBody(fleet[0].device.ID)

	w := postDownload(handler, body)
	assert.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
	assert.Empty(t, w.Header().Get("Content-Disposition"))
	assert.NotContains(t, w.Body.String(), "mqtt")

	// An unparseable body is still 401 first: the admin check comes before any parsing.
	assert.Equal(t, http.StatusUnauthorized, postDownload(handler, "not json").Code)

	req := httptest.NewRequest("POST", bulkDownloadPath, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer bulk-download-key")
	ok := httptest.NewRecorder()
	handler.BulkDownloadConfigs(ok, req)
	assert.Equal(t, http.StatusOK, ok.Code, ok.Body.String())
}

// Decision 12 of the design note: the download returns no secret value. The desired configuration
// is read through GetDesiredConfig, which merges the templates and the overrides, so both carry a
// secret in every field here, and the file is searched for every secret path. The secret paths come
// from populated, which finds them by the field names and not from secretSlots, so a secret that
// maskSecrets misses fails here.
//
// What this proves is the one read the download uses. It does not prove the typed routes
// (maskTypedSecrets, which also covers the raw part), the config diff output
// (config_diff_redact.go, a separate rule) or the device-secrets path: the download does not read
// them.
func TestBulkDownload_ReturnsNoSecretValue(t *testing.T) {
	handler, cleanup := setupTestHandler(t)
	defer cleanup()

	full, secrets, others := populated(t)
	full = valid(t, full)
	fullJSON := marshalConfig(t, full)

	device := testutil.TestDevice()
	device.Type = ""
	device.Settings = ""
	require.NoError(t, handler.DB.AddDevice(device))
	svc := handler.ConfigService.ConfigurationSvc
	template := &configuration.ServiceConfigTemplate{Name: "Global", Scope: "global", Config: []byte(fullJSON)}
	require.NoError(t, svc.CreateTemplate(template))
	require.NoError(t, svc.SetDeviceTemplates(device.ID, []uint{template.ID}))
	require.NoError(t, svc.SetDeviceOverrides(device.ID, full))

	w := postDownload(handler, idsBody(device.ID))
	file := decodeDownload(t, w)
	require.Len(t, file.Devices, 1)
	require.Equal(t, "ok", file.Devices[0].Status, w.Body.String())

	assertNoSecret(t, "bulk-download", w.Body.String(), secrets)
	assert.Equal(t, len(secrets), strings.Count(string(file.Devices[0].Config), `"`+secretMask+`"`), "one mask per secret")
	// Mask, not omit: a field that is not a secret is still there.
	for _, p := range others[:1] {
		assert.Contains(t, string(file.Devices[0].Config), quoted(p))
	}
}
