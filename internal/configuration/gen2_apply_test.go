package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/shelly"
)

// gen2Call is one call to a Gen2 <Component>.SetConfig method.
type gen2Call struct {
	method string
	id     int // the component id, or -1 for a component with none
	config map[string]interface{}
}

// fakeGen2Device is a Gen2 device. It keeps the Shelly.GetConfig document between writes. A write
// stores exactly the keys it carries: the official documentation does not say that SetConfig
// merges, so the fake never fills in a key the caller left out.
type fakeGen2Device struct {
	applyTestClient
	doc          map[string]interface{}
	calls        []gen2Call
	restart      bool  // the answer's restart_required
	refuse       error // the device refuses every write
	ignoreWrites bool  // the device accepts a write but keeps its old document
	legacyCalls  int   // calls to the Gen1-shaped SetConfig, which a Gen2 apply must never make
}

func newFakeGen2Device() *fakeGen2Device {
	d := &fakeGen2Device{}
	d.ip = "192.168.1.60"
	d.generation = 2
	d.doc = map[string]interface{}{
		"sys": map[string]interface{}{
			"device":   map[string]interface{}{"name": nil, "mac": "AABBCCDDEEFF", "eco_mode": false, "discoverable": true},
			"location": map[string]interface{}{"tz": "Europe/Brussels", "lat": 50.8, "lon": 4.3},
			"sntp":     map[string]interface{}{"server": "time.google.com"},
		},
		"wifi":  map[string]interface{}{"sta": map[string]interface{}{"ssid": "home", "enable": true}},
		"mqtt":  map[string]interface{}{"enable": false, "server": nil},
		"cloud": map[string]interface{}{"enable": true, "server": "iot.shelly.cloud:6012/jrpc"},
	}
	return d
}

func (d *fakeGen2Device) SetConfig(context.Context, map[string]interface{}) error {
	d.legacyCalls++
	return errors.New("the Gen1-shaped SetConfig was called on a Gen2 device")
}

func (d *fakeGen2Device) GetConfig(context.Context) (*shelly.DeviceConfig, error) {
	raw, err := json.Marshal(d.doc)
	if err != nil {
		return nil, err
	}
	return &shelly.DeviceConfig{Raw: raw}, nil
}

func (d *fakeGen2Device) CallConfigSetter(_ context.Context, method string, params map[string]interface{}) (bool, error) {
	if d.refuse != nil {
		return false, d.refuse
	}
	config, _ := params["config"].(map[string]interface{})
	key := strings.ToLower(strings.TrimSuffix(method, ".SetConfig"))
	id := -1
	if n, ok := params["id"].(int); ok {
		id = n
		key = fmt.Sprintf("%s:%d", key, n)
	}
	d.calls = append(d.calls, gen2Call{method: method, id: id, config: config})
	if method == "Switch.SetConfig" {
		if err := d.checkInMode(id, config); err != nil {
			return false, err
		}
	}
	if section, ok := d.doc[key].(map[string]interface{}); ok && !d.ignoreWrites {
		storeKeys(section, config)
		// A device never returns the MQTT password.
		delete(section, "pass")
	}
	return d.restart, nil
}

// checkInMode refuses an in_mode that the type of the switch's input cannot drive, as a device
// does: follow and flip need a stateful input (switch), momentary and cycle a stateless one (button).
func (d *fakeGen2Device) checkInMode(id int, config map[string]interface{}) error {
	mode, ok := config["in_mode"].(string)
	if !ok {
		return nil
	}
	// The switch runs on the input it names (input_id), or on its own number.
	inputID := id
	if sw, ok := d.doc[fmt.Sprintf("switch:%d", id)].(map[string]interface{}); ok {
		if selected, ok := sw["input_id"].(float64); ok {
			inputID = int(selected)
		}
	}
	input, _ := d.doc[fmt.Sprintf("input:%d", inputID)].(map[string]interface{})
	kind, _ := input["type"].(string)
	switch {
	case (mode == "follow" || mode == "flip") && kind != "switch":
		return fmt.Errorf("in_mode %s needs a switch input, input %d is %q", mode, id, kind)
	case (mode == "momentary" || mode == "cycle") && kind != "button":
		return fmt.Errorf("in_mode %s needs a button input, input %d is %q", mode, id, kind)
	}
	return nil
}

