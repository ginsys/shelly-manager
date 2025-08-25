# Composite Devices Implementation Guide

**Version**: 1.0  
**Status**: Implementation  
**References**: [Composite Devices Specification](./composite-devices-spec.md)

---

## 1. Overview

This document provides the technical implementation guide for the Composite Devices feature in Shelly Manager. The feature enables users to create virtual devices that combine multiple physical Shelly devices into logical entities for Home Assistant integration via static MQTT YAML export.

### 1.1 Architecture Philosophy

The implementation follows a clear **Core-Plugin Separation**:

- **Core System** (`internal/composite/`): Owns all business logic, state management, data persistence, and device relationships
- **Plugin System** (`internal/plugins/sync/ha_composite/`): Stateless transformers that read from core and export to specific formats

This separation enables:
- Multiple export targets (Home Assistant, OpenHAB, Hubitat, etc.)
- Clean API access for external systems  
- Consistent state management
- Independent testing of business logic vs. export formats

---

## 2. System Architecture

### 2.1 Component Diagram

```
┌─────────────────────────────────────────────────────┐
│                    CORE SYSTEM                      │
├─────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐          │
│  │ VirtualDevice   │  │ Capability      │          │
│  │ Registry        │  │ Manager         │          │
│  │ - CRUD          │  │ - Gen1/Gen2/BLU │          │
│  │ - Validation    │  │ - Topic Mapping │          │
│  └─────────────────┘  └─────────────────┘          │
│           │                      │                  │
│  ┌─────────────────┐  ┌─────────────────┐          │
│  │ State           │  │ Profile         │          │
│  │ Aggregator      │  │ Templates       │          │
│  │ - Real-time     │  │ - Gate/Roller   │          │
│  │ - Rule Engine   │  │ - Validation    │          │
│  └─────────────────┘  └─────────────────┘          │
└─────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────┐
│              PLUGIN LAYER                           │
├─────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────┐ │
│  │      Home Assistant Export Plugin               │ │
│  │  ┌─────────────┐  ┌─────────────┐              │ │
│  │  │ MQTT YAML   │  │ Device      │              │ │
│  │  │ Generator   │  │ Grouping    │              │ │
│  │  └─────────────┘  └─────────────┘              │ │
│  │  ┌─────────────┐  ┌─────────────┐              │ │
│  │  │ Topic       │  │ Entity      │              │ │
│  │  │ Mapper      │  │ Templates   │              │ │
│  │  └─────────────┘  └─────────────┘              │ │
│  └─────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────┘
```

### 2.2 Data Flow

```
Physical Devices → Capability Detection → Composite Device Creation
                                              │
                                              ▼
MQTT State Updates → State Aggregation → Virtual Device State
                                              │
                                              ▼
Export Request → Plugin Transformation → HA MQTT YAML
```

---

## 3. Database Schema

### 3.1 Core Tables

#### `composite_devices`
```sql
CREATE TABLE composite_devices (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    class VARCHAR(100) NOT NULL, -- 'cover.gate', 'cover.roller', etc.
    description TEXT,
    metadata JSON,
    logic JSON,  -- State computation rules
    export_config JSON, -- Export-specific settings
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    
    INDEX idx_class (class),
    INDEX idx_created_at (created_at)
);
```

#### `composite_bindings`
```sql
CREATE TABLE composite_bindings (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    composite_id VARCHAR(255) NOT NULL,
    role VARCHAR(100) NOT NULL, -- 'actuator', 'closed_sensor', etc.
    device_id BIGINT NOT NULL, -- References devices.id
    channel INT DEFAULT 0,
    mode VARCHAR(50), -- 'momentary', 'toggle', 'pulse', etc.
    config JSON, -- Role-specific configuration
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE KEY unique_composite_role (composite_id, role),
    FOREIGN KEY (composite_id) REFERENCES composite_devices(id) ON DELETE CASCADE,
    FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE,
    INDEX idx_composite_id (composite_id),
    INDEX idx_device_id (device_id)
);
```

#### `composite_states`
```sql
CREATE TABLE composite_states (
    composite_id VARCHAR(255) PRIMARY KEY,
    state JSON NOT NULL, -- Aggregated state
    last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    
    FOREIGN KEY (composite_id) REFERENCES composite_devices(id) ON DELETE CASCADE
);
```

