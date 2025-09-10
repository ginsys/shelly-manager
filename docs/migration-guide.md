# Migration Guide - Shelly Manager Export/Import System

## Overview

This guide provides step-by-step instructions for migrating to the new Shelly Manager Export/Import System introduced in v0.5.4. The new system brings significant enhancements including the SMA format, comprehensive plugin architecture, and advanced security features.

## Table of Contents

- [Pre-Migration Checklist](#pre-migration-checklist)
- [Version Compatibility](#version-compatibility)
- [Migration Steps](#migration-steps)
- [Configuration Changes](#configuration-changes)
- [API Changes](#api-changes)
- [Database Changes](#database-changes)
- [UI Changes](#ui-changes)
- [Security Updates](#security-updates)
- [Rollback Procedures](#rollback-procedures)
- [Post-Migration Validation](#post-migration-validation)
- [Troubleshooting](#troubleshooting)

## Pre-Migration Checklist

Before starting the migration process, ensure you have completed the following steps:

### ✅ Backup Current System

**Critical**: Create a complete backup of your current system before beginning migration.

```bash
# 1. Stop the Shelly Manager service
sudo systemctl stop shelly-manager

# 2. Backup database
cp /var/lib/shelly/shelly.db /var/lib/shelly/shelly.db.backup.$(date +%Y%m%d)

# 3. Backup configuration
cp -r /etc/shelly/ /etc/shelly.backup.$(date +%Y%m%d)/

# 4. Backup application data
tar -czf /tmp/shelly-manager-backup-$(date +%Y%m%d).tar.gz \
  /var/lib/shelly/ \
  /etc/shelly/ \
  /var/log/shelly/

# 5. Export current devices (if export functionality exists)
curl -o /tmp/devices-backup.json http://localhost:8080/api/v1/export?format=json
```

### ✅ System Requirements Verification

Verify your system meets the requirements for the new version:

**Minimum Requirements**:
- Go 1.23+ (for building from source)
- 512MB RAM (1GB+ recommended)
- 1GB free disk space for exports/imports
- Compatible database: SQLite 3.35+, PostgreSQL 13+, or MySQL 8.0+

**New Dependencies**:
- Admin API key management
- Export/import directory with proper permissions
- WebSocket support for real-time features

### ✅ Network and Security Preparation

**Firewall Configuration**:
```bash
# Ensure WebSocket connections are allowed
sudo ufw allow 8080/tcp
```

**Directory Preparation**:
```bash
# Create export/import directories
sudo mkdir -p /var/exports/shelly-manager
sudo mkdir -p /var/imports/shelly-manager
sudo chown shelly-manager:shelly-manager /var/exports/shelly-manager
sudo chown shelly-manager:shelly-manager /var/imports/shelly-manager
sudo chmod 755 /var/exports/shelly-manager
sudo chmod 755 /var/imports/shelly-manager
```

## Version Compatibility

### Supported Migration Paths

| From Version | To Version | Migration Type | Difficulty |
|-------------|------------|----------------|------------|
| v0.5.3 | v0.5.4 | Direct | Easy |
| v0.5.2 | v0.5.4 | Direct | Medium |
| v0.5.1 | v0.5.4 | Direct | Medium |
| v0.5.0 | v0.5.4 | Direct | Hard |
| < v0.5.0 | v0.5.4 | Multi-step | Hard |

### Breaking Changes

**API Changes**:
- Export/import endpoints now require admin authentication when configured
- Response format updated to include new metadata fields
- WebSocket endpoints added for real-time metrics

**Configuration Changes**:
- New security section for admin API key
- Export/import configuration sections added
- Plugin configuration structure updated

**Database Changes**:
- New tables for export/import history
- New tables for scheduled operations
- Updated indexes for performance optimization

## Migration Steps

### Step 1: Prepare New Configuration

#### 1.1 Generate Admin API Key

```bash
# Generate a secure admin API key
ADMIN_API_KEY=$(openssl rand -hex 32)
echo "Generated Admin API Key: $ADMIN_API_KEY"

# Store securely for configuration
echo "$ADMIN_API_KEY" > /etc/shelly/admin-key.txt
chmod 600 /etc/shelly/admin-key.txt
```

#### 1.2 Update Configuration File

Create or update your configuration file to include new sections:

```yaml
# /etc/shelly/config.yaml

# Existing configuration sections remain unchanged
server:
  port: 8080
  host: 0.0.0.0

database:
  provider: sqlite
  path: /var/lib/shelly/shelly.db

# NEW: Security configuration
security:
  admin_api_key_file: "/etc/shelly/admin-key.txt"
  export_import_protection: true
  cors:
    allowed_origins:
      - "http://localhost:8080"
      - "https://your-domain.com"

# NEW: Export configuration
export:
  output_directory: "/var/exports/shelly-manager"
  schedule_enabled: true
  default_format: "sma"
  compression_enabled: true
  max_file_size: "100MB"
  retention:
    daily: 7
    weekly: 4
    monthly: 12

# NEW: Import configuration
import:
  temp_directory: "/var/imports/shelly-manager"
  validation_enabled: true
  backup_before_import: true
  conflict_resolution: "prompt"
  max_file_size: "100MB"

# NEW: Metrics configuration
metrics:
  enabled: true
  collection_interval: "30s"
  retention_period: "24h"
  websocket_enabled: true

# NEW: Notification configuration
notifications:
  enabled: true
  channels: []
  rules: []
```

#### 1.3 Environment Variables (Alternative Configuration)

If you prefer environment variables:

```bash
# /etc/systemd/system/shelly-manager.service.d/override.conf
[Service]
Environment="SHELLY_SECURITY_ADMIN_API_KEY_FILE=/etc/shelly/admin-key.txt"
Environment="SHELLY_EXPORT_OUTPUT_DIRECTORY=/var/exports/shelly-manager"
Environment="SHELLY_IMPORT_TEMP_DIRECTORY=/var/imports/shelly-manager"
Environment="SHELLY_SECURITY_EXPORT_IMPORT_PROTECTION=true"
Environment="SHELLY_METRICS_ENABLED=true"
Environment="SHELLY_NOTIFICATIONS_ENABLED=true"
```

### Step 2: Download and Install New Version

#### 2.1 Download Shelly Manager v0.5.4

```bash
# Download from GitHub releases
wget https://github.com/ginsys/shelly-manager/releases/download/v0.5.4/shelly-manager-linux-amd64.tar.gz

# Extract
tar -xzf shelly-manager-linux-amd64.tar.gz

# Backup current binary
sudo cp /usr/local/bin/shelly-manager /usr/local/bin/shelly-manager.backup

# Install new binary
sudo cp shelly-manager /usr/local/bin/shelly-manager
sudo chmod +x /usr/local/bin/shelly-manager
```

#### 2.2 Build from Source (Alternative)

```bash
# Clone repository
git clone https://github.com/ginsys/shelly-manager.git
cd shelly-manager
git checkout v0.5.4

# Build
make build

# Install
sudo cp bin/shelly-manager /usr/local/bin/shelly-manager
```

### Step 3: Database Migration

#### 3.1 Automatic Migration

The new version includes automatic database migration:

```bash
# Run migration (dry run first)
sudo -u shelly-manager /usr/local/bin/shelly-manager migrate --dry-run --config /etc/shelly/config.yaml

# Run actual migration
sudo -u shelly-manager /usr/local/bin/shelly-manager migrate --config /etc/shelly/config.yaml
```

#### 3.2 Manual Migration (If Automatic Fails)

If automatic migration fails, run manual SQL commands:

```sql
-- SQLite migration scripts
-- Create export history table
CREATE TABLE IF NOT EXISTS export_history (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  export_id TEXT UNIQUE NOT NULL,
  plugin_name TEXT NOT NULL,
  format TEXT NOT NULL,
  status TEXT NOT NULL,
  record_count INTEGER,
  file_size INTEGER,
  file_path TEXT,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  completed_at DATETIME,
  error_message TEXT
);

-- Create import history table
CREATE TABLE IF NOT EXISTS import_history (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  import_id TEXT UNIQUE NOT NULL,
  plugin_name TEXT NOT NULL,
  format TEXT NOT NULL,
  status TEXT NOT NULL,
  records_imported INTEGER,
  records_skipped INTEGER,
  source_file TEXT,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  completed_at DATETIME,
  error_message TEXT
);

-- Create scheduled operations table
CREATE TABLE IF NOT EXISTS scheduled_operations (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  type TEXT NOT NULL, -- 'export' or 'import'
  schedule TEXT NOT NULL, -- cron expression
  config TEXT NOT NULL, -- JSON configuration
  enabled BOOLEAN DEFAULT TRUE,
  last_run DATETIME,
  next_run DATETIME,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_export_history_created_at ON export_history(created_at);
CREATE INDEX IF NOT EXISTS idx_import_history_created_at ON import_history(created_at);
CREATE INDEX IF NOT EXISTS idx_scheduled_operations_next_run ON scheduled_operations(next_run, enabled);
```

### Step 4: Start New Version

#### 4.1 Start Service

```bash
# Start the service
sudo systemctl start shelly-manager

# Check status
sudo systemctl status shelly-manager

# Check logs
sudo journalctl -u shelly-manager -f
```

#### 4.2 Verify Service Health

```bash
# Health check
curl http://localhost:8080/health

# Ready check
curl http://localhost:8080/ready

# API version check
curl http://localhost:8080/api/v1/system/info
```

### Step 5: Validate Migration

#### 5.1 Verify Data Integrity

```bash
# Check device count
curl http://localhost:8080/api/v1/devices | jq '.data | length'

# Check template count
curl http://localhost:8080/api/v1/templates | jq '.data | length'

# Check configuration count
curl http://localhost:8080/api/v1/configurations | jq '.data | length'
```

#### 5.2 Test Export Functionality

```bash
# Test SMA export
curl -X POST http://localhost:8080/api/v1/export \
  -H "Authorization: Bearer $(cat /etc/shelly/admin-key.txt)" \
  -H "Content-Type: application/json" \
  -d '{
    "plugin_name": "sma",
    "format": "sma",
    "config": {"include_metadata": true}
  }'
```

#### 5.3 Test Import Functionality

```bash
# Test import preview
curl -X POST http://localhost:8080/api/v1/import/preview \
  -H "Authorization: Bearer $(cat /etc/shelly/admin-key.txt)" \
  -F "file=@/tmp/test-export.sma"
```

## Configuration Changes

### Legacy Configuration Mapping

| Legacy Setting | New Setting | Notes |
|---------------|------------|-------|
| `api.export_enabled` | `export.enabled` | Moved to export section |
| `api.export_formats` | `export.plugins` | Now plugin-based |
| `server.auth_enabled` | `security.admin_api_key` | Enhanced security |
| `logging.export_logs` | `logging.level` | Unified logging |

### New Configuration Sections

#### Security Configuration
```yaml
security:
  admin_api_key: "${ADMIN_API_KEY}"
  admin_api_key_file: "/path/to/key/file"
  export_import_protection: true
  trusted_proxies: ["10.0.0.0/8"]
  cors:
    allowed_origins: ["https://your-domain.com"]
    allowed_methods: ["GET", "POST", "PUT", "DELETE"]
    allowed_headers: ["Content-Type", "Authorization"]
```

#### Export Configuration
```yaml
export:
  output_directory: "/var/exports/shelly-manager"
  schedule_enabled: true
  default_format: "sma"
  compression_enabled: true
  max_file_size: "100MB"
  retention:
    daily: 7
    weekly: 4
    monthly: 12
  plugins:
    sma:
      enabled: true
      config:
        compression_level: 6
    terraform:
      enabled: true
      config:
        provider_version: "latest"
```

#### Import Configuration
```yaml
import:
  temp_directory: "/var/imports/shelly-manager"
  validation_enabled: true
  backup_before_import: true
  conflict_resolution: "prompt"
  max_file_size: "100MB"
  allowed_formats: ["sma", "json", "yaml"]
```

## API Changes

### New Endpoints

The following endpoints have been added:

#### Export Endpoints
- `GET /api/v1/export/plugins` - List available export plugins
- `POST /api/v1/export` - Create export operation
- `POST /api/v1/export/preview` - Preview export operation
- `GET /api/v1/export/{id}` - Get export result
- `GET /api/v1/export/{id}/download` - Download export file
- `GET /api/v1/export/history` - List export history
- `GET /api/v1/export/statistics` - Export statistics

#### Import Endpoints
- `POST /api/v1/import` - Create import operation
- `POST /api/v1/import/preview` - Preview import operation
- `GET /api/v1/import/{id}` - Get import result
- `GET /api/v1/import/history` - List import history
- `GET /api/v1/import/statistics` - Import statistics

#### Metrics Endpoints
- `GET /metrics/prometheus` - Prometheus metrics
- `GET /metrics/status` - Metrics status
- `POST /metrics/enable` - Enable metrics collection
- `POST /metrics/disable` - Disable metrics collection
- `GET /metrics/ws` - WebSocket metrics stream

#### Notification Endpoints
- `POST /api/v1/notifications/channels` - Create notification channel
- `GET /api/v1/notifications/channels` - List notification channels
- `POST /api/v1/notifications/rules` - Create notification rule
- `GET /api/v1/notifications/history` - Notification history

### Modified Endpoints

#### Authentication Requirements
Endpoints that now require admin authentication:
- All export endpoints (`/api/v1/export/*`)
- All import endpoints (`/api/v1/import/*`)
- All metrics endpoints (`/metrics/*`) except `/metrics/prometheus`
- All notification endpoints (`/api/v1/notifications/*`)

#### Response Format Changes
All endpoints now use standardized response format:
```json
{
  "success": true,
  "data": { ... },
  "meta": { ... },
  "timestamp": "2024-01-15T10:30:00Z",
  "request_id": "req_123456"
}
```

### Legacy Endpoint Compatibility

Legacy endpoints remain functional but deprecated:
- `GET /api/v1/export?format=json` → Use new export API
- `GET /api/v1/export?format=csv` → Use new export API

## Database Changes

### New Tables

The migration adds several new tables:

#### export_history
Tracks all export operations with metadata and status.

#### import_history  
Tracks all import operations with conflict resolution details.

#### scheduled_operations
Manages scheduled export/import operations.

#### notification_channels
Stores notification channel configurations.

#### notification_rules
Defines notification rules and triggers.

#### notification_history
Logs all sent notifications with delivery status.

### Index Optimization

New indexes are added for performance:
- Time-based queries on history tables
- Plugin and status filtering
- Scheduled operation lookup

### Data Migration

Existing data is preserved and enhanced:
- Device data remains unchanged
- Template data gains export metadata
- Configuration data includes import tracking

## UI Changes

### New Navigation Structure

The UI now includes new sections:
- Export/Import management
- Plugin configuration  
- Metrics dashboard
- Notification management

### Updated Workflows

#### Export Workflow
1. Select export format and configuration
2. Preview export contents and size
3. Execute export with progress tracking
4. Download or access via history

#### Import Workflow
1. Upload or select import file
2. Validate file format and contents
3. Preview changes and resolve conflicts
4. Execute import with rollback option

### Responsive Design Updates

The UI has been updated for mobile compatibility:
- Touch-friendly interface elements
- Responsive layouts for all screen sizes
- Progressive enhancement for advanced features

## Security Updates

### Admin API Key System

The new version introduces admin API key authentication:

#### Key Generation
```bash
# Generate secure key
openssl rand -hex 32

# Store in file
echo "your-generated-key" > /etc/shelly/admin-key.txt
chmod 600 /etc/shelly/admin-key.txt
```

#### Key Configuration
```yaml
security:
  admin_api_key_file: "/etc/shelly/admin-key.txt"
  # OR
  admin_api_key: "${ADMIN_API_KEY}"
```

#### Key Rotation
```bash
# Generate new key
NEW_KEY=$(openssl rand -hex 32)

# Update configuration
echo "$NEW_KEY" > /etc/shelly/admin-key.txt

# Restart service
sudo systemctl restart shelly-manager
```

### CORS Configuration

Configure CORS for web UI access:
```yaml
security:
  cors:
    allowed_origins:
      - "https://your-domain.com"
      - "http://localhost:3000"  # Development
    allowed_methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
    allowed_headers: ["Content-Type", "Authorization", "X-Requested-With"]
    max_age: 86400
```

### Safe Download Configuration

Restrict export downloads to specific directory:
```yaml
export:
  output_directory: "/var/exports/shelly-manager"
  safe_downloads: true
```

## Rollback Procedures

### Emergency Rollback

If you encounter critical issues, follow these steps for emergency rollback:

#### 1. Stop Current Service
```bash
sudo systemctl stop shelly-manager
```

#### 2. Restore Previous Binary
```bash
sudo cp /usr/local/bin/shelly-manager.backup /usr/local/bin/shelly-manager
```

#### 3. Restore Database
```bash
# For SQLite
cp /var/lib/shelly/shelly.db.backup.YYYYMMDD /var/lib/shelly/shelly.db

# For PostgreSQL
psql -U shelly_user -d shelly_db < /tmp/database-backup.sql
```

#### 4. Restore Configuration
```bash
sudo cp -r /etc/shelly.backup.YYYYMMDD/* /etc/shelly/
```

#### 5. Start Previous Version
```bash
sudo systemctl start shelly-manager
sudo systemctl status shelly-manager
```

### Gradual Rollback

For non-critical issues, you can disable new features while keeping the new version:

#### Disable Export/Import
```yaml
export:
  enabled: false
import:
  enabled: false
```

#### Disable Metrics
```yaml
metrics:
  enabled: false
```

#### Disable Notifications
```yaml
notifications:
  enabled: false
```

## Post-Migration Validation

### Functional Testing

#### 1. Core Functionality
```bash
# Test device management
curl http://localhost:8080/api/v1/devices

# Test configuration management
curl http://localhost:8080/api/v1/configurations

# Test template management
curl http://localhost:8080/api/v1/templates
```

#### 2. Export Functionality
```bash
# Test SMA export
curl -X POST http://localhost:8080/api/v1/export \
  -H "Authorization: Bearer $ADMIN_KEY" \
  -d '{"plugin_name": "sma", "format": "sma"}'

# Test JSON export
curl -X POST http://localhost:8080/api/v1/export \
  -H "Authorization: Bearer $ADMIN_KEY" \
  -d '{"plugin_name": "json", "format": "json"}'
```

#### 3. Import Functionality
```bash
# Test import validation
curl -X POST http://localhost:8080/api/v1/import/preview \
  -H "Authorization: Bearer $ADMIN_KEY" \
  -F "file=@test-import.sma"
```

#### 4. Metrics and Monitoring
```bash
# Test metrics endpoint
curl http://localhost:8080/metrics/prometheus

# Test metrics status
curl -H "Authorization: Bearer $ADMIN_KEY" \
  http://localhost:8080/metrics/status
```

### Performance Testing

#### Load Testing
```bash
# Test export performance with large dataset
time curl -X POST http://localhost:8080/api/v1/export \
  -H "Authorization: Bearer $ADMIN_KEY" \
  -d '{"plugin_name": "sma", "format": "sma"}'
```

#### Memory Testing
```bash
# Monitor memory usage during operations
sudo systemctl status shelly-manager
ps aux | grep shelly-manager
```

### Security Testing

#### Authentication Testing
```bash
# Test unauthenticated access (should fail)
curl -X POST http://localhost:8080/api/v1/export \
  -d '{"plugin_name": "sma", "format": "sma"}'

# Test invalid key (should fail)
curl -X POST http://localhost:8080/api/v1/export \
  -H "Authorization: Bearer invalid-key" \
  -d '{"plugin_name": "sma", "format": "sma"}'
```

#### CORS Testing
```bash
# Test CORS headers
curl -H "Origin: https://your-domain.com" \
  -H "Access-Control-Request-Method: POST" \
  -H "Access-Control-Request-Headers: X-Requested-With" \
  -X OPTIONS http://localhost:8080/api/v1/export
```

## Troubleshooting

### Common Migration Issues

#### 1. Database Migration Failures

**Symptom**: Migration fails with database errors
**Causes**: 
- Insufficient permissions
- Database corruption
- Version incompatibility

**Solutions**:
```bash
# Check database permissions
ls -la /var/lib/shelly/shelly.db

# Fix permissions
sudo chown shelly-manager:shelly-manager /var/lib/shelly/shelly.db

# Check database integrity
sqlite3 /var/lib/shelly/shelly.db "PRAGMA integrity_check;"

# Manual migration if automatic fails
sqlite3 /var/lib/shelly/shelly.db < migration-scripts/v0.5.4.sql
```

#### 2. Configuration Issues

**Symptom**: Service fails to start with configuration errors
**Causes**:
- Invalid YAML syntax
- Missing required fields
- File permission issues

**Solutions**:
```bash
# Validate YAML syntax
yamllint /etc/shelly/config.yaml

# Check file permissions
ls -la /etc/shelly/config.yaml

# Validate configuration
/usr/local/bin/shelly-manager validate-config --config /etc/shelly/config.yaml
```

#### 3. Authentication Issues

**Symptom**: Export/import operations fail with 401 errors
**Causes**:
- Missing admin API key
- Incorrect key configuration
- File permission issues

**Solutions**:
```bash
# Check admin key file
ls -la /etc/shelly/admin-key.txt

# Verify key content
cat /etc/shelly/admin-key.txt

# Test authentication
curl -H "Authorization: Bearer $(cat /etc/shelly/admin-key.txt)" \
  http://localhost:8080/api/v1/export/plugins
```

#### 4. File Permission Issues

**Symptom**: Export/import operations fail with permission errors
**Causes**:
- Incorrect directory permissions
- Missing directories
- SELinux/AppArmor restrictions

**Solutions**:
```bash
# Create missing directories
sudo mkdir -p /var/exports/shelly-manager
sudo mkdir -p /var/imports/shelly-manager

# Fix ownership
sudo chown -R shelly-manager:shelly-manager /var/exports/shelly-manager
sudo chown -R shelly-manager:shelly-manager /var/imports/shelly-manager

# Fix permissions
sudo chmod 755 /var/exports/shelly-manager
sudo chmod 755 /var/imports/shelly-manager

# Check SELinux context
ls -Z /var/exports/shelly-manager
```

#### 5. WebSocket Connection Issues

**Symptom**: Real-time metrics not working
**Causes**:
- Firewall blocking WebSocket connections
- Proxy configuration issues
- Authentication problems

**Solutions**:
```bash
# Check firewall
sudo ufw status
sudo ufw allow 8080/tcp

# Test WebSocket connection
curl -i -N -H "Connection: Upgrade" \
  -H "Upgrade: websocket" \
  -H "Sec-WebSocket-Version: 13" \
  -H "Sec-WebSocket-Key: SGVsbG8sIHdvcmxkIQ==" \
  http://localhost:8080/metrics/ws
```

### Debug Mode

Enable debug mode for detailed troubleshooting:

```yaml
logging:
  level: "debug"
  format: "json"
  export_import_debug: true
  websocket_debug: true
```

### Log Analysis

Check logs for specific issues:

```bash
# General service logs
sudo journalctl -u shelly-manager -f

# Filter for export/import
sudo journalctl -u shelly-manager | grep -i "export\|import"

# Filter for errors
sudo journalctl -u shelly-manager | grep -i "error\|fail"

# Check for database issues
sudo journalctl -u shelly-manager | grep -i "database\|migration"
```

### Support Resources

If you encounter issues not covered in this guide:

1. **Check GitHub Issues**: Search for similar issues
2. **Review Documentation**: Check API and configuration documentation
3. **Enable Debug Logging**: Capture detailed logs for analysis
4. **Create Support Request**: Include logs, configuration, and reproduction steps

### Contact Information

- **GitHub Issues**: https://github.com/ginsys/shelly-manager/issues
- **Documentation**: https://github.com/ginsys/shelly-manager/docs
- **Community Support**: GitHub Discussions

---

**Migration Guide Version**: 1.0  
**Last Updated**: 2024-01-15  
**Target Version**: Shelly Manager v0.5.4  
**Compatibility**: Covers migration from v0.5.0+