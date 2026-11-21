package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// The generation of a stored device (ginsys/shelly-manager#798): the `gen` setting first, then the shape
// of the firmware, and 0 where neither says. Unknown claims no Bluetooth and no Ethernet.

const (
	gen1Firmware   = "20230913-114008/v1.14.0-gcb84623" // the Gen1 /shelly "fw" field
	gen2Firmware   = "1.4.4"                            // the Gen2 "ver" field
	gen2FirmwareID = "20241011-114455/1.4.4-g6d2a586"   // the Gen2 "fw_id" field
	buildIDOnly    = "20231219-134356"
)

var generationCases = []struct {
	name     string
	firmware string
	settings string
	want     int
}{
	{"Gen1 firmware, no gen", gen1Firmware, `{}`, 1},
	{"Gen1 firmware v1.0.0, no gen", "v1.0.0", `{}`, 1},
	{"Gen2 firmware 1.4.4, no gen", gen2Firmware, `{}`, 2},
	{"Gen3 firmware 1.2.0, no gen", "1.2.0", `{}`, 2},
	{"Gen2 fw_id, no gen", gen2FirmwareID, `{}`, 2},
	{"empty firmware, no gen", "", `{}`, 0},
	{"build id only, no gen", buildIDOnly, `{}`, 0},
	{"junk firmware, no gen", "unknown", `{}`, 0},
	{"gen 1 wins over Gen2 firmware", gen2Firmware, `{"gen":1}`, 1},
	{"gen 2 wins over Gen1 firmware", gen1Firmware, `{"gen":2}`, 2},
	{"gen 3 with empty firmware", "", `{"gen":3}`, 3},
	{"gen 1 with empty firmware", "", `{"gen":1}`, 1},
	{"fractional gen is no value: Gen2 firmware", gen2Firmware, `{"gen":2.5}`, 2},
	{"fractional gen is no value: empty firmware", "", `{"gen":2.5}`, 0},
	{"gen 0 is no value", gen2Firmware, `{"gen":0}`, 2},
	{"negative gen is no value", "", `{"gen":-1}`, 0},
	{"text gen is no value", gen1Firmware, `{"gen":"2"}`, 1},
	{"null gen is no value", "", `{"gen":null}`, 0},
}

func TestDeviceGeneration(t *testing.T) {
	for _, tc := range generationCases {
		t.Run(tc.name, func(t *testing.T) {
			got := deviceGeneration(&database.Device{Firmware: tc.firmware, Settings: tc.settings})
			assert.Equal(t, tc.want, got)
		})
	}

	t.Run("settings that are not JSON fall back to the firmware", func(t *testing.T) {
		assert.Equal(t, 1, deviceGeneration(&database.Device{Firmware: gen1Firmware, Settings: `{`}))
		assert.Equal(t, generationUnknown, deviceGeneration(&database.Device{Settings: `{`}))
		assert.Equal(t, generationUnknown, deviceGeneration(&database.Device{}))
	})
}

// The capabilities route answers the generation and lists ble and ethernet for 2 and later only.
func TestGetDeviceCapabilities_GenerationFromSettingsThenFirmware(t *testing.T) {
	h, cleanup := typedHandler(t)
	defer cleanup()

	for i, tc := range generationCases {
		t.Run(tc.name, func(t *testing.T) {
			device := testutil.TestDevice()
			device.IP = fmt.Sprintf("192.0.2.%d", i+1)
			device.MAC = fmt.Sprintf("AABBCCDDEE%02X", i+1)
			device.Firmware = tc.firmware
			device.Settings = tc.settings
			require.NoError(t, h.DB.AddDevice(device))

			w := serve(h.GetDeviceCapabilities, "GET", "/api/v1/devices/1/capabilities", idString(device), "")
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			var body struct {
				Generation   int      `json:"generation"`
				Capabilities []string `json:"capabilities"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			assert.Equal(t, tc.want, body.Generation)
			if tc.want >= 2 {
				assert.Contains(t, body.Capabilities, "ble")
				assert.Contains(t, body.Capabilities, "ethernet")
			} else {
				assert.NotContains(t, body.Capabilities, "ble")
				assert.NotContains(t, body.Capabilities, "ethernet")
			}
		})
	}
}

const bleConfig = `{"system":{"device":{"ble":{"enable":true}}}}`

func hasWarning(result []string, code string) bool {
	for _, c := range result {
		if c == code {
			return true
		}
	}
	return false
}

// The validator reads the generation as the capabilities route does: a stored device's `gen` counts.
// Only a Gen1 device draws the Bluetooth warning, and an unknown generation draws none.
func TestCreateValidator_GenerationFromSettingsThenFirmware(t *testing.T) {
	h := &Handler{}
	for _, tc := range generationCases {
		t.Run(tc.name, func(t *testing.T) {
			device := &database.Device{Type: "SHSW-1", Firmware: tc.firmware, Settings: tc.settings}
			result := h.createValidator("basic", device).ValidateConfiguration(json.RawMessage(bleConfig))

			var codes []string
			for _, w := range result.Warnings {
				codes = append(codes, w.Code)
			}
			assert.Equal(t, tc.want == 1, hasWarning(codes, "BLE_NOT_SUPPORTED_GEN1"))
		})
	}
}

// A request that names a generation builds a device context with that generation: 1 draws the Gen1
// warning, 2 and an omitted generation do not.
func TestValidateTypedConfig_RequestGeneration(t *testing.T) {
	h := &Handler{}
	for _, tc := range []struct {
		name       string
		generation string
		warns      bool
	}{{"generation 1", `,"generation":1`, true}, {"generation 2", `,"generation":2`, false}, {"no generation", ``, false}} {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"configuration":` + bleConfig + `,"device_model":"SHSW-1"` + tc.generation + `}`
			w := serve(h.ValidateTypedConfig, "POST", "/api/v1/config/validate-typed", "", body)
			var warnings []struct {
				Code string `json:"code"`
			}
			// The key is left out when there is no warning.
			if raw, ok := requireData(t, w)["warnings"]; ok {
				require.NoError(t, json.Unmarshal(raw, &warnings))
			}
			var codes []string
			for _, w := range warnings {
				codes = append(codes, w.Code)
			}
			assert.Equal(t, tc.warns, hasWarning(codes, "BLE_NOT_SUPPORTED_GEN1"))
		})
	}
}
