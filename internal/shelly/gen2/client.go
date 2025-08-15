package gen2

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/shelly"
)

// Client implements the shelly.Client interface for Gen2+ devices
type Client struct {
	ip         string
	httpClient *http.Client
	config     *clientConfig
	logger     *logging.Logger
	generation int
}

// clientConfig holds configuration for the Gen2 client
type clientConfig struct {
	username      string
	password      string
	timeout       time.Duration
	retryAttempts int
	retryDelay    time.Duration
	skipTLSVerify bool
	userAgent     string
}

// ClientOption represents a configuration option for Gen2 client
type ClientOption func(*clientConfig)

// WithAuth sets authentication credentials
func WithAuth(username, password string) ClientOption {
	return func(c *clientConfig) {
		c.username = username
		c.password = password
	}
}

// WithTimeout sets the HTTP timeout
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *clientConfig) {
		c.timeout = timeout
	}
}

// WithRetry configures retry behavior
func WithRetry(attempts int, delay time.Duration) ClientOption {
	return func(c *clientConfig) {
		c.retryAttempts = attempts
		c.retryDelay = delay
	}
}

// WithSkipTLSVerify disables TLS certificate verification
func WithSkipTLSVerify(skip bool) ClientOption {
	return func(c *clientConfig) {
		c.skipTLSVerify = skip
	}
}

// WithUserAgent sets the user agent string
func WithUserAgent(userAgent string) ClientOption {
	return func(c *clientConfig) {
		c.userAgent = userAgent
	}
}

// NewClient creates a new Gen2+ Shelly client
func NewClient(ip string, opts ...ClientOption) *Client {
	cfg := &clientConfig{
		timeout:       10 * time.Second,
		retryAttempts: 3,
		retryDelay:    1 * time.Second,
		userAgent:     "shelly-manager/1.0",
	}

	for _, opt := range opts {
		opt(cfg)
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.skipTLSVerify,
		},
	}

	return &Client{
		ip: ip,
		httpClient: &http.Client{
			Timeout:   cfg.timeout,
			Transport: transport,
		},
		config:     cfg,
		logger:     logging.GetDefault(),
		generation: 2, // Default to Gen2, can be updated after GetInfo
	}
}

// RPCRequest represents a JSON-RPC request
type RPCRequest struct {
	ID     int         `json:"id"`
	Method string      `json:"method"`
	Params interface{} `json:"params,omitempty"`
}

// RPCResponse represents a JSON-RPC response
type RPCResponse struct {
	ID     int              `json:"id"`
	Result json.RawMessage  `json:"result,omitempty"`
	Error  *shelly.RPCError `json:"error,omitempty"`
}

// GetInfo retrieves device information
func (c *Client) GetInfo(ctx context.Context) (*shelly.DeviceInfo, error) {
	var result struct {
		ID         string `json:"id"`
		MAC        string `json:"mac"`
		Model      string `json:"model"`
		Generation int    `json:"gen"`
		FirmwareID string `json:"fw_id"`
		Version    string `json:"ver"`
		App        string `json:"app"`
		AuthEn     bool   `json:"auth_en"`
		AuthDomain string `json:"auth_domain"`
	}

	if err := c.rpcCall(ctx, "Shelly.GetDeviceInfo", nil, &result); err != nil {
		return nil, err
	}

	// Update our generation if it's Gen3
	if result.Generation > 2 {
		c.generation = result.Generation
	}

	return &shelly.DeviceInfo{
		ID:         result.ID,
		MAC:        result.MAC,
		Model:      result.Model,
		Generation: result.Generation,
		FirmwareID: result.FirmwareID,
		Version:    result.Version,
		App:        result.App,
		AuthEn:     result.AuthEn,
		AuthDomain: result.AuthDomain,
		IP:         c.ip,
		Discovered: time.Now(),
	}, nil
}

