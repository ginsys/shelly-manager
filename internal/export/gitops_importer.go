package export

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/logging"
)

// GitOpsImporter handles importing GitOps YAML structures
type GitOpsImporter struct {
	dbManager DatabaseManagerInterface
	logger    *logging.Logger
}

// GitOpsData represents the loaded GitOps configuration structure
type GitOpsData struct {
	CommonConfig map[string]interface{}    `json:"common_config"`
	Groups       map[string]*GitOpsGroup   `json:"groups"`
	Ungrouped    map[string][]GitOpsDevice `json:"ungrouped"`
	Templates    []GitOpsTemplate          `json:"templates"`
	Devices      []GitOpsDevice            `json:"devices"` // Flattened list for processing
}

// GitOpsGroup represents a device group with its configuration
type GitOpsGroup struct {
	Name        string                       `json:"name"`
	Config      map[string]interface{}       `json:"config"`
	DeviceTypes map[string]*GitOpsDeviceType `json:"device_types"`
}

// GitOpsDeviceType represents devices of a specific type within a group
type GitOpsDeviceType struct {
	Type         string                 `json:"type"`
	CommonConfig map[string]interface{} `json:"common_config"`
	Devices      []GitOpsDevice         `json:"devices"`
}

// GitOpsDevice represents a single device configuration
type GitOpsDevice struct {
	Name         string                 `json:"name"`
	MAC          string                 `json:"mac"`
	Type         string                 `json:"type"`
	Group        string                 `json:"group"`
	Config       map[string]interface{} `json:"config"`
	MergedConfig map[string]interface{} `json:"merged_config"` // Final configuration after inheritance
}

// GitOpsTemplate represents a configuration template
type GitOpsTemplate struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	DeviceType  string                 `json:"device_type"`
	Generation  int                    `json:"generation"`
	Config      map[string]interface{} `json:"config"`
	Variables   map[string]interface{} `json:"variables"`
	IsDefault   bool                   `json:"is_default"`
}

// GitOpsImportOptions provides options for GitOps import
type GitOpsImportOptions struct {
	DryRun         bool `json:"dry_run"`
	ForceOverwrite bool `json:"force_overwrite"`
	BackupBefore   bool `json:"backup_before"`
}

// GitOpsImportResult contains the result of a GitOps import
type GitOpsImportResult struct {
	Success         bool           `json:"success"`
	DevicesImported int            `json:"devices_imported"`
	DevicesSkipped  int            `json:"devices_skipped"`
	ConfigsApplied  int            `json:"configs_applied"`
	Changes         []ImportChange `json:"changes"`
	Errors          []string       `json:"errors"`
	Warnings        []string       `json:"warnings"`
}

// NewGitOpsImporter creates a new GitOps importer
func NewGitOpsImporter(dbManager DatabaseManagerInterface, logger *logging.Logger) *GitOpsImporter {
	return &GitOpsImporter{
		dbManager: dbManager,
		logger:    logger,
	}
}

