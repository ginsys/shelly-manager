# PostgreSQL Troubleshooting Guide

## Overview

This guide provides comprehensive troubleshooting procedures for the PostgreSQL database provider in Shelly Manager, covering common issues, diagnostic procedures, and resolution strategies for production environments.

## Connection Issues

### Failed to Connect to PostgreSQL Database

#### Symptoms
```
ERROR: failed to connect to PostgreSQL database: dial tcp: connect: connection refused
```

#### Diagnostic Steps

**1. Check PostgreSQL Service Status**
```bash
# On PostgreSQL server
systemctl status postgresql
# or for Docker
docker ps | grep postgres

# Check PostgreSQL process
ps aux | grep postgres
```

**2. Verify Network Connectivity**
```bash
# Test basic connectivity
telnet postgres-host 5432
# or
nc -zv postgres-host 5432

# Check if PostgreSQL is listening
netstat -tlnp | grep 5432
```

**3. Check PostgreSQL Configuration**
```bash
# Verify listen_addresses
grep listen_addresses /etc/postgresql/*/main/postgresql.conf

# Check host-based authentication
cat /etc/postgresql/*/main/pg_hba.conf
```

#### Resolution Steps

**1. Start PostgreSQL Service**
```bash
# System service
systemctl start postgresql
systemctl enable postgresql

# Docker container
docker start postgres-container
```

**2. Fix Network Configuration**
```conf
# postgresql.conf
listen_addresses = '*'  # or specific IP addresses
port = 5432

# pg_hba.conf - Add appropriate host entries
host all all 10.0.0.0/8 md5
hostssl all all 0.0.0.0/0 md5
```

**3. Firewall Configuration**
```bash
# Allow PostgreSQL port
ufw allow 5432
# or
iptables -A INPUT -p tcp --dport 5432 -j ACCEPT
```

### SSL Connection Issues

#### Symptoms
```
ERROR: SSL is not enabled on the server
ERROR: SSL connection has been closed unexpectedly
ERROR: SSL root certificate not found
```

#### SSL Mode Troubleshooting

**1. Check SSL Configuration**
```go
// Check provider SSL configuration
p.logger.WithFields(map[string]any{
    "sslmode": options["sslmode"],
    "sslcert": options["sslcert"],
    "sslkey": options["sslkey"],
    "sslrootcert": options["sslrootcert"],
}).Debug("SSL configuration")
```

**2. Verify SSL Mode Compatibility**
```sql
-- Check if SSL is enabled on server
SHOW ssl;

-- Check SSL cipher and version
SELECT * FROM pg_stat_ssl WHERE pid = pg_backend_pid();
```

#### Resolution Steps

**1. Enable SSL on PostgreSQL Server**
```conf
# postgresql.conf
ssl = on
ssl_cert_file = '/etc/postgresql/certs/server.crt'
ssl_key_file = '/etc/postgresql/certs/server.key'
ssl_ca_file = '/etc/postgresql/certs/ca.crt'
```

**2. Fix SSL Certificate Issues**
```bash
# Check certificate files exist and are readable
ls -la /path/to/certs/
chmod 600 /path/to/certs/*.key
chmod 644 /path/to/certs/*.crt

# Verify certificate validity
openssl x509 -in /path/to/certs/client.crt -text -noout
openssl verify -CAfile /path/to/certs/ca.crt /path/to/certs/client.crt
```

**3. Adjust SSL Mode**
```yaml
database:
  options:
    sslmode: "prefer"  # or "allow" for testing
```

### Authentication Failures

#### Symptoms
```
ERROR: password authentication failed for user "shelly"
ERROR: role "shelly" does not exist
```

#### Diagnostic Steps

**1. Verify User Credentials**
```bash
# Test authentication manually
psql -h postgres-host -U shelly -d shelly_manager
```

**2. Check User Exists**
```sql
-- Connect as superuser and check
\du  -- List all users
SELECT rolname FROM pg_roles WHERE rolname = 'shelly';
```

#### Resolution Steps

**1. Create User if Missing**
```sql
-- Create user with proper privileges
CREATE USER shelly WITH PASSWORD 'secure_password';
GRANT CONNECT ON DATABASE shelly_manager TO shelly;
GRANT USAGE ON SCHEMA public TO shelly;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO shelly;
```

**2. Reset Password**
```sql
-- Reset user password
ALTER USER shelly WITH PASSWORD 'new_secure_password';
```

**3. Check pg_hba.conf**
```conf
# Add appropriate authentication method
host shelly_manager shelly 10.0.0.0/8 md5
hostssl shelly_manager shelly 0.0.0.0/0 md5
```

## Performance Issues

### Slow Query Performance

#### Symptoms
```
WARN: Query took longer than 200ms
ERROR: Query timeout after 5 minutes
```

#### Diagnostic Tools

