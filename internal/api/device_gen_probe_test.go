package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/shelly"
)

// A manually added device keeps the generation it really has (ginsys/shelly-manager#811). The UI add
// form sends no settings, so the create asks the device once, and the update keeps what is stored.

// probe replaces the generation probe for one test and counts its calls. The test is not parallel,
// and the variable is restored when it ends.
type probe struct{ calls atomic.Int32 }

func withProbe(t *testing.T, gen int, err error) *probe {
	t.Helper()
	p := &probe{}
	previous := detectGeneration
	detectGeneration = func(ctx context.Context, ip string) (int, error) {
		p.calls.Add(1)
		return gen, err
	}
	t.Cleanup(func() { detectGeneration = previous })
	return p
}

// storedSettings is the settings document stored for the device, as a map.
func storedSettings(t *testing.T, h *Handler, id uint) map[string]any {
	t.Helper()
	device, err := h.DB.GetDevice(id)
	require.NoError(t, err)
	var settings map[string]any
	require.NoError(t, json.Unmarshal([]byte(device.Settings), &settings), device.Settings)
	return settings
}

// createDevice posts a device and returns the id the route answered.
func createDevice(t *testing.T, h *Handler, settings string) (uint, int) {
	t.Helper()
	body, err := json.Marshal(database.Device{
		IP: "192.0.2.10", MAC: "AA:BB:CC:DD:EE:01", Type: "Test Device", Name: "Manual", Settings: settings,
	})
	require.NoError(t, err)
	w := serve(h.AddDevice, "POST", "/api/v1/devices", "", string(body))
	if w.Code != http.StatusCreated {
		return 0, w.Code
	}
	var wrap struct {
		Data struct {
			ID uint `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &wrap), w.Body.String())
	return wrap.Data.ID, w.Code
}

func TestAddDevice_StoresTheGenerationTheDeviceReports(t *testing.T) {
	for _, tc := range []struct {
		name     string
		settings string
		detected int
	}{
		{"empty settings, Gen2", "", 2},
		{"settings with no gen, Gen2", `{"model":"SNSW-001X16EU"}`, 2},
		{"settings with no gen, Gen3", `{"model":"S3SW-001X16EU","auth_enabled":true}`, 3},
		{"settings with no gen, Gen1", `{"model":"SHPLG-S"}`, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, cleanup := typedHandler(t)
			defer cleanup()
			p := withProbe(t, tc.detected, nil)

			id, code := createDevice(t, h, tc.settings)

			require.Equal(t, http.StatusCreated, code)
			assert.EqualValues(t, 1, p.calls.Load(), "the device is asked once")
			settings := storedSettings(t, h, id)
			assert.EqualValues(t, tc.detected, settings["gen"])
			assert.Contains(t, settings, "model")
			assert.Contains(t, settings, "auth_enabled")
		})
	}
}

// A device that does not answer, or that is not a Shelly, stays Gen1 as before: the create is not refused.
func TestAddDevice_ADeviceThatCannotBeAskedStaysGen1(t *testing.T) {
	for name, probeErr := range map[string]error{
		"unreachable":  fmt.Errorf("device not reachable at 192.0.2.10: %w", context.DeadlineExceeded),
		"not a Shelly": shelly.ErrInvalidGeneration,
		"a bad answer": errors.New("no device is probed in a test"),
	} {
		t.Run(name, func(t *testing.T) {
			h, cleanup := typedHandler(t)
			defer cleanup()
			p := withProbe(t, 0, probeErr)

			id, code := createDevice(t, h, "")

			require.Equal(t, http.StatusCreated, code)
			assert.EqualValues(t, 1, p.calls.Load())
			assert.EqualValues(t, 1, storedSettings(t, h, id)["gen"])
		})
	}
}

func TestAddDevice_AGivenGenerationIsKeptAndNothingIsAsked(t *testing.T) {
	for name, tc := range map[string]struct {
		given string
		want  any
	}{
		"a number":         {`2`, float64(2)},
		"zero":             {`0`, float64(0)},
		"null":             {`null`, nil},
		"a string":         {`"3"`, "3"},
		"a negative gen":   {`-1`, float64(-1)},
		"a fractional gen": {`2.5`, 2.5},
	} {
		t.Run(name, func(t *testing.T) {
			h, cleanup := typedHandler(t)
			defer cleanup()
			p := withProbe(t, 3, nil)

			id, code := createDevice(t, h, `{"model":"SNSW-001X16EU","gen":`+tc.given+`}`)

			require.Equal(t, http.StatusCreated, code)
			assert.EqualValues(t, 0, p.calls.Load(), "a given gen is not asked about")
			settings := storedSettings(t, h, id)
			assert.Contains(t, settings, "gen")
			assert.Equal(t, tc.want, settings["gen"])
		})
	}
}

func TestAddDevice_InvalidSettingsAreRefusedBeforeTheDeviceIsAsked(t *testing.T) {
	h, cleanup := typedHandler(t)
	defer cleanup()
	p := withProbe(t, 2, nil)

	_, code := createDevice(t, h, `{"model":"SHPLG-S",}`)

	assert.Equal(t, http.StatusBadRequest, code)
	assert.EqualValues(t, 0, p.calls.Load(), "a refused request sends nothing")
}

func TestUpdateDevice_KeepsTheStoredGenerationAndNeverWritesOne(t *testing.T) {
	for _, tc := range []struct {
		name     string
		stored   string
		sent     string
		wantGen  any // nil: no gen key at all
		wantKeys []string
	}{
		{"stored Gen2, body with no gen", `{"model":"M","gen":2,"auth_enabled":false}`, `{"model":"M2"}`, float64(2), []string{"model"}},
		{"stored Gen2, empty body settings", `{"model":"M","gen":2,"auth_enabled":false}`, ``, float64(2), nil},
		{"stored with no gen, body with no gen", `{"model":"M","auth_enabled":false}`, `{"model":"M2"}`, nil, []string{"model"}},
		{"a gen in the body wins", `{"model":"M","gen":2,"auth_enabled":false}`, `{"model":"M","gen":3}`, float64(3), nil},
		// A create keeps any gen it is given, so a stored one need not be a positive whole number.
		// An update keeps it exactly as it was.
		{"stored gen is a string", `{"model":"M","gen":"2"}`, `{"model":"M2"}`, "2", []string{"model"}},
		{"stored gen is 0", `{"model":"M","gen":0}`, `{"model":"M2"}`, float64(0), []string{"model"}},
		{"stored gen is negative", `{"model":"M","gen":-1}`, `{"model":"M2"}`, float64(-1), []string{"model"}},
		{"stored gen is fractional", `{"model":"M","gen":2.5}`, `{"model":"M2"}`, 2.5, []string{"model"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, cleanup := typedHandler(t)
			defer cleanup()
			p := withProbe(t, 3, nil)
			device := &database.Device{IP: "192.0.2.11", MAC: "AA:BB:CC:DD:EE:02", Type: "Test Device", Name: "Manual", Settings: tc.stored}
			require.NoError(t, h.DB.AddDevice(device))
			body, err := json.Marshal(database.Device{IP: device.IP, MAC: device.MAC, Type: device.Type, Name: "Renamed", Settings: tc.sent})
			require.NoError(t, err)

			w := serve(h.UpdateDevice, "PUT", "/api/v1/devices/"+idString(device), idString(device), string(body))

			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			assert.EqualValues(t, 0, p.calls.Load(), "an update never asks the device")
			settings := storedSettings(t, h, device.ID)
			if tc.wantGen == nil {
				assert.NotContains(t, settings, "gen", "an update never writes a gen the row did not have")
			} else {
				assert.Equal(t, tc.wantGen, settings["gen"])
			}
			for _, key := range tc.wantKeys {
				assert.Contains(t, settings, key)
			}
		})
	}
}

// A create keeps `gen: null`, so a row can hold it, and an update keeps the key too: absent and null are
// two different stored states, which the table above cannot tell apart.
func TestUpdateDevice_KeepsAStoredNullGen(t *testing.T) {
	h, cleanup := typedHandler(t)
	defer cleanup()
	p := withProbe(t, 3, nil)
	device := &database.Device{IP: "192.0.2.12", MAC: "AA:BB:CC:DD:EE:03", Type: "Test Device", Name: "Manual", Settings: `{"model":"M","gen":null}`}
	require.NoError(t, h.DB.AddDevice(device))
	body, err := json.Marshal(database.Device{IP: device.IP, MAC: device.MAC, Type: device.Type, Name: "Renamed", Settings: `{"model":"M2"}`})
	require.NoError(t, err)

	w := serve(h.UpdateDevice, "PUT", "/api/v1/devices/"+idString(device), idString(device), string(body))

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.EqualValues(t, 0, p.calls.Load(), "an update never asks the device")
	settings := storedSettings(t, h, device.ID)
	require.Contains(t, settings, "gen", "the stored null is carried, never dropped")
	assert.Nil(t, settings["gen"])
}

// The symptom of ginsys/shelly-manager#811: a Gen2 device added by hand listed Gen1's capabilities.
func TestAddDevice_AGen2DeviceAddedByHandReadsAsGen2(t *testing.T) {
	for _, tc := range []struct {
		name       string
		probeGen   int
		probeErr   error
		wantGen    int
		wantBLE    bool
		wantDetail string
	}{
		{"the device answers as Gen2", 2, nil, 2, true, "ble and ethernet listed"},
		{"the device cannot be asked", 0, errors.New("unreachable"), 1, false, "Gen1 as before"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, cleanup := typedHandler(t)
			defer cleanup()
			withProbe(t, tc.probeGen, tc.probeErr)
			id, code := createDevice(t, h, "")
			require.Equal(t, http.StatusCreated, code)

			w := serve(h.GetDeviceCapabilities, "GET", fmt.Sprintf("/api/v1/devices/%d/capabilities", id), fmt.Sprint(id), "")

			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			var body struct {
				Generation   int      `json:"generation"`
				Capabilities []string `json:"capabilities"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			assert.Equal(t, tc.wantGen, body.Generation, tc.wantDetail)
			assert.Equal(t, tc.wantBLE, strings.Contains(strings.Join(body.Capabilities, ","), "ble"), tc.wantDetail)
		})
	}
}

