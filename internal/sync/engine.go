package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/database/provider"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/security"
)

// DatabaseManagerInterface defines what we need from database.Manager
type DatabaseManagerInterface interface {
	GetProvider() provider.DatabaseProvider
	GetDB() *gorm.DB
	Close() error
}

// SyncEngine manages sync plugins and operations (unified export/import)
type SyncEngine struct {
	plugins   map[string]SyncPlugin
	dbManager DatabaseManagerInterface
	logger    *logging.Logger
	mutex     sync.RWMutex

	// In-memory result stores (recent results for retrieval/download)
	exportResults map[string]*ExportResult
	importResults map[string]*ImportResult

	// Security: Base directories for path validation
	// If set, file imports/exports are restricted to these directories
	importBaseDir string
	exportBaseDir string
}

// ExportEngine provides backward compatibility
type ExportEngine = SyncEngine

// NewSyncEngine creates a new sync engine
func NewSyncEngine(dbManager DatabaseManagerInterface, logger *logging.Logger) *SyncEngine {
	return &SyncEngine{
		plugins:       make(map[string]SyncPlugin),
		dbManager:     dbManager,
		logger:        logger,
		exportResults: make(map[string]*ExportResult),
		importResults: make(map[string]*ImportResult),
	}
}

// SetImportBaseDir sets the base directory for import path validation.
// If set, file-based imports are restricted to paths within this directory.
func (e *SyncEngine) SetImportBaseDir(dir string) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.importBaseDir = dir
	for _, plugin := range e.plugins {
		if restricted, ok := plugin.(ImportPathRestrictedPlugin); ok {
			restricted.SetImportBaseDir(dir)
		}
	}
}

// SetExportBaseDir sets the base directory for export path validation.
// If set, file-based exports are restricted to paths within this directory.
func (e *SyncEngine) SetExportBaseDir(dir string) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.exportBaseDir = dir
	for _, plugin := range e.plugins {
		if restricted, ok := plugin.(ExportPathRestrictedPlugin); ok {
			restricted.SetExportBaseDir(dir)
		}
	}
}

// GetExportResult retrieves a stored export result by ID
func (e *SyncEngine) GetExportResult(id string) (*ExportResult, bool) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	res, ok := e.exportResults[id]
	return res, ok
}

// DeleteExport removes an export record from memory and history; optionally removes the file.
func (e *SyncEngine) DeleteExport(ctx context.Context, exportID string, removeFile bool) error {
	// Attempt to get in-memory result for path info
	var path string
	if res, ok := e.GetExportResult(exportID); ok && res != nil {
		path = res.OutputPath
	} else {
		// Try history for path
		if db := e.dbManager.GetDB(); db != nil {
			var rec database.ExportHistory
			if err := db.WithContext(ctx).Where("export_id = ?", exportID).First(&rec).Error; err == nil {
				path = rec.FilePath
			}
		}
	}

	// Remove file if requested
	if removeFile && path != "" {
		_ = os.Remove(path)
	}

	// Delete DB history
	if db := e.dbManager.GetDB(); db != nil {
		_ = db.WithContext(ctx).Where("export_id = ?", exportID).Delete(&database.ExportHistory{}).Error
	}

	// Remove from in-memory map
	e.mutex.Lock()
	delete(e.exportResults, exportID)
	e.mutex.Unlock()
	return nil
}

// GetImportResult retrieves a stored import result by ID
func (e *SyncEngine) GetImportResult(id string) (*ImportResult, bool) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	res, ok := e.importResults[id]
	return res, ok
}

// NewExportEngine creates a new export engine (backward compatibility)
func NewExportEngine(dbManager DatabaseManagerInterface, logger *logging.Logger) *ExportEngine {
	return (*ExportEngine)(NewSyncEngine(dbManager, logger))
}

