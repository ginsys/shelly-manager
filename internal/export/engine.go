package export

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/database/provider"
	"github.com/ginsys/shelly-manager/internal/logging"
)

// DatabaseManagerInterface defines what we need from database.Manager
type DatabaseManagerInterface interface {
	GetProvider() provider.DatabaseProvider
	GetDB() interface{}
	Close() error
}

// ExportEngine manages export plugins and operations
type ExportEngine struct {
	plugins   map[string]ExportPlugin
	dbManager DatabaseManagerInterface
	logger    *logging.Logger
	mutex     sync.RWMutex
}

// NewExportEngine creates a new export engine
func NewExportEngine(dbManager DatabaseManagerInterface, logger *logging.Logger) *ExportEngine {
	return &ExportEngine{
		plugins:   make(map[string]ExportPlugin),
		dbManager: dbManager,
		logger:    logger,
	}
}

// RegisterPlugin registers a new export plugin
func (e *ExportEngine) RegisterPlugin(plugin ExportPlugin) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	info := plugin.Info()
	if info.Name == "" {
		return fmt.Errorf("plugin name cannot be empty")
	}

	// Check if plugin is already registered
	if _, exists := e.plugins[info.Name]; exists {
		return fmt.Errorf("plugin %s is already registered", info.Name)
	}

	// Initialize the plugin
	if err := plugin.Initialize(e.logger); err != nil {
		return fmt.Errorf("failed to initialize plugin %s: %w", info.Name, err)
	}

	e.plugins[info.Name] = plugin
	e.logger.Info("Registered export plugin",
		"name", info.Name,
		"version", info.Version,
		"category", info.Category,
	)

	return nil
}

// UnregisterPlugin removes a plugin from the engine
func (e *ExportEngine) UnregisterPlugin(pluginName string) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	plugin, exists := e.plugins[pluginName]
	if !exists {
		return fmt.Errorf("plugin %s is not registered", pluginName)
	}

	// Cleanup the plugin
	if err := plugin.Cleanup(); err != nil {
		e.logger.Warn("Error cleaning up plugin", "name", pluginName, "error", err)
	}

	delete(e.plugins, pluginName)
	e.logger.Info("Unregistered export plugin", "name", pluginName)

	return nil
}

// ListPlugins returns information about all registered plugins
func (e *ExportEngine) ListPlugins() []PluginInfo {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	plugins := make([]PluginInfo, 0, len(e.plugins))
	for _, plugin := range e.plugins {
		plugins = append(plugins, plugin.Info())
	}

	return plugins
}

// GetPlugin returns a specific plugin by name
func (e *ExportEngine) GetPlugin(name string) (ExportPlugin, error) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	plugin, exists := e.plugins[name]
	if !exists {
		return nil, fmt.Errorf("plugin %s is not registered", name)
	}

	return plugin, nil
}