// storeKeys writes the keys of src into dst, one level at a time, and leaves every other key of
// dst as it was.
func storeKeys(dst, src map[string]interface{}) {
	for key, value := range src {
		if srcMap, ok := value.(map[string]interface{}); ok {
			if dstMap, ok := dst[key].(map[string]interface{}); ok {
				storeKeys(dstMap, srcMap)
				continue
			}
		}
		dst[key] = value
	}
}

func newGen2Service(t *testing.T, desired *DeviceConfiguration) (*ConfigurationService, *mockRepository) {
	t.Helper()
	repo := newMockRepository()
	repo.devices[1] = &ServiceDevice{ID: 1}
	setDesired(t, repo, desired)
	return NewConfigurationService(repo, nil, nil), repo
}

func systemDesired() *DeviceConfiguration {
	return &DeviceConfiguration{
		System: &SystemConfiguration{
			Device: &TypedDeviceConfig{Name: StringPtr("lamp"), EcoMode: BoolPtr(true), Discoverable: BoolPtr(false)},
			SNTP:   &SNTPConfig{Server: "ntp.example.net"},
		},
		Location: &LocationConfiguration{Timezone: StringPtr("Europe/Amsterdam"), Latitude: Float64Ptr(52.37), Longitude: Float64Ptr(4.9)},
	}
}

func TestGen2Converter_ReadsTheSysSection(t *testing.T) {
	device := newFakeGen2Device()
	device.doc["sys"].(map[string]interface{})["device"].(map[string]interface{})["name"] = "porch"
	raw, err := json.Marshal(device.doc)
	require.NoError(t, err)

	got, err := NewGen2Converter(nil).FromAPIConfig(raw, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.Equal(t, 2, NewGen2Converter(nil).Generation())
	require.NotNil(t, got.System)
	assert.Equal(t, "porch", *got.System.Device.Name)
	assert.False(t, *got.System.Device.EcoMode)
	assert.True(t, *got.System.Device.Discoverable)
	assert.Equal(t, "time.google.com", got.System.SNTP.Server)
	assert.Equal(t, "Europe/Brussels", *got.Location.Timezone)
	assert.Equal(t, 50.8, *got.Location.Latitude)
	assert.Equal(t, 4.3, *got.Location.Longitude)
}

// A null name is a device with no name set. It reads back as an unset name, never as an empty one.
func TestGen2Converter_ANullNameIsUnset(t *testing.T) {
	raw, err := json.Marshal(newFakeGen2Device().doc)
	require.NoError(t, err)

	got, err := NewGen2Converter(nil).FromAPIConfig(raw, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.Nil(t, got.System.Device.Name)
}

func TestGen2Converter_WritesTheSysShapeAndNothingElse(t *testing.T) {
	raw, err := NewGen2Converter(nil).ToAPIConfig(systemDesired(), "SNSW-001X16EU")

	require.NoError(t, err)
	assert.JSONEq(t, `{"sys":{
		"device":{"name":"lamp","eco_mode":true,"discoverable":false},
		"location":{"tz":"Europe/Amsterdam","lat":52.37,"lon":4.9},
		"sntp":{"server":"ntp.example.net"}}}`, string(raw))
}

func TestApplyDesiredConfig_Gen2WritesSysThenReadsItBack(t *testing.T) {
	svc, repo := newGen2Service(t, systemDesired())
	dev := newFakeGen2Device()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied)
	assert.True(t, repo.devices[1].ConfigApplied)
	assert.Equal(t, 0, dev.legacyCalls)
	require.Len(t, dev.calls, 1)
	assert.Equal(t, "Sys.SetConfig", dev.calls[0].method)
	assert.Equal(t, map[string]interface{}{
		"device":   map[string]interface{}{"name": "lamp", "eco_mode": true, "discoverable": false},
		"location": map[string]interface{}{"tz": "Europe/Amsterdam", "lat": 52.37, "lon": 4.9},
		"sntp":     map[string]interface{}{"server": "ntp.example.net"},
	}, dev.calls[0].config)
	assert.Equal(t, 7, result.ApplyResult.AppliedCount)
	assert.Equal(t, 0, result.ApplyResult.FailedCount)
	assert.Equal(t, 0, dev.reboots, "restart_required was false")
	assert.False(t, result.ApplyResult.RequiresReboot)
}

// The documentation does not say that SetConfig merges, so a key the desired configuration leaves
// out is never sent.
func TestApplyDesiredConfig_Gen2SendsOnlyTheKeysTheDesiredConfigSets(t *testing.T) {
	svc, _ := newGen2Service(t, &DeviceConfiguration{
		System: &SystemConfiguration{Device: &TypedDeviceConfig{Name: StringPtr("lamp")}},
	})
	dev := newFakeGen2Device()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied)
	require.Len(t, dev.calls, 1)
	assert.Equal(t, map[string]interface{}{"device": map[string]interface{}{"name": "lamp"}}, dev.calls[0].config)
	assert.Equal(t, 1, result.ApplyResult.AppliedCount)
}

