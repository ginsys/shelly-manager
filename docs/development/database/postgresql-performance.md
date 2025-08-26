# PostgreSQL Performance Guide

## Overview

This guide covers performance optimization, monitoring, and tuning for the PostgreSQL database provider in Shelly Manager. It includes connection pool optimization, query performance tuning, and production monitoring strategies.

## Connection Pool Performance

### Pool Configuration Strategy

The PostgreSQL provider implements intelligent connection pool management optimized for PostgreSQL's connection model:

```go
// PostgreSQL-optimized defaults in configureConnectionPool()
maxOpenConns := p.config.MaxOpenConns
if maxOpenConns == 0 {
    maxOpenConns = 25 // PostgreSQL can handle more concurrent connections
}

connMaxLifetime := p.config.ConnMaxLifetime
if connMaxLifetime == 0 {
    connMaxLifetime = time.Hour // PostgreSQL connections can live longer
}
```

### Pool Sizing Guidelines

#### Calculating Optimal Pool Size

**Base Formula:**
```
optimal_pool_size = number_of_cpu_cores * workload_factor

Where workload_factor:
- I/O intensive workloads: 6-8
- CPU intensive workloads: 2-3
- Mixed workloads: 4-5
- Web applications: 4-6
```

#### Environment-Specific Pool Configurations

**Development Environment:**
```yaml
database:
  max_open_conns: 10          # Lower concurrency for dev
  max_idle_conns: 2           # Minimal resource usage
  conn_max_lifetime: "30m"    # Shorter lifetime for rapid iteration
  conn_max_idle_time: "5m"    # Quick cleanup
```

**Production Environment:**
```yaml
database:
  max_open_conns: 50          # Higher concurrency for production load
  max_idle_conns: 10          # Balance between reuse and resource consumption
  conn_max_lifetime: "2h"     # Longer lifetime for stability
  conn_max_idle_time: "15m"   # Reasonable idle timeout
```

**High-Traffic Environment:**
```yaml
database:
  max_open_conns: 100         # Maximum concurrency
  max_idle_conns: 25          # High idle count for rapid reuse
  conn_max_lifetime: "4h"     # Very long lifetime to minimize churn
  conn_max_idle_time: "30m"   # Extended idle time
```

### Connection Pool Monitoring

#### Built-in Pool Statistics

The provider tracks comprehensive pool metrics:

```go
func (p *PostgreSQLProvider) updateStats() {
    if !p.connected || p.db == nil {
        return
    }

    sqlDB, err := p.db.DB()
    if err != nil {
        return
    }

    stats := sqlDB.Stats()
    p.stats.OpenConnections = stats.OpenConnections      // Currently active
    p.stats.InUseConnections = stats.InUse               // Processing queries
    p.stats.IdleConnections = stats.Idle                 // Available for reuse
}
```

#### Pool Performance Metrics

**Key Performance Indicators:**
- **Pool Utilization**: `InUseConnections / MaxOpenConnections`
- **Connection Efficiency**: `IdleConnections / MaxIdleConnections`
- **Connection Churn**: Rate of connection creation/destruction
- **Wait Time**: Time spent waiting for available connections

**Optimal Ranges:**
- Pool Utilization: 60-80% during peak load
- Idle Connection Ratio: 20-40% of max open connections
- Connection Wait Time: <10ms average
- Connection Establishment Time: <100ms average

### Pool Optimization Strategies

#### Dynamic Pool Sizing

**Load-Based Adjustment:**
```go
func (p *PostgreSQLProvider) optimizePoolSize() {
    stats := p.GetStats()
    
    // If pool utilization is consistently high, consider increasing size
    utilization := float64(stats.InUseConnections) / float64(p.config.MaxOpenConns)
    
    if utilization > 0.8 {
        p.logger.WithFields(map[string]any{
            "current_utilization": utilization,
            "recommendation": "consider increasing max_open_conns",
        }).Warn("High connection pool utilization detected")
    }
    
    // If idle connections are consistently low, consider reducing idle count
    idleRatio := float64(stats.IdleConnections) / float64(p.config.MaxIdleConns)
    
    if idleRatio < 0.2 {
        p.logger.WithFields(map[string]any{
            "idle_ratio": idleRatio,
            "recommendation": "consider reducing max_idle_conns",
        }).Info("Low idle connection utilization detected")
    }
}
```

#### Connection Lifecycle Management

