package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// An override of only an empty value is accepted, and the desired configuration it produces holds no
// section for Apply to refuse (ginsys/shelly-manager#843).
func TestDeviceOverrides_AnEmptyValueAloneAnswers200AndLeavesNoSection(t *testing.T) {
	const body = `{"system":{"sntp":{"server":""}}}`
	routes := map[string]func(*Handler, http.ResponseWriter, *http.Request){
		"PUT":   func(h *Handler, w http.ResponseWriter, r *http.Request) { h.SetDeviceNewOverrides(w, r) },
		"PATCH": func(h *Handler, w http.ResponseWriter, r *http.Request) { h.PatchDeviceNewOverrides(w, r) },
	}
	for method, call := range routes {
		t.Run(method, func(t *testing.T) {
			handler, cleanup := setupTestHandler(t)
			defer cleanup()
			device := testutil.TestDevice()
			require.NoError(t, handler.DB.AddDevice(device))

			w := httptest.NewRecorder()
			call(handler, w, overridesRequest(t, method, device.ID, body))
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())

			stored, err := handler.DB.GetDevice(device.ID)
			require.NoError(t, err)
			var desired configuration.DeviceConfiguration
			require.NoError(t, json.Unmarshal([]byte(stored.DesiredConfig), &desired))
			assert.Nil(t, desired.System, "the desired configuration holds a section: %s", stored.DesiredConfig)
		})
	}
}