// GetStatus retrieves the current device status
func (c *Client) GetStatus(ctx context.Context) (*shelly.DeviceStatus, error) {
	var rawStatus map[string]interface{}
	if err := c.rpcCall(ctx, "Shelly.GetStatus", nil, &rawStatus); err != nil {
		return nil, err
	}

	status := &shelly.DeviceStatus{
		Raw: rawStatus,
	}

	// Parse system status
	if sys, ok := rawStatus["sys"].(map[string]interface{}); ok {
		if temp, ok := sys["temp"].(float64); ok {
			status.Temperature = temp
		}
		if overtemp, ok := sys["overtemp"].(bool); ok {
			status.Overtemperature = overtemp
		}
		if uptime, ok := sys["uptime"].(float64); ok {
			status.Uptime = int(uptime)
		}
		if ramTotal, ok := sys["ram_total"].(float64); ok {
			status.RAMTotal = int(ramTotal)
		}
		if ramFree, ok := sys["ram_free"].(float64); ok {
			status.RAMFree = int(ramFree)
		}
		if fsSize, ok := sys["fs_size"].(float64); ok {
			status.FSSize = int(fsSize)
		}
		if fsFree, ok := sys["fs_free"].(float64); ok {
			status.FSFree = int(fsFree)
		}
	}

	// Parse WiFi status
	if wifi, ok := rawStatus["wifi"].(map[string]interface{}); ok {
		status.WiFiStatus = &shelly.WiFiStatus{}
		if sta, ok := wifi["sta_ip"].(string); ok && sta != "" {
			status.WiFiStatus.Connected = true
			status.WiFiStatus.IP = sta
		}
		if ssid, ok := wifi["ssid"].(string); ok {
			status.WiFiStatus.SSID = ssid
		}
		if rssi, ok := wifi["rssi"].(float64); ok {
			status.WiFiStatus.RSSI = int(rssi)
		}
	}

	// Parse switch status (switch:0, switch:1, etc.)
	for key, value := range rawStatus {
		if len(key) > 7 && key[:7] == "switch:" {
			if switchData, ok := value.(map[string]interface{}); ok {
				sw := shelly.SwitchStatus{}
				// Parse switch ID from key
				if _, err := fmt.Sscanf(key, "switch:%d", &sw.ID); err != nil {
					c.logger.WithFields(map[string]any{
						"component": "shelly_gen2",
						"key":       key,
						"error":     err,
					}).Debug("Failed to parse switch ID from key")
					continue
				}

				if output, ok := switchData["output"].(bool); ok {
					sw.Output = output
				}
				if apower, ok := switchData["apower"].(float64); ok {
					sw.APower = apower
				}
				if voltage, ok := switchData["voltage"].(float64); ok {
					sw.Voltage = voltage
				}
				if current, ok := switchData["current"].(float64); ok {
					sw.Current = current
				}
				if temp, ok := switchData["temperature"].(map[string]interface{}); ok {
					if tC, ok := temp["tC"].(float64); ok {
						sw.Temperature = tC
					}
				}
				if source, ok := switchData["source"].(string); ok {
					sw.Source = source
				}

				status.Switches = append(status.Switches, sw)
			}
		}
	}

	return status, nil
}

