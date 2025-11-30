# PostgreSQL Migration Guide

## Overview

This guide provides comprehensive instructions for migrating from SQLite to PostgreSQL in Shelly Manager, including data migration strategies, configuration updates, deployment considerations, and rollback procedures.

## Migration Prerequisites

### System Requirements

#### PostgreSQL Server Requirements
- **PostgreSQL Version**: 12.0 or higher (recommended: 15.0+)
- **Memory**: Minimum 2GB RAM (4GB+ recommended for production)
- **Storage**: SSD recommended for optimal performance
- **Network**: Reliable network connection between application and database

#### Application Requirements
- **Shelly Manager**: Version with PostgreSQL provider support
- **Go Version**: 1.19 or higher
- **GORM**: v1.25.0 or higher with PostgreSQL driver

### Pre-Migration Checklist

```bash
# 1. Verify current SQLite database
sqlite3 /path/to/shelly.db ".schema" > schema_backup.sql
sqlite3 /path/to/shelly.db ".dump" > data_backup.sql

# 2. Check data size
du -h /path/to/shelly.db

# 3. Verify PostgreSQL connectivity
pg_isready -h postgres-host -p 5432 -U postgres

# 4. Test application with PostgreSQL (using test database)
POSTGRES_DSN="postgres://test:test@localhost:5432/test_db" go test ./...
```

## Migration Strategies

### Strategy 1: Zero-Downtime Migration (Recommended for Production)

This approach maintains service availability during migration using parallel sync.

#### Phase 1: Preparation

**1. Set up PostgreSQL Instance**
```yaml
# docker-compose.postgresql.yml
version: '3.8'
services:
  postgres:
    image: postgres:15
    environment:
      POSTGRES_USER: shelly
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
      POSTGRES_DB: shelly_manager
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./postgresql.conf:/etc/postgresql/postgresql.conf
    ports:
      - "5432:5432"
    command: postgres -c config_file=/etc/postgresql/postgresql.conf

volumes:
  postgres_data:
```

**2. Configure Dual Database Setup**
```yaml
# shelly-manager.migration.yaml
database:
  # Primary (current SQLite)
  provider: "sqlite"
  dsn: "file:./data/shelly.db?_foreign_keys=on"
  
  # Migration target (PostgreSQL)
  migration_target:
    provider: "postgresql"
    dsn: "postgres://shelly:${POSTGRES_PASSWORD}@postgres:5432/shelly_manager?sslmode=require"
    max_open_conns: 25
    max_idle_conns: 5
```

#### Phase 2: Schema Migration

**1. Export SQLite Schema**
```bash
#!/bin/bash
# export-sqlite-schema.sh

SQLITE_DB="./data/shelly.db"
OUTPUT_DIR="./migration"

mkdir -p "$OUTPUT_DIR"

# Extract table definitions
sqlite3 "$SQLITE_DB" ".schema" > "$OUTPUT_DIR/sqlite_schema.sql"

# Extract table data counts
sqlite3 "$SQLITE_DB" "
SELECT 
    name, 
    (SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=t.name) as table_count
FROM sqlite_master t WHERE type='table'
" > "$OUTPUT_DIR/table_counts.txt"

echo "Schema exported to $OUTPUT_DIR/"
```

**2. Create PostgreSQL Schema**
```go
// cmd/migrate/main.go
package main

import (
    "log"
    "github.com/ginsys/shelly-manager/internal/database/provider"
    "github.com/ginsys/shelly-manager/internal/database"
    "github.com/ginsys/shelly-manager/internal/logging"
)

func main() {
    logger := logging.GetDefault()
    
    // Connect to PostgreSQL
    pgProvider := provider.NewPostgreSQLProvider(logger)
    config := provider.DatabaseConfig{
        Provider: "postgresql",
        DSN: os.Getenv("POSTGRES_DSN"),
        MaxOpenConns: 10,
    }
    
    if err := pgProvider.Connect(config); err != nil {
        log.Fatal("Failed to connect to PostgreSQL:", err)
    }
    defer pgProvider.Close()
    
    // Run migrations (create tables)
    if err := pgProvider.Migrate(
        &database.Device{},
        &database.DeviceConfiguration{},
        &database.DiscoveredDevice{},
        &database.SyncResult{},
        // Add all your models here
    ); err != nil {
        log.Fatal("Migration failed:", err)
    }
    
    log.Println("PostgreSQL schema migration completed successfully")
}
```

