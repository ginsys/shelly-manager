# Shelly Manager Development Context

## Current Status
Successfully implemented comprehensive configuration management system with 3-level hierarchy (System → Template → Device) and resolved all authentication issues. All 17 Gen1 Shelly devices now import configurations successfully.

## Recently Completed Work

### Authentication Issues Fixed
- Implemented proper authentication retry logic with config credential fallback
- Added bulk import endpoint at `/api/v1/config/bulk-import`
- Successfully import configuration from all 17 devices
- Authentication flow now correctly:
  1. Uses saved device credentials when available
  2. Falls back to config file credentials (admin:0pen4me) on failure
  3. Saves working credentials to devices for future use

### UI Improvements
- Removed all browser popups (alert, confirm, prompt) from web interface
- Replaced with console logging for better user experience
- Fixed auto-refresh to work in background without full page reload

### Configuration Management
- 3-level configuration hierarchy implemented (System → Template → Device)
- Configuration stored in SQLite database using GORM
- Configuration drift detection
- Template-based configuration management
- Bulk operations with sequential processing to avoid auth race conditions

## Current Issues & Next Steps

### Critical Discovery Problem
**Issue**: Discovery process overwrites existing device data and resets sync status
- After running discovery, devices lose their "synced" status
- Configuration import needs to be done again after discovery
- Discovery should be about finding NEW devices only

### Required Fixes:
1. **Fix discovery to not overwrite existing device data**
   - Discovery currently resets device state including sync status
   - Should preserve all existing configuration and status data

2. **Ensure devices are uniquely identified by MAC address**
   - Current implementation may use IP address as identifier
   - MAC address is the proper unique identifier for network devices
   - Will prevent duplicate entries when devices change IP

3. **Preserve sync status during discovery**
   - Configuration sync status should be maintained
   - Only basic connectivity info (IP address) should update if device moved

4. **Discovery should only add new devices, not update existing ones**
   - Check if device already exists before adding/updating
   - Only add truly new devices to the system

5. **Investigate why configuration sync status is lost after discovery**
   - Trace through discovery logic to find where data is being overwritten
   - Ensure existing device records are preserved

## Architecture Overview

### Package Organization & Code Visibility
The project follows Go's standard convention for private vs. public packages:

#### **Private (Internal) Code - `internal/` directory**
All implementation details are kept private using Go's `internal/` package convention:
- `internal/api/` - HTTP handlers and REST API implementation
- `internal/config/` - Configuration management using Viper
- `internal/configuration/` - Device configuration system (3-level hierarchy)
- `internal/database/` - Database models and GORM operations
- `internal/discovery/` - Device discovery logic (HTTP, mDNS, SSDP)
- `internal/logging/` - Structured logging infrastructure
- `internal/provisioning/` - WiFi provisioning system
- `internal/service/` - Core Shelly service logic with authentication
- `internal/shelly/` - Shelly device clients (Gen1/Gen2 APIs)
- `internal/testutil/` - Testing utilities and mocks

#### **Public Code - `pkg/` directory**
Currently **empty** - no public APIs exposed for external consumption.

#### **Design Philosophy**
- **Application-first architecture**: Designed as standalone Kubernetes application
- **Complete encapsulation**: All business logic kept internal and private
- **Dual-binary design**: Main API server + provisioning agent (both use internal packages)
- **No external library intent**: Not designed for import by other Go projects

#### **Future Considerations**
When the project matures, consider exposing select functionality in `pkg/` for:
- **Device discovery libraries**: `pkg/discovery/` for network scanning capabilities
- **Shelly client APIs**: `pkg/shelly/` for device communication protocols
- **Configuration management**: `pkg/config/` for template and drift detection logic
- **Provisioning utilities**: `pkg/provisioning/` for WiFi setup workflows

This would enable other projects to leverage Shelly Manager's capabilities without duplicating implementation.

### Configuration System
- **DeviceConfig**: Per-device configuration with sync tracking
- **ConfigTemplate**: Reusable configuration templates
- **ConfigHistory**: Audit trail of configuration changes
- **3-Level Hierarchy**: System defaults → Template settings → Device overrides

### Authentication System
- Config file credentials: admin/0pen4me
- Device-specific credential storage
- Automatic credential fallback and retry logic
- Client caching with mutex protection

### Key Files
- `/internal/configuration/models.go` - Configuration data models
- `/internal/configuration/service.go` - Configuration business logic
- `/internal/service/service.go` - Main service with auth retry logic
- `/internal/api/handlers.go` - REST API handlers including bulk operations
- `/web/static/index.html` - Web UI with no browser popups

### Database Schema
- `devices` table - Device inventory with auth credentials
- `device_configs` table - Configuration data per device
- `config_templates` table - Reusable configuration templates
- `config_histories` table - Configuration change audit trail

## Technical Decisions Made
1. **Sequential processing for bulk operations** - Prevents auth race conditions
2. **Config credential fallback** - Robust authentication handling
3. **Console logging over popups** - Better developer/user experience
4. **MAC address as unique ID** - Planned for discovery fix
5. **GORM for database operations** - Consistent with existing codebase

## Testing Status
- All 17 Gen1 devices successfully authenticate and import configurations
- Bulk import endpoint working correctly
- Individual device operations working
- Web UI functional with console logging

## Next Session Priorities
1. Investigate and fix discovery data overwrite issue
2. Implement MAC-based device identification
3. Ensure configuration sync status preservation
4. Test discovery with existing devices to verify no data loss