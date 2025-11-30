# PostgreSQL Configuration Guide

## Overview

This guide covers all PostgreSQL-specific configuration options for the Shelly Manager database provider, including connection settings, security options, performance tuning, and environment-specific configurations.

## Configuration Structure

### Basic Configuration Schema

```yaml
database:
  provider: "postgresql"                    # Database provider type
  dsn: "connection_string"                  # PostgreSQL connection string
  options:                                  # Provider-specific options
    key: "value"
  max_open_conns: 25                       # Maximum concurrent connections
  max_idle_conns: 5                        # Maximum idle connections
  conn_max_lifetime: "1h"                  # Maximum connection lifetime
  conn_max_idle_time: "10m"                # Maximum connection idle time
  slow_query_threshold: "200ms"            # Slow query detection threshold
  log_level: "warn"                        # Logging level for database operations
```

## Connection String (DSN) Configuration

### Standard PostgreSQL DSN Format

```
postgres://username:password@hostname:port/database_name?param1=value1&param2=value2
```

### DSN Components

| Component | Description | Example | Required |
|-----------|-------------|---------|----------|
| `protocol` | Database protocol | `postgres://` or `postgresql://` | Yes |
| `username` | Database user | `shelly_user` | Yes |
| `password` | User password | `secure_password123` | Yes |
| `hostname` | Database server host | `localhost`, `db.example.com` | Yes |
| `port` | Database server port | `5432` (default) | No |
| `database` | Database name | `shelly_manager` | Yes |
| `parameters` | Query parameters | `?sslmode=require&timeout=30` | No |

### Example DSN Configurations

#### Development (Local PostgreSQL)
```yaml
database:
  dsn: "postgres://shelly:password@localhost:5432/shelly_dev?sslmode=prefer"
```

#### Production (Remote PostgreSQL with SSL)
```yaml
database:
  dsn: "postgres://shelly:password@db.prod.com:5432/shelly_prod?sslmode=require&connect_timeout=30"
```

#### High Security (Certificate-based authentication)
```yaml
database:
  dsn: "postgres://shelly@db.secure.com:5432/shelly?sslmode=verify-full&sslcert=/certs/client.crt&sslkey=/certs/client.key&sslrootcert=/certs/ca.crt"
```

## PostgreSQL-Specific Options

### SSL/TLS Options

#### SSL Mode Configuration

| Option | Security Level | Description | Use Case |
|--------|----------------|-------------|----------|
| `disable` | None | No SSL encryption | Development only, internal networks |
| `allow` | Low | SSL if available | Transitioning environments |
| `prefer` | Medium | SSL preferred | Development, testing |
| `require` | High | SSL required (default) | Production standard |
| `verify-ca` | Higher | SSL + CA verification | High security environments |
| `verify-full` | Highest | SSL + full certificate verification | Maximum security |

```yaml
database:
  options:
    sslmode: "require"                    # Default: require
    sslcert: "/path/to/client.crt"       # Client certificate
    sslkey: "/path/to/client.key"        # Client private key
    sslrootcert: "/path/to/ca.crt"       # Root CA certificate
```

#### Certificate File Requirements

For `verify-ca` and `verify-full` modes:

**Root CA Certificate** (`sslrootcert`):
```bash
# Must be readable by the application
chmod 600 /path/to/ca.crt
chown shelly-manager:shelly-manager /path/to/ca.crt
```

**Client Certificate** (`sslcert`):
```bash
# Client certificate for mutual TLS
chmod 600 /path/to/client.crt
```

**Client Private Key** (`sslkey`):
```bash
# Must be secured and readable only by application
chmod 400 /path/to/client.key
chown shelly-manager:shelly-manager /path/to/client.key
```

### Connection Options

#### Timeout Configuration

```yaml
database:
  options:
    connect_timeout: "30"                 # Connection timeout (seconds)
    statement_timeout: "300000"           # Statement timeout (milliseconds)
    idle_in_transaction_session_timeout: "600000"  # Idle transaction timeout
```

#### Application Identification

```yaml
database:
  options:
    application_name: "shelly-manager"    # Identifies application in PostgreSQL logs
    search_path: "public,shelly"          # Schema search path
```

#### Performance Options

