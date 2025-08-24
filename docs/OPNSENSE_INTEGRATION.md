# OPNSense Integration for Shelly Manager

This document provides comprehensive documentation for integrating Shelly Manager with OPNSense firewalls for automated DHCP and firewall management.

## Overview

The OPNSense integration enables:

- **Automated DHCP Reservations**: Sync Shelly device IP/MAC mappings to OPNSense DHCP server
- **Firewall Alias Management**: Automatically update firewall aliases with device IP addresses  
- **Bidirectional Synchronization**: Import existing OPNSense reservations and resolve conflicts
- **Real-time Updates**: Optional webhook support for instant synchronization
- **Advanced Templates**: Flexible export formats using powerful template engine

## Architecture

```
┌─────────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Shelly Manager    │────│  OPNSense Plugin │────│   OPNSense      │
│   Export Engine     │    │                  │    │   Firewall      │
└─────────────────────┘    └──────────────────┘    └─────────────────┘
          │                           │                        │
          ▼                           ▼                        ▼
┌─────────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Device Database   │    │  Sync Service    │    │  DHCP Server    │
│   Configuration     │    │  Conflict Res.   │    │  Firewall Rules │
└─────────────────────┘    └──────────────────┘    └─────────────────┘
```

## Installation

### 1. Enable OPNSense API

In OPNSense web interface:

1. Go to **System > Access > Users**
2. Create a new user for Shelly Manager
3. Go to **System > Access > Groups** 
4. Create group with appropriate permissions:
   - `DHCP Leases: Read/Write`
   - `Firewall: Aliases: Read/Write`
   - `System: Configuration History: Read`

### 2. Generate API Credentials

1. Go to **System > Access > Users**
2. Edit the Shelly Manager user
3. Generate API key and secret
4. Note down the credentials for configuration

### 3. Configure Shelly Manager

Add OPNSense configuration to your `shelly-manager.yaml`:

```yaml
export:
  opnsense:
    enabled: true
    host: "192.168.1.1"
    port: 443
    use_https: true
    api_key: "${OPNSENSE_API_KEY}"
    api_secret: "${OPNSENSE_API_SECRET}"
    insecure_skip_verify: false  # Set true for self-signed certificates
    
    # DHCP Configuration
    dhcp_interface: "lan"
    hostname_template: "shelly-{{.Type}}-{{.MAC | last4}}"
    
    # Firewall Configuration  
    firewall_alias_name: "shelly_devices"
    
    # Sync Settings
    sync_mode: "bidirectional"
    conflict_resolution: "manager_wins"
    apply_changes: true
    backup_before_changes: true
```

## Configuration Options

### Basic Settings

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `host` | string | - | OPNSense hostname or IP address |
| `port` | number | 443 | OPNSense API port |
| `use_https` | boolean | true | Use HTTPS for API connections |
| `api_key` | string | - | OPNSense API key (required) |
| `api_secret` | string | - | OPNSense API secret (required) |
| `insecure_skip_verify` | boolean | false | Skip TLS certificate verification |
| `timeout` | number | 30 | API request timeout in seconds |

### DHCP Settings

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `dhcp_interface` | string | "lan" | DHCP interface name |
| `hostname_template` | string | "shelly-{{.Type}}-{{.MAC \| last4}}" | Hostname generation template |
| `include_discovered` | boolean | false | Include discovered devices |

### Firewall Settings

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `firewall_alias_name` | string | "shelly_devices" | Firewall alias name |
| `sync_firewall` | boolean | true | Enable firewall alias sync |

### Sync Settings

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `sync_mode` | string | "unidirectional" | `unidirectional` or `bidirectional` |
| `conflict_resolution` | string | "manager_wins" | `manager_wins`, `opnsense_wins`, `skip`, `manual` |
| `apply_changes` | boolean | true | Automatically apply configuration |
| `backup_before_changes` | boolean | true | Create backup before changes |

## Usage Examples

### 1. Basic DHCP Synchronization

```bash
# Export DHCP reservations to OPNSense
curl -X POST http://localhost:8080/api/v1/export \
  -H "Content-Type: application/json" \
  -d '{
    "plugin_name": "opnsense",
    "format": "dhcp_reservations",
    "config": {
      "host": "192.168.1.1",
      "api_key": "your_api_key",
      "api_secret": "your_api_secret",
      "dhcp_interface": "lan",
      "apply_changes": true
    }
  }'
```

### 2. Firewall Alias Update

```bash
# Update firewall alias with device IPs
curl -X POST http://localhost:8080/api/v1/export \
  -H "Content-Type: application/json" \
  -d '{
    "plugin_name": "opnsense",
    "format": "firewall_aliases",
    "config": {
      "host": "192.168.1.1", 
      "api_key": "your_api_key",
      "api_secret": "your_api_secret",
      "firewall_alias_name": "shelly_devices"
    }
  }'
```

