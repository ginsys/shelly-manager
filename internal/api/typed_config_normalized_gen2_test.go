package api

import (
	"encoding/json"
	"net/http"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/testutil"
)

// The normalized read of a stored second-generation import (ginsys/shelly-manager#801): the location and
// the time zone come from the top-level location that the Gen2 reader fills, and raw.additional lists
// only the keys that nothing is read from, not a second copy of the sections the typed form holds.

type gen2Normalized struct {
	Location struct {
		Lat float64 `json:"lat"`
		Lng float64 `json:"lng"`
	} `json:"location"`
	Time struct {
		Timezone string `json:"timezone"`
	} `json:"time"`
	Raw struct {
		Additional map[string]json.RawMessage `json:"Additional"`
	} `json:"raw"`
}

func readGen2Normalized(t *testing.T, h *Handler, id string) (gen2Normalized, []string) {
	t.Helper()
	w := serve(h.GetTypedDeviceConfigNormalized, "GET", "/api/v1/devices/1/config/typed/normalized", id, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var n gen2Normalized
	require.NoError(t, json.Unmarshal(requireData(t, w)["configuration"], &n))
	var keys []string
	for k := range n.Raw.Additional {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return n, keys
}

func TestGetTypedDeviceConfigNormalized_Gen2(t *testing.T) {
	h, cleanup := typedHandler(t)
	defer cleanup()
	device := testutil.TestDevice()
	device.Type, device.Firmware = "SNSW-001P16EU", "1.4.4"
	device.Settings = `{"model":"SNSW-001P16EU","gen":2,"auth_enabled":false}`
	require.NoError(t, h.DB.AddDevice(device))
	require.NoError(t, h.Service.ConfigSvc.UpdateDeviceConfigFromJSON(device.ID, json.RawMessage(gen2Fixture(t))))
	id := idString(device)

	t.Run("an import", func(t *testing.T) {
		n, keys := readGen2Normalized(t, h, id)
		assert.Equal(t, 50.85, n.Location.Lat)
		assert.Equal(t, 4.35, n.Location.Lng)
		assert.Equal(t, "Europe/Brussels", n.Time.Timezone)
		assert.Equal(t, plus1PMUnread, keys, "only the keys nothing is read from are additional")
		assert.JSONEq(t, `{"enable":false,"pass":"********"}`, string(n.Raw.Additional["modbus"]), "the secret stays masked")
	})

	t.Run("after a typed edit the stale copy is not listed", func(t *testing.T) {
		get := serve(h.GetTypedDeviceConfig, "GET", "/api/v1/devices/1/config/typed", id, "")
		var read struct {
			Configuration map[string]json.RawMessage `json:"configuration"`
		}
		require.NoError(t, json.Unmarshal(dataOf(t, get.Body.Bytes()), &read))
		read.Configuration["cloud"] = json.RawMessage(`{"enable":false}`)
		body, err := json.Marshal(map[string]any{"configuration": read.Configuration})
		require.NoError(t, err)
		put := serve(h.UpdateTypedDeviceConfig, "PUT", "/api/v1/devices/1/config/typed", id, string(body))
		require.Equal(t, http.StatusOK, put.Code, put.Body.String())

		n, keys := readGen2Normalized(t, h, id)
		assert.Equal(t, plus1PMUnread, keys, "cloud, sys, wifi and the channels are not listed beside their typed values")
		assert.Equal(t, 50.85, n.Location.Lat)
	})
}
