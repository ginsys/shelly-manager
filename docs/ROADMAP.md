# Shelly Manager Roadmap

## Project Overview
Shelly Manager is a comprehensive Golang application for managing Shelly smart home devices, designed for deployment in Kubernetes with support for 20-100+ devices.

## Current Status
- ✅ **Infrastructure Complete**: API server, database, web UI, Docker support
- ⚠️ **Core Functionality In Progress**: Shelly device communication (~40% complete)
- 📋 **Architecture Designed**: Configuration management system planned

## Development Roadmap

### Phase 1: Core Device Management (In Progress)
Complete implementation of Shelly device communication and management
- Full Gen1 and Gen2+ API support
- Type-safe configuration management
- Device authentication and status polling
- Configuration import/export capabilities
- Drift detection and bulk operations

### Phase 2: Dual-Binary Architecture
Separate provisioning agent for WiFi operations
- Dedicated provisioner binary for network operations
- Task queue and agent registration system
- API communication between components

### Phase 3: WiFi Provisioning
Automated device provisioning workflow
- Shelly AP mode detection and connection
- WiFi credential configuration
- Network verification and rollback capabilities

### Phase 4: Kubernetes Deployment
Production-ready container deployment
- Optimized Docker images
- Kubernetes manifests (no Helm required)
- Health checks and resource management

### Phase 5: Data Exchange
Comprehensive import/export functionality
- JSON full backup/restore
- Git-friendly TOML format
- CSV, hosts file, and DHCP formats
- Scheduled automatic exports

### Phase 6: OPNSense Integration
Automated network management
- DHCP reservation synchronization
- Static mapping management
- Firewall alias updates

### Phase 7: Production Features
Enterprise-ready capabilities
- Prometheus metrics
- Backup and restore
- Database migrations
- Scheduled discovery
- Device grouping and automation

### Phase 8: Advanced Features
Enhanced user experience
- WebSocket real-time updates
- Advanced scheduling
- Template system
- Batch provisioning
- Network topology visualization

## Timeline Estimates
- **Phase 1**: 2-4 weeks (critical path)
- **Phases 2-4**: 3-4 weeks combined
- **Phases 5-6**: 2 weeks
- **Phases 7-8**: 3-5 weeks

## Scaling Considerations

### Current Target (20-100 devices)
- SQLite database
- Single API instance
- Simple polling

### Future Support (1000+ devices)
- PostgreSQL migration path
- Redis caching
- Horizontal scaling
- Event-driven architecture

## Contributing
This project is under active development. See [README.md](../README.md) for setup instructions and [TESTING.md](TESTING.md) for testing guidelines.