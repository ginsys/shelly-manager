package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A stored desired configuration may hold a section with nothing set in it, such as
// {"system":{"sntp":{}}}. A section that holds nothing is nothing: Apply, verify and the drift check
// read the row as if it were not there, and the stored row itself is never rewritten.

// emptySectionPaths walks t to its full depth and returns the JSON path of every pointer to a struct
// field, so a section added to the typed model later is covered without an edit here.
func emptySectionPaths(t reflect.Type, prefix []string) [][]string {
	var out [][]string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name := strings.Split(f.Tag.Get("json"), ",")[0]
		if !f.IsExported() || name == "" || name == "-" {
			continue
		}
		path := append(append([]string{}, prefix...), name)
		ft := f.Type
		switch {
		case ft.Kind() == reflect.Pointer && ft.Elem().Kind() == reflect.Struct && !isOpaqueStruct(ft.Elem()):
			out = append(out, path)
			out = append(out, emptySectionPaths(ft.Elem(), path)...)
		case ft.Kind() == reflect.Struct && !isOpaqueStruct(ft):
			out = append(out, emptySectionPaths(ft, path)...)
		}
	}
	return out
}

// emptyRowAt is the stored row that holds one empty object at path.
func emptyRowAt(path []string) string {
	return `{"` + strings.Join(path, `":{"`) + `":{` + strings.Repeat("}", len(path)) + `}`
}

func setStoredDesired(repo *mockRepository, raw string) {
	repo.devices[1].DesiredConfig = raw
}

func TestEmptySectionPaths_ReachTheFullDepthOfTheModel(t *testing.T) {
	longest := 0
	for _, p := range emptySectionPaths(reflect.TypeOf(DeviceConfiguration{}), nil) {
		if len(p) > longest {
			longest = len(p)
		}
	}
	assert.GreaterOrEqual(t, longest, 4, "the walk must reach network.wifi.sta.ip")
}

func TestApplyDesiredConfig_AnEmptySectionAtAnyDepthIsNothing(t *testing.T) {
	paths := emptySectionPaths(reflect.TypeOf(DeviceConfiguration{}), nil)
	require.NotEmpty(t, paths)
	for _, p := range paths {
		row := emptyRowAt(p)
		t.Run("gen1 "+strings.Join(p, "."), func(t *testing.T) {
			svc, repo := newServiceWithDesiredName(t, "x")
			setStoredDesired(repo, row)
			dev := newFakeGen1Device()

			_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

			assert.False(t, errors.Is(err, ErrUnsupportedSettings), "%s: %v", row, err)
			for _, posted := range dev.posted {
				assert.Empty(t, posted, "nothing is sent for an empty section")
			}
		})
		t.Run("gen2 "+strings.Join(p, "."), func(t *testing.T) {
			svc, repo := newGen2Service(t, &DeviceConfiguration{})
			setStoredDesired(repo, row)
			dev := newFakeGen2Device()

			_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

			assert.False(t, errors.Is(err, ErrUnsupportedSettings), "%s: %v", row, err)
			for _, c := range dev.calls {
				assert.Empty(t, c.config, "nothing is sent for an empty section")
			}
		})
	}
}