// The 2 s deadline bounds both of the probe's requests and comes from the request: a caller that goes
// away ends the probe. A failed probe is logged with the reason, so the log says why the device is Gen 1.
func TestProbeGeneration_IsBoundedByTheRequestAndSaysWhyItFellBack(t *testing.T) {
	newHandler := func(t *testing.T) (*Handler, string) {
		t.Helper()
		path := filepath.Join(t.TempDir(), "probe.log")
		logger, err := logging.New(logging.Config{Level: "warn", Format: "text", Output: path})
		require.NoError(t, err)
		t.Cleanup(func() { _ = logger.Close() })
		return &Handler{logger: logger}, path
	}

	t.Run("a deadline of 2 s, and a cancelled request ends it", func(t *testing.T) {
		h, _ := newHandler(t)
		var remaining time.Duration
		var hasDeadline bool
		previous := detectGeneration
		detectGeneration = func(ctx context.Context, ip string) (int, error) {
			deadline, ok := ctx.Deadline()
			hasDeadline, remaining = ok, time.Until(deadline)
			return 2, nil
		}
		t.Cleanup(func() { detectGeneration = previous })

		assert.Equal(t, 2, h.probeGeneration(context.Background(), "192.0.2.10"))
		assert.True(t, hasDeadline, "the probe has a deadline")
		assert.LessOrEqual(t, remaining, 2*time.Second)
		assert.Greater(t, remaining, time.Second, "the deadline is 2 s, not a shorter one")

		cancelled, cancel := context.WithCancel(context.Background())
		cancel()
		var seen error
		detectGeneration = func(ctx context.Context, ip string) (int, error) {
			seen = ctx.Err()
			return 0, ctx.Err()
		}
		assert.Equal(t, 1, h.probeGeneration(cancelled, "192.0.2.10"))
		assert.Error(t, seen, "the probe's context follows the request's")
	})

	for name, tc := range map[string]struct {
		err  error
		want string
	}{
		"no answer":    {fmt.Errorf("device not reachable at 192.0.2.10: %w", context.DeadlineExceeded), "the device did not answer"},
		"not a Shelly": {shelly.ErrInvalidGeneration, "not a Shelly device"},
	} {
		t.Run("log: "+name, func(t *testing.T) {
			h, path := newHandler(t)
			withProbe(t, 0, tc.err)

			assert.Equal(t, 1, h.probeGeneration(context.Background(), "192.0.2.10"))

			logged, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Contains(t, string(logged), tc.want)
			assert.Contains(t, string(logged), "192.0.2.10")
		})
	}
}
