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
- **Features**: Full device management UI
- **Styling**: Modern gradient design with responsive layout
- **Requirements**: See [`.claude/web-ui-requirements.md`](.claude/web-ui-requirements.md) for detailed validated requirements and remaining questions

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

### Project Documentation

#### Claude-Specific Documentation
- **Development Tasks**: [`.claude/development-tasks.md`](.claude/development-tasks.md) - Numbered task list (57 tasks), priorities
- **Web UI Requirements**: [`.claude/web-ui-requirements.md`](.claude/web-ui-requirements.md) - Validated requirements, remaining questions
- **Project Memory**: [`.claude/memory.md`](.claude/memory.md) - Historical context and insights

#### User-Facing Documentation
- **Main README**: [`README.md`](README.md) - User-facing documentation and quick start
- **Roadmap**: [`docs/ROADMAP.md`](docs/ROADMAP.md) - User-facing development roadmap
- **Testing Guide**: [`docs/TESTING.md`](docs/TESTING.md) - Testing framework and commands
- **Configuration Architecture**: [`docs/DEVICE_CONFIGURATION_ARCHITECTURE.md`](docs/DEVICE_CONFIGURATION_ARCHITECTURE.md) - Composition-based design

#### Implementation Documentation
- **Configuration Implementation**: [`internal/configuration/README.md`](internal/configuration/README.md) - 3-level hierarchy details
- **Gen1 Device Specs**: [`internal/shelly/gen1/devices.md`](internal/shelly/gen1/devices.md) - Gen1 device capabilities and API endpoints
- **Gen2+ Device Specs**: [`internal/shelly/gen2/devices.md`](internal/shelly/gen2/devices.md) - Gen2+ device capabilities and RPC methods

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

## Development Status & Roadmap

### Current Implementation Status
- **Infrastructure**: ✅ Complete (packages, testing, API, web UI, Docker)
- **Shelly Communication**: ⚠️ ~40% complete (interfaces exist, implementation needed)
- **Configuration Management**: 📋 Designed but not implemented

### Development TODO List
**See**: [`.claude/development-tasks.md`](.claude/development-tasks.md) for the complete numbered task list (57 tasks across 8 phases)

### User Requirements & Constraints
- **Primary Goal**: Fully working system with advanced features for managing ~20-100 Shelly devices
- **Deployment**: Kubernetes-first with container architecture
- **Architecture**: Dual-binary design (API server + provisioning agent)
- **Security**: Basic authentication sufficient (home project)
- **IPv6**: Code prepared but not required
- **Integrations**: Export/import functionality first (JSON + Git-friendly TOML), OPNSense second

### Key Architecture Decisions

#### Composition-Based Configuration
Devices combine capability "mixins" for flexible, type-safe configuration:
- **Architecture Document**: [`docs/DEVICE_CONFIGURATION_ARCHITECTURE.md`](docs/DEVICE_CONFIGURATION_ARCHITECTURE.md)
- **Implementation Details**: [`internal/configuration/README.md`](internal/configuration/README.md)
- **Core Capabilities**: RelayConfig, PowerMeteringConfig, DimmingConfig, RollerConfig, InputConfig, LEDConfig, ColorConfig
- **Template Benefits**: Target by capability or device type with compile-time checking

#### Package Architecture
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
- always keep the progress, todo list etc up to date
- always update project memory with changed / updated insights, new analysis,
keep the todo list updated when changes were implemented
