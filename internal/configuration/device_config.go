package configuration

type DeviceConfiguration struct {
	WiFi     *WiFiConfiguration     `json:"wifi,omitempty"`
	MQTT     *MQTTConfiguration     `json:"mqtt,omitempty"`
	Auth     *AuthConfiguration     `json:"auth,omitempty"`
	System   *SystemConfiguration   `json:"system,omitempty"`
	Network  *NetworkConfiguration  `json:"network,omitempty"`
	Cloud    *CloudConfiguration    `json:"cloud,omitempty"`
	Location *LocationConfiguration `json:"location,omitempty"`
	CoIoT    *CoIoTConfiguration    `json:"coiot,omitempty"`

	Relay         *RelayConfig         `json:"relay,omitempty"`
	PowerMetering *PowerMeteringConfig `json:"power_metering,omitempty"`
	Dimming       *DimmingConfig       `json:"dimming,omitempty"`
	Roller        *RollerConfig        `json:"roller,omitempty"`
	Input         *InputConfig         `json:"input,omitempty"`
	LED           *LEDConfig           `json:"led,omitempty"`
}

type CoIoTConfiguration struct {
	Enable       *bool   `json:"enable,omitempty"`
	UpdatePeriod *int    `json:"update_period,omitempty"`
	Peer         *string `json:"peer,omitempty"`
}
