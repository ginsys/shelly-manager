# MySQL Provider - Usage Guide

## Executive Summary

This guide provides comprehensive instructions for configuring, deploying, and using the MySQL database provider in Shelly Manager. The MySQL provider offers enterprise-grade database capabilities with SSL/TLS security, connection pooling, and performance monitoring, making it ideal for production deployments requiring scalability and reliability.

**Key Benefits:**
- **Scalability**: Supports large-scale deployments with connection pooling
- **Security**: SSL/TLS encryption with certificate validation
- **Performance**: MySQL-optimized connection settings and monitoring  
- **Compatibility**: Drop-in replacement for SQLite provider

## Getting Started

### Prerequisites

**MySQL Server Requirements:**
- MySQL 5.7+ or MySQL 8.0+ (recommended)
- SSL/TLS support enabled (for production)
- User account with appropriate privileges
- Network connectivity to MySQL server

**Application Requirements:**
- Go 1.19+ with MySQL driver support
- Shelly Manager with database abstraction layer
- Configuration management system
- Logging framework

### Quick Start

#### 1. Basic Configuration

```yaml
# shelly-manager.yaml
database:
  provider: "mysql"
  dsn: "user:password@tcp(localhost:3306)/shelly_manager"
  options:
    tls: "preferred"  # Secure by default
    charset: "utf8mb4"
    timeout: "10s"
  max_open_conns: 20
  max_idle_conns: 5
  conn_max_lifetime: "30m"
  conn_max_idle_time: "5m"
  slow_query_threshold: "200ms"
  log_level: "warn"
```

#### 2. Environment Variables

```bash
# Production environment variables
export MYSQL_HOST=mysql.internal
export MYSQL_PORT=3306
export MYSQL_USER=shelly_app
export MYSQL_PASSWORD=secure_password
export MYSQL_DATABASE=shelly_manager
export MYSQL_SSL_MODE=verify-identity
export MYSQL_CA_CERT=/etc/ssl/mysql-ca.pem
export MYSQL_CLIENT_CERT=/etc/ssl/mysql-client.pem  
export MYSQL_CLIENT_KEY=/etc/ssl/mysql-client.key
```

#### 3. Code Integration

```go
package main

import (
    "log"
    "os"
    
    "github.com/ginsys/shelly-manager/internal/database/provider"
    "github.com/ginsys/shelly-manager/internal/logging"
)

func main() {
    // Initialize logger
    logger := logging.GetDefault()
    
    // Create MySQL provider
    mysqlProvider := provider.NewMySQLProvider(logger)
    
    // Configure connection
    config := provider.DatabaseConfig{
        Provider: "mysql",
        DSN:      os.Getenv("MYSQL_DSN"),
        Options: map[string]string{
            "tls":     "verify-identity",
            "charset": "utf8mb4",
        },
        MaxOpenConns:       20,
        MaxIdleConns:       5,
        ConnMaxLifetime:    30 * time.Minute,
        SlowQueryThreshold: 200 * time.Millisecond,
        LogLevel:          "warn",
    }
    
    // Connect to database
    if err := mysqlProvider.Connect(config); err != nil {
        log.Fatal("Failed to connect to MySQL:", err)
    }
    defer mysqlProvider.Close()
    
    // Use database
    db := mysqlProvider.GetDB()
    // ... your application code
}
```

## Configuration Options

### DSN (Data Source Name) Format

The MySQL provider uses standard MySQL DSN format with security enhancements:

**Basic Format:**
```
[user[:password]@][tcp[(host[:port])]]/database[?param1=value1&...&paramN=valueN]
```

**Examples:**

```bash
# Basic local connection
user:password@tcp(localhost:3306)/shelly_manager

# Remote connection with SSL
app:secure_pass@tcp(mysql.internal:3306)/production?tls=required

# Complete production DSN
app_user:${MYSQL_PASSWORD}@tcp(mysql.internal:3306)/shelly_manager?tls=verify-identity&charset=utf8mb4&timeout=10s
```

### Configuration Parameters

#### Core Database Settings