```yaml
database:
  options:
    shared_preload_libraries: "pg_stat_statements"  # Enable query statistics
    work_mem: "256MB"                     # Working memory for queries
    maintenance_work_mem: "512MB"         # Memory for maintenance operations
```

## Connection Pool Configuration

### Pool Sizing Guidelines

#### Development Environment
```yaml
database:
  max_open_conns: 10                      # Lower concurrency
  max_idle_conns: 2                       # Minimal idle connections
  conn_max_lifetime: "30m"                # Shorter lifetime for development
  conn_max_idle_time: "5m"                # Quick cleanup
```

#### Production Environment
```yaml
database:
  max_open_conns: 50                      # Higher concurrency
  max_idle_conns: 10                      # Maintain ready connections
  conn_max_lifetime: "2h"                 # Longer lifetime for stability
  conn_max_idle_time: "15m"               # Balanced cleanup
```

#### High-Traffic Environment
```yaml
database:
  max_open_conns: 100                     # Maximum concurrency
  max_idle_conns: 25                      # High idle count
  conn_max_lifetime: "4h"                 # Very long lifetime
  conn_max_idle_time: "30m"               # Extended idle time
```

### Pool Sizing Calculation

**Rule of Thumb**: 
- Start with: `max_open_conns = number_of_cpu_cores * 4`
- Adjust based on application characteristics:
  - I/O intensive: Higher pool size (6-8x cores)
  - CPU intensive: Lower pool size (2-3x cores)
  - Mixed workload: Moderate pool size (4-5x cores)

**Maximum Idle Connections**:
- Typically 20-40% of maximum open connections
- Balance between connection reuse and resource consumption

## Performance Configuration

### Query Performance

#### Slow Query Detection
```yaml
database:
  slow_query_threshold: "200ms"           # Log queries slower than 200ms
  log_level: "warn"                       # Include slow queries in warnings
```

#### Query Optimization Settings
```yaml
database:
  options:
    default_statistics_target: "100"      # Statistics collection detail
    random_page_cost: "1.1"               # SSD-optimized random access cost
    effective_cache_size: "4GB"           # Available memory for caching
```

### Connection Performance

#### Connection Caching
```yaml
database:
  options:
    tcp_keepalives_idle: "300"            # TCP keepalive idle time
    tcp_keepalives_interval: "30"         # TCP keepalive interval
    tcp_keepalives_count: "3"             # TCP keepalive probe count
```

## Environment-Specific Configurations

### Development Environment

**File: `configs/shelly-manager.dev.yaml`**
```yaml
database:
  provider: "postgresql"
  dsn: "postgres://shelly:devpass@localhost:5432/shelly_dev?sslmode=prefer"
  options:
    sslmode: "prefer"
    connect_timeout: "10"
    application_name: "shelly-manager-dev"
  max_open_conns: 10
  max_idle_conns: 2
  conn_max_lifetime: "30m"
  conn_max_idle_time: "5m"
  slow_query_threshold: "100ms"
  log_level: "info"
```

### Staging Environment

**File: `configs/shelly-manager.staging.yaml`**
```yaml
database:
  provider: "postgresql"
  dsn: "postgres://shelly:stagingpass@db-staging:5432/shelly_staging?sslmode=require"
  options:
    sslmode: "require"
    connect_timeout: "20"
    application_name: "shelly-manager-staging"
  max_open_conns: 25
  max_idle_conns: 5
  conn_max_lifetime: "1h"
  conn_max_idle_time: "10m"
  slow_query_threshold: "200ms"
  log_level: "warn"
```

### Production Environment

**File: `configs/shelly-manager.prod.yaml`**
```yaml
database:
  provider: "postgresql"
  dsn: "postgres://shelly:${POSTGRES_PASSWORD}@db-prod:5432/shelly_prod?sslmode=verify-full&sslcert=/certs/client.crt&sslkey=/certs/client.key&sslrootcert=/certs/ca.crt"
  options:
    sslmode: "verify-full"
    sslcert: "/certs/client.crt"
    sslkey: "/certs/client.key"
    sslrootcert: "/certs/ca.crt"
    connect_timeout: "30"
    application_name: "shelly-manager-prod"
  max_open_conns: 50
  max_idle_conns: 10
  conn_max_lifetime: "2h"
  conn_max_idle_time: "15m"
  slow_query_threshold: "500ms"
  log_level: "error"
```

