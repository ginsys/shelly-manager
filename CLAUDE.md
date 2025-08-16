# Shelly Manager - SuperClaude Project Memory

## Project Overview
Comprehensive Golang Shelly smart home device manager with dual-binary Kubernetes-native architecture.

## Current Development Status

### ✅ COMPLETED - Phase 2: Dual-Binary Architecture (v0.2.0-alpha)

**Achievement**: Successfully implemented the dual-binary provisioning system with separate `shelly-manager` API server and `shelly-provisioner` WiFi agent.

**Key Deliverables**:
- ✅ **Main API Server** (`cmd/shelly-manager/main.go`): Complete REST API with 25+ endpoints, device management, configuration system
- ✅ **Provisioning Agent** (`cmd/shelly-provisioner/main.go`): Standalone CLI tool with WiFi provisioning capabilities
- ✅ **Platform-Specific WiFi**: Linux/macOS/Windows network interfaces for provisioning
- ✅ **Core Architecture**: Clean separation between containerized API and host-based provisioning
- ✅ **Configuration Management**: Import/export/drift detection system with JSON blob storage
- ✅ **Device Authentication**: Gen1 & Gen2+ Shelly device support with Basic/Digest auth
- ✅ **Web Interface**: Feature-complete UI with error handling
- ✅ **Testing Coverage**: Comprehensive test suite for core packages

**Architecture Benefits**:
- API server runs securely in Kubernetes without WiFi access requirements
- Provisioning agent runs on host with direct WiFi interface access
- Clear separation of concerns with inter-service communication protocol (foundation laid)
- Security boundary between containerized and host-based components

### 🎯 CURRENT PRIORITY - Configuration System Enhancement

**Context**: The current configuration system uses raw JSON blob storage which creates technical debt and limits user experience. Need to transition to structured, typed configuration fields with proper validation and templating.

**Critical TODOs Identified** (from `internal/configuration/service.go`):

1. **Line 840-841**: Variable substitution implementation
   ```go
   // TODO: Implement proper variable substitution with text/template or similar
   ```

2. **Template Engine**: Currently returns config as-is, needs robust template processing

3. **JSON Blob Migration**: Move from raw JSON storage to structured typed fields

**Priority Requirements**:

#### 1. Variable Substitution System
- Replace basic string replacement with `text/template` or similar
- Support complex variable expressions: `${WIFI_PASSWORD}`, `{{.device.name}}`
- Environment variable integration with fallback values
- Validation of template syntax before storage

#### 2. Structured Configuration Fields
- **Current**: `DeviceConfig.Config` as `json.RawMessage` blob
- **Target**: Typed fields for common configurations (WiFi, auth, MQTT, etc.)
- Maintain backward compatibility during migration
- Implement configuration schema versioning

#### 3. Template-Based Configuration Management
- Configuration templates with inheritance
- Device-type specific templates (Gen1/Gen2+, by model)
- User-friendly template creation interface
- Template validation and testing framework

#### 4. Enhanced Validation System
- Pre-deployment configuration validation
- Device compatibility checking
- Safety checks for critical settings (WiFi, auth)
- Configuration diff and preview capabilities

#### 5. User Experience Improvements
- Form-based configuration UI (move away from raw JSON editing)
- Configuration wizards for common scenarios
- Real-time validation feedback
- Configuration import/export with proper formatting

### 📋 Technical Debt to Address

1. **Configuration Storage Architecture**:
   - Replace `json.RawMessage` with structured models
   - Implement configuration versioning
   - Add migration system for existing configs

2. **Template System Implementation**:
   - Choose between `text/template`, `html/template`, or third-party solution
   - Design template inheritance system
   - Implement variable scoping and validation

3. **Validation Enhancement**:
   - Extend `validateConfigForExport()` with comprehensive checks
   - Add pre-save validation pipeline
   - Implement configuration testing framework

4. **User Interface Improvements**:
   - Replace raw JSON editors with form-based interfaces
   - Add configuration preview and diff views
   - Implement guided configuration workflows

### 🚧 Next Immediate Actions

1. **Research & Design** (Day 1-2):
   - Evaluate template engine options (`text/template` vs alternatives)
   - Design new configuration schema structure
   - Plan migration strategy for existing JSON blob configs

2. **Core Implementation** (Week 1):
   - Implement variable substitution in `substituteVariables()` function
   - Create structured configuration models alongside existing JSON blob
   - Add template validation and testing utilities

3. **Migration System** (Week 2):
   - Build JSON blob → structured field migration
   - Implement backward compatibility layer
   - Create data migration utilities

4. **User Interface** (Week 3):
   - Replace raw JSON editors with structured forms
   - Add configuration preview and validation UI
   - Implement template management interface

### 🏗️ Project Architecture Status

**Current State**: Solid foundation with dual-binary architecture complete

**Infrastructure**:
- ✅ Database layer with GORM (SQLite)
- ✅ REST API with 25+ endpoints
- ✅ Structured logging (slog)
- ✅ Configuration management (Viper)
- ✅ Platform-specific WiFi interfaces
- ✅ Real Shelly device integration (Gen1 & Gen2+)
- ✅ Web interface with authentication

**Key Files**:
- `internal/configuration/service.go` - Primary focus for improvements
- `internal/configuration/models.go` - Configuration data models
- `internal/config/config.go` - Application configuration
- `cmd/shelly-provisioner/main.go` - Completed provisioning agent
- `cmd/shelly-manager/main.go` - Main API server

### 📊 Future Phases Overview

**Phase 3: Container & Kubernetes** (Post-configuration system):
- Multi-stage Docker builds
- Kubernetes manifests and Helm charts
- ConfigMaps and Secrets integration
- Service mesh integration

**Phase 4: Integration & Export** (Planned):
- Export API implementation (JSON, CSV, hosts, DHCP)
- OPNSense integration
- DHCP reservation management
- Bulk operations

**Phase 5: Production Features** (Future):
- Monitoring and metrics (Prometheus)
- Backup/restore capabilities
- High availability setup
- Advanced automation features

### 🎯 Success Metrics for Configuration System

**User Experience**:
- Reduce configuration errors by 80% through validation
- Replace 100% of raw JSON editing with structured forms
- Enable template-based configuration for 90%+ use cases

**Technical Quality**:
- Achieve 95%+ configuration validation coverage
- Implement zero-downtime configuration migrations
- Maintain <100ms configuration rendering performance

**Developer Experience**:
- Clear separation between configuration storage and presentation
- Comprehensive test coverage for configuration logic
- Documentation for template syntax and variable usage

---

**Last Updated**: 2025-01-16  
**Phase Completed**: Phase 2 - Dual-Binary Architecture  
**Current Focus**: Configuration System Enhancement  
**Next Major Milestone**: Template-based configuration with variable substitution