package gen1

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/shelly"
)

// Client implements the shelly.Client interface for Gen1 devices
type Client struct {
	ip         string
	httpClient *http.Client
	config     *clientConfig
	logger     *logging.Logger
	generation int
}

// clientConfig holds configuration for the Gen1 client
type clientConfig struct {
	username      string
	password      string
	timeout       time.Duration
	retryAttempts int
	retryDelay    time.Duration
	skipTLSVerify bool
	userAgent     string
}

// ClientOption represents a configuration option for Gen1 client
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

// NewClient creates a new Gen1 Shelly client
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
		generation: 1,
	}
}

// GetInfo retrieves device information
func (c *Client) GetInfo(ctx context.Context) (*shelly.DeviceInfo, error) {
	url := fmt.Sprintf("http://%s/shelly", c.ip)
	
	var rawInfo struct {
		Type       string `json:"type"`
		MAC        string `json:"mac"`
		Auth       bool   `json:"auth"`
		FW         string `json:"fw"`
		LongID     int    `json:"longid"`
		NumOutputs int    `json:"num_outputs"`
		NumMeters  int    `json:"num_meters"`
		NumRollers int    `json:"num_rollers"`
	}
	
	if err := c.getJSON(ctx, url, &rawInfo); err != nil {
		return nil, err
	}
	
	info := &shelly.DeviceInfo{
		ID:         fmt.Sprintf("shelly%s-%s", rawInfo.Type, rawInfo.MAC),
		MAC:        rawInfo.MAC,
		Model:      rawInfo.Type,
		Type:       rawInfo.Type,
		Generation: 1,
		FW:         rawInfo.FW,
		Version:    rawInfo.FW,
		Auth:       rawInfo.Auth,
		AuthEn:     rawInfo.Auth,
		IP:         c.ip,
		Discovered: time.Now(),
	}
	
	return info, nil
}

// GetStatus retrieves the current device status
func (c *Client) GetStatus(ctx context.Context) (*shelly.DeviceStatus, error) {
	url := fmt.Sprintf("http://%s/status", c.ip)
	
	var rawStatus map[string]interface{}
	if err := c.getJSON(ctx, url, &rawStatus); err != nil {
		return nil, err
	}
	
	status := &shelly.DeviceStatus{
		Raw: rawStatus,
	}
	
	// Parse common fields
	if temp, ok := rawStatus["temperature"].(float64); ok {
		status.Temperature = temp
	}
	
	if overtemp, ok := rawStatus["overtemperature"].(bool); ok {
		status.Overtemperature = overtemp
	}
	
	if uptime, ok := rawStatus["uptime"].(float64); ok {
		status.Uptime = int(uptime)
	}
	
	if hasUpdate, ok := rawStatus["has_update"].(bool); ok {
		status.HasUpdate = hasUpdate
	}
	
	// Parse WiFi status
	if wifiData, ok := rawStatus["wifi_sta"].(map[string]interface{}); ok {
		status.WiFiStatus = &shelly.WiFiStatus{
			Connected: wifiData["connected"].(bool),
		}
		if ssid, ok := wifiData["ssid"].(string); ok {
			status.WiFiStatus.SSID = ssid
		}
		if ip, ok := wifiData["ip"].(string); ok {
			status.WiFiStatus.IP = ip
		}
		if rssi, ok := wifiData["rssi"].(float64); ok {
			status.WiFiStatus.RSSI = int(rssi)
		}
	}
	
	// Parse relays/switches
	if relays, ok := rawStatus["relays"].([]interface{}); ok {
		for i, relay := range relays {
			if relayData, ok := relay.(map[string]interface{}); ok {
				sw := shelly.SwitchStatus{
					ID: i,
				}
				if ison, ok := relayData["ison"].(bool); ok {
					sw.Output = ison
				}
				if source, ok := relayData["source"].(string); ok {
					sw.Source = source
				}
				status.Switches = append(status.Switches, sw)
			}
		}
	}
	
	// Parse meters
	if meters, ok := rawStatus["meters"].([]interface{}); ok {
		for i, meter := range meters {
			if meterData, ok := meter.(map[string]interface{}); ok {
				m := shelly.MeterStatus{
					ID: i,
				}
				if power, ok := meterData["power"].(float64); ok {
					m.Power = power
				}
				if total, ok := meterData["total"].(float64); ok {
					m.Total = total
				}
				if isValid, ok := meterData["is_valid"].(bool); ok {
					m.IsValid = isValid
				}
				status.Meters = append(status.Meters, m)
			}
		}
	}
	
	return status, nil
}

