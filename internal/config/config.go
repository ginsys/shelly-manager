package config

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

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
		// Legacy field for backward compatibility
		Path string `mapstructure:"path"`

		// New provider-based configuration
		Provider        string            `mapstructure:"provider"` // "sqlite", "postgresql", "mysql"
		DSN             string            `mapstructure:"dsn"`      // Data Source Name
		MaxOpenConns    int               `mapstructure:"max_open_conns"`
		MaxIdleConns    int               `mapstructure:"max_idle_conns"`
		ConnMaxLifetime int               `mapstructure:"conn_max_lifetime"`  // seconds
		ConnMaxIdleTime int               `mapstructure:"conn_max_idle_time"` // seconds
		SlowQueryTime   int               `mapstructure:"slow_query_time"`    // milliseconds
		LogLevel        string            `mapstructure:"log_level"`          // "silent", "error", "warn", "info"
		Options         map[string]string `mapstructure:"options"`            // Provider-specific options
	} `mapstructure:"database"`
	Discovery struct {
		Enabled         bool     `mapstructure:"enabled"`
		Networks        []string `mapstructure:"networks"`
		Interval        int      `mapstructure:"interval"`
		Timeout         int      `mapstructure:"timeout"`
		EnableMDNS      bool     `mapstructure:"enable_mdns"`
		EnableSSDP      bool     `mapstructure:"enable_ssdp"`
		ConcurrentScans int      `mapstructure:"concurrent_scans"`
	} `mapstructure:"discovery"`
	Provisioning struct {
		AuthEnabled       bool   `mapstructure:"auth_enabled"`
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
		Network     string `mapstructure:"network"`
		StartIP     string `mapstructure:"start_ip"`
		EndIP       string `mapstructure:"end_ip"`
		AutoReserve bool   `mapstructure:"auto_reserve"`
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
	API struct {
		URL           string `mapstructure:"url"`
		Key           string `mapstructure:"key"`
		Timeout       int    `mapstructure:"timeout"`
		RetryAttempts int    `mapstructure:"retry_attempts"`
		RetryDelay    int    `mapstructure:"retry_delay"`
	} `mapstructure:"api"`
	Notifications struct {
		Enabled bool `mapstructure:"enabled"`
		Email   struct {
			SMTPHost     string `mapstructure:"smtp_host"`
			SMTPPort     int    `mapstructure:"smtp_port"`
			SMTPUser     string `mapstructure:"smtp_user"`
			SMTPPassword string `mapstructure:"smtp_password"`
			FromAddress  string `mapstructure:"from_address"`
			TLS          bool   `mapstructure:"tls"`
		} `mapstructure:"email"`
		Webhooks []struct {
			URL     string            `mapstructure:"url"`
			Secret  string            `mapstructure:"secret"`
			Headers map[string]string `mapstructure:"headers"`
		} `mapstructure:"webhooks"`
		Thresholds struct {
			CriticalDriftCount int `mapstructure:"critical_drift_count"`
			WarningDriftCount  int `mapstructure:"warning_drift_count"`
			MaxPerHour         int `mapstructure:"max_per_hour"`
		} `mapstructure:"thresholds"`
	} `mapstructure:"notifications"`
	Resolution struct {
		AutoFixEnabled    bool     `mapstructure:"auto_fix_enabled"`
		SafeMode          bool     `mapstructure:"safe_mode"`
		ApprovalRequired  bool     `mapstructure:"approval_required"`
		AutoFixCategories []string `mapstructure:"auto_fix_categories"`
		ExcludedPaths     []string `mapstructure:"excluded_paths"`
	} `mapstructure:"resolution"`
	Metrics struct {
		Enabled              bool `mapstructure:"enabled"`
		PrometheusEnabled    bool `mapstructure:"prometheus_enabled"`
		PrometheusPort       int  `mapstructure:"prometheus_port"`
		CollectionInterval   int  `mapstructure:"collection_interval"`
		RetentionDays        int  `mapstructure:"retention_days"`
		EnableHTTPMetrics    bool `mapstructure:"enable_http_metrics"`
		EnableDetailedTiming bool `mapstructure:"enable_detailed_timing"`
	} `mapstructure:"metrics"`
	Security struct {
		UseProxyHeaders bool     `mapstructure:"use_proxy_headers"`
		TrustedProxies  []string `mapstructure:"trusted_proxies"`
		CORS            struct {
			AllowedOrigins []string `mapstructure:"allowed_origins"`
			AllowedMethods []string `mapstructure:"allowed_methods"`
			AllowedHeaders []string `mapstructure:"allowed_headers"`
			MaxAge         int      `mapstructure:"max_age"`
		} `mapstructure:"cors"`
		// Simple admin API key for protecting sensitive endpoints until full auth is implemented
		AdminAPIKey string `mapstructure:"admin_api_key"`
		// Test mode to bypass security validations (for E2E testing)
		ValidationTestMode bool `mapstructure:"validation_test_mode"`
	} `mapstructure:"security"`

	// Export settings
	Export struct {
		// Optional base directory for generated export files. If set, downloads are restricted to this directory.
		OutputDirectory string `mapstructure:"output_directory"`
	} `mapstructure:"export"`

	// Sync settings (import/export base directories for path traversal protection)
	Sync struct {
		// ImportBaseDir restricts file imports to paths within this directory.
		// If empty, no restriction is applied.
		ImportBaseDir string `mapstructure:"import_base_dir"`
		// ExportBaseDir restricts file exports to paths within this directory.
		// If empty, no restriction is applied.
		ExportBaseDir string `mapstructure:"export_base_dir"`
	} `mapstructure:"sync"`
}