### 3. Bidirectional Synchronization

```bash
# Perform bidirectional sync with conflict resolution
curl -X POST http://localhost:8080/api/v1/export \
  -H "Content-Type: application/json" \
  -d '{
    "plugin_name": "opnsense",
    "format": "bidirectional_sync",
    "config": {
      "host": "192.168.1.1",
      "api_key": "your_api_key", 
      "api_secret": "your_api_secret",
      "conflict_resolution": "manager_wins",
      "import_from_opnsense": true,
      "export_to_opnsense": true
    }
  }'
```

### 4. Preview Mode

```bash
# Preview changes before applying
curl -X POST http://localhost:8080/api/v1/export/preview \
  -H "Content-Type: application/json" \
  -d '{
    "plugin_name": "opnsense",
    "format": "dhcp_reservations",
    "config": {
      "host": "192.168.1.1",
      "api_key": "your_api_key",
      "api_secret": "your_api_secret"
    }
  }'
```

## Hostname Templates

The hostname template system supports various placeholders and functions:

### Available Placeholders

- `{{.Type}}` - Device type (e.g., "ShellyPlus1")
- `{{.Name}}` - Device name
- `{{.MAC}}` - Full MAC address
- `{{.MAC | last4}}` - Last 4 characters of MAC
- `{{.IP}}` - Device IP address
- `{{.Model}}` - Device model

### Template Examples

```yaml
# Basic template
hostname_template: "shelly-{{.Type}}-{{.MAC | last4}}"
# Result: shelly-shellyplus1-a1b2

# Name-based template
hostname_template: "{{.Name | lower | replace \" \" \"-\"}}"
# Result: kitchen-light

# Complex template with conditionals
hostname_template: "{{if eq .Type \"ShellyPlus1\"}}switch{{else if eq .Type \"ShellyPlugS\"}}plug{{else}}device{{end}}-{{.MAC | last4}}"
# Result: switch-a1b2
```

## Conflict Resolution Strategies

When bidirectional sync is enabled, conflicts between Shelly Manager and OPNSense data are resolved based on the `conflict_resolution` setting:

### 1. manager_wins (Default)
- Shelly Manager data takes precedence
- OPNSense reservations are updated to match Shelly Manager
- Recommended for most use cases

### 2. opnsense_wins  
- OPNSense data takes precedence
- Shelly Manager data is updated from OPNSense
- Useful when OPNSense is the authoritative source

### 3. skip
- Conflicted devices are skipped during sync
- No changes made to either system
- Safe option when manual review is needed

### 4. manual
- Conflicts are flagged but not automatically resolved
- Requires manual intervention to resolve
- Best for environments requiring strict change control

## Scheduled Synchronization

### Using Cron Jobs

```bash
# Add to crontab for hourly sync
0 * * * * curl -X POST http://localhost:8080/api/v1/export \
  -H "Content-Type: application/json" \
  -d '@/etc/shelly-manager/opnsense-sync.json' >/dev/null 2>&1
```

### Configuration File Example

```json
{
  "plugin_name": "opnsense",
  "format": "bidirectional_sync",
  "config": {
    "host": "192.168.1.1",
    "api_key": "${OPNSENSE_API_KEY}",
    "api_secret": "${OPNSENSE_API_SECRET}",
    "conflict_resolution": "manager_wins",
    "apply_changes": true
  }
}
```

## Monitoring and Troubleshooting

### Log Analysis

Monitor Shelly Manager logs for OPNSense integration issues:

```bash
# Filter OPNSense-related logs
journalctl -u shelly-manager | grep -i opnsense

# Watch live logs
journalctl -u shelly-manager -f | grep -i opnsense
```

### Common Issues

#### 1. API Authentication Failures

**Error**: `HTTP 401: Unauthorized`

**Solutions**:
- Verify API key and secret are correct
- Check user permissions in OPNSense
- Ensure API access is enabled

#### 2. Connection Timeouts

**Error**: `connection timed out`

**Solutions**:
- Check network connectivity to OPNSense
- Verify firewall rules allow API access
- Increase timeout value in configuration

#### 3. Certificate Issues

**Error**: `certificate verification failed`

**Solutions**:
- Use valid SSL certificate on OPNSense
- Set `insecure_skip_verify: true` for self-signed certificates
- Add CA certificate to system trust store

#### 4. DHCP Interface Not Found

**Error**: `interface not found`

**Solutions**:
- Check interface name in OPNSense
- Verify DHCP service is running on interface
- Use correct interface identifier (e.g., "lan", "opt1")

### Testing Connectivity

