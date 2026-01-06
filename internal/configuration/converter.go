package configuration

import "encoding/json"

// ConfigConverter defines the interface for API ↔ internal conversion
type ConfigConverter interface {
	// FromAPIConfig converts raw API JSON to internal DeviceConfiguration struct
	FromAPIConfig(apiJSON json.RawMessage, deviceType string) (*DeviceConfiguration, error)

	// ToAPIConfig converts internal DeviceConfiguration struct to API JSON
	ToAPIConfig(config *DeviceConfiguration, deviceType string) (json.RawMessage, error)

	// SupportedDeviceTypes returns list of supported device types
	SupportedDeviceTypes() []string

	// Generation returns the Shelly generation (1 or 2)
	Generation() int
}