#### Phase 3: Data Migration

**1. Create Migration Tool**
```go
// cmd/migrate-data/main.go
package main

import (
    "context"
    "fmt"
    "log"
    "time"
    
    "github.com/ginsys/shelly-manager/internal/database/provider"
    "github.com/ginsys/shelly-manager/internal/database"
)

type DataMigrator struct {
    source *provider.SQLiteProvider
    target *provider.PostgreSQLProvider
    batchSize int
}

func (m *DataMigrator) MigrateTable(tableName string, model interface{}) error {
    log.Printf("Starting migration of table: %s", tableName)
    
    // Get total count
    var totalCount int64
    m.source.GetDB().Model(model).Count(&totalCount)
    log.Printf("Total records to migrate: %d", totalCount)
    
    // Migrate in batches
    offset := 0
    migrated := int64(0)
    
    for migrated < totalCount {
        var records []interface{}
        
        // Fetch batch from SQLite
        result := m.source.GetDB().
            Limit(m.batchSize).
            Offset(offset).
            Find(&records)
            
        if result.Error != nil {
            return fmt.Errorf("failed to fetch batch: %w", result.Error)
        }
        
        if len(records) == 0 {
            break
        }
        
        // Insert batch into PostgreSQL
        if err := m.target.GetDB().CreateInBatches(records, m.batchSize).Error; err != nil {
            return fmt.Errorf("failed to insert batch: %w", err)
        }
        
        migrated += int64(len(records))
        offset += m.batchSize
        
        log.Printf("Migrated %d/%d records (%.1f%%)", 
            migrated, totalCount, float64(migrated)/float64(totalCount)*100)
    }
    
    log.Printf("Completed migration of table: %s", tableName)
    return nil
}

func main() {
    // Initialize providers
    sqliteProvider := provider.NewSQLiteProvider(logging.GetDefault())
    pgProvider := provider.NewPostgreSQLProvider(logging.GetDefault())
    
    // Connect to both databases
    sqliteConfig := provider.DatabaseConfig{
        Provider: "sqlite",
        DSN: "file:./data/shelly.db?_foreign_keys=on",
    }
    
    pgConfig := provider.DatabaseConfig{
        Provider: "postgresql", 
        DSN: os.Getenv("POSTGRES_DSN"),
        MaxOpenConns: 10,
    }
    
    if err := sqliteProvider.Connect(sqliteConfig); err != nil {
        log.Fatal("Failed to connect to SQLite:", err)
    }
    defer sqliteProvider.Close()
    
    if err := pgProvider.Connect(pgConfig); err != nil {
        log.Fatal("Failed to connect to PostgreSQL:", err)
    }
    defer pgProvider.Close()
    
    migrator := &DataMigrator{
        source: sqliteProvider,
        target: pgProvider,
        batchSize: 1000,
    }
    
    // Migrate tables in dependency order
    tables := []struct{
        name string
        model interface{}
    }{
        {"devices", &database.Device{}},
        {"device_configurations", &database.DeviceConfiguration{}},
        {"discovered_devices", &database.DiscoveredDevice{}},
        {"sync_results", &database.SyncResult{}},
    }
    
    for _, table := range tables {
        if err := migrator.MigrateTable(table.name, table.model); err != nil {
            log.Fatal("Migration failed:", err)
        }
    }
    
    log.Println("Data migration completed successfully")
}
```

**2. Execute Data Migration**
```bash
#!/bin/bash
# migrate-data.sh

set -e

echo "Starting data migration from SQLite to PostgreSQL..."

# Set environment variables
export POSTGRES_DSN="postgres://shelly:${POSTGRES_PASSWORD}@localhost:5432/shelly_manager?sslmode=require"
export SQLITE_DSN="file:./data/shelly.db?_foreign_keys=on"

# Run schema migration first
echo "Creating PostgreSQL schema..."
go run ./cmd/migrate/main.go

# Run data migration
echo "Migrating data..."
go run ./cmd/migrate-data/main.go

# Verify migration
echo "Verifying migration..."
go run ./cmd/verify-migration/main.go

echo "Migration completed successfully!"
```

#### Phase 4: Verification and Cutover

