# Shelly Device Manager

A comprehensive Golang application for managing Shelly smart home devices with Kubernetes-native architecture, dual-binary design for secure WiFi provisioning, and extensible integration capabilities.

## 🏗️ Architecture Overview

### Dual-Binary Design
- **Main API Server** (`shelly-manager`): Runs in Kubernetes, manages device database, provides REST API
- **Provisioning Agent** (`shelly-provisioner`): Runs on host with WiFi access, handles device provisioning
- **Communication**: Provisioning agent connects to main API for instructions and device registration

## 📊 Project Status

See [docs/ROADMAP.md](docs/ROADMAP.md) for the detailed development roadmap.

### ✅ Completed
- Core package architecture with clear separation
- SQLite database with GORM ORM  
- Structured logging (slog)
- Configuration management (Viper)
- HTTP REST API with Gorilla Mux
- CLI framework (Cobra)
- Basic web interface
- Discovery framework (HTTP/mDNS)
- Platform-specific WiFi interfaces (Linux/macOS/Windows)
- Comprehensive test coverage for core packages

### 🚧 In Progress
- Real Shelly device API integration
- WiFi provisioning flow implementation
- Inter-service communication protocol
- Kubernetes deployment manifests

### 📋 Planned
- DHCP reservation management
- OPNSense integration
- Device export functionality
- Monitoring and metrics
- Advanced automation features

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

# Deploy to Kubernetes
kubectl apply -f deployments/kubernetes/

# Check status
kubectl get pods -n shelly-manager
```

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

### Device Management
- `GET    /api/v1/devices` - List all devices
- `POST   /api/v1/devices` - Add new device
- `GET    /api/v1/devices/{id}` - Get device details
- `PUT    /api/v1/devices/{id}` - Update device
- `DELETE /api/v1/devices/{id}` - Delete device

### Discovery & Provisioning
- `POST   /api/v1/discover` - Trigger network discovery
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

## 🏗️ Architecture Details

### Package Structure
```
cmd/
├── shelly-manager/      # Main API server binary
└── shelly-provisioner/  # WiFi provisioning agent (planned)

internal/
├── api/                 # REST API handlers
├── config/             # Configuration management
├── database/           # Models and database operations
├── discovery/          # Device discovery (HTTP/mDNS/SSDP)
├── provisioning/       # WiFi provisioning logic
├── service/            # Business logic layer
├── logging/            # Structured logging
└── integration/        # External system integrations (planned)
    ├── opnsense/       # OPNSense API client
    └── export/         # Export formatters
```

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

### Phase 1: Core Shelly Management (Current Focus)
- [x] Package architecture
- [x] Database layer
- [x] Basic API
- [ ] Real Shelly device communication
- [ ] Device authentication
- [ ] Status polling
- [ ] Configuration management

### Phase 2: Provisioning System
- [ ] Separate provisioner binary
- [ ] API communication protocol
- [ ] WiFi AP scanning
- [ ] Device provisioning flow
- [ ] Queue management

### Phase 3: Container & Kubernetes
- [ ] Multi-stage Docker builds
- [ ] Kubernetes manifests
- [ ] Helm chart
- [ ] ConfigMaps and Secrets
- [ ] Service mesh integration

### Phase 4: Integration & Export
- [ ] Export API implementation
- [ ] OPNSense client
- [ ] DHCP reservation generation
- [ ] Bulk operations

### Phase 5: Production Features
- [ ] Monitoring and metrics
- [ ] Backup/restore
- [ ] High availability setup
- [ ] Advanced automation

## 📚 Documentation

- [API Documentation](docs/api.md) (planned)
- [Deployment Guide](docs/deployment.md) (planned)
- [Integration Guide](docs/integrations.md) (planned)
- [Development Guide](docs/development.md) (planned)

## 🤝 Contributing

This is primarily a personal project, but contributions are welcome! 

### Development Setup
```bash
# Clone repository
git clone https://github.com/ginsys/shelly-manager

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

**Current Version**: v0.2.0-alpha  
**Supported Devices**: Shelly Gen1 & Gen2+ devices  
**Minimum Go Version**: 1.21  
**Container Registry**: ghcr.io/ginsys/shelly-manager