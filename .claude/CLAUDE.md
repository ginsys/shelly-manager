# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

### Build and Run
```bash
# Build the application (requires CGO for SQLite)
make build

# Run the API server
make run
# Or directly: go run ./cmd/shelly-manager server

# Run specific CLI commands after building
./bin/shelly-manager list              # List all devices
./bin/shelly-manager discover 192.168.1.0/24  # Discover devices
./bin/shelly-manager add 192.168.1.100 "Device Name"  # Add device
./bin/shelly-manager provision         # Provision unconfigured devices
```

### Development
```bash
# Install/update dependencies
make deps

# Run tests
make test

# Docker deployment
make docker-build
make docker-run
docker-compose logs -f  # View logs
```

## Project Overview

### Current Status
**Type**: Shelly smart home device manager  
**Language**: Go 1.21  
**Module**: `github.com/ginsys/shelly-manager`  
**Stage**: Phase 1 Complete - Ready for Phase 2 implementation

### Recent Status (August 2025)
- **Phase 1**: ✅ **100% COMPLETE** - Comprehensive Shelly device management
- **Configuration System**: ✅ 3-level hierarchy (System → Template → Device) implemented
- **Authentication**: ✅ Resolved all authentication issues with retry logic
- **Web UI**: ✅ Complete with error handling and no browser popups

### Critical Discovery Issue Identified
**Problem**: Discovery process overwrites existing device data and resets sync status
- After running discovery, devices lose their "synced" status
- Configuration import needs to be done again after discovery
- Discovery should only find NEW devices, not overwrite existing ones

## Architecture

### Package Organization & Code Visibility

The project follows **Go's standard convention** for package organization with a clear separation between private and public code:

#### **Private (Internal) Packages - `internal/` directory**
All business logic and implementation details are kept in the `internal/` directory, which in Go has special meaning - packages under `internal/` can only be imported by code within the same module:

```
internal/
├── api/          # HTTP handlers and REST API implementation
├── config/       # Configuration management with Viper
├── database/     # Database models and GORM operations
├── dhcp/         # DHCP reservation management
├── discovery/    # Device discovery (HTTP, mDNS, SSDP)
├── logging/      # Structured logging infrastructure
├── provisioning/ # WiFi provisioning system
├── configuration/# Enhanced configuration system (3-level hierarchy)
├── service/      # Core Shelly service logic with authentication
├── shelly/       # Shelly device clients and API implementations
│   ├── gen1/     # Gen1-specific HTTP REST API client
│   └── gen2/     # Gen2+ RPC implementation with digest auth
└── testutil/     # Testing utilities and mocks
```

#### **Public Packages - `pkg/` directory**
Currently **empty** - the project exposes no public API for external consumption. This indicates the project is designed as a **standalone application** rather than a reusable library.

#### **Design Philosophy**
- **Application-first architecture**: Built for deployment as Kubernetes services
- **Complete encapsulation**: All implementation details kept internal and private
- **Dual-binary design**: Main API server + provisioning agent (both using internal packages)
- **No external library intent**: Not designed for import by other Go projects

#### **Future Public API Considerations**
When the project matures and there's demand for external integration, consider exposing select functionality in `pkg/` for:

- **`pkg/discovery/`** - Network scanning and device discovery capabilities
- **`pkg/shelly/`** - Shelly device communication protocols and client interfaces
- **`pkg/config/`** - Configuration template system and drift detection logic
- **`pkg/provisioning/`** - WiFi setup and device provisioning workflows

This would enable other Go projects to leverage Shelly Manager's capabilities without duplicating the implementation, while maintaining the current application-focused design.

### Code Organization

### Dependencies
- **CLI Framework**: spf13/cobra v1.8.0
- **Configuration**: spf13/viper v1.18.2
- **HTTP Router**: gorilla/mux v1.8.0
- **Database ORM**: gorm.io/gorm v1.25.5 with SQLite driver
- **Database**: SQLite (requires CGO_ENABLED=1 for compilation)
- **Logging**: slog (Go 1.21+ standard library)

### Key Components

**Device Model**: Core entity with IP, MAC, type, name, firmware, status, and settings. Uses GORM with SQLite.

**Configuration System**: 3-level hierarchy with YAML-based config using Viper:
- System defaults → Template settings → Device overrides
- Configuration stored in SQLite database
- Configuration drift detection and sync tracking

**API Endpoints**: RESTful API on port 8080:
- `/api/v1/devices` - CRUD operations
- `/api/v1/devices/{id}/config` - Device configuration management
- `/api/v1/config/bulk-import` - Bulk configuration import
- `/api/v1/discover` - Trigger network discovery
- `/api/v1/provisioning/*` - WiFi provisioning operations
- `/api/v1/dhcp/*` - DHCP reservation management
- Static web UI served from `/web/static/`

