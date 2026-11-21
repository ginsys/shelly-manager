package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/testutil"
)

// identityProbeUserAgent is what the service's identity read sends; the fake keys on it.
const identityProbeUserAgent = "shelly-manager/identity-probe"

// refusingDevice is a Gen1 device that refuses every request with 401, the saved login included,
// except the service's identity read. That read either never answers (probeHangs) or names storedMAC.
// Keying on the user agent reaches the identity read whatever order the client uses.
func refusingDevice(t *testing.T, probeHangs bool) (addr string) {
	t.Helper()
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.UserAgent() != identityProbeUserAgent {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if probeHangs {
			select {
			case <-release:
			case <-r.Context().Done():
			}
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"type": "SHPLG-S", "mac": reportedMAC, "auth": true, "fw": "1.14.0"})
	}))
	t.Cleanup(func() { close(release); server.Close() })
	return strings.TrimPrefix(server.URL, "http://")
}

// A device that refuses the saved login is answered 502 with the credentials message on every read,
// also when the identity read that follows the refusal fails (ginsys/shelly-manager#759). Status and
// energy reach the builder's second clear site, current config the first. When the identity read
// times out, all four said "did not answer" or a bare 500 before. When it answers with the stored
// MAC the login is cleared and retried and neither clear site returns early: those rows are
// controls, and only status and energy answered 500 there before.
func TestDeviceReads_ARefusedLoginIsA502WhateverTheIdentityReadDoes(t *testing.T) {
	routes := []struct {
		name string
		path string
		call func(*Handler, http.ResponseWriter, *http.Request)
	}{
		{"status", "/api/v1/devices/%d/status", (*Handler).GetDeviceStatus},
		{"energy", "/api/v1/devices/%d/energy", (*Handler).GetDeviceEnergy},
		{"current config", "/api/v1/devices/%d/config/current", (*Handler).GetCurrentDeviceConfig},
		{"current config, normalized", "/api/v1/devices/%d/config/current/normalized", (*Handler).GetCurrentDeviceConfigNormalized},
	}
	for _, probeHangs := range []bool{true, false} {
		for _, route := range routes {
			name := route.name + "/identity read answers"
			if probeHangs {
				name = route.name + "/identity read times out"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				db, cleanup := testutil.TestDatabase(t)
				defer cleanup()
				handler := exportHandler(t, db)
				addr := refusingDevice(t, probeHangs)
				device := deviceAt(t, db, addr, storedMAC)
				require.NoError(t, db.GetDB().Model(device).Update("settings",
					`{"gen":1,"auth_enabled":true,"auth_user":"old","auth_pass":"oldpw"}`).Error)
				path := strings.Replace(route.path, "%d", strconv.Itoa(int(device.ID)), 1)

				w := getDeviceRoute(handler, route.call, path, device.ID)

				require.Equal(t, http.StatusBadGateway, w.Code, w.Body.String())
				_, message := errorOf(t, w)
				assert.Contains(t, message, "rejected the stored credentials")
				assert.NotContains(t, message, "did not answer")
			})
		}
	}
}
