package provider

import (
	"context"
	"os"
	"testing"

	"github.com/ginsys/shelly-manager/internal/logging"
)

func TestFactoryCreate(t *testing.T) {
	logger := logging.GetDefault()
	factory := NewFactory(logger)

	tests := []struct {
		name         string
		providerType string
		expectError  bool
	}{
		{"SQLite", "sqlite", false},
		{"SQLite3", "sqlite3", false},
		{"PostgreSQL", "postgresql", false},
		{"PostgreSQL Alt", "postgres", false},
		{"MySQL", "mysql", false},
		{"Invalid", "invalid", true},
		{"Empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := factory.Create(tt.providerType)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for provider type '%s', but got none", tt.providerType)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error creating provider '%s': %v", tt.providerType, err)
				return
			}

			if provider == nil {
				t.Errorf("Provider is nil for type '%s'", tt.providerType)
			}
		})
	}
}

func TestFactoryListSupportedProviders(t *testing.T) {
	logger := logging.GetDefault()
	factory := NewFactory(logger)

	providers := factory.ListSupportedProviders()
	expectedProviders := []string{"sqlite", "postgresql", "mysql"}

	if len(providers) != len(expectedProviders) {
		t.Errorf("Expected %d providers, got %d", len(expectedProviders), len(providers))
	}

	for _, expected := range expectedProviders {
		found := false
		for _, provider := range providers {
			if provider == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected provider '%s' not found in list", expected)
		}
	}
}

func TestSQLiteProvider(t *testing.T) {
	logger := logging.GetDefault()
	provider := NewSQLiteProvider(logger)

	// Test provider info
	if provider.Name() != "SQLite" {
		t.Errorf("Expected provider name 'SQLite', got '%s'", provider.Name())
	}

	// Test connection to in-memory database
	config := DatabaseConfig{
		Provider:     "sqlite",
		DSN:          ":memory:",
		MaxOpenConns: 1,
		MaxIdleConns: 1,
		LogLevel:     "silent",
	}

	err := provider.Connect(config)
	if err != nil {
		t.Fatalf("Failed to connect to SQLite: %v", err)
	}
	defer provider.Close()

	// Test ping
	if err := provider.Ping(); err != nil {
		t.Errorf("Ping failed: %v", err)
	}

	// Test migration with a simple model
	type TestModel struct {
		ID   uint   `gorm:"primaryKey"`
		Name string `gorm:"not null"`
	}

	if err := provider.Migrate(&TestModel{}); err != nil {
		t.Errorf("Migration failed: %v", err)
	}

	// Test stats
	stats := provider.GetStats()
	if stats.ProviderName != "SQLite" {
		t.Errorf("Expected provider name 'SQLite' in stats, got '%s'", stats.ProviderName)
	}

	// Test health check
	health := provider.HealthCheck(context.Background())
	if !health.Healthy {
		t.Errorf("Health check failed: %s", health.Error)
	}
}

func TestSQLiteProviderFileDatabase(t *testing.T) {
	// Create temporary database file
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	logger := logging.GetDefault()
	provider := NewSQLiteProvider(logger)

	config := DatabaseConfig{
		Provider:     "sqlite",
		DSN:          dbPath,
		MaxOpenConns: 1,
		MaxIdleConns: 1,
		LogLevel:     "silent",
		Options: map[string]string{
			"foreign_keys": "true",
			"journal_mode": "WAL",
		},
	}

	err := provider.Connect(config)
	if err != nil {
		t.Fatalf("Failed to connect to SQLite: %v", err)
	}
	defer provider.Close()

	// Verify database file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Errorf("Database file was not created")
	}

	// Test transaction
	tx, err := provider.BeginTransaction()
	if err != nil {
		t.Errorf("Failed to begin transaction: %v", err)
	} else {
		if err := tx.Rollback(); err != nil {
			t.Errorf("Failed to rollback transaction: %v", err)
		}
	}
}

func TestDatabaseConfig(t *testing.T) {
	tests := []struct {
		name   string
		config DatabaseConfig
		valid  bool
	}{
		{
			name: "Valid SQLite config",
			config: DatabaseConfig{
				Provider:     "sqlite",
				DSN:          ":memory:",
				MaxOpenConns: 1,
				MaxIdleConns: 1,
			},
			valid: true,
		},
		{
			name: "Invalid provider",
			config: DatabaseConfig{
				Provider: "invalid",
				DSN:      "test",
			},
			valid: false,
		},
		{
			name: "Empty DSN",
			config: DatabaseConfig{
				Provider: "sqlite",
				DSN:      "",
			},
			valid: false,
		},
	}

	factory := NewFactory(logging.GetDefault())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := factory.ValidateConfig(tt.config.Provider, tt.config)

			if tt.valid && err != nil {
				t.Errorf("Expected valid config, but got error: %v", err)
			}

			if !tt.valid && err == nil {
				t.Errorf("Expected invalid config, but got no error")
			}
		})
	}
}

func TestProviderInfo(t *testing.T) {
	factory := NewFactory(logging.GetDefault())

	tests := []string{"sqlite", "postgresql", "mysql"}

	for _, providerType := range tests {
		t.Run(providerType, func(t *testing.T) {
			info, err := factory.GetProviderInfo(providerType)
			if err != nil {
				t.Errorf("Failed to get provider info for '%s': %v", providerType, err)
				return
			}

			if info.Name == "" {
				t.Errorf("Provider info has empty name")
			}

			if info.Type != providerType {
				t.Errorf("Expected type '%s', got '%s'", providerType, info.Type)
			}

			if len(info.Features) == 0 {
				t.Errorf("Provider info has no features listed")
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	factory := NewFactory(logging.GetDefault())

	tests := []string{"sqlite", "postgresql", "mysql"}

	for _, providerType := range tests {
		t.Run(providerType, func(t *testing.T) {
			config := factory.GetDefaultConfig(providerType)

			if config.Provider != providerType {
				t.Errorf("Expected provider '%s', got '%s'", providerType, config.Provider)
			}

			if config.DSN == "" {
				t.Errorf("Default config has empty DSN")
			}

			if config.SlowQueryThreshold == 0 {
				t.Errorf("Default config has zero slow query threshold")
			}

			if config.MaxOpenConns <= 0 {
				t.Errorf("Default config has invalid MaxOpenConns: %d", config.MaxOpenConns)
			}
		})
	}
}
