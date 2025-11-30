# PostgreSQL Provider API Reference

## Overview

This document provides complete API reference for the PostgreSQL database provider, including all methods, interfaces, configuration options, and usage examples.

## Core Interfaces

### DatabaseProvider Interface

The main interface implemented by the PostgreSQL provider for database operations.

```go
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
```

### HealthChecker Interface

Provides database health monitoring capabilities.

```go
type HealthChecker interface {
    HealthCheck(ctx context.Context) HealthStatus
}
```

### Transaction Interface

Provides transaction management capabilities.

```go
type Transaction interface {
    GetDB() *gorm.DB
    Commit() error
    Rollback() error
}
```

## PostgreSQL Provider Methods

### Constructor

#### NewPostgreSQLProvider

Creates a new PostgreSQL database provider instance.

```go
func NewPostgreSQLProvider(logger *logging.Logger) *PostgreSQLProvider
```

**Parameters:**
- `logger` (*logging.Logger): Logger instance for database operations. If nil, uses default logger.

**Returns:**
- `*PostgreSQLProvider`: New PostgreSQL provider instance

**Example:**
```go
logger := logging.GetDefault()
provider := provider.NewPostgreSQLProvider(logger)
```

### Connection Management

#### Connect

Establishes connection to PostgreSQL database with comprehensive SSL/TLS support and connection pooling.

```go
func (p *PostgreSQLProvider) Connect(config DatabaseConfig) error
```

**Parameters:**
- `config` (DatabaseConfig): Database configuration including DSN, connection pool settings, and options

**Returns:**
- `error`: nil on success, error on failure

**Behavior:**
1. Validates and builds DSN with SSL defaults
2. Configures GORM logger based on settings
3. Establishes database connection
4. Configures connection pool parameters
5. Tests connection with Ping()
6. Retrieves PostgreSQL version information

**Example:**
```go
config := provider.DatabaseConfig{
    Provider: "postgresql",
    DSN: "postgres://user:pass@localhost:5432/db?sslmode=require",
    Options: map[string]string{
        "sslmode": "require",
        "connect_timeout": "30",
        "application_name": "shelly-manager",
    },
    MaxOpenConns: 25,
    MaxIdleConns: 5,
    ConnMaxLifetime: time.Hour,
    ConnMaxIdleTime: 10 * time.Minute,
    SlowQueryThreshold: 200 * time.Millisecond,
    LogLevel: "warn",
}

if err := provider.Connect(config); err != nil {
    log.Fatal("Connection failed:", err)
}
```

#### Close

Closes the database connection and cleans up resources.

```go
func (p *PostgreSQLProvider) Close() error
```

**Returns:**
- `error`: nil on success, error on failure

**Behavior:**
1. Acquires connection lock
2. Checks if connection exists
3. Closes underlying SQL database connection
4. Cleans up provider state

**Example:**
```go
defer func() {
    if err := provider.Close(); err != nil {
        log.Printf("Failed to close database: %v", err)
    }
}()
```

#### Ping

Tests database connection availability with timeout.

```go
func (p *PostgreSQLProvider) Ping() error
```

**Returns:**
- `error`: nil if connection is alive, error if connection failed

**Behavior:**
1. Verifies provider is connected
2. Gets underlying SQL database instance
3. Executes ping with 5-second timeout

**Example:**
```go
if err := provider.Ping(); err != nil {
    log.Printf("Database connection lost: %v", err)
    // Implement reconnection logic
}
```

### Schema Management

#### Migrate

Performs database schema migration using GORM AutoMigrate.

```go
func (p *PostgreSQLProvider) Migrate(models ...interface{}) error
```

**Parameters:**
- `models` (...interface{}): GORM model structs to migrate

**Returns:**
- `error`: nil on success, error on migration failure

**Behavior:**
1. Verifies database connection
2. Measures migration duration
3. Executes GORM AutoMigrate
4. Logs migration results
5. Updates query statistics