// RegisterPlugin registers a new sync plugin
func (e *SyncEngine) RegisterPlugin(plugin SyncPlugin) error {
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

	// Give plugins the distinct roots they must enforce at the filesystem
	// boundary. Retain the legacy export-root hook for older plugins.
	if restricted, ok := plugin.(ExportPathRestrictedPlugin); ok {
		restricted.SetExportBaseDir(e.exportBaseDir)
	} else if restricted, ok := plugin.(PathRestrictedPlugin); ok {
		restricted.SetBaseDir(e.exportBaseDir)
	}
	if restricted, ok := plugin.(ImportPathRestrictedPlugin); ok {
		restricted.SetImportBaseDir(e.importBaseDir)
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
func (e *SyncEngine) UnregisterPlugin(pluginName string) error {
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
func (e *SyncEngine) ListPlugins() []PluginInfo {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	plugins := make([]PluginInfo, 0, len(e.plugins))
	for _, plugin := range e.plugins {
		plugins = append(plugins, plugin.Info())
	}

	return plugins
}

// GetPlugin returns a specific plugin by name
func (e *SyncEngine) GetPlugin(name string) (SyncPlugin, error) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	plugin, exists := e.plugins[name]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrPluginNotFound, name)
	}

	return plugin, nil
}

// Export performs an export operation using the specified plugin
func (e *SyncEngine) Export(ctx context.Context, request ExportRequest) (*ExportResult, error) {
	startTime := time.Now()

	// Generate unique export ID
	exportID := uuid.New().String()

	e.logger.Info("Starting export operation",
		"export_id", exportID,
		"plugin", request.PluginName,
		"format", request.Format,
	)

	plugin, err := e.validateRequest(&request, true)
	if err != nil {
		return failedExportResult(exportID, request, startTime, err), err
	}

	// Load data from database
	data, err := e.loadExportData(ctx, request.Filters)
	if err != nil {
		wrapped := fmt.Errorf("failed to load data: %w", err)
		return failedExportResult(exportID, request, startTime, wrapped), wrapped
	}

	// Enhance metadata with export information
	data.Metadata.ExportID = exportID
	data.Metadata.RequestedBy = strings.TrimSpace(request.CreatedBy)
	if data.Metadata.RequestedBy == "" {
		data.Metadata.RequestedBy = "shelly-manager"
	}
	data.Metadata.ExportType = strings.TrimSpace(request.ExportType)
	if data.Metadata.ExportType == "" {
		data.Metadata.ExportType = "manual"
	}
	data.Timestamp = time.Now()

	// Create export config
	config := exportConfigFromRequest(request)

	// Perform the export
	result, err := plugin.Export(ctx, data, config)
	if err != nil {
		e.logger.Error("Export operation failed",
			"export_id", exportID,
			"plugin", request.PluginName,
			"error", err,
		)
		return failedExportResult(exportID, request, startTime, err), err
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

	// Store result for later retrieval/download
	e.mutex.Lock()
	e.exportResults[result.ExportID] = result
	// Optional: cap memory usage by trimming old entries (simple heuristic)
	if len(e.exportResults) > 2000 {
		// Best-effort cleanup: reset the map when too large
		e.exportResults = map[string]*ExportResult{result.ExportID: result}
	}
	e.mutex.Unlock()

	return result, nil
}

func failedExportResult(exportID string, request ExportRequest, started time.Time, err error) *ExportResult {
	return &ExportResult{
		Success:    false,
		ExportID:   exportID,
		PluginName: request.PluginName,
		Format:     request.Format,
		Errors:     []string{err.Error()},
		Duration:   time.Since(started),
		CreatedAt:  time.Now(),
	}
}

func exportConfigFromRequest(request ExportRequest) ExportConfig {
	return ExportConfig{
		PluginName: request.PluginName,
		Format:     request.Format,
		Config:     request.Config,
		Filters:    request.Filters,
		Output:     request.Output,
		Options:    request.Options,
	}
}

// Preview generates a preview of what would be exported
func (e *SyncEngine) Preview(ctx context.Context, request ExportRequest) (*PreviewResult, error) {
	e.logger.Info("Starting export preview",
		"plugin", request.PluginName,
		"format", request.Format,
	)

	plugin, err := e.validateRequest(&request, true)
	if err != nil {
		return nil, err
	}

	// Load data from database
	data, err := e.loadExportData(ctx, request.Filters)
	if err != nil {
		return nil, fmt.Errorf("failed to load data: %w", err)
	}
	data.Metadata.RequestedBy = strings.TrimSpace(request.CreatedBy)
	if data.Metadata.RequestedBy == "" {
		data.Metadata.RequestedBy = "shelly-manager"
	}
	data.Metadata.ExportType = strings.TrimSpace(request.ExportType)
	if data.Metadata.ExportType == "" {
		data.Metadata.ExportType = "manual"
	}

	// Create export config
	config := exportConfigFromRequest(request)

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

// Import performs an import operation using the specified plugin
func (e *SyncEngine) Import(ctx context.Context, request ImportRequest) (*ImportResult, error) {
	startTime := time.Now()

	// Generate unique import ID
	importID := uuid.New().String()

	e.logger.Info("Starting import operation",
		"import_id", importID,
		"plugin", request.PluginName,
		"format", request.Format,
	)

	plugin, pluginErr := e.GetPlugin(request.PluginName)
	if pluginErr != nil {
		return &ImportResult{
			Success:   false,
			ImportID:  importID,
			Errors:    []string{pluginErr.Error()},
			CreatedAt: time.Now(),
		}, pluginErr
	}

	if err := validatePluginFormat(plugin, request.Format); err != nil {
		return &ImportResult{
			Success:   false,
			ImportID:  importID,
			Errors:    []string{err.Error()},
			CreatedAt: time.Now(),
		}, err
	}

	if validationErr := plugin.ValidateConfig(request.Config); validationErr != nil {
		err := fmt.Errorf("%w: %v", ErrInvalidPluginConfig, validationErr)
		return &ImportResult{
			Success:   false,
			ImportID:  importID,
			Errors:    []string{err.Error()},
			CreatedAt: time.Now(),
		}, err
	}

	if request.Source.Type == "file" && request.Source.Path != "" {
		validatedPath, pathErr := e.validateImportPath(request.Source.Path)
		if pathErr != nil {
			return &ImportResult{
				Success:   false,
				ImportID:  importID,
				Errors:    []string{fmt.Sprintf("path validation failed: %v", pathErr)},
				CreatedAt: time.Now(),
			}, pathErr
		}
		request.Source.Path = validatedPath
	}

	// Create import config
	config := ImportConfig{
		PluginName: request.PluginName,
		Format:     request.Format,
		Config:     request.Config,
		Options:    request.Options,
	}

	// Perform the import
	result, err := plugin.Import(ctx, request.Source, config)
	if err != nil {
		e.logger.Error("Import operation failed",
			"import_id", importID,
			"plugin", request.PluginName,
			"error", err,
		)
		return &ImportResult{
			Success:   false,
			ImportID:  importID,
			Errors:    []string{err.Error()},
			Duration:  time.Since(startTime),
			CreatedAt: time.Now(),
		}, err
	}

	// Update result with common fields
	result.ImportID = importID
	result.PluginName = request.PluginName
	result.Format = request.Format
	result.Duration = time.Since(startTime)
	result.CreatedAt = time.Now()

	e.logger.Info("Import operation completed",
		"import_id", importID,
		"plugin", request.PluginName,
		"success", result.Success,
		"duration", result.Duration,
		"records", result.RecordsImported,
		"skipped", result.RecordsSkipped,
	)

	// Store result for later retrieval
	e.mutex.Lock()
	e.importResults[result.ImportID] = result
	if len(e.importResults) > 2000 {
		e.importResults = map[string]*ImportResult{result.ImportID: result}
	}
	e.mutex.Unlock()

	return result, nil
}

// ValidateExport validates an export configuration without performing the export
func (e *SyncEngine) ValidateExport(request ExportRequest) error {
	_, err := e.validateRequest(&request, true)
	return err
}

func validatePluginFormat(plugin SyncPlugin, format string) error {
	for _, supported := range plugin.Info().SupportedFormats {
		if supported == format {
			return nil
		}
	}
	return fmt.Errorf("%w: plugin %s does not support %q", ErrUnsupportedFormat, plugin.Info().Name, format)
}

// validateRequest preserves the public validation precedence: registry lookup,
// format, plugin configuration, then engine-owned path checks.
func (e *SyncEngine) validateRequest(request *ExportRequest, normalizePaths bool) (SyncPlugin, error) {
	plugin, err := e.GetPlugin(request.PluginName)
	if err != nil {
		return nil, err
	}
	if err := validatePluginFormat(plugin, request.Format); err != nil {
		return nil, err
	}
	if err := plugin.ValidateConfig(request.Config); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPluginConfig, err)
	}
	if outputPath, ok := request.Config["output_path"].(string); ok && outputPath != "" {
		validated, err := e.validateOutputPath(outputPath)
		if err != nil {
			return nil, fmt.Errorf("%w: output_path: %v", ErrInvalidExportPath, err)
		}
		if normalizePaths {
			request.Config["output_path"] = validated
		}
	}
	if request.Output.Destination != "" {
		validated, err := e.validateExportPath(request.Output.Destination)
		if err != nil {
			return nil, fmt.Errorf("%w: output destination: %v", ErrInvalidExportPath, err)
		}
		if normalizePaths {
			request.Output.Destination = validated
		}
	}
	return plugin, nil
}