**1. Enable Query Logging**
```yaml
database:
  slow_query_threshold: "100ms"
  log_level: "info"
```

**2. Query Analysis**
```sql
-- Enable pg_stat_statements
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- Find slow queries
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

-- Check query execution plan
EXPLAIN ANALYZE SELECT * FROM devices WHERE status = 'active';
```

#### Resolution Strategies

**1. Add Missing Indexes**
```sql
-- Create indexes for common queries
CREATE INDEX CONCURRENTLY idx_devices_status ON devices(status);
CREATE INDEX CONCURRENTLY idx_devices_last_seen ON devices(last_seen);
CREATE INDEX CONCURRENTLY idx_device_config_device_id ON device_configurations(device_id);
```

**2. Optimize Queries**
```go
// Use Select to limit columns
db.Select("id", "name", "status").Find(&devices)

// Use Preload for related data
db.Preload("Configurations").Find(&devices)

// Use raw queries for complex operations
db.Raw("SELECT * FROM devices WHERE last_seen > ?", 
    time.Now().Add(-24*time.Hour)).Find(&devices)
```

**3. Update Statistics**
```sql
-- Update table statistics for better query planning
ANALYZE;
ANALYZE devices;
```

### Connection Pool Exhaustion

#### Symptoms
```
ERROR: remaining connection slots are reserved for non-replication superuser connections
ERROR: sorry, too many clients already
```

#### Diagnostic Steps

**1. Check Current Connections**
```sql
-- Check current connection count
SELECT count(*) FROM pg_stat_activity;

-- Check connections by database
SELECT datname, count(*) 
FROM pg_stat_activity 
GROUP BY datname;

-- Check connection states
SELECT state, count(*) 
FROM pg_stat_activity 
GROUP BY state;
```

**2. Monitor Pool Statistics**
```go
// Check pool utilization
stats := provider.GetStats()
utilization := float64(stats.InUseConnections) / float64(maxOpenConns)
fmt.Printf("Pool utilization: %.1f%%\n", utilization*100)
```

#### Resolution Steps

**1. Increase Connection Limits**
```conf
# postgresql.conf
max_connections = 200  # Increase from default 100
```

**2. Optimize Pool Configuration**
```yaml
database:
  max_open_conns: 50      # Increase pool size
  max_idle_conns: 10      # Maintain more idle connections
  conn_max_lifetime: "1h" # Reduce lifetime to prevent stale connections
  conn_max_idle_time: "10m" # Clean up idle connections
```

**3. Find and Kill Long-Running Queries**
```sql
-- Find long-running queries
SELECT pid, now() - pg_stat_activity.query_start AS duration, query 
FROM pg_stat_activity 
WHERE (now() - pg_stat_activity.query_start) > interval '5 minutes';

-- Kill problematic queries
SELECT pg_terminate_backend(pid) FROM pg_stat_activity 
WHERE state = 'idle in transaction' 
AND now() - state_change > interval '10 minutes';
```

### Memory Issues

#### Symptoms
```
ERROR: out of memory
DETAIL: Failed on request of size X
```

#### Diagnostic Steps

**1. Check Memory Usage**
```bash
# System memory
free -h

# PostgreSQL memory usage
ps aux | grep postgres | awk '{sum+=$6} END {print sum/1024 "MB"}'
```

**2. Check PostgreSQL Memory Settings**
```sql
-- Check current memory settings
SHOW shared_buffers;
SHOW work_mem;
SHOW maintenance_work_mem;
SHOW effective_cache_size;
```

#### Resolution Steps

**1. Optimize Memory Settings**
```conf
# postgresql.conf - Adjust based on available RAM
shared_buffers = 256MB        # 25% of RAM
work_mem = 64MB              # Per-connection working memory
maintenance_work_mem = 256MB  # Maintenance operations
effective_cache_size = 2GB    # Available cache memory
```

**2. Optimize Application Queries**
```go
// Limit result set size
db.Limit(1000).Find(&devices)

// Process in batches
offset := 0
batchSize := 100
for {
    var batch []Device
    result := db.Offset(offset).Limit(batchSize).Find(&batch)
    if len(batch) == 0 {
        break
    }
    // Process batch
    offset += batchSize
}
```

## Migration Issues

### Schema Migration Failures

#### Symptoms
```
ERROR: migration failed: relation "devices" already exists
ERROR: column "new_column" of relation "devices" does not exist
```

#### Diagnostic Steps

**1. Check Migration State**
```sql
-- Check current schema version
SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1;

-- Check table structure
\d devices
\d device_configurations
```

**2. Check for Conflicting Data**
```sql
-- Check for data that might prevent migration
SELECT COUNT(*) FROM devices WHERE problematic_column IS NULL;
```

#### Resolution Steps