```yaml
database:
  provider: "mysql"                    # Required: Database provider type
  dsn: "connection_string"             # Required: MySQL connection string
  
  # Connection Pool Settings
  max_open_conns: 20                   # Maximum open connections (default: 20)
  max_idle_conns: 5                    # Maximum idle connections (default: 5)
  conn_max_lifetime: "30m"             # Connection lifetime (default: 30m)
  conn_max_idle_time: "5m"             # Idle connection timeout (default: 5m)
  
  # Performance Settings  
  slow_query_threshold: "200ms"        # Slow query threshold (default: 200ms)
  log_level: "warn"                    # Logging level: silent|error|warn|info
  
  # MySQL-Specific Options
  options:
    tls: "preferred"                   # SSL/TLS mode (default: preferred)
    charset: "utf8mb4"                 # Character set (default: utf8mb4)
    collation: "utf8mb4_unicode_ci"    # Collation (optional)
    timeout: "10s"                     # Connection timeout (default: 10s)
    readTimeout: "30s"                 # Read timeout (default: 30s)  
    writeTimeout: "30s"                # Write timeout (default: 30s)
    ca: "/path/to/ca.pem"              # CA certificate path (for TLS)
    cert: "/path/to/client.pem"        # Client certificate path (for TLS)
    key: "/path/to/client.key"         # Client key path (for TLS)
    serverName: "mysql.example.com"    # Server name for TLS verification
```

### SSL/TLS Configuration

#### Available TLS Modes

| Mode | Description | Security Level | Use Case |
|------|-------------|----------------|----------|
| `false` | Disable SSL | None | Development only |
| `true` | Enable SSL without verification | Low | Testing |
| `skip-verify` | SSL without certificate verification | Medium | Staging |
| `preferred` | Prefer SSL, fallback to non-SSL | Medium | **Default** |
| `required` | Require SSL connection | High | Production |
| `verify-ca` | Verify CA certificate | High | Production |
| `verify-identity` | Verify CA and hostname | **Highest** | Production |
| `custom` | Custom TLS configuration | Variable | Advanced |

#### SSL Configuration Examples

##### Development Environment
```yaml
database:
  provider: "mysql"
  dsn: "dev:devpass@tcp(localhost:3306)/shelly_dev"
  options:
    tls: "preferred"  # Default - secure but compatible
```

##### Production Environment  
```yaml
database:
  provider: "mysql"
  dsn: "app:${MYSQL_PASSWORD}@tcp(mysql.internal:3306)/production"
  options:
    tls: "verify-identity"
    ca: "/etc/ssl/certs/mysql-ca.pem"
    cert: "/etc/ssl/certs/mysql-client.pem"
    key: "/etc/ssl/private/mysql-client.key"
    serverName: "mysql.internal"
```

##### High-Security Environment
```yaml  
database:
  provider: "mysql"
  dsn: "secure_app:${MYSQL_PASSWORD}@tcp(mysql.secure.internal:3306)/secure_db"
  options:
    tls: "verify-identity"
    ca: "/secrets/mysql/ca.pem"
    cert: "/secrets/mysql/client.pem"
    key: "/secrets/mysql/client.key"
    serverName: "mysql.secure.internal"
    charset: "utf8mb4"
    collation: "utf8mb4_unicode_ci"
    timeout: "5s"
    readTimeout: "15s"
    writeTimeout: "15s"
```

## Environment-Specific Configurations

### Development Environment

**Characteristics:**
- Local MySQL instance or Docker container
- Relaxed security for development convenience
- Enhanced logging for debugging
- Shorter timeouts for faster feedback

```yaml
# config/development.yaml
database:
  provider: "mysql"
  dsn: "dev:devpass@tcp(localhost:3306)/shelly_dev"
  options:
    tls: "preferred"
    charset: "utf8mb4"
    timeout: "5s"
    readTimeout: "10s"
    writeTimeout: "10s"
  max_open_conns: 10
  max_idle_conns: 3
  conn_max_lifetime: "15m"
  conn_max_idle_time: "2m"
  slow_query_threshold: "100ms"
  log_level: "info"  # Verbose logging for development
```

**Docker Development Setup:**
```yaml
# docker-compose.dev.yml
version: '3.8'
services:
  shelly-manager-dev:
    build: .
    environment:
      - DATABASE_PROVIDER=mysql
      - MYSQL_HOST=mysql-dev
      - MYSQL_USER=dev
      - MYSQL_PASSWORD=devpass
      - MYSQL_DATABASE=shelly_dev
      - MYSQL_SSL_MODE=preferred
    depends_on:
      - mysql-dev

  mysql-dev:
    image: mysql:8.0
    environment:
      - MYSQL_ROOT_PASSWORD=rootpass
      - MYSQL_USER=dev
      - MYSQL_PASSWORD=devpass
      - MYSQL_DATABASE=shelly_dev
    ports:
      - "3306:3306"
    volumes:
      - mysql_dev_data:/var/lib/mysql

volumes:
  mysql_dev_data:
```