**Example:**
```go
type Device struct {
    ID   uint   `gorm:"primarykey"`
    Name string `gorm:"not null"`
    IP   string `gorm:"unique;not null"`
}

type DeviceConfiguration struct {
    ID       uint   `gorm:"primarykey"`
    DeviceID uint   `gorm:"not null"`
    Config   string `gorm:"type:jsonb"`
    Device   Device `gorm:"foreignKey:DeviceID"`
}

if err := provider.Migrate(&Device{}, &DeviceConfiguration{}); err != nil {
    log.Fatal("Migration failed:", err)
}
```

#### DropTables

Drops specified tables from the database.

```go
func (p *PostgreSQLProvider) DropTables(models ...interface{}) error
```

**Parameters:**
- `models` (...interface{}): GORM model structs representing tables to drop

**Returns:**
- `error`: nil on success, error on failure

**Example:**
```go
// Drop tables (usually for testing)
if err := provider.DropTables(&Device{}, &DeviceConfiguration{}); err != nil {
    log.Fatal("Failed to drop tables:", err)
}
```

### Transaction Management

#### BeginTransaction

Starts a new database transaction with PostgreSQL default isolation level.

```go
func (p *PostgreSQLProvider) BeginTransaction() (Transaction, error)
```

**Returns:**
- `Transaction`: Transaction interface for commit/rollback operations
- `error`: nil on success, error on failure

**Behavior:**
1. Verifies database connection
2. Begins transaction with Read Committed isolation level (PostgreSQL default)
3. Returns transaction wrapper

**Example:**
```go
tx, err := provider.BeginTransaction()
if err != nil {
    log.Fatal("Failed to begin transaction:", err)
}

// Use transaction
device := &Device{Name: "New Device", IP: "192.168.1.100"}
if err := tx.GetDB().Create(device).Error; err != nil {
    tx.Rollback()
    log.Fatal("Create failed:", err)
}

if err := tx.Commit(); err != nil {
    log.Fatal("Commit failed:", err)
}
```

### Database Access

#### GetDB

Returns the underlying GORM database instance for direct query operations.

```go
func (p *PostgreSQLProvider) GetDB() *gorm.DB
```

**Returns:**
- `*gorm.DB`: GORM database instance

**Example:**
```go
db := provider.GetDB()

// Direct GORM operations
var devices []Device
db.Where("status = ?", "active").Find(&devices)

// Complex queries
db.Raw("SELECT * FROM devices WHERE last_seen > ?", 
    time.Now().Add(-24*time.Hour)).Find(&devices)

// Batch operations
db.CreateInBatches(devices, 100)
```

### Monitoring and Statistics

#### GetStats

Returns comprehensive database performance and connection statistics.

```go
func (p *PostgreSQLProvider) GetStats() DatabaseStats
```

**Returns:**
- `DatabaseStats`: Comprehensive statistics structure

**Statistics Included:**
- Connection pool utilization
- Query performance metrics
- Database size information
- Provider version details

**Example:**
```go
stats := provider.GetStats()

fmt.Printf("Provider: %s %s\n", stats.ProviderName, stats.ProviderVersion)
fmt.Printf("Connections: %d/%d (in use/open)\n", 
    stats.InUseConnections, stats.OpenConnections)
fmt.Printf("Total Queries: %d (Slow: %d, Failed: %d)\n", 
    stats.TotalQueries, stats.SlowQueries, stats.FailedQueries)
fmt.Printf("Average Latency: %v\n", stats.AverageLatency)
fmt.Printf("Database Size: %s\n", 
    humanizeBytes(stats.DatabaseSize))
```

#### HealthCheck

Performs comprehensive health check with detailed status information.

```go
func (p *PostgreSQLProvider) HealthCheck(ctx context.Context) HealthStatus
```

**Parameters:**
- `ctx` (context.Context): Context for timeout and cancellation

**Returns:**
- `HealthStatus`: Detailed health status with metrics

**Health Check Components:**
- Connection availability (ping test)
- Response time measurement
- Connection pool status
- Database size metrics
- Version information

