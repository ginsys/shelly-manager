# Database Architecture & Enhancement Plan

## Executive Summary

This document outlines the comprehensive database architecture enhancement plan for Shelly Manager, including database abstraction, backup/restore capabilities, and export/import systems. The current SQLite-based implementation will be enhanced with multi-database support, plugin-based export systems, and robust backup strategies.

## Current State Analysis

### Database Implementation Status: ✅ **Good Foundation**

**Technology Stack**:
- **Database**: SQLite with file-based storage
- **ORM**: GORM v2 with auto-migration
- **Location**: `internal/database/database.go`
- **Pattern**: Repository/Manager pattern

**Strengths**:
- Clean separation of concerns
- Proper error handling and logging
- Performance instrumentation with timing metrics
- Write permissions validation
- Graceful connection management

**Limitations**:
1. **Tight Coupling**: Direct dependency on SQLite driver
2. **No Interface Abstraction**: Concrete implementation without abstraction layer
3. **Limited Scalability**: AutoMigrate doesn't scale for complex production scenarios
4. **No Transaction Management**: Missing explicit transaction support
5. **No Bulk Operations**: Lacks optimized bulk insert/update capabilities

## Data Classification & Recovery Strategy

### Data Categories by Origin and Criticality

#### 1. Device Registry Data 🔴 **CRITICAL**
- **Source**: Shelly device discovery and registration
- **Tables**: `devices`
- **Criticality**: HIGH - Core operational data
- **Recovery Impact**: Major - Loss requires full network rediscovery and device reconfiguration
- **Backup Priority**: Real-time
- **Estimated Size**: Small-Medium (~1-10K records)

#### 2. Configuration Management Data 🔴 **CRITICAL**
- **Source**: User-created templates, imported device configurations
- **Tables**: `config_templates`, `device_configs`, `config_history`
- **Criticality**: CRITICAL - Contains all device configurations and templates
- **Recovery Impact**: Severe - Loss means manual reconfiguration of all devices
- **Backup Priority**: Real-time with versioning
- **Estimated Size**: Medium (grows with configuration changes)

#### 3. Notification System Data 🟡 **IMPORTANT**
- **Source**: User configuration for alerts and notifications
- **Tables**: `notification_channels`, `notification_rules`, `notification_history`, `notification_templates`
- **Criticality**: MEDIUM - Can be recreated but time-consuming
- **Recovery Impact**: Moderate - Notification setup must be redone
- **Backup Priority**: Daily
- **Estimated Size**: Small

#### 4. Resolution & Drift Management 🟢 **OPERATIONAL**
- **Source**: System-generated policies and resolution tracking
- **Tables**: `resolution_policies`, `resolution_requests`, `resolution_history`, `resolution_schedules`, `resolution_metrics`
- **Criticality**: LOW-MEDIUM - Historical and analytical data
- **Recovery Impact**: Low - Can be regenerated from current device states
- **Backup Priority**: Weekly
- **Estimated Size**: Large (continuous growth)

#### 5. Analytics & Reporting Data 🔵 **INFORMATIONAL**
- **Source**: System analytics and historical reporting
- **Tables**: `drift_detection_schedules`, `drift_detection_runs`, `drift_reports`, `drift_trends`
- **Criticality**: LOW - Historical analytics data
- **Recovery Impact**: Minimal - Can be regenerated over time
- **Backup Priority**: Monthly
- **Estimated Size**: Very Large (historical accumulation)

## Database Abstraction Layer Architecture

### Interface Design

```go
// internal/database/provider/interface.go
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
    
    // Performance & Monitoring
    GetStats() DatabaseStats
    SetLogger(logger Logger)
}

type Transaction interface {
    GetDB() *gorm.DB
    Commit() error
    Rollback() error
}

type DatabaseConfig struct {
    Provider string            // "sqlite", "postgres", "mysql"
    DSN      string            // Data Source Name
    Options  map[string]string // Provider-specific options
}
```

### Provider Implementations