**1. Manual Schema Fix**
```sql
-- Add missing column
ALTER TABLE devices ADD COLUMN IF NOT EXISTS new_column VARCHAR(255);

-- Fix data types
ALTER TABLE devices ALTER COLUMN status TYPE VARCHAR(50);

-- Add constraints
ALTER TABLE devices ADD CONSTRAINT unique_ip UNIQUE(ip);
```

**2. Reset and Retry Migration**
```go
// Drop tables and retry (development only)
provider.DropTables(&Device{}, &DeviceConfiguration{})
provider.Migrate(&Device{}, &DeviceConfiguration{})
```

### Data Migration Issues

#### Symptoms
```
ERROR: duplicate key value violates unique constraint
ERROR: foreign key constraint violation
```

#### Resolution Steps

**1. Handle Duplicate Data**
```sql
-- Find duplicates
SELECT ip, COUNT(*) FROM devices GROUP BY ip HAVING COUNT(*) > 1;

-- Remove duplicates (keep latest)
DELETE FROM devices WHERE id IN (
    SELECT id FROM (
        SELECT id, ROW_NUMBER() OVER (
            PARTITION BY ip ORDER BY created_at DESC
        ) as rnum FROM devices
    ) t WHERE rnum > 1
);
```

**2. Fix Foreign Key Issues**
```sql
-- Find orphaned records
SELECT d.* FROM device_configurations d 
LEFT JOIN devices dev ON d.device_id = dev.id 
WHERE dev.id IS NULL;

-- Clean up orphaned records
DELETE FROM device_configurations WHERE device_id NOT IN (
    SELECT id FROM devices
);
```

## Logging and Monitoring Issues

### Missing or Inadequate Logs

#### Configuration for Better Logging

**1. Enable Comprehensive Logging**
```conf
# postgresql.conf
logging_collector = on
log_destination = 'stderr,csvlog'
log_directory = '/var/log/postgresql'
log_filename = 'postgresql-%Y-%m-%d_%H%M%S.log'
log_rotation_age = 1d
log_rotation_size = 100MB

# What to log
log_min_duration_statement = 1000  # Log queries > 1 second
log_connections = on
log_disconnections = on
log_checkpoints = on
log_lock_waits = on
log_statement = 'ddl'
log_line_prefix = '%t [%p]: user=%u,db=%d,app=%a,client=%h '
```

**2. Application Logging Configuration**
```yaml
database:
  log_level: "info"  # Increase verbosity
  slow_query_threshold: "100ms"  # Lower threshold for debugging
```

### Health Check Failures

#### Symptoms
```
ERROR: health check failed: connection refused
WARNING: slow health check response: 5.2s
```

#### Diagnostic Steps

**1. Manual Health Check**
```bash
# Test database connectivity
pg_isready -h postgres-host -p 5432 -U shelly

# Test application health endpoint
curl -v http://localhost:8080/health
```

**2. Check Health Check Implementation**
```go
// Add debugging to health check
func (p *PostgreSQLProvider) HealthCheck(ctx context.Context) HealthStatus {
    start := time.Now()
    
    p.logger.Debug("Starting health check")
    
    if err := p.Ping(); err != nil {
        p.logger.WithFields(map[string]any{
            "error": err.Error(),
            "duration": time.Since(start),
        }).Error("Health check ping failed")
        
        return HealthStatus{
            Healthy: false,
            Error: err.Error(),
            ResponseTime: time.Since(start),
            CheckedAt: time.Now(),
        }
    }
    
    p.logger.WithFields(map[string]any{
        "duration": time.Since(start),
    }).Debug("Health check completed successfully")
    
    return HealthStatus{
        Healthy: true,
        ResponseTime: time.Since(start),
        CheckedAt: time.Now(),
    }
}
```

#### Resolution Steps

**1. Fix Health Check Timeout**
```yaml
database:
  options:
    connect_timeout: "5"  # Reduce timeout for faster health checks
```

**2. Implement Health Check Retry Logic**
```go
func (p *PostgreSQLProvider) HealthCheckWithRetry(ctx context.Context, retries int) HealthStatus {
    for i := 0; i <= retries; i++ {
        status := p.HealthCheck(ctx)
        if status.Healthy {
            return status
        }
        
        if i < retries {
            time.Sleep(time.Duration(i+1) * time.Second)
        }
    }
    
    return p.HealthCheck(ctx) // Final attempt
}
```

## Emergency Recovery Procedures

### Database Corruption

#### Immediate Actions

**1. Stop All Connections**
```bash
# Stop application
systemctl stop shelly-manager

# Kill all PostgreSQL connections
sudo -u postgres psql -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = 'shelly_manager' AND pid <> pg_backend_pid();"
```