**Example:**
```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

status := provider.HealthCheck(ctx)

if status.Healthy {
    fmt.Printf("Database healthy - Response time: %v\n", status.ResponseTime)
    fmt.Printf("Details: %+v\n", status.Details)
} else {
    log.Printf("Database unhealthy: %s", status.Error)
}
```

### Provider Information

#### Name

Returns the provider name identifier.

```go
func (p *PostgreSQLProvider) Name() string
```

**Returns:**
- `string`: Always returns "PostgreSQL"

#### Version

Returns the connected PostgreSQL server version.

```go
func (p *PostgreSQLProvider) Version() string
```

**Returns:**
- `string`: PostgreSQL version string (e.g., "15.3")

#### SetLogger

Updates the logger instance used by the provider.

```go
func (p *PostgreSQLProvider) SetLogger(logger *logging.Logger)
```

**Parameters:**
- `logger` (*logging.Logger): New logger instance

**Example:**
```go
newLogger := logging.NewLogger(logging.Config{
    Level: "debug",
    Format: "json",
})
provider.SetLogger(newLogger)
```

## Configuration Structures

### DatabaseConfig

Complete configuration structure for database providers.

```go
type DatabaseConfig struct {
    Provider string            `mapstructure:"provider"` // "postgresql"
    DSN      string            `mapstructure:"dsn"`      // Connection string
    Options  map[string]string `mapstructure:"options"`  // Provider-specific options

    // Connection Pool Settings
    MaxOpenConns    int           `mapstructure:"max_open_conns"`
    MaxIdleConns    int           `mapstructure:"max_idle_conns"`
    ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
    ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`

    // Performance Settings
    SlowQueryThreshold time.Duration `mapstructure:"slow_query_threshold"`
    LogLevel           string        `mapstructure:"log_level"`
}
```

**Default Values:**
- `MaxOpenConns`: 25 (PostgreSQL optimized)
- `MaxIdleConns`: 5
- `ConnMaxLifetime`: 1 hour
- `ConnMaxIdleTime`: 10 minutes
- `SlowQueryThreshold`: 200ms
- `LogLevel`: "warn"

### PostgreSQL Options

Supported options in the `Options` map:

| Option | Description | Default | Example |
|--------|-------------|---------|---------|
| `sslmode` | SSL connection mode | "require" | "require", "verify-full" |
| `sslcert` | Client certificate path | - | "/certs/client.crt" |
| `sslkey` | Client private key path | - | "/certs/client.key" |
| `sslrootcert` | Root CA certificate path | - | "/certs/ca.crt" |
| `connect_timeout` | Connection timeout (seconds) | "10" | "30" |
| `statement_timeout` | Statement timeout (milliseconds) | - | "300000" |
| `application_name` | Application identifier | - | "shelly-manager" |
| `search_path` | Schema search path | - | "public,shelly" |

## Statistics and Monitoring

### DatabaseStats Structure

Comprehensive statistics structure returned by GetStats().

```go
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
```

### HealthStatus Structure

Health check result structure.

```go
type HealthStatus struct {
    Healthy      bool                   `json:"healthy"`
    ResponseTime time.Duration          `json:"response_time"`
    Error        string                 `json:"error,omitempty"`
    Details      map[string]interface{} `json:"details,omitempty"`
    CheckedAt    time.Time              `json:"checked_at"`
}
```

**Health Details Include:**
- `database_size`: Current database size in bytes
- `total_queries`: Total number of executed queries
- `connection_count`: Current number of open connections
- `version`: PostgreSQL server version

## Error Handling

### Connection Errors

Common connection-related errors and their meanings:

```go
// Connection refused - PostgreSQL not running or network issue
err := provider.Connect(config)
if err != nil && strings.Contains(err.Error(), "connection refused") {
    // Handle connection failure
}

// Authentication failed - invalid credentials
if strings.Contains(err.Error(), "password authentication failed") {
    // Handle authentication failure
}

