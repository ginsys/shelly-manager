# MySQL Provider - Technical Architecture

## Executive Summary

The MySQL database provider for Shelly Manager implements a production-ready, secure, and performant database abstraction layer designed for enterprise-grade applications. Built on GORM and the MySQL driver, it provides comprehensive SSL/TLS security, connection pooling, health monitoring, and transaction management while maintaining full compatibility with the existing database interface.

**Key Architecture Principles:**
- **Security-First Design**: Default SSL enforcement with comprehensive credential protection
- **Performance Optimization**: MySQL-specific connection pool tuning and performance monitoring
- **Thread-Safe Operations**: Concurrent-safe design with proper synchronization primitives
- **Error Resilience**: Comprehensive error handling with sanitized error messages

## Architecture Overview

### System Integration

The MySQL provider integrates seamlessly into the Shelly Manager database architecture through a clean provider interface pattern:

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

#### 1. MySQLProvider Structure

```go
type MySQLProvider struct {
    db     *gorm.DB              // GORM database instance
    config DatabaseConfig        // Configuration settings
    logger *logging.Logger       // Structured logging

    // Statistics tracking with atomic operations
    stats         DatabaseStats
    statsMu       sync.RWMutex
    queryCount    int64
    slowQueries   int64
    failedQueries int64
    totalLatency  int64

    // Thread-safe connection management
    connected bool
    connMu    sync.RWMutex
}
```

**Thread Safety Design:**
- `RWMutex` for connection state protection
- Atomic operations for performance counters
- Thread-safe statistics collection

#### 2. Interface Implementation

The MySQL provider implements the complete `DatabaseProvider` interface:

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

#### 3. Transaction Management

```go
type mysqlTransaction struct {
    tx *gorm.DB
}

func (t *mysqlTransaction) GetDB() *gorm.DB {
    return t.tx
}

func (t *mysqlTransaction) Commit() error {
    return t.tx.Commit().Error
}

func (t *mysqlTransaction) Rollback() error {
    return t.tx.Rollback().Error
}
```

**Transaction Features:**
- Read Committed isolation level (MySQL default)
- Proper resource cleanup
- ACID compliance

## Design Decisions and Rationale

### 1. Security-First Architecture

**DSN Security Validation**
```go
func (m *MySQLProvider) validateDSNInput(dsn string) error {
    dangerousPatterns := []string{
        "DROP TABLE", "DROP DATABASE", "DELETE FROM", "INSERT INTO",
        "UPDATE SET", "CREATE TABLE", "ALTER TABLE", "TRUNCATE",
        "--", "/*", "*/", ";", "EXEC", "EXECUTE",
        "sp_", "xp_", "UNION SELECT", "INFORMATION_SCHEMA",
        "<script", "javascript:", "onload=", "onerror=",
    }
    // ... validation logic
}
```

**Decision Rationale:**
- Prevents SQL injection attacks through DSN parameters
- Protects against XSS attempts in configuration
- Validates all user input before connection attempts

**Credential Sanitization**
```go
func (m *MySQLProvider) sanitizeError(err error) error {
    credentialPatterns := []*regexp.Regexp{
        regexp.MustCompile(`:[^:@/]+@`),        // Remove password in user:password@host format
        regexp.MustCompile(`user=\w+`),         // Remove user parameter
        regexp.MustCompile(`password=[^\s&]+`), // Remove password parameter
        regexp.MustCompile(`dbname=\w+`),       // Remove database name
    }
    // ... sanitization logic
}
```

**Decision Rationale:**
- Prevents credential leakage in error messages and logs
- Maintains debugging information while ensuring security
- Covers multiple DSN formats and credential patterns

### 2. Performance Optimization Strategy

**MySQL-Specific Connection Pool Defaults**
```go
// MySQL-appropriate defaults in configureConnectionPool()
maxOpenConns := 20     // Conservative setting for MySQL
maxIdleConns := 5      // Minimal idle connections
connMaxLifetime := 30 * time.Minute  // MySQL connections should be rotated more frequently
connMaxIdleTime := 5 * time.Minute
```

**Decision Rationale:**
- MySQL performs better with moderate connection counts
- Frequent connection rotation prevents connection staleness
- Balances performance with resource utilization

