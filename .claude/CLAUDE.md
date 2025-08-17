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
**Language**: Go 1.23.0  
**Module**: `github.com/ginsys/shelly-manager`  
**Stage**: Phase 4 Complete - Configuration System Implementation Required

### Recent Status (January 2025)
- **Phase 1**: ✅ **100% COMPLETE** - Foundation Architecture
- **Phase 2**: ✅ **100% COMPLETE** - Dual-Binary Architecture with API server and provisioning agent
- **Phase 2.5**: ✅ **100% COMPLETE** - Template System Enhancement with Sprig v3 integration
- **Phase 3**: ✅ **85% COMPLETE** - JSON to Structured Configuration Migration (API complete, UI pending)
- **Authentication**: ✅ Gen1 & Gen2+ Shelly device support with Basic/Digest auth
- **Configuration**: ✅ Typed configuration models with validation and conversion utilities
- **API**: ✅ Complete REST API with 25+ endpoints including typed configuration management

### Latest Major Implementation (August 2025)
**CI/CD Pipeline & Development Quality** (Latest - commit d19a928):
- **Linting Resolution**: Fixed all 114+ golangci-lint issues, downgraded to v1.60.3 for compatibility
- **Error Handling**: Systematic error handling with proper logging throughout codebase
- **Context Safety**: Implemented typed context keys to prevent middleware collisions
- **Development Automation**: Pre-commit hooks, 90+ make targets for formatting and testing
- **Documentation**: Comprehensive TESTING.md with 40+ test commands and workflows
- **Code Quality**: Shadow variable fixes, JSON response helpers, systematic logging

**Real-time Metrics Dashboard with WebSocket Integration** (Phase 4 - commit a6337a0):
- **WebSocket Server**: Real-time metrics streaming with client management and broadcasting
- **Dashboard UI**: 5 comprehensive tabs (Overview, Devices, Drift, Notifications, Resolutions)
- **Chart.js Integration**: Interactive charts with device status and drift analysis
- **WebSocket Fixes**: Resolved Hijacker interface issues in HTTP middleware
- **Dependencies**: Added gorilla/websocket v1.5.3 for WebSocket support

### **🎯 CURRENT PRIORITY: User Interface Enhancement**
**Status**: JSON to Structured Configuration Migration 85% complete - API layer fully implemented, UI modernization needed.

**Current Gaps**:
- Form-based configuration UI still uses raw JSON editors
- Configuration wizards not implemented
- Real-time validation feedback missing in UI
- Template preview system not integrated in web interface
- Configuration diff views not implemented

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
├── configuration/# Enhanced configuration system (3-level hierarchy + resolution)
├── notification/ # Alert system (Email, Webhook, Slack) with threshold management
├── metrics/      # Prometheus monitoring with background collection
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
- **CLI Framework**: spf13/cobra v1.9.1
- **Configuration**: spf13/viper v1.20.1
- **HTTP Router**: gorilla/mux v1.8.1
- **WebSocket**: gorilla/websocket v1.5.3 for real-time metrics
- **Database ORM**: gorm.io/gorm v1.30.1 with SQLite driver
- **Database**: SQLite (requires CGO_ENABLED=1 for compilation)
- **Logging**: slog (Go 1.23+ standard library)
- **Metrics**: prometheus/client_golang v1.23.0 with custom registry
- **Scheduling**: robfig/cron/v3 v3.0.1 for automated tasks
- **Testing**: stretchr/testify v1.10.0 for comprehensive test suite

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
- `/api/v1/notifications/*` - Notification system management
- `/api/v1/metrics/*` - Metrics collection and status
- `/metrics` - Prometheus metrics endpoint
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
5. ✅ **Import device configuration from physical devices to database**
6. ✅ **Export device configuration from database to physical devices**
7. ✅ **Implement configuration drift detection and reporting**
8. ✅ **Implement bulk configuration sync operations**

### Phase 2: Notification & Resolution System - ✅ **100% COMPLETE**
1. ✅ **Create notification system core models and service** - Email, webhook, Slack channels
2. ✅ **Implement email and webhook notification channels** - Template support and error handling
3. ✅ **Add basic alert rules and test endpoints** - Configurable thresholds and severity levels
4. ✅ **Create resolution workflow models and policies** - Auto-fix categories and exclusions
5. ✅ **Implement auto-fix engine (safe mode)** - Automated drift resolution with safety controls
6. ✅ **Build manual review queue and approval workflow** - History tracking and approval process

