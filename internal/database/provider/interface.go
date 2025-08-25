package provider

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// DatabaseProvider defines the interface for database providers
type DatabaseProvider interface {
	// Connection Management
	Connect(config DatabaseConfig) error
	Close() error
	Ping() error

	// Schema Management
	Migrate(models ...interface{}) error
	DropTables(models ...interface{}) error

	// Transaction Management
	BeginTransaction() (Transaction, error)

	// Database Access
	GetDB() *gorm.DB

	// Performance & Monitoring
	GetStats() DatabaseStats
	SetLogger(logger *logging.Logger)

	// Provider Info
	Name() string
	Version() string
}

// Transaction interface for database transactions
type Transaction interface {
	GetDB() *gorm.DB
	Commit() error
	Rollback() error
}

// DatabaseConfig holds configuration for database providers
type DatabaseConfig struct {
	Provider string            `mapstructure:"provider"` // "sqlite", "postgres", "mysql"
	DSN      string            `mapstructure:"dsn"`      // Data Source Name
	Options  map[string]string `mapstructure:"options"`  // Provider-specific options

	// Connection Pool Settings
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`

	// Performance Settings
	SlowQueryThreshold time.Duration `mapstructure:"slow_query_threshold"`
	LogLevel           string        `mapstructure:"log_level"` // "silent", "error", "warn", "info"
}

// DatabaseStats provides performance and connection statistics
type DatabaseStats struct {
	// Connection Statistics
	OpenConnections  int `json:"open_connections"`
	InUseConnections int `json:"in_use_connections"`
	IdleConnections  int `json:"idle_connections"`

	// Operation Statistics
	TotalQueries   int64         `json:"total_queries"`
	SlowQueries    int64         `json:"slow_queries"`
	FailedQueries  int64         `json:"failed_queries"`
	AverageLatency time.Duration `json:"average_latency"`

	// Resource Usage
	DatabaseSize int64      `json:"database_size"`
	LastBackup   *time.Time `json:"last_backup,omitempty"`

	// Provider Specific
	ProviderName    string                 `json:"provider_name"`
	ProviderVersion string                 `json:"provider_version"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// ProviderFactory creates database providers
type ProviderFactory interface {
	Create(providerType string) (DatabaseProvider, error)
	ListSupportedProviders() []string
}

// HealthChecker provides database health checking capabilities
type HealthChecker interface {
	HealthCheck(ctx context.Context) HealthStatus
}

// HealthStatus represents the health status of the database
type HealthStatus struct {
	Healthy      bool                   `json:"healthy"`
	ResponseTime time.Duration          `json:"response_time"`
	Error        string                 `json:"error,omitempty"`
	Details      map[string]interface{} `json:"details,omitempty"`
	CheckedAt    time.Time              `json:"checked_at"`
}

// BackupProvider defines backup capabilities for database providers
type BackupProvider interface {
	// Backup Operations
	CreateBackup(ctx context.Context, config BackupConfig) (*BackupResult, error)
	RestoreBackup(ctx context.Context, config RestoreConfig) (*RestoreResult, error)
	ValidateBackup(ctx context.Context, backupPath string) (*ValidationResult, error)

	// Backup Management
	ListBackups() ([]BackupInfo, error)
	DeleteBackup(backupID string) error
}

// BackupConfig defines backup operation configuration
type BackupConfig struct {
	BackupPath    string            `json:"backup_path"`
	BackupType    BackupType        `json:"backup_type"`
	Compression   bool              `json:"compression"`
	Encryption    bool              `json:"encryption"`
	IncludeTables []string          `json:"include_tables,omitempty"`
	ExcludeTables []string          `json:"exclude_tables,omitempty"`
	Options       map[string]string `json:"options,omitempty"`
}

// RestoreConfig defines restore operation configuration
type RestoreConfig struct {
	BackupPath      string            `json:"backup_path"`
	TargetDatabase  string            `json:"target_database,omitempty"`
	PreserveData    bool              `json:"preserve_data"`
	SelectiveTables []string          `json:"selective_tables,omitempty"`
	DryRun          bool              `json:"dry_run"`
	Options         map[string]string `json:"options,omitempty"`
}

// BackupType defines the type of backup
type BackupType string

const (
	BackupTypeFull         BackupType = "full"
	BackupTypeIncremental  BackupType = "incremental"
	BackupTypeDifferential BackupType = "differential"
)

// BackupResult contains the result of a backup operation
type BackupResult struct {
	Success     bool                   `json:"success"`
	BackupID    string                 `json:"backup_id"`
	BackupPath  string                 `json:"backup_path"`
	BackupType  BackupType             `json:"backup_type"`
	StartTime   time.Time              `json:"start_time"`
	EndTime     time.Time              `json:"end_time"`
	Duration    time.Duration          `json:"duration"`
	Size        int64                  `json:"size"`
	RecordCount int64                  `json:"record_count"`
	TableCount  int                    `json:"table_count"`
	Checksum    string                 `json:"checksum,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Warnings    []string               `json:"warnings,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// RestoreResult contains the result of a restore operation
type RestoreResult struct {
	Success         bool                   `json:"success"`
	RestoreID       string                 `json:"restore_id"`
	BackupPath      string                 `json:"backup_path"`
	StartTime       time.Time              `json:"start_time"`
	EndTime         time.Time              `json:"end_time"`
	Duration        time.Duration          `json:"duration"`
	TablesRestored  []string               `json:"tables_restored"`
	RecordsRestored int64                  `json:"records_restored"`
	Error           string                 `json:"error,omitempty"`
	Warnings        []string               `json:"warnings,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// ValidationResult contains backup validation results
type ValidationResult struct {
	Valid         bool       `json:"valid"`
	BackupID      string     `json:"backup_id"`
	BackupType    BackupType `json:"backup_type"`
	Size          int64      `json:"size"`
	RecordCount   int64      `json:"record_count"`
	ChecksumValid bool       `json:"checksum_valid"`
	Errors        []string   `json:"errors,omitempty"`
	Warnings      []string   `json:"warnings,omitempty"`
}

// BackupInfo contains information about a backup
type BackupInfo struct {
	BackupID    string                 `json:"backup_id"`
	BackupPath  string                 `json:"backup_path"`
	BackupType  BackupType             `json:"backup_type"`
	CreatedAt   time.Time              `json:"created_at"`
	Size        int64                  `json:"size"`
	RecordCount int64                  `json:"record_count"`
	TableCount  int                    `json:"table_count"`
	Checksum    string                 `json:"checksum"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}