func TestApplyDesiredConfig_Gen2RebootsOnlyWhenTheAnswerAsksForIt(t *testing.T) {
	svc, repo := newGen2Service(t, systemDesired())
	dev := newFakeGen2Device()
	dev.restart = true

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.True(t, result.ApplyResult.RequiresReboot)
	assert.Equal(t, 1, dev.reboots)
	assert.True(t, result.ConfigApplied)
	assert.False(t, repo.devices[1].PendingReboot, "cleared once the device rebooted")
}

// A reboot that the device asked for stays owed when a later step fails: the first answer is
// recorded before the apply goes on.
func TestApplyDesiredConfig_Gen2KeepsAnAskedRebootOwedWhenTheRebootFails(t *testing.T) {
	svc, repo := newGen2Service(t, systemDesired())
	dev := newFakeGen2Device()
	dev.restart = true
	dev.rebootErr = errors.New("no route")

	_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.Error(t, err)
	assert.True(t, repo.devices[1].PendingReboot)
}

// When the restart the device asked for cannot be recorded, a failed reboot would leave nothing
// owed, so the apply fails and names the reboot instead of going on.
func TestApplyDesiredConfig_Gen2FailsWhenTheAskedRebootCannotBeRecorded(t *testing.T) {
	svc, repo := newGen2Service(t, systemDesired())
	repo.pendingRebootErr = errors.New("database is locked")
	dev := newFakeGen2Device()
	dev.restart = true

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.False(t, result.ConfigApplied)
	assert.False(t, result.ApplyResult.Success)
	assert.Equal(t, 1, result.ApplyResult.FailedCount)
	assert.Equal(t, "reboot", result.ApplyResult.Failures[len(result.ApplyResult.Failures)-1].Path)
	// The API lists a failure by its path and returns the warnings in full, so the instruction
	// is a warning.
	assert.Contains(t, strings.Join(result.ApplyResult.Warnings, " "), "Reboot the device by hand")
	assert.Equal(t, 0, dev.reboots, "an apply that failed does not reboot")
	assert.Nil(t, result.VerifyResult)
}

func TestApplyDesiredConfig_Gen2StaysPendingWhenTheDeviceRefusesTheWrite(t *testing.T) {
	svc, repo := newGen2Service(t, systemDesired())
	dev := newFakeGen2Device()
	dev.refuse = errors.New("invalid argument")

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.False(t, result.ConfigApplied)
	assert.False(t, result.ApplyResult.Success)
	assert.Equal(t, 0, result.ApplyResult.AppliedCount)
	assert.Equal(t, 7, result.ApplyResult.FailedCount)
	assert.Contains(t, result.ApplyResult.Failures[0].Path, "sys.")
	assert.Nil(t, result.VerifyResult, "no verify after a failed apply")
	assert.False(t, repo.devices[1].ConfigApplied)
	assert.Equal(t, 0, dev.reboots)
}