**Authentication System**:
- Gen1: Basic HTTP authentication
- Gen2+: Digest authentication (RFC 2617)
- Config file credentials: admin/0pen4me
- Device-specific credential storage with automatic fallback

**WiFi Provisioning**: Platform-specific implementations for:
- Linux: NetworkManager/nmcli
- macOS: CoreWLAN/networksetup
- Windows: netsh/PowerShell

### Web Interface
- **Location**: `web/static/index.html`
- **Technology**: Vanilla JavaScript (no framework)
- **Features**: Full device management UI matching CLI functionality
- **Styling**: Modern gradient design with responsive layout
- **UI Pattern**: Console logging (no browser popups/alerts)

## Development Status & Roadmap

### Phase 1: Core Shelly Device Management - ✅ **100% COMPLETE**
1. ✅ **Complete Gen1 API implementation** - 80+ REST endpoints for all device types
2. ✅ **Complete Gen2+ RPC implementation** - 100+ RPC methods with digest authentication
3. ✅ **Implement capability-based configuration** - Type-safe capability structs with template system
4. ✅ **Implement detailed per-device configuration management** - Full CRUD with web UI error handling

**Configuration Management Features Completed**:
- 3-level configuration hierarchy (System → Template → Device)
- Configuration stored in SQLite database using GORM
- Configuration drift detection and sync status tracking
- Template-based configuration management
- Bulk operations with sequential processing to avoid auth race conditions
- Authentication retry logic with config credential fallback
- Successful import configuration from all 17 Gen1 devices

### Immediate Priorities (Phase 1 Continuation)
5. **Fix critical discovery issue** - Prevent overwriting existing device data
6. **Import device configuration from physical devices to database**
7. **Export device configuration from database to physical devices**
8. **Implement configuration drift detection and reporting**
9. **Implement bulk configuration sync operations**
10. **Complete device authentication handling**
11. **Implement real-time status polling**
12. **Add firmware version tracking and update management**
13. **Create comprehensive real device testing suite**

### Future Phases
- **Phase 2**: Dual-Binary Architecture (separate provisioning agent)
- **Phase 3**: WiFi Provisioning Implementation
- **Phase 4**: Kubernetes Deployment
- **Phase 5**: Export & Import Functionality
- **Phase 6**: OPNSense Integration
- **Phase 7**: Production Features (monitoring, backup)
- **Phase 8**: Advanced Features (WebSocket, automation)

**See**: [`.claude/development-tasks.md`](.claude/development-tasks.md) for complete numbered task list (57 tasks across 8 phases)

## Key Implementation Details

### Authentication System
- **Config file credentials**: admin/0pen4me
- **Device-specific credential storage**: Saved to devices table
- **Automatic credential fallback and retry logic**: Falls back to config credentials on failure
- **Client caching with mutex protection**: Thread-safe operations

### Configuration System
- **DeviceConfig**: Per-device configuration with sync tracking
- **ConfigTemplate**: Reusable configuration templates  
- **ConfigHistory**: Audit trail of configuration changes
- **Device Capabilities**: RelayConfig, PowerMeteringConfig, DimmingConfig, RollerConfig, InputConfig, LEDConfig, ColorConfig

### Database Schema
- `devices` table - Device inventory with auth credentials
- `device_configs` table - Configuration data per device
- `config_templates` table - Reusable configuration templates
- `config_histories` table - Configuration change audit trail

### Testing Status
- ✅ All 17 Gen1 devices successfully authenticate and import configurations
- ✅ Bulk import endpoint working correctly
- ✅ Individual device operations working
- ✅ Web UI functional with console logging

### Current Technical Issues
1. **Discovery data overwrite** - Discovery resets device state including sync status
2. **MAC address identification** - Should use MAC address as unique identifier, not IP
3. **Sync status preservation** - Configuration sync status lost after discovery

## Original Requirements & Context

### Core Functionality
Golang application for managing Shelly smart home devices with:
- **Headless operation** in containers
- **CLI interface** for all functionality
- **SQLite database** for persistence
- **Configuration file** support (YAML)
- **API server mode** for web frontend integration

### Key Features

#### Device Discovery & Management
- HTTP scanning of network ranges for Shelly devices
- mDNS/Bonjour discovery for advertised devices
- SSDP/UPnP discovery for modern devices
- Database persistence of device information
- Real-time status monitoring

#### WiFi Provisioning System
Handle unconfigured devices that expose their own WiFi SSID:
- Network interface control (requires host system access)
- WiFi AP scanning for Shelly device patterns (`shelly1-XXXXXX`, `SHSW-1#XXXXXX`)
- Automated connection to device APs (default IP: `192.168.33.1`)
- Device configuration via HTTP API (`/shelly`, `/settings`, `/status`)
- Production WiFi setup and device reboot
- Platform-specific implementations (Linux, macOS, Windows)