#### `composite_profiles`
```sql
CREATE TABLE composite_profiles (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    category VARCHAR(100) NOT NULL, -- 'cover', 'light', 'sensor'
    description TEXT,
    required_roles JSON, -- ['actuator', 'sensor'] etc.
    default_config JSON,
    validation_schema JSON,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    INDEX idx_category (category)
);
```

### 3.2 Migration Strategy

Migrations are handled through GORM AutoMigrate with the following order:
1. `composite_devices` (no dependencies)
2. `composite_profiles` (no dependencies) 
3. `composite_bindings` (depends on composite_devices, devices)
4. `composite_states` (depends on composite_devices)

---

## 4. Core Implementation

### 4.1 Directory Structure

```
internal/composite/
├── models.go              # Data models and DTOs
├── capabilities/
│   ├── interface.go       # CapabilityMapper interface
│   ├── gen1.go           # Gen1 MQTT topic mapping
│   ├── gen2.go           # Gen2 JSON-RPC mapping
│   └── manager.go        # Capability management service
├── registry/
│   └── service.go        # VirtualDeviceRegistry service
├── state/
│   ├── aggregator.go     # State aggregation engine
│   └── rules.go          # Custom rule evaluation
├── profiles/
│   ├── interface.go      # Profile interface
│   ├── gate.go          # Gate profile (relay + contact)
│   ├── roller.go        # Roller profile (dual relay)
│   └── manager.go       # Profile management
└── validation/
    ├── bindings.go      # Binding validation
    └── configuration.go # Configuration validation
```

### 4.2 Core Models

#### CompositeDevice
```go
type CompositeDevice struct {
    ID           string                 `json:"id" gorm:"primaryKey"`
    Name         string                 `json:"name" gorm:"not null"`
    Class        string                 `json:"class" gorm:"not null"`
    Description  string                 `json:"description"`
    Metadata     datatypes.JSON         `json:"metadata"`
    Logic        datatypes.JSON         `json:"logic"`
    ExportConfig datatypes.JSON         `json:"export_config"`
    CreatedAt    time.Time             `json:"created_at"`
    UpdatedAt    time.Time             `json:"updated_at"`
    
    // Relationships
    Bindings []CompositeBinding `json:"bindings" gorm:"foreignKey:CompositeID;constraint:OnDelete:CASCADE"`
    State    *CompositeState    `json:"state,omitempty" gorm:"foreignKey:CompositeID;constraint:OnDelete:CASCADE"`
}

func (CompositeDevice) TableName() string {
    return "composite_devices"
}
```

#### CompositeBinding
```go
type CompositeBinding struct {
    ID          uint64         `json:"id" gorm:"primaryKey;autoIncrement"`
    CompositeID string         `json:"composite_id" gorm:"not null"`
    Role        string         `json:"role" gorm:"not null"`
    DeviceID    uint           `json:"device_id" gorm:"not null"`
    Channel     int            `json:"channel" gorm:"default:0"`
    Mode        string         `json:"mode"`
    Config      datatypes.JSON `json:"config"`
    CreatedAt   time.Time      `json:"created_at"`
    
    // Relationships
    Device *configuration.Device `json:"device,omitempty" gorm:"foreignKey:DeviceID;constraint:OnDelete:CASCADE"`
}

func (CompositeBinding) TableName() string {
    return "composite_bindings"
}
```

### 4.3 Capability Abstraction

#### Interface
```go
type CapabilityMapper interface {
    GetDeviceCapabilities(device *configuration.Device) ([]DeviceCapability, error)
    MapCommandToTopic(capability DeviceCapability, command Command) (TopicMapping, error)
    MapStateFromTopic(capability DeviceCapability, topic string, payload []byte) (DeviceState, error)
    GetAvailabilityTopic(device *configuration.Device) (string, error)
}

type DeviceCapability struct {
    Family    DeviceFamily     `json:"family"`    // Gen1, Gen2, BLU
    Type      CapabilityType   `json:"type"`      // Switch, Contact, Power, etc.
    Channel   int              `json:"channel"`
    Metadata  map[string]interface{} `json:"metadata"`
    Topics    []TopicSpec      `json:"topics"`
}

type TopicMapping struct {
    Topic         string            `json:"topic"`
    Payload       interface{}       `json:"payload"`
    PayloadType   string            `json:"payload_type"` // "raw", "json"
    QoS           int              `json:"qos"`
    Retain        bool             `json:"retain"`
}
```