// GetConfig retrieves device configuration
func (c *Client) GetConfig(ctx context.Context) (*shelly.DeviceConfig, error) {
	url := fmt.Sprintf("http://%s/settings", c.ip)
	
	var rawConfig map[string]interface{}
	if err := c.getJSON(ctx, url, &rawConfig); err != nil {
		return nil, err
	}
	
	rawJSON, _ := json.Marshal(rawConfig)
	
	config := &shelly.DeviceConfig{
		Raw: rawJSON,
	}
	
	// Parse basic settings
	if name, ok := rawConfig["name"].(string); ok {
		config.Name = name
	}
	
	if tz, ok := rawConfig["timezone"].(string); ok {
		config.Timezone = tz
	}
	
	if lat, ok := rawConfig["lat"].(float64); ok {
		config.Lat = lat
	}
	
	if lng, ok := rawConfig["lng"].(float64); ok {
		config.Lng = lng
	}
	
	// Parse WiFi settings
	if wifiData, ok := rawConfig["wifi_sta"].(map[string]interface{}); ok {
		config.WiFi = &shelly.WiFiConfig{}
		if enabled, ok := wifiData["enabled"].(bool); ok {
			config.WiFi.Enable = enabled
		}
		if ssid, ok := wifiData["ssid"].(string); ok {
			config.WiFi.SSID = ssid
		}
		if ipv4Method, ok := wifiData["ipv4_method"].(string); ok {
			config.WiFi.IPV4Mode = ipv4Method
		}
		if ip, ok := wifiData["ip"].(string); ok {
			config.WiFi.IP = ip
		}
		if netmask, ok := wifiData["mask"].(string); ok {
			config.WiFi.Netmask = netmask
		}
		if gw, ok := wifiData["gw"].(string); ok {
			config.WiFi.Gateway = gw
		}
		if dns, ok := wifiData["dns"].(string); ok {
			config.WiFi.DNS = dns
		}
	}
	
	// Parse cloud settings
	if cloudData, ok := rawConfig["cloud"].(map[string]interface{}); ok {
		config.Cloud = &shelly.CloudConfig{}
		if enabled, ok := cloudData["enabled"].(bool); ok {
			config.Cloud.Enable = enabled
		}
	}
	
	// Parse relay settings
	if relays, ok := rawConfig["relays"].([]interface{}); ok {
		for i, relay := range relays {
			if relayData, ok := relay.(map[string]interface{}); ok {
				sw := shelly.SwitchConfig{
					ID: i,
				}
				if name, ok := relayData["name"].(string); ok {
					sw.Name = name
				}
				if autoOn, ok := relayData["auto_on"].(float64); ok {
					sw.AutoOn = int(autoOn)
				}
				if autoOff, ok := relayData["auto_off"].(float64); ok {
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
	url := fmt.Sprintf("http://%s/settings", c.ip)
	return c.postForm(ctx, url, config)
}

// SetAuth sets authentication credentials
func (c *Client) SetAuth(ctx context.Context, username, password string) error {
	url := fmt.Sprintf("http://%s/settings/login", c.ip)
	params := map[string]interface{}{
		"enabled":  true,
		"username": username,
		"password": password,
	}
	return c.postForm(ctx, url, params)
}

// ResetAuth disables authentication
func (c *Client) ResetAuth(ctx context.Context) error {
	url := fmt.Sprintf("http://%s/settings/login", c.ip)
	params := map[string]interface{}{
		"enabled": false,
	}
	return c.postForm(ctx, url, params)
}

// SetSwitch controls a switch/relay
func (c *Client) SetSwitch(ctx context.Context, channel int, on bool) error {
	url := fmt.Sprintf("http://%s/relay/%d", c.ip, channel)
	params := map[string]interface{}{
		"turn": map[bool]string{true: "on", false: "off"}[on],
	}
	return c.postForm(ctx, url, params)
}

// SetBrightness controls light brightness (for dimmers and lights)
func (c *Client) SetBrightness(ctx context.Context, channel int, brightness int) error {
	// Ensure brightness is within valid range
	if brightness < 0 {
		brightness = 0
	} else if brightness > 100 {
		brightness = 100
	}
	
	url := fmt.Sprintf("http://%s/light/%d", c.ip, channel)
	params := map[string]interface{}{
		"turn":       "on",
		"brightness": brightness,
	}
	return c.postForm(ctx, url, params)
}

// SetColorRGB sets RGB color for RGBW devices
func (c *Client) SetColorRGB(ctx context.Context, channel int, r, g, b uint8) error {
	url := fmt.Sprintf("http://%s/color/%d", c.ip, channel)
	params := map[string]interface{}{
		"turn":  "on",
		"red":   r,
		"green": g,
		"blue":  b,
	}
	return c.postForm(ctx, url, params)
}

// SetColorTemp sets color temperature for CCT lights
func (c *Client) SetColorTemp(ctx context.Context, channel int, temp int) error {
	// Temp is typically in Kelvin (2700-6500K)
	if temp < 2700 {
		temp = 2700
	} else if temp > 6500 {
		temp = 6500
	}
	
	url := fmt.Sprintf("http://%s/light/%d", c.ip, channel)
	params := map[string]interface{}{
		"turn": "on",
		"temp": temp,
	}
	return c.postForm(ctx, url, params)
}

// Reboot reboots the device
func (c *Client) Reboot(ctx context.Context) error {
	url := fmt.Sprintf("http://%s/reboot", c.ip)
	return c.postForm(ctx, url, nil)
}

// FactoryReset performs a factory reset of the device
func (c *Client) FactoryReset(ctx context.Context) error {
	url := fmt.Sprintf("http://%s/settings/factory_reset", c.ip)
	return c.postForm(ctx, url, nil)
}

// CheckUpdate checks for firmware updates
func (c *Client) CheckUpdate(ctx context.Context) (*shelly.UpdateInfo, error) {
	url := fmt.Sprintf("http://%s/ota", c.ip)
	
	var result struct {
		HasUpdate bool   `json:"has_update"`
		NewVersion string `json:"new_version"`
		OldVersion string `json:"old_version"`
		ReleaseNotes string `json:"release_notes"`
	}
	
	if err := c.getJSON(ctx, url, &result); err != nil {
		return nil, err
	}
	
	return &shelly.UpdateInfo{
		HasUpdate:    result.HasUpdate,
		NewVersion:   result.NewVersion,
		OldVersion:   result.OldVersion,
		ReleaseNotes: result.ReleaseNotes,
	}, nil
}

// PerformUpdate triggers a firmware update
func (c *Client) PerformUpdate(ctx context.Context) error {
	url := fmt.Sprintf("http://%s/ota", c.ip)
	params := map[string]interface{}{
		"update": true,
	}
	return c.postForm(ctx, url, params)
}

// GetMetrics retrieves device performance metrics
func (c *Client) GetMetrics(ctx context.Context) (*shelly.DeviceMetrics, error) {
	// Get device status for metrics
	status, err := c.GetStatus(ctx)
	if err != nil {
		return nil, err
	}
	
	metrics := &shelly.DeviceMetrics{
		Timestamp: time.Now(),
		Uptime:    status.Uptime,
	}
	
	// Calculate RAM usage if available
	if status.RAMTotal > 0 && status.RAMFree > 0 {
		used := status.RAMTotal - status.RAMFree
		metrics.RAMUsage = float64(used) / float64(status.RAMTotal) * 100
	}
	
	// Calculate FS usage if available
	if status.FSSize > 0 && status.FSFree > 0 {
		used := status.FSSize - status.FSFree
		metrics.FSUsage = float64(used) / float64(status.FSSize) * 100
	}
	
	// Add temperature
	metrics.Temperature = status.Temperature
	
	// Add WiFi RSSI
	if status.WiFiStatus != nil {
		metrics.WiFiRSSI = status.WiFiStatus.RSSI
	}
	
	return metrics, nil
}

// GetEnergyData retrieves energy consumption data for a channel
func (c *Client) GetEnergyData(ctx context.Context, channel int) (*shelly.EnergyData, error) {
	url := fmt.Sprintf("http://%s/meter/%d", c.ip, channel)
	
	var result struct {
		Power         float64 `json:"power"`
		IsValid       bool    `json:"is_valid"`
		Total         float64 `json:"total"`
		TotalReturned float64 `json:"total_returned"`
		Voltage       float64 `json:"voltage"`
		Current       float64 `json:"current"`
		PF            float64 `json:"pf"`
	}
	
	if err := c.getJSON(ctx, url, &result); err != nil {
		return nil, err
	}
	
	return &shelly.EnergyData{
		Timestamp:     time.Now(),
		Power:         result.Power,
		Total:         result.Total / 1000,         // Convert Wh to kWh
		TotalReturned: result.TotalReturned / 1000, // Convert Wh to kWh
		Voltage:       result.Voltage,
		Current:       result.Current,
		PowerFactor:   result.PF,
	}, nil
}

// TestConnection tests the connection to the device
func (c *Client) TestConnection(ctx context.Context) error {
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

// Helper methods for HTTP operations

func (c *Client) getJSON(ctx context.Context, url string, result interface{}) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	
	// Add authentication if configured
	if c.config.username != "" && c.config.password != "" {
		req.SetBasicAuth(c.config.username, c.config.password)
	}
	
	req.Header.Set("User-Agent", c.config.userAgent)
	
	// Retry logic
	var lastErr error
	for attempt := 0; attempt <= c.config.retryAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(c.config.retryDelay)
		}
		
		resp, err := c.httpClient.Do(req)
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
				Operation:  "GET " + url,
				StatusCode: resp.StatusCode,
			}
			continue
		}
		
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return &shelly.DeviceError{
				IP:         c.ip,
				Generation: c.generation,
				Operation:  "GET " + url,
				Err:        err,
			}
		}
		
		return nil
	}
	
	return lastErr
}

