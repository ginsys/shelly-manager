package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/api/middleware"
	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/database/provider"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/service"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// Three secrets that must stay out of every log line and every answer of a rollback: one sent in
// `secrets`, one a legacy history row still holds in the clear, and one the device holds now.
const (
	rbBodySecret    = "sentinel-body-value"
	rbStoredSecret  = "sentinel-stored-value"
	rbCurrentSecret = "sentinel-current-value"
	rbAdminKey      = "rb-admin-key"
)

var rbSecrets = []string{rbBodySecret, rbStoredSecret, rbCurrentSecret}

// editsBeforeRollbackRepo changes the device between the service's read and the repository's write,
// which is how a conditional write is lost (the 409 that says the configuration changed while prepared),
// or fails the write, which the handler answers as an internal error.
type editsBeforeRollbackRepo struct {
	*configuration.GormConfigRepository
	edit func()
	fail error
}

func (r *editsBeforeRollbackRepo) RollbackDeviceDesired(deviceID uint, read configuration.DesiredRead, write configuration.DesiredWrite, restoredFrom uint) error {
	if r.fail != nil {
		err := r.fail
		r.fail = nil
		return err
	}
	if r.edit != nil {
		r.edit()
	}
	return r.GormConfigRepository.RollbackDeviceDesired(deviceID, read, write, restoredFrom)
}

// rbLogRig is the production request chain over a real database, with the handler, the services, the
// database and the middleware all writing to one log file at debug level.
type rbLogRig struct {
	t       *testing.T
	handler *Handler
	logger  *logging.Logger
	repo    *editsBeforeRollbackRepo
	id      uint
	logPath string
}

func newRBLogRig(t *testing.T) *rbLogRig {
	t.Helper()
	// An empty value selects the production router; `make test` sets it, and the sqlite provider then
	// swaps the GORM logger for one that discards output.
	t.Setenv(testModeEnv, "")
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")
	logger, err := logging.New(logging.Config{Level: "debug", Format: "text", Output: logPath})
	require.NoError(t, err)
	db, err := database.NewManagerWithLogger(provider.DatabaseConfig{
		Provider: "sqlite", DSN: filepath.Join(dir, "test.db"), LogLevel: "info",
	}, logger)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
		_ = logger.Close()
	})

	svc := service.NewServiceWithLogger(db, testutil.TestConfig(), logger)
	svc.SetDiscoverFunc(noDiscovery)
	handler := NewHandlerWithLogger(db, svc, testNotificationHandler(t, db), nil, logger)
	handler.SetAdminAPIKey(rbAdminKey)

	device := &database.Device{IP: "192.0.2.50", MAC: "AA:BB:CC:DD:EE:61", Type: "SHPLG-S", Name: "rollback-logs"}
	require.NoError(t, db.AddDevice(device))

	repo := &editsBeforeRollbackRepo{GormConfigRepository: configuration.NewGormConfigRepository(db.GetDB(), logger)}
	handler.ConfigService.ConfigurationSvc = configuration.NewConfigurationService(repo, nil, logger)
	return &rbLogRig{t: t, handler: handler, logger: logger, repo: repo, id: device.ID, logPath: logPath}
}

func (r *rbLogRig) log() string {
	raw, err := os.ReadFile(r.logPath)
	require.NoError(r.t, err)
	return string(raw)
}

// send puts one request through the production chain. Each call builds its own router, so the security
// monitor of an earlier call cannot block the next one for answering a run of 4xx.
func (r *rbLogRig) send(method, path, body string) *httptest.ResponseRecorder {
	r.t.Helper()
	router := SetupRoutesWithSecurity(r.handler, r.logger, middleware.DefaultSecurityConfig(), middleware.DefaultValidationConfig())
	closeMonitor(r.t, r.handler)
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+rbAdminKey)
	req.Header.Set("User-Agent", "Shelly-Manager-UI/1.0 (axios)")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func (r *rbLogRig) rollbackPath() string {
	return fmt.Sprintf("/api/v1/devices/%d/config/new/rollback", r.id)
}

func (r *rbLogRig) addRow(text *string) uint {
	r.t.Helper()
	row := configuration.DesiredConfigHistory{DeviceID: r.id, Action: "recompute", NewDesired: text, ChangedBy: "system"}
	require.NoError(r.t, r.handler.DB.GetDB().Create(&row).Error)
	return row.ID
}

