package configuration

import (
	"encoding/json"
	"fmt"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// Gen2Converter converts between the stored configuration and the document that a Gen2 or later
// device returns from Shelly.GetConfig, which is keyed by component (sys, wifi, mqtt, switch:0 and
// so on). It reads and writes the sys section only so far: a setting of any other section has no
// mapping, so the round-trip check in unsupportedGen2Settings refuses it before the device is
// touched. Each later section is added together with the code that writes it.
type Gen2Converter struct {
	logger *logging.Logger
}

// NewGen2Converter creates a Gen2 converter.
func NewGen2Converter(logger *logging.Logger) *Gen2Converter {
	if logger == nil {
		logger = logging.GetDefault()
	}
	return &Gen2Converter{logger: logger}
}

// Generation returns 2: Gen3 and later devices speak the same RPC.
func (c *Gen2Converter) Generation() int { return 2 }

// SupportedDeviceTypes returns no list: the Gen2 path does not match on a model name.
func (c *Gen2Converter) SupportedDeviceTypes() []string { return nil }

// FromAPIConfig reads the sys section of a Shelly.GetConfig document. A null value, such as a
// device with no name, is an unset setting.
func (c *Gen2Converter) FromAPIConfig(apiJSON json.RawMessage, _ string) (*DeviceConfiguration, error) {
	var doc map[string]interface{}
	if err := json.Unmarshal(apiJSON, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse Gen2 config: %w", err)
	}
	config := &DeviceConfiguration{}
	sys, _ := doc["sys"].(map[string]interface{})

	device := &TypedDeviceConfig{}
	if raw, ok := sys["device"].(map[string]interface{}); ok {
		if name, ok := raw["name"].(string); ok {
			device.Name = StringPtr(name)
		}
		if eco, ok := raw["eco_mode"].(bool); ok {
			device.EcoMode = BoolPtr(eco)
		}
		if discoverable, ok := raw["discoverable"].(bool); ok {
			device.Discoverable = BoolPtr(discoverable)
		}
	}
	if device.Name != nil || device.EcoMode != nil || device.Discoverable != nil {
		config.System = &SystemConfiguration{Device: device}
	}

	// An empty time server means none, and cannot be told from an unset one in SNTPConfig.
	if raw, ok := sys["sntp"].(map[string]interface{}); ok {
		if server, ok := raw["server"].(string); ok && server != "" {
			if config.System == nil {
				config.System = &SystemConfiguration{}
			}
			config.System.SNTP = &SNTPConfig{Server: server}
		}
	}

	location := &LocationConfiguration{}
	if raw, ok := sys["location"].(map[string]interface{}); ok {
		if tz, ok := raw["tz"].(string); ok {
			location.Timezone = StringPtr(tz)
		}
		if lat, ok := raw["lat"].(float64); ok {
			location.Latitude = Float64Ptr(lat)
		}
		if lon, ok := raw["lon"].(float64); ok {
			location.Longitude = Float64Ptr(lon)
		}
	}
	if location.Timezone != nil || location.Latitude != nil || location.Longitude != nil {
		config.Location = location
	}
	return config, nil
}

// ToAPIConfig returns the sys section as the config object of Sys.SetConfig, with only the keys
// the configuration sets: the documentation does not say that SetConfig merges, so a key left out
// is never sent.
func (c *Gen2Converter) ToAPIConfig(config *DeviceConfiguration, _ string) (json.RawMessage, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	sys := map[string]interface{}{}

	device := map[string]interface{}{}
	if d := configDevice(config); d != nil {
		if d.Name != nil {
			device["name"] = *d.Name
		}
		if d.EcoMode != nil {
			device["eco_mode"] = *d.EcoMode
		}
		if d.Discoverable != nil {
			device["discoverable"] = *d.Discoverable
		}
	}
	if len(device) > 0 {
		sys["device"] = device
	}

	location := map[string]interface{}{}
	if l := config.Location; l != nil {
		if l.Timezone != nil {
			location["tz"] = *l.Timezone
		}
		if l.Latitude != nil {
			location["lat"] = *l.Latitude
		}
		if l.Longitude != nil {
			location["lon"] = *l.Longitude
		}
	}
	if len(location) > 0 {
		sys["location"] = location
	}

	if config.System != nil && config.System.SNTP != nil && config.System.SNTP.Server != "" {
		sys["sntp"] = map[string]interface{}{"server": config.System.SNTP.Server}
	}

	doc := map[string]interface{}{}
	if len(sys) > 0 {
		doc["sys"] = sys
	}
	return json.Marshal(doc)
}

func configDevice(config *DeviceConfiguration) *TypedDeviceConfig {
	if config.System == nil {
		return nil
	}
	return config.System.Device
}