func (c *Client) postForm(ctx context.Context, endpoint string, params map[string]interface{}) error {
	// Convert params to URL-encoded form data
	formData := make(url.Values)
	for key, value := range params {
		switch v := value.(type) {
		case string:
			formData.Set(key, v)
		case bool:
			formData.Set(key, fmt.Sprintf("%t", v))
		case int:
			formData.Set(key, fmt.Sprintf("%d", v))
		case float64:
			formData.Set(key, fmt.Sprintf("%g", v))
		default:
			formData.Set(key, fmt.Sprintf("%v", v))
		}
	}
	
	// Retry logic
	var lastErr error
	for attempt := 0; attempt <= c.config.retryAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(c.config.retryDelay)
		}
		
		req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(formData.Encode()))
		if err != nil {
			return err
		}
		
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("User-Agent", c.config.userAgent)
		
		// Add authentication if configured
		if c.config.username != "" && c.config.password != "" {
			req.SetBasicAuth(c.config.username, c.config.password)
		}
		
		resp, err := c.httpClient.Do(req)
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
				Operation:  "POST " + endpoint,
				StatusCode: resp.StatusCode,
			}
			continue
		}
		
		// Gen1 devices typically return a simple JSON response for POST
		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			// Some endpoints return plain text, that's OK
			return nil
		}
		
		// Check for error in response
		if errMsg, ok := result["error"].(string); ok && errMsg != "" {
			return &shelly.DeviceError{
				IP:         c.ip,
				Generation: c.generation,
				Operation:  "POST " + endpoint,
				Message:    errMsg,
			}
		}
		
		return nil
	}
	
	return lastErr
}