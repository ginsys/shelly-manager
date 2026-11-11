package configuration

// DeviceTypeInfo is one model a device-type template can name.
type DeviceTypeInfo struct {
	DeviceType string `json:"device_type"`
	Name       string `json:"name"`
	Generation int    `json:"generation"`
}

// deviceTypeNames gives each supported code a readable name. A test pins the keys to
// SupportedDeviceTypes, so a model cannot be added to the converter without a name here.
var deviceTypeNames = map[string]string{
	"SHPLG-S": "Shelly Plug S",
	"SHPLG-1": "Shelly Plug",
	"SHSW-1":  "Shelly 1",
	"SHSW-PM": "Shelly 1PM",
	"SHSW-25": "Shelly 2.5",
	"SHIX3-1": "Shelly i3",
}

// SupportedDeviceTypeInfos lists the models a device-type template can name, in the converter's order.
func SupportedDeviceTypeInfos() []DeviceTypeInfo {
	converter := NewGen1Converter(nil)
	var infos []DeviceTypeInfo
	for _, code := range converter.SupportedDeviceTypes() {
		infos = append(infos, DeviceTypeInfo{
			DeviceType: code,
			Name:       deviceTypeNames[code],
			Generation: converter.Generation(),
		})
	}
	return infos
}

// IsSupportedDeviceType reports whether code is exactly one of the supported device types.
func IsSupportedDeviceType(code string) bool {
	for _, supported := range NewGen1Converter(nil).SupportedDeviceTypes() {
		if supported == code {
			return true
		}
	}
	return false
}