**Performance Monitoring**
```go
type DatabaseStats struct {
    OpenConnections  int
    InUseConnections int  
    IdleConnections  int
    TotalQueries     int64
    SlowQueries      int64
    FailedQueries    int64
    AverageLatency   time.Duration
    DatabaseSize     int64
}
```

**Decision Rationale:**
- Provides comprehensive performance visibility
- Enables proactive monitoring and alerting
- Supports capacity planning and optimization

### 3. SSL/TLS Security Architecture

**Comprehensive TLS Mode Support**
```go
validTLSModes := map[string]bool{
    "false":           true, // Disable SSL
    "true":            true, // Enable SSL without verification
    "skip-verify":     true, // Enable SSL but skip certificate verification
    "preferred":       true, // Prefer SSL, fallback to non-SSL
    "custom":          true, // Use custom TLS config
    "required":        true, // Require SSL connection
    "verify-ca":       true, // Verify CA certificate
    "verify-identity": true, // Verify CA certificate and server hostname
}
```

**Decision Rationale:**
- Supports all MySQL TLS modes for flexibility
- Default to "preferred" for security with compatibility
- Validates certificate files for verification modes

**Certificate Validation**
```go
func (m *MySQLProvider) validateSSLConfig(options map[string]string) error {
    // Validate TLS mode and certificate file existence
    if tlsMode == "custom" || tlsMode == "verify-ca" || tlsMode == "verify-identity" {
        if ca, ok := options["ca"]; ok && ca != "" {
            if _, err := os.Stat(ca); os.IsNotExist(err) {
                return fmt.Errorf("CA certificate not found: %s", ca)
            }
        }
        // ... additional certificate validation
    }
}
```

**Decision Rationale:**
- Ensures certificate files exist before connection attempts
- Provides clear error messages for configuration issues
- Supports mutual TLS authentication

## Data Flow and Processing

### 1. Connection Establishment Flow

```
User Config → DSN Validation → SSL Validation → Connection Pool Config → Database Connection → Version Discovery → Statistics Initialization
```

**Detailed Flow:**
1. **Configuration Parsing**: Parse and validate user configuration
2. **Security Validation**: Validate DSN for injection patterns
3. **SSL Configuration**: Validate SSL settings and certificates
4. **DSN Construction**: Build complete DSN with security defaults
5. **Connection Establishment**: Create GORM database instance
6. **Pool Configuration**: Apply MySQL-specific connection pool settings
7. **Health Check**: Verify connection with ping
8. **Version Discovery**: Query MySQL version for metadata
9. **Statistics Initialization**: Initialize performance tracking

### 2. Query Execution Flow

```
Application Query → GORM Processing → MySQL Driver → Database → Response → Statistics Update → Application Response
```

**Performance Tracking:**
- Query count increment (atomic)
- Latency measurement
- Slow query detection
- Failed query tracking

### 3. Transaction Management Flow

```
BeginTransaction → Create Transaction Context → Execute Operations → Commit/Rollback → Cleanup
```

**Transaction Features:**
- Read Committed isolation level
- Proper resource cleanup
- Error handling and rollback

## Integration Points

### 1. GORM Integration

**GORM Logger Configuration**
```go
func (m *MySQLProvider) createGormLogger() logger.Interface {
    return logger.New(
        log.New(&gormLogWriter{logger: m.logger}, "", 0),
        logger.Config{
            SlowThreshold:             m.config.SlowQueryThreshold,
            LogLevel:                  logLevel,
            IgnoreRecordNotFoundError: true,
            Colorful:                  false,
        },
    )
}
```

**Integration Benefits:**
- Structured logging integration
- Configurable log levels
- Slow query detection
- Security-safe log output

### 2. Health Checking Integration

```go
func (m *MySQLProvider) HealthCheck(ctx context.Context) HealthStatus {
    status := HealthStatus{
        CheckedAt: time.Now(),
        Details:   make(map[string]interface{}),
    }
    
    // Perform health check with timeout
    if err := m.Ping(); err != nil {
        status.Healthy = false
        status.Error = err.Error()
        return status
    }
    
    // Include performance metrics
    stats := m.GetStats()
    status.Details["database_size"] = stats.DatabaseSize
    status.Details["total_queries"] = stats.TotalQueries
    status.Details["connection_count"] = stats.OpenConnections
    
    return status
}
```