// A rollback of every kind of answer the handler gives goes through the production chain. For each, the
// status and the message prove which branch answered (a request can also be refused before the handler),
// the request line is the control that the call wrote to the log at all, and no secret is in what the call
// wrote or in the answer.
func TestRollbackRoute_NoSecretReachesALogOrAnAnswer(t *testing.T) {
	rig := newRBLogRig(t)
	h := rig.handler

	putOverrides(t, h, rig.id, `{"mqtt":{"server":"a.example","user":"iot","pass":"old-secret"}}`)
	first := versionHolding(t, h, rig.id, "a.example")
	putOverrides(t, h, rig.id, `{"mqtt":{"server":"b.example","user":"iot","pass":"`+rbCurrentSecret+`"}}`)
	second := versionHolding(t, h, rig.id, "b.example")

	legacy := `{"mqtt":{"server":"legacy.example","user":"iot","pass":"` + rbStoredSecret + `"}}`
	legacyID := rig.addRow(&legacy)
	noText := rig.addRow(nil)
	noSlot := `{"custom":{"token":"` + secretMask + `"}}`
	noSlotID := rig.addRow(&noSlot)
	loses := `{"mqtt":{"server":"a"},"made_up":{"x":1}}`
	losesID := rig.addRow(&loses)

	type run struct {
		name    string
		body    string
		prepare func()
		status  int
		message string
	}
	body := func(id uint, secrets string) string {
		if secrets == "" {
			return fmt.Sprintf(`{"history_id":%d}`, id)
		}
		return fmt.Sprintf(`{"history_id":%d,"secrets":%s}`, id, secrets)
	}
	runs := []run{
		{name: "200 a version is set and the current secret is kept", body: body(second, ""), status: http.StatusOK, message: "Desired configuration set to version"},
		{name: "400 a body the validation middleware refuses", body: `{"history_id":` + fmt.Sprint(first) + `,"secrets":{"mqtt.pass":"` + rbBodySecret, status: http.StatusBadRequest, message: "Validation failed"},
		{name: "400 a body with no history id", body: `{"secrets":{"mqtt.pass":"` + rbBodySecret + `"}}`, status: http.StatusBadRequest, message: "Invalid JSON request body"},
		{name: "400 a key off the list", body: body(first, `{"mqtt.pass":"`+rbBodySecret+`","auth.pass":"`+rbBodySecret+`"}`), status: http.StatusBadRequest, message: "not an answer to the list"},
		{name: "409 asks for the secret with its path", body: body(first, ""), status: http.StatusConflict, message: "Enter them for the listed paths"},
		{name: "409 the configuration changed while it was prepared", body: body(first, `{"mqtt.pass":"`+rbBodySecret+`"}`),
			prepare: func() {
				rig.repo.edit = func() {
					rig.repo.edit = nil
					putOverrides(t, h, rig.id, `{"mqtt":{"server":"edited.example","pass":"`+rbCurrentSecret+`"}}`)
				}
			},
			status: http.StatusConflict, message: "changed while the rollback was prepared"},
		{name: "500 the write fails", body: body(first, `{"mqtt.pass":"`+rbBodySecret+`"}`),
			prepare: func() { rig.repo.fail = errors.New("write failed") },
			status:  http.StatusInternalServerError, message: "Internal"},
		{name: "422 a version with no stored text", body: body(noText, ""), status: http.StatusUnprocessableEntity, message: "holds no stored configuration"},
		{name: "422 a mask where no slot is", body: body(noSlotID, `{"custom.token":"`+rbBodySecret+`"}`), status: http.StatusUnprocessableEntity, message: "takes no secret"},
		{name: "422 a version the recompute would change", body: body(losesID, ""), status: http.StatusUnprocessableEntity, message: "would change when the configuration is recomputed"},
		{name: "404 a version of no device", body: body(999999, `{"mqtt.pass":"`+rbBodySecret+`"}`), status: http.StatusNotFound, message: "not found"},
		{name: "200 a version a legacy row holds in the clear", body: body(legacyID, ""), status: http.StatusOK, message: "Desired configuration set to version"},
		{name: "200 the secret is entered", body: body(second, `{"mqtt.pass":"`+rbBodySecret+`"}`), status: http.StatusOK, message: "Desired configuration set to version"},
		{name: "200 the same version again writes nothing", body: body(second, ""), status: http.StatusOK, message: "Nothing was written"},
	}

	for _, tc := range runs {
		t.Run(tc.name, func(t *testing.T) {
			if tc.prepare != nil {
				tc.prepare()
			}
			before := len(rig.log())
			w := rig.send("POST", rig.rollbackPath(), tc.body)
			written := rig.log()[before:]

			require.Equal(t, tc.status, w.Code, w.Body.String())
			var answer errorAnswer
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &answer))
			if tc.status != http.StatusOK {
				assert.Contains(t, answer.Error.Message, tc.message, w.Body.String())
			} else {
				assert.Contains(t, w.Body.String(), tc.message)
			}
			require.Contains(t, written, "/config/new/rollback", "this call wrote no line of its request, so a clean read proves nothing")
			for _, secret := range rbSecrets {
				assert.NotContains(t, written, secret, "the log of the call holds a secret")
				assert.NotContains(t, w.Body.String(), secret, "the answer holds a secret")
			}
		})
	}
}

// The capture holds the database's own trace too: without a statement in it, a clean read of the log
// would say nothing about what the database logs.
func TestRollbackRoute_TheLogHoldsTheDatabaseTraceToo(t *testing.T) {
	rig := newRBLogRig(t)
	putOverrides(t, rig.handler, rig.id, `{"mqtt":{"server":"a.example"}}`)
	assert.Contains(t, rig.log(), "SELECT", "the database trace is not in the capture")
}