**1. Data Verification Tool**
```go
// cmd/verify-migration/main.go
package main

import (
    "fmt"
    "log"
    "os"
)

type MigrationVerifier struct {
    sqlite *provider.SQLiteProvider
    postgres *provider.PostgreSQLProvider
}

func (v *MigrationVerifier) VerifyTableCounts() error {
    tables := []string{"devices", "device_configurations", "discovered_devices", "sync_results"}
    
    for _, table := range tables {
        var sqliteCount, pgCount int64
        
        v.sqlite.GetDB().Table(table).Count(&sqliteCount)
        v.postgres.GetDB().Table(table).Count(&pgCount)
        
        if sqliteCount != pgCount {
            return fmt.Errorf("count mismatch for table %s: sqlite=%d, postgres=%d", 
                table, sqliteCount, pgCount)
        }
        
        log.Printf("✓ Table %s: %d records verified", table, pgCount)
    }
    
    return nil
}

func (v *MigrationVerifier) VerifyDataIntegrity() error {
    // Verify specific records
    var sqliteDevice, pgDevice database.Device
    
    v.sqlite.GetDB().First(&sqliteDevice)
    v.postgres.GetDB().First(&pgDevice)
    
    if sqliteDevice.Name != pgDevice.Name || 
       sqliteDevice.IP != pgDevice.IP ||
       !sqliteDevice.CreatedAt.Equal(pgDevice.CreatedAt) {
        return fmt.Errorf("data integrity check failed for device %d", sqliteDevice.ID)
    }
    
    log.Println("✓ Data integrity verified")
    return nil
}

func main() {
    // Initialize and verify migration
    verifier := &MigrationVerifier{
        sqlite: initSQLiteProvider(),
        postgres: initPostgreSQLProvider(),
    }
    
    if err := verifier.VerifyTableCounts(); err != nil {
        log.Fatal("Count verification failed:", err)
    }
    
    if err := verifier.VerifyDataIntegrity(); err != nil {
        log.Fatal("Integrity verification failed:", err)
    }
    
    log.Println("Migration verification completed successfully!")
}
```

**2. Application Cutover**
```bash
#!/bin/bash
# cutover.sh - Switch to PostgreSQL

# Stop application
systemctl stop shelly-manager

# Backup current configuration
cp configs/shelly-manager.yaml configs/shelly-manager.sqlite.yaml

# Update configuration to use PostgreSQL
cat > configs/shelly-manager.yaml << EOF
database:
  provider: "postgresql"
  dsn: "postgres://shelly:\${POSTGRES_PASSWORD}@postgres:5432/shelly_manager?sslmode=require"
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
EOF

# Start application with PostgreSQL
systemctl start shelly-manager

# Verify application health
sleep 10
curl -f http://localhost:8080/health || {
    echo "Health check failed, rolling back..."
    systemctl stop shelly-manager
    cp configs/shelly-manager.sqlite.yaml configs/shelly-manager.yaml
    systemctl start shelly-manager
    exit 1
}

echo "Cutover completed successfully!"
```

### Strategy 2: Maintenance Window Migration (Simpler Approach)

For environments where brief downtime is acceptable.

#### Step 1: Prepare PostgreSQL
```bash
#!/bin/bash
# prepare-postgres.sh

# Start PostgreSQL
docker-compose -f docker-compose.postgresql.yml up -d postgres

# Wait for PostgreSQL to be ready
until pg_isready -h localhost -p 5432 -U shelly; do
    echo "Waiting for PostgreSQL..."
    sleep 2
done

echo "PostgreSQL is ready"
```

#### Step 2: Stop Application and Migrate
```bash
#!/bin/bash
# maintenance-migration.sh

echo "Starting maintenance window migration..."

# 1. Stop application
systemctl stop shelly-manager

# 2. Backup SQLite database
cp ./data/shelly.db ./backups/shelly-$(date +%Y%m%d-%H%M%S).db

# 3. Run migration
export POSTGRES_DSN="postgres://shelly:${POSTGRES_PASSWORD}@localhost:5432/shelly_manager?sslmode=require"
go run ./cmd/migrate/main.go
go run ./cmd/migrate-data/main.go

# 4. Update configuration
cp configs/shelly-manager.postgresql.yaml configs/shelly-manager.yaml

# 5. Start application
systemctl start shelly-manager

# 6. Verify health
sleep 10
curl -f http://localhost:8080/health

echo "Migration completed!"
```