// SSL errors - certificate or configuration issues
if strings.Contains(err.Error(), "SSL") {
    // Handle SSL configuration issues
}
```

### Query Errors

GORM query errors through the provider:

```go
db := provider.GetDB()

var device Device
result := db.First(&device, "ip = ?", "192.168.1.100")

if errors.Is(result.Error, gorm.ErrRecordNotFound) {
    // Handle record not found
} else if result.Error != nil {
    // Handle other database errors
}
```

### Transaction Errors

Transaction-related error handling:

```go
tx, err := provider.BeginTransaction()
if err != nil {
    // Handle transaction creation failure
}

// Perform operations
if err := tx.GetDB().Create(&device).Error; err != nil {
    tx.Rollback()  // Always rollback on error
    return err
}

if err := tx.Commit(); err != nil {
    // Handle commit failure - transaction already rolled back
    return err
}
```

## Usage Examples

### Basic Connection and Query

```go
package main

import (
    "log"
    "time"
    
    "github.com/ginsys/shelly-manager/internal/database/provider"
    "github.com/ginsys/shelly-manager/internal/logging"
)

func main() {
    // Create provider
    logger := logging.GetDefault()
    pgProvider := provider.NewPostgreSQLProvider(logger)
    
    // Configure connection
    config := provider.DatabaseConfig{
        Provider: "postgresql",
        DSN: "postgres://user:pass@localhost:5432/db?sslmode=require",
        MaxOpenConns: 25,
        MaxIdleConns: 5,
        ConnMaxLifetime: time.Hour,
        SlowQueryThreshold: 200 * time.Millisecond,
        LogLevel: "warn",
    }
    
    // Connect
    if err := pgProvider.Connect(config); err != nil {
        log.Fatal("Connection failed:", err)
    }
    defer pgProvider.Close()
    
    // Use database
    db := pgProvider.GetDB()
    
    var devices []Device
    db.Where("status = ?", "active").Find(&devices)
    
    log.Printf("Found %d active devices", len(devices))
}
```

### Transaction Example

```go
func TransferDeviceConfiguration(provider *provider.PostgreSQLProvider, 
    fromDeviceID, toDeviceID uint) error {
    
    tx, err := provider.BeginTransaction()
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }
    
    // Get configuration from source device
    var config DeviceConfiguration
    if err := tx.GetDB().Where("device_id = ?", fromDeviceID).First(&config).Error; err != nil {
        tx.Rollback()
        return fmt.Errorf("failed to find source config: %w", err)
    }
    
    // Create new configuration for target device
    newConfig := DeviceConfiguration{
        DeviceID: toDeviceID,
        Config:   config.Config,
        Version:  config.Version,
    }
    
    if err := tx.GetDB().Create(&newConfig).Error; err != nil {
        tx.Rollback()
        return fmt.Errorf("failed to create new config: %w", err)
    }
    
    // Delete old configuration
    if err := tx.GetDB().Delete(&config).Error; err != nil {
        tx.Rollback()
        return fmt.Errorf("failed to delete old config: %w", err)
    }
    
    return tx.Commit()
}
```

### Health Monitoring Example

```go
func MonitorDatabaseHealth(provider *provider.PostgreSQLProvider) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
            
            status := provider.HealthCheck(ctx)
            stats := provider.GetStats()
            
            if status.Healthy {
                log.Printf("DB Health: OK - Response: %v, Connections: %d/%d", 
                    status.ResponseTime, stats.InUseConnections, stats.OpenConnections)
                    
                // Check for performance issues
                if stats.SlowQueries > 0 {
                    log.Printf("Warning: %d slow queries detected", stats.SlowQueries)
                }
                
                utilization := float64(stats.InUseConnections) / float64(stats.OpenConnections)
                if utilization > 0.8 {
                    log.Printf("Warning: High connection utilization: %.1f%%", utilization*100)
                }
            } else {
                log.Printf("DB Health: FAILED - %s", status.Error)
            }
            
            cancel()
        }
    }
}
```

This comprehensive API reference provides complete documentation for integrating and using the PostgreSQL database provider in Shelly Manager applications.