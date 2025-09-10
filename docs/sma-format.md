# SMA (Shelly Management Archive) Format Specification

## Overview

The **Shelly Management Archive (SMA)** format is a structured, compressed archive format designed specifically for exporting and importing complete Shelly device management configurations, data, and metadata.

## Format Structure

### File Extension
- `.sma` - Standard SMA format file

### Format Type
- **JSON-based**: Human-readable and parseable format
- **Compressed**: Gzip compression for reduced file size
- **Versioned**: Forward and backward compatibility support

### Archive Structure

```json
{
  "sma_version": "1.0",
  "format_version": "2024.1",
  "metadata": {
    "export_id": "uuid",
    "created_at": "2024-01-15T10:30:00Z",
    "created_by": "user@example.com",
    "export_type": "manual|scheduled|api",
    "system_info": {
      "version": "v0.5.4-alpha",
      "database_type": "sqlite|postgresql|mysql",
      "hostname": "shelly-manager-prod",
      "total_size_bytes": 12345678,
      "compression_ratio": 0.35
    },
    "integrity": {
      "checksum": "sha256:abcd1234...",
      "record_count": 150,
      "file_count": 5
    }
  },
  "devices": [
    {
      "id": 1,
      "mac": "AB:CD:EF:12:34:56",
      "ip": "192.168.1.100",
      "type": "shelly1",
      "name": "Living Room Switch",
      "model": "SHSW-1",
      "firmware": "20231215-111232/v1.14.1-rc1",
      "status": "online|offline|unknown",
      "last_seen": "2024-01-15T10:25:00Z",
      "settings": {
        "device_specific_settings": "JSON object"
      },
      "configuration": {
        "template_id": 1,
        "config": {
          "device_specific_config": "JSON object"
        },
        "last_synced": "2024-01-15T09:00:00Z",
        "sync_status": "synced|pending|failed"
      },
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-01-15T10:00:00Z"
    }
  ],
  "templates": [
    {
      "id": 1,
      "name": "Standard Switch Template",
      "description": "Default configuration for Shelly 1 switches",
      "device_type": "shelly1",
      "generation": 1,
      "config": {
        "template_specific_config": "JSON object"
      },
      "variables": {
        "template_variables": "JSON object"
      },
      "is_default": true,
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-01-15T10:00:00Z"
    }
  ],
  "discovered_devices": [
    {
      "mac": "AB:CD:EF:12:34:57",
      "ssid": "ShellySwitch-123456",
      "model": "SHSW-25",
      "generation": 1,
      "ip": "192.168.1.101",
      "signal": -45,
      "agent_id": "agent-001",
      "discovered": "2024-01-15T10:20:00Z"
    }
  ],
  "network_settings": {
    "wifi_networks": [
      {
        "ssid": "HomeNetwork",
        "security": "WPA2",
        "priority": 1
      }
    ],
    "mqtt_config": {
      "server": "mqtt.local",
      "port": 1883,
      "username": "shelly",
      "retain": false,
      "qos": 0
    },
    "ntp_servers": ["pool.ntp.org", "time.google.com"]
  },
  "plugin_configurations": [
    {
      "plugin_name": "backup",
      "version": "1.0.0",
      "config": {
        "output_path": "/var/backups/shelly-manager",
        "compression": true,
        "backup_type": "full"
      },
      "enabled": true
    }
  ],
  "system_settings": {
    "log_level": "info",
    "api_settings": {
      "rate_limit": 100,
      "cors_enabled": true
    },
    "database_settings": {
      "connection_pool_size": 10,
      "query_timeout": "30s"
    }
  }
}
```

## File Format Details

### Compression
- **Algorithm**: Gzip (RFC 1952)
- **Level**: Level 6 (balanced compression/speed)
- **Extension**: `.sma` files are compressed JSON

### Integrity Verification
- **Checksum**: SHA-256 hash of uncompressed JSON content
- **Record Counting**: Verification of expected vs actual record counts
- **Format Validation**: JSON schema validation against SMA specification

### Version Compatibility

#### SMA Version (`sma_version`)
- **Current**: `1.0`
- **Compatibility**: Major version changes indicate breaking changes
- **Migration**: Automatic migration between compatible versions

#### Format Version (`format_version`)
- **Current**: `2024.1`
- **Purpose**: Database schema and field evolution tracking
- **Updates**: Released with Shelly Manager versions

## Data Sections