// Shutdown cleanly shuts down the export engine
func (e *SyncEngine) Shutdown() error {
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

// SaveExportHistory persists an export operation record for audit/history purposes.
func (e *SyncEngine) SaveExportHistory(ctx context.Context, request ExportRequest, result *ExportResult, requestedBy string) error {
	db := e.dbManager.GetDB()
	if db == nil || result == nil {
		return nil
	}
	var fileSize int64
	if result.OutputPath != "" {
		if fi, err := os.Stat(result.OutputPath); err == nil {
			fileSize = fi.Size()
		}
	}
	rec := &database.ExportHistory{
		ExportID:   result.ExportID,
		PluginName: request.PluginName,
		Format:     request.Format,
		Name: func() string {
			if request.Config != nil {
				if v, ok := request.Config["name"].(string); ok {
					return v
				}
			}
			return ""
		}(),
		Description: func() string {
			if request.Config != nil {
				if v, ok := request.Config["description"].(string); ok {
					return v
				}
			}
			return ""
		}(),
		RequestedBy: requestedBy,
		Success:     result.Success,
		RecordCount: result.RecordCount,
		FileSize:    fileSize,
		FilePath:    result.OutputPath,
		DurationMs:  result.Duration.Milliseconds(),
		ErrorMessage: func() string {
			if len(result.Errors) > 0 {
				return result.Errors[0]
			}
			return ""
		}(),
		CreatedAt: time.Now(),
	}
	if err := db.WithContext(ctx).Create(rec).Error; err != nil {
		e.logger.WithFields(map[string]any{"error": err.Error(), "component": "sync_engine"}).Warn("Failed to save export history")
		return err
	}
	return nil
}

// SaveImportHistory persists an import operation record.
func (e *SyncEngine) SaveImportHistory(ctx context.Context, request ImportRequest, result *ImportResult, requestedBy string) error {
	db := e.dbManager.GetDB()
	if db == nil || result == nil {
		return nil
	}
	rec := &database.ImportHistory{
		ImportID:        result.ImportID,
		PluginName:      request.PluginName,
		Format:          request.Format,
		RequestedBy:     requestedBy,
		Success:         result.Success,
		RecordsImported: result.RecordsImported,
		RecordsSkipped:  result.RecordsSkipped,
		DurationMs:      result.Duration.Milliseconds(),
		ErrorMessage: func() string {
			if len(result.Errors) > 0 {
				return result.Errors[0]
			}
			return ""
		}(),
		CreatedAt: time.Now(),
	}
	if err := db.WithContext(ctx).Create(rec).Error; err != nil {
		e.logger.WithFields(map[string]any{"error": err.Error(), "component": "sync_engine"}).Warn("Failed to save import history")
		return err
	}
	return nil
}

// ListExportHistory returns paginated export history with optional filters.
func (e *SyncEngine) ListExportHistory(ctx context.Context, page, pageSize int, plugin string, success *bool) ([]database.ExportHistory, int, error) {
	db := e.dbManager.GetDB()
	if db == nil {
		return []database.ExportHistory{}, 0, nil
	}
	var items []database.ExportHistory
	q := db.WithContext(ctx).Model(&database.ExportHistory{})
	if plugin != "" {
		q = q.Where("plugin_name = ?", plugin)
	}
	if success != nil {
		q = q.Where("success = ?", *success)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := q.Order("created_at desc").Limit(pageSize).Offset((page - 1) * pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, int(total), nil
}

// GetExportHistory fetches a single export history record by export ID.
func (e *SyncEngine) GetExportHistory(ctx context.Context, exportID string) (*database.ExportHistory, error) {
	db := e.dbManager.GetDB()
	if db == nil {
		return nil, nil
	}
	var rec database.ExportHistory
	if err := db.WithContext(ctx).Where("export_id = ?", exportID).First(&rec).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &rec, nil
}

// ExportStatistics provides aggregate statistics for export operations.
func (e *SyncEngine) GetExportStatistics(ctx context.Context) (map[string]interface{}, error) {
	db := e.dbManager.GetDB()
	if db == nil {
		return map[string]interface{}{"total": 0}, nil
	}
	var total int64
	_ = db.WithContext(ctx).Model(&database.ExportHistory{}).Count(&total).Error
	var successCnt int64
	_ = db.WithContext(ctx).Model(&database.ExportHistory{}).Where("success = ?", true).Count(&successCnt).Error
	// Count by plugin
	type Row struct {
		PluginName string
		Cnt        int64
	}
	var rows []Row
	_ = db.WithContext(ctx).Model(&database.ExportHistory{}).Select("plugin_name, COUNT(*) as cnt").Group("plugin_name").Find(&rows).Error
	byPlugin := map[string]int64{}
	for _, r := range rows {
		byPlugin[r.PluginName] = r.Cnt
	}
	return map[string]interface{}{
		"total":     total,
		"success":   successCnt,
		"failure":   total - successCnt,
		"by_plugin": byPlugin,
	}, nil
}

// ListImportHistory returns paginated import history with optional filters.
func (e *SyncEngine) ListImportHistory(ctx context.Context, page, pageSize int, plugin string, success *bool) ([]database.ImportHistory, int, error) {
	db := e.dbManager.GetDB()
	if db == nil {
		return []database.ImportHistory{}, 0, nil
	}
	var items []database.ImportHistory
	q := db.WithContext(ctx).Model(&database.ImportHistory{})
	if plugin != "" {
		q = q.Where("plugin_name = ?", plugin)
	}
	if success != nil {
		q = q.Where("success = ?", *success)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := q.Order("created_at desc").Limit(pageSize).Offset((page - 1) * pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, int(total), nil
}

// GetImportHistory fetches a single import history record by import ID.
func (e *SyncEngine) GetImportHistory(ctx context.Context, importID string) (*database.ImportHistory, error) {
	db := e.dbManager.GetDB()
	if db == nil {
		return nil, nil
	}
	var rec database.ImportHistory
	if err := db.WithContext(ctx).Where("import_id = ?", importID).First(&rec).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &rec, nil
}

// ImportStatistics provides aggregate statistics for import operations.
func (e *SyncEngine) GetImportStatistics(ctx context.Context) (map[string]interface{}, error) {
	db := e.dbManager.GetDB()
	if db == nil {
		return map[string]interface{}{"total": 0}, nil
	}
	var total int64
	_ = db.WithContext(ctx).Model(&database.ImportHistory{}).Count(&total).Error
	var successCnt int64
	_ = db.WithContext(ctx).Model(&database.ImportHistory{}).Where("success = ?", true).Count(&successCnt).Error
	type Row struct {
		PluginName string
		Cnt        int64
	}
	var rows []Row
	_ = db.WithContext(ctx).Model(&database.ImportHistory{}).Select("plugin_name, COUNT(*) as cnt").Group("plugin_name").Find(&rows).Error
	byPlugin := map[string]int64{}
	for _, r := range rows {
		byPlugin[r.PluginName] = r.Cnt
	}
	return map[string]interface{}{
		"total":     total,
		"success":   successCnt,
		"failure":   total - successCnt,
		"by_plugin": byPlugin,
	}, nil
}

// loadExportData loads data from the database based on filters
func (e *SyncEngine) loadExportData(ctx context.Context, filters ExportFilters) (*ExportData, error) {
	db := e.dbManager.GetDB()

	// Handle nil database (testing mode)
	if db == nil {
		return &ExportData{
			Devices:           []DeviceData{},
			Configurations:    []ConfigurationData{},
			DiscoveredDevices: []DiscoveredDeviceData{},
			Templates:         []TemplateData{},
			Metadata:          ExportMetadata{},
			Timestamp:         time.Now(),
		}, nil
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
		settings, err := parseJSONToMap(device.Settings)
		if err != nil {
			return nil, fmt.Errorf("device %d settings: %w", device.ID, err)
		}
		model := device.Type
		if storedModel, ok := settings["model"].(string); ok && strings.TrimSpace(storedModel) != "" {
			model = storedModel
		}
		exportDevices[i] = DeviceData{
			ID:        device.ID,
			MAC:       device.MAC,
			IP:        device.IP,
			Type:      device.Type,
			Name:      device.Name,
			Model:     model,
			Firmware:  device.Firmware,
			Status:    device.Status,
			LastSeen:  device.LastSeen,
			Settings:  settings,
			CreatedAt: device.CreatedAt,
			UpdatedAt: device.UpdatedAt,
		}
	}

	// Load the persisted per-device configuration rows. DesiredConfig is the
	// authoritative export payload when present; the older device_configs row
	// supplies its synchronization metadata and remains the fallback for
	// installations that have not populated DesiredConfig yet.
	configRows := make([]configuration.DeviceConfig, 0)
	if db.Migrator().HasTable(&configuration.DeviceConfig{}) && len(devices) > 0 {
		deviceIDs := make([]uint, len(devices))
		for i := range devices {
			deviceIDs[i] = devices[i].ID
		}
		if err := db.WithContext(ctx).
			Where("device_id IN ?", deviceIDs).
			Find(&configRows).Error; err != nil {
			return nil, fmt.Errorf("failed to load device configurations: %w", err)
		}
	}
	configByDevice := make(map[uint]configuration.DeviceConfig, len(configRows))
	for _, stored := range configRows {
		if _, duplicate := configByDevice[stored.DeviceID]; duplicate {
			return nil, fmt.Errorf("duplicate stored configurations for device %d", stored.DeviceID)
		}
		configByDevice[stored.DeviceID] = stored
	}

	configurations := make([]ConfigurationData, 0, len(devices))
	configuredDevices := make(map[uint]bool, len(devices))
	for _, device := range devices {
		stored, hasStored := configByDevice[device.ID]
		raw := strings.TrimSpace(device.DesiredConfig)
		hasDesired := raw != "" && raw != "{}" && raw != "null"
		if !hasDesired && hasStored {
			raw = string(stored.Config)
		}
		if !hasDesired && !hasStored {
			continue
		}
		config, err := parseJSONToMap(raw)
		if err != nil {
			return nil, fmt.Errorf("device %d desired configuration: %w", device.ID, err)
		}
		item := ConfigurationData{
			DeviceID:   device.ID,
			Config:     config,
			SyncStatus: "pending",
			UpdatedAt:  device.UpdatedAt,
		}
		if device.ConfigApplied {
			item.SyncStatus = "synced"
		}
		if hasStored {
			item.TemplateID = stored.TemplateID
			item.LastSynced = stored.LastSynced
			if !hasDesired && stored.SyncStatus != "" {
				item.SyncStatus = stored.SyncStatus
			}
			if !stored.UpdatedAt.IsZero() {
				item.UpdatedAt = stored.UpdatedAt
			}
		}
		configurations = append(configurations, item)
		configuredDevices[device.ID] = true
	}

	if filters.HasConfig != nil {
		filteredDevices := make([]DeviceData, 0, len(exportDevices))
		for _, device := range exportDevices {
			if configuredDevices[device.ID] == *filters.HasConfig {
				filteredDevices = append(filteredDevices, device)
			}
		}
		exportDevices = filteredDevices
		filteredConfigurations := make([]ConfigurationData, 0, len(configurations))
		for _, item := range configurations {
			if *filters.HasConfig {
				filteredConfigurations = append(filteredConfigurations, item)
			}
		}
		configurations = filteredConfigurations
	}

	// Load reusable templates with their full configuration payload.
	var storedTemplates []configuration.ConfigTemplate
	templateQuery := db.WithContext(ctx)
	if len(filters.TemplateIDs) > 0 {
		templateQuery = templateQuery.Where("id IN ?", filters.TemplateIDs)
	}
	if err := templateQuery.Find(&storedTemplates).Error; err != nil {
		return nil, fmt.Errorf("failed to load configuration templates: %w", err)
	}
	templates := make([]TemplateData, len(storedTemplates))
	for i, stored := range storedTemplates {
		config, err := parseJSONBytesToMap(stored.Config)
		if err != nil {
			return nil, fmt.Errorf("template %d config: %w", stored.ID, err)
		}
		variables, err := parseJSONBytesToMap(stored.Variables)
		if err != nil {
			return nil, fmt.Errorf("template %d variables: %w", stored.ID, err)
		}
		templates[i] = TemplateData{
			ID:          stored.ID,
			Name:        stored.Name,
			Description: stored.Description,
			DeviceType:  stored.DeviceType,
			Generation:  stored.Generation,
			Config:      config,
			Variables:   variables,
			IsDefault:   stored.IsDefault,
			CreatedAt:   stored.CreatedAt,
			UpdatedAt:   stored.UpdatedAt,
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
		TotalConfigs:  len(configurations),
		FilterApplied: e.hasFilters(filters),
		SystemVersion: "v0.5.3-alpha", // TODO: Get from build info
		DatabaseType:  e.dbManager.GetProvider().Name(),
	}

	return &ExportData{
		Devices:           exportDevices,
		Configurations:    configurations,
		Templates:         templates,
		DiscoveredDevices: discoveredDevices,
		Metadata:          metadata,
		Timestamp:         time.Now(),
	}, nil
}

// Helper functions

func parseJSONToMap(jsonText string) (map[string]interface{}, error) {
	return parseJSONBytesToMap([]byte(jsonText))
}

func parseJSONBytesToMap(data []byte) (map[string]interface{}, error) {
	if len(strings.TrimSpace(string(data))) == 0 {
		return map[string]interface{}{}, nil
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("invalid JSON object: %w", err)
	}
	if result == nil {
		return nil, fmt.Errorf("JSON object must not be null")
	}
	return result, nil
}

func (filters ExportFilters) excludeDiscoveredDevices() bool {
	// For now, always include discovered devices unless explicitly filtered
	return false
}

func (e *SyncEngine) hasFilters(filters ExportFilters) bool {
	return len(filters.DeviceIDs) > 0 ||
		len(filters.DeviceTypes) > 0 ||
		len(filters.DeviceStatus) > 0 ||
		filters.LastSeenAfter != nil ||
		filters.HasConfig != nil ||
		len(filters.TemplateIDs) > 0 ||
		len(filters.Tags) > 0
}

// validateImportPath validates a file path for import operations.
// If importBaseDir is set, it ensures the path is within that directory.
// Returns the validated absolute path or an error.
func (e *SyncEngine) validateImportPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("import path cannot be empty")
	}

	// If no base directory is configured, validate that the path is absolute
	// and doesn't contain traversal sequences as a basic safety check
	if e.importBaseDir == "" {
		// Basic validation: ensure path is absolute
		absPath, err := filepath.Abs(path)
		if err != nil {
			return "", fmt.Errorf("invalid import path: %w", err)
		}
		return absPath, nil
	}

	// Use security.ValidatePath for comprehensive validation
	validatedPath, err := security.ValidatePath(e.importBaseDir, path)
	if err != nil {
		return "", fmt.Errorf("import path validation failed: %w", err)
	}

	return validatedPath, nil
}

// validateExportPath validates a file path for export operations.
// If exportBaseDir is set, it ensures the path is within that directory.
// Returns the validated absolute path or an error.
func (e *SyncEngine) validateExportPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("export path cannot be empty")
	}

	// If no base directory is configured, validate that the path is absolute
	if e.exportBaseDir == "" {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return "", fmt.Errorf("invalid export path: %w", err)
		}
		return absPath, nil
	}

	// Use security.ValidatePath for comprehensive validation
	validatedPath, err := security.ValidatePath(e.exportBaseDir, path)
	if err != nil {
		return "", fmt.Errorf("export path validation failed: %w", err)
	}

	return validatedPath, nil
}

// validateOutputPath validates output_path from plugin config.
// Returns the validated path or an error.
func (e *SyncEngine) validateOutputPath(outputPath string) (string, error) {
	if outputPath == "" {
		return "", nil // Empty is allowed, plugin will use default
	}

	// If no export base directory is configured, validate basic safety
	if e.exportBaseDir == "" {
		// Ensure path doesn't contain traversal sequences
		cleanPath := filepath.Clean(outputPath)
		if strings.Contains(cleanPath, "..") {
			return "", fmt.Errorf("path traversal not allowed")
		}
		return filepath.Abs(cleanPath)
	}

	return security.ValidatePath(e.exportBaseDir, outputPath)
}