#### SQLite Provider (Current)
```go
type SQLiteProvider struct {
    db     *gorm.DB
    config SQLiteConfig
}

type SQLiteConfig struct {
    Path            string
    WALMode         bool
    ForeignKeys     bool
    JournalMode     string
    Synchronous     string
    CacheSize       int
    BusyTimeout     time.Duration
}
```

#### PostgreSQL Provider (Future)
```go
type PostgreSQLProvider struct {
    db     *gorm.DB
    config PostgreSQLConfig
}

type PostgreSQLConfig struct {
    Host            string
    Port            int
    Database        string
    Username        string
    Password        string
    SSLMode         string
    MaxOpenConns    int
    MaxIdleConns    int
    ConnMaxLifetime time.Duration
}
```

#### MySQL Provider (Future)
```go
type MySQLProvider struct {
    db     *gorm.DB
    config MySQLConfig
}

type MySQLConfig struct {
    Host            string
    Port            int
    Database        string
    Username        string
    Password        string
    Charset         string
    ParseTime       bool
    MaxOpenConns    int
    MaxIdleConns    int
    ConnMaxLifetime time.Duration
}
```

## Backup & Restore System Architecture

### Shelly Manager Archive Format (.sma)

The Shelly Manager Archive format provides database-agnostic backup and restore capabilities:

```yaml
# Archive Header
version: "1.0.0"
created_at: "2024-01-19T10:00:00Z"
shelly_manager_version: "0.5.1"
database_schema_version: "2024.01.01"
checksum_algorithm: "sha256"
compression: "gzip"
encryption: "aes-256-gcm"

# Metadata
metadata:
  total_records: 1542
  total_size: 2847392
  backup_type: "full" # "full", "incremental", "differential"
  source_database: "sqlite"
  
# Data Sections
data:
  # Critical Data (always included)
  devices:
    count: 24
    checksum: "sha256:abc123..."
    records: [...]
    
  config_templates:
    count: 8
    checksum: "sha256:def456..."
    records: [...]
    
  device_configs:
    count: 24
    checksum: "sha256:ghi789..."
    records: [...]
    
  # Optional sections (configurable)
  notification_channels:
    count: 3
    checksum: "sha256:jkl012..."
    records: [...]
    
  # Historical data (optional in backups)
  resolution_history:
    count: 1245
    checksum: "sha256:mno345..."
    records: [...]
```

### Backup Strategy Implementation

#### Full Backup
- Complete database export
- All tables included
- Suitable for disaster recovery
- Recommended frequency: Weekly

#### Incremental Backup
- Only changed records since last backup
- Requires change tracking
- Faster backup process
- Recommended frequency: Daily

#### Differential Backup
- All changes since last full backup
- Balance between full and incremental
- Good for medium-term recovery
- Recommended frequency: Daily (alternate with incremental)

### Restore Process

```go
type RestoreOptions struct {
    ArchivePath        string
    TargetDatabase     DatabaseConfig
    VerifyIntegrity    bool
    PreserveExisting   bool
    SelectiveTables    []string
    DryRun            bool
}

type RestoreResult struct {
    Success           bool
    TablesRestored    []string
    RecordsRestored   int
    Errors            []error
    Warnings          []string
    Duration          time.Duration
}
```

## Export & Import System Architecture

### Export Data Flow

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Data Sources  │───▶│  Export Engine   │───▶│  Format Plugins │
│                 │    │                  │    │                 │
│ • Devices       │    │ • Data Query     │    │ • Home Assistant│
│ • Configurations│    │ • Transformation │    │ • DHCP Servers  │
│ • Templates     │    │ • Validation     │    │ • Network Docs  │
│ • History       │    │ • Scheduling     │    │ • Custom Format │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                              │                          │
                              ▼                          ▼
                    ┌──────────────────┐    ┌─────────────────┐
                    │  Export History  │    │  Output Files   │
                    │                  │    │                 │
                    │ • Success/Failure│    │ • Multiple      │
                    │ • Validation     │    │   Formats       │
                    │ • Scheduling     │    │ • Webhooks      │
                    └──────────────────┘    └─────────────────┘