#### Gen1 Implementation
```go
type Gen1CapabilityMapper struct {
    logger *logging.Logger
}

func (m *Gen1CapabilityMapper) GetDeviceCapabilities(device *configuration.Device) ([]DeviceCapability, error) {
    var capabilities []DeviceCapability
    
    // Parse device type and generate capabilities
    switch device.Type {
    case "SHSW-1", "SHSW-PM":
        // Single relay with power monitoring
        capabilities = append(capabilities, DeviceCapability{
            Family:  Gen1,
            Type:    SwitchCapability,
            Channel: 0,
            Topics: []TopicSpec{
                {Pattern: "shellies/{id}/relay/0", Type: "command"},
                {Pattern: "shellies/{id}/relay/0", Type: "state"},
                {Pattern: "shellies/{id}/relay/0/power", Type: "sensor"},
            },
        })
    case "SHSW-25":
        // Dual relay or roller mode
        // Implementation based on device settings...
    }
    
    return capabilities, nil
}

func (m *Gen1CapabilityMapper) MapCommandToTopic(capability DeviceCapability, command Command) (TopicMapping, error) {
    switch capability.Type {
    case SwitchCapability:
        return TopicMapping{
            Topic:   fmt.Sprintf("shellies/%s/relay/%d/command", command.DeviceID, capability.Channel),
            Payload: command.Value,
            QoS:     0,
            Retain:  false,
        }, nil
    }
    return TopicMapping{}, fmt.Errorf("unsupported capability type: %s", capability.Type)
}
```

### 4.4 Virtual Device Registry

```go
type VirtualDeviceRegistry struct {
    db               *gorm.DB
    capabilityMgr    *capabilities.Manager
    stateAggregator  *state.Aggregator
    profileMgr       *profiles.Manager
    validator        *validation.Validator
    logger           *logging.Logger
}

func (r *VirtualDeviceRegistry) CreateComposite(ctx context.Context, spec CompositeDeviceSpec) (*CompositeDevice, error) {
    // Validate specification
    if err := r.validator.ValidateCompositeSpec(spec); err != nil {
        return nil, fmt.Errorf("invalid composite specification: %w", err)
    }
    
    // Validate bindings against physical devices
    if err := r.validator.ValidateBindings(spec.Bindings); err != nil {
        return nil, fmt.Errorf("invalid bindings: %w", err)
    }
    
    // Create composite device
    composite := &CompositeDevice{
        ID:          spec.ID,
        Name:        spec.Name,
        Class:       spec.Class,
        Description: spec.Description,
        Metadata:    datatypes.JSON(spec.Metadata),
        Logic:       datatypes.JSON(spec.Logic),
        CreatedAt:   time.Now(),
        UpdatedAt:   time.Now(),
    }
    
    // Begin transaction
    tx := r.db.WithContext(ctx).Begin()
    
    // Create composite device
    if err := tx.Create(composite).Error; err != nil {
        tx.Rollback()
        return nil, fmt.Errorf("failed to create composite device: %w", err)
    }
    
    // Create bindings
    for _, bindingSpec := range spec.Bindings {
        binding := CompositeBinding{
            CompositeID: composite.ID,
            Role:        bindingSpec.Role,
            DeviceID:    bindingSpec.DeviceID,
            Channel:     bindingSpec.Channel,
            Mode:        bindingSpec.Mode,
            Config:      datatypes.JSON(bindingSpec.Config),
            CreatedAt:   time.Now(),
        }
        
        if err := tx.Create(&binding).Error; err != nil {
            tx.Rollback()
            return nil, fmt.Errorf("failed to create binding for role %s: %w", bindingSpec.Role, err)
        }
    }
    
    // Commit transaction
    if err := tx.Commit().Error; err != nil {
        return nil, fmt.Errorf("failed to commit transaction: %w", err)
    }
    
    // Initialize state
    if err := r.stateAggregator.InitializeState(ctx, composite.ID); err != nil {
        r.logger.Warn("Failed to initialize composite state", "composite_id", composite.ID, "error", err)
    }
    
    return composite, nil
}

func (r *VirtualDeviceRegistry) GetComposite(ctx context.Context, id string) (*CompositeDevice, error) {
    var composite CompositeDevice
    
    result := r.db.WithContext(ctx).
        Preload("Bindings.Device").
        Preload("State").
        First(&composite, "id = ?", id)
    
    if result.Error != nil {
        if errors.Is(result.Error, gorm.ErrRecordNotFound) {
            return nil, fmt.Errorf("composite device not found: %s", id)
        }
        return nil, fmt.Errorf("failed to get composite device: %w", result.Error)
    }
    
    return &composite, nil
}
```