## Configuration Updates

### Application Configuration Changes

#### Before Migration (SQLite)
```yaml
database:
  provider: "sqlite"
  dsn: "file:./data/shelly.db?_foreign_keys=on&_journal_mode=WAL"
  max_open_conns: 1
  max_idle_conns: 1
  conn_max_lifetime: "0"
  slow_query_threshold: "100ms"
```

#### After Migration (PostgreSQL)
```yaml
database:
  provider: "postgresql"
  dsn: "postgres://shelly:${POSTGRES_PASSWORD}@postgres:5432/shelly_manager?sslmode=require"
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

### Environment Variable Updates

#### Development Environment
```bash
# Remove SQLite variables
unset SQLITE_DB_PATH
unset SQLITE_JOURNAL_MODE

# Add PostgreSQL variables
export DATABASE_PROVIDER="postgresql"
export POSTGRES_HOST="localhost"
export POSTGRES_PORT="5432"
export POSTGRES_USER="shelly"
export POSTGRES_PASSWORD="development_password"
export POSTGRES_DB="shelly_manager_dev"
export POSTGRES_SSL_MODE="prefer"
export POSTGRES_MAX_OPEN_CONNS="10"
export POSTGRES_MAX_IDLE_CONNS="2"
```

#### Production Environment
```bash
# PostgreSQL production configuration
export DATABASE_PROVIDER="postgresql"
export POSTGRES_HOST="postgres-prod.internal"
export POSTGRES_PORT="5432"
export POSTGRES_USER="shelly_prod"
export POSTGRES_PASSWORD="$(cat /secrets/postgres-password)"
export POSTGRES_DB="shelly_manager"
export POSTGRES_SSL_MODE="require"
export POSTGRES_SSL_CERT="/certs/client.crt"
export POSTGRES_SSL_KEY="/certs/client.key"
export POSTGRES_SSL_ROOT_CERT="/certs/ca.crt"
export POSTGRES_MAX_OPEN_CONNS="50"
export POSTGRES_MAX_IDLE_CONNS="10"
export POSTGRES_CONN_MAX_LIFETIME="2h"
export POSTGRES_CONN_MAX_IDLE_TIME="15m"
```

### Docker Compose Updates

#### Before (SQLite)
```yaml
services:
  shelly-manager:
    image: shelly-manager:latest
    volumes:
      - ./data:/app/data
    environment:
      - DATABASE_PROVIDER=sqlite
      - SQLITE_DB_PATH=/app/data/shelly.db
```

#### After (PostgreSQL)
```yaml
services:
  shelly-manager:
    image: shelly-manager:latest
    environment:
      - DATABASE_PROVIDER=postgresql
      - POSTGRES_HOST=postgres
      - POSTGRES_USER=shelly
      - POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
      - POSTGRES_DB=shelly_manager
      - POSTGRES_SSL_MODE=require
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
    ports:
      - "5432:5432"

volumes:
  postgres_data:
```

## Post-Migration Optimization

### PostgreSQL Server Tuning

**postgresql.conf Optimization:**
```conf
# Memory Settings (adjust based on available RAM)
shared_buffers = 256MB
effective_cache_size = 1GB
work_mem = 64MB
maintenance_work_mem = 256MB

# Connection Settings
max_connections = 100

# Performance Settings
random_page_cost = 1.1          # SSD-optimized
effective_io_concurrency = 200
checkpoint_completion_target = 0.9
wal_buffers = 16MB

# Logging
log_min_duration_statement = 1000
log_checkpoints = on
log_connections = on
log_disconnections = on
```

### Index Optimization

**Create Optimal Indexes:**
```sql
-- Create indexes based on query patterns
CREATE INDEX CONCURRENTLY idx_devices_ip ON devices(ip);
CREATE INDEX CONCURRENTLY idx_devices_status ON devices(status);
CREATE INDEX CONCURRENTLY idx_devices_last_seen ON devices(last_seen);
CREATE INDEX CONCURRENTLY idx_device_configs_device_id ON device_configurations(device_id);
CREATE INDEX CONCURRENTLY idx_discovered_devices_ip ON discovered_devices(ip);

