# Database Provider Documentation

## Overview

This directory contains comprehensive documentation for the database provider implementations in Shelly Manager. The database abstraction layer supports multiple database backends with enterprise-grade capabilities including SSL/TLS security, connection pooling, performance monitoring, and production-ready features.

## Supported Database Providers

### 🐘 [PostgreSQL Provider](./database/)
**Enterprise-grade relational database with advanced features**
- Complete PostgreSQL documentation suite (6 comprehensive guides)
- SSL/TLS security with certificate validation
- Connection pooling with PostgreSQL-optimized settings
- Performance monitoring and health checking
- Zero-downtime migration from SQLite

**Documentation:**
- [Technical Architecture](./database/postgresql-architecture.md)
- [Configuration Guide](./database/postgresql-configuration.md)
- [Security Guide](./database/postgresql-security.md)
- [Performance Guide](./database/postgresql-performance.md)
- [Migration Guide](./database/postgresql-migration-guide.md)
- [Troubleshooting Guide](./database/postgresql-troubleshooting.md)
- [API Reference](./database/postgresql-api-reference.md)

### 🐬 MySQL Provider
**Scalable relational database optimized for performance**
- Complete MySQL documentation with security-first architecture
- Comprehensive SSL/TLS support with all MySQL TLS modes
- MySQL-specific connection pool optimization
- Injection prevention and credential protection
- Production-ready with extensive test coverage (65+ tests)

**Documentation:**
- [Technical Architecture](./mysql-provider-architecture.md)
- [Security Guide](./mysql-provider-security.md)
- [Usage Guide](./mysql-provider-usage.md)
- [Testing Guide](./mysql-provider-testing.md)

### 🗃️ SQLite Provider
**Lightweight embedded database for development and single-node deployments**
- File-based database with zero configuration
- Built-in backup and restore capabilities
- Development and testing friendly
- Migration path to PostgreSQL/MySQL

## Provider Selection Guide

### Development Environment
**Recommended**: SQLite → MySQL → PostgreSQL
- **SQLite**: Quick setup, zero configuration
- **MySQL**: Production-like testing with moderate complexity
- **PostgreSQL**: Full feature testing with advanced SQL features

### Production Environment
**Recommended**: PostgreSQL → MySQL → SQLite
- **PostgreSQL**: Best for complex applications with advanced features
- **MySQL**: Excellent for high-performance, high-availability deployments
- **SQLite**: Only for single-node applications with light load

### Migration Path
**Typical Evolution**: SQLite → MySQL/PostgreSQL
- Start with SQLite for rapid development
- Migrate to MySQL for scalability and performance
- Choose PostgreSQL for advanced features and complex queries

## Quick Comparison

| Feature | SQLite | MySQL | PostgreSQL |
|---------|--------|-------|------------|
| **Setup Complexity** | None | Low | Medium |
| **Performance** | Good | Excellent | Excellent |
| **Scalability** | Limited | High | High |
| **SSL/TLS** | N/A | ✅ Full Support | ✅ Full Support |
| **Connection Pooling** | N/A | ✅ Optimized | ✅ Optimized |
| **Transactions** | ✅ Basic | ✅ ACID | ✅ ACID |
| **Concurrent Writes** | Limited | Excellent | Excellent |
| **Backup/Restore** | File Copy | ✅ Advanced | ✅ Advanced |
| **High Availability** | No | ✅ Clustering | ✅ Replication |
| **Cloud Ready** | No | ✅ Yes | ✅ Yes |

## Implementation Features

### Security Features
**All Providers Include:**
- ✅ Credential sanitization in logs and error messages
- ✅ Input validation and injection prevention
- ✅ Secure error handling with information protection
- ✅ Thread-safe operations with proper synchronization

**MySQL & PostgreSQL Additional:**
- ✅ SSL/TLS encryption with certificate validation
- ✅ MySQL TLS modes false, true, skip-verify and preferred (`true` verifies the chain and host name against the system trust store); a CA or client certificate by path is not supported
- ✅ PostgreSQL `sslmode` values, including verify-ca and verify-full, with `sslrootcert`, `sslcert` and `sslkey`
- ✅ Connection timeout and resource protection

### Performance Features
**All Providers Include:**
- ✅ Performance statistics collection
- ✅ Health checking with detailed metrics
- ✅ Query performance monitoring
- ✅ Connection pool management (MySQL/PostgreSQL)