// Load loads configuration from file
func Load(configFile string) (*Config, error) {
	return LoadWithName(configFile, "shelly-manager")
}

// LoadWithName loads configuration from file with a specific config name
func LoadWithName(configFile string, configName string) (*Config, error) {
	// Reset viper state to prevent interference between config loads
	viper.Reset()

	// Environment variable overrides: SHELLY_ prefix, nested keys with underscores
	viper.SetEnvPrefix("SHELLY")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		// Default configuration search paths - be more specific to avoid binary files
		viper.SetConfigName(configName)
		viper.SetConfigType("yaml")

		// Only search in specific directories for config files
		viper.AddConfigPath("./configs")             // Project configs directory
		viper.AddConfigPath(".")                     // Current directory
		viper.AddConfigPath("$HOME/.shelly-manager") // User config directory
		viper.AddConfigPath("/etc/shelly-manager")   // System config directory

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
		// Enhanced error reporting with exact file path
		if configFile != "" {
			return nil, fmt.Errorf("failed to read config file at '%s': %w", configFile, err)
		}
		return nil, fmt.Errorf("failed to read config file (searched paths: %s): %w",
			"./configs, ., $HOME/.shelly-manager, /etc/shelly-manager", err)
	}

	// Report which config file was loaded
	configFilePath := viper.ConfigFileUsed()

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config from '%s': %w", configFilePath, err)
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
	viper.SetDefault("database.path", "data/shelly.db") // Legacy compatibility
	viper.SetDefault("database.provider", "sqlite")
	viper.SetDefault("database.dsn", "data/shelly.db")
	viper.SetDefault("database.max_open_conns", 1)
	viper.SetDefault("database.max_idle_conns", 1)
	viper.SetDefault("database.conn_max_lifetime", 300)  // 5 minutes
	viper.SetDefault("database.conn_max_idle_time", 600) // 10 minutes
	viper.SetDefault("database.slow_query_time", 500)    // 500ms
	viper.SetDefault("database.log_level", "warn")
	viper.SetDefault("database.options", map[string]string{
		"foreign_keys": "true",
		"journal_mode": "WAL",
		"synchronous":  "NORMAL",
		"cache_size":   "-64000", // 64MB
		"busy_timeout": "5000",   // 5 seconds
	})

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

	// API defaults (for provisioner agent mode)
	viper.SetDefault("api.url", "http://0.0.0.0:8080")
	viper.SetDefault("api.key", "")
	viper.SetDefault("api.timeout", 30)
	viper.SetDefault("api.retry_attempts", 3)
	viper.SetDefault("api.retry_delay", 5)

	// Metrics defaults
	viper.SetDefault("metrics.enabled", true)
	viper.SetDefault("metrics.prometheus_enabled", true)
	viper.SetDefault("metrics.prometheus_port", 9090)
	viper.SetDefault("metrics.collection_interval", 300)
	viper.SetDefault("metrics.retention_days", 30)
	viper.SetDefault("metrics.enable_http_metrics", true)
	viper.SetDefault("metrics.enable_detailed_timing", false)

	// Notification defaults
	viper.SetDefault("notifications.enabled", true)
	viper.SetDefault("notifications.thresholds.critical_drift_count", 5)
	viper.SetDefault("notifications.thresholds.warning_drift_count", 10)
	viper.SetDefault("notifications.thresholds.max_per_hour", 20)

	// Resolution defaults
	viper.SetDefault("resolution.auto_fix_enabled", false)
	viper.SetDefault("resolution.safe_mode", true)
	viper.SetDefault("resolution.approval_required", true)
	viper.SetDefault("resolution.auto_fix_categories", []string{"network", "time"})
	viper.SetDefault("resolution.excluded_paths", []string{"/debug", "/test"})

	// Security defaults
	viper.SetDefault("security.use_proxy_headers", false)
	viper.SetDefault("security.trusted_proxies", []string{})
	viper.SetDefault("security.cors.allowed_origins", []string{}) // empty => *
	viper.SetDefault("security.cors.allowed_methods", []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"})
	viper.SetDefault("security.cors.allowed_headers", []string{"Content-Type", "Authorization", "X-Requested-With"})
	viper.SetDefault("security.cors.max_age", 86400)
	// Admin API key disabled by default (empty)
	viper.SetDefault("security.admin_api_key", "")
	// Validation test mode disabled by default (security validations enabled)
	viper.SetDefault("security.validation_test_mode", false)

	// Export defaults
	viper.SetDefault("export.output_directory", "")

	// Sync defaults (path restriction disabled by default)
	viper.SetDefault("sync.import_base_dir", "")
	viper.SetDefault("sync.export_base_dir", "")
}