#### DHCP Integration
Generate DHCP reservations for OPNSense firewall:
- MAC address extraction from provisioned devices
- Hostname standardization for network management
- IP pool management with automatic assignment
- OPNSense API integration for automated reservation creation
- Export capabilities (JSON, CSV, XML formats)

### Deployment Architecture

#### Container Requirements
- **Privileged mode** required for WiFi operations
- **Host network access** for network interface control
- **Volume mounts** for data persistence
- **Device access** (/dev/rfkill for WiFi)

### Complete Device Lifecycle
1. **Discovery**: Scan for unconfigured Shelly AP networks
2. **Provisioning**: Connect to AP, configure WiFi credentials
3. **Network Integration**: Device joins production network
4. **DHCP Reservation**: Extract MAC/hostname, create reservation
5. **Firewall Sync**: Push reservations to OPNSense
6. **Device Management**: Configure settings via main application

### Shelly API Details

#### Gen1 API Endpoints
- `/shelly` - Device information
- `/status` - Current status
- `/settings` - Get/set configuration
- `/settings/relay/{id}` - Relay control
- `/settings/light/{id}` - Light control
- `/settings/login` - Authentication setup
- `/ota` - Firmware updates
- `/reboot` - Device reboot

#### Gen2+ RPC Methods
- `/rpc/Shelly.GetDeviceInfo` - Device information
- `/rpc/Shelly.GetStatus` - Current status
- `/rpc/Shelly.GetConfig` - Get configuration
- `/rpc/Switch.Set` - Switch control
- `/rpc/Light.Set` - Light control
- `/rpc/Sys.SetAuth` - Authentication setup
- `/rpc/Shelly.Update` - Firmware updates
- `/rpc/Shelly.Reboot` - Device reboot

## Documentation Structure

### Claude-Specific Documentation
- **Main Guide**: `.claude/CLAUDE.md` (this file) - Primary development guidance
- **Development Tasks**: [`.claude/development-tasks.md`](.claude/development-tasks.md) - Numbered task list (57 tasks), priorities
- **Web UI Requirements**: [`.claude/web-ui-requirements.md`](.claude/web-ui-requirements.md) - Validated UI requirements, remaining questions
- **Development Context**: [`.claude/development-context.md`](.claude/development-context.md) - Current session context and issues
- **Project Memory**: [`.claude/memory.md`](.claude/memory.md) - Historical context and insights

### User-Facing Documentation
- **Main README**: [`README.md`](README.md) - User-facing documentation and quick start
- **Roadmap**: [`docs/ROADMAP.md`](docs/ROADMAP.md) - User-facing development roadmap
- **Testing Guide**: [`docs/TESTING.md`](docs/TESTING.md) - Testing framework and commands
- **Configuration Architecture**: [`docs/DEVICE_CONFIGURATION_ARCHITECTURE.md`](docs/DEVICE_CONFIGURATION_ARCHITECTURE.md) - Composition-based design

### Implementation Documentation
- **Configuration Implementation**: [`internal/configuration/README.md`](internal/configuration/README.md) - 3-level hierarchy details
- **Gen1 Device Specs**: [`internal/shelly/gen1/devices.md`](internal/shelly/gen1/devices.md) - Gen1 device capabilities and API endpoints
- **Gen2+ Device Specs**: [`internal/shelly/gen2/devices.md`](internal/shelly/gen2/devices.md) - Gen2+ device capabilities and RPC methods

## Settings Management

- Always save settings in `.claude/settings.json` (never use `.claude/settings.local.json`)
- Memory should point to `.claude/CLAUDE.md` (this file)

## Development Guidelines

### Code Standards
- Always follow existing patterns and conventions
- Update tests with any code changes
- Keep documentation synchronized with implementation
- Use structured logging (slog) throughout
- Proper error handling with context

### Development Workflow
1. Always update project memory with insights and changes
2. Keep todo list updated when tasks are implemented
3. Ask whether to commit or extend work when code is ready
4. Always add extensive tests for new code
5. Update README.md and other documentation with changes

### Scaling Considerations

**Current (20-100 devices)**:
- SQLite performs well
- Single API instance sufficient
- Simple polling adequate

**Future (1000+ devices)**:
- PostgreSQL migration needed
- Redis caching layer
- Worker pool for discovery
- Connection pooling
- Batch API operations
- Event-driven updates
- Horizontal scaling with load balancer

## Reference Information

### Shelly API Documentation
- Official Shelly API: https://shelly-api-docs.shelly.cloud/
- Device discovery patterns and endpoints
- Configuration payload formats

### Network Integration
- OPNSense API documentation
- DHCP reservation XML format
- NetworkManager/systemd-networkd integration

### Container Deployment
- Privileged container requirements
- Host networking for WiFi access
- Volume mapping for persistence