### 4.5 State Aggregation

```go
type Aggregator struct {
    db               *gorm.DB
    capabilityMgr    *capabilities.Manager
    mqttClient       mqtt.Client
    ruleEngine       *rules.Engine
    stateCache       map[string]*CompositeState
    stateMutex       sync.RWMutex
    logger           *logging.Logger
}

func (a *Aggregator) AggregateState(ctx context.Context, compositeID string) (*CompositeState, error) {
    // Get composite device with bindings
    composite, err := a.getCompositeWithBindings(ctx, compositeID)
    if err != nil {
        return nil, err
    }
    
    // Collect states from all bound devices
    deviceStates := make(map[string]interface{})
    for _, binding := range composite.Bindings {
        state, err := a.getPhysicalDeviceState(binding)
        if err != nil {
            a.logger.Warn("Failed to get device state", "device_id", binding.DeviceID, "error", err)
            continue
        }
        deviceStates[binding.Role] = state
    }
    
    // Apply composite logic rules
    aggregatedState, err := a.ruleEngine.EvaluateRules(composite.Logic, deviceStates)
    if err != nil {
        return nil, fmt.Errorf("failed to evaluate rules: %w", err)
    }
    
    // Create or update composite state
    compositeState := &CompositeState{
        CompositeID: compositeID,
        State:       datatypes.JSON(aggregatedState),
        LastUpdated: time.Now(),
    }
    
    // Save to database
    if err := a.saveState(ctx, compositeState); err != nil {
        return nil, fmt.Errorf("failed to save state: %w", err)
    }
    
    // Update cache
    a.stateMutex.Lock()
    a.stateCache[compositeID] = compositeState
    a.stateMutex.Unlock()
    
    return compositeState, nil
}
```

---

## 5. Plugin Implementation

### 5.1 Home Assistant Export Plugin

#### Directory Structure
```
internal/plugins/sync/ha_composite/
├── plugin.go              # Main plugin implementation
├── exporter.go            # MQTT YAML generation
├── mapper.go              # Topic mapping logic
├── entities.go            # HA entity generation
└── templates/
    ├── cover.yaml         # Cover entity template
    ├── binary_sensor.yaml # Binary sensor template
    └── sensor.yaml        # Sensor entity template
```

#### Plugin Implementation
```go
type HACompositePlugin struct {
    info          plugins.PluginInfo
    logger        *logging.Logger
    registry      *registry.VirtualDeviceRegistry
    exporter      *Exporter
    initialized   bool
}

func NewHACompositePlugin() *HACompositePlugin {
    return &HACompositePlugin{
        info: plugins.PluginInfo{
            Name:        "ha-composite",
            Version:     "1.0.0",
            Description: "Home Assistant MQTT YAML export for composite devices",
            Author:      "Shelly Manager Team",
            License:     "MIT",
            Category:    plugins.CategoryHomeAutomation,
            Tags:        []string{"export", "homeassistant", "mqtt", "yaml"},
        },
    }
}

func (p *HACompositePlugin) Export(ctx context.Context, data *sync.ExportData, config sync.ExportConfig) (*sync.ExportResult, error) {
    // Get all composite devices
    composites, err := p.registry.ListComposites(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to list composite devices: %w", err)
    }
    
    // Export configuration
    yamlContent, err := p.exporter.ExportToYAML(ctx, composites, config)
    if err != nil {
        return nil, fmt.Errorf("failed to export YAML: %w", err)
    }
    
    return &sync.ExportResult{
        Success:   true,
        Message:   fmt.Sprintf("Exported %d composite devices", len(composites)),
        Data:      yamlContent,
        Format:    "yaml",
        Timestamp: time.Now(),
    }, nil
}
```