**Health Check Features:**
- Connection verification with timeout
- Performance metrics inclusion
- Response time measurement
- Detailed status information

### 3. Statistics Collection Integration

**Real-Time Statistics**
```go
func (m *MySQLProvider) updateStats() {
    sqlDB, err := m.db.DB()
    if err != nil {
        return
    }

    stats := sqlDB.Stats()
    m.stats.OpenConnections = stats.OpenConnections
    m.stats.InUseConnections = stats.InUse
    m.stats.IdleConnections = stats.Idle

    // MySQL-specific database size query
    var dbSize int64
    query := "SELECT SUM(data_length + index_length) AS size FROM information_schema.tables WHERE table_schema = DATABASE()"
    if err := m.db.Raw(query).Scan(&dbSize).Error; err == nil {
        m.stats.DatabaseSize = dbSize
    }
}
```

**Statistics Features:**
- Real-time connection pool metrics
- Database size calculation
- Query performance tracking
- Thread-safe statistics updates

## Error Handling Strategy

### 1. Connection Error Handling

**Graceful Connection Failures**
```go
func (m *MySQLProvider) Connect(config DatabaseConfig) error {
    if err := db.Ping(); err != nil {
        m.db = nil
        return fmt.Errorf("failed to ping database: %w", m.sanitizeError(err))
    }
}
```

**Error Handling Principles:**
- Clean resource cleanup on failure
- Sanitized error messages
- Proper error wrapping
- Detailed logging for debugging

### 2. Operation Error Handling

**Safe Operation Execution**
```go
func (m *MySQLProvider) Migrate(models ...interface{}) error {
    if !m.connected || m.db == nil {
        return fmt.Errorf("not connected to database")
    }

    start := time.Now()
    err := m.db.AutoMigrate(models...)
    duration := time.Since(start)

    if err != nil {
        m.logger.WithFields(map[string]any{
            "error":    err.Error(),
            "duration": duration,
            "models":   len(models),
        }).Error("Database migration failed")
        atomic.AddInt64(&m.failedQueries, 1)
        return fmt.Errorf("migration failed: %w", err)
    }
    
    return nil
}
```

**Error Handling Features:**
- Connection state validation
- Performance metric tracking
- Structured error logging
- Atomic failure counting

## Security Architecture

### 1. Input Validation

**Multi-Layer Validation**
- DSN format validation
- SQL injection pattern detection
- XSS pattern detection
- Certificate file validation

### 2. Credential Protection

**Comprehensive Sanitization**
- Error message sanitization
- Log output sanitization
- Multiple credential pattern detection
- Safe placeholder replacement

### 3. Connection Security

**Default Security Settings**
- Preferred SSL mode by default
- Connection timeout enforcement
- Resource limit enforcement
- Secure configuration validation

## Performance Characteristics

### 1. Connection Pool Performance

**Optimized for MySQL**
- Conservative connection limits (20 max)
- Frequent connection rotation (30 minutes)
- Minimal idle connections (5)
- Fast idle timeout (5 minutes)

### 2. Query Performance

**Performance Monitoring**
- Query execution time tracking
- Slow query identification (configurable threshold)
- Failed query tracking
- Average latency calculation

### 3. Memory Management

**Efficient Resource Usage**
- Atomic counters for statistics
- RWMutex for minimal lock contention
- Efficient string operations
- Proper resource cleanup

## Implementation Benefits

### For Developers
- **Drop-in Replacement**: Implements same interface as other providers
- **Rich Diagnostics**: Comprehensive error messages and logging
- **Performance Visibility**: Built-in performance monitoring
- **Security Built-in**: Automatic credential protection

### For DevOps
- **Production Ready**: Comprehensive testing and validation
- **Monitoring Integration**: Built-in health checking and statistics
- **Configuration Flexibility**: Extensive configuration options
- **Secure by Default**: SSL enforcement and input validation

### For Security Teams
- **Defense in Depth**: Multiple security layers
- **Credential Protection**: Comprehensive credential sanitization
- **Audit Trail**: Structured security logging
- **Compliance Ready**: SSL/TLS and certificate validation

This architecture provides a robust, secure, and performant foundation for MySQL database operations in the Shelly Manager application while maintaining compatibility with the existing database abstraction layer.