```bash
# Test API connectivity
curl -k -u "api_key:api_secret" \
  https://192.168.1.1/api/core/system/status

# Verify DHCP service status  
curl -k -u "api_key:api_secret" \
  https://192.168.1.1/api/dhcp/leases/searchReservations
```

## Security Considerations

### API Access Control

1. **Dedicated User**: Create dedicated user for Shelly Manager
2. **Minimal Permissions**: Grant only required permissions
3. **API Key Rotation**: Rotate keys periodically
4. **Network Restrictions**: Limit API access by source IP

### Data Protection

1. **Encrypted Credentials**: Store API credentials encrypted
2. **Secure Communication**: Always use HTTPS
3. **Audit Logging**: Enable audit logging in OPNSense
4. **Backup Security**: Secure configuration backups

### Best Practices

```yaml
export:
  opnsense:
    # Use environment variables for credentials
    api_key: "${OPNSENSE_API_KEY}"
    api_secret: "${OPNSENSE_API_SECRET}"
    
    # Always verify certificates in production
    insecure_skip_verify: false
    
    # Create backups before changes
    backup_before_changes: true
    
    # Use dry run for testing
    dry_run: false  # Set to true for testing
```

## Advanced Configuration

### Custom Templates

Create custom export templates for specific formats:

```yaml
# Custom template plugin manifest
name: "custom_opnsense"
version: "1.0.0"
description: "Custom OPNSense export format"
supported_formats: ["custom_dhcp"]

templates:
  custom_dhcp: |
    {{range .Devices}}
    # Device: {{.Name}}
    host {{.Name | sanitize}} {
        hardware ethernet {{.MAC}};
        fixed-address {{.IP}};
        {{if .Type}}option host-name "{{.Name}}";{{end}}
    }
    {{end}}

variables:
  network_domain: "local"
  dns_servers: ["192.168.1.1", "1.1.1.1"]
```

### Multiple OPNSense Instances

Configure multiple OPNSense instances:

```yaml
export:
  opnsense_main:
    host: "192.168.1.1"
    api_key: "${OPNSENSE_MAIN_KEY}"
    api_secret: "${OPNSENSE_MAIN_SECRET}"
    dhcp_interface: "lan"
    
  opnsense_backup:
    host: "192.168.2.1"
    api_key: "${OPNSENSE_BACKUP_KEY}"
    api_secret: "${OPNSENSE_BACKUP_SECRET}"
    dhcp_interface: "lan"
```

## Performance Optimization

### Large Deployments (100+ Devices)

```yaml
export:
  opnsense:
    # Batch processing for large datasets
    batch_size: 50
    
    # Parallel processing
    concurrency: 3
    
    # Connection pooling
    max_connections: 5
    
    # Caching
    cache_duration: "5m"
    
    # Rate limiting
    rate_limit: "10/s"
```

### Monitoring Integration

```yaml
metrics:
  export:
    opnsense:
      # Track sync metrics
      track_duration: true
      track_success_rate: true
      track_conflict_count: true
      
      # Alerting thresholds
      max_duration: "30s"
      min_success_rate: 0.95
      max_conflicts: 5
```

## Migration Guide

### From Manual DHCP Management

1. **Export Current Reservations**: Export existing OPNSense DHCP reservations
2. **Import to Shelly Manager**: Import devices to Shelly Manager database
3. **Configure Sync**: Set up bidirectional sync with `opnsense_wins` initially
4. **Test Sync**: Run sync in dry-run mode to verify behavior
5. **Switch to Manager Control**: Change to `manager_wins` after validation

### From Other Management Systems

1. **Export Data**: Export device data from existing system
2. **Transform Format**: Convert to Shelly Manager format
3. **Import Data**: Import devices via API
4. **Configure OPNSense**: Set up OPNSense integration
5. **Sync**: Perform initial synchronization

## API Reference

### Export Endpoints

- `POST /api/v1/export` - Perform export
- `POST /api/v1/export/preview` - Preview export
- `GET /api/v1/export/plugins` - List available plugins
- `GET /api/v1/export/plugins/opnsense` - Get OPNSense plugin info

### Configuration Endpoints  

- `GET /api/v1/export/config` - Get export configuration
- `PUT /api/v1/export/config` - Update export configuration
- `POST /api/v1/export/test` - Test export configuration

For complete API documentation, see [API_REFERENCE.md](API_REFERENCE.md).

## Support and Contributing

- **Issues**: Report issues on GitHub
- **Documentation**: Contribute to documentation improvements  
- **Plugin Development**: See [PLUGIN_DEVELOPMENT.md](PLUGIN_DEVELOPMENT.md)
- **Community**: Join discussions in GitHub Discussions

---

**Last Updated**: August 2024  
**Version**: 1.0.0  
**Compatibility**: Shelly Manager v0.5.3+, OPNSense 23.1+