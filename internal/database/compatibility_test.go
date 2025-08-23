package database

import (
	"testing"

	"github.com/ginsys/shelly-manager/internal/config"
	"github.com/ginsys/shelly-manager/internal/logging"
)

func TestCreateManagerV1Compatibility(t *testing.T) {
	// Create a legacy-style configuration
	cfg := &config.Config{}
	cfg.Database.Path = ":memory:" // In-memory SQLite database for testing

	logger := logging.GetDefault()

	// This should create a V1 manager since no advanced features are configured
	manager, err := CreateManagerWithLogger(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create database manager: %v", err)
	}
	defer manager.Close()

	// Verify it's a V1 manager
	managerType := GetManagerType(manager)
	if managerType != "V1-Legacy" {
		t.Errorf("Expected V1-Legacy manager, got %s", managerType)
	}

	// Test basic operations
	if manager.GetDB() == nil {
		t.Error("Manager returned nil database")
	}

	// Test that we can migrate models
	if err := manager.Migrate(&Device{}); err != nil {
		t.Errorf("Failed to migrate models: %v", err)
	}
}

func TestCreateManagerV2Advanced(t *testing.T) {
	// Create a configuration that should trigger V2 manager
	cfg := &config.Config{}
	cfg.Database.Provider = "sqlite"
	cfg.Database.DSN = ":memory:"
	cfg.Database.MaxOpenConns = 1
	cfg.Database.MaxIdleConns = 1
	cfg.Database.LogLevel = "silent"
	cfg.Database.Options = map[string]string{
		"foreign_keys": "true",
	}

	logger := logging.GetDefault()

	// This should create a V2 manager due to advanced configuration
	manager, err := CreateManagerWithLogger(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create database manager: %v", err)
	}
	defer manager.Close()

	// Verify it's a V2 manager
	managerType := GetManagerType(manager)
	if managerType != "V2-Provider-Based" {
		t.Errorf("Expected V2-Provider-Based manager, got %s", managerType)
	}

	// Test basic operations
	if manager.GetDB() == nil {
		t.Error("Manager returned nil database")
	}

	// Test that we can migrate models
	if err := manager.Migrate(&Device{}); err != nil {
		t.Errorf("Failed to migrate models: %v", err)
	}
}

func TestShouldUseV2Manager(t *testing.T) {
	tests := []struct {
		name     string
		config   config.Config
		expected bool
	}{
		{
			name: "Legacy path only",
			config: config.Config{
				Database: struct {
					Path            string            `mapstructure:"path"`
					Provider        string            `mapstructure:"provider"`
					DSN             string            `mapstructure:"dsn"`
					MaxOpenConns    int               `mapstructure:"max_open_conns"`
					MaxIdleConns    int               `mapstructure:"max_idle_conns"`
					ConnMaxLifetime int               `mapstructure:"conn_max_lifetime"`
					ConnMaxIdleTime int               `mapstructure:"conn_max_idle_time"`
					SlowQueryTime   int               `mapstructure:"slow_query_time"`
					LogLevel        string            `mapstructure:"log_level"`
					Options         map[string]string `mapstructure:"options"`
				}{
					Path: "data/shelly.db",
				},
			},
			expected: false,
		},
		{
			name: "PostgreSQL provider",
			config: config.Config{
				Database: struct {
					Path            string            `mapstructure:"path"`
					Provider        string            `mapstructure:"provider"`
					DSN             string            `mapstructure:"dsn"`
					MaxOpenConns    int               `mapstructure:"max_open_conns"`
					MaxIdleConns    int               `mapstructure:"max_idle_conns"`
					ConnMaxLifetime int               `mapstructure:"conn_max_lifetime"`
					ConnMaxIdleTime int               `mapstructure:"conn_max_idle_time"`
					SlowQueryTime   int               `mapstructure:"slow_query_time"`
					LogLevel        string            `mapstructure:"log_level"`
					Options         map[string]string `mapstructure:"options"`
				}{
					Provider: "postgresql",
					DSN:      "postgres://user:pass@localhost/db",
				},
			},
			expected: true,
		},
		{
			name: "Advanced connection pool",
			config: config.Config{
				Database: struct {
					Path            string            `mapstructure:"path"`
					Provider        string            `mapstructure:"provider"`
					DSN             string            `mapstructure:"dsn"`
					MaxOpenConns    int               `mapstructure:"max_open_conns"`
					MaxIdleConns    int               `mapstructure:"max_idle_conns"`
					ConnMaxLifetime int               `mapstructure:"conn_max_lifetime"`
					ConnMaxIdleTime int               `mapstructure:"conn_max_idle_time"`
					SlowQueryTime   int               `mapstructure:"slow_query_time"`
					LogLevel        string            `mapstructure:"log_level"`
					Options         map[string]string `mapstructure:"options"`
				}{
					Provider:     "sqlite",
					MaxOpenConns: 10,
					MaxIdleConns: 5,
				},
			},
			expected: true,
		},
		{
			name: "Custom options",
			config: config.Config{
				Database: struct {
					Path            string            `mapstructure:"path"`
					Provider        string            `mapstructure:"provider"`
					DSN             string            `mapstructure:"dsn"`
					MaxOpenConns    int               `mapstructure:"max_open_conns"`
					MaxIdleConns    int               `mapstructure:"max_idle_conns"`
					ConnMaxLifetime int               `mapstructure:"conn_max_lifetime"`
					ConnMaxIdleTime int               `mapstructure:"conn_max_idle_time"`
					SlowQueryTime   int               `mapstructure:"slow_query_time"`
					LogLevel        string            `mapstructure:"log_level"`
					Options         map[string]string `mapstructure:"options"`
				}{
					Options: map[string]string{
						"journal_mode": "WAL",
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldUseV2Manager(&tt.config)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetManagerInfo(t *testing.T) {
	// Test with V1 manager
	cfg := &config.Config{}
	cfg.Database.Path = ":memory:"

	manager, err := CreateManagerWithLogger(cfg, logging.GetDefault())
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Close()

	info := GetManagerInfo(manager)
	if info["type"] != "V1-Legacy" {
		t.Errorf("Expected V1-Legacy type, got %s", info["type"])
	}

	if info["provider"] != "SQLite" {
		t.Errorf("Expected SQLite provider, got %s", info["provider"])
	}
}
