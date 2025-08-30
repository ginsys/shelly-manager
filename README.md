# Shelly Device Manager

[![CI Status](https://github.com/ginsys/shelly-manager/actions/workflows/test.yml/badge.svg?branch=main)](https://github.com/ginsys/shelly-manager/actions/workflows/test.yml)
[![Docker Publish](https://github.com/ginsys/shelly-manager/actions/workflows/docker-build.yml/badge.svg?branch=main)](https://github.com/ginsys/shelly-manager/actions/workflows/docker-build.yml)
[![codecov](https://codecov.io/gh/ginsys/shelly-manager/branch/main/graph/badge.svg)](https://codecov.io/gh/ginsys/shelly-manager)

A comprehensive Golang application for managing Shelly smart home devices with Kubernetes-native architecture, dual-binary design for secure WiFi provisioning, and advanced configuration management with complete normalization and comparison capabilities.

## 🏗️ Architecture Overview

### Dual-Binary Design
- **Main API Server** (`shelly-manager`): Runs in Kubernetes, manages device database, provides REST API with standardized responses
- **Provisioning Agent** (`shelly-provisioner`): Runs on host with WiFi access, handles device provisioning with enhanced error handling
- **Communication**: Provisioning agent connects to main API for instructions and device registration with comprehensive task orchestration

### Advanced Configuration System
- **Typed Configuration Models**: Complete structured configuration with validation for all device types
- **Configuration Normalization**: Server-side normalization for accurate comparison between raw and saved configurations
- **Bidirectional Conversion**: Seamless conversion between raw JSON and typed structures with complete field preservation
- **Enhanced Device Support**: Full support for relay devices (Shelly 1, Plus 1, etc.), smart plugs, power meters, and 3-input controllers

## 📊 Project Status

**Current Version**: v0.5.4-alpha  
**Status**: Production-ready with comprehensive security framework, 82.8% database test coverage, and advanced configuration normalization

**Repository Scale**: 165 Go files, 77,770 lines of code, 31 packages across 19 internal modules  
**Testing**: 69 test files with comprehensive security validation and isolation framework  
**API Coverage**: 112+ endpoints across 6 handler modules with standardized security responses

### Strategic Modernization Phase
The project is now entering a strategic modernization phase to transform from a basic device manager to a comprehensive infrastructure platform. With 80+ backend endpoints across 8 functional areas, only ~40% of backend functionality is currently exposed to users through the frontend.

#### Critical Assessment Findings:
- ✅ **Substantial Backend Investment**: 80+ endpoints across 8 major functional areas
- ⚠️ **Limited Frontend Integration**: Only ~40% of backend functionality exposed to users
- ❌ **Critical Systems Unexposed**: Export/Import (0%), Notification (0%), Metrics (0%) systems with zero frontend integration
- ⚠️ **Technical Debt**: 70% code duplication across 6 HTML files (9,400+ lines)

#### Planned Modernization (Phases 6.9-8):
- **Phase 6.9**: Security & Testing Foundation (Critical Prerequisite)
- **Phase 7**: Backend-Frontend Integration with security controls
- **Phase 8**: Vue.js Frontend Modernization with security-first design

### ✅ COMPLETED - Production Ready
- **Phase 1**: ✅ Core Shelly Device Management - Complete REST API, device authentication, real device integration
- **Phase 2**: ✅ Dual-Binary Architecture - API server + provisioning agent with complete inter-service communication
- **Phase 2.5**: ✅ Template System Enhancement - Sprig v3 integration, security controls, template inheritance
- **Phase 3**: ✅ JSON to Structured Migration - Typed configuration models, bidirectional conversion, API endpoints
- **Phase 4**: ✅ User Interface Enhancement - Modern structured forms, configuration wizards, real-time validation
- **Phase 5**: ✅ Container & Kubernetes Integration - Production-ready containerization and security hardening
- **Phase 5.1**: ✅ API Integration Enhancement - Complete provisioner-API communication with comprehensive testing
- **Phase 5.1.1**: ✅ Discovered Device Database Persistence - Real-time device discovery with database integration
- **Phase 5.2**: ✅ UI Modernization - Complete discovered devices integration with modern web interface
- **Phase 5.3**: ✅ Configuration Normalization & API Standardization - Complete field preservation, standardized responses
- **Phase 6.9.2**: ✅ Comprehensive Testing Foundation - COMPLETED with critical security vulnerability fixes, 82.8% database coverage (29/31 methods), 63.3% plugin registry coverage, comprehensive test isolation framework

### 🎯 Key Achievements
- **Dual-Binary Architecture**: API server (containerized) + provisioning agent (host-based) with full communication
- **Advanced Configuration System**: Complete normalization and comparison with field preservation, structured forms
- **API Standardization**: Consistent `{success: true/false, data/error}` response format across all endpoints
- **Enhanced Device Support**: Full support for all Shelly device types with accurate capability detection
- **Production Deployment**: Multi-stage Docker builds, Kubernetes manifests, security hardening
- **Real Device Integration**: Gen1 & Gen2+ Shelly devices with comprehensive API coverage and error handling
- **Database Persistence**: Discovered device storage with 24-hour TTL and automatic cleanup
- **Modern Web UI**: Real-time device discovery, configuration wizards, diff tools, responsive design with improved feedback
- **Configuration Comparison**: Server-side normalization enabling accurate configuration diff and validation
- **Comprehensive Testing**: Major testing milestone achieved with 82.8% database coverage (29/31 methods tested), 63.3% plugin registry coverage, critical security vulnerabilities resolved including rate limiting bypass, comprehensive test automation framework with isolation

### 📊 Current Capabilities
- **Device Management**: 112+ REST endpoints across 6 handler modules with standardized security responses
- **Configuration**: Advanced template-based configuration with normalization, inheritance, validation, and comparison
- **Discovery**: Real-time device discovery with database persistence and web UI integration
- **Provisioning**: Task-based orchestration between API server and provisioning agent with enhanced error handling
- **Web Interface**: Modern UI with structured forms, wizards, real-time feedback, and bulk operation progress
- **API Integration**: Comprehensive configuration comparison with server-side normalization for accurate field preservation
- **Container Support**: Production-ready Docker images and Kubernetes deployment with security hardening
- **Database Providers**: Multi-provider support with SQLite, PostgreSQL, MySQL (13 provider files)
- **Plugin Architecture**: Extensible plugin system with 19 files supporting sync, notification, and discovery
- **Security Framework**: Comprehensive security testing with vulnerability resolution and rate limiting protection

## 🚀 Quick Start

```bash
# Build the application
make build

# Start the API server
make run

# Run provisioning agent (separate binary - planned)
./bin/shelly-provisioner --api-url http://api-server:8080

# Access web interface at http://localhost:8080
```

## 🛠️ CLI Commands

### Device Management
```bash
# List all devices
./bin/shelly-manager list

# Discover devices on network
./bin/shelly-manager discover 192.168.1.0/24

# Add device manually
./bin/shelly-manager add 192.168.1.100 "Living Room Light"

# Export devices (planned)
./bin/shelly-manager export --format json > devices.json
./bin/shelly-manager export --format csv > devices.csv
```

### Server Operation
```bash
# Start API server
./bin/shelly-manager server --config /etc/shelly/config.yaml

# Start with specific port
./bin/shelly-manager server --port 8080
```

## 📦 Deployment

### Kubernetes Deployment (Primary)
```bash
# Build container
make docker-build

# Deploy to Kubernetes using Kustomize
kubectl apply -k k8s/

# Or deploy individual manifests
kubectl apply -f k8s/

# Check deployment status
kubectl get pods,svc,ingress -l app=shelly-manager
```

See [k8s/README.md](k8s/README.md) for comprehensive Kubernetes deployment documentation including TLS setup, monitoring configuration, and production considerations.

### Docker Compose (Development)
```bash
# Start services
docker-compose up -d

# View logs
docker-compose logs -f
```

### Standalone Binary
```bash
# Install
make install

# Run with systemd (Linux)
sudo systemctl start shelly-manager
```

## 🔌 API Endpoints

All API endpoints return standardized responses in the format: `{success: true/false, data/error: ...}`

### Device Management
- `GET    /api/v1/devices` - List all devices with standardized response format
- `POST   /api/v1/devices` - Add new device
- `GET    /api/v1/devices/{id}` - Get device details
- `PUT    /api/v1/devices/{id}` - Update device
- `DELETE /api/v1/devices/{id}` - Delete device

### Configuration Management
- `GET    /api/v1/devices/{id}/config` - Get saved device configuration
- `PUT    /api/v1/devices/{id}/config` - Update device configuration
- `GET    /api/v1/devices/{id}/config/current` - Get current live device configuration
- `GET    /api/v1/devices/{id}/config/current/normalized` - Get normalized current configuration
- `GET    /api/v1/devices/{id}/config/typed` - Get typed configuration with conversion info
- `GET    /api/v1/devices/{id}/config/typed/normalized` - Get normalized typed configuration
- `POST   /api/v1/devices/{id}/config/import` - Import configuration from device
- `GET    /api/v1/devices/{id}/config/status` - Get import status
- `POST   /api/v1/devices/{id}/config/export` - Export configuration to device

### Discovery & Provisioning
- `POST   /api/v1/discover` - Trigger network discovery
- `GET    /api/v1/provisioner/discovered-devices` - Get discovered devices list
- `POST   /api/v1/provisioner/report-device` - Report discovered device
- `GET    /api/v1/provisioning/status` - Provisioning status
- `POST   /api/v1/provisioning/start` - Start provisioning
- `GET    /api/v1/provisioning/queue` - List devices awaiting provisioning

### Export & Integration
- `GET    /api/v1/export?format=json` - Export devices as JSON
- `GET    /api/v1/export?format=csv` - Export devices as CSV
- `GET    /api/v1/export?format=hosts` - Export as hosts file
- `GET    /api/v1/dhcp/reservations` - Get DHCP reservations
- `POST   /api/v1/integrations/opnsense/sync` - Sync with OPNSense

### System
- `GET    /health` - Health check
- `GET    /ready` - Readiness probe
- `GET    /metrics` - Prometheus metrics (planned)

## ⚙️ Configuration Management Features

### Advanced Configuration System
The Shelly Manager includes a comprehensive configuration management system with the following key features:

#### Configuration Normalization
- **Server-Side Normalization**: All configurations are normalized on the server side for accurate comparison
- **Complete Field Preservation**: All device configuration fields are captured and preserved, including raw fields
- **Bidirectional Conversion**: Seamless conversion between raw JSON and typed structures
- **Comparison-Ready Format**: Normalized configurations enable accurate diff and comparison operations

#### Device Support Matrix
| Device Type | Model Examples | Supported Features |
|-------------|----------------|-------------------|
| **Relay Switches** | Shelly 1, Shelly Plus 1, SHSW-1 | Relay configuration, auto-on/off, default state, button type |
| **Smart Plugs** | Shelly Plug, Shelly Plus Plug | Power metering, relay control, energy monitoring |
| **Power Meter Switches** | Shelly 1PM, Shelly Plus 1PM | Power monitoring, relay control, energy thresholds |
| **Multi-Input Controllers** | SHIX3-1 (3-Input) | Input configuration, button types, timing settings |

#### Configuration Features
- **Typed Configuration Models**: Complete structured models for all device configuration sections
- **Template-Based Configuration**: Sprig v3 template engine with security controls and inheritance
- **Real-Time Validation**: Configuration validation with device-specific context and warnings
- **Configuration Diff**: Visual comparison between current device state and saved configuration
- **Bulk Operations**: Mass configuration import/export with progress feedback

#### API Integration
- **Standardized Responses**: All API endpoints return consistent `{success: boolean, data/error}` format
- **Enhanced Error Handling**: Comprehensive error responses with actionable information
- **Configuration Endpoints**: Multiple endpoints for different configuration views (raw, typed, normalized)

## 🏗️ Architecture Details

### Package Structure
```
cmd/
├── shelly-manager/      # Main API server binary
└── shelly-provisioner/  # WiFi provisioning agent

internal/
├── api/                 # REST API handlers and configuration normalization
├── config/              # Application configuration management
├── configuration/       # Device configuration models and services
├── database/            # Models and database operations
├── discovery/           # Device discovery (HTTP/mDNS/SSDP)
├── provisioning/        # WiFi provisioning logic and network interfaces
├── service/             # Business logic layer
├── logging/             # Structured logging
├── metrics/             # Metrics collection and monitoring
└── integration/         # External system integrations (planned)
    ├── opnsense/        # OPNSense API client
    └── export/          # Export formatters
```

### Key Components

#### Configuration System (`internal/api/config_normalizer.go`)
- **NormalizedConfig**: Unified configuration structure for comparison
- **Bidirectional Conversion**: Raw JSON ↔ Typed Configuration ↔ Normalized Format
- **Complete Field Preservation**: Captures all device fields including unknown/additional fields
- **Device-Aware Processing**: Handles device-specific capabilities and configurations

#### API Handlers (`internal/api/`)
- **Standardized Responses**: Consistent `{success, data/error}` format across all endpoints
- **Configuration Endpoints**: Multiple views of device configuration (raw, typed, normalized)
- **Enhanced Error Handling**: Comprehensive error context and actionable messages
- **Bulk Operation Support**: Progress feedback and status reporting

#### Device Configuration (`internal/configuration/`)
- **Typed Models**: Complete structured models for all configuration sections
- **Template Engine**: Sprig v3 integration with security controls
- **Service Layer**: Business logic for configuration conversion and validation
- **Device Support**: Relay switches, smart plugs, power meters, input controllers

### Scaling Considerations

**Current Design (20-100 devices)**
- SQLite database (sufficient for <1000 devices)
- Single API server instance
- In-memory caching for device status
- Polling-based discovery

**Future Scaling (1000+ devices)**
- Migration path to PostgreSQL
- Horizontal scaling with Redis cache
- Event-driven architecture with message queue
- Batch operations for bulk updates
- Connection pooling for device communications
- Partitioned discovery with worker pools

## 🔧 Configuration

### Environment Overrides
- Supports environment variable overrides with `SHELLY_` prefix and nested key mapping using underscores.
- Precedence: environment > config file > defaults.
- Examples:
  - `SHELLY_SERVER_PORT=9091` overrides `server.port`.
  - `SHELLY_SERVER_LOG_LEVEL=debug` overrides `server.log_level`.
  - `SHELLY_DATABASE_PROVIDER=postgresql` overrides `database.provider`.
  - `SHELLY_DATABASE_DSN="host=localhost user=app dbname=shelly sslmode=disable"` overrides `database.dsn`.
  - Arrays and complex types should be configured via file where possible; use env for scalars and secrets.


### Main API Server (`/etc/shelly/config.yaml`)
```yaml
server:
  port: 8080
  host: 0.0.0.0

database:
  path: /var/lib/shelly/shelly.db
  
discovery:
  networks:
    - 192.168.1.0/24
  timeout: 5s
  concurrent_scans: 10

provisioning:
  wifi_ssid: "ProductionNetwork"
  wifi_password: "${WIFI_PASSWORD}"  # From environment
  device_password: "${DEVICE_PASSWORD}"
```

### Provisioning Agent (`/etc/shelly/provisioner.yaml`)
```yaml
api:
  url: http://shelly-api.default.svc.cluster.local:8080
  key: "${API_KEY}"

scanner:
  interval: 60s
  interface: wlan0  # WiFi interface to use

provisioning:
  timeout: 30s
  retry_count: 3
```

## 🔒 Security Features

### Current Implementation
- Environment variable support for secrets
- Input validation on all API endpoints
- SQL injection prevention via ORM
- Structured logging (no secrets in logs)

### Planned Security
- API key authentication
- Rate limiting
- HTTPS/TLS support
- Encrypted configuration storage
- Audit logging

## 🧪 Testing

```bash
# Run all tests
make test

# Run with coverage
make test-coverage

# Run specific package tests
go test ./internal/discovery/...

# Run integration tests
make test-integration
```

## 📊 Monitoring & Observability

### Implemented
- Structured logging with slog
- Request/response logging middleware
- Error tracking and reporting

### Planned
- Prometheus metrics endpoint
- Health check endpoints for Kubernetes
- Distributed tracing support
- Custom Grafana dashboards

## 🔄 Integration Options

### Export Formats (Priority 1)
- **JSON**: Full device details for programmatic access
- **CSV**: Spreadsheet-compatible format
- **Hosts**: Unix hosts file format
- **DHCP**: ISC DHCP format

### OPNSense Integration (Priority 2)
- Automatic DHCP reservation sync
- Firewall rule generation
- Alias management

### Future Integrations
- Home Assistant discovery
- MQTT publishing
- Webhook notifications
- Prometheus service discovery

## 🚦 Development Roadmap

### Phase 1: Core Shelly Management - ✅ COMPLETE
- [x] Package architecture
- [x] Database layer
- [x] Complete REST API with all endpoints
- [x] Real Shelly device communication (Gen1 & Gen2+)
- [x] Device authentication (Basic & Digest auth)
- [x] Status polling and energy monitoring
- [x] Comprehensive configuration management
- [x] Web UI with error handling and authentication flow

### Strategic Modernization Roadmap

#### Phase 6.9: Security & Testing Foundation (Critical Prerequisite) ✅ **MAJOR PROGRESS**
- [ ] RBAC framework for 80+ API endpoints
- [ ] JWT authentication system for Vue.js SPA integration  
- [x] **COMPLETED**: Comprehensive testing strategy with security vulnerability resolution
  - [x] Fixed 6+ critical security issues including rate limiting bypass vulnerability
  - [x] Achieved 82.8% database manager test coverage (29/31 methods)
  - [x] Added comprehensive Plugin Registry tests (0% → 63.3% coverage)
  - [x] Implemented test isolation framework and systematic quality validation
- [ ] Resource validation and phase coordination protocols

#### Phase 7: Backend-Frontend Integration (Critical Priority)
- [ ] Database abstraction completion (PostgreSQL, MySQL)
- [ ] API standardization with security headers
- [ ] Export/Import system integration (21 endpoints) with encryption
- [ ] Notification system integration (7 endpoints) with access controls
- [ ] Metrics system enhancement with WebSocket security
- [ ] Real-time features with authentication and rate limiting

#### Phase 8: Vue.js Frontend Modernization (High Priority)
- [ ] Vue.js 3 + TypeScript foundation with security configuration
- [ ] API integration layer with authentication
- [ ] Core component development with input validation
- [ ] Advanced features UI with secure file handling
- [ ] Real-time dashboard with security validation
- [ ] Production deployment with penetration testing

#### Future Enhancements (Optional)
- [ ] Composite Devices feature for advanced device grouping
- [ ] Monitoring and metrics (Prometheus)
- [ ] High availability setup
- [ ] Advanced automation features
- [ ] Enhanced security features

## 📚 Documentation

- [API Documentation](docs/api.md) (planned)
- [Deployment Guide](docs/deployment.md) (planned)
- [Integration Guide](docs/integrations.md) (planned)
- [Development Guide](docs/development.md) (planned)
- [Repository Guidelines](AGENTS.md)
 - [Tasks & Roadmap](TASKS.md) — single source of truth for ongoing work

## 🤝 Contributing

This is primarily a personal project, but contributions are welcome! 

### Development Setup

See the contributor guide in [AGENTS.md](AGENTS.md) for project structure, commands, coding style, testing, and PR requirements.
Before starting work, review [TASKS.md](TASKS.md); it is the single source of truth for current priorities and acceptance criteria.

**Prerequisites:**
- Go 1.23 or later (managed via [mise](https://mise.jdx.dev/))
- SQLite3 development libraries
- GCC (for CGO compilation)

```bash
# Clone repository
git clone https://github.com/ginsys/shelly-manager

# Install Go 1.23 (if using mise)
mise install

# Install dependencies
make deps

# Run tests
make test

# Build binaries
make build
```

## 📝 License

MIT License - See [LICENSE](LICENSE) file for details

## 🔗 Resources

- [Shelly API Documentation](https://shelly-api-docs.shelly.cloud/)
- [OPNSense API Reference](https://docs.opnsense.org/development/api.html)
- [Kubernetes Deployment Best Practices](https://kubernetes.io/docs/concepts/workloads/)

---

**Current Version**: v0.5.4-alpha  
**Status**: Production-ready with comprehensive testing foundation, security hardening, and advanced configuration normalization  
**Testing Coverage**: 82.8% database coverage, critical security vulnerabilities resolved, comprehensive test automation  
**Supported Devices**: Shelly Gen1 & Gen2+ devices with comprehensive configuration support  
**Minimum Go Version**: 1.21  
**Container Registry**: ghcr.io/ginsys/shelly-manager  
**Architecture**: Dual-binary (API server + provisioning agent) with standardized API responses and comprehensive testing
- CORS & Proxy Settings (Security)
  - Configure via `security` in config or `SHELLY_SECURITY_*` env keys:
    - `security.use_proxy_headers`: whether to trust proxy headers for client IP.
    - `security.trusted_proxies`: list of trusted proxies (IPs/CIDRs) for `X-Forwarded-For` parsing.
    - `security.cors.allowed_origins`: list of allowed origins (empty = allow all; set explicit origins for production).
    - `security.cors.allowed_methods`, `security.cors.allowed_headers`, `security.cors.max_age`.
  - Example (YAML):
    ```yaml
    security:
      use_proxy_headers: true
      trusted_proxies:
        - 10.0.0.0/8
        - 192.168.0.0/16
      cors:
        allowed_origins:
          - https://app.example.com
        allowed_methods: [GET, POST, PUT, DELETE, OPTIONS]
        allowed_headers: [Content-Type, Authorization, X-Requested-With]
        max_age: 86400
    ```
  - Example (env):
    - `SHELLY_SECURITY_USE_PROXY_HEADERS=true`
    - `SHELLY_SECURITY_TRUSTED_PROXIES=10.0.0.0/8,192.168.0.0/16`
    - `SHELLY_SECURITY_CORS_ALLOWED_ORIGINS=https://app.example.com`
    - `SHELLY_SECURITY_CORS_ALLOWED_METHODS=GET,POST,PUT,DELETE,OPTIONS`
    - `SHELLY_SECURITY_CORS_ALLOWED_HEADERS=Content-Type,Authorization,X-Requested-With`
    - `SHELLY_SECURITY_CORS_MAX_AGE=86400`