// Export performs an export operation using the specified plugin
func (e *ExportEngine) Export(ctx context.Context, request ExportRequest) (*ExportResult, error) {
	startTime := time.Now()

	// Generate unique export ID
	exportID := uuid.New().String()

	e.logger.Info("Starting export operation",
		"export_id", exportID,
		"plugin", request.PluginName,
		"format", request.Format,
	)

	// Get the plugin
	plugin, err := e.GetPlugin(request.PluginName)
	if err != nil {
		return &ExportResult{
			Success:   false,
			ExportID:  exportID,
			Errors:    []string{err.Error()},
			CreatedAt: time.Now(),
		}, err
	}

	// Validate plugin configuration
	if validationErr := plugin.ValidateConfig(request.Config); validationErr != nil {
		return &ExportResult{
			Success:   false,
			ExportID:  exportID,
			Errors:    []string{fmt.Sprintf("invalid configuration: %v", validationErr)},
			CreatedAt: time.Now(),
		}, err
	}

	// Load data from database
	data, err := e.loadExportData(ctx, request.Filters)
	if err != nil {
		return &ExportResult{
			Success:   false,
			ExportID:  exportID,
			Errors:    []string{fmt.Sprintf("failed to load data: %v", err)},
			CreatedAt: time.Now(),
		}, err
	}

	// Enhance metadata with export information
	data.Metadata.ExportID = exportID
	data.Metadata.ExportType = "manual"
	data.Timestamp = time.Now()

	// Create export config
	config := ExportConfig(request)

	// Perform the export
	result, err := plugin.Export(ctx, data, config)
	if err != nil {
		e.logger.Error("Export operation failed",
			"export_id", exportID,
			"plugin", request.PluginName,
			"error", err,
		)
		return &ExportResult{
			Success:   false,
			ExportID:  exportID,
			Errors:    []string{err.Error()},
			Duration:  time.Since(startTime),
			CreatedAt: time.Now(),
		}, err
	}

	// Update result with common fields
	result.ExportID = exportID
	result.PluginName = request.PluginName
	result.Format = request.Format
	result.Duration = time.Since(startTime)
	result.CreatedAt = time.Now()

	e.logger.Info("Export operation completed",
		"export_id", exportID,
		"plugin", request.PluginName,
		"success", result.Success,
		"duration", result.Duration,
		"records", result.RecordCount,
	)

	return result, nil
}

// Preview generates a preview of what would be exported
func (e *ExportEngine) Preview(ctx context.Context, request ExportRequest) (*PreviewResult, error) {
	e.logger.Info("Starting export preview",
		"plugin", request.PluginName,
		"format", request.Format,
	)

	// Get the plugin
	plugin, err := e.GetPlugin(request.PluginName)
	if err != nil {
		return nil, err
	}

	// Validate plugin configuration
	if validationErr := plugin.ValidateConfig(request.Config); validationErr != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Load data from database
	data, err := e.loadExportData(ctx, request.Filters)
	if err != nil {
		return nil, fmt.Errorf("failed to load data: %w", err)
	}

	// Create export config
	config := ExportConfig(request)

	// Generate preview
	preview, err := plugin.Preview(ctx, data, config)
	if err != nil {
		e.logger.Error("Export preview failed",
			"plugin", request.PluginName,
			"error", err,
		)
		return nil, err
	}

	e.logger.Info("Export preview completed",
		"plugin", request.PluginName,
		"records", preview.RecordCount,
		"estimated_size", preview.EstimatedSize,
	)

	return preview, nil
}

// ValidateExport validates an export configuration without performing the export
func (e *ExportEngine) ValidateExport(request ExportRequest) error {
	// Get the plugin
	plugin, err := e.GetPlugin(request.PluginName)
	if err != nil {
		return err
	}

	// Validate plugin configuration
	if validationErr := plugin.ValidateConfig(request.Config); validationErr != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Validate that the plugin supports the requested format
	info := plugin.Info()
	formatSupported := false
	for _, format := range info.SupportedFormats {
		if format == request.Format {
			formatSupported = true
			break
		}
	}

	if !formatSupported {
		return fmt.Errorf("plugin %s does not support format %s", request.PluginName, request.Format)
	}

	return nil
}