// GetConfig retrieves device configuration
func (c *Client) GetConfig(ctx context.Context) (*shelly.DeviceConfig, error) {
	var rawConfig map[string]interface{}
	if err := c.rpcCall(ctx, "Shelly.GetConfig", nil, &rawConfig); err != nil {
		return nil, err
	}

	rawJSON, _ := json.Marshal(rawConfig)

	config := &shelly.DeviceConfig{
		Raw: rawJSON,
	}

	// Parse system config
	if sys, ok := rawConfig["sys"].(map[string]interface{}); ok {
		if device, ok := sys["device"].(map[string]interface{}); ok {
			if name, ok := device["name"].(string); ok {
				config.Name = name
			}
		}
		if location, ok := sys["location"].(map[string]interface{}); ok {
			if tz, ok := location["tz"].(string); ok {
				config.Timezone = tz
			}
			if lat, ok := location["lat"].(float64); ok {
				config.Lat = lat
			}
			if lon, ok := location["lon"].(float64); ok {
				config.Lng = lon
			}
		}
		if debug, ok := sys["debug"].(map[string]interface{}); ok {
			if enable, ok := debug["enable"].(bool); ok {
				config.Debug = enable
			}
		}
	}

	// Parse WiFi config
	if wifi, ok := rawConfig["wifi"].(map[string]interface{}); ok {
		if sta, ok := wifi["sta"].(map[string]interface{}); ok {
			config.WiFi = &shelly.WiFiConfig{}
			if enable, ok := sta["enable"].(bool); ok {
				config.WiFi.Enable = enable
			}
			if ssid, ok := sta["ssid"].(string); ok {
				config.WiFi.SSID = ssid
			}
			if pass, ok := sta["pass"].(string); ok {
				config.WiFi.Password = pass
			}
			if ipv4mode, ok := sta["ipv4mode"].(string); ok {
				config.WiFi.IPV4Mode = ipv4mode
			}
			if ip, ok := sta["ip"].(string); ok {
				config.WiFi.IP = ip
			}
			if netmask, ok := sta["netmask"].(string); ok {
				config.WiFi.Netmask = netmask
			}
			if gw, ok := sta["gw"].(string); ok {
				config.WiFi.Gateway = gw
			}
			if nameserver, ok := sta["nameserver"].(string); ok {
				config.WiFi.DNS = nameserver
			}
		}
	}

	// Parse cloud config
	if cloud, ok := rawConfig["cloud"].(map[string]interface{}); ok {
		config.Cloud = &shelly.CloudConfig{}
		if enable, ok := cloud["enable"].(bool); ok {
			config.Cloud.Enable = enable
		}
		if server, ok := cloud["server"].(string); ok {
			config.Cloud.Server = server
		}
	}

	// Parse switch configs
	for key, value := range rawConfig {
		if len(key) > 7 && key[:7] == "switch:" {
			if switchData, ok := value.(map[string]interface{}); ok {
				sw := shelly.SwitchConfig{}
				if _, err := fmt.Sscanf(key, "switch:%d", &sw.ID); err != nil {
					c.logger.WithFields(map[string]any{
						"component": "shelly_gen2",
						"key":       key,
						"error":     err,
					}).Debug("Failed to parse switch ID from config key")
					continue
				}

				if name, ok := switchData["name"].(string); ok {
					sw.Name = name
				}
				if inMode, ok := switchData["in_mode"].(string); ok {
					sw.InMode = inMode
				}
				if initialState, ok := switchData["initial_state"].(string); ok {
					sw.InitialState = initialState
				}
				if autoOn, ok := switchData["auto_on"].(float64); ok {
					sw.AutoOn = int(autoOn)
				}
				if autoOff, ok := switchData["auto_off"].(float64); ok {
					sw.AutoOff = int(autoOff)
				}

				config.Switches = append(config.Switches, sw)
			}
		}
	}

	return config, nil
}

// SetConfig updates device configuration
func (c *Client) SetConfig(ctx context.Context, config map[string]interface{}) error {
	// Gen2+ devices require component-specific config calls
	// This is a simplified implementation
	return c.rpcCall(ctx, "Shelly.SetConfig", map[string]interface{}{
		"config": config,
	}, nil)
}

// SetAuth sets authentication credentials
func (c *Client) SetAuth(ctx context.Context, username, password string) error {
	return c.rpcCall(ctx, "Shelly.SetAuth", map[string]interface{}{
		"user": username,
		"pass": password,
	}, nil)
}

// ResetAuth disables authentication
func (c *Client) ResetAuth(ctx context.Context) error {
	return c.rpcCall(ctx, "Shelly.SetAuth", map[string]interface{}{
		"user": "",
	}, nil)
}

// SetSwitch controls a switch/relay
func (c *Client) SetSwitch(ctx context.Context, channel int, on bool) error {
	return c.rpcCall(ctx, "Switch.Set", map[string]interface{}{
		"id": channel,
		"on": on,
	}, nil)
}

// TestConnection tests the connection to the device
func (c *Client) TestConnection(ctx context.Context) error {
	// GetInfo is sufficient to test connection and returns auth errors
	_, err := c.GetInfo(ctx)
	return err
}

// GetGeneration returns the device generation
func (c *Client) GetGeneration() int {
	return c.generation
}

// GetIP returns the device IP address
func (c *Client) GetIP() string {
	return c.ip
}