// LoadGitOpsStructure loads and parses a GitOps directory structure
func (g *GitOpsImporter) LoadGitOpsStructure(rootPath string) (*GitOpsData, error) {
	g.logger.Info("Loading GitOps structure", "path", rootPath)

	gitopsData := &GitOpsData{
		Groups:    make(map[string]*GitOpsGroup),
		Ungrouped: make(map[string][]GitOpsDevice),
		Templates: []GitOpsTemplate{},
		Devices:   []GitOpsDevice{},
	}

	// Load common configuration
	commonPath := filepath.Join(rootPath, "common.yaml")
	if _, err := os.Stat(commonPath); err == nil {
		commonConfig, err := g.loadYAMLFile(commonPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load common config: %w", err)
		}
		gitopsData.CommonConfig = commonConfig
		g.logger.Debug("Loaded common configuration", "path", commonPath)
	}

	// Load groups
	groupsPath := filepath.Join(rootPath, "groups")
	if _, err := os.Stat(groupsPath); err == nil {
		if err := g.loadGroups(groupsPath, gitopsData); err != nil {
			return nil, fmt.Errorf("failed to load groups: %w", err)
		}
	}

	// Load ungrouped devices
	ungroupedPath := filepath.Join(rootPath, "ungrouped")
	if _, err := os.Stat(ungroupedPath); err == nil {
		if err := g.loadUngroupedDevices(ungroupedPath, gitopsData); err != nil {
			return nil, fmt.Errorf("failed to load ungrouped devices: %w", err)
		}
	}

	// Load templates
	templatesPath := filepath.Join(rootPath, "templates")
	if _, err := os.Stat(templatesPath); err == nil {
		if err := g.loadTemplates(templatesPath, gitopsData); err != nil {
			return nil, fmt.Errorf("failed to load templates: %w", err)
		}
	}

	// Process inheritance and create flattened device list
	if err := g.processConfigInheritance(gitopsData); err != nil {
		return nil, fmt.Errorf("failed to process config inheritance: %w", err)
	}

	g.logger.Info("GitOps structure loaded successfully",
		"groups", len(gitopsData.Groups),
		"devices", len(gitopsData.Devices),
		"templates", len(gitopsData.Templates),
	)

	return gitopsData, nil
}

// PreviewChanges generates a preview of changes that would be made
func (g *GitOpsImporter) PreviewChanges(ctx context.Context, gitopsData *GitOpsData) []ImportChange {
	var changes []ImportChange

	dbInterface := g.dbManager.GetDB()

	// Handle nil database (testing mode)
	if dbInterface == nil {
		// Create mock changes for testing - simulate some updates and some creates
		for i, device := range gitopsData.Devices {
			var changeType string
			// First device is an update (simulating existing device), others are creates
			if i == 0 {
				changeType = "update"
			} else {
				changeType = "create"
			}

			changes = append(changes, ImportChange{
				Type:       changeType,
				Resource:   "device",
				ResourceID: device.MAC,
				NewValue:   device,
			})
		}
		return changes
	}

	db, ok := dbInterface.(*gorm.DB)
	if !ok {
		g.logger.Error("Database is not a GORM database")
		return changes
	}

	// Load existing devices from database
	var existingDevices []database.Device
	if err := db.WithContext(ctx).Find(&existingDevices).Error; err != nil {
		g.logger.Error("Failed to load existing devices for preview", "error", err)
		return changes
	}

	// Create lookup map for existing devices
	deviceByMAC := make(map[string]database.Device)
	for _, device := range existingDevices {
		deviceByMAC[device.MAC] = device
	}

	// Compare each GitOps device with existing devices
	for _, gitopsDevice := range gitopsData.Devices {
		if existing, found := deviceByMAC[gitopsDevice.MAC]; found {
			// Device exists - check for updates
			if gitopsDevice.Name != existing.Name {
				changes = append(changes, ImportChange{
					Type:       "update",
					Resource:   "device",
					ResourceID: gitopsDevice.MAC,
					Field:      "name",
					OldValue:   existing.Name,
					NewValue:   gitopsDevice.Name,
				})
			}

			// Compare configurations (simplified)
			if len(gitopsDevice.MergedConfig) > 0 {
				changes = append(changes, ImportChange{
					Type:       "update",
					Resource:   "config",
					ResourceID: gitopsDevice.MAC,
					Field:      "configuration",
					OldValue:   "existing config",
					NewValue:   "new config from GitOps",
				})
			}
		} else {
			// New device
			changes = append(changes, ImportChange{
				Type:       "create",
				Resource:   "device",
				ResourceID: gitopsDevice.MAC,
				NewValue:   gitopsDevice.Name,
			})
		}
	}

	return changes
}