// Shutdown cleanly shuts down the export engine
func (e *ExportEngine) Shutdown() error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	e.logger.Info("Shutting down export engine")

	var errors []string
	for name, plugin := range e.plugins {
		if err := plugin.Cleanup(); err != nil {
			errors = append(errors, fmt.Sprintf("plugin %s: %v", name, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors during shutdown: %v", errors)
	}

	e.logger.Info("Export engine shutdown complete")
	return nil
}

// loadExportData loads data from the database based on filters
func (e *ExportEngine) loadExportData(ctx context.Context, filters ExportFilters) (*ExportData, error) {
	dbInterface := e.dbManager.GetDB()

	// Handle nil database (testing mode)
	if dbInterface == nil {
		return &ExportData{
			Devices:           []DeviceData{},
			Configurations:    []ConfigurationData{},
			DiscoveredDevices: []DiscoveredDeviceData{},
			Templates:         []TemplateData{},
			Metadata:          ExportMetadata{},
			Timestamp:         time.Now(),
		}, nil
	}

	db, ok := dbInterface.(*gorm.DB)
	if !ok {
		return nil, fmt.Errorf("database is not a GORM database")
	}

	// Load devices
	var devices []database.Device
	deviceQuery := db.WithContext(ctx)

	// Apply device filters
	if len(filters.DeviceIDs) > 0 {
		deviceQuery = deviceQuery.Where("id IN ?", filters.DeviceIDs)
	}
	if len(filters.DeviceTypes) > 0 {
		deviceQuery = deviceQuery.Where("type IN ?", filters.DeviceTypes)
	}
	if len(filters.DeviceStatus) > 0 {
		deviceQuery = deviceQuery.Where("status IN ?", filters.DeviceStatus)
	}
	if filters.LastSeenAfter != nil {
		deviceQuery = deviceQuery.Where("last_seen > ?", *filters.LastSeenAfter)
	}

	if err := deviceQuery.Find(&devices).Error; err != nil {
		return nil, fmt.Errorf("failed to load devices: %w", err)
	}

	// Convert devices to export format
	exportDevices := make([]DeviceData, len(devices))
	for i, device := range devices {
		exportDevices[i] = DeviceData{
			ID:        device.ID,
			MAC:       device.MAC,
			IP:        device.IP,
			Type:      device.Type,
			Name:      device.Name,
			Firmware:  device.Firmware,
			Status:    device.Status,
			LastSeen:  device.LastSeen,
			Settings:  parseJSONToMap(device.Settings),
			CreatedAt: device.CreatedAt,
			UpdatedAt: device.UpdatedAt,
		}
	}

	// Load discovered devices if not filtered out
	var discoveredDevices []DiscoveredDeviceData
	if !filters.excludeDiscoveredDevices() {
		var dbDiscoveredDevices []database.DiscoveredDevice
		if err := db.WithContext(ctx).Find(&dbDiscoveredDevices).Error; err != nil {
			e.logger.Warn("Failed to load discovered devices", "error", err)
		} else {
			discoveredDevices = make([]DiscoveredDeviceData, len(dbDiscoveredDevices))
			for i, dd := range dbDiscoveredDevices {
				discoveredDevices[i] = DiscoveredDeviceData{
					MAC:        dd.MAC,
					SSID:       dd.SSID,
					Model:      dd.Model,
					Generation: dd.Generation,
					IP:         dd.IP,
					Signal:     dd.Signal,
					AgentID:    dd.AgentID,
					Discovered: dd.Discovered,
				}
			}
		}
	}

	// Create metadata
	metadata := ExportMetadata{
		TotalDevices:  len(exportDevices),
		FilterApplied: e.hasFilters(filters),
		SystemVersion: "v0.5.3-alpha", // TODO: Get from build info
		DatabaseType:  e.dbManager.GetProvider().Name(),
	}

	return &ExportData{
		Devices:           exportDevices,
		Configurations:    []ConfigurationData{}, // TODO: Load configurations
		Templates:         []TemplateData{},      // TODO: Load templates
		DiscoveredDevices: discoveredDevices,
		Metadata:          metadata,
		Timestamp:         time.Now(),
	}, nil
}

// Helper functions

func parseJSONToMap(jsonStr string) map[string]interface{} {
	// TODO: Implement JSON parsing
	return make(map[string]interface{})
}

func (filters ExportFilters) excludeDiscoveredDevices() bool {
	// For now, always include discovered devices unless explicitly filtered
	return false
}

func (e *ExportEngine) hasFilters(filters ExportFilters) bool {
	return len(filters.DeviceIDs) > 0 ||
		len(filters.DeviceTypes) > 0 ||
		len(filters.DeviceStatus) > 0 ||
		filters.LastSeenAfter != nil ||
		filters.HasConfig != nil ||
		len(filters.TemplateIDs) > 0 ||
		len(filters.Tags) > 0
}
