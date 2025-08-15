package gen2

import (
	"context"
)

// Power Monitoring Methods

// ResetEnergyCounters resets energy counters for a switch
func (c *Client) ResetEnergyCounters(ctx context.Context, channel int) error {
	params := map[string]interface{}{
		"id":   channel,
		"type": []string{"aenergy"},
	}
	return c.rpcCall(ctx, "Switch.ResetCounters", params, nil)
}

// GetPowerStatus retrieves detailed power metrics for a channel
func (c *Client) GetPowerStatus(ctx context.Context, channel int) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"id": channel,
	}
	var result map[string]interface{}
	if err := c.rpcCall(ctx, "PM.GetStatus", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetPowerData retrieves historical power data
func (c *Client) GetPowerData(ctx context.Context, channel int) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"id": channel,
	}
	var result map[string]interface{}
	if err := c.rpcCall(ctx, "PM.GetData", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Cover/Roller Methods

// GetCoverStatus retrieves the status of a cover/roller
func (c *Client) GetCoverStatus(ctx context.Context, channel int) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"id": channel,
	}
	var result map[string]interface{}
	if err := c.rpcCall(ctx, "Cover.GetStatus", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetCoverConfig retrieves cover configuration
func (c *Client) GetCoverConfig(ctx context.Context, channel int) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"id": channel,
	}
	var result map[string]interface{}
	if err := c.rpcCall(ctx, "Cover.GetConfig", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// SetCoverConfig updates cover configuration
func (c *Client) SetCoverConfig(ctx context.Context, channel int, config map[string]interface{}) error {
	params := map[string]interface{}{
		"id":     channel,
		"config": config,
	}
	return c.rpcCall(ctx, "Cover.SetConfig", params, nil)
}

// CalibrateCover starts calibration for a cover
func (c *Client) CalibrateCover(ctx context.Context, channel int) error {
	params := map[string]interface{}{
		"id": channel,
	}
	return c.rpcCall(ctx, "Cover.Calibrate", params, nil)
}

// Light Methods

// GetLightStatus retrieves the status of a light
func (c *Client) GetLightStatus(ctx context.Context, channel int) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"id": channel,
	}
	var result map[string]interface{}
	if err := c.rpcCall(ctx, "Light.GetStatus", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetLightConfig retrieves light configuration
func (c *Client) GetLightConfig(ctx context.Context, channel int) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"id": channel,
	}
	var result map[string]interface{}
	if err := c.rpcCall(ctx, "Light.GetConfig", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// SetLight controls light parameters
func (c *Client) SetLight(ctx context.Context, channel int, params map[string]interface{}) error {
	params["id"] = channel
	return c.rpcCall(ctx, "Light.Set", params, nil)
}

// RGBW Methods

// GetRGBWStatus retrieves RGBW status
func (c *Client) GetRGBWStatus(ctx context.Context, channel int) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"id": channel,
	}
	var result map[string]interface{}
	if err := c.rpcCall(ctx, "RGBW.GetStatus", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// SetRGBW controls RGBW parameters
func (c *Client) SetRGBW(ctx context.Context, channel int, params map[string]interface{}) error {
	params["id"] = channel
	return c.rpcCall(ctx, "RGBW.Set", params, nil)
}

// SetRGBWConfig updates RGBW configuration
func (c *Client) SetRGBWConfig(ctx context.Context, channel int, config map[string]interface{}) error {
	params := map[string]interface{}{
		"id":     channel,
		"config": config,
	}
	return c.rpcCall(ctx, "RGBW.SetConfig", params, nil)
}

// GetRGBStatus retrieves RGB mode status
func (c *Client) GetRGBStatus(ctx context.Context, channel int) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"id": channel,
	}
	var result map[string]interface{}
	if err := c.rpcCall(ctx, "RGB.GetStatus", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// SetRGB controls RGB values
func (c *Client) SetRGB(ctx context.Context, channel int, r, g, b uint8, brightness int) error {
	params := map[string]interface{}{
		"id": channel,
		"on": true,
		"rgb": map[string]interface{}{
			"r": r,
			"g": g,
			"b": b,
		},
		"brightness": brightness,
	}
	return c.rpcCall(ctx, "RGB.Set", params, nil)
}

// Input Methods

// GetInputStatus retrieves input status
func (c *Client) GetInputStatus(ctx context.Context, input int) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"id": input,
	}
	var result map[string]interface{}
	if err := c.rpcCall(ctx, "Input.GetStatus", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetInputConfig retrieves input configuration
func (c *Client) GetInputConfig(ctx context.Context, input int) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"id": input,
	}
	var result map[string]interface{}
	if err := c.rpcCall(ctx, "Input.GetConfig", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// SetInputConfig updates input configuration
func (c *Client) SetInputConfig(ctx context.Context, input int, config map[string]interface{}) error {
	params := map[string]interface{}{
		"id":     input,
		"config": config,
	}
	return c.rpcCall(ctx, "Input.SetConfig", params, nil)
}

// System Methods

// GetSysConfig retrieves system configuration
func (c *Client) GetSysConfig(ctx context.Context) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.rpcCall(ctx, "Sys.GetConfig", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// SetSysConfig updates system configuration
func (c *Client) SetSysConfig(ctx context.Context, config map[string]interface{}) error {
	params := map[string]interface{}{
		"config": config,
	}
	return c.rpcCall(ctx, "Sys.SetConfig", params, nil)
}

// GetSysStatus retrieves system status
func (c *Client) GetSysStatus(ctx context.Context) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.rpcCall(ctx, "Sys.GetStatus", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Ethernet Methods (Pro devices)

// GetEthStatus retrieves ethernet status
func (c *Client) GetEthStatus(ctx context.Context) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.rpcCall(ctx, "Eth.GetStatus", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetEthConfig retrieves ethernet configuration
func (c *Client) GetEthConfig(ctx context.Context) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.rpcCall(ctx, "Eth.GetConfig", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// SetEthConfig updates ethernet configuration
func (c *Client) SetEthConfig(ctx context.Context, config map[string]interface{}) error {
	params := map[string]interface{}{
		"config": config,
	}
	return c.rpcCall(ctx, "Eth.SetConfig", params, nil)
}

// LED Methods (devices with LED indicators)

// GetLEDConfig retrieves LED configuration
func (c *Client) GetLEDConfig(ctx context.Context) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.rpcCall(ctx, "LED.GetConfig", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// SetLEDConfig updates LED configuration
func (c *Client) SetLEDConfig(ctx context.Context, config map[string]interface{}) error {
	params := map[string]interface{}{
		"config": config,
	}
	return c.rpcCall(ctx, "LED.SetConfig", params, nil)
}

// Temperature & Humidity Methods (Plus H&T)

// GetTemperatureStatus retrieves temperature sensor status
func (c *Client) GetTemperatureStatus(ctx context.Context, sensor int) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"id": sensor,
	}
	var result map[string]interface{}
	if err := c.rpcCall(ctx, "Temperature.GetStatus", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetHumidityStatus retrieves humidity sensor status
func (c *Client) GetHumidityStatus(ctx context.Context, sensor int) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"id": sensor,
	}
	var result map[string]interface{}
	if err := c.rpcCall(ctx, "Humidity.GetStatus", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetDevicePowerStatus retrieves device power/battery status
func (c *Client) GetDevicePowerStatus(ctx context.Context) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.rpcCall(ctx, "DevicePower.GetStatus", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Script Methods (Pro 4PM and other devices with scripting)

// ListScripts lists available scripts
func (c *Client) ListScripts(ctx context.Context) ([]interface{}, error) {
	var result struct {
		Scripts []interface{} `json:"scripts"`
	}
	if err := c.rpcCall(ctx, "Script.List", nil, &result); err != nil {
		return nil, err
	}
	return result.Scripts, nil
}

// StartScript starts a script
func (c *Client) StartScript(ctx context.Context, scriptID int) error {
	params := map[string]interface{}{
		"id": scriptID,
	}
	return c.rpcCall(ctx, "Script.Start", params, nil)
}

// StopScript stops a script
func (c *Client) StopScript(ctx context.Context, scriptID int) error {
	params := map[string]interface{}{
		"id": scriptID,
	}
	return c.rpcCall(ctx, "Script.Stop", params, nil)
}

// GetScriptStatus retrieves script status
func (c *Client) GetScriptStatus(ctx context.Context, scriptID int) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"id": scriptID,
	}
	var result map[string]interface{}
	if err := c.rpcCall(ctx, "Script.GetStatus", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// CreateScript creates a new script
func (c *Client) CreateScript(ctx context.Context, name string, code string) (int, error) {
	params := map[string]interface{}{
		"name": name,
		"code": code,
	}
	var result struct {
		ID int `json:"id"`
	}
	if err := c.rpcCall(ctx, "Script.Create", params, &result); err != nil {
		return 0, err
	}
	return result.ID, nil
}

// DeleteScript deletes a script
func (c *Client) DeleteScript(ctx context.Context, scriptID int) error {
	params := map[string]interface{}{
		"id": scriptID,
	}
	return c.rpcCall(ctx, "Script.Delete", params, nil)
}

// Webhook Methods

// CreateWebhook creates a webhook for events
func (c *Client) CreateWebhook(ctx context.Context, event string, urls []string, enabled bool) error {
	params := map[string]interface{}{
		"event":   event,
		"urls":    urls,
		"enabled": enabled,
	}
	return c.rpcCall(ctx, "Webhook.Create", params, nil)
}

// ListWebhooks lists configured webhooks
func (c *Client) ListWebhooks(ctx context.Context) ([]interface{}, error) {
	var result struct {
		Hooks []interface{} `json:"hooks"`
	}
	if err := c.rpcCall(ctx, "Webhook.List", nil, &result); err != nil {
		return nil, err
	}
	return result.Hooks, nil
}

// DeleteWebhook deletes a webhook
func (c *Client) DeleteWebhook(ctx context.Context, hookID int) error {
	params := map[string]interface{}{
		"id": hookID,
	}
	return c.rpcCall(ctx, "Webhook.Delete", params, nil)
}

// WiFi Methods

// GetWiFiConfig retrieves WiFi configuration
func (c *Client) GetWiFiConfig(ctx context.Context) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.rpcCall(ctx, "WiFi.GetConfig", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// SetWiFiConfig updates WiFi configuration
func (c *Client) SetWiFiConfig(ctx context.Context, config map[string]interface{}) error {
	params := map[string]interface{}{
		"config": config,
	}
	return c.rpcCall(ctx, "WiFi.SetConfig", params, nil)
}

// GetWiFiStatus retrieves WiFi status
func (c *Client) GetWiFiStatus(ctx context.Context) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.rpcCall(ctx, "WiFi.GetStatus", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Scan for WiFi networks
func (c *Client) ScanWiFi(ctx context.Context) ([]interface{}, error) {
	var result struct {
		Results []interface{} `json:"results"`
	}
	if err := c.rpcCall(ctx, "WiFi.Scan", nil, &result); err != nil {
		return nil, err
	}
	return result.Results, nil
}

// KVS (Key-Value Store) Methods

// KVSSet sets a value in the key-value store
func (c *Client) KVSSet(ctx context.Context, key string, value interface{}) error {
	params := map[string]interface{}{
		"key":   key,
		"value": value,
	}
	return c.rpcCall(ctx, "KVS.Set", params, nil)
}

// KVSGet retrieves a value from the key-value store
func (c *Client) KVSGet(ctx context.Context, key string) (interface{}, error) {
	params := map[string]interface{}{
		"key": key,
	}
	var result struct {
		Value interface{} `json:"value"`
	}
	if err := c.rpcCall(ctx, "KVS.Get", params, &result); err != nil {
		return nil, err
	}
	return result.Value, nil
}

// KVSDelete deletes a key from the key-value store
func (c *Client) KVSDelete(ctx context.Context, key string) error {
	params := map[string]interface{}{
		"key": key,
	}
	return c.rpcCall(ctx, "KVS.Delete", params, nil)
}

// KVSList lists all keys in the key-value store
func (c *Client) KVSList(ctx context.Context, match string) ([]string, error) {
	params := map[string]interface{}{}
	if match != "" {
		params["match"] = match
	}
	var result struct {
		Keys []string `json:"keys"`
	}
	if err := c.rpcCall(ctx, "KVS.List", params, &result); err != nil {
		return nil, err
	}
	return result.Keys, nil
}

// Component Discovery

// GetComponents retrieves available components
func (c *Client) GetComponents(ctx context.Context) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.rpcCall(ctx, "Shelly.GetComponents", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Energy Methods (Pro 3EM)

// GetEMStatus retrieves energy meter status
func (c *Client) GetEMStatus(ctx context.Context, phase int) (map[string]interface{}, error) {
	params := map[string]interface{}{}
	if phase >= 0 {
		params["id"] = phase
	}
	var result map[string]interface{}
	if err := c.rpcCall(ctx, "EM.GetStatus", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetEMData retrieves energy meter data
func (c *Client) GetEMData(ctx context.Context, phase int, dataType string) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"id":   phase,
		"type": dataType, // "1min", "1hour", "1day"
	}
	var result map[string]interface{}
	if err := c.rpcCall(ctx, "EM.GetData", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ResetEMData resets energy meter data
func (c *Client) ResetEMData(ctx context.Context, phase int) error {
	params := map[string]interface{}{
		"id": phase,
	}
	return c.rpcCall(ctx, "EM.ResetData", params, nil)
}

// BLE (Bluetooth Low Energy) Methods

// GetBLEConfig retrieves Bluetooth configuration
func (c *Client) GetBLEConfig(ctx context.Context) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.rpcCall(ctx, "BLE.GetConfig", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// SetBLEConfig updates Bluetooth configuration
func (c *Client) SetBLEConfig(ctx context.Context, config map[string]interface{}) error {
	params := map[string]interface{}{
		"config": config,
	}
	return c.rpcCall(ctx, "BLE.SetConfig", params, nil)
}

// Cloud Methods

// GetCloudConfig retrieves cloud configuration
func (c *Client) GetCloudConfig(ctx context.Context) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.rpcCall(ctx, "Cloud.GetConfig", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// SetCloudConfig updates cloud configuration
func (c *Client) SetCloudConfig(ctx context.Context, config map[string]interface{}) error {
	params := map[string]interface{}{
		"config": config,
	}
	return c.rpcCall(ctx, "Cloud.SetConfig", params, nil)
}

// GetCloudStatus retrieves cloud connection status
func (c *Client) GetCloudStatus(ctx context.Context) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.rpcCall(ctx, "Cloud.GetStatus", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Schedule Methods

// CreateSchedule creates a schedule
func (c *Client) CreateSchedule(ctx context.Context, schedule map[string]interface{}) (int, error) {
	var result struct {
		ID int `json:"id"`
	}
	if err := c.rpcCall(ctx, "Schedule.Create", schedule, &result); err != nil {
		return 0, err
	}
	return result.ID, nil
}

// UpdateSchedule updates a schedule
func (c *Client) UpdateSchedule(ctx context.Context, scheduleID int, schedule map[string]interface{}) error {
	schedule["id"] = scheduleID
	return c.rpcCall(ctx, "Schedule.Update", schedule, nil)
}

// DeleteSchedule deletes a schedule
func (c *Client) DeleteSchedule(ctx context.Context, scheduleID int) error {
	params := map[string]interface{}{
		"id": scheduleID,
	}
	return c.rpcCall(ctx, "Schedule.Delete", params, nil)
}

// ListSchedules lists all schedules
func (c *Client) ListSchedules(ctx context.Context) ([]interface{}, error) {
	var result struct {
		Schedules []interface{} `json:"schedules"`
	}
	if err := c.rpcCall(ctx, "Schedule.List", nil, &result); err != nil {
		return nil, err
	}
	return result.Schedules, nil
}

// MQTT Methods

// GetMQTTConfig retrieves MQTT configuration
func (c *Client) GetMQTTConfig(ctx context.Context) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.rpcCall(ctx, "MQTT.GetConfig", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// SetMQTTConfig updates MQTT configuration
func (c *Client) SetMQTTConfig(ctx context.Context, config map[string]interface{}) error {
	params := map[string]interface{}{
		"config": config,
	}
	return c.rpcCall(ctx, "MQTT.SetConfig", params, nil)
}

// GetMQTTStatus retrieves MQTT connection status
func (c *Client) GetMQTTStatus(ctx context.Context) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.rpcCall(ctx, "MQTT.GetStatus", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Outbound WebSocket Methods

// GetWSConfig retrieves outbound websocket configuration
func (c *Client) GetWSConfig(ctx context.Context) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.rpcCall(ctx, "WS.GetConfig", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// SetWSConfig updates outbound websocket configuration
func (c *Client) SetWSConfig(ctx context.Context, config map[string]interface{}) error {
	params := map[string]interface{}{
		"config": config,
	}
	return c.rpcCall(ctx, "WS.SetConfig", params, nil)
}

// GetWSStatus retrieves outbound websocket status
func (c *Client) GetWSStatus(ctx context.Context) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.rpcCall(ctx, "WS.GetStatus", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}