**Optimal Connection Settings:**
```yaml
database:
  # Balance connection reuse with resource management
  conn_max_lifetime: "2h"     # Prevent stale connections
  conn_max_idle_time: "15m"   # Clean up unused connections
  
  # PostgreSQL-specific timeout settings
  options:
    connect_timeout: "30"     # Connection establishment timeout
    statement_timeout: "300000"  # 5 minute query timeout
    idle_in_transaction_session_timeout: "600000"  # 10 minute idle transaction timeout
```

## Query Performance

### Slow Query Detection

#### Built-in Query Monitoring

The provider automatically tracks query performance:

```go
func (p *PostgreSQLProvider) createGormLogger() logger.Interface {
    return logger.New(
        log.New(&gormLogWriter{logger: p.logger}, "", 0),
        logger.Config{
            SlowThreshold:             p.config.SlowQueryThreshold,  // Default: 200ms
            LogLevel:                  logLevel,
            IgnoreRecordNotFoundError: true,
            Colorful:                  false,
        },
    )
}
```

#### Query Performance Metrics

**Automatic Statistics Collection:**
```go
// Query timing tracking
atomic.AddInt64(&p.queryCount, 1)
if duration > p.config.SlowQueryThreshold {
    atomic.AddInt64(&p.slowQueries, 1)
}
atomic.AddInt64(&p.totalLatency, int64(duration))
```

**Performance Thresholds:**
- **Fast Queries**: <50ms (simple selects, lookups)
- **Normal Queries**: 50-200ms (joins, aggregations)
- **Slow Queries**: >200ms (complex operations, large datasets)
- **Critical Queries**: >1000ms (requires immediate attention)

### Query Optimization Strategies

#### Index Optimization

**Automatic Index Recommendations:**
```sql
-- Enable PostgreSQL statistics collection
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- Find queries that would benefit from indexes
SELECT 
    query,
    calls,
    total_time,
    mean_time,
    rows
FROM pg_stat_statements 
WHERE mean_time > 100  -- Queries slower than 100ms
ORDER BY total_time DESC 
LIMIT 10;

-- Identify missing indexes
SELECT 
    schemaname,
    tablename,
    attname,
    n_distinct,
    correlation
FROM pg_stats 
WHERE schemaname = 'public' 
    AND n_distinct > 100  -- High cardinality columns
    AND correlation < 0.1; -- Poor correlation (good for indexing)
```

#### Connection-Level Optimizations

**PostgreSQL Session Optimization:**
```yaml
database:
  options:
    # Optimize for application workload
    work_mem: "256MB"                    # Memory for sorts and hashes
    maintenance_work_mem: "512MB"        # Memory for maintenance operations
    effective_cache_size: "4GB"         # Available memory for caching
    random_page_cost: "1.1"             # SSD-optimized (default 4.0 for HDD)
    
    # Connection-specific settings
    statement_timeout: "300000"          # 5 minute statement timeout
    lock_timeout: "30000"                # 30 second lock timeout
    idle_in_transaction_session_timeout: "600000"  # 10 minute idle transaction timeout
```

### GORM Performance Optimization

#### Efficient Query Patterns

**Optimized GORM Usage:**
```go
// Use Select to limit columns
db.Select("id", "name", "status").Find(&devices)

// Use preloading for related data
db.Preload("Configurations").Find(&devices)

// Use raw queries for complex operations
db.Raw("SELECT * FROM devices WHERE last_seen > ?", time.Now().Add(-24*time.Hour)).Find(&devices)

// Batch operations for better performance
db.CreateInBatches(devices, 100)
```

#### Query Caching Strategies

**Application-Level Caching:**
```go
type CachedProvider struct {
    *PostgreSQLProvider
    cache map[string]interface{}
    cacheMu sync.RWMutex
    cacheTTL time.Duration
}

func (c *CachedProvider) GetDeviceByID(id uint) (*Device, error) {
    c.cacheMu.RLock()
    if cached, ok := c.cache["device_"+strconv.Itoa(int(id))]; ok {
        c.cacheMu.RUnlock()
        return cached.(*Device), nil
    }
    c.cacheMu.RUnlock()
    
    // Cache miss - query database
    device := &Device{}
    if err := c.db.First(device, id).Error; err != nil {
        return nil, err
    }
    
    // Cache result
    c.cacheMu.Lock()
    c.cache["device_"+strconv.Itoa(int(id))] = device
    c.cacheMu.Unlock()
    
    return device, nil
}
```

## Performance Monitoring

### Real-Time Performance Metrics

#### Database Statistics Collection