### Staging Environment

**Characteristics:**
- Production-like MySQL setup
- SSL enforcement with certificate validation
- Performance monitoring enabled
- Production-like connection settings

```yaml
# config/staging.yaml
database:
  provider: "mysql"
  dsn: "staging_app:${MYSQL_PASSWORD}@tcp(mysql.staging:3306)/shelly_staging"
  options:
    tls: "required"
    ca: "/etc/ssl/mysql-ca.pem" 
    charset: "utf8mb4"
    collation: "utf8mb4_unicode_ci"
    timeout: "10s"
    readTimeout: "30s"
    writeTimeout: "30s"
  max_open_conns: 20
  max_idle_conns: 5
  conn_max_lifetime: "30m"
  conn_max_idle_time: "5m"
  slow_query_threshold: "200ms"
  log_level: "warn"
```

### Production Environment

**Characteristics:**
- Highly available MySQL cluster
- Full SSL/TLS with certificate validation
- Optimized performance settings
- Comprehensive monitoring and alerting

```yaml
# config/production.yaml  
database:
  provider: "mysql"
  dsn: "prod_app:${MYSQL_PASSWORD}@tcp(mysql.internal:3306)/shelly_manager"
  options:
    tls: "verify-identity"
    ca: "/etc/ssl/certs/mysql-ca.pem"
    cert: "/etc/ssl/certs/mysql-client.pem"
    key: "/etc/ssl/private/mysql-client.key"
    serverName: "mysql.internal"
    charset: "utf8mb4"
    collation: "utf8mb4_unicode_ci"
    timeout: "10s"
    readTimeout: "30s"
    writeTimeout: "30s"
    application_name: "shelly-manager"
  max_open_conns: 25                   # Higher for production load
  max_idle_conns: 5
  conn_max_lifetime: "1h"              # Longer for production stability
  conn_max_idle_time: "10m"
  slow_query_threshold: "500ms"        # Production threshold
  log_level: "error"                   # Minimal logging for performance
```

## Deployment Scenarios

### Docker Deployment

#### Single Container Deployment
```yaml
# docker-compose.yml
version: '3.8'
services:
  shelly-manager:
    image: shelly-manager:latest
    environment:
      - DATABASE_PROVIDER=mysql
      - MYSQL_HOST=mysql
      - MYSQL_USER=shelly
      - MYSQL_PASSWORD=${MYSQL_PASSWORD}
      - MYSQL_DATABASE=shelly_manager
      - MYSQL_SSL_MODE=required
    depends_on:
      mysql:
        condition: service_healthy
    ports:
      - "8080:8080"

  mysql:
    image: mysql:8.0
    environment:
      - MYSQL_ROOT_PASSWORD=${MYSQL_ROOT_PASSWORD}
      - MYSQL_USER=shelly
      - MYSQL_PASSWORD=${MYSQL_PASSWORD}
      - MYSQL_DATABASE=shelly_manager
    ports:
      - "3306:3306"
    volumes:
      - mysql_data:/var/lib/mysql
      - ./ssl/:/etc/mysql/ssl/
    command: >
      --ssl-ca=/etc/mysql/ssl/ca.pem
      --ssl-cert=/etc/mysql/ssl/server-cert.pem
      --ssl-key=/etc/mysql/ssl/server-key.pem
      --require-secure-transport=ON
    healthcheck:
      test: ["CMD", "mysqladmin", "ping", "-h", "localhost"]
      timeout: 20s
      retries: 10

volumes:
  mysql_data:
```

