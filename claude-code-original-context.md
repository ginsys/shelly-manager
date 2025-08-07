# Shelly Device Manager - Development Context

## 📋 Original Requirements

The user requested a Golang application for managing Shelly smart home devices with these specific requirements:

### Core Functionality
- **Headless operation** in containers
- **CLI interface** for all functionality
- **SQLite database** for persistence
- **Configuration file** support (YAML)
- **API server mode** for web frontend integration

### Key Features Developed

#### 1. Device Discovery & Management
- **HTTP scanning** of network ranges for Shelly devices
- **mDNS/Bonjour discovery** for advertised devices
- **SSDP/UPnP discovery** for modern devices
- **Database persistence** of device information
- **Real-time status monitoring**

#### 2. WiFi Provisioning System
**Critical Requirement**: Handle unconfigured devices that expose their own WiFi SSID

- **Network interface control** (requires host system access)
- **WiFi AP scanning** for Shelly device patterns
- **Automated connection** to device APs
- **Device configuration** via HTTP API
- **Production WiFi setup** and device reboot
- **Platform-specific implementations** (Linux, macOS, Windows)

#### 3. DHCP Integration
**New Requirement**: Generate DHCP reservations for OPNSense firewall

- **MAC address extraction** from provisioned devices
- **Hostname standardization** for network management
- **IP pool management** with automatic assignment
- **OPNSense API integration** for automated reservation creation
- **Export capabilities** (JSON, CSV, XML formats)
- **Inter-application communication** for device authentication

#### 4. Web Interface
- **Modern responsive UI** matching CLI functionality
- **Real-time device management**
- **Provisioning workflow** with progress tracking
- **DHCP reservation management**
- **Export and sync capabilities**

## 🔧 Technical Architecture

### Database Schema
```sql
-- devices table
id          INTEGER PRIMARY KEY
ip          TEXT UNIQUE
mac         TEXT
type        TEXT
name        TEXT
firmware    TEXT
status      TEXT
last_seen   DATETIME
settings    TEXT (JSON)
created_at  DATETIME
updated_at  DATETIME
```

### Configuration Structure
```yaml
server:          # API server settings
database:        # SQLite path and options
discovery:       # Network discovery settings
provisioning:    # WiFi provisioning configuration
dhcp:           # IP pool and reservation settings
opnsense:       # Firewall integration
main_app:       # Inter-app communication
```

### API Endpoints
```
# Core device management
GET    /api/v1/devices
POST   /api/v1/devices
GET    /api/v1/devices/{id}
PUT    /api/v1/devices/{id}
DELETE /api/v1/devices/{id}
GET    /api/v1/devices/by-mac/{mac}

# Discovery and provisioning
POST   /api/v1/discover
GET    /api/v1/provisioning/status
POST   /api/v1/provisioning/start

# DHCP management
GET    /api/v1/dhcp/reservations
GET    /api/v1/dhcp/reservations/export
POST   /api/v1/dhcp/reservations
POST   /api/v1/dhcp/opnsense/sync

# Device authentication
GET    /api/v1/devices/{id}/auth
PUT    /api/v1/devices/{id}/auth
```

## 🚀 Deployment Architecture

### Container Requirements
- **Privileged mode** required for WiFi operations
- **Host network access** for network interface control
- **Volume mounts** for data persistence
- **Device access** (/dev/rfkill for WiFi)

### Platform Compatibility
- **Linux**: Full support (nmcli, iwlist)
- **macOS**: Full support (networksetup, airport)
- **Windows**: Basic support (netsh, PowerShell)
- **Docker**: Privileged mode required

## 📊 Workflow Process

### Complete Device Lifecycle
1. **Discovery**: Scan for unconfigured Shelly AP networks
2. **Provisioning**: Connect to AP, configure WiFi credentials
3. **Network Integration**: Device joins production network
4. **DHCP Reservation**: Extract MAC/hostname, create reservation
5. **Firewall Sync**: Push reservations to OPNSense
6. **Device Management**: Configure settings via main application

### Inter-Application Communication
- **Provisioning App** ↔ **Main App**: Share device authentication
- **Main App** ↔ **OPNSense**: DHCP reservation management
- **CLI** ↔ **Web UI**: Feature parity across interfaces

## 🔐 Security Considerations

### Device Authentication
- **Default credentials** set during provisioning
- **Authentication required** for device management
- **Cloud connectivity** disabled by default
- **Local network only** operation

### Network Security
- **Temporary AP connections** only during provisioning
- **Credential protection** in memory only
- **HTTPS** for OPNSense API communication
- **API key authentication** between applications

## 💡 Key Insights from Development

### Shelly Device Patterns
- **AP Mode SSID**: `shelly1-XXXXXX`, `SHSW-1#XXXXXX`
- **Default AP IP**: `192.168.33.1`
- **API Endpoints**: `/shelly`, `/settings`, `/status`
- **Configuration**: JSON payload via HTTP POST

### Network Discovery Methods
1. **HTTP Scan**: Most reliable, scans IP ranges
2. **mDNS**: Good for advertised devices
3. **SSDP**: Newer devices with UPnP support

### DHCP Integration Workflow
1. **Extract** MAC addresses post-provisioning
2. **Generate** hostnames from MAC/device type
3. **Assign** IPs from configured pool
4. **Export** in multiple formats for firewall config
5. **Sync** automatically via API when possible

## 🎯 Next Development Priorities

### Code Organization
- **Split monolithic main.go** into packages
- **Implement proper error handling** throughout
- **Add comprehensive testing** for all components
- **Improve logging and debugging** capabilities

### Feature Enhancements
- **Device group management** for bulk operations
- **Scheduled provisioning** for large deployments
- **Backup/restore** of device configurations
- **Monitoring and alerting** for device failures

### Platform Support
- **Windows PowerShell** integration improvements
- **Systemd service** files for Linux deployment
- **macOS LaunchAgent** for background operation
- **ARM/Raspberry Pi** optimization

## 📚 Reference Information

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

---

*This document captures the complete context and requirements from the original conversation for continued development with Claude Code.*