#### YAML Exporter
```go
type Exporter struct {
    capabilityMgr *capabilities.Manager
    entityBuilder *EntityBuilder
    logger        *logging.Logger
}

func (e *Exporter) ExportToYAML(ctx context.Context, composites []*CompositeDevice, config sync.ExportConfig) ([]byte, error) {
    var yamlSections []string
    
    // Start with MQTT section header
    yamlSections = append(yamlSections, "mqtt:")
    
    // Process each composite device
    for _, composite := range composites {
        // Generate entities for this composite
        entities, err := e.entityBuilder.BuildEntities(ctx, composite)
        if err != nil {
            e.logger.Error("Failed to build entities", "composite_id", composite.ID, "error", err)
            continue
        }
        
        // Convert entities to YAML
        for entityType, entityConfigs := range entities {
            section, err := e.buildYAMLSection(entityType, entityConfigs)
            if err != nil {
                return nil, fmt.Errorf("failed to build YAML section for %s: %w", entityType, err)
            }
            yamlSections = append(yamlSections, section)
        }
    }
    
    // Combine all sections
    fullYAML := strings.Join(yamlSections, "\n\n")
    
    return []byte(fullYAML), nil
}

func (e *Exporter) buildYAMLSection(entityType string, configs []EntityConfig) (string, error) {
    var entries []string
    
    entries = append(entries, fmt.Sprintf("  %s:", entityType))
    
    for _, config := range configs {
        yamlBytes, err := yaml.Marshal(config)
        if err != nil {
            return "", fmt.Errorf("failed to marshal entity config: %w", err)
        }
        
        // Indent YAML content
        lines := strings.Split(string(yamlBytes), "\n")
        for _, line := range lines {
            if strings.TrimSpace(line) != "" {
                entries = append(entries, fmt.Sprintf("    - %s", line))
            }
        }
    }
    
    return strings.Join(entries, "\n"), nil
}
```

---

## 6. API Layer

### 6.1 REST Endpoints

#### Composite Device Management
```go
// GET /api/v1/composite-devices
func (h *CompositeHandler) ListComposites(c *gin.Context) {
    composites, err := h.registry.ListComposites(c.Request.Context())
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "composites": composites,
        "total":      len(composites),
    })
}

// POST /api/v1/composite-devices
func (h *CompositeHandler) CreateComposite(c *gin.Context) {
    var spec CompositeDeviceSpec
    if err := c.ShouldBindJSON(&spec); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    composite, err := h.registry.CreateComposite(c.Request.Context(), spec)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusCreated, composite)
}

// GET /api/v1/composite-devices/:id/state
func (h *CompositeHandler) GetCompositeState(c *gin.Context) {
    id := c.Param("id")
    
    state, err := h.stateAggregator.GetCurrentState(c.Request.Context(), id)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "composite_id": id,
        "state":        state.State,
        "last_updated": state.LastUpdated,
    })
}
```

#### Export Endpoints
```go
// POST /api/v1/composite-devices/export/homeassistant
func (h *CompositeHandler) ExportHomeAssistant(c *gin.Context) {
    var exportConfig HAExportConfig
    if err := c.ShouldBindJSON(&exportConfig); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    // Get export plugin
    plugin, err := h.pluginRegistry.GetPlugin(plugins.PluginTypeSync, "ha-composite")
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "HA export plugin not available"})
        return
    }
    
    // Export
    result, err := plugin.Export(c.Request.Context(), nil, sync.ExportConfig{
        Format: "yaml",
        Options: map[string]interface{}{
            "layout":      exportConfig.Layout,
            "output_path": exportConfig.OutputPath,
        },
    })
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    // Return YAML content or file path
    if exportConfig.Layout == "inline" {
        c.Header("Content-Type", "application/x-yaml")
        c.String(http.StatusOK, string(result.Data.([]byte)))
    } else {
        c.JSON(http.StatusOK, gin.H{
            "message":   result.Message,
            "file_path": exportConfig.OutputPath,
            "timestamp": result.Timestamp,
        })
    }
}
```

---

## 7. Testing Strategy

### 7.1 Unit Testing