The provider exposes comprehensive performance metrics:

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
}
```

#### Health Check Integration

**Comprehensive Health Monitoring:**
```go
func (p *PostgreSQLProvider) HealthCheck(ctx context.Context) HealthStatus {
    status := HealthStatus{
        CheckedAt: time.Now(),
        Details:   make(map[string]interface{}),
    }

    start := time.Now()
    
    if err := p.Ping(); err != nil {
        status.Healthy = false
        status.Error = err.Error()
        status.ResponseTime = time.Since(start)
        return status
    }

    status.Healthy = true
    status.ResponseTime = time.Since(start)

    // Add performance details
    stats := p.GetStats()
    status.Details["database_size"] = stats.DatabaseSize
    status.Details["total_queries"] = stats.TotalQueries
    status.Details["connection_count"] = stats.OpenConnections
    status.Details["average_latency"] = stats.AverageLatency.String()
    
    return status
}
```

### Monitoring Integration

#### Prometheus Metrics

**PostgreSQL Provider Metrics:**
```go
var (
    dbConnectionsActive = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "postgres_connections_active",
            Help: "Number of active PostgreSQL connections",
        },
        []string{"database", "host"},
    )
    
    dbConnectionsIdle = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "postgres_connections_idle",
            Help: "Number of idle PostgreSQL connections",
        },
        []string{"database", "host"},
    )
    
    dbQueryDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "postgres_query_duration_seconds",
            Help: "PostgreSQL query duration in seconds",
            Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
        },
        []string{"database", "operation"},
    )
    
    dbSlowQueries = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "postgres_slow_queries_total",
            Help: "Total number of slow PostgreSQL queries",
        },
        []string{"database", "threshold"},
    )
)

// Update metrics in statistics collection
func (p *PostgreSQLProvider) updatePrometheusMetrics() {
    stats := p.GetStats()
    
    dbConnectionsActive.WithLabelValues(p.database, p.host).Set(float64(stats.InUseConnections))
    dbConnectionsIdle.WithLabelValues(p.database, p.host).Set(float64(stats.IdleConnections))
    
    if stats.TotalQueries > 0 {
        avgLatency := float64(stats.AverageLatency) / float64(time.Second)
        dbQueryDuration.WithLabelValues(p.database, "average").Observe(avgLatency)
    }
    
    dbSlowQueries.WithLabelValues(p.database, "200ms").Add(float64(stats.SlowQueries))
}
```

#### Grafana Dashboard Configuration

**PostgreSQL Performance Dashboard:**
```json
{
  "dashboard": {
    "title": "Shelly Manager PostgreSQL Performance",
    "panels": [
      {
        "title": "Connection Pool Status",
        "type": "graph",
        "targets": [
          {
            "expr": "postgres_connections_active{database=\"shelly_manager\"}",
            "legendFormat": "Active Connections"
          },
          {
            "expr": "postgres_connections_idle{database=\"shelly_manager\"}",
            "legendFormat": "Idle Connections"
          }
        ]
      },
      {
        "title": "Query Performance",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(postgres_query_duration_seconds_sum[5m]) / rate(postgres_query_duration_seconds_count[5m])",
            "legendFormat": "Average Query Duration"
          },
          {
            "expr": "rate(postgres_slow_queries_total[5m])",
            "legendFormat": "Slow Queries per Second"
          }
        ]
      },
      {
        "title": "Database Size",
        "type": "singlestat",
        "targets": [
          {
            "expr": "postgres_database_size_bytes{database=\"shelly_manager\"}",
            "legendFormat": "Database Size"
          }
        ]
      }
    ]
  }
}
```

### Performance Alerting

#### Critical Performance Alerts

**Grafana Alert Rules:**
```yaml
groups:
- name: postgresql.performance
  rules:
  - alert: PostgreSQLHighConnectionUtilization
    expr: postgres_connections_active / postgres_connections_max > 0.8
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "High PostgreSQL connection utilization"
      description: "Connection pool utilization is {{ $value | humanizePercentage }}"

  - alert: PostgreSQLSlowQueries
    expr: rate(postgres_slow_queries_total[5m]) > 10
    for: 2m
    labels:
      severity: warning
    annotations:
      summary: "High rate of slow PostgreSQL queries"
      description: "{{ $value }} slow queries per second over the last 5 minutes"

  - alert: PostgreSQLQueryLatency
    expr: postgres_query_duration_seconds{quantile="0.95"} > 1
    for: 3m
    labels:
      severity: critical
    annotations:
      summary: "High PostgreSQL query latency"
      description: "95th percentile query latency is {{ $value }}s"

  - alert: PostgreSQLConnectionPoolExhaustion
    expr: postgres_connections_active >= postgres_connections_max
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "PostgreSQL connection pool exhausted"
      description: "All available connections are in use"
