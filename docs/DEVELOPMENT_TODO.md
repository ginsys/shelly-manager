# Shelly Manager Development TODO List

## Current Status Overview

### ✅ Completed Infrastructure
- **Package Organization**: Well-structured `/internal/` packages with clear separation
- **Testing Infrastructure**: Comprehensive test files covering all major packages  
- **CLI Interface**: Full Cobra-based CLI with all planned commands
- **Database Layer**: GORM + SQLite with proper migrations
- **Configuration System**: Viper-based YAML configuration
- **Logging**: Structured logging with slog throughout
- **API Server**: HTTP REST API with endpoints for all operations
- **Web Interface**: Complete HTML/JavaScript UI
- **Docker Support**: Multi-stage Dockerfile and docker-compose
- **Build System**: Comprehensive Makefile

### ⚠️ Main Implementation Gap
**Phase 1 (~40% complete)**: Core Shelly device communication and configuration management need to be implemented. The interfaces exist but lack full device API coverage and type-safe configuration.

## Development Phases with Numbered Tasks

### Phase 1: Core Shelly Device Management (PRIORITY 1)
**Goal**: Real device communication and management with type-safe configuration

1. **Complete Gen1 API implementation** - full /settings, /status, /control endpoints with device-specific operations
2. **Complete Gen2+ RPC implementation** - all RPC methods with proper digest authentication
3. **Implement capability-based configuration** - replace json.RawMessage with type-safe capability structs
4. **Implement detailed per-device configuration management** - individual device settings, overrides, and custom configurations
5. **Import device configuration from physical devices to database**
6. **Export device configuration from database to physical devices**
7. **Implement configuration drift detection and reporting**
8. **Implement bulk configuration sync operations**
9. **Complete device authentication handling for both Gen1 and Gen2+**
10. **Implement real-time status polling with configurable intervals**
11. **Add firmware version tracking and update management**
12. **Implement error recovery and retry logic for device communication**
13. **Create comprehensive real device testing suite**

### Phase 2: Dual-Binary Architecture (PRIORITY 2)
**Goal**: Separate provisioning agent for WiFi operations

14. **Create separate shelly-provisioner binary for WiFi operations**
15. **Implement API communication protocol between main app and provisioner**
16. **Build queue management system for provisioning tasks**
17. **Implement agent registration and heartbeat system**
18. **Create task distribution mechanism for provisioner agents**
19. **Add status reporting from provisioner to main application**

### Phase 3: WiFi Provisioning Implementation (PRIORITY 3)
**Goal**: Complete device provisioning flow

20. **Implement Shelly AP mode detection and scanning**
21. **Complete real AP connection logic for device provisioning**
22. **Implement WiFi credential injection to devices**
23. **Add network verification after provisioning**
24. **Build provisioning state machine with rollback on failure**

### Phase 4: Kubernetes Deployment (PRIORITY 4)
**Goal**: Production-ready K8s deployment (YAML manifests only, no Helm)

25. **Create optimized multi-stage Docker builds**
26. **Create Kubernetes YAML manifests (Deployment, Service, ConfigMap, Secret, PVC)**
27. **Implement health and readiness probes for K8s**
28. **Define resource limits and requests for containers**
29. **Create Kubernetes network policies**

### Backend Data Abstraction
30. **Create database abstraction layer supporting SQLite now, PostgreSQL future**

### Phase 5: Export & Import Functionality (PRIORITY 5)
**Goal**: Bidirectional data exchange in multiple formats

31. **Export functionality - full device configuration to single JSON file**
32. **Import functionality - restore device configuration from JSON file**
33. **Export to Git-friendly format - separate TOML files per device for version control**
34. **Import from Git repository - read TOML files and update device configurations**
35. **Export device list to CSV for spreadsheets**
36. **Export to hosts file format**
37. **Export DHCP reservation format**
38. **Implement scheduled automatic exports**

### Phase 6: OPNSense Integration (PRIORITY 6)
**Goal**: Automated DHCP management

39. **Create OPNSense API client for integration**
40. **Implement DHCP reservation sync with OPNSense**
41. **Add static mapping creation in OPNSense**
42. **Implement lease management with OPNSense**
43. **Add firewall alias updates for device groups**
44. **Implement error handling and rollback for failed syncs**

### Phase 7: Production Features (PRIORITY 7)
**Goal**: Monitoring, backup, and automation

45. **Add Prometheus metrics for monitoring**
46. **Implement backup and restore functionality**
47. **Create database migration system**
48. **Add scheduled automatic discovery**
49. **Implement device grouping and tagging**
50. **Create rule-based automation engine**
51. **Add event notification system**

### Phase 8: Advanced Features (PRIORITY 8)
**Goal**: Enhanced capabilities

52. **Implement WebSocket for real-time UI updates**
53. **Add advanced scheduling capabilities**
54. **Create template-based configuration system**
55. **Implement device profiles for common configurations**
56. **Add batch provisioning for multiple devices**
57. **Create network topology visualization**

## Implementation Timeline Estimates

- **Phase 1**: 2-4 weeks (Critical - blocks all other work)
- **Phase 2**: 1 week
- **Phase 3**: 1-2 weeks  
- **Phase 4**: 1 week
- **Data Abstraction**: 2-3 days (can be done in parallel)
- **Phase 5**: 1 week
- **Phase 6**: 1 week
- **Phase 7**: 1-2 weeks
- **Phase 8**: 2-3 weeks

## Scaling Considerations

### Current Target: 20-100 devices
- SQLite performs well
- Single API instance sufficient
- Simple polling adequate

### Future: 1000+ devices
- PostgreSQL migration (enabled by data abstraction layer)
- Redis caching layer
- Worker pool for discovery
- Connection pooling
- Batch API operations
- Event-driven updates
- Horizontal scaling with load balancer

## Key Architecture Decisions

1. **Composition-based Configuration**: Devices combine capability "mixins" (RelayConfig, PowerMeteringConfig, etc.) for flexible, type-safe configuration management

2. **Dual-Binary Design**: Separate provisioner agent handles WiFi operations, enabling container deployment with appropriate permissions

3. **Git-Friendly Export**: TOML format for device configurations enables version control and GitOps workflows

4. **Database Abstraction**: Prepare for PostgreSQL migration while starting with SQLite

## Next Steps

**Immediate Priority**: Begin Phase 1 implementation focusing on tasks 1-3 (API implementations and type-safe configuration). This unlocks all subsequent functionality and provides immediate value with real device management.