### Core Data
1. **Devices**: Complete device inventory with settings and configurations
2. **Templates**: Configuration templates and their variables
3. **Discovered Devices**: Unmanaged devices found during discovery

### Extended Data
4. **Network Settings**: WiFi, MQTT, and network configuration
5. **Plugin Configurations**: Enabled plugins and their settings
6. **System Settings**: Application-level configuration

### Metadata
- **Export Information**: Who, when, why the archive was created
- **System Information**: Source system details and versions
- **Integrity Information**: Checksums and verification data

## Import Behavior

### Conflict Resolution
- **Device MAC Conflicts**: Update existing devices with imported data
- **Template Name Conflicts**: Create new template with suffix `_imported`
- **Configuration Conflicts**: Prompt user or use configurable merge strategy

### Validation Rules
1. **Schema Validation**: Ensure JSON matches SMA specification
2. **Integrity Verification**: Verify checksums and record counts
3. **Dependency Validation**: Ensure templates exist for device configurations
4. **Network Validation**: Validate IP addresses and network settings

### Import Options
- **Dry Run**: Preview changes without applying them
- **Selective Import**: Import only specific sections (devices, templates, etc.)
- **Merge Strategy**: Overwrite, merge, or skip existing data
- **Backup Before Import**: Create backup before applying changes

## Security Considerations

### Sensitive Data Handling
- **Passwords**: WiFi passwords and MQTT credentials are included
- **API Keys**: Plugin configurations may contain sensitive keys
- **Network Information**: Internal network topology is exposed

### Security Recommendations
1. **Encryption**: Encrypt SMA files when storing or transmitting
2. **Access Control**: Restrict access to SMA files
3. **Audit Trail**: Log all import/export operations
4. **Data Sanitization**: Option to exclude sensitive fields

## Performance Characteristics

### File Size Estimates
- **Small Installation**: 50 devices → ~500KB compressed
- **Medium Installation**: 200 devices → ~2MB compressed  
- **Large Installation**: 1000 devices → ~10MB compressed

### Processing Performance
- **Export Time**: ~1-5 seconds for typical installations
- **Import Time**: ~2-10 seconds depending on conflict resolution
- **Memory Usage**: ~2-3x uncompressed file size during processing

## CLI Usage Examples

### Export to SMA Format
```bash
# Export all data to SMA format
shelly-manager export --format sma --output /backups/

# Export specific devices only
shelly-manager export --format sma --devices 1,2,3 --output /backups/

# Export with custom compression
shelly-manager export --format sma --compress --output /backups/
```

### Import from SMA Format
```bash
# Import from SMA file
shelly-manager import --format sma --file /backups/shelly-backup-20240115.sma

# Dry run to preview changes
shelly-manager import --format sma --file backup.sma --dry-run

# Import only devices and templates
shelly-manager import --format sma --file backup.sma --sections devices,templates
```

## API Endpoints

### Export SMA
```http
POST /api/v1/export/sma
Content-Type: application/json

{
  "format": "sma",
  "filters": {
    "device_ids": [1, 2, 3],
    "include_discovered": true
  },
  "options": {
    "compression": true,
    "include_metadata": true
  }
}
```

### Import SMA
```http
POST /api/v1/import/sma
Content-Type: multipart/form-data

{
  "file": "<sma-file-upload>",
  "options": {
    "dry_run": false,
    "merge_strategy": "overwrite",
    "backup_before": true
  }
}
```

## Error Handling

### Common Errors
- **Format Version Mismatch**: SMA version not supported
- **Integrity Check Failed**: Checksum mismatch or corrupted data
- **Schema Validation Failed**: JSON structure doesn't match specification
- **Dependency Missing**: Template referenced by device doesn't exist

### Recovery Strategies
- **Version Migration**: Automatic migration for compatible versions
- **Partial Import**: Import valid records, report failures
- **Rollback**: Restore from pre-import backup on critical failures
- **Data Repair**: Attempt to fix common data inconsistencies

## Future Enhancements

### Version 1.1 (Planned)
- **Incremental Archives**: Export only changes since last backup
- **Encryption Support**: Built-in encryption for sensitive data
- **Multi-Site Support**: Export/Import across multiple Shelly Manager instances

### Version 2.0 (Future)
- **Binary Format**: More efficient binary representation
- **Streaming Support**: Support for very large installations
- **Cloud Integration**: Direct export/import to/from cloud storage

---

**Specification Version**: 1.0  
**Last Updated**: 2024-01-15  
**Compatible Shelly Manager Versions**: v0.5.4+