#### Multi-Container Production Deployment
```yaml  
# docker-compose.prod.yml
version: '3.8'
services:
  shelly-manager:
    image: shelly-manager:${VERSION}
    deploy:
      replicas: 3
      resources:
        limits:
          memory: 512M
        reservations:
          memory: 256M
    environment:
      - DATABASE_PROVIDER=mysql
      - MYSQL_DSN=app_user:${MYSQL_PASSWORD}@tcp(mysql-primary:3306)/shelly_manager?tls=verify-identity
    secrets:
      - mysql_ca_cert
      - mysql_client_cert
      - mysql_client_key
    depends_on:
      - mysql-primary
    ports:
      - "8080:8080"

  mysql-primary:
    image: mysql:8.0
    environment:
      - MYSQL_ROOT_PASSWORD=${MYSQL_ROOT_PASSWORD}
      - MYSQL_USER=app_user
      - MYSQL_PASSWORD=${MYSQL_PASSWORD}
      - MYSQL_DATABASE=shelly_manager
    volumes:
      - mysql_primary_data:/var/lib/mysql
      - ./ssl/:/etc/mysql/ssl/
    command: >
      --ssl-ca=/etc/mysql/ssl/ca.pem
      --ssl-cert=/etc/mysql/ssl/server-cert.pem
      --ssl-key=/etc/mysql/ssl/server-key.pem
      --require-secure-transport=ON
      --binlog-format=ROW
    ports:
      - "3306:3306"

secrets:
  mysql_ca_cert:
    file: ./ssl/ca.pem
  mysql_client_cert:
    file: ./ssl/client-cert.pem
  mysql_client_key:
    file: ./ssl/client-key.pem

volumes:
  mysql_primary_data:
```

### Kubernetes Deployment

#### Configuration and Secrets
```yaml
# mysql-config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: mysql-config
data:
  database.yaml: |
    database:
      provider: "mysql"
      dsn: "app_user:$(MYSQL_PASSWORD)@tcp(mysql:3306)/shelly_manager"
      options:
        tls: "verify-identity"
        ca: "/etc/ssl/mysql/ca.pem"
        cert: "/etc/ssl/mysql/client.pem"
        key: "/etc/ssl/mysql/client.key"
        serverName: "mysql.default.svc.cluster.local"
        charset: "utf8mb4"
      max_open_conns: 25
      max_idle_conns: 5
      conn_max_lifetime: "1h"
---
apiVersion: v1
kind: Secret
metadata:
  name: mysql-secret
type: Opaque
data:
  password: $(echo -n "${MYSQL_PASSWORD}" | base64)
  ca.pem: $(cat ssl/ca.pem | base64 -w 0)
  client.pem: $(cat ssl/client.pem | base64 -w 0)
  client.key: $(cat ssl/client.key | base64 -w 0)
```

#### Application Deployment
```yaml
# shelly-manager-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: shelly-manager
spec:
  replicas: 3
  selector:
    matchLabels:
      app: shelly-manager
  template:
    metadata:
      labels:
        app: shelly-manager
    spec:
      containers:
      - name: shelly-manager
        image: shelly-manager:latest
        env:
        - name: MYSQL_PASSWORD
          valueFrom:
            secretKeyRef:
              name: mysql-secret
              key: password
        - name: DATABASE_PROVIDER
          value: "mysql"
        volumeMounts:
        - name: mysql-ssl
          mountPath: /etc/ssl/mysql
          readOnly: true
        - name: config
          mountPath: /etc/shelly-manager
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
      volumes:
      - name: mysql-ssl
        secret:
          secretName: mysql-secret
          items:
          - key: ca.pem
            path: ca.pem
          - key: client.pem
            path: client.pem
          - key: client.key
            path: client.key
            mode: 0600
      - name: config
        configMap:
          name: mysql-config
```

## Performance Tuning

### Connection Pool Optimization

#### Default MySQL Settings (Recommended)
```yaml
database:
  max_open_conns: 20      # Conservative for MySQL
  max_idle_conns: 5       # Minimal idle connections
  conn_max_lifetime: "30m" # Frequent rotation
  conn_max_idle_time: "5m" # Quick cleanup
```

#### High-Load Environment
```yaml
database:
  max_open_conns: 50      # Higher for high load
  max_idle_conns: 10      # More idle connections
  conn_max_lifetime: "1h"  # Longer for stability
  conn_max_idle_time: "10m" # Balanced cleanup
```

#### Low-Resource Environment
```yaml
database:
  max_open_conns: 10      # Lower resource usage
  max_idle_conns: 2       # Minimal idle
  conn_max_lifetime: "15m" # Frequent rotation
  conn_max_idle_time: "2m"  # Aggressive cleanup
```

### Performance Monitoring