-- Composite indexes for common query patterns
CREATE INDEX CONCURRENTLY idx_devices_status_last_seen ON devices(status, last_seen);
CREATE INDEX CONCURRENTLY idx_sync_results_device_timestamp ON sync_results(device_id, created_at);
```

## Rollback Procedures

### Emergency Rollback Plan

**1. Immediate Rollback (if migration fails)**
```bash
#!/bin/bash
# emergency-rollback.sh

echo "Executing emergency rollback..."

# Stop application
systemctl stop shelly-manager

# Restore SQLite configuration
cp configs/shelly-manager.sqlite.backup configs/shelly-manager.yaml

# Verify SQLite database integrity
sqlite3 ./data/shelly.db "PRAGMA integrity_check;"

# Start application with SQLite
systemctl start shelly-manager

# Verify health
sleep 5
curl -f http://localhost:8080/health

echo "Emergency rollback completed"
```

**2. Planned Rollback (after successful migration)**
```bash
#!/bin/bash
# planned-rollback.sh

echo "Executing planned rollback to SQLite..."

# 1. Stop application
systemctl stop shelly-manager

# 2. Export current PostgreSQL data
pg_dump -h postgres-host -U shelly -d shelly_manager > /tmp/postgres_backup.sql

# 3. Clear SQLite database and import PostgreSQL data
rm -f ./data/shelly.db
sqlite3 ./data/shelly.db < /tmp/postgres_to_sqlite.sql

# 4. Restore SQLite configuration
cp configs/shelly-manager.sqlite.yaml configs/shelly-manager.yaml

# 5. Start application
systemctl start shelly-manager

echo "Rollback completed"
```

### Data Consistency Verification

**Post-Migration Verification:**
```go
// cmd/verify-consistency/main.go
func VerifyDataConsistency() error {
    // Connect to both databases
    sqlite := initSQLiteProvider()
    postgres := initPostgreSQLProvider()
    
    // Compare critical data
    checks := []ConsistencyCheck{
        {Table: "devices", Key: "ip", Columns: []string{"name", "status", "created_at"}},
        {Table: "device_configurations", Key: "device_id", Columns: []string{"config_data", "version"}},
    }
    
    for _, check := range checks {
        if err := verifyTableConsistency(sqlite, postgres, check); err != nil {
            return fmt.Errorf("consistency check failed for %s: %w", check.Table, err)
        }
    }
    
    return nil
}
```

## Monitoring and Maintenance

### Migration Monitoring

**Key Metrics to Track:**
- Migration progress (records migrated vs. total)
- Data consistency verification results
- Application performance before/after migration
- Database size and resource usage
- Connection pool utilization

**Monitoring Script:**
```bash
#!/bin/bash
# monitor-migration.sh

while true; do
    echo "=== Migration Status $(date) ==="
    
    # Check PostgreSQL connection
    if pg_isready -h postgres-host -p 5432 -U shelly; then
        echo "✓ PostgreSQL connection: OK"
    else
        echo "✗ PostgreSQL connection: FAILED"
    fi
    
    # Check record counts
    SQLITE_COUNT=$(sqlite3 ./data/shelly.db "SELECT COUNT(*) FROM devices;")
    PG_COUNT=$(psql -h postgres-host -U shelly -d shelly_manager -t -c "SELECT COUNT(*) FROM devices;")
    
    echo "Record counts - SQLite: $SQLITE_COUNT, PostgreSQL: $PG_COUNT"
    
    # Check application health
    if curl -s -f http://localhost:8080/health > /dev/null; then
        echo "✓ Application health: OK"
    else
        echo "✗ Application health: FAILED"
    fi
    
    echo "================================="
    sleep 30
done
```

### Post-Migration Maintenance

**Regular Maintenance Tasks:**
1. **Database Statistics Updates**: `ANALYZE` tables regularly
2. **Index Maintenance**: Monitor and rebuild indexes as needed
3. **Vacuum Operations**: Regular `VACUUM` and `VACUUM ANALYZE`
4. **Connection Pool Monitoring**: Track pool utilization and adjust as needed
5. **Performance Monitoring**: Monitor query performance and optimize as needed

This comprehensive migration guide ensures a smooth transition from SQLite to PostgreSQL with minimal risk and optimal performance.