**Advanced Performance (MySQL/PostgreSQL):**
- ✅ Database-specific connection pool optimization
- ✅ Real-time performance metrics
- ✅ Slow query detection and monitoring
- ✅ Resource usage optimization

### Developer Experience
**All Providers Include:**
- ✅ Unified DatabaseProvider interface
- ✅ Drop-in replacement capability
- ✅ Comprehensive error handling
- ✅ Structured logging with configurable levels

**Enhanced Developer Experience:**
- ✅ Complete API documentation with examples
- ✅ Configuration validation and helpful error messages
- ✅ Extensive test coverage (65+ tests for MySQL)
- ✅ Docker and Kubernetes deployment examples

## Getting Started

### 1. Choose Your Provider

```yaml
# SQLite (Development)
database:
  provider: "sqlite"
  dsn: "/data/shelly-manager.db"

# MySQL (Production)
database:
  provider: "mysql"  
  dsn: "user:password@tcp(mysql:3306)/shelly_manager"
  options:
    tls: "preferred"

# PostgreSQL (Advanced Features)  
database:
  provider: "postgresql"
  dsn: "postgres://user:password@postgres:5432/shelly_manager?sslmode=require"
```

### 2. Code Integration

```go
import "github.com/ginsys/shelly-manager/internal/database/provider"

// All providers implement the same interface
var dbProvider provider.DatabaseProvider

switch config.Provider {
case "sqlite":
    dbProvider = provider.NewSQLiteProvider(logger)
case "mysql":
    dbProvider = provider.NewMySQLProvider(logger)
case "postgresql":
    dbProvider = provider.NewPostgreSQLProvider(logger)
}

// Same usage pattern for all providers
if err := dbProvider.Connect(config); err != nil {
    log.Fatal(err)
}
defer dbProvider.Close()

db := dbProvider.GetDB()
// Use GORM as normal...
```

### 3. Environment Configuration

```bash
# Development
export SHELLY_DATABASE_PROVIDER=sqlite
export SHELLY_DATABASE_DSN="/tmp/dev.db"

# Production MySQL (the MySQL provider takes its credentials in the DSN)
export SHELLY_DATABASE_PROVIDER=mysql
export SHELLY_DATABASE_DSN="app:${MYSQL_PASSWORD}@tcp(mysql:3306)/production"

# Production PostgreSQL (credentials are kept out of the DSN)
export SHELLY_DATABASE_PROVIDER=postgresql
export SHELLY_DATABASE_DSN="postgres://postgres:5432/production?sslmode=require"
export SHELLY_DATABASE_USERNAME=app
export SHELLY_DATABASE_PASSWORD="${PG_PASSWORD}"
```

The application reads `SHELLY_` plus the config key, with `.` written as `_`. `SHELLY_DATABASE_USERNAME`
and `SHELLY_DATABASE_PASSWORD` cannot be combined with credentials in the DSN; the PostgreSQL provider refuses
a start that does both. The MySQL provider refuses `username` and `password` and reads the DSN only.

## Migration Between Providers

### SQLite → MySQL
```go
// 1. Export SQLite data
sqliteProvider := provider.NewSQLiteProvider(logger)
// ... export logic

// 2. Import to MySQL
mysqlProvider := provider.NewMySQLProvider(logger)
// ... import logic
```

### SQLite → PostgreSQL
```go
// 1. Export SQLite data
sqliteProvider := provider.NewSQLiteProvider(logger)
// ... export logic  

// 2. Import to PostgreSQL
pgProvider := provider.NewPostgreSQLProvider(logger)
// ... import logic
```

### Configuration Migration
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
  max_open_conns: 20

# After (PostgreSQL)
database:
  provider: "postgresql"
  dsn: "postgres://user:password@postgres:5432/shelly_manager?sslmode=require"  
  max_open_conns: 25
```

## Production Deployment

### Docker Deployment
```yaml
# docker-compose.yml
version: '3.8'
services:
  app:
    image: shelly-manager:latest
    environment:
      - SHELLY_DATABASE_PROVIDER=${DB_PROVIDER:-mysql}
      - SHELLY_DATABASE_DSN=${DATABASE_DSN}
    depends_on:
      - database

  database:
    image: ${DB_IMAGE:-mysql:8.0}
    environment:
      # Database-specific environment variables
    volumes:
      - db_data:/var/lib/${DB_TYPE:-mysql}