// A plain zero is "unset", as in the merge: these sections hold nothing too.
func TestApplyDesiredConfig_APlainZeroSectionIsNothing(t *testing.T) {
	for name, row := range map[string]string{
		"sleep_mode false": `{"system":{"sleep_mode":{"enable":false}}}`,
		"ble false":        `{"system":{"device":{"ble":{"enable":false}}}}`,
		"debug level 0":    `{"system":{"debug":{"level":0}}}`,
		"sntp empty":       `{"system":{"sntp":{"server":""}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			svc, repo := newServiceWithDesiredName(t, "x")
			setStoredDesired(repo, row)

			_, err := svc.ApplyDesiredConfig(context.Background(), 1, newFakeGen1Device(), "SHPLG-S")

			assert.False(t, errors.Is(err, ErrUnsupportedSettings), "%s: %v", row, err)
		})
	}
}

const emptyBesideMQTT = `{"system":{"sntp":{}},"mqtt":{"server":"broker.local"}}`

func TestApplyDesiredConfig_AnEmptySectionBesideARealValueApplies(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "x")
	setStoredDesired(repo, emptyBesideMQTT)
	dev := newFakeGen1Device()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied)
	assert.True(t, repo.devices[1].ConfigApplied)
	require.NotNil(t, result.VerifyResult)
	assert.True(t, result.VerifyResult.Match)
	mqtt, _ := dev.settings["mqtt"].(map[string]interface{})
	assert.Equal(t, "broker.local", mqtt["server"])
	assert.NotContains(t, dev.settings, "sntp")
}

func TestApplyDesiredConfig_Gen2AnEmptySectionBesideARealValueApplies(t *testing.T) {
	svc, repo := newGen2Service(t, &DeviceConfiguration{})
	setStoredDesired(repo, `{"system":{"sntp":{},"device":{"name":"lamp"}}}`)
	dev := newFakeGen2Device()

	result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

	require.NoError(t, err)
	assert.True(t, result.ConfigApplied)
	require.Len(t, dev.calls, 1)
	assert.Equal(t, map[string]interface{}{"device": map[string]interface{}{"name": "lamp"}}, dev.calls[0].config)
	assert.Equal(t, "time.google.com", dev.doc["sys"].(map[string]interface{})["sntp"].(map[string]interface{})["server"])
}

// A real value is still refused, with the same path, and an empty section beside it does not change that.
func TestApplyDesiredConfig_ARealValueBesideAnEmptySectionIsStillRefused(t *testing.T) {
	t.Run("gen1 cloud server", func(t *testing.T) {
		svc, repo := newServiceWithDesiredName(t, "x")
		setStoredDesired(repo, `{"system":{"sntp":{}},"cloud":{"server":"x"}}`)
		dev := newFakeGen1Device()

		_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrUnsupportedSettings))
		assert.Contains(t, err.Error(), "cloud.server")
		assert.NotContains(t, err.Error(), "system", "the empty section is not named")
		assert.Equal(t, 0, dev.setCalls)
	})
	t.Run("gen2 time server", func(t *testing.T) {
		svc, repo := newGen2Service(t, &DeviceConfiguration{})
		setStoredDesired(repo, `{"system":{"device":{},"sntp":{"server":"not a host!"}}}`)
		dev := newFakeGen2Device()

		_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrUnsupportedSettings))
		assert.Contains(t, err.Error(), "system.sntp.server")
		assert.NotContains(t, err.Error(), "system,", "the empty device section is not named")
		assert.Empty(t, dev.calls)
	})
}

// A set zero is not empty: a pointer to false or 0 is a value somebody chose.
func TestApplyDesiredConfig_ASetZeroKeepsItsAnswer(t *testing.T) {
	t.Run("gen1 mqtt enable false is still written", func(t *testing.T) {
		svc, repo := newServiceWithDesiredName(t, "x")
		setStoredDesired(repo, `{"mqtt":{"enable":false}}`)
		dev := newFakeGen1Device()

		_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

		require.NoError(t, err)
		require.NotEmpty(t, dev.posted, "a set false is a value and is sent")
		assert.Equal(t, false, dev.posted[0]["mqtt_enable"])
	})
	t.Run("gen1 relay schedule false is not dropped", func(t *testing.T) {
		cfg, err := parseComparableDesiredConfig(`{"relay":{"relays":[{"schedule":false,"auto_on":0}]}}`)
		require.NoError(t, err)
		require.NotNil(t, cfg.Relay)
		relays := cfg.Relay.Relays
		require.Len(t, relays, 1)
		require.NotNil(t, relays[0].Schedule)
		require.NotNil(t, relays[0].AutoOn)
	})
	t.Run("gen2 roller zero speed", func(t *testing.T) {
		svc, repo := newGen2Service(t, &DeviceConfiguration{})
		setStoredDesired(repo, `{"roller":{"motor_speed":0}}`)

		_, err := svc.ApplyDesiredConfig(context.Background(), 1, newFakeGen2Device(), "SNSW-001X16EU")

		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrUnsupportedSettings), "a set value is not dropped: %v", err)
	})
	t.Run("gen2 actions empty list keeps today's answer", func(t *testing.T) {
		svc, repo := newGen2Service(t, &DeviceConfiguration{})
		setStoredDesired(repo, `{"actions":[]}`)

		_, err := svc.ApplyDesiredConfig(context.Background(), 1, newFakeGen2Device(), "SNSW-001X16EU")

		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrUnsupportedSettings))
		assert.Contains(t, err.Error(), "actions")
	})
	t.Run("gen1 actions empty list keeps today's answer", func(t *testing.T) {
		svc, repo := newServiceWithDesiredName(t, "x")
		setStoredDesired(repo, `{"actions":[]}`)

		_, err := svc.ApplyDesiredConfig(context.Background(), 1, newFakeGen1Device(), "SHPLG-S")

		assert.False(t, errors.Is(err, ErrUnsupportedSettings), "%v", err)
	})
}

// The stored row is read, never rewritten, and the download of it still shows what was stored.
func TestApplyDesiredConfig_NeverRewritesTheStoredRow(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "x")
	setStoredDesired(repo, emptyBesideMQTT)

	_, err := svc.ApplyDesiredConfig(context.Background(), 1, newFakeGen1Device(), "SHPLG-S")
	require.NoError(t, err)

	assert.Equal(t, emptyBesideMQTT, repo.devices[1].DesiredConfig)
	stored, err := StoredDesiredConfig(repo.devices[1].DesiredConfig)
	require.NoError(t, err)
	require.NotNil(t, stored.System)
	assert.NotNil(t, stored.System.SNTP, "the stored row still holds its empty section")
}

// Verify and the drift check read the row the same way.
func TestVerifyDesiredConfig_AnEmptySectionIsNotADifference(t *testing.T) {
	t.Run("gen1", func(t *testing.T) {
		svc, repo := newServiceWithDesiredName(t, "x")
		setStoredDesired(repo, emptyBesideMQTT)
		dev := newFakeGen1Device()
		_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")
		require.NoError(t, err)

		result, err := svc.VerifyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

		require.NoError(t, err)
		assert.True(t, result.Match, "%+v", result.Differences)
		assert.Empty(t, result.Differences)
	})
	t.Run("gen2", func(t *testing.T) {
		svc, repo := newGen2Service(t, &DeviceConfiguration{})
		setStoredDesired(repo, `{"system":{"sntp":{},"device":{"name":"lamp"}}}`)
		dev := newFakeGen2Device()
		_, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")
		require.NoError(t, err)

		result, err := svc.VerifyDesiredConfig(context.Background(), 1, dev, "SNSW-001X16EU")

		require.NoError(t, err)
		assert.True(t, result.Match, "%+v", result.Differences)
		assert.Empty(t, result.Differences)
	})
}

func TestVerifyDesiredConfig_ARealDifferenceBesideAnEmptySectionIsStillReported(t *testing.T) {
	svc, repo := newServiceWithDesiredName(t, "x")
	setStoredDesired(repo, `{"system":{"sntp":{},"device":{"name":"tv-office"}}}`)
	dev := newFakeGen1Device()
	dev.settings["name"] = "other"

	result, err := svc.VerifyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

	require.NoError(t, err)
	assert.False(t, result.Match)
	require.Len(t, result.Differences, 1)
	assert.Contains(t, result.Differences[0].Path, "name")
}

func TestDetectDrift_AnEmptySectionIsNotDrift(t *testing.T) {
	service, _, _ := newDriftService(t, `{"system":{"device":{"name":"tv-office"},"sntp":{}}}`)

	drift, err := service.DetectDrift(1, driftClient("tv-office", driftTestMAC))

	require.NoError(t, err)
	assert.Nil(t, drift, "the empty section is not a difference")
}

func TestDetectDrift_ARealDifferenceBesideAnEmptySectionIsStillDrift(t *testing.T) {
	service, _, _ := newDriftService(t, `{"system":{"device":{"name":"tv-office"},"sntp":{}}}`)

	drift, err := service.DetectDrift(1, driftClient("other", driftTestMAC))

	require.NoError(t, err)
	require.NotNil(t, drift)
	require.Len(t, drift.Differences, 1)
	assert.Contains(t, drift.Differences[0].Path, "name")
}

// The comparator is untouched: it still reports an empty section the way it always did.
func TestComparator_StillReportsAnEmptySection(t *testing.T) {
	want := &DeviceConfiguration{System: &SystemConfiguration{SNTP: &SNTPConfig{}}}
	result := NewConfigComparator().Compare(want, &DeviceConfiguration{})
	assert.False(t, result.Match)
	require.NotEmpty(t, result.Differences)
}

// A time value is one opaque value, even the zero time: a set energy_reset is not an empty section.
func TestParseComparableDesiredConfig_KeepsAnOpaqueValue(t *testing.T) {
	cfg, err := parseComparableDesiredConfig(`{"power_metering":{"energy_reset":"0001-01-01T00:00:00Z"}}`)
	require.NoError(t, err)

	require.NotNil(t, cfg.PowerMetering)
	require.NotNil(t, cfg.PowerMetering.EnergyReset)
}

// dropEmptySections works on the parsed copy: a value, a set zero and a list survive.
func TestParseComparableDesiredConfig_KeepsWhatIsSet(t *testing.T) {
	cfg, err := parseComparableDesiredConfig(`{"system":{"sntp":{},"device":{"name":"a"}},"mqtt":{"enable":false},"actions":[],"led":{}}`)
	require.NoError(t, err)

	assert.Nil(t, cfg.LED)
	assert.Nil(t, cfg.System.SNTP)
	require.NotNil(t, cfg.System.Device)
	require.NotNil(t, cfg.MQTT, "a set false is a value")
	require.NotNil(t, cfg.MQTT.Enable)
	require.NotNil(t, cfg.Actions, "a set empty list is a value")
	raw, _ := json.Marshal(cfg)
	assert.NotContains(t, string(raw), "sntp")
}