func TestApplyDesiredConfig_Gen2StaysPendingWhenTheDeviceIgnoresTheWrite(t *testing.T) {
	svc, repo := newGen2Service(t, systemDesired())
	dev := newFakeGen2Device()
	dev.ignoreWrites = true

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.False(t, result.ConfigApplied)
	require.NotNil(t, result.VerifyResult)
	assert.False(t, result.VerifyResult.Match)
	assert.False(t, repo.devices[1].ConfigApplied)
}

// Sections that this change does not write yet are refused before the device is touched, never
// dropped.
func TestApplyDesiredConfig_Gen2RefusesSectionsItCannotWriteYet(t *testing.T) {
	cases := map[string]struct {
		desired *DeviceConfiguration
		path    string
	}{
		"wifi":   {&DeviceConfiguration{WiFi: &WiFiConfiguration{SSID: StringPtr("home")}}, "wifi"},
		"login":  {&DeviceConfiguration{Auth: &AuthConfiguration{Enable: BoolPtr(true), Username: StringPtr("a"), Password: StringPtr("b")}}, "auth"},
		"roller": {&DeviceConfiguration{Roller: &RollerConfig{MotorSpeed: IntPtr(50)}}, "roller"},
		"mixed": {&DeviceConfiguration{
			System: &SystemConfiguration{Device: &TypedDeviceConfig{Name: StringPtr("lamp")}},
			WiFi:   &WiFiConfiguration{SSID: StringPtr("home")},
		}, "wifi"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svc, repo := newGen2Service(t, tc.desired)
			dev := newFakeGen2Device()

			result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

			require.Error(t, err)
			assert.Nil(t, result)
			assert.True(t, errors.Is(err, ErrUnsupportedSettings))
			assert.Contains(t, err.Error(), tc.path)
			assert.Empty(t, dev.calls, "nothing is written when a setting is refused")
			assert.Equal(t, 0, dev.legacyCalls)
			assert.False(t, repo.devices[1].ConfigApplied)
		})
	}
}

func TestApplyDesiredConfig_Gen2RefusesATimeServerThatIsNotAHost(t *testing.T) {
	svc, _ := newGen2Service(t, &DeviceConfiguration{System: &SystemConfiguration{SNTP: &SNTPConfig{Server: "not a host!"}}})
	dev := newFakeGen2Device()

	_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnsupportedSettings))
	assert.Contains(t, err.Error(), "system.sntp.server")
	assert.Empty(t, dev.calls)
}

// A generation 3 device speaks the same RPC.
func TestApplyDesiredConfig_Gen3UsesTheGen2Path(t *testing.T) {
	svc, _ := newGen2Service(t, systemDesired())
	dev := newFakeGen2Device()
	dev.generation = 3

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied)
	assert.Len(t, dev.calls, 1)
}

func TestVerifyDesiredConfig_Gen2ReadsWithoutWriting(t *testing.T) {
	svc, _ := newGen2Service(t, systemDesired())
	dev := newFakeGen2Device()

	result, err := svc.VerifyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.False(t, result.Match, "the device holds other values")
	assert.NotEmpty(t, result.Differences)
	assert.Empty(t, dev.calls)
	assert.Equal(t, 0, dev.legacyCalls)
}

// A Gen2 client that cannot write component settings is refused as before, with nothing sent.
func TestApplyDesiredConfig_Gen2WithoutTheSetterIsStillRefused(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "tv-office")
	dev := newFakeGen1Device()
	dev.generation = 2

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, ErrUnsupportedGeneration))
	assert.Equal(t, 0, dev.setCalls)
	assert.False(t, repo.devices[1].ConfigApplied)
}
