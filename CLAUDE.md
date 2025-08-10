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

# Run tests (when implemented)
make test

# Docker deployment
make docker-build
make docker-run
docker-compose logs -f  # View logs
```

## Architecture

### Current State (as of 2025-08-09)

#### Project Status
- **Type**: Shelly smart home device manager
- **Stage**: Refactored with comprehensive package structure and testing
- **Module Path**: `github.com/ginsys/shelly-manager`
- **Go Version**: 1.21

#### Recent Major Updates
✅ **Comprehensive Testing Framework** - Full test coverage for all packages
✅ **Structured Logging** - slog implementation throughout
✅ **WiFi Provisioning System** - Complete platform-specific implementations
✅ **Core Packages** - Fully tested internal packages with clear separation

#### Code Organization
The application has been refactored from a monolithic structure into well-organized packages:

```
internal/
├── api/          # HTTP handlers and REST API
├── config/       # Configuration management with Viper
├── database/     # Models and GORM operations
├── dhcp/         # DHCP reservation management
├── discovery/    # Device discovery (HTTP, mDNS, SSDP)
├── logger/       # Structured logging with slog
├── network/      # Network utilities and operations
├── platform/     # Platform-specific WiFi implementations
├── provisioning/ # WiFi provisioning system
└── shelly/       # Shelly device client and API
```

#### Dependencies
- **CLI Framework**: spf13/cobra v1.8.0
- **Configuration**: spf13/viper v1.18.2
- **HTTP Router**: gorilla/mux v1.8.0
- **Database ORM**: gorm.io/gorm v1.25.5 with SQLite driver
- **Database**: SQLite (requires CGO_ENABLED=1 for compilation)
- **Logging**: slog (Go 1.21+ standard library)

#### Web Interface
- **Location**: `web/static/index.html`
- **Technology**: Vanilla JavaScript (no framework)
- **Features**: Full device management UI with validated requirements
- **Styling**: Modern gradient design with responsive layout

##### Web UI Requirements (Validated 2025-08-10)

**Core Display & Navigation:**
- Default view: Table/list with sortable columns, card view as option
- Flexible card layouts: multiple per row vs single column
- Pagination: 10 devices default, dropdown for 20/30/50/100/All
- Search: All fields (name, IP, MAC, type, status, hostname, notes)
- Auto-refresh: Configurable intervals (1s/5s/10s/30s/off), default 30s
- Updates: Only existing devices, discovery remains separate manual process

**Device Management:**
- Core fields display: name, type, IP, MAC, model (firmware in detail view)
- Authentication status: filterable with 🔐 icon
- Device-specific UI profiles based on actual capabilities:
  - Single switch: one on/off/toggle
  - Dual switch: two separate controls
  - Dimmer: slider + on/off
  - Roller shutter: up/down/stop/position
  - Sensors: readings only, no controls
- Notes field implemented, extensible metadata design
- Modal confirmations (not browser alerts) for destructive operations
- Optimistic UI with timeout/fallback for control commands

**Configuration Management:**
- Import/export: Complete device configuration (EVERYTHING)
- Side-by-side diff UI for all config operations
- Full validation: schema, ranges, compatibility, network, dependencies, security
- Hierarchical templates: Global → Generation → Device-type → Individual
- Template inheritance support (simple implementation first)
- History tracking: configurable retention (count + time)
- View modes: full config + diff between any versions
- Device + template history/rollback (device-level priority)
- Automatic drift detection: every 4 hours (configurable)
- Any configuration change = drift
- Visual indicators + hook system for alerts
- Auto-sync option for GitOps mode

**WiFi Provisioning:**
- Separate UI page working with dedicated provisioning binary
- WiFi credentials: stored encrypted in database, not config file
- Multiple target networks supported (not fallbacks)
- No additional authentication required
- Retry logic: none for device config, 2-3 for WiFi connection issues
- Timeout: 30s default, configurable per session and in config file
- Missing device handling: mark as missing in API/UI

**DHCP Management:**
- IP assignment: auto-assign next available, allow manual override
- Single IP pool for all device types
- Hostname templates with variables: {type}, {id}, {name}, {mac-short}
- Conflict handling: report + propose fixes, user approves
- OPNSense sync: user validation required before any push
- Sync modes: manual + scheduled, both require validation
- Rollback support for failed syncs
- API credentials: encrypted in database, manual UI rotation

**System Features:**
- Comprehensive audit log for all operations
- Performance: start simple, prepare for batching at scale
- Error handling: graceful fallbacks, clear user feedback
- Security: data encryption preparation, extensible auth system

##### Remaining Web UI Validation Questions (High Priority)

**Performance & Scalability (Critical for 100+ devices):**
- Q47: Large list handling - at what device count should pagination/virtualization kick in?
- Q48: Caching strategy - cache duration for device status (none, 1s, 5s)?
- Q49: Offline mode - should UI work partially when API unavailable?
- Q50: Background operations - should long operations run in background with progress indication?

**Browser Compatibility (Deployment needs):**
- Q51: Legacy browser support - is IE11/Legacy Edge support needed?
- Q52: Design approach - desktop-first or mobile-first design approach?
- Q53: Touch gestures - support swipe actions on mobile devices?
- Q54: PWA capability - should it be installable as Progressive Web App?

**Security (Before production):**
- Q55: Authentication - will UI require login in future?
- Q56: Session management - how long should sessions remain active?
- Q57: Data encryption - should sensitive data be encrypted in local storage?
- Q58: Audit logging - track all user actions for security audit?

**UI/UX Priorities (User experience):**
- Q59: Feature prioritization - which features are blocking vs nice-to-have?
- Q60: Timeline - target completion date for each priority level?
- Q61: User feedback - how to collect and prioritize user feature requests?
- Q66: Dark mode - is dark theme a priority?
- Q67: Dashboard customization - should users customize dashboard layout?
- Q68: Notifications - in-app only or also email/push notifications?
- Q69: Accessibility - WCAG compliance level (A, AA, AAA)?

**Future Features (Lower priority):**
- Q62: Scheduling - what types of schedules (time-based, sunrise/sunset, conditions)?
- Q63: Automation - rule engine complexity (simple if-then vs complex logic)?
- Q64: Integrations - which third-party systems to integrate with first?
- Q65: Monitoring - what metrics are most important (power, uptime, response time)?

**Status: 46/69 questions answered and validated**

#### Build & Deployment
- **Build System**: Makefile with common targets (build, run, test, docker-build)
- **Docker**: Multi-stage Dockerfile for minimal image size
- **Docker Compose**: Available for local development
- **Binary Output**: `bin/shelly-manager`

### Key Components

**Device Model**: Core entity with IP, MAC, type, name, firmware, status, and settings (JSON). Uses GORM with SQLite.

**Configuration System**: YAML-based config using Viper, supports nested structures for server, database, discovery, provisioning, DHCP, and OPNSense settings.

**API Endpoints**: RESTful API on port 8080:
- `/api/v1/devices` - CRUD operations
- `/api/v1/discover` - Trigger network discovery
- `/api/v1/provisioning/*` - WiFi provisioning operations
- `/api/v1/dhcp/*` - DHCP reservation management
- Static web UI served from `/web/static/`

**Logging System**: Structured logging with slog, context propagation, and configurable levels.

**WiFi Provisioning**: Platform-specific implementations for:
- Linux: NetworkManager/nmcli
- macOS: CoreWLAN/networksetup
- Windows: netsh/PowerShell

### Platform Considerations
WiFi provisioning requires platform-specific network interface control:
- Linux: NetworkManager/wpa_supplicant
- macOS: CoreWLAN framework
- Windows: Windows.Devices.WiFi API

## Important Context

This is a Shelly smart home device manager designed for:
1. Discovering Shelly devices on the network
2. Provisioning unconfigured devices via their AP mode
3. Managing DHCP reservations for stable IP assignments
4. Integration with OPNSense firewall

The application has been fully refactored with comprehensive architecture and real implementations. All major components have unit tests and integration tests.

## Development Notes

### Key Implementation Details
- **Testing**: Comprehensive test coverage for all packages
- **Logging**: Structured logging with slog throughout
- **Error Handling**: Proper error wrapping and context
- **Platform Support**: Abstracted platform-specific operations
- **Database**: SQLite with GORM, migrations, and proper transactions
- **Configuration**: YAML file with environment variable support
- **Web Interface**: Feature-complete UI matching CLI functionality

### API Endpoints (Port 8080)
- `GET /api/v1/devices` - List all devices
- `POST /api/v1/devices` - Add new device
- `GET /api/v1/devices/{id}` - Get specific device
- `PUT /api/v1/devices/{id}` - Update device
- `DELETE /api/v1/devices/{id}` - Delete device
- `POST /api/v1/discover` - Trigger network discovery
- `GET /api/v1/provisioning/status` - Get provisioning status
- `POST /api/v1/provisioning/provision` - Start provisioning
- `GET /api/v1/dhcp/reservations` - Get DHCP reservations
- Static files served from `/web/static/`

## Settings Management

- Always save settings in `.claude/settings.json` (never use `.claude/settings.local.json`)

## Development Priorities (Updated 2025-08-10)

### User Requirements & Constraints
- **Primary Goal**: Fully working system with advanced features for managing ~20-100 Shelly devices
- **Deployment**: Kubernetes-first with container architecture
- **Architecture**: Dual-binary design (API server + provisioning agent)
- **Security**: Basic authentication sufficient (home project)
- **IPv6**: Code prepared but not required
- **Integrations**: Export functionality first, OPNSense second

### Implementation Roadmap

#### Phase 1: Core Shelly Device Management (PRIORITY 1)
**Goal**: Real device communication and management with type-safe configuration
- [ ] Shelly API client implementation (Gen1 & Gen2+)
- [ ] Device authentication handling
- [ ] Real-time status polling
- [ ] **NEW: Composition-based configuration management**
  - [ ] Implement capability-based configuration architecture
  - [ ] Replace json.RawMessage with type-safe configuration blocks
  - [ ] Import device configuration from physical devices to database
  - [ ] Update device configuration from database to physical devices
  - [ ] Verify/compare current device config against database stored config
  - [ ] Configuration drift detection and reporting
  - [ ] Bulk configuration sync operations
- [ ] Firmware version tracking
- [ ] Bulk device operations
- [ ] Error recovery and retry logic

##### NEW: Composition-Based Configuration Architecture
**Document**: `docs/DEVICE_CONFIGURATION_ARCHITECTURE.md`

The configuration system now uses a composition-based approach where devices combine multiple capability "mixins":

**Core Capabilities**:
- `RelayConfig` - Switch/relay control settings
- `PowerMeteringConfig` - Power monitoring and protection
- `DimmingConfig` - Brightness control for dimmers/lights
- `RollerConfig` - Roller shutter/motor control
- `InputConfig` - Button and input configuration
- `LEDConfig` - LED indicator settings
- `ColorConfig` - RGB/color control

**Device Examples**:
- `SHSW-1` = RelayConfig + InputConfig
- `SHSW-PM` = RelayConfig + PowerMeteringConfig + InputConfig + TempProtectionConfig  
- `SHSW-25` = Mode-specific (Relay1+Relay2 OR RollerConfig) + PowerMeteringConfig

**Template Benefits**:
- Target by capability: "All devices with power metering"
- Target by device type: "All SHPLG-S devices"
- Type-safe configuration with compile-time checking

##### Package Architecture
```
internal/
├── configuration/           # Enhanced configuration system
│   ├── capabilities.go      # Capability interfaces
│   ├── configs.go           # Configuration block definitions
│   ├── devices.go           # Device-specific compositions
│   ├── templates.go         # Enhanced template system
│   ├── service.go           # Configuration service
│   └── models.go            # Updated database models
├── shelly/
│   ├── client.go            # Main client interface
│   ├── gen1/                # Gen1-specific implementation
│   ├── gen2/                # Gen2+ RPC implementation
│   ├── models.go            # Common data models
│   ├── auth.go              # Authentication handling
│   └── factory.go           # Device client factory
```

#### Phase 2: Dual-Binary Architecture (PRIORITY 2)
**Goal**: Separate provisioning agent for WiFi operations
- [ ] Create `cmd/shelly-provisioner/` binary
- [ ] API communication protocol (REST + WebSocket)
- [ ] Queue management system
- [ ] Agent registration/heartbeat
- [ ] Task distribution
- [ ] Status reporting
- [ ] Error handling between services

#### Phase 3: WiFi Provisioning Implementation (PRIORITY 3)
**Goal**: Complete device provisioning flow
- [ ] Real AP connection logic
- [ ] Shelly AP mode detection
- [ ] Credential injection
- [ ] Device configuration
- [ ] Network verification
- [ ] Provisioning state machine
- [ ] Rollback on failure

#### Phase 4: Kubernetes Deployment (PRIORITY 4)
**Goal**: Production-ready K8s deployment
- [ ] Multi-stage Docker builds
- [ ] Kubernetes manifests (Deployment, Service, ConfigMap, Secret)
- [ ] Helm chart with values customization
- [ ] Health/readiness probes
- [ ] Resource limits and requests
- [ ] Network policies
- [ ] Persistent volume claims for database

#### Phase 5: Export & Basic Integration (PRIORITY 5)
**Goal**: Device data export in multiple formats
- [ ] JSON export with full details
- [ ] CSV export for spreadsheets
- [ ] Hosts file format
- [ ] DHCP reservation format
- [ ] MAC address list
- [ ] Bulk export API
- [ ] Scheduled exports

#### Phase 6: OPNSense Integration (PRIORITY 6)
**Goal**: Automated DHCP management
- [ ] OPNSense API client
- [ ] DHCP reservation sync
- [ ] Static mapping creation
- [ ] Lease management
- [ ] Firewall alias updates
- [ ] Error handling and rollback

#### Phase 7: Production Features (PRIORITY 7)
**Goal**: Monitoring, backup, and automation
- [ ] Prometheus metrics
- [ ] Backup/restore functionality
- [ ] Database migrations
- [ ] Scheduled discovery
- [ ] Device grouping
- [ ] Rule-based automation
- [ ] Event notifications

#### Phase 8: Advanced Features (PRIORITY 8)
**Goal**: Enhanced capabilities
- [ ] WebSocket real-time updates
- [ ] Advanced scheduling
- [ ] Template-based configuration
- [ ] Device profiles
- [ ] Batch provisioning
- [ ] Network topology visualization

### Scaling Path (Current: 20 devices → Future: 1000+ devices)

**At 100 devices (current target)**:
- SQLite performs well
- Single API instance sufficient
- Simple polling adequate

**At 1000 devices (future consideration)**:
- PostgreSQL migration needed
- Redis caching layer
- Worker pool for discovery
- Connection pooling
- Batch API operations
- Event-driven updates
- Horizontal scaling with load balancer

## Original Requirements & Context

### Core Functionality
The user requested a Golang application for managing Shelly smart home devices with:
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
- WiFi AP scanning for Shelly device patterns
- Automated connection to device APs
- Device configuration via HTTP API
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

### Shelly Device Patterns
- **AP Mode SSID**: `shelly1-XXXXXX`, `SHSW-1#XXXXXX`
- **Default AP IP**: `192.168.33.1`
- **API Endpoints**: `/shelly`, `/settings`, `/status`
- **Configuration**: JSON payload via HTTP POST

### Shelly API Details (Phase 1 Implementation)

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

#### Authentication
- **Gen1**: Basic HTTP authentication
- **Gen2+**: Digest authentication (RFC 2617)

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
- always keep README.md and other documentation up to date with any other changes
- always create and update tests with any code change
- when a piece of code is ready, always say so, and ask whether to commit, or to extend the work
- always update project memory with changed insights, new analysis, updated to lists etc.