package config

import (
	"fmt"
	"strconv"
	"time"

	"github.com/ginsys/shelly-manager/internal/database/provider"
)

// GetDatabaseConfig converts the config struct to a provider.DatabaseConfig
// This provides backward compatibility and automatic provider detection
func (c *Config) GetDatabaseConfig() provider.DatabaseConfig {
	config := provider.DatabaseConfig{
		Provider: c.Database.Provider,
		DSN:      c.Database.DSN,
		Options:  c.Database.Options,
	}

	// Handle backward compatibility - if no provider specified, use legacy path
	if config.Provider == "" {
		config.Provider = "sqlite"
	}

	if config.DSN == "" {
		if c.Database.Path != "" {
			config.DSN = c.Database.Path
		} else {
			config.DSN = "data/shelly.db"
		}
	}

	// Convert connection settings
	config.MaxOpenConns = c.Database.MaxOpenConns
	config.MaxIdleConns = c.Database.MaxIdleConns
	config.ConnMaxLifetime = time.Duration(c.Database.ConnMaxLifetime) * time.Second
	config.ConnMaxIdleTime = time.Duration(c.Database.ConnMaxIdleTime) * time.Second
	config.SlowQueryThreshold = time.Duration(c.Database.SlowQueryTime) * time.Millisecond
	config.LogLevel = c.Database.LogLevel

	// Ensure we have sensible defaults based on provider
	if config.MaxOpenConns == 0 {
		switch config.Provider {
		case "sqlite":
			config.MaxOpenConns = 1
		case "postgresql", "mysql":
			config.MaxOpenConns = 25
		default:
			config.MaxOpenConns = 10
		}
	}

	if config.MaxIdleConns == 0 {
		switch config.Provider {
		case "sqlite":
			config.MaxIdleConns = 1
		case "postgresql", "mysql":
			config.MaxIdleConns = 5
		default:
			config.MaxIdleConns = 2
		}
	}

	if config.ConnMaxLifetime == 0 {
		config.ConnMaxLifetime = 5 * time.Minute
	}

	if config.ConnMaxIdleTime == 0 {
		config.ConnMaxIdleTime = 10 * time.Minute
	}

	if config.SlowQueryThreshold == 0 {
		config.SlowQueryThreshold = 500 * time.Millisecond
	}

	if config.LogLevel == "" {
		config.LogLevel = "warn"
	}

	// Initialize options if nil
	if config.Options == nil {
		config.Options = make(map[string]string)
	}

	return config
}

// ValidateDatabaseConfig validates the database configuration
func (c *Config) ValidateDatabaseConfig() error {
	dbConfig := c.GetDatabaseConfig()

	// Basic validation
	if dbConfig.Provider == "" {
		return fmt.Errorf("database provider is required")
	}

	if dbConfig.DSN == "" {
		return fmt.Errorf("database DSN is required")
	}

	// Provider-specific validation
	switch dbConfig.Provider {
	case "sqlite":
		return c.validateSQLiteConfig(dbConfig)
	case "postgresql", "postgres":
		return c.validatePostgreSQLConfig(dbConfig)
	case "mysql":
		return c.validateMySQLConfig(dbConfig)
	default:
		return fmt.Errorf("unsupported database provider: %s", dbConfig.Provider)
	}
}

