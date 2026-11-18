package configuration

import (
	"testing"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// Each Gen1 MQTT and CoIoT number keeps a whole value and reads a fraction, or a number no int holds,
// as no value, so a stored 7 never stands for a device's 7.5.
func TestGen1Converter_FromAPIConfig_MQTTAndCoIoTNumbersReadAsWholeNumbersOnly(t *testing.T) {
	converter := NewGen1Converter(nil)

	sites := []struct {
		name    string
		section string
		key     string
		pick    func(*DeviceConfiguration) *int
	}{
		{"mqtt keep_alive", "mqtt", "keep_alive", func(c *DeviceConfiguration) *int {
			if c.MQTT == nil {
				return nil
			}
			return c.MQTT.KeepAlive
		}},
		{"mqtt update_period", "mqtt", "update_period", func(c *DeviceConfiguration) *int {
			if c.MQTT == nil {
				return nil
			}
			return c.MQTT.UpdatePeriod
		}},
		{"coiot update_period", "coiot", "update_period", func(c *DeviceConfiguration) *int {
			if c.CoIoT == nil {
				return nil
			}
			return c.CoIoT.UpdatePeriod
		}},
	}

	for _, site := range sites {
		t.Run(site.name, func(t *testing.T) {
			read := func(entry map[string]interface{}) interface{} {
				config, err := converter.FromAPIConfig(mustJSON(t, map[string]interface{}{site.section: entry}), "SHSW-1")
				if err != nil {
					t.Fatalf("FromAPIConfig: %v", err)
				}
				return intAt(site.pick(config))
			}

			if got := read(map[string]interface{}{site.key: float64(7)}); got != 7 {
				t.Errorf("whole: got %v, want 7", got)
			}
			if got := read(map[string]interface{}{"enable": true}); got != nil {
				t.Errorf("absent: got %v, want unset", got)
			}
			for _, tc := range notWholeNumbers {
				if got := read(map[string]interface{}{site.key: tc.value}); got != nil {
					t.Errorf("%s: got %v, want unset", tc.name, got)
				}
			}
		})
	}
}

// A raw MQTT section converts with a whole port kept and a fraction, or a number no int holds, unset.
func TestConvertRawToTyped_MQTTPortReadsAsWholeNumberOnly(t *testing.T) {
	service := createTestService(t)
	read := func(entry map[string]interface{}) interface{} {
		// The manual conversion runs only where the document does not parse as a typed one, so a wifi
		// section with a flag that is no bool sits beside the MQTT one to send every shape through it.
		typed, _, err := service.ConvertRawToTyped(mustJSON(t, map[string]interface{}{
			"mqtt": entry,
			"wifi": map[string]interface{}{"enable": "not a bool"},
		}))
		if err != nil {
			t.Fatalf("ConvertRawToTyped: %v", err)
		}
		if typed.MQTT == nil {
			t.Fatal("mqtt section is nil")
		}
		return intAt(typed.MQTT.Port)
	}

	if got := read(map[string]interface{}{"port": float64(1883)}); got != 1883 {
		t.Errorf("whole: got %v, want 1883", got)
	}
	if got := read(map[string]interface{}{"server": "broker"}); got != nil {
		t.Errorf("absent: got %v, want unset", got)
	}
	for _, tc := range notWholeNumbers {
		if got := read(map[string]interface{}{"port": tc.value}); got != nil {
			t.Errorf("%s: got %v, want unset", tc.name, got)
		}
	}
}

// A device's "gen" setting fills the template context only when it is a whole number.
func TestTemplateEngine_CreateTemplateContext_GenerationReadsAWholeNumberOnly(t *testing.T) {
	logger, err := logging.New(logging.Config{Level: "info", Format: "text", Output: "stdout"})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	engine := NewTemplateEngine(logger)
	read := func(settings string) int {
		device := &Device{ID: 1, MAC: "AA:BB:CC:DD:EE:FF", Type: "SHSW-25", Settings: settings}
		return engine.CreateTemplateContext(device, nil).Device.Generation
	}

	if got := read(`{"model":"SHSW-25","gen":2}`); got != 2 {
		t.Errorf("whole: got %d, want 2", got)
	}
	if got := read(`{"model":"SHSW-25"}`); got != 0 {
		t.Errorf("absent: got %d, want 0", got)
	}
	for _, settings := range []string{
		`{"gen":1.5}`, `{"gen":0.5}`, `{"gen":1e300}`, `{"gen":-1e300}`, `{"gen":9007199254740992}`,
	} {
		if got := read(settings); got != 0 {
			t.Errorf("%s: got %d, want 0 (no value)", settings, got)
		}
	}
}