### Phase 3: Metrics & Monitoring System - ✅ **100% COMPLETE**
1. ✅ **Implement metrics collection and Prometheus exporter** - Custom registry with drift metrics
2. ✅ **Write comprehensive tests for all metrics components** - 71+ tests with concurrent testing
3. ✅ **Background collector with configurable intervals** - Graceful shutdown and memory management
4. ✅ **HTTP middleware for request/response metrics** - Operation timing and status tracking
5. ✅ **System integration with main application lifecycle** - Service initialization and configuration

### Phase 4: Real-time Metrics Dashboard - ✅ **100% COMPLETE**
1. ✅ **WebSocket server implementation** - Real-time metrics streaming with client management
2. ✅ **Comprehensive dashboard UI** - 5 tabs with device status, drift, notifications, resolutions
3. ✅ **Chart.js integration** - Interactive charts for system status and drift analysis
4. ✅ **WebSocket middleware fixes** - Resolved Hijacker interface compatibility issues
5. ✅ **Real-time connection management** - Auto-reconnection and connection health monitoring

### **🚨 CURRENT PRIORITY: Configuration System Implementation**
**Critical Gap**: Sophisticated architecture exists but not fully implemented

1. 🔄 **Fix non-functional editDevice() function** - Currently console.log stub
2. 🔄 **Replace JSON textareas with typed forms** - Device-specific configuration interfaces
3. 🔄 **Implement NetworkConfig capability** - MQTT/WiFi/Cloud settings in UI
4. 🔄 **Replace json.RawMessage with typed structures** - Backend capability composition
5. 🔄 **Create capability-based template targeting** - Templates by capability, not device type
6. 🔄 **Add device capability detection/indicators** - Show device capabilities in UI
7. 🔄 **Implement capability-specific API endpoints** - GET/PUT per capability
8. 🔄 **Visual template builder** - Form-based configuration instead of JSON
9. 🔄 **Group configuration management** - Apply settings to device groups by capability  
10. 🔄 **Configuration hierarchy merging** - Proper system → template → device inheritance

### Future Phases (After Configuration System)
- **Phase 5**: WiFi Provisioning Implementation
- **Phase 6**: Kubernetes Deployment
- **Phase 7**: OPNSense Integration
- **Phase 8**: Production Features (backup, monitoring)
- **Phase 9**: Advanced Features (automation, scheduling)

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
**Core Tables**:
- `devices` table - Device inventory with auth credentials
- `device_configs` table - Configuration data per device
- `config_templates` table - Reusable configuration templates
- `config_histories` table - Configuration change audit trail

**Drift Detection & Resolution**:
- `drift_reports` table - Configuration drift detection results
- `drift_trends` table - Long-term drift analysis and trends
- `resolution_policies` table - Auto-fix policies and rules
- `resolution_requests` table - Manual resolution requests
- `resolution_histories` table - Resolution attempt audit trail

**Notification & Monitoring**:
- `notification_histories` table - Sent notification tracking
- `alert_rules` table - Notification threshold configuration

### Testing Status
- ✅ **Core System**: All 17 Gen1 devices successfully authenticate and import configurations
- ✅ **API Operations**: Bulk import endpoint and individual device operations working
- ✅ **Web UI**: Functional with console logging and error handling
- ✅ **Notification System**: Email, webhook, and alert rule testing complete
- ✅ **Resolution Engine**: Auto-fix engine with safe mode operation verified
- ✅ **Metrics System**: 71+ comprehensive tests covering service, collector, handlers, middleware
- ✅ **Dashboard & WebSocket**: Real-time metrics streaming with 17 devices, live updates every 5 seconds
- ✅ **System Integration**: All services properly integrated and tested together
- ❌ **Configuration UI**: Edit buttons non-functional, MQTT settings not exposed, needs typed forms

### System Architecture Status
**Current Architecture**: Monolithic application with comprehensive monitoring and automation
- **Configuration Management**: 3-level hierarchy with drift detection
- **Notification System**: Multi-channel alerting with threshold management  
- **Resolution Engine**: Automated drift fixing with safety controls
- **Metrics Collection**: Prometheus integration with background collection
- **Database Design**: Normalized schema with audit trails and history tracking

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