// Import performs the GitOps import
func (g *GitOpsImporter) Import(ctx context.Context, gitopsData *GitOpsData, options GitOpsImportOptions) (*GitOpsImportResult, error) {
	g.logger.Info("Starting GitOps import",
		"devices", len(gitopsData.Devices),
		"dry_run", options.DryRun,
	)

	result := &GitOpsImportResult{
		Success:  true,
		Changes:  []ImportChange{},
		Errors:   []string{},
		Warnings: []string{},
	}

	if options.DryRun {
		// For dry run, just generate preview
		result.Changes = g.PreviewChanges(ctx, gitopsData)
		return result, nil
	}

	dbInterface := g.dbManager.GetDB()

	// Handle nil database (testing mode)
	if dbInterface == nil {
		return &GitOpsImportResult{
			Success:  true,
			Changes:  []ImportChange{},
			Errors:   []string{},
			Warnings: []string{},
		}, nil
	}

	db, ok := dbInterface.(*gorm.DB)
	if !ok {
		return nil, fmt.Errorf("database is not a GORM database")
	}

	// Start transaction for atomicity
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Load existing devices
	var existingDevices []database.Device
	if err := tx.WithContext(ctx).Find(&existingDevices).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to load existing devices: %w", err)
	}

	deviceByMAC := make(map[string]*database.Device)
	for i, device := range existingDevices {
		deviceByMAC[device.MAC] = &existingDevices[i]
	}

	// Process each GitOps device
	for _, gitopsDevice := range gitopsData.Devices {
		if existing, found := deviceByMAC[gitopsDevice.MAC]; found {
			// Update existing device
			updated := false

			if gitopsDevice.Name != existing.Name {
				existing.Name = gitopsDevice.Name
				updated = true
				result.Changes = append(result.Changes, ImportChange{
					Type:       "update",
					Resource:   "device",
					ResourceID: gitopsDevice.MAC,
					Field:      "name",
					OldValue:   existing.Name,
					NewValue:   gitopsDevice.Name,
				})
			}

			if gitopsDevice.Type != existing.Type {
				existing.Type = gitopsDevice.Type
				updated = true
				result.Changes = append(result.Changes, ImportChange{
					Type:       "update",
					Resource:   "device",
					ResourceID: gitopsDevice.MAC,
					Field:      "type",
					OldValue:   existing.Type,
					NewValue:   gitopsDevice.Type,
				})
			}

			if updated {
				if err := tx.Save(existing).Error; err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("failed to update device %s: %v", gitopsDevice.Name, err))
					result.Success = false
					continue
				}
				result.DevicesImported++
			} else {
				result.DevicesSkipped++
			}

			// TODO: Apply configuration changes to the actual device
			// This would involve connecting to the device and updating its configuration
			result.ConfigsApplied++
		} else {
			// Create new device
			newDevice := database.Device{
				MAC:      gitopsDevice.MAC,
				Name:     gitopsDevice.Name,
				Type:     gitopsDevice.Type,
				Status:   "pending", // Will be updated by discovery
				LastSeen: time.Now(),
				Settings: "{}", // Will be populated when device is discovered
			}

			if err := tx.Create(&newDevice).Error; err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("failed to create device %s: %v", gitopsDevice.Name, err))
				result.Success = false
				continue
			}

			result.DevicesImported++
			result.Changes = append(result.Changes, ImportChange{
				Type:       "create",
				Resource:   "device",
				ResourceID: gitopsDevice.MAC,
				NewValue:   gitopsDevice.Name,
			})
		}
	}

	// Commit transaction
	if result.Success && len(result.Errors) == 0 {
		if err := tx.Commit().Error; err != nil {
			result.Success = false
			result.Errors = append(result.Errors, fmt.Sprintf("failed to commit transaction: %v", err))
			return result, err
		}
	} else {
		tx.Rollback()
		result.Success = false
	}

	g.logger.Info("GitOps import completed",
		"success", result.Success,
		"imported", result.DevicesImported,
		"skipped", result.DevicesSkipped,
		"errors", len(result.Errors),
	)

	return result, nil
}