```

### Import Data Flow

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│  Input Sources  │───▶│  Import Engine   │───▶│   Data Store    │
│                 │    │                  │    │                 │
│ • Device Config │    │ • Format Detection│   │ • Database      │
│ • Bulk Upload   │    │ • Validation     │    │ • Configuration │
│ • API Calls     │    │ • Transformation │    │ • History       │
│ • File Upload   │    │ • Conflict Res.  │    │ • Audit Trail   │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                              │                          
                              ▼                          
                    ┌──────────────────┐                 
                    │  Import History  │                 
                    │                  │                 
                    │ • Success/Failure│                 
                    │ • Conflict Log   │                 
                    │ • Data Changes   │                 
                    └──────────────────┘                 
```

### Export Plugin System

#### Built-in Exporters

1. **Home Assistant Exporter**
   - MQTT Discovery format
   - Direct entity configuration YAML
   - Custom component integration files

2. **DHCP Reservation Exporter**
   - OPNSense API format
   - ISC DHCP configuration
   - pfSense XML format
   - Generic CSV format

3. **Network Documentation Exporter**
   - Hosts file format (`/etc/hosts`)
   - Ansible inventory YAML
   - NetBox import JSON
   - Network topology diagrams (GraphViz)

4. **Monitoring Integration Exporter**
   - Prometheus targets
   - Nagios configuration
   - Zabbix host definitions

#### Plugin Interface

```go
type ExportPlugin interface {
    // Plugin Metadata
    Name() string
    Version() string
    Description() string
    SupportedFormats() []string
    
    // Configuration
    ConfigSchema() map[string]interface{}
    ValidateConfig(config map[string]interface{}) error
    
    // Export Operations
    Export(data ExportData, config ExportConfig) (*ExportResult, error)
    ExportPreview(data ExportData, config ExportConfig) ([]byte, error)
    
    // Capabilities
    SupportsIncremental() bool
    RequiresAuthentication() bool
}

type ExportData struct {
    Devices        []database.Device
    Configurations []configuration.DeviceConfig
    Templates      []configuration.ConfigTemplate
    Metadata       map[string]interface{}
    Timestamp      time.Time
}

type ExportConfig struct {
    Format     string
    Options    map[string]interface{}
    OutputPath string
    Webhook    *WebhookConfig
}

type ExportResult struct {
    Success     bool
    OutputPath  string
    RecordCount int
    FileSize    int64
    Checksum    string
    Errors      []error
    Warnings    []string
    Duration    time.Duration
}
```

## Implementation Roadmap

### Phase 1: Database Abstraction Foundation (2-3 weeks)

**Week 1: Core Abstraction**
- [ ] Create `internal/database/provider/` package structure
- [ ] Define `DatabaseProvider` interface
- [ ] Implement `SQLiteProvider` (refactor existing code)
- [ ] Add provider factory pattern
- [ ] Update configuration system for provider selection

**Week 2: Enhanced SQLite Support**
- [ ] Add advanced SQLite configuration options
- [ ] Implement transaction management
- [ ] Add connection pooling support
- [ ] Create performance monitoring hooks
- [ ] Add bulk operation support

**Week 3: Testing & Validation**
- [ ] Comprehensive unit tests for abstraction layer
- [ ] Integration tests with existing functionality
- [ ] Performance benchmarking
- [ ] Migration testing
- [ ] Documentation updates

### Phase 2: PostgreSQL Support (2-3 weeks)

**Week 1: PostgreSQL Provider**
- [ ] Implement `PostgreSQLProvider`
- [ ] Add PostgreSQL-specific configuration
- [ ] Database connection management
- [ ] Schema migration support

**Week 2: Feature Parity**
- [ ] Ensure all operations work with PostgreSQL
- [ ] Performance optimization
- [ ] Connection pooling implementation
- [ ] Transaction support validation

**Week 3: Testing & Documentation**
- [ ] PostgreSQL integration tests
- [ ] Performance comparison with SQLite
- [ ] Migration guide from SQLite to PostgreSQL
- [ ] Configuration examples and best practices

### Phase 3: Backup & Restore System (3-4 weeks)

**Week 1: Core Backup Infrastructure**
- [ ] Create `internal/backup/` package
- [ ] Define Shelly Manager Archive (`.sma`) format
- [ ] Implement serialization engine
- [ ] Add compression and encryption support