#### Built-in Statistics
```go
// Get performance statistics
stats := mysqlProvider.GetStats()

fmt.Printf("Connections: %d open, %d in use, %d idle\n",
    stats.OpenConnections, stats.InUseConnections, stats.IdleConnections)
fmt.Printf("Queries: %d total, %d slow, %d failed\n",
    stats.TotalQueries, stats.SlowQueries, stats.FailedQueries)
fmt.Printf("Average latency: %v\n", stats.AverageLatency)
fmt.Printf("Database size: %d bytes\n", stats.DatabaseSize)
```

#### Health Check Integration
```go
// Perform health check
ctx := context.WithTimeout(context.Background(), 5*time.Second)
status := mysqlProvider.HealthCheck(ctx)

if !status.Healthy {
    log.Printf("Database unhealthy: %s", status.Error)
    log.Printf("Response time: %v", status.ResponseTime)
}

// Access detailed metrics
fmt.Printf("Connection count: %v\n", status.Details["connection_count"])
fmt.Printf("Total queries: %v\n", status.Details["total_queries"])
fmt.Printf("Database size: %v\n", status.Details["database_size"])
```

## Migration from SQLite

### Migration Strategy

The MySQL provider is designed as a drop-in replacement for the SQLite provider:

#### 1. Update Configuration
```yaml
# Before (SQLite)
database:
  provider: "sqlite"
  dsn: "/data/shelly-manager.db"

# After (MySQL)
database:
  provider: "mysql"
  dsn: "user:password@tcp(mysql:3306)/shelly_manager"
  options:
    tls: "preferred"
    charset: "utf8mb4"
```

#### 2. Code Changes (Minimal)
```go
// No code changes required - same interface
provider := provider.NewMySQLProvider(logger)  // Changed from NewSQLiteProvider
if err := provider.Connect(config); err != nil {
    log.Fatal(err)
}

// All other code remains the same
db := provider.GetDB()
tx, err := provider.BeginTransaction()
// ... rest of application code unchanged
```

#### 3. Data Migration (if needed)
```bash
# Export SQLite data
sqlite3 /data/shelly-manager.db .dump > backup.sql

# Import to MySQL (with adjustments)
mysql -u user -p shelly_manager < converted_backup.sql
```

### Migration Checklist

- [ ] Update database configuration
- [ ] Install MySQL server and configure SSL
- [ ] Create MySQL user and database  
- [ ] Migrate data from SQLite (if needed)
- [ ] Update connection pool settings
- [ ] Test application connectivity
- [ ] Update monitoring and alerting
- [ ] Validate SSL/TLS configuration
- [ ] Performance test with production load
- [ ] Update deployment scripts and documentation

## Troubleshooting

### Common Issues and Solutions

#### Connection Failures
```bash
# Error: failed to connect to MySQL database
# Solution: Check DSN format and credentials
export MYSQL_DSN="user:password@tcp(mysql:3306)/database?tls=preferred"

# Test connection manually
mysql -h mysql -u user -p database
```

#### SSL/TLS Issues
```bash
# Error: certificate verification failed
# Solution: Validate certificate paths and permissions
ls -la /etc/ssl/mysql/
chmod 644 /etc/ssl/mysql/ca.pem
chmod 644 /etc/ssl/mysql/client.pem  
chmod 600 /etc/ssl/mysql/client.key
```

#### Performance Issues
```bash
# Check connection pool statistics
curl http://localhost:8080/health/database

# Adjust connection pool settings
database:
  max_open_conns: 30    # Increase if needed
  max_idle_conns: 10    # Increase if needed
```

#### Migration Issues
```go
// Check migration errors
if err := provider.Migrate(&models.Device{}, &models.Config{}); err != nil {
    log.Printf("Migration failed: %v", err)
    // Check database permissions and schema compatibility
}
```

### Debug Configuration

```yaml
# Enhanced debugging configuration
database:
  provider: "mysql"
  dsn: "debug_user:debug_pass@tcp(localhost:3306)/debug_db"
  options:
    tls: "preferred"
    charset: "utf8mb4"
    timeout: "30s"      # Longer timeout for debugging
  log_level: "info"     # Verbose logging
  slow_query_threshold: "50ms"  # Lower threshold for debugging
```

This comprehensive usage guide provides all the information needed to successfully configure, deploy, and operate the MySQL database provider in various environments and scenarios.