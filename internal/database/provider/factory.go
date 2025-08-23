package provider

import (
	"fmt"
	"strings"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// Factory implements the ProviderFactory interface
type Factory struct {
	logger *logging.Logger
}

// NewFactory creates a new provider factory
func NewFactory(logger *logging.Logger) *Factory {
	if logger == nil {
		logger = logging.GetDefault()
	}

	return &Factory{
		logger: logger,
	}
}

// Create creates a new database provider of the specified type
func (f *Factory) Create(providerType string) (DatabaseProvider, error) {
	providerType = strings.ToLower(strings.TrimSpace(providerType))

	switch providerType {
	case "sqlite", "sqlite3":
		return NewSQLiteProvider(f.logger), nil
	case "postgres", "postgresql":
		return NewPostgreSQLProvider(f.logger), nil
	case "mysql":
		return NewMySQLProvider(f.logger), nil
	default:
		return nil, fmt.Errorf("unsupported database provider: %s", providerType)
	}
}

// ListSupportedProviders returns a list of supported database providers
func (f *Factory) ListSupportedProviders() []string {
	return []string{
		"sqlite",
		"postgresql",
		"mysql",
	}
}

// GetProviderInfo returns information about a specific provider
func (f *Factory) GetProviderInfo(providerType string) (*ProviderInfo, error) {
	providerType = strings.ToLower(strings.TrimSpace(providerType))

	info := &ProviderInfo{}

	switch providerType {
	case "sqlite", "sqlite3":
		info.Name = "SQLite"
		info.Type = "sqlite"
		info.Description = "Embedded SQL database engine"
		info.Features = []string{
			"embedded",
			"serverless",
			"zero-configuration",
			"cross-platform",
			"full-text-search",
		}
		info.Limitations = []string{
			"concurrent-writes-limited",
			"no-network-access",
			"limited-concurrency",
		}
		info.RecommendedFor = []string{
			"development",
			"small-to-medium-datasets",
			"single-server-deployments",
		}

	case "postgres", "postgresql":
		info.Name = "PostgreSQL"
		info.Type = "postgresql"
		info.Description = "Advanced open-source relational database"
		info.Features = []string{
			"acid-compliant",
			"extensible",
			"standards-compliant",
			"high-concurrency",
			"advanced-indexing",
			"full-text-search",
			"json-support",
		}
		info.Limitations = []string{
			"requires-server-setup",
			"higher-resource-usage",
		}
		info.RecommendedFor = []string{
			"production",
			"large-datasets",
			"multi-server-deployments",
			"high-concurrency",
		}

	case "mysql":
		info.Name = "MySQL"
		info.Type = "mysql"
		info.Description = "Popular open-source relational database"
		info.Features = []string{
			"mature",
			"high-performance",
			"replication-support",
			"clustering",
			"partitioning",
		}
		info.Limitations = []string{
			"requires-server-setup",
			"less-feature-rich-than-postgresql",
		}
		info.RecommendedFor = []string{
			"production",
			"web-applications",
			"read-heavy-workloads",
		}

	default:
		return nil, fmt.Errorf("unknown provider type: %s", providerType)
	}

	return info, nil
}

// ValidateConfig validates a database configuration for a specific provider
func (f *Factory) ValidateConfig(providerType string, config DatabaseConfig) error {
	providerType = strings.ToLower(strings.TrimSpace(providerType))

	// Common validation
	if config.DSN == "" {
		return fmt.Errorf("DSN is required")
	}

	// Provider-specific validation
	switch providerType {
	case "sqlite", "sqlite3":
		return f.validateSQLiteConfig(config)
	case "postgres", "postgresql":
		return f.validatePostgreSQLConfig(config)
	case "mysql":
		return f.validateMySQLConfig(config)
	default:
		return fmt.Errorf("unsupported provider type: %s", providerType)
	}
}

// validateSQLiteConfig validates SQLite-specific configuration
func (f *Factory) validateSQLiteConfig(config DatabaseConfig) error {
	if config.MaxOpenConns > 1 {
		f.logger.Warn("SQLite supports only 1 concurrent write connection, adjusting MaxOpenConns")
	}

	return nil
}

// validatePostgreSQLConfig validates PostgreSQL-specific configuration
func (f *Factory) validatePostgreSQLConfig(config DatabaseConfig) error {
	if config.MaxOpenConns == 0 {
		return fmt.Errorf("MaxOpenConns must be set for PostgreSQL")
	}

	if config.MaxIdleConns == 0 {
		return fmt.Errorf("MaxIdleConns must be set for PostgreSQL")
	}

	return nil
}

// validateMySQLConfig validates MySQL-specific configuration
func (f *Factory) validateMySQLConfig(config DatabaseConfig) error {
	if config.MaxOpenConns == 0 {
		return fmt.Errorf("MaxOpenConns must be set for MySQL")
	}

	if config.MaxIdleConns == 0 {
		return fmt.Errorf("MaxIdleConns must be set for MySQL")
	}

	return nil
}

// ProviderInfo contains information about a database provider
type ProviderInfo struct {
	Name           string   `json:"name"`
	Type           string   `json:"type"`
	Description    string   `json:"description"`
	Features       []string `json:"features"`
	Limitations    []string `json:"limitations"`
	RecommendedFor []string `json:"recommended_for"`
}

// GetDefaultConfig returns default configuration for a provider type
func (f *Factory) GetDefaultConfig(providerType string) DatabaseConfig {
	providerType = strings.ToLower(strings.TrimSpace(providerType))

	config := DatabaseConfig{
		Provider:           providerType,
		SlowQueryThreshold: 500 * 1000000, // 500ms in nanoseconds
		LogLevel:           "warn",
	}

	switch providerType {
	case "sqlite", "sqlite3":
		config.DSN = "shelly-manager.db"
		config.MaxOpenConns = 1
		config.MaxIdleConns = 1
		config.Options = map[string]string{
			"foreign_keys": "true",
			"journal_mode": "WAL",
			"synchronous":  "NORMAL",
			"cache_size":   "-64000", // 64MB
			"busy_timeout": "5000",   // 5 seconds
		}

	case "postgres", "postgresql":
		config.DSN = "postgres://username:password@localhost:5432/shelly_manager?sslmode=disable"
		config.MaxOpenConns = 25
		config.MaxIdleConns = 5
		config.ConnMaxLifetime = 5 * 60 * 1000000000  // 5 minutes in nanoseconds
		config.ConnMaxIdleTime = 10 * 60 * 1000000000 // 10 minutes in nanoseconds
		config.Options = map[string]string{
			"timezone":              "UTC",
			"default_query_timeout": "30s",
			"statement_timeout":     "60s",
			"lock_timeout":          "30s",
		}

	case "mysql":
		config.DSN = "username:password@tcp(localhost:3306)/shelly_manager?charset=utf8mb4&parseTime=True&loc=Local"
		config.MaxOpenConns = 25
		config.MaxIdleConns = 5
		config.ConnMaxLifetime = 5 * 60 * 1000000000  // 5 minutes in nanoseconds
		config.ConnMaxIdleTime = 10 * 60 * 1000000000 // 10 minutes in nanoseconds
		config.Options = map[string]string{
			"charset":   "utf8mb4",
			"collation": "utf8mb4_unicode_ci",
			"timeout":   "30s",
		}
	}

	return config
}