```

## Performance Benchmarking

### Built-in Benchmarking

The test suite includes comprehensive performance benchmarks:

```go
func BenchmarkConnectionEstablishment(b *testing.B) {
    provider := NewPostgreSQLProvider(logging.GetDefault())
    config := getTestConfig()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        if err := provider.Connect(config); err != nil {
            b.Fatal(err)
        }
        provider.Close()
    }
}

func BenchmarkCRUDOperations(b *testing.B) {
    provider := setupTestProvider(b)
    defer provider.Close()
    
    b.Run("Insert", func(b *testing.B) {
        b.ResetTimer()
        for i := 0; i < b.N; i++ {
            device := &TestDevice{Name: fmt.Sprintf("Device%d", i)}
            provider.GetDB().Create(device)
        }
    })
    
    b.Run("Select", func(b *testing.B) {
        b.ResetTimer()
        for i := 0; i < b.N; i++ {
            var devices []TestDevice
            provider.GetDB().Limit(100).Find(&devices)
        }
    })
    
    b.Run("Update", func(b *testing.B) {
        b.ResetTimer()
        for i := 0; i < b.N; i++ {
            provider.GetDB().Model(&TestDevice{}).Where("id = ?", i%1000).Update("status", "updated")
        }
    })
}
```

### Performance Testing

#### Load Testing Script

**PostgreSQL Load Test:**
```bash
#!/bin/bash
# postgres-load-test.sh - PostgreSQL performance testing

POSTGRES_HOST="localhost"
POSTGRES_DB="shelly_test"
POSTGRES_USER="shelly"
CONCURRENT_CONNECTIONS=50
TEST_DURATION=300  # 5 minutes

echo "Starting PostgreSQL load test..."
echo "Host: $POSTGRES_HOST"
echo "Database: $POSTGRES_DB"
echo "Concurrent connections: $CONCURRENT_CONNECTIONS"
echo "Duration: ${TEST_DURATION}s"

# pgbench initialization
pgbench -i -s 10 -h "$POSTGRES_HOST" -d "$POSTGRES_DB" -U "$POSTGRES_USER"

# Run load test
pgbench -c "$CONCURRENT_CONNECTIONS" \
        -j 4 \
        -T "$TEST_DURATION" \
        -h "$POSTGRES_HOST" \
        -d "$POSTGRES_DB" \
        -U "$POSTGRES_USER" \
        --progress=10 \
        --log

# Analyze results
echo "Load test completed. Results:"
echo "============================================="
cat pgbench_log.*

# Connection pool stress test
echo "Testing connection pool limits..."
for i in $(seq 1 $((CONCURRENT_CONNECTIONS + 10))); do
    psql -h "$POSTGRES_HOST" -d "$POSTGRES_DB" -U "$POSTGRES_USER" \
         -c "SELECT pg_sleep(30);" &
done

wait
echo "Connection pool stress test completed."
```

### Performance Tuning Recommendations

#### PostgreSQL Server Optimization

**postgresql.conf Optimizations:**
```conf
# Memory Settings
shared_buffers = 256MB              # 25% of system RAM
effective_cache_size = 1GB          # 50-75% of system RAM
work_mem = 64MB                     # Memory for sorts and hashes
maintenance_work_mem = 256MB        # Memory for maintenance operations

# Connection Settings
max_connections = 100               # Adjust based on pool configuration
shared_preload_libraries = 'pg_stat_statements'

# Performance Settings
random_page_cost = 1.1              # SSD-optimized (4.0 for HDD)
effective_io_concurrency = 200      # SSD concurrent I/O capacity
checkpoint_completion_target = 0.9   # Spread checkpoint I/O
wal_buffers = 16MB                  # WAL buffer size
default_statistics_target = 100     # Statistics detail level

# Logging (for performance analysis)
log_min_duration_statement = 1000   # Log queries > 1 second
log_checkpoints = on
log_connections = on
log_disconnections = on
log_lock_waits = on
```

#### Application-Level Optimizations

**Connection Pool Tuning:**
```yaml
database:
  # Optimize for your workload
  max_open_conns: 50                # Match application concurrency
  max_idle_conns: 10                # 20% of max_open_conns
  conn_max_lifetime: "2h"           # Balance between reuse and freshness
  conn_max_idle_time: "15m"         # Clean up idle connections
  
  # Query performance
  slow_query_threshold: "200ms"     # Detect performance issues early
  
  # PostgreSQL-specific optimizations
  options:
    application_name: "shelly-manager"
    connect_timeout: "30"
    statement_timeout: "300000"     # 5 minutes
```

This comprehensive performance guide ensures optimal PostgreSQL performance for the Shelly Manager system across all deployment scenarios.