// rpcCall performs a JSON-RPC call to the device
func (c *Client) rpcCall(ctx context.Context, method string, params interface{}, result interface{}) error {
	url := fmt.Sprintf("http://%s/rpc", c.ip)

	request := RPCRequest{
		ID:     1,
		Method: method,
		Params: params,
	}

	reqBody, err := json.Marshal(request)
	if err != nil {
		return err
	}

	// Retry logic
	var lastErr error
	for attempt := 0; attempt <= c.config.retryAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(c.config.retryDelay)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
		if err != nil {
			return err
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", c.config.userAgent)

		var resp *http.Response
		// Use digest auth if configured
		if c.config.username != "" && c.config.password != "" {
			resp, err = doRequestWithDigestAuth(c.httpClient, req, c.config.username, c.config.password)
		} else {
			resp, err = c.httpClient.Do(req)
		}
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusUnauthorized {
			return shelly.ErrAuthRequired
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = &shelly.DeviceError{
				IP:         c.ip,
				Generation: c.generation,
				Operation:  method,
				StatusCode: resp.StatusCode,
			}
			continue
		}

		var rpcResp RPCResponse
		if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
			return &shelly.DeviceError{
				IP:         c.ip,
				Generation: c.generation,
				Operation:  method,
				Err:        err,
			}
		}

		if rpcResp.Error != nil {
			return &shelly.DeviceError{
				IP:         c.ip,
				Generation: c.generation,
				Operation:  method,
				Message:    rpcResp.Error.Message,
			}
		}

		if result != nil && rpcResp.Result != nil {
			if err := json.Unmarshal(rpcResp.Result, result); err != nil {
				return &shelly.DeviceError{
					IP:         c.ip,
					Generation: c.generation,
					Operation:  method,
					Err:        err,
				}
			}
		}

		return nil
	}

	return lastErr
}

// SetBrightness sets the brightness of a light channel
func (c *Client) SetBrightness(ctx context.Context, channel int, brightness int) error {
	params := map[string]interface{}{
		"id":         channel,
		"brightness": brightness,
	}
	return c.rpcCall(ctx, "Light.Set", params, nil)
}

// SetColorRGB sets the RGB color of a light channel
func (c *Client) SetColorRGB(ctx context.Context, channel int, r, g, b uint8) error {
	params := map[string]interface{}{
		"id":  channel,
		"rgb": []int{int(r), int(g), int(b)},
	}
	return c.rpcCall(ctx, "Light.Set", params, nil)
}

// SetColorTemp sets the color temperature of a light channel
func (c *Client) SetColorTemp(ctx context.Context, channel int, temp int) error {
	params := map[string]interface{}{
		"id":   channel,
		"temp": temp,
	}
	return c.rpcCall(ctx, "Light.Set", params, nil)
}

// Reboot restarts the device
func (c *Client) Reboot(ctx context.Context) error {
	return c.rpcCall(ctx, "Shelly.Reboot", nil, nil)
}

// FactoryReset resets the device to factory defaults
func (c *Client) FactoryReset(ctx context.Context) error {
	return c.rpcCall(ctx, "Shelly.FactoryReset", nil, nil)
}

// CheckUpdate checks for firmware updates
func (c *Client) CheckUpdate(ctx context.Context) (*shelly.UpdateInfo, error) {
	var result struct {
		Stable struct {
			Version string `json:"version"`
			Build   string `json:"build_id"`
		} `json:"stable"`
		Beta struct {
			Version string `json:"version"`
			Build   string `json:"build_id"`
		} `json:"beta"`
	}

	if err := c.rpcCall(ctx, "Shelly.CheckForUpdate", nil, &result); err != nil {
		return nil, err
	}

	// Get current version from device info
	info, err := c.GetInfo(ctx)
	if err != nil {
		return nil, err
	}

	updateInfo := &shelly.UpdateInfo{
		OldVersion: info.FW,
		HasUpdate:  result.Stable.Version != "" && result.Stable.Version != info.FW,
	}

	if updateInfo.HasUpdate {
		updateInfo.NewVersion = result.Stable.Version
		if result.Beta.Version != "" && result.Beta.Version != info.FW {
			updateInfo.ReleaseNotes = fmt.Sprintf("Beta version %s also available", result.Beta.Version)
		}
	}

	return updateInfo, nil
}

// PerformUpdate initiates a firmware update
func (c *Client) PerformUpdate(ctx context.Context) error {
	params := map[string]interface{}{
		"stage": "stable",
	}
	return c.rpcCall(ctx, "Shelly.Update", params, nil)
}

// GetMetrics retrieves device metrics
func (c *Client) GetMetrics(ctx context.Context) (*shelly.DeviceMetrics, error) {
	status, err := c.GetStatus(ctx)
	if err != nil {
		return nil, err
	}

	metrics := &shelly.DeviceMetrics{
		Timestamp:   time.Now(),
		Temperature: status.Temperature,
		Uptime:      status.Uptime,
	}

	if status.WiFiStatus != nil {
		metrics.WiFiRSSI = status.WiFiStatus.RSSI
	}

	return metrics, nil
}

