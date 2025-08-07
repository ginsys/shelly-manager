package config

import (
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	Server struct {
		Port     int    `mapstructure:"port"`
		Host     string `mapstructure:"host"`
		LogLevel string `mapstructure:"log_level"`
	} `mapstructure:"server"`
	Logging struct {
		Level  string `mapstructure:"level"`
		Format string `mapstructure:"format"` // json, text
		Output string `mapstructure:"output"` // stdout, stderr, or file path
	} `mapstructure:"logging"`
	Database struct {
		Path string `mapstructure:"path"`
	} `mapstructure:"database"`
	Discovery struct {
		Enabled          bool     `mapstructure:"enabled"`
		Networks         []string `mapstructure:"networks"`
		Interval         int      `mapstructure:"interval"`
		Timeout          int      `mapstructure:"timeout"`
		EnableMDNS       bool     `mapstructure:"enable_mdns"`
		EnableSSDP       bool     `mapstructure:"enable_ssdp"`
		ConcurrentScans  int      `mapstructure:"concurrent_scans"`
	} `mapstructure:"discovery"`
	Provisioning struct {
		AuthEnabled        bool   `mapstructure:"auth_enabled"`
		AuthUser          string `mapstructure:"auth_user"`
		AuthPassword      string `mapstructure:"auth_password"`
		CloudEnabled      bool   `mapstructure:"cloud_enabled"`
		MQTTEnabled       bool   `mapstructure:"mqtt_enabled"`
		MQTTServer        string `mapstructure:"mqtt_server"`
		DeviceNamePattern string `mapstructure:"device_name_pattern"`
		AutoProvision     bool   `mapstructure:"auto_provision"`
		ProvisionInterval int    `mapstructure:"provision_interval"`
	} `mapstructure:"provisioning"`
	DHCP struct {
		Network   string `mapstructure:"network"`
		StartIP   string `mapstructure:"start_ip"`
		EndIP     string `mapstructure:"end_ip"`
		AutoReserve bool `mapstructure:"auto_reserve"`
	} `mapstructure:"dhcp"`
	OPNSense struct {
		Enabled   bool   `mapstructure:"enabled"`
		Host      string `mapstructure:"host"`
		Port      int    `mapstructure:"port"`
		APIKey    string `mapstructure:"api_key"`
		APISecret string `mapstructure:"api_secret"`
		AutoApply bool   `mapstructure:"auto_apply"`
	} `mapstructure:"opnsense"`
	MainApp struct {
		URL     string `mapstructure:"url"`
		APIKey  string `mapstructure:"api_key"`
		Enabled bool   `mapstructure:"enabled"`
	} `mapstructure:"main_app"`
}

// Load loads configuration from file
func Load(configFile string) (*Config, error) {
	if configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		// Default configuration search paths
		viper.SetConfigName("shelly-manager")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("./configs")
		viper.AddConfigPath("$HOME/.shelly-manager")
		viper.AddConfigPath("/etc/shelly-manager")
		
		// Add current directory based on executable location
		if _, filename, _, ok := runtime.Caller(0); ok {
			configDir := filepath.Dir(filepath.Dir(filepath.Dir(filename))) // Go up to project root
			viper.AddConfigPath(filepath.Join(configDir, "configs"))
		}
	}

	// Set default values
	setDefaults()

	// Read configuration
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

// setDefaults sets default configuration values
func setDefaults() {
	// Server defaults
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.log_level", "info")

	// Logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "text")
	viper.SetDefault("logging.output", "stdout")

	// Database defaults
	viper.SetDefault("database.path", "data/shelly.db")

	// Discovery defaults
	viper.SetDefault("discovery.enabled", true)
	viper.SetDefault("discovery.networks", []string{"192.168.1.0/24"})
	viper.SetDefault("discovery.interval", 300)
	viper.SetDefault("discovery.timeout", 5)
	viper.SetDefault("discovery.enable_mdns", true)
	viper.SetDefault("discovery.enable_ssdp", true)
	viper.SetDefault("discovery.concurrent_scans", 20)

	// Provisioning defaults
	viper.SetDefault("provisioning.auth_enabled", false)
	viper.SetDefault("provisioning.auth_user", "admin")
	viper.SetDefault("provisioning.cloud_enabled", false)
	viper.SetDefault("provisioning.mqtt_enabled", false)
	viper.SetDefault("provisioning.device_name_pattern", "shelly_{type}_{mac}")
	viper.SetDefault("provisioning.auto_provision", false)
	viper.SetDefault("provisioning.provision_interval", 600)

	// DHCP defaults
	viper.SetDefault("dhcp.network", "192.168.1.0/24")
	viper.SetDefault("dhcp.start_ip", "192.168.1.100")
	viper.SetDefault("dhcp.end_ip", "192.168.1.199")
	viper.SetDefault("dhcp.auto_reserve", false)

	// OPNSense defaults
	viper.SetDefault("opnsense.enabled", false)
	viper.SetDefault("opnsense.port", 443)
	viper.SetDefault("opnsense.auto_apply", false)

	// Main app defaults
	viper.SetDefault("main_app.url", "http://localhost:8080")
	viper.SetDefault("main_app.enabled", true)
}