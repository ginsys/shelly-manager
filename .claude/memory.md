# Shelly Manager Project Memory

## Project Overview
- **Type**: Shelly smart home device manager
- **Language**: Go 1.21
- **Module**: github.com/ginsys/shelly-manager
- **Stage**: Active development - Phase 1 implementation

## Documentation Reorganization (2025-08-11)
- Moved Claude-specific docs to `.claude/` directory
- Created `.claude/development-tasks.md` for task tracking
- Created `.claude/web-ui-requirements.md` for UI requirements
- Created `docs/ROADMAP.md` for user-facing roadmap
- Moved `TESTING.md` to `docs/` directory
- Updated all document references in CLAUDE.md and README.md

## Current Development Status (2025-08-10)

### ✅ Completed Tasks (4/13 Phase 1 tasks)
1. **Gen1 API Implementation** - COMPLETE
   - All /settings, /status, /control endpoints implemented
   - Advanced relay settings, dimmer control, RGBW control
   - Input configuration, LED control, schedule management
   - CoIoT and MQTT settings
   - Power monitoring for 1PM devices

2. **Gen2+ RPC Implementation** - COMPLETE
   - Full JSON-RPC protocol implementation
   - Proper digest authentication (RFC 2617)
   - All device-specific RPC methods (100+ methods)
   - Cover/Roller, Light, RGBW, Input management
   - System, Ethernet, Script, Webhook support
   - Energy meter, Bluetooth, Cloud, MQTT configuration

3. **Authentication Handling** - COMPLETE
   - Basic auth for Gen1 devices
   - Digest auth for Gen2+ devices
   - Proper challenge-response implementation

4. **Error Recovery & Retry Logic** - COMPLETE
   - Configurable retry attempts and delays
   - Proper error wrapping and context
   - Connection testing with auth detection

### 🔄 Pending Tasks (9/13 remaining)
- Task 3: Capability-based configuration (type-safe structs)
- Task 4: Per-device configuration management
- Task 5: Import device config from physical devices
- Task 6: Export device config to physical devices
- Task 7: Configuration drift detection
- Task 8: Bulk configuration sync operations
- Task 10: Real-time status polling
- Task 11: Firmware version tracking
- Task 13: Real device testing suite

## Technical Decisions & Solutions

### Import Cycle Resolution
- **Problem**: Circular dependency between shelly, gen1, and gen2 packages
- **Solution**: Factory pattern returns error, services use DetectGeneration then create clients directly
- **Files affected**: factory.go, removed wrapper files

### Authentication Implementation
- **Gen1**: Basic HTTP authentication via SetBasicAuth
- **Gen2+**: Full digest authentication (RFC 2617) in digest_auth.go
- **Location**: internal/shelly/gen2/digest_auth.go

### Interface Design
- **Main Client Interface**: internal/shelly/client.go
- Extended with roller shutter operations
- Added advanced settings methods
- RGBW operations included
- Both Gen1 and Gen2 clients fully implement the interface

## File Structure Updates
```
internal/shelly/
├── client.go           # Main Client interface (updated)
├── factory.go          # Factory for device detection
├── gen1/
│   ├── client.go       # Gen1 implementation (extended)
│   └── roller.go       # Roller shutter operations
├── gen2/
│   ├── client.go       # Gen2 implementation
│   ├── digest_auth.go  # NEW: Digest auth implementation
│   └── extended_methods.go # NEW: All RPC methods
```

## Key Implementation Details

### Gen1 Client
- Location: internal/shelly/gen1/client.go
- ~30 new methods added for complete device control
- Supports all Gen1 device types
- Retry logic with configurable attempts

### Gen2+ Client  
- Location: internal/shelly/gen2/client.go
- Digest authentication implemented
- 100+ RPC methods in extended_methods.go
- Full coverage of Pro, Plus, and specialized devices
- Component-based architecture support

## Next Priority
**Task 3**: Implement capability-based configuration
- Replace json.RawMessage with type-safe structs
- Use composition pattern from docs/DEVICE_CONFIGURATION_ARCHITECTURE.md
- Create capability interfaces and device-specific compositions

## Testing Status
- Unit tests exist but need updates for new methods
- Integration tests pending (Task 13)
- Manual testing required with real devices

## Build Status
- ✅ Project builds successfully
- ✅ No import cycles
- ✅ All packages compile independently

## Dependencies
No new dependencies added. Using standard library for:
- crypto/md5 for digest auth
- net/http for HTTP client
- encoding/json for JSON handling

## Phase 1 Progress (Tasks 1-4) - COMPLETED ✅

### Task 1-2: Gen1/Gen2+ API Implementations (COMPLETE)
- **Gen1 Client**: 80+ REST endpoints covering all device types (relay, dimmer, RGBW, roller, energy monitoring)
- **Gen2+ Client**: 100+ RPC methods including Pro series, Plus series, scripting, webhooks, sensors
- **Authentication**: Basic auth (Gen1) and digest auth (Gen2+) with retry logic
- **Error Handling**: Comprehensive error types, retry mechanisms, proper resource management
- **Device Support**: Complete coverage of Shelly 1/1PM/2.5, Dimmer 2, RGBW2, i3, Plug S, Pro 3EM, Pro 4PM, Plus series

### Task 3: Capability-Based Configuration (COMPLETE)
- **Type Safety**: Replaced json.RawMessage with structured capability configs
- **Device Capabilities**: RelayConfig, PowerMeteringConfig, DimmingConfig, RollerConfig, InputConfig, LEDConfig, ColorConfig
- **Template System**: Device-specific and universal templates with variable substitution
- **Configuration Levels**: System, Template, and Device-level configuration hierarchy
- **Database Models**: Full GORM models with proper relationships and history tracking

### Task 4: Detailed Per-Device Configuration Management (COMPLETE)
- **API Endpoints**: 
  - `GET/PUT /api/v1/devices/{id}/config` - Full device configuration CRUD
  - `PUT /api/v1/devices/{id}/config/{capability}` - Capability-specific updates (relay, dimming, roller, power-metering)
  - `PUT /api/v1/devices/{id}/config/auth` - Authentication credential management
- **Service Layer**: Complete CRUD operations with capability-specific updates
- **Database Persistence**: All configuration changes saved with history tracking
- **Change Management**: Automatic sync status tracking (synced/pending/error/drift)
- **Audit Trail**: Full history of configuration changes with user attribution

## Phase 1 Status Summary
- **Tasks 1-4**: ✅ **FULLY COMPLETE** - All core device management and configuration functionality implemented
- **Tasks 5-13**: ⏳ **READY FOR IMPLEMENTATION** - Built on solid foundation of Tasks 1-4

## Next Development Priorities
1. **Web UI Enhancement** - Create configuration editor interface for Task 4 functionality
2. **Task 5-8** - Complete remaining configuration management features (import/export/drift/bulk operations)
3. **Task 9-13** - Device operations and reliability features (auth handling, polling, firmware management, testing)

## Notes for Future Development
1. Factory pattern limitation documented - services must detect and create directly
2. Digest auth implementation is RFC 2617 compliant
3. All Shelly device generations and types now fully supported with comprehensive API coverage
4. Configuration system ready for real device integration and advanced management features