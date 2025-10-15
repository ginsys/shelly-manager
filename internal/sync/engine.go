package sync

import (
	"context"
	"fmt"
	"os"
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

	// Scheduling (in-memory for now)
	schedules map[string]*ExportSchedule
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
		schedules:     make(map[string]*ExportSchedule),
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
		return nil, fmt.Errorf("plugin %s is not registered", name)
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

// Preview generates a preview of what would be exported
func (e *SyncEngine) Preview(ctx context.Context, request ExportRequest) (*PreviewResult, error) {
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

	// Get the plugin
	plugin, err := e.GetPlugin(request.PluginName)
	if err != nil {
		return &ImportResult{
			Success:   false,
			ImportID:  importID,
			Errors:    []string{err.Error()},
			CreatedAt: time.Now(),
		}, err
	}

	// Validate plugin configuration
	if validationErr := plugin.ValidateConfig(request.Config); validationErr != nil {
		return &ImportResult{
			Success:   false,
			ImportID:  importID,
			Errors:    []string{fmt.Sprintf("invalid configuration: %v", validationErr)},
			CreatedAt: time.Now(),
		}, validationErr
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
		PluginName: result.PluginName,
		Format:     result.Format,
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

// ---- Scheduling (simple in-memory implementation) ----

// ExportSchedule defines a scheduled export job
type ExportSchedule struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	IntervalSec int                    `json:"interval_sec"` // simple interval scheduling
	Enabled     bool                   `json:"enabled"`
	Request     ExportRequest          `json:"request"`
	LastRun     *time.Time             `json:"last_run,omitempty"`
	NextRun     *time.Time             `json:"next_run,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ExportScheduleRequest is used to create/update schedules
type ExportScheduleRequest struct {
	Name        string        `json:"name"`
	IntervalSec int           `json:"interval_sec"`
	Enabled     bool          `json:"enabled"`
	Request     ExportRequest `json:"request"`
}

// ListSchedules lists all schedules
func (e *SyncEngine) ListSchedules() []*ExportSchedule {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	list := make([]*ExportSchedule, 0, len(e.schedules))
	for _, s := range e.schedules {
		list = append(list, s)
	}
	return list
}

// CreateSchedule creates a new schedule
func (e *SyncEngine) CreateSchedule(req ExportScheduleRequest) (*ExportSchedule, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if req.IntervalSec <= 0 {
		return nil, fmt.Errorf("interval_sec must be > 0")
	}
	if req.Request.PluginName == "" || req.Request.Format == "" {
		return nil, fmt.Errorf("request.plugin_name and request.format are required")
	}
	// basic validation against plugin
	if err := e.ValidateExport(req.Request); err != nil {
		return nil, err
	}
	now := time.Now()
	id := uuid.New().String()
	next := now.Add(time.Duration(req.IntervalSec) * time.Second)
	sch := &ExportSchedule{
		ID:          id,
		Name:        req.Name,
		IntervalSec: req.IntervalSec,
		Enabled:     req.Enabled,
		Request:     req.Request,
		LastRun:     nil,
		NextRun:     &next,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	e.mutex.Lock()
	e.schedules[id] = sch
	e.mutex.Unlock()
	return sch, nil
}

// GetSchedule retrieves a schedule by ID
func (e *SyncEngine) GetSchedule(id string) (*ExportSchedule, bool) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	s, ok := e.schedules[id]
	return s, ok
}

// UpdateSchedule updates a schedule by ID
func (e *SyncEngine) UpdateSchedule(id string, req ExportScheduleRequest) (*ExportSchedule, error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	sch, ok := e.schedules[id]
	if !ok {
		return nil, fmt.Errorf("schedule not found")
	}
	if req.Name != "" {
		sch.Name = req.Name
	}
	if req.IntervalSec > 0 {
		sch.IntervalSec = req.IntervalSec
	}
	sch.Enabled = req.Enabled
	if req.Request.PluginName != "" {
		sch.Request = req.Request
	}
	now := time.Now()
	sch.UpdatedAt = now
	next := now.Add(time.Duration(sch.IntervalSec) * time.Second)
	sch.NextRun = &next
	return sch, nil
}

// DeleteSchedule deletes a schedule by ID
func (e *SyncEngine) DeleteSchedule(id string) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	if _, ok := e.schedules[id]; !ok {
		return fmt.Errorf("schedule not found")
	}
	delete(e.schedules, id)
	return nil
}

// RunSchedule triggers the export for a schedule immediately
func (e *SyncEngine) RunSchedule(ctx context.Context, id string) (*ExportResult, error) {
	e.mutex.RLock()
	sch, ok := e.schedules[id]
	e.mutex.RUnlock()
	if !ok {
		return nil, fmt.Errorf("schedule not found")
	}
	// Execute export now
	res, err := e.Export(ctx, sch.Request)
	// Update last/next run timestamps
	now := time.Now()
	e.mutex.Lock()
	if ok {
		sch.LastRun = &now
		next := now.Add(time.Duration(sch.IntervalSec) * time.Second)
		sch.NextRun = &next
		sch.UpdatedAt = now
	}
	e.mutex.Unlock()
	return res, err
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

func (e *SyncEngine) hasFilters(filters ExportFilters) bool {
	return len(filters.DeviceIDs) > 0 ||
		len(filters.DeviceTypes) > 0 ||
		len(filters.DeviceStatus) > 0 ||
		filters.LastSeenAfter != nil ||
		filters.HasConfig != nil ||
		len(filters.TemplateIDs) > 0 ||
		len(filters.Tags) > 0
}