**Week 2: Backup Operations**
- [ ] Full backup implementation
- [ ] Incremental backup with change tracking
- [ ] Differential backup support
- [ ] Backup scheduling system

**Week 3: Restore Operations**
- [ ] Archive validation and integrity checking
- [ ] Selective restore functionality
- [ ] Conflict resolution strategies
- [ ] Rollback capabilities

**Week 4: CLI & API Integration**
- [ ] CLI commands for backup/restore
- [ ] REST API endpoints
- [ ] Web UI integration
- [ ] Automated testing and validation

### Phase 4: Export Plugin System (3-4 weeks)

**Week 1: Plugin Infrastructure**
- [ ] Create `internal/export/` package
- [ ] Define plugin interfaces and contracts
- [ ] Implement plugin discovery and loading
- [ ] Create template-based export engine

**Week 2: Built-in Exporters**
- [ ] Home Assistant MQTT exporter
- [ ] DHCP reservation exporter (multiple formats)
- [ ] Network documentation exporter
- [ ] Generic CSV/JSON/YAML exporters

**Week 3: Export Management**
- [ ] Export scheduling system
- [ ] Export history and audit trails
- [ ] Webhook integration for automation
- [ ] Export validation and dry-run capabilities

**Week 4: Import System**
- [ ] Import engine with format detection
- [ ] Conflict resolution strategies
- [ ] Bulk import capabilities
- [ ] Import validation and rollback

### Phase 5: Advanced Features (2-3 weeks)

**Week 1: MySQL Support**
- [ ] Implement `MySQLProvider`
- [ ] Performance optimization
- [ ] Feature parity validation

**Week 2: External Plugin Support**
- [ ] External process plugin architecture
- [ ] Plugin sandboxing and security
- [ ] Plugin marketplace concept

**Week 3: Production Features**
- [ ] High availability configurations
- [ ] Load balancing support
- [ ] Monitoring and alerting integration
- [ ] Performance optimization and tuning

## Risk Assessment & Mitigation

### Technical Risks

1. **Database Migration Complexity**
   - **Risk**: Data loss during provider migration
   - **Mitigation**: Comprehensive backup before migration, rollback procedures, extensive testing

2. **Performance Impact**
   - **Risk**: Abstraction layer overhead
   - **Mitigation**: Performance benchmarking, optimization, caching strategies

3. **Plugin Security**
   - **Risk**: Malicious or buggy plugins
   - **Mitigation**: Plugin sandboxing, validation, review process

### Operational Risks

1. **Backward Compatibility**
   - **Risk**: Breaking existing installations
   - **Mitigation**: Gradual migration path, compatibility testing, versioned APIs

2. **Complexity Growth**
   - **Risk**: System becoming too complex
   - **Mitigation**: Clear documentation, modular design, optional features

## Success Metrics

### Technical Metrics
- Database operation performance (target: <5% overhead)
- Backup/restore speed (target: <10 minutes for typical datasets)
- Plugin loading time (target: <1 second per plugin)
- Export processing time (target: <30 seconds for standard exports)

### Operational Metrics
- Migration success rate (target: 100% with proper procedures)
- Backup reliability (target: 99.9% success rate)
- Export accuracy (target: 100% data integrity)
- System uptime during migrations (target: >95%)

### User Experience Metrics
- Configuration complexity reduction (target: 50% fewer manual steps)
- Time to recovery from backup (target: <1 hour)
- Export setup time (target: <5 minutes per format)
- Learning curve for new features (target: <2 hours for power users)

## Conclusion

This architecture enhancement provides Shelly Manager with enterprise-grade database capabilities while maintaining the simplicity and reliability of the current system. The modular design allows for gradual implementation and provides flexibility for future growth and integration requirements.

The plugin-based export system enables seamless integration with various home automation and network management platforms, while the robust backup/restore system ensures data reliability and disaster recovery capabilities.

---

**Document Version**: 1.0  
**Last Updated**: 2024-01-19  
**Next Review**: 2024-04-19  
**Owner**: Shelly Manager Development Team