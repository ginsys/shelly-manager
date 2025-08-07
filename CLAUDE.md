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

### Current State (as of 2025-08-07)

#### Project Status
- **Type**: Shelly smart home device manager
- **Stage**: Initial development with mock implementations
- **Module Path**: `github.com/ginsys/shelly-manager` (fixed from generic path)
- **Go Version**: 1.21
- **Main File Size**: 585 lines (manageable monolithic structure)

#### Code Organization
The application is currently monolithic with all code in `cmd/shelly-manager/main.go`. This includes:
- Cobra CLI commands with subcommands (list, discover, add, provision, server)
- HTTP API server using Gorilla Mux (port 8080)
- SQLite database via GORM (stored in `data/shelly.db`)
- Device model and configuration structures
- Mock implementations for device discovery and provisioning

#### Dependencies
- **CLI Framework**: spf13/cobra v1.8.0
- **Configuration**: spf13/viper v1.18.2
- **HTTP Router**: gorilla/mux v1.8.0
- **Database ORM**: gorm.io/gorm v1.25.5 with SQLite driver
- **Database**: SQLite (requires CGO_ENABLED=1 for compilation)

#### Package Structure (Empty, Ready for Refactoring)
```
internal/
├── api/          # Reserved for HTTP handlers
├── config/       # Reserved for configuration management
├── database/     # Reserved for models & operations
├── dhcp/         # Reserved for DHCP management
├── discovery/    # Reserved for device discovery
└── provisioning/ # Reserved for WiFi provisioning
```

#### Web Interface
- **Location**: `web/static/index.html`
- **Technology**: Vanilla JavaScript (no framework)
- **Features**: Basic device listing and discovery UI
- **Styling**: Modern gradient design with responsive layout

#### Build & Deployment
- **Build System**: Makefile with common targets (build, run, test, docker-build)
- **Docker**: Multi-stage Dockerfile for minimal image size
- **Docker Compose**: Available for local development
- **Binary Output**: `bin/shelly-manager`

### Refactoring Target
The code is ready to be split into packages:
- `internal/api/` - HTTP handlers and routing
- `internal/database/` - Models and database operations
- `internal/discovery/` - Device discovery logic (HTTP scan, mDNS, SSDP)
- `internal/provisioning/` - WiFi provisioning system
- `internal/dhcp/` - DHCP reservation management
- `internal/config/` - Configuration management

### Key Components

**Device Model**: Core entity with IP, MAC, type, name, firmware, status, and settings (JSON). Uses GORM with SQLite.

**Configuration System**: YAML-based config using Viper, supports nested structures for server, database, discovery, provisioning, DHCP, and OPNSense settings.

**API Endpoints**: RESTful API on port 8080:
- `/api/v1/devices` - CRUD operations
- `/api/v1/discover` - Trigger network discovery
- Static web UI served from `/web/static/`

**Mock Implementations**: Currently uses mock data for demonstration. Real Shelly API calls need implementation at:
- Discovery: Replace mock device generation with actual HTTP/mDNS/SSDP scanning
- Provisioning: Implement WiFi connection and configuration via Shelly AP mode

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

The application was generated with comprehensive architecture but mock implementations. All mock sections are clearly marked for replacement with real Shelly API integration.

## Development Notes

### Key Implementation Details
- **Main Application**: All logic in `cmd/shelly-manager/main.go` (585 lines as of 2025-08-07)
- **Database**: SQLite with GORM, stored in `data/shelly.db`
- **Configuration**: YAML file at `configs/shelly-manager.yaml`
- **Web Interface**: Static HTML at `web/static/index.html` (vanilla JS, no framework)
- **Docker Support**: Multi-stage build for minimal image size

### Functions in main.go
- **Database Operations**: InitDB, AddDevice, GetDevices, GetDevice, UpdateDevice, DeleteDevice
- **Discovery**: DiscoverDevices (currently mock implementation)
- **HTTP Handlers**: 8 API endpoint handlers for CRUD + discovery + provisioning
- **CLI Commands**: list, discover, add, provision, server
- **Configuration**: initConfig using Viper

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

### Mock Implementations Requiring Replacement
1. **Device Discovery** (lines 142-299 in main.go)
   - Currently generates random mock devices
   - Needs: Real HTTP scanning, mDNS, SSDP protocols
   
2. **WiFi Provisioning**
   - Currently returns mock status
   - Needs: Platform-specific WiFi control implementation
   
3. **DHCP Reservations**
   - Currently returns empty list
   - Needs: Integration with DHCP server/OPNSense

## Settings Management

- Always save settings in `.claude/settings.json` (never use `.claude/settings.local.json`)

## Development TODO List

Priority tasks for improving the Shelly Manager application:

### High Priority
1. **Implement Real Shelly API Integration** - Replace mock implementations with actual Shelly HTTP API calls
   - Device discovery via HTTP scan on ports 80/443
   - mDNS/SSDP discovery protocols
   - Real device status polling
   - WiFi provisioning via Shelly AP mode

2. **Refactor to Package Structure** - Split monolithic main.go into logical packages
   - `internal/api/` → HTTP handlers (lines 301-455)
   - `internal/database/` → Models & GORM operations (lines 25-141)
   - `internal/discovery/` → Device discovery logic (lines 142-299)
   - `internal/config/` → Configuration management (lines 39-87, 502-548)
   - `internal/provisioning/` → WiFi provisioning system

3. **Add Testing** - Ensure reliability
   - Unit tests for business logic
   - Integration tests for API endpoints
   - Mock Shelly device responses for testing

### Medium Priority
4. **Enhance Error Handling & Logging**
   - Implement structured logging (consider `slog` or `zerolog`)
   - Add proper error wrapping and context
   - Implement retry logic for network operations

5. **Improve Configuration**
   - Add environment variable support for sensitive data (API keys)
   - Implement config validation
   - Add config hot-reload capability

6. **Security Enhancements**
   - Add authentication/authorization to API
   - Implement rate limiting
   - Use HTTPS for production
   - Encrypt sensitive config values

### Low Priority
7. **Enhance Web Interface**
   - Add real-time updates via WebSockets
   - Implement device grouping/filtering
   - Add dark mode toggle
   - Consider React/Vue for complex interactions

8. **Performance Optimizations**
   - Implement connection pooling for device checks
   - Add caching layer for device status
   - Use goroutine worker pools for concurrent operations

9. **Documentation**
   - API documentation (OpenAPI/Swagger)
   - Device provisioning workflow docs
   - Deployment guide for production

### Completed
✓ **Fix Module Path** - Changed from `github.com/yourusername/shelly-manager` to `github.com/ginsys/shelly-manager`