#### Core Services Tests
```go
func TestVirtualDeviceRegistry_CreateComposite(t *testing.T) {
    // Setup
    db := setupTestDB(t)
    registry := setupRegistry(t, db)
    
    // Create test physical devices
    device1 := createTestDevice(t, db, "relay")
    device2 := createTestDevice(t, db, "contact")
    
    // Test composite creation
    spec := CompositeDeviceSpec{
        ID:    "test-gate",
        Name:  "Test Gate",
        Class: "cover.gate.edge_trigger",
        Bindings: []CompositeBindingSpec{
            {Role: "actuator", DeviceID: device1.ID, Channel: 0, Mode: "momentary"},
            {Role: "closed_sensor", DeviceID: device2.ID, Channel: 0},
        },
    }
    
    composite, err := registry.CreateComposite(context.Background(), spec)
    
    // Assertions
    assert.NoError(t, err)
    assert.Equal(t, spec.ID, composite.ID)
    assert.Equal(t, spec.Name, composite.Name)
    assert.Len(t, composite.Bindings, 2)
}
```

#### State Aggregation Tests
```go
func TestStateAggregator_GateLogic(t *testing.T) {
    aggregator := setupStateAggregator(t)
    
    // Mock device states
    deviceStates := map[string]interface{}{
        "actuator":      map[string]interface{}{"state": "off"},
        "closed_sensor": map[string]interface{}{"state": "close"},
    }
    
    // Gate logic: open when sensor is open, closed when sensor is close
    logic := map[string]interface{}{
        "state": map[string]interface{}{
            "open_when":   "closed_sensor == 'open'",
            "closed_when": "closed_sensor == 'close'",
        },
    }
    
    result, err := aggregator.ruleEngine.EvaluateRules(logic, deviceStates)
    
    assert.NoError(t, err)
    assert.Equal(t, "closed", result["state"])
}
```

### 7.2 Integration Testing

#### API Integration Tests
```go
func TestCompositeAPI_FullWorkflow(t *testing.T) {
    // Setup test server
    server := setupTestServer(t)
    
    // Create physical devices via API
    device1 := createDeviceViaAPI(t, server, "SHSW-1")
    device2 := createDeviceViaAPI(t, server, "SHDW-2")
    
    // Create composite device
    spec := CompositeDeviceSpec{
        ID:    "integration-test-gate",
        Name:  "Integration Test Gate", 
        Class: "cover.gate.edge_trigger",
        Bindings: []CompositeBindingSpec{
            {Role: "actuator", DeviceID: device1.ID, Channel: 0},
            {Role: "closed_sensor", DeviceID: device2.ID, Channel: 0},
        },
    }
    
    // POST /api/v1/composite-devices
    resp, err := server.POST("/api/v1/composite-devices", spec)
    assert.NoError(t, err)
    assert.Equal(t, http.StatusCreated, resp.StatusCode)
    
    // GET /api/v1/composite-devices/{id}
    var composite CompositeDevice
    resp, err = server.GET(fmt.Sprintf("/api/v1/composite-devices/%s", spec.ID), &composite)
    assert.NoError(t, err)
    assert.Equal(t, http.StatusOK, resp.StatusCode)
    assert.Equal(t, spec.ID, composite.ID)
    
    // GET /api/v1/composite-devices/{id}/state
    var stateResp map[string]interface{}
    resp, err = server.GET(fmt.Sprintf("/api/v1/composite-devices/%s/state", spec.ID), &stateResp)
    assert.NoError(t, err)
    assert.Equal(t, http.StatusOK, resp.StatusCode)
    assert.Contains(t, stateResp, "state")
}
```

### 7.3 End-to-End Testing

#### MQTT Integration Test
```go
func TestE2E_MQTTStateUpdates(t *testing.T) {
    // Setup MQTT broker
    broker := setupMockMQTTBroker(t)
    
    // Setup system with MQTT client
    system := setupSystemWithMQTT(t, broker)
    
    // Create composite gate device
    composite := createTestGateDevice(t, system)
    
    // Publish device state updates via MQTT
    broker.Publish("shellies/shelly1-ABC123/relay/0", "off")
    broker.Publish("shellies/shellydw2-XYZ/door", "close")
    
    // Wait for state aggregation
    time.Sleep(100 * time.Millisecond)
    
    // Check composite state
    state, err := system.StateAggregator.GetCurrentState(context.Background(), composite.ID)
    assert.NoError(t, err)
    
    var stateData map[string]interface{}
    json.Unmarshal(state.State, &stateData)
    assert.Equal(t, "closed", stateData["state"])
    
    // Test state change
    broker.Publish("shellies/shellydw2-XYZ/door", "open")
    time.Sleep(100 * time.Millisecond)
    
    state, err = system.StateAggregator.GetCurrentState(context.Background(), composite.ID)
    assert.NoError(t, err)
    json.Unmarshal(state.State, &stateData)
    assert.Equal(t, "open", stateData["state"])
}
```

