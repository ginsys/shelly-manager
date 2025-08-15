package shelly

import (
	"context"
	"time"
)

// Client represents the main interface for interacting with Shelly devices
type Client interface {
	// Device Information
	GetInfo(ctx context.Context) (*DeviceInfo, error)
	GetStatus(ctx context.Context) (*DeviceStatus, error)
	GetConfig(ctx context.Context) (*DeviceConfig, error)

	// Configuration
	SetConfig(ctx context.Context, config map[string]interface{}) error
	SetAuth(ctx context.Context, username, password string) error
	ResetAuth(ctx context.Context) error

	// Control Operations
	SetSwitch(ctx context.Context, channel int, on bool) error
	SetBrightness(ctx context.Context, channel int, brightness int) error
	SetColorRGB(ctx context.Context, channel int, r, g, b uint8) error
	SetColorTemp(ctx context.Context, channel int, temp int) error

	// Roller Shutter Operations
	SetRollerPosition(ctx context.Context, channel int, position int) error
	OpenRoller(ctx context.Context, channel int) error
	CloseRoller(ctx context.Context, channel int) error
	StopRoller(ctx context.Context, channel int) error

	// Advanced Settings
	SetRelaySettings(ctx context.Context, channel int, settings map[string]interface{}) error
	SetLightSettings(ctx context.Context, channel int, settings map[string]interface{}) error
	SetInputSettings(ctx context.Context, input int, settings map[string]interface{}) error
	SetLEDSettings(ctx context.Context, settings map[string]interface{}) error

	// RGBW Operations
	SetWhiteChannel(ctx context.Context, channel int, brightness int, temp int) error
	SetColorMode(ctx context.Context, mode string) error

	// System Operations
	Reboot(ctx context.Context) error
	FactoryReset(ctx context.Context) error

	// Firmware Management
	CheckUpdate(ctx context.Context) (*UpdateInfo, error)
	PerformUpdate(ctx context.Context) error

	// Metrics and Monitoring
	GetMetrics(ctx context.Context) (*DeviceMetrics, error)
	GetEnergyData(ctx context.Context, channel int) (*EnergyData, error)

	// Connection Management
	TestConnection(ctx context.Context) error
	GetGeneration() int
	GetIP() string
}

// ClientOption represents a configuration option for the client
type ClientOption func(*clientConfig)

// clientConfig holds the configuration for a Shelly client
type clientConfig struct {
	username      string
	password      string
	timeout       time.Duration
	retryAttempts int
	retryDelay    time.Duration
	skipTLSVerify bool
	userAgent     string
}

// WithAuth sets authentication credentials
func WithAuth(username, password string) ClientOption {
	return func(c *clientConfig) {
		c.username = username
		c.password = password
	}
}

// WithTimeout sets the HTTP timeout for requests
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

// WithSkipTLSVerify disables TLS certificate verification (for self-signed certs)
func WithSkipTLSVerify(skip bool) ClientOption {
	return func(c *clientConfig) {
		c.skipTLSVerify = skip
	}
}

// WithUserAgent sets a custom user agent string
func WithUserAgent(userAgent string) ClientOption {
	return func(c *clientConfig) {
		c.userAgent = userAgent
	}
}
