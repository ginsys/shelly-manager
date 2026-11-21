package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// A device that refuses the saved login, and the identity error, are answered the same way by every
// handler that talks to a device (ginsys/shelly-manager#814): a refused login is 502 with the message
// that names the fix, and a device that is not the stored one is 409 with the identity text.

// withRefusedLogin gives the record the saved login that the refusing device then rejects, and a
// desired configuration for the apply and verify routes.
func withRefusedLogin(t *testing.T, db *database.Manager, device *database.Device) {
	t.Helper()
	require.NoError(t, db.GetDB().Model(device).Update("settings",
		`{"gen":1,"auth_enabled":true,"auth_user":"old","auth_pass":"oldpw"}`).Error)
	require.NoError(t, db.GetDB().Model(device).Update("desired_config",
		`{"system":{"device":{"name":"fleet"}}}`).Error)
}

// deviceRoute is one handler that talks to a device, called for one device record.
type deviceRoute struct {
	name string
	call func(*Handler, *database.Device) *httptest.ResponseRecorder
}

func readRoute(name, path string, call func(*Handler, http.ResponseWriter, *http.Request)) deviceRoute {
	return deviceRoute{name, func(h *Handler, d *database.Device) *httptest.ResponseRecorder {
		return getDeviceRoute(h, call, "/api/v1/devices/"+strconv.Itoa(int(d.ID))+path, d.ID)
	}}
}

func postDeviceRoute(name, path string, call func(*Handler, http.ResponseWriter, *http.Request)) deviceRoute {
	return deviceRoute{name, func(h *Handler, d *database.Device) *httptest.ResponseRecorder {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		id := strconv.Itoa(int(d.ID))
		req := httptest.NewRequest("POST", "/api/v1/devices/"+id+path, nil).WithContext(ctx)
		req = mux.SetURLVars(req, map[string]string{"id": id})
		w := httptest.NewRecorder()
		call(h, w, req)
		return w
	}}
}

var (
	statusRoute     = readRoute("status", "/status", (*Handler).GetDeviceStatus)
	energyRoute     = readRoute("energy", "/energy", (*Handler).GetDeviceEnergy)
	currentRoute    = readRoute("current config", "/config/current", (*Handler).GetCurrentDeviceConfig)
	normalizedRoute = readRoute("current config, normalized", "/config/current/normalized", (*Handler).GetCurrentDeviceConfigNormalized)
	applyRoute      = postDeviceRoute("apply", "/config/new/apply", (*Handler).ApplyDeviceNewConfig)
	verifyRoute     = postDeviceRoute("verify", "/config/new/verify", (*Handler).VerifyDeviceNewConfig)
	controlRoute    = deviceRoute{"control", func(h *Handler, d *database.Device) *httptest.ResponseRecorder {
		return postControl(h, d.ID, "on")
	}}
)

// The identity error has three causes: another MAC answered, the record has no MAC, the device
// reports none. Every handler answers each of them 409 CONFLICT with the identity text, whether the
// device also refuses the saved login (the reads reach the error that way) or not.
func TestDeviceRoutes_AnswerTheIdentityErrorWith409(t *testing.T) {
	causes := []struct {
		name, stored, reports, want string
	}{
		{"another MAC answers", storedMAC, otherDeviceID, "stored MAC"},
		{"the record has no MAC", "", reportedMAC, "no MAC address"},
		{"the device reports none", storedMAC, "", "did not report a MAC"},
	}
	routes := []struct {
		deviceRoute
		refusing bool // the device refuses the saved login, as the reads need to reach the error
	}{
		{statusRoute, true}, {energyRoute, true}, {currentRoute, true}, {normalizedRoute, true},
		{applyRoute, false}, {applyRoute, true}, {verifyRoute, false}, {verifyRoute, true}, {controlRoute, false},
	}
	for _, route := range routes {
		for _, cause := range causes {
			name := route.name + "/" + cause.name
			if route.refusing {
				name += "/login refused"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				db, cleanup := testutil.TestDatabase(t)
				defer cleanup()
				handler := exportHandler(t, db)
				var addr string
				if route.refusing {
					addr = refusingDeviceReporting(t, false, cause.reports)
				} else {
					addr, _ = gen1Device(t, cause.reports, http.StatusOK)
				}
				device := deviceAt(t, db, addr, cause.stored)
				withRefusedLogin(t, db, device)

				w := route.call(handler, device)

				require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
				code, message := errorOf(t, w)
				assert.Equal(t, "CONFLICT", code)
				assert.Contains(t, message, cause.want)
			})
		}
	}
}

// A refused login stays a 502 on every handler that reads or writes a device, whatever the identity
// read that follows it does (ginsys/shelly-manager#759). The reads are tested in
// TestDeviceReads_ARefusedLoginIsA502WhateverTheIdentityReadDoes; apply, verify and the bulk apply are
// tested here (ginsys/shelly-manager#814).
func TestConfigRoutes_ARefusedLoginIsA502WhateverTheIdentityReadDoes(t *testing.T) {
	for _, probeHangs := range []bool{true, false} {
		for _, route := range []deviceRoute{applyRoute, verifyRoute} {
			name := route.name + "/identity read answers"
			if probeHangs {
				name = route.name + "/identity read times out"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				db, cleanup := testutil.TestDatabase(t)
				defer cleanup()
				handler := exportHandler(t, db)
				device := deviceAt(t, db, refusingDevice(t, probeHangs), storedMAC)
				withRefusedLogin(t, db, device)

				w := route.call(handler, device)

				require.Equal(t, http.StatusBadGateway, w.Code, w.Body.String())
				_, message := errorOf(t, w)
				assert.Contains(t, message, "rejected the stored credentials")
				assert.NotContains(t, message, "did not answer")
			})
		}
	}
}

// A bulk-apply entry whose device refuses the saved login records the rejected-login reason, also
// when the identity read that follows fails.
func TestBulkApply_ARefusedLoginIsRecordedWhateverTheIdentityReadDoes(t *testing.T) {
	for _, probeHangs := range []bool{true, false} {
		name := "identity read answers"
		if probeHangs {
			name = "identity read times out"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			db, cleanup := testutil.TestDatabase(t)
			defer cleanup()
			handler := exportHandler(t, db)
			device := deviceAt(t, db, refusingDevice(t, probeHangs), storedMAC)
			withRefusedLogin(t, db, device)

			result := decodeApply(t, postApply(handler, applyBody(applyNamePatch, false, device.ID)))

			require.Len(t, result.Results, 1)
			assert.Equal(t, reasonRejectedLogin, result.Results[0].Reason)
			assert.Equal(t, 1, result.Failed)
		})
	}
}
