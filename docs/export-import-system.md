# Export/Import System - Comprehensive Guide

## Overview

The Shelly Manager Export/Import System is a comprehensive data management solution that enables complete system backup, migration, and infrastructure integration. This system transforms your device management capabilities by providing multiple export formats, automated scheduling, and intelligent import validation.

## Table of Contents

- [Core Features](#core-features)
- [Export System](#export-system)
- [Import System](#import-system)
- [SMA Format](#sma-format)
- [Plugin Architecture](#plugin-architecture)
- [Web UI Features](#web-ui-features)
- [Security & Access Control](#security--access-control)
- [Configuration](#configuration)
- [Usage Examples](#usage-examples)
- [Troubleshooting](#troubleshooting)
- [Migration Guide](#migration-guide)

## Core Features

### 🗃️ Multiple Export Formats

The system supports various export formats to meet different integration and backup needs:

#### SMA (Shelly Management Archive)
- **Purpose**: Complete system backup and migration
- **Format**: Compressed JSON with metadata
- **Features**: Version compatibility, integrity verification, complete system state
- **Use Cases**: System backup, disaster recovery, migration between instances

#### Infrastructure as Code Formats
- **Terraform**: Export devices as Terraform resources for infrastructure management
- **Ansible**: Generate Ansible playbooks for automated device configuration
- **Kubernetes**: Create ConfigMaps and Secrets for container deployments
- **Docker Compose**: Generate compose files for containerized environments

#### Data Exchange Formats
- **JSON**: Structured data export for programmatic access
- **CSV**: Spreadsheet-compatible format for data analysis
- **YAML**: Human-readable configuration format

### 📤 Advanced Export Operations

#### Export Preview
Before executing any export operation, you can preview the results:
- **Record Counts**: See exactly how many devices, templates, and configurations will be exported
- **Size Estimates**: Get accurate file size predictions
- **Content Summary**: Review what data will be included
- **Validation Warnings**: Identify potential issues before export

#### Scheduled Exports
Automate your backup strategy with scheduled exports:
- **Flexible Scheduling**: Daily, weekly, monthly, or custom intervals
- **Retention Policies**: Automatic cleanup of old backups
- **Format Selection**: Choose different formats for different schedules
- **Notification Integration**: Get alerts on export success or failure

#### Filter and Selection
Export exactly what you need:
- **Device Filtering**: Export specific devices by ID, name, or type
- **Template Selection**: Include only relevant configuration templates
- **Time-based Filtering**: Export devices modified within a date range
- **Status-based Filtering**: Export only online/offline devices

### 📥 Intelligent Import System

#### Validation First Approach
All imports are validated before any changes are applied:
- **Schema Validation**: Ensure data structure matches expected format
- **Dependency Checking**: Verify all referenced templates and dependencies exist
- **Integrity Verification**: Check checksums and data consistency
- **Conflict Detection**: Identify potential conflicts before import

#### Dry Run Mode
Preview import operations without making any changes:
- **Change Preview**: See exactly what will be created, updated, or deleted
- **Impact Assessment**: Understand the full scope of changes
- **Conflict Resolution**: Review how conflicts will be handled
- **Risk Assessment**: Identify potential issues before execution

#### Smart Conflict Resolution
Handle conflicts intelligently:
- **Device MAC Conflicts**: Update existing devices with imported data
- **Template Name Conflicts**: Create new templates with import suffixes
- **Configuration Conflicts**: Choose merge strategies (overwrite, merge, skip)
- **User Prompts**: Interactive resolution for complex conflicts

## Export System

### Export Process Flow

1. **Selection**: Choose export format and filters
2. **Preview**: Review what will be exported
3. **Validation**: Verify export configuration
4. **Execution**: Generate export file
5. **Storage**: Save to configured output directory
6. **Notification**: Send completion alerts if configured

### Export Formats Details

#### SMA Format Export
```bash
# Complete system export
curl -X POST http://localhost:8080/api/v1/export \
  -H "Authorization: Bearer <ADMIN_KEY>" \
  -H "Content-Type: application/json" \
  -d '{
    "plugin_name": "sma",
    "format": "sma",
    "config": {
      "include_metadata": true,
      "compression": true
    },
    "output": {
      "type": "file",
      "destination": "/backups/"
    }
  }'
```

#### Terraform Export
```bash
# Export devices as Terraform resources
curl -X POST http://localhost:8080/api/v1/export \
  -H "Authorization: Bearer <ADMIN_KEY>" \
  -H "Content-Type: application/json" \
  -d '{
    "plugin_name": "terraform",
    "format": "tf",
    "filters": {
      "device_ids": [1, 2, 3]
    },
    "config": {
      "provider_version": "latest",
      "resource_prefix": "shelly_"
    }
  }'
```

#### Ansible Playbook Export
```bash
# Generate Ansible playbooks
curl -X POST http://localhost:8080/api/v1/export \
  -H "Authorization: Bearer <ADMIN_KEY>" \
  -H "Content-Type: application/json" \
  -d '{
    "plugin_name": "ansible",
    "format": "yaml",
    "config": {
      "playbook_name": "shelly_devices",
      "include_vars": true,
      "task_organization": "by_device_type"
    }
  }'
```

### Export History and Monitoring

#### Access Export History
```bash
# Get export history with pagination
curl -H "Authorization: Bearer <ADMIN_KEY>" \
  "http://localhost:8080/api/v1/export/history?page=1&page_size=20"

# Filter by plugin and success status
curl -H "Authorization: Bearer <ADMIN_KEY>" \
  "http://localhost:8080/api/v1/export/history?plugin=sma&success=true"
```

#### Export Statistics
```bash
# Get export statistics
curl -H "Authorization: Bearer <ADMIN_KEY>" \
  "http://localhost:8080/api/v1/export/statistics"
```

Response includes:
- Total exports by plugin
- Success/failure rates
- Average export times
- File size statistics

## Import System

### Import Process Flow

1. **File Upload**: Provide import file or URL
2. **Format Detection**: Automatically detect file format
3. **Validation**: Comprehensive validation checks
4. **Preview**: Show what changes will be made
5. **Confirmation**: User confirms import operation
6. **Backup**: Optional backup before import
7. **Execution**: Apply changes to system
8. **Verification**: Confirm import success

### Import Validation Stages

#### Stage 1: File Validation
- File format verification
- Compression and encoding checks
- Basic structure validation
- Size and limit checks

#### Stage 2: Schema Validation
- JSON/YAML schema compliance
- Required field presence
- Data type validation
- Format-specific rules

#### Stage 3: Data Validation
- Device MAC address validation
- IP address format checking
- Template reference validation
- Configuration syntax checking

#### Stage 4: Dependency Validation
- Template dependency checking
- Network configuration validation
- Plugin configuration verification
- System compatibility checking

### Import Options

#### Conflict Resolution Strategies
```yaml
conflict_resolution:
  devices: "update"        # update, skip, prompt
  templates: "rename"      # rename, overwrite, skip, prompt
  configurations: "merge"  # merge, overwrite, skip, prompt
```

#### Import Modes
- **Dry Run**: Preview only, no changes applied
- **Validate Only**: Validation without import
- **Force Overwrite**: Override all conflicts
- **Interactive**: Prompt for each conflict

#### Backup Options
- **Backup Before Import**: Create automatic backup
- **Selective Backup**: Backup only affected resources
- **No Backup**: Skip backup creation (not recommended)

### Import Examples

#### SMA File Import
```bash
# Import with dry run
curl -X POST http://localhost:8080/api/v1/import \
  -H "Authorization: Bearer <ADMIN_KEY>" \
  -F "file=@backup.sma" \
  -F 'options={
    "dry_run": true,
    "backup_before": true,
    "conflict_resolution": "prompt"
  }'
```

#### Configuration Import
```bash
# Import device configurations only
curl -X POST http://localhost:8080/api/v1/import \
  -H "Authorization: Bearer <ADMIN_KEY>" \
  -F "file=@devices.json" \
  -F 'options={
    "sections": ["devices", "configurations"],
    "merge_strategy": "overwrite"
  }'
```

## SMA Format

### Format Specification

The SMA (Shelly Management Archive) format is specifically designed for complete system backup and migration. See [docs/sma-format.md](sma-format.md) for detailed specifications.

#### Key Features
- **Compression**: Gzip compression for reduced file size
- **Versioning**: Forward and backward compatibility
- **Integrity**: SHA-256 checksums for data verification
- **Metadata**: Rich export context and system information

#### Data Sections
- **Devices**: Complete device inventory with configurations
- **Templates**: Configuration templates and variables
- **Discovered Devices**: Unmanaged devices from discovery
- **Network Settings**: WiFi, MQTT, and network configuration
- **Plugin Configurations**: Enabled plugins and settings
- **System Settings**: Application-level configuration

### SMA Import Behavior

#### Conflict Resolution
- **Device Conflicts**: MAC address-based matching and updating
- **Template Conflicts**: Name-based conflict detection with rename options
- **Configuration Conflicts**: Merge strategies based on user preferences

#### Data Migration
- **Schema Migration**: Automatic migration between compatible versions
- **Field Mapping**: Intelligent mapping of changed field names
- **Default Values**: Automatic application of new default values
- **Validation**: Comprehensive validation with repair suggestions

## Plugin Architecture

### Plugin System Overview

The export/import system uses a plugin architecture that allows for extensible format support and custom integration scenarios.

#### Built-in Plugins
- **SMA Plugin**: Complete system backup and restore
- **JSON Plugin**: Structured data export/import
- **CSV Plugin**: Spreadsheet-compatible data export
- **Terraform Plugin**: Infrastructure as Code generation
- **Ansible Plugin**: Playbook generation for automation
- **Kubernetes Plugin**: ConfigMap and Secret generation

#### Plugin Development
```go
type ExportPlugin interface {
    Name() string
    Description() string
    SupportedFormats() []string
    Export(context.Context, ExportRequest) (ExportResult, error)
    GetSchema() (Schema, error)
}

type ImportPlugin interface {
    Name() string
    Description() string
    SupportedFormats() []string
    Import(context.Context, ImportRequest) (ImportResult, error)
    Validate(context.Context, ValidationRequest) (ValidationResult, error)
}
```

#### Custom Plugin Configuration
```yaml
plugins:
  export:
    - name: custom_terraform
      enabled: true
      config:
        provider_version: "1.5.0"
        resource_naming: "snake_case"
    - name: custom_integration
      enabled: false
      config:
        endpoint: "https://api.example.com"
        api_key: "${INTEGRATION_API_KEY}"
```

## Web UI Features

### Export Management Interface

#### Export Wizard
- **Step-by-step Process**: Guided export configuration
- **Format Selection**: Visual format chooser with descriptions
- **Filter Configuration**: Interactive device and template selection
- **Preview Mode**: Real-time preview of export content
- **Download Management**: Secure download links with expiration

#### Export Dashboard
- **Recent Exports**: List of recent export operations
- **Status Monitoring**: Real-time export progress tracking
- **History Browser**: Searchable export history with filters
- **Statistics View**: Visual export statistics and trends

### Import Management Interface

#### Import Wizard
- **File Upload**: Drag-and-drop file upload with validation
- **Format Detection**: Automatic format detection with override options
- **Validation Results**: Visual validation results with error details
- **Change Preview**: Interactive preview of import changes
- **Conflict Resolution**: Visual conflict resolution interface

#### Import Dashboard
- **Import Queue**: View pending import operations
- **Progress Tracking**: Real-time import progress monitoring
- **History Browser**: Complete import history with status
- **Rollback Interface**: Easy rollback to previous states

### Responsive Design Features

#### Mobile Optimization
- **Touch-friendly Interface**: Optimized for mobile devices
- **Responsive Layouts**: Adaptive layouts for all screen sizes
- **Offline Capability**: Basic functionality available offline
- **Progressive Enhancement**: Enhanced features on capable devices

#### Accessibility
- **WCAG 2.1 AA Compliance**: Full accessibility compliance
- **Keyboard Navigation**: Complete keyboard accessibility
- **Screen Reader Support**: Optimized for assistive technologies
- **High Contrast Mode**: Support for high contrast themes

## Security & Access Control

### Authentication and Authorization

#### Admin API Key Protection
All export/import operations require admin-level access when security is enabled:

```yaml
security:
  admin_api_key: "${ADMIN_API_KEY}"
  export_import_protection: true
```

#### Request Authentication
```bash
# Header-based authentication
curl -H "Authorization: Bearer <ADMIN_KEY>" \
  http://localhost:8080/api/v1/export/history

# Alternative header format
curl -H "X-API-Key: <ADMIN_KEY>" \
  http://localhost:8080/api/v1/export/history
```

### Data Protection

#### Safe Downloads
Prevent path traversal and unauthorized file access:

```yaml
export:
  output_directory: "/var/exports/shelly-manager"
  safe_downloads: true
  allowed_extensions: [".sma", ".json", ".yaml", ".tf"]
```

#### Sensitive Data Handling
- **Password Filtering**: Option to exclude sensitive data from exports
- **API Key Masking**: Automatic masking of API keys in exports
- **Audit Trails**: Complete audit logs of all operations
- **Data Encryption**: Support for encrypted exports (future feature)

### Access Control Features

#### Role-based Access
- **Admin Operations**: Full export/import access
- **Read-only Access**: View-only access to export history
- **No Access**: Complete restriction of export/import features

#### IP Restrictions
```yaml
security:
  admin_api_key: "${ADMIN_API_KEY}"
  allowed_ips:
    - "192.168.1.0/24"
    - "10.0.0.0/8"
  blocked_ips:
    - "192.168.1.100"
```

## Configuration

### Environment Variables

#### Security Configuration
```bash
# Admin API key for export/import protection
SHELLY_SECURITY_ADMIN_API_KEY=your-secure-admin-key

# Enable export/import protection
SHELLY_SECURITY_EXPORT_IMPORT_PROTECTION=true
```

#### Export Configuration
```bash
# Safe download directory restriction
SHELLY_EXPORT_OUTPUT_DIRECTORY=/var/exports/shelly-manager

# Enable scheduled exports
SHELLY_EXPORT_SCHEDULE_ENABLED=true

# Default export format
SHELLY_EXPORT_DEFAULT_FORMAT=sma

# Enable compression
SHELLY_EXPORT_COMPRESSION_ENABLED=true
```

#### Import Configuration
```bash
# Enable import validation
SHELLY_IMPORT_VALIDATION_ENABLED=true

# Backup before import
SHELLY_IMPORT_BACKUP_BEFORE_IMPORT=true

# Default conflict resolution
SHELLY_IMPORT_CONFLICT_RESOLUTION=prompt
```

### YAML Configuration

#### Complete Configuration Example
```yaml
# Security settings
security:
  admin_api_key: "${ADMIN_API_KEY}"
  export_import_protection: true
  cors:
    allowed_origins:
      - "https://manager.example.com"
  
# Export configuration
export:
  output_directory: "/var/exports/shelly-manager"
  schedule_enabled: true
  default_format: "sma"
  compression_enabled: true
  retention:
    daily: 7     # Keep 7 daily backups
    weekly: 4    # Keep 4 weekly backups
    monthly: 12  # Keep 12 monthly backups
  plugins:
    sma:
      enabled: true
      config:
        include_metadata: true
        compression_level: 6
    terraform:
      enabled: true
      config:
        provider_version: "latest"
        resource_prefix: "shelly_"
    ansible:
      enabled: true
      config:
        playbook_format: "yaml"
        include_vars: true

# Import configuration
import:
  validation_enabled: true
  backup_before_import: true
  conflict_resolution: "prompt"  # prompt|overwrite|skip|merge
  max_file_size: "100MB"
  allowed_formats: ["sma", "json", "yaml"]
  temp_directory: "/tmp/shelly-imports"

# Notification integration
notifications:
  export_success:
    channel_id: 1
    enabled: true
  export_failure:
    channel_id: 1
    enabled: true
  import_success:
    channel_id: 2
    enabled: true
  import_failure:
    channel_id: 2
    enabled: true
```

### Plugin Configuration

#### Custom Plugin Settings
```yaml
plugins:
  export:
    custom_backup:
      enabled: true
      config:
        remote_storage: "s3"
        bucket: "shelly-backups"
        encryption: true
    custom_monitoring:
      enabled: true
      config:
        webhook_url: "https://monitoring.example.com/webhook"
        include_metrics: true
```

## Usage Examples

### Complete Backup Workflow

#### 1. Create Full System Backup
```bash
# CLI approach
./bin/shelly-manager export --format sma --output /backups/ --include-all

# API approach
curl -X POST http://localhost:8080/api/v1/export \
  -H "Authorization: Bearer <ADMIN_KEY>" \
  -H "Content-Type: application/json" \
  -d '{
    "plugin_name": "sma",
    "format": "sma",
    "config": {
      "include_metadata": true,
      "include_discovered": true,
      "include_system_settings": true
    },
    "output": {
      "type": "file",
      "destination": "/backups/"
    }
  }'
```

#### 2. Verify Backup Integrity
```bash
# CLI verification
./bin/shelly-manager validate --file /backups/shelly-backup-20240115.sma

# API verification
curl -X POST http://localhost:8080/api/v1/import/preview \
  -H "Authorization: Bearer <ADMIN_KEY>" \
  -F "file=@/backups/shelly-backup-20240115.sma" \
  -F 'options={"validate_only": true}'
```

#### 3. Restore from Backup
```bash
# CLI restore with dry run first
./bin/shelly-manager import --file /backups/shelly-backup-20240115.sma --dry-run
./bin/shelly-manager import --file /backups/shelly-backup-20240115.sma --backup-before

# API restore
curl -X POST http://localhost:8080/api/v1/import \
  -H "Authorization: Bearer <ADMIN_KEY>" \
  -F "file=@/backups/shelly-backup-20240115.sma" \
  -F 'options={
    "backup_before": true,
    "conflict_resolution": "overwrite"
  }'
```

### Infrastructure as Code Integration

#### 1. Export to Terraform
```bash
# Export devices as Terraform resources
curl -X POST http://localhost:8080/api/v1/export \
  -H "Authorization: Bearer <ADMIN_KEY>" \
  -H "Content-Type: application/json" \
  -d '{
    "plugin_name": "terraform",
    "format": "tf",
    "config": {
      "provider_version": "1.5.0",
      "resource_naming": "snake_case",
      "include_variables": true
    },
    "output": {
      "type": "file",
      "destination": "./infrastructure/"
    }
  }'
```

#### 2. Generate Ansible Playbooks
```bash
# Export as Ansible playbook
curl -X POST http://localhost:8080/api/v1/export \
  -H "Authorization: Bearer <ADMIN_KEY>" \
  -H "Content-Type: application/json" \
  -d '{
    "plugin_name": "ansible",
    "format": "yaml",
    "config": {
      "playbook_name": "shelly_device_management",
      "task_organization": "by_device_type",
      "include_vars": true,
      "include_handlers": true
    }
  }'
```

### Migration Scenarios

#### 1. Migrate Between Environments
```bash
# Export from source environment
curl -X POST http://source-manager:8080/api/v1/export \
  -H "Authorization: Bearer <ADMIN_KEY>" \
  -d '{"plugin_name": "sma", "format": "sma"}' > migration.sma

# Import to target environment
curl -X POST http://target-manager:8080/api/v1/import \
  -H "Authorization: Bearer <ADMIN_KEY>" \
  -F "file=@migration.sma" \
  -F 'options={"backup_before": true}'
```

#### 2. Selective Migration
```bash
# Export specific devices and templates
curl -X POST http://localhost:8080/api/v1/export \
  -H "Authorization: Bearer <ADMIN_KEY>" \
  -d '{
    "plugin_name": "sma",
    "format": "sma",
    "filters": {
      "device_ids": [1, 2, 3, 4],
      "template_names": ["production_switch", "production_dimmer"]
    }
  }'
```

## Troubleshooting

### Common Issues

#### Export Issues

##### "Export Permission Denied"
**Symptom**: Export fails with permission denied error
**Cause**: Insufficient permissions on output directory
**Solution**:
```bash
# Check directory permissions
ls -la /var/exports/shelly-manager/

# Fix permissions
sudo chown shelly-manager:shelly-manager /var/exports/shelly-manager/
sudo chmod 755 /var/exports/shelly-manager/
```

##### "Export Size Limit Exceeded"
**Symptom**: Export fails due to size limitations
**Cause**: Export exceeds configured size limits
**Solution**:
```yaml
export:
  max_file_size: "500MB"  # Increase limit
  compression_enabled: true  # Enable compression
```

##### "Plugin Not Found"
**Symptom**: Export fails with plugin not found error
**Cause**: Requested plugin is not available or disabled
**Solution**:
```bash
# List available plugins
curl -H "Authorization: Bearer <ADMIN_KEY>" \
  http://localhost:8080/api/v1/export/plugins

# Check plugin configuration
curl -H "Authorization: Bearer <ADMIN_KEY>" \
  http://localhost:8080/api/v1/export/plugins/terraform
```

#### Import Issues

##### "Import Validation Failed"
**Symptom**: Import fails during validation stage
**Cause**: Invalid data format or missing dependencies
**Solution**:
```bash
# Run validation only to see specific errors
curl -X POST http://localhost:8080/api/v1/import/preview \
  -H "Authorization: Bearer <ADMIN_KEY>" \
  -F "file=@import.sma" \
  -F 'options={"validate_only": true}'
```

##### "Checksum Mismatch"
**Symptom**: SMA import fails with checksum error
**Cause**: File corruption or tampering
**Solution**:
1. Re-download or re-create the SMA file
2. Verify file integrity with external tools
3. Check network transfer errors

##### "Template Dependencies Missing"
**Symptom**: Import fails due to missing template references
**Cause**: Imported devices reference templates not in the system
**Solution**:
```bash
# Import templates first
curl -X POST http://localhost:8080/api/v1/import \
  -H "Authorization: Bearer <ADMIN_KEY>" \
  -F "file=@templates.json" \
  -F 'options={"sections": ["templates"]}'

# Then import devices
curl -X POST http://localhost:8080/api/v1/import \
  -H "Authorization: Bearer <ADMIN_KEY>" \
  -F "file=@devices.json" \
  -F 'options={"sections": ["devices"]}'
```

### Error Codes

#### Export Error Codes
- `EXPORT_PERMISSION_DENIED`: Insufficient permissions for export operation
- `EXPORT_SIZE_LIMIT_EXCEEDED`: Export exceeds configured size limits
- `EXPORT_PLUGIN_NOT_FOUND`: Requested export plugin not available
- `EXPORT_INVALID_CONFIG`: Invalid export configuration provided
- `EXPORT_OUTPUT_PATH_INVALID`: Invalid or unsafe output path specified

#### Import Error Codes
- `IMPORT_VALIDATION_FAILED`: Import data failed validation checks
- `IMPORT_CHECKSUM_MISMATCH`: File integrity check failed
- `IMPORT_UNSUPPORTED_FORMAT`: Unsupported import file format
- `IMPORT_DEPENDENCY_MISSING`: Required dependencies not found
- `IMPORT_CONFLICT_UNRESOLVED`: Unresolved conflicts in import data

### Debug Mode

#### Enable Debug Logging
```yaml
logging:
  level: "debug"
  export_import_debug: true
```

#### Debug Output Examples
```
2024-01-15T10:30:00Z DEBUG export starting plugin=sma format=sma
2024-01-15T10:30:01Z DEBUG export device_count=150 template_count=25
2024-01-15T10:30:02Z DEBUG export compression_ratio=0.35 original_size=1024KB compressed_size=358KB
2024-01-15T10:30:02Z DEBUG export completed duration=2.1s output_path=/backups/shelly-backup-20240115.sma
```

### Performance Optimization

#### Large Dataset Handling
```yaml
export:
  chunk_size: 1000        # Process in chunks
  compression_level: 6    # Balance compression vs speed
  parallel_processing: true

import:
  batch_size: 500         # Import in batches
  validation_threads: 4   # Parallel validation
  memory_limit: "512MB"   # Memory usage limit
```

#### Network Optimization
```yaml
api:
  timeout: "300s"         # Increase timeout for large operations
  max_request_size: "100MB"
  compression: true
```

## Migration Guide

### Upgrading from Previous Versions

#### From v0.5.3 to v0.5.4
1. **Backup Current System**: Create full backup before upgrade
2. **Update Configuration**: Add new export/import configuration sections
3. **Update Admin Key**: Ensure admin API key is properly configured
4. **Test Export/Import**: Verify functionality with test operations

#### Configuration Migration
```yaml
# Old configuration (v0.5.3)
api:
  export_enabled: true
  export_formats: ["json", "csv"]

# New configuration (v0.5.4)
security:
  admin_api_key: "${ADMIN_API_KEY}"

export:
  output_directory: "/var/exports"
  default_format: "sma"
  plugins:
    sma:
      enabled: true
    json:
      enabled: true
    csv:
      enabled: true
```

#### Database Migration
The system automatically handles database migrations for export/import history and scheduling tables. No manual intervention required.

#### API Changes
- **New Endpoints**: Added comprehensive export/import API endpoints
- **Authentication**: Export/import operations now require admin authentication
- **Response Format**: All endpoints use standardized response wrapper

### Best Practices

#### Backup Strategy
1. **Regular Backups**: Schedule daily SMA exports
2. **Multiple Locations**: Store backups in multiple locations
3. **Version Control**: Keep multiple backup versions
4. **Test Restores**: Regularly test backup restoration

#### Security Practices
1. **Strong Admin Keys**: Use cryptographically strong admin API keys
2. **Access Restriction**: Limit export/import access to authorized users
3. **Audit Logs**: Regularly review export/import audit logs
4. **Secure Storage**: Store backups in secure locations

#### Performance Guidelines
1. **Scheduled Operations**: Run large exports during off-peak hours
2. **Resource Monitoring**: Monitor system resources during operations
3. **Network Bandwidth**: Consider network impact of large transfers
4. **Storage Management**: Implement backup retention policies

---

**Documentation Version**: 1.0  
**Last Updated**: 2024-01-15  
**Compatible Versions**: Shelly Manager v0.5.4+  
**Author**: Shelly Manager Development Team