volumes:
  db_data:
```

### Kubernetes Deployment
```yaml
# configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: database-config
data:
  provider: "mysql"  # or "postgresql"
  
---
# secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: database-secret
type: Opaque
data:
  dsn: $(echo -n "${DATABASE_DSN}" | base64)
```

## Monitoring and Health Checking

### Health Check Endpoints
```go
// All providers support health checking
status := provider.HealthCheck(context.Background())

if !status.Healthy {
    log.Printf("Database unhealthy: %s", status.Error)
}

fmt.Printf("Response time: %v\n", status.ResponseTime)
fmt.Printf("Details: %+v\n", status.Details)
```

### Performance Metrics
```go
// Get performance statistics
stats := provider.GetStats()

fmt.Printf("Connections: %d open, %d in use, %d idle\n",
    stats.OpenConnections, stats.InUseConnections, stats.IdleConnections)
fmt.Printf("Queries: %d total, %d slow, %d failed\n", 
    stats.TotalQueries, stats.SlowQueries, stats.FailedQueries)
fmt.Printf("Average latency: %v\n", stats.AverageLatency)
```

### Monitoring Integration
```yaml
# Prometheus metrics endpoint
GET /metrics

# Health check endpoint  
GET /health/database

# Statistics endpoint
GET /api/v1/database/stats
```

## Testing

### Unit Tests
```bash
# Test all providers
go test ./internal/database/provider -v

# Test specific provider
go test ./internal/database/provider -v -run MySQL
go test ./internal/database/provider -v -run PostgreSQL
```

### Integration Tests
```bash
# MySQL integration tests
MYSQL_TEST_DSN="test:test@tcp(localhost:3306)/test" \
INTEGRATION_TESTS=true \
go test ./internal/database/provider -v -run Integration

# PostgreSQL integration tests  
POSTGRES_TEST_DSN="postgres://test:test@localhost:5432/test?sslmode=disable" \
INTEGRATION_TESTS=true \
go test ./internal/database/provider -v -run Integration
```

### Security Tests
```bash
# Run security-focused tests
go test ./internal/database/provider -v -run Security

# Test credential protection
go test ./internal/database/provider -v -run Credential

# Test SSL/TLS validation
go test ./internal/database/provider -v -run SSL
```

## Documentation Navigation

### For New Projects
1. **Start Here**: Choose your database provider based on requirements
2. **SQLite**: Simple file-based database for development
3. **MySQL**: [Usage Guide](./mysql-provider-usage.md) for production deployments
4. **PostgreSQL**: [Database Documentation](./database/) for advanced features

### For Security Requirements
1. **MySQL**: [Security Guide](./mysql-provider-security.md) - SSL/TLS, credentials and how the DSN is read
2. **PostgreSQL**: [Security Guide](./database/postgresql-security.md) - Advanced security features

### For Performance Optimization
1. **MySQL**: [Usage Guide](./mysql-provider-usage.md#performance-tuning) - Connection pool optimization
2. **PostgreSQL**: [Performance Guide](./database/postgresql-performance.md) - Advanced performance tuning

### For Production Deployment
1. **MySQL**: [Usage Guide](./mysql-provider-usage.md#deployment-scenarios) - Docker and Kubernetes
2. **PostgreSQL**: [Configuration Guide](./database/postgresql-configuration.md) - Production settings

### For Troubleshooting
1. **MySQL**: [Usage Guide](./mysql-provider-usage.md#troubleshooting) - Common issues and solutions
2. **PostgreSQL**: [Troubleshooting Guide](./database/postgresql-troubleshooting.md) - Comprehensive problem resolution

## Contributing

When adding new database providers or enhancing existing ones:

1. **Follow Interface**: Implement complete `DatabaseProvider` interface
2. **Security First**: Include comprehensive input validation and credential protection  
3. **Test Coverage**: Achieve >90% coverage for critical paths
4. **Documentation**: Provide complete documentation suite
5. **Performance**: Include benchmark tests and optimization

### Documentation Standards
- **Architecture**: Technical design and implementation details
- **Security**: Comprehensive security features and configuration
- **Usage**: Practical configuration and deployment examples
- **Testing**: Complete test coverage and execution instructions

This database provider documentation provides everything needed to successfully implement, deploy, and maintain database operations across different environments and requirements.