**2. Check Database Integrity**
```bash
# Check filesystem
sudo -u postgres /usr/lib/postgresql/*/bin/pg_resetwal --dry-run /var/lib/postgresql/*/main

# Check database consistency
sudo -u postgres psql -d shelly_manager -c "SELECT * FROM pg_stat_database WHERE datname = 'shelly_manager';"
```

**3. Restore from Backup**
```bash
# Stop PostgreSQL
systemctl stop postgresql

# Restore data directory from backup
rm -rf /var/lib/postgresql/*/main/*
tar -xzf /backups/postgres-backup-latest.tar.gz -C /var/lib/postgresql/*/main/

# Start PostgreSQL
systemctl start postgresql

# Verify integrity
sudo -u postgres psql -d shelly_manager -c "\dt"
```

### Complete System Recovery

#### Recovery Checklist

**1. Backup Current State**
```bash
# Even if corrupted, backup current state
pg_dump -h localhost -U shelly shelly_manager > /tmp/emergency-backup-$(date +%Y%m%d-%H%M%S).sql
```

**2. Fresh Installation**
```bash
# Remove corrupted installation
systemctl stop postgresql
apt remove --purge postgresql-*

# Reinstall PostgreSQL
apt update
apt install postgresql postgresql-contrib

# Restore configuration
cp /backups/postgresql.conf /etc/postgresql/*/main/
cp /backups/pg_hba.conf /etc/postgresql/*/main/

# Start service
systemctl start postgresql
systemctl enable postgresql
```

**3. Restore Data**
```bash
# Create database and user
sudo -u postgres psql << EOF
CREATE DATABASE shelly_manager;
CREATE USER shelly WITH PASSWORD 'password';
GRANT ALL PRIVILEGES ON DATABASE shelly_manager TO shelly;
EOF

# Restore from backup
psql -h localhost -U shelly -d shelly_manager < /backups/latest-backup.sql

# Run migrations if needed
cd /app && go run ./cmd/migrate/main.go
```

## Diagnostic Scripts

### Connection Diagnostic Script

```bash
#!/bin/bash
# postgres-diagnostic.sh

echo "=== PostgreSQL Connection Diagnostic ==="
echo "Date: $(date)"
echo

# Test basic connectivity
echo "1. Testing network connectivity..."
if timeout 5 nc -zv $POSTGRES_HOST 5432; then
    echo "✓ Network connectivity: OK"
else
    echo "✗ Network connectivity: FAILED"
fi

# Test authentication
echo "2. Testing authentication..."
if PGPASSWORD=$POSTGRES_PASSWORD timeout 10 psql -h $POSTGRES_HOST -U $POSTGRES_USER -d $POSTGRES_DB -c "SELECT 1;" > /dev/null 2>&1; then
    echo "✓ Authentication: OK"
else
    echo "✗ Authentication: FAILED"
fi

# Check SSL
echo "3. Testing SSL connection..."
if PGPASSWORD=$POSTGRES_PASSWORD timeout 10 psql "host=$POSTGRES_HOST user=$POSTGRES_USER dbname=$POSTGRES_DB sslmode=require" -c "SELECT 1;" > /dev/null 2>&1; then
    echo "✓ SSL connection: OK"
else
    echo "✗ SSL connection: FAILED"
fi

# Check application connectivity
echo "4. Testing application health..."
if curl -s -f http://localhost:8080/health > /dev/null; then
    echo "✓ Application health: OK"
else
    echo "✗ Application health: FAILED"
fi

echo
echo "=== Diagnostic Complete ==="
```

### Performance Diagnostic Script

```bash
#!/bin/bash
# postgres-performance.sh

echo "=== PostgreSQL Performance Diagnostic ==="

# Connection stats
echo "Connection Statistics:"
PGPASSWORD=$POSTGRES_PASSWORD psql -h $POSTGRES_HOST -U $POSTGRES_USER -d $POSTGRES_DB << EOF
SELECT 
    state,
    count(*) as connections
FROM pg_stat_activity 
WHERE datname = '$POSTGRES_DB'
GROUP BY state;
EOF

# Slow queries
echo "Recent Slow Queries:"
PGPASSWORD=$POSTGRES_PASSWORD psql -h $POSTGRES_HOST -U $POSTGRES_USER -d $POSTGRES_DB << EOF
SELECT 
    query,
    calls,
    total_time,
    mean_time
FROM pg_stat_statements 
WHERE mean_time > 100 
ORDER BY total_time DESC 
LIMIT 5;
EOF

# Database size
echo "Database Size:"
PGPASSWORD=$POSTGRES_PASSWORD psql -h $POSTGRES_HOST -U $POSTGRES_USER -d $POSTGRES_DB << EOF
SELECT pg_size_pretty(pg_database_size('$POSTGRES_DB')) as database_size;
EOF

echo "=== Performance Diagnostic Complete ==="
```

This comprehensive troubleshooting guide provides systematic approaches to diagnose and resolve common PostgreSQL issues in production environments.