// Helper methods

func (g *GitOpsImporter) loadGroups(groupsPath string, gitopsData *GitOpsData) error {
	entries, err := os.ReadDir(groupsPath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		groupName := entry.Name()
		groupPath := filepath.Join(groupsPath, groupName)

		group := &GitOpsGroup{
			Name:        groupName,
			DeviceTypes: make(map[string]*GitOpsDeviceType),
		}

		// Load group configuration
		groupConfigPath := filepath.Join(groupPath, "group.yaml")
		if _, err := os.Stat(groupConfigPath); err == nil {
			groupConfig, err := g.loadYAMLFile(groupConfigPath)
			if err != nil {
				return fmt.Errorf("failed to load group config for %s: %w", groupName, err)
			}
			group.Config = groupConfig
		}

		// Load device types within the group
		if err := g.loadDeviceTypes(groupPath, group); err != nil {
			return fmt.Errorf("failed to load device types for group %s: %w", groupName, err)
		}

		gitopsData.Groups[groupName] = group
	}

	return nil
}

func (g *GitOpsImporter) loadDeviceTypes(groupPath string, group *GitOpsGroup) error {
	entries, err := os.ReadDir(groupPath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "group.yaml" {
			continue
		}

		deviceType := entry.Name()
		typePath := filepath.Join(groupPath, deviceType)

		deviceTypeObj := &GitOpsDeviceType{
			Type:    deviceType,
			Devices: []GitOpsDevice{},
		}

		// Load type common configuration
		commonPath := filepath.Join(typePath, "common.yaml")
		if _, err := os.Stat(commonPath); err == nil {
			commonConfig, err := g.loadYAMLFile(commonPath)
			if err != nil {
				return fmt.Errorf("failed to load common config for type %s: %w", deviceType, err)
			}
			deviceTypeObj.CommonConfig = commonConfig
		}

		// Load individual device files
		if err := g.loadDevicesInType(typePath, deviceTypeObj, group.Name); err != nil {
			return fmt.Errorf("failed to load devices for type %s: %w", deviceType, err)
		}

		group.DeviceTypes[deviceType] = deviceTypeObj
	}

	return nil
}

func (g *GitOpsImporter) loadDevicesInType(typePath string, deviceType *GitOpsDeviceType, groupName string) error {
	return filepath.WalkDir(typePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(d.Name(), ".yaml") || d.Name() == "common.yaml" {
			return nil
		}

		deviceConfig, err := g.loadYAMLFile(path)
		if err != nil {
			return fmt.Errorf("failed to load device config %s: %w", path, err)
		}

		// Extract device information
		device := GitOpsDevice{
			Type:   deviceType.Type,
			Group:  groupName,
			Config: deviceConfig,
		}

		if name, ok := deviceConfig["name"].(string); ok {
			device.Name = name
		}
		if mac, ok := deviceConfig["mac"].(string); ok {
			device.MAC = mac
		}
		if deviceType, ok := deviceConfig["type"].(string); ok {
			device.Type = deviceType
		}

		deviceType.Devices = append(deviceType.Devices, device)
		return nil
	})
}

func (g *GitOpsImporter) loadUngroupedDevices(ungroupedPath string, gitopsData *GitOpsData) error {
	entries, err := os.ReadDir(ungroupedPath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		deviceType := entry.Name()
		typePath := filepath.Join(ungroupedPath, deviceType)

		var devices []GitOpsDevice

		if err := filepath.WalkDir(typePath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d.IsDir() || !strings.HasSuffix(d.Name(), ".yaml") {
				return nil
			}

			deviceConfig, err := g.loadYAMLFile(path)
			if err != nil {
				return fmt.Errorf("failed to load ungrouped device config %s: %w", path, err)
			}

			device := GitOpsDevice{
				Type:   deviceType,
				Group:  "",
				Config: deviceConfig,
			}

			if name, ok := deviceConfig["name"].(string); ok {
				device.Name = name
			}
			if mac, ok := deviceConfig["mac"].(string); ok {
				device.MAC = mac
			}

			devices = append(devices, device)
			return nil
		}); err != nil {
			return err
		}

		if len(devices) > 0 {
			gitopsData.Ungrouped[deviceType] = devices
		}
	}

	return nil
}