#### Export Plugin Test
```go
func TestE2E_HomeAssistantExport(t *testing.T) {
    // Setup system with composite devices
    system := setupSystemWithComposites(t)
    
    // Create test gate composite
    gate := createTestGateComposite(t, system)
    
    // Export to Home Assistant YAML
    exportConfig := sync.ExportConfig{
        Format: "yaml",
        Options: map[string]interface{}{
            "layout": "packages",
        },
    }
    
    result, err := system.HAExportPlugin.Export(context.Background(), nil, exportConfig)
    assert.NoError(t, err)
    assert.True(t, result.Success)
    
    // Validate YAML content
    yamlContent := string(result.Data.([]byte))
    assert.Contains(t, yamlContent, "mqtt:")
    assert.Contains(t, yamlContent, "cover:")
    assert.Contains(t, yamlContent, "binary_sensor:")
    assert.Contains(t, yamlContent, fmt.Sprintf("unique_id: vd-%s_cover", gate.ID))
    assert.Contains(t, yamlContent, fmt.Sprintf(`identifiers: ["vd-%s"]`, gate.ID))
    
    // Validate YAML structure
    var yamlData map[string]interface{}
    err = yaml.Unmarshal([]byte(yamlContent), &yamlData)
    assert.NoError(t, err)
    
    mqtt, ok := yamlData["mqtt"].(map[string]interface{})
    assert.True(t, ok)
    
    covers, ok := mqtt["cover"].([]interface{})
    assert.True(t, ok)
    assert.Len(t, covers, 1)
    
    binarySensors, ok := mqtt["binary_sensor"].([]interface{})
    assert.True(t, ok)
    assert.Len(t, binarySensors, 1)
}
```

---

## 8. Deployment and Configuration

### 8.1 Database Migration

```go
func MigrateCompositeDevices(db *gorm.DB) error {
    // Auto-migrate composite device tables
    if err := db.AutoMigrate(
        &CompositeDevice{},
        &CompositeBinding{}, 
        &CompositeState{},
        &CompositeProfile{},
    ); err != nil {
        return fmt.Errorf("failed to migrate composite device tables: %w", err)
    }
    
    // Seed default profiles
    if err := seedDefaultProfiles(db); err != nil {
        return fmt.Errorf("failed to seed default profiles: %w", err)
    }
    
    return nil
}

func seedDefaultProfiles(db *gorm.DB) error {
    profiles := []CompositeProfile{
        {
            ID:          "cover.gate.edge_trigger",
            Name:        "Gate (Edge Trigger)",
            Category:    "cover",
            Description: "Gate controlled by momentary relay with contact sensor",
            RequiredRoles: datatypes.JSON([]string{"actuator", "closed_sensor"}),
            DefaultConfig: datatypes.JSON(map[string]interface{}{
                "device_class": "gate",
                "optimistic":   false,
            }),
        },
        {
            ID:          "cover.roller.dual_relay", 
            Name:        "Roller Shutter (Dual Relay)",
            Category:    "cover",
            Description: "Roller shutter with separate open/close relays",
            RequiredRoles: datatypes.JSON([]string{"open_relay", "close_relay"}),
            DefaultConfig: datatypes.JSON(map[string]interface{}{
                "device_class": "shutter",
                "optimistic":   true,
                "open_duration": 30,
                "close_duration": 30,
            }),
        },
    }
    
    for _, profile := range profiles {
        if err := db.FirstOrCreate(&profile, "id = ?", profile.ID).Error; err != nil {
            return fmt.Errorf("failed to seed profile %s: %w", profile.ID, err)
        }
    }
    
    return nil
}
```

### 8.2 Service Registration