// GetEnergyData retrieves energy consumption data for a channel
func (c *Client) GetEnergyData(ctx context.Context, channel int) (*shelly.EnergyData, error) {
	var result struct {
		Total   float64 `json:"total"`
		Current float64 `json:"current"`
		Voltage float64 `json:"voltage"`
		Power   float64 `json:"apower"`
	}

	params := map[string]interface{}{
		"id": channel,
	}

	if err := c.rpcCall(ctx, "Switch.GetStatus", params, &result); err != nil {
		return nil, err
	}

	return &shelly.EnergyData{
		Power:   result.Power,
		Total:   result.Total / 1000, // Convert Wh to kWh
		Voltage: result.Voltage,
		Current: result.Current,
	}, nil
}

// Roller Shutter Operations for Gen2+ devices

// SetRollerPosition sets the roller/cover position
func (c *Client) SetRollerPosition(ctx context.Context, channel int, position int) error {
	if position < 0 {
		position = 0
	} else if position > 100 {
		position = 100
	}

	params := map[string]interface{}{
		"id":  channel,
		"pos": position,
	}
	return c.rpcCall(ctx, "Cover.GoToPosition", params, nil)
}

// OpenRoller opens the roller/cover
func (c *Client) OpenRoller(ctx context.Context, channel int) error {
	params := map[string]interface{}{
		"id": channel,
	}
	return c.rpcCall(ctx, "Cover.Open", params, nil)
}

// CloseRoller closes the roller/cover
func (c *Client) CloseRoller(ctx context.Context, channel int) error {
	params := map[string]interface{}{
		"id": channel,
	}
	return c.rpcCall(ctx, "Cover.Close", params, nil)
}

// StopRoller stops the roller/cover movement
func (c *Client) StopRoller(ctx context.Context, channel int) error {
	params := map[string]interface{}{
		"id": channel,
	}
	return c.rpcCall(ctx, "Cover.Stop", params, nil)
}

// Advanced Settings Operations for Gen2+ devices

// SetRelaySettings updates relay-specific settings
func (c *Client) SetRelaySettings(ctx context.Context, channel int, settings map[string]interface{}) error {
	params := map[string]interface{}{
		"id":     channel,
		"config": settings,
	}
	return c.rpcCall(ctx, "Switch.SetConfig", params, nil)
}

// SetLightSettings updates light-specific settings
func (c *Client) SetLightSettings(ctx context.Context, channel int, settings map[string]interface{}) error {
	params := map[string]interface{}{
		"id":     channel,
		"config": settings,
	}
	return c.rpcCall(ctx, "Light.SetConfig", params, nil)
}

// SetInputSettings configures input behavior
func (c *Client) SetInputSettings(ctx context.Context, input int, settings map[string]interface{}) error {
	params := map[string]interface{}{
		"id":     input,
		"config": settings,
	}
	return c.rpcCall(ctx, "Input.SetConfig", params, nil)
}

// SetLEDSettings configures LED indicator behavior (Gen2+ uses Sys.SetConfig)
func (c *Client) SetLEDSettings(ctx context.Context, settings map[string]interface{}) error {
	params := map[string]interface{}{
		"config": map[string]interface{}{
			"ui_data": settings,
		},
	}
	return c.rpcCall(ctx, "Sys.SetConfig", params, nil)
}

// RGBW Operations for Gen2+ devices

// SetWhiteChannel controls white channel for RGBW devices
func (c *Client) SetWhiteChannel(ctx context.Context, channel int, brightness int, temp int) error {
	params := map[string]interface{}{
		"id":         channel,
		"on":         true,
		"brightness": brightness,
	}
	if temp > 0 {
		params["temp"] = temp
	}
	return c.rpcCall(ctx, "Light.Set", params, nil)
}

// SetColorMode sets the mode for RGBW devices (not directly available in Gen2+, handled via Light.Set)
func (c *Client) SetColorMode(ctx context.Context, mode string) error {
	// Gen2+ handles this differently - the mode is implicit based on what parameters are set
	// in Light.Set calls. This is a compatibility method.
	return nil
}