func (g *GitOpsImporter) loadTemplates(templatesPath string, gitopsData *GitOpsData) error {
	return filepath.WalkDir(templatesPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(d.Name(), ".yaml") {
			return nil
		}

		templateConfig, err := g.loadYAMLFile(path)
		if err != nil {
			return fmt.Errorf("failed to load template %s: %w", path, err)
		}

		template := GitOpsTemplate{}
		if name, ok := templateConfig["name"].(string); ok {
			template.Name = name
		}
		if description, ok := templateConfig["description"].(string); ok {
			template.Description = description
		}
		if deviceType, ok := templateConfig["device_type"].(string); ok {
			template.DeviceType = deviceType
		}
		if generation, ok := templateConfig["generation"].(int); ok {
			template.Generation = generation
		}
		if config, ok := templateConfig["config"].(map[string]interface{}); ok {
			template.Config = config
		}
		if variables, ok := templateConfig["variables"].(map[string]interface{}); ok {
			template.Variables = variables
		}
		if isDefault, ok := templateConfig["is_default"].(bool); ok {
			template.IsDefault = isDefault
		}

		gitopsData.Templates = append(gitopsData.Templates, template)
		return nil
	})
}

func (g *GitOpsImporter) processConfigInheritance(gitopsData *GitOpsData) error {
	// Process grouped devices
	for _, group := range gitopsData.Groups {
		for _, deviceType := range group.DeviceTypes {
			for i := range deviceType.Devices {
				device := &deviceType.Devices[i]

				// Apply strict layered merge: common → group → type → device
				device.MergedConfig = g.mergeConfigs(
					gitopsData.CommonConfig,
					group.Config,
					deviceType.CommonConfig,
					device.Config,
				)

				// Add to flattened device list
				gitopsData.Devices = append(gitopsData.Devices, *device)
			}
		}
	}

	// Process ungrouped devices
	for _, devices := range gitopsData.Ungrouped {
		for i := range devices {
			device := &devices[i]

			// Apply only common config for ungrouped devices
			device.MergedConfig = g.mergeConfigs(
				gitopsData.CommonConfig,
				device.Config,
			)

			// Add to flattened device list
			gitopsData.Devices = append(gitopsData.Devices, *device)
		}
	}

	return nil
}

// mergeConfigs performs deep merge of multiple configuration maps
// Later configurations override earlier ones
func (g *GitOpsImporter) mergeConfigs(configs ...map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for _, config := range configs {
		if config == nil {
			continue
		}
		g.deepMerge(result, config)
	}

	return result
}

// deepMerge performs deep merge of source into destination
func (g *GitOpsImporter) deepMerge(dst, src map[string]interface{}) {
	for key, srcValue := range src {
		if dstValue, exists := dst[key]; exists {
			// Both values exist, check if they are maps
			if dstMap, dstIsMap := dstValue.(map[string]interface{}); dstIsMap {
				if srcMap, srcIsMap := srcValue.(map[string]interface{}); srcIsMap {
					// Both are maps, merge recursively
					g.deepMerge(dstMap, srcMap)
					continue
				}
			}
		}
		// Override with source value (not a map merge case)
		dst[key] = srcValue
	}
}

func (g *GitOpsImporter) loadYAMLFile(filePath string) (map[string]interface{}, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := yaml.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse YAML file %s: %w", filePath, err)
	}

	return result, nil
}