## Environment Variables

### Standard Environment Variables

```bash
# Database connection
export POSTGRES_HOST="localhost"
export POSTGRES_PORT="5432"
export POSTGRES_USER="shelly"
export POSTGRES_PASSWORD="secure_password"
export POSTGRES_DB="shelly_manager"

# SSL configuration
export POSTGRES_SSL_MODE="require"
export POSTGRES_SSL_CERT="/certs/client.crt"
export POSTGRES_SSL_KEY="/certs/client.key"
export POSTGRES_SSL_ROOT_CERT="/certs/ca.crt"

# Connection pool
export POSTGRES_MAX_OPEN_CONNS="25"
export POSTGRES_MAX_IDLE_CONNS="5"
export POSTGRES_CONN_MAX_LIFETIME="1h"
export POSTGRES_CONN_MAX_IDLE_TIME="10m"

# Performance
export POSTGRES_SLOW_QUERY_THRESHOLD="200ms"
export POSTGRES_LOG_LEVEL="warn"
```

### Docker Environment Variables

**File: `docker-compose.yml`**
```yaml
services:
  shelly-manager:
    environment:
      - DATABASE_PROVIDER=postgresql
      - POSTGRES_HOST=postgres
      - POSTGRES_PORT=5432
      - POSTGRES_USER=shelly
      - POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
      - POSTGRES_DB=shelly_manager
      - POSTGRES_SSL_MODE=require
      - POSTGRES_MAX_OPEN_CONNS=25
      - POSTGRES_MAX_IDLE_CONNS=5
    depends_on:
      - postgres

  postgres:
    image: postgres:15
    environment:
      - POSTGRES_USER=shelly
      - POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
      - POSTGRES_DB=shelly_manager
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./certs:/certs:ro
    ports:
      - "5432:5432"
```

### Kubernetes Configuration

**File: `k8s/configmap.yaml`**
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: shelly-manager-config
data:
  DATABASE_PROVIDER: "postgresql"
  POSTGRES_HOST: "postgres-service"
  POSTGRES_PORT: "5432"
  POSTGRES_USER: "shelly"
  POSTGRES_DB: "shelly_manager"
  POSTGRES_SSL_MODE: "require"
  POSTGRES_MAX_OPEN_CONNS: "50"
  POSTGRES_MAX_IDLE_CONNS: "10"
  POSTGRES_CONN_MAX_LIFETIME: "2h"
  POSTGRES_CONN_MAX_IDLE_TIME: "15m"
  POSTGRES_SLOW_QUERY_THRESHOLD: "500ms"
  POSTGRES_LOG_LEVEL: "warn"
```

**File: `k8s/secret.yaml`**
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: shelly-manager-db-secret
type: Opaque
data:
  POSTGRES_PASSWORD: <base64-encoded-password>
```

## Configuration Validation

### Automatic Validation

The PostgreSQL provider automatically validates configuration during connection:

```go
// DSN validation
if baseDSN == "" {
    return "", fmt.Errorf("DSN cannot be empty")
}

// SSL configuration validation
if sslMode == "verify-ca" || sslMode == "verify-full" {
    if sslRootCert, ok := options["sslrootcert"]; ok {
        if _, err := os.Stat(sslRootCert); os.IsNotExist(err) {
            return fmt.Errorf("SSL root certificate not found: %s", sslRootCert)
        }
    }
}
```

### Pre-deployment Configuration Check

**Script: `scripts/check-postgres-config.sh`**
```bash
#!/bin/bash

echo "Checking PostgreSQL configuration..."

# Check required environment variables
required_vars=("POSTGRES_HOST" "POSTGRES_USER" "POSTGRES_PASSWORD" "POSTGRES_DB")
for var in "${required_vars[@]}"; do
    if [ -z "${!var}" ]; then
        echo "ERROR: Required environment variable $var is not set"
        exit 1
    fi
done

# Check SSL certificates if using verify modes
if [[ "$POSTGRES_SSL_MODE" == "verify-ca" || "$POSTGRES_SSL_MODE" == "verify-full" ]]; then
    if [ ! -f "$POSTGRES_SSL_ROOT_CERT" ]; then
        echo "ERROR: SSL root certificate not found: $POSTGRES_SSL_ROOT_CERT"
        exit 1
    fi
    
    if [ ! -f "$POSTGRES_SSL_CERT" ]; then
        echo "ERROR: SSL client certificate not found: $POSTGRES_SSL_CERT"
        exit 1
    fi
    
    if [ ! -f "$POSTGRES_SSL_KEY" ]; then
        echo "ERROR: SSL private key not found: $POSTGRES_SSL_KEY"
        exit 1
    fi
fi

# Test connection
echo "Testing PostgreSQL connection..."
if ! timeout 10 pg_isready -h "$POSTGRES_HOST" -p "$POSTGRES_PORT" -U "$POSTGRES_USER"; then
    echo "ERROR: Cannot connect to PostgreSQL server"
    exit 1
fi

echo "PostgreSQL configuration check passed ✓"
```

## Common Configuration Patterns

### High Availability Setup

```yaml
database:
  dsn: "postgres://shelly:pass@postgres-primary:5432/shelly?sslmode=require&target_session_attrs=read-write"
  options:
    sslmode: "require"
    connect_timeout: "5"
    application_name: "shelly-manager"
    target_session_attrs: "read-write"  # Connect to primary only
  max_open_conns: 50
  max_idle_conns: 10
  conn_max_lifetime: "1h"
  conn_max_idle_time: "10m"
```

### Read Replica Configuration

```yaml
database:
  dsn: "postgres://shelly:pass@postgres-replica:5432/shelly?sslmode=require&target_session_attrs=read-only"
  options:
    sslmode: "require"
    target_session_attrs: "read-only"   # Connect to replica only
    application_name: "shelly-manager-readonly"
```

### Connection Pooler (PgBouncer) Setup

```yaml
database:
  dsn: "postgres://shelly:pass@pgbouncer:6432/shelly?sslmode=require&pool_mode=transaction"
  options:
    sslmode: "require"
    pool_mode: "transaction"            # PgBouncer transaction pooling
    application_name: "shelly-manager-pooled"
  max_open_conns: 20                    # Lower due to pooler
  max_idle_conns: 5
  conn_max_lifetime: "10m"              # Shorter for pooled connections
```

## Usage Examples

### Complete PostgreSQL Connection Example

The following example demonstrates how to use the PostgreSQL provider with various configurations:

```go
package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/ginsys/shelly-manager/internal/database/provider"
	"github.com/ginsys/shelly-manager/internal/logging"
)

// Example model to demonstrate migrations
type Device struct {
	ID   uint   `gorm:"primarykey"`
	Name string `gorm:"not null"`
	IP   string `gorm:"unique;not null"`
}

func main() {
	// Initialize logger
	logger := logging.GetDefault()

	// Create PostgreSQL provider
	postgresProvider := provider.NewPostgreSQLProvider(logger)

	// Configuration for PostgreSQL connection
	config := provider.DatabaseConfig{
		Provider: "postgresql",
		DSN:      "postgres://username:password@localhost:5432/shelly_manager?sslmode=require",
		Options: map[string]string{
			"sslmode":          "require",        // Enforce SSL
			"connect_timeout":  "30",             // 30 second connection timeout
			"application_name": "shelly-manager", // Application identifier
		},
		MaxOpenConns:       25,                     // PostgreSQL can handle more connections
		MaxIdleConns:       5,                      // Keep some connections idle
		ConnMaxLifetime:    time.Hour,              // Connections live longer in PostgreSQL
		ConnMaxIdleTime:    10 * time.Minute,       // Idle timeout
		SlowQueryThreshold: 200 * time.Millisecond, // Log slow queries
		LogLevel:           "warn",                 // Log level for GORM
	}

	// Example 1: Basic Connection
	fmt.Println("=== Basic PostgreSQL Connection Example ===")
	if err := connectAndTest(postgresProvider, config); err != nil {
		log.Printf("Connection failed: %v", err)
		return
	}

	// Example 2: Connection with SSL certificates
	fmt.Println("\n=== PostgreSQL Connection with SSL Certificates ===")
	sslConfig := config
	sslConfig.Options = map[string]string{
		"sslmode":     "verify-full",
		"sslcert":     "/path/to/client-cert.pem",
		"sslkey":      "/path/to/client-key.pem",
		"sslrootcert": "/path/to/ca-cert.pem",
	}
	if err := connectAndTest(postgresProvider, sslConfig); err != nil {
		log.Printf("SSL connection failed (expected if certificates don't exist): %v", err)
	}

	// Example 3: Connection Pool Configuration
	fmt.Println("\n=== Connection Pool Configuration Example ===")
	poolConfig := config
	poolConfig.MaxOpenConns = 50               // High-traffic configuration
	poolConfig.MaxIdleConns = 10               // More idle connections
	poolConfig.ConnMaxLifetime = 2 * time.Hour // Longer lifetime
	if err := connectAndTest(postgresProvider, poolConfig); err != nil {
		log.Printf("Pool configuration failed: %v", err)
	}
}

func connectAndTest(provider *provider.PostgreSQLProvider, config provider.DatabaseConfig) error {
	fmt.Printf("Attempting to connect with DSN: %s\n", maskPassword(config.DSN))

	// Connect to database
	if err := provider.Connect(config); err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer provider.Close()

	fmt.Printf("✓ Connected successfully to PostgreSQL %s\n", provider.Version())

	// Test connection
	if err := provider.Ping(); err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}
	fmt.Println("✓ Connection ping successful")

	// Perform migration
	if err := provider.Migrate(&Device{}); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}
	fmt.Println("✓ Migration completed successfully")

	// Get database statistics
	stats := provider.GetStats()
	fmt.Printf("✓ Database Stats - Connections: %d/%d (in use/open), Queries: %d\n",
		stats.InUseConnections, stats.OpenConnections, stats.TotalQueries)

	// Test transaction
	tx, err := provider.BeginTransaction()
	if err != nil {
		return fmt.Errorf("transaction failed: %w", err)
	}

	// Create a test device
	device := &Device{Name: "Test Device", IP: "192.168.1.100"}
	if err := tx.GetDB().Create(device).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("create failed: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}
	fmt.Println("✓ Transaction test successful")

	// Clean up
	if err := provider.DropTables(&Device{}); err != nil {
		return fmt.Errorf("cleanup failed: %w", err)
	}
	fmt.Println("✓ Cleanup completed")

	return nil
}

// maskPassword masks the password in the DSN for logging
func maskPassword(dsn string) string {
	// Simple password masking - in production, use proper URL parsing
	if idx := strings.Index(dsn, "://"); idx != -1 {
		if idx2 := strings.Index(dsn[idx+3:], "@"); idx2 != -1 {
			return dsn[:idx+3] + "****:****" + dsn[idx+3+idx2:]
		}
	}
	return dsn
}
```

### Running the Example

To run this example:

```bash
# Ensure PostgreSQL is running and accessible
cd /path/to/shelly-manager
go run -

# Copy the example code above to a file and run:
go run your_example.go
```

### Example Scenarios Covered

1. **Basic Connection**: Standard PostgreSQL connection with SSL requirement
2. **SSL Certificates**: Full SSL certificate-based authentication 
3. **Connection Pool**: High-traffic connection pool configuration
4. **Complete Workflow**: Connection → Migration → CRUD → Transaction → Cleanup

### Expected Output

```
=== Basic PostgreSQL Connection Example ===
Attempting to connect with DSN: postgres://****:****@localhost:5432/shelly_manager?sslmode=require
✓ Connected successfully to PostgreSQL 15.3
✓ Connection ping successful
✓ Migration completed successfully
✓ Database Stats - Connections: 1/1 (in use/open), Queries: 5
✓ Transaction test successful
✓ Cleanup completed

=== PostgreSQL Connection with SSL Certificates ===
Attempting to connect with DSN: postgres://****:****@localhost:5432/shelly_manager?sslmode=verify-full
SSL connection failed (expected if certificates don't exist): connection failed: pq: could not open file "/path/to/client-cert.pem": no such file or directory

=== Connection Pool Configuration Example ===
Attempting to connect with DSN: postgres://****:****@localhost:5432/shelly_manager?sslmode=require
✓ Connected successfully to PostgreSQL 15.3
✓ Connection ping successful
✓ Migration completed successfully
✓ Database Stats - Connections: 1/50 (in use/open), Queries: 5
✓ Transaction test successful
✓ Cleanup completed
```

This comprehensive configuration guide ensures optimal PostgreSQL integration across all deployment scenarios.