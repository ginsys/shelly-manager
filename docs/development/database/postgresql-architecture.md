# PostgreSQL Provider - Technical Architecture

## Executive Summary

The PostgreSQL database provider for Shelly Manager implements a production-ready, secure, and performant database abstraction layer. Built on GORM and the PostgreSQL driver, it provides comprehensive SSL/TLS security, connection pooling, health monitoring, and transaction management while maintaining full compatibility with the existing database interface.

## Architecture Overview

### System Integration

The PostgreSQL provider integrates seamlessly into the Shelly Manager database architecture through a clean provider interface pattern:

```
┌─────────────────────────────────────────────────────────────┐
│                    Shelly Manager Application               │
├─────────────────────────────────────────────────────────────┤
│                    Database Manager Layer                  │
├─────────────────────────────────────────────────────────────┤
│                  DatabaseProvider Interface                │
├─────────────────────────────────────────────────────────────┤
│  SQLiteProvider  │  PostgreSQLProvider  │  MySQLProvider   │
├─────────────────────────────────────────────────────────────┤
│      GORM ORM Framework & Database Drivers                 │
├─────────────────────────────────────────────────────────────┤
│    SQLite         │    PostgreSQL        │     MySQL       │
└─────────────────────────────────────────────────────────────┘
```

### Core Components

#### 1. PostgreSQLProvider Structure

```go
type PostgreSQLProvider struct {
    db     *gorm.DB              // GORM database instance
    config DatabaseConfig        // Configuration settings
    logger *logging.Logger       // Structured logging

    // Statistics tracking
    stats         DatabaseStats
    statsMu       sync.RWMutex
    queryCount    int64
    slowQueries   int64
    failedQueries int64
    totalLatency  int64

    // Connection management
    connected bool
    connMu    sync.RWMutex
}
```

#### 2. Interface Implementation

The provider implements multiple interfaces for comprehensive functionality:

- **DatabaseProvider**: Core database operations (Connect, Close, Migrate, etc.)
- **HealthChecker**: Health monitoring and status reporting
- **Transaction Management**: ACID-compliant transaction support
- **Statistics Provider**: Performance metrics and monitoring

### Design Decisions

#### Security-First Architecture

**Default SSL Enforcement**: All connections default to `sslmode=require` unless explicitly configured otherwise. This ensures encryption in transit by default.

**Credential Protection**: The provider implements comprehensive credential sanitization:
- DSN passwords are masked in logs
- Error messages never expose credentials
- Connection details are logged safely with credential redaction

**Certificate Validation**: Full support for SSL certificate validation including:
- Client certificates for mutual TLS authentication
- Root CA certificate validation
- Full certificate chain verification

#### Connection Pool Optimization

**PostgreSQL-Optimized Defaults**:
- Maximum Open Connections: 25 (PostgreSQL can handle higher concurrency than SQLite)
- Maximum Idle Connections: 5 (balanced for quick reuse without resource waste)
- Connection Lifetime: 1 hour (PostgreSQL connections can live longer safely)
- Idle Timeout: 10 minutes (appropriate for PostgreSQL workloads)

**Dynamic Configuration**: All pool parameters are configurable based on deployment requirements.

#### Performance Monitoring

**Built-in Metrics Collection**:
- Query count and timing statistics
- Slow query identification and logging
- Connection pool utilization metrics
- Database size monitoring with PostgreSQL-specific queries

**Thread-Safe Statistics**: All statistics collection uses atomic operations and proper locking to ensure accuracy under concurrent load.

## Component Interactions

### Connection Lifecycle

```mermaid
sequenceDiagram
    participant App as Application
    participant Provider as PostgreSQLProvider
    participant Pool as Connection Pool
    participant DB as PostgreSQL Database

    App->>Provider: Connect(config)
    Provider->>Provider: buildDSN(config)
    Provider->>Provider: validateSSLConfig()
    Provider->>DB: Open connection with SSL
    DB-->>Provider: Connection established
    Provider->>Pool: Configure connection pool
    Provider->>DB: Ping() health check
    DB-->>Provider: Pong
    Provider->>DB: SELECT version()
    DB-->>Provider: Version info
    Provider-->>App: Connection ready
```

### Transaction Management

The provider implements PostgreSQL's default Read Committed isolation level with full ACID compliance:

```go
func (p *PostgreSQLProvider) BeginTransaction() (Transaction, error) {
    tx := p.db.Begin(&sql.TxOptions{
        Isolation: sql.LevelReadCommitted, // PostgreSQL default
    })
    return &postgresTransaction{tx: tx}, nil
}
```

### Health Monitoring Integration

The health check system provides comprehensive database status:

```go
type HealthStatus struct {
    Healthy      bool                   `json:"healthy"`
    ResponseTime time.Duration          `json:"response_time"`
    Details      map[string]interface{} `json:"details"`
    CheckedAt    time.Time              `json:"checked_at"`
}
```

Health checks include:
- Connection availability (ping test)
- Response time measurement
- Connection pool status
- Database size metrics
- Version information

## Integration Patterns

### Database Manager Integration

The provider integrates with the existing database manager through the factory pattern:

```go
// File: internal/database/provider/factory.go
func CreateProvider(providerType string) (DatabaseProvider, error) {
    switch providerType {
    case "postgresql":
        return NewPostgreSQLProvider(logger), nil
    case "sqlite":
        return NewSQLiteProvider(logger), nil
    default:
        return nil, fmt.Errorf("unsupported provider: %s", providerType)
    }
}
```

### Configuration Integration

The provider reads configuration from the standard Shelly Manager configuration structure:

```yaml
database:
  provider: "postgresql"
  dsn: "postgres://user:password@localhost:5432/shelly_manager?sslmode=require"
  options:
    sslmode: "require"
    connect_timeout: "30"
    application_name: "shelly-manager"
  max_open_conns: 25
  max_idle_conns: 5
  conn_max_lifetime: "1h"
  conn_max_idle_time: "10m"
  slow_query_threshold: "200ms"
  log_level: "warn"
```

### Logging Integration

The provider integrates with Shelly Manager's structured logging system:

```go
p.logger.WithFields(map[string]any{
    "provider": "postgresql",
    "host":     p.getHostFromDSN(dsn),
    "database": p.getDatabaseFromDSN(dsn),
}).Info("Connected to PostgreSQL database")
```

## Data Flow Architecture

### Query Execution Flow

```
Application Request
       ↓
DatabaseProvider.GetDB()
       ↓
GORM Query Builder
       ↓
PostgreSQL Driver
       ↓
Connection Pool
       ↓
PostgreSQL Database
       ↓
Result Processing
       ↓
Statistics Update
       ↓
Response to Application
```

### Migration Flow

The provider supports GORM's AutoMigrate functionality with PostgreSQL-specific optimizations:

```go
func (p *PostgreSQLProvider) Migrate(models ...interface{}) error {
    start := time.Now()
    err := p.db.AutoMigrate(models...)
    duration := time.Since(start)
    
    // Log migration results with timing
    p.logger.WithFields(map[string]any{
        "duration": duration,
        "models":   len(models),
    }).Info("Database migration completed successfully")
    
    return err
}
```

## Error Handling Strategy

### Connection Error Recovery

The provider implements graceful error handling for connection issues:

1. **Initial Connection Failures**: Clear error messages without credential exposure
2. **Connection Loss**: Automatic connection pool management handles reconnection
3. **Transaction Failures**: Proper rollback with detailed error reporting
4. **SSL/TLS Issues**: Specific error messages for certificate and configuration problems

### Security Error Handling

All error messages are sanitized to prevent information disclosure:

```go
func (p *PostgreSQLProvider) getHostFromDSN(dsn string) string {
    if parsedURL, err := url.Parse(dsn); err == nil && parsedURL.Host != "" {
        return parsedURL.Host  // Only host, no credentials
    }
    return "unknown"
}
```

## Performance Characteristics

### Connection Pool Efficiency

The provider is optimized for PostgreSQL's connection model:
- **Higher Concurrency**: PostgreSQL can handle more concurrent connections than SQLite
- **Longer Connection Lifetimes**: PostgreSQL connections are more expensive to establish but can live longer
- **Intelligent Pool Management**: Dynamic adjustment based on workload patterns

### Query Performance

The provider includes built-in query performance monitoring:
- Slow query detection with configurable thresholds
- Query count and timing statistics
- Connection pool utilization metrics
- Database size monitoring for capacity planning

### Memory Management

Careful memory management prevents resource leaks:
- Proper connection cleanup on provider shutdown
- Statistics collection uses atomic operations
- GORM integration provides efficient ORM performance

## Security Architecture

### SSL/TLS Implementation

The provider implements comprehensive SSL/TLS security:

```go
// Default to secure connection
sslMode := "require" // Default to requiring SSL
if mode, ok := options["sslmode"]; ok {
    sslMode = mode
}
```

**Supported SSL Modes**:
- `disable`: No SSL (only for development/internal networks)
- `allow`: SSL if available, fallback to non-SSL
- `prefer`: SSL preferred, fallback to non-SSL  
- `require`: SSL required, no fallback (default)
- `verify-ca`: SSL with CA certificate validation
- `verify-full`: Full SSL certificate validation including hostname

### Certificate Management

Full support for SSL certificate management:

```go
func (p *PostgreSQLProvider) validateSSLConfig(options map[string]string) error {
    // Validate certificate files exist
    if sslMode == "verify-ca" || sslMode == "verify-full" {
        if sslRootCert, ok := options["sslrootcert"]; ok {
            if _, err := os.Stat(sslRootCert); os.IsNotExist(err) {
                return fmt.Errorf("SSL root certificate not found: %s", sslRootCert)
            }
        }
    }
}
```

### Connection Security

**Timeout Protection**: Default connection timeouts prevent resource exhaustion attacks
**Parameter Validation**: All DSN parameters are validated to prevent injection attacks
**Thread Safety**: All operations are thread-safe with proper locking mechanisms

This architectural foundation provides a robust, secure, and performant PostgreSQL integration for the Shelly Manager system, with full compatibility with existing code and deployment patterns.