// validateSQLiteConfig validates SQLite-specific configuration
func (c *Config) validateSQLiteConfig(config provider.DatabaseConfig) error {
	if config.MaxOpenConns > 1 {
		return fmt.Errorf("SQLite only supports 1 concurrent write connection, but %d configured", config.MaxOpenConns)
	}

	// Validate SQLite-specific options
	for key, value := range config.Options {
		switch key {
		case "journal_mode":
			validModes := []string{"DELETE", "TRUNCATE", "PERSIST", "MEMORY", "WAL", "OFF"}
			valid := false
			for _, mode := range validModes {
				if value == mode {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("invalid journal_mode '%s', must be one of: %v", value, validModes)
			}

		case "synchronous":
			validModes := []string{"OFF", "NORMAL", "FULL", "EXTRA"}
			valid := false
			for _, mode := range validModes {
				if value == mode {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("invalid synchronous '%s', must be one of: %v", value, validModes)
			}

		case "foreign_keys":
			if value != "true" && value != "false" && value != "ON" && value != "OFF" {
				return fmt.Errorf("foreign_keys must be 'true', 'false', 'ON', or 'OFF'")
			}

		case "cache_size":
			if _, err := strconv.Atoi(value); err != nil {
				return fmt.Errorf("cache_size must be an integer: %s", value)
			}

		case "busy_timeout":
			if timeout, err := strconv.Atoi(value); err != nil || timeout < 0 {
				return fmt.Errorf("busy_timeout must be a non-negative integer: %s", value)
			}
		}
	}

	return nil
}

// validatePostgreSQLConfig validates PostgreSQL-specific configuration
func (c *Config) validatePostgreSQLConfig(config provider.DatabaseConfig) error {
	if config.MaxOpenConns <= 0 {
		return fmt.Errorf("max_open_conns must be positive for PostgreSQL")
	}

	if config.MaxIdleConns <= 0 {
		return fmt.Errorf("max_idle_conns must be positive for PostgreSQL")
	}

	if config.MaxOpenConns < config.MaxIdleConns {
		return fmt.Errorf("max_open_conns (%d) must be >= max_idle_conns (%d)", config.MaxOpenConns, config.MaxIdleConns)
	}

	return nil
}

// validateMySQLConfig validates MySQL-specific configuration
func (c *Config) validateMySQLConfig(config provider.DatabaseConfig) error {
	if config.MaxOpenConns <= 0 {
		return fmt.Errorf("max_open_conns must be positive for MySQL")
	}

	if config.MaxIdleConns <= 0 {
		return fmt.Errorf("max_idle_conns must be positive for MySQL")
	}

	if config.MaxOpenConns < config.MaxIdleConns {
		return fmt.Errorf("max_open_conns (%d) must be >= max_idle_conns (%d)", config.MaxOpenConns, config.MaxIdleConns)
	}

	return nil
}

// GetDatabaseProviderRecommendation provides provider recommendation based on use case
func GetDatabaseProviderRecommendation(deviceCount int, concurrent bool, production bool) string {
	if !production && deviceCount < 100 {
		return "sqlite" // Development and small deployments
	}

	if deviceCount > 1000 || concurrent {
		return "postgresql" // Large deployments and high concurrency
	}

	if production && deviceCount < 1000 {
		return "mysql" // Medium production deployments
	}

	return "sqlite" // Default fallback
}

// GetMigrationPath provides guidance for migrating between database providers
func GetMigrationPath(from, to string) ([]string, error) {
	if from == to {
		return nil, fmt.Errorf("source and target providers are the same")
	}

	switch from {
	case "sqlite":
		switch to {
		case "postgresql":
			return []string{
				"1. Export data using backup system",
				"2. Install and configure PostgreSQL",
				"3. Update configuration to use PostgreSQL provider",
				"4. Restore data from backup",
				"5. Verify data integrity",
				"6. Update connection pool settings for production load",
			}, nil
		case "mysql":
			return []string{
				"1. Export data using backup system",
				"2. Install and configure MySQL",
				"3. Update configuration to use MySQL provider",
				"4. Restore data from backup",
				"5. Verify data integrity",
				"6. Configure MySQL-specific optimizations",
			}, nil
		}
	case "postgresql":
		switch to {
		case "sqlite":
			return []string{
				"1. Export data using backup system",
				"2. Update configuration to use SQLite provider",
				"3. Restore data from backup (note: concurrency will be limited)",
				"4. Verify data integrity",
			}, nil
		case "mysql":
			return []string{
				"1. Export data using backup system",
				"2. Install and configure MySQL",
				"3. Update configuration to use MySQL provider",
				"4. Restore data from backup",
				"5. Verify data integrity",
			}, nil
		}
	case "mysql":
		switch to {
		case "sqlite":
			return []string{
				"1. Export data using backup system",
				"2. Update configuration to use SQLite provider",
				"3. Restore data from backup (note: concurrency will be limited)",
				"4. Verify data integrity",
			}, nil
		case "postgresql":
			return []string{
				"1. Export data using backup system",
				"2. Install and configure PostgreSQL",
				"3. Update configuration to use PostgreSQL provider",
				"4. Restore data from backup",
				"5. Verify data integrity",
			}, nil
		}
	}

	return nil, fmt.Errorf("unsupported migration path from %s to %s", from, to)
}