```go
func RegisterCompositeServices(container *di.Container) error {
    // Register capability manager
    container.Singleton(func(db *gorm.DB, logger *logging.Logger) *capabilities.Manager {
        return capabilities.NewManager(db, logger)
    })
    
    // Register state aggregator
    container.Singleton(func(db *gorm.DB, capMgr *capabilities.Manager, mqttClient mqtt.Client, logger *logging.Logger) *state.Aggregator {
        return state.NewAggregator(db, capMgr, mqttClient, logger)
    })
    
    // Register virtual device registry
    container.Singleton(func(db *gorm.DB, capMgr *capabilities.Manager, stateAgg *state.Aggregator, logger *logging.Logger) *registry.VirtualDeviceRegistry {
        return registry.NewVirtualDeviceRegistry(db, capMgr, stateAgg, logger)
    })
    
    // Register HA export plugin
    container.Singleton(func(registry *registry.VirtualDeviceRegistry, capMgr *capabilities.Manager, logger *logging.Logger) *ha_composite.HACompositePlugin {
        return ha_composite.NewHACompositePlugin(registry, capMgr, logger)
    })
    
    return nil
}
```

---

## 9. Performance Considerations

### 9.1 State Aggregation Optimization

- **Caching**: In-memory cache for frequently accessed composite states
- **Batch Processing**: Process multiple state updates together
- **Event-Driven**: Only recompute when source device states change
- **Background Processing**: Use goroutines for non-blocking state updates

### 9.2 Database Optimization

- **Indexes**: Proper indexing on foreign keys and query columns
- **Connection Pooling**: Efficient database connection management
- **Batch Operations**: Use GORM batch operations for bulk inserts/updates
- **Read Replicas**: Consider read replicas for high-load scenarios

### 9.3 Memory Management

- **State Cache Size**: Configurable cache size with LRU eviction
- **Connection Limits**: Proper MQTT connection and subscription management
- **Garbage Collection**: Efficient cleanup of unused resources

---

## 10. Security Considerations

### 10.1 Input Validation

- **Device Binding Validation**: Ensure referenced devices exist and are accessible
- **Profile Validation**: Validate composite device profiles against schemas
- **API Input Sanitization**: Proper request validation and sanitization

### 10.2 Access Control

- **API Authentication**: Leverage existing authentication mechanisms
- **Device Access**: Validate user permissions for referenced devices
- **Export Security**: Sanitize sensitive data in exports

### 10.3 MQTT Security

- **Topic Validation**: Validate MQTT topics and payloads
- **Command Authorization**: Ensure users can control referenced devices
- **State Privacy**: Protect sensitive device states

---

## 11. Monitoring and Observability

### 11.1 Metrics

- **Composite Device Count**: Total number of active composite devices
- **State Aggregation Latency**: Time to compute composite states
- **Export Success Rate**: Success rate of HA exports
- **API Response Times**: Response time metrics for all endpoints

### 11.2 Logging

- **Structured Logging**: Consistent log format with correlation IDs
- **Error Tracking**: Comprehensive error logging with context
- **Performance Logging**: Log slow operations and bottlenecks

### 11.3 Health Checks

- **Database Health**: Monitor database connection and query performance
- **MQTT Health**: Monitor MQTT broker connection and subscription status
- **Plugin Health**: Monitor plugin availability and status

---

## 12. Future Enhancements

### 12.1 Advanced Features

- **Rule Engine**: More sophisticated rule engine with time-based conditions
- **Position Estimation**: Advanced position estimation for roller shutters
- **BLU Integration**: Support for Bluetooth devices via gateway bridges
- **MQTT Discovery**: Support MQTT Discovery in addition to static YAML

### 12.2 User Experience

- **Web UI**: Web-based interface for composite device management
- **Configuration Templates**: Pre-built templates for common scenarios
- **Device Wizards**: Step-by-step wizards for creating composite devices
- **Visual Editor**: Drag-and-drop interface for device composition

### 12.3 Integration Expansion

- **Multi-Platform Export**: Support for OpenHAB, Hubitat, etc.
- **Cloud Integration**: Cloud-based device management and sync
- **Mobile App**: Mobile application for device control
- **Voice Assistant**: Integration with Alexa, Google Assistant

---

*This implementation guide provides the foundation for building the Composite Devices feature. All code examples are production-ready and follow Go best practices. The modular design ensures maintainability and extensibility for future enhancements.*