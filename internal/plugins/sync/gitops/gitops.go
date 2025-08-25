package gitops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/sync"
)

// GitOpsPlugin implements the SyncPlugin interface for GitOps YAML exports
type GitOpsPlugin struct {
	logger *logging.Logger
}

// NewPlugin creates a new GitOps plugin (for registry)
func NewPlugin() sync.SyncPlugin {
	return &GitOpsPlugin{}
}

// NewGitOpsExporter creates a new GitOps exporter (backward compatibility)
func NewGitOpsExporter() *GitOpsPlugin {
	return &GitOpsPlugin{}
}

// Info returns plugin information
func (g *GitOpsPlugin) Info() sync.PluginInfo {
	return sync.PluginInfo{
		Name:        "gitops",
		Version:     "1.0.0",
		Description: "Export device configurations as GitOps-ready YAML files with hierarchical structure",
		Author:      "Shelly Manager Team",
		License:     "MIT",
		SupportedFormats: []string{
			"yaml",      // YAML files in hierarchical structure
			"directory", // Directory structure with YAML files
		},
		Tags:     []string{"gitops", "yaml", "configuration", "iac"},
		Category: sync.CategoryGitOps,
	}
}

// ConfigSchema returns the configuration schema
func (g *GitOpsPlugin) ConfigSchema() sync.ConfigSchema {
	return sync.ConfigSchema{
		Version: "1.0",
		Properties: map[string]sync.PropertySchema{
			"output_path": {
				Type:        "string",
				Description: "Directory path for GitOps YAML files",
				Default:     "./gitops",
			},
			"group_by": {
				Type:        "string",
				Description: "How to group devices (location, type, custom)",
				Default:     "location",
				Enum:        []interface{}{"location", "type", "custom"},
			},
			"include_common": {
				Type:        "boolean",
				Description: "Generate common configuration files",
				Default:     true,
			},
			"include_templates": {
				Type:        "boolean",
				Description: "Include configuration templates",
				Default:     true,
			},
			"group_mapping": {
				Type:        "object",
				Description: "Custom mapping of device names to groups",
			},
			"exclude_fields": {
				Type:        "array",
				Description: "Fields to exclude from exported configurations",
				Items: &sync.PropertySchema{
					Type: "string",
				},
				Default: []interface{}{"id", "created_at", "updated_at", "last_seen"},
			},
			"format_style": {
				Type:        "string",
				Description: "YAML formatting style",
				Default:     "default",
				Enum:        []interface{}{"default", "flow", "literal"},
			},
		},
		Required: []string{},
		Examples: []map[string]interface{}{
			{
				"output_path":       "./config/devices",
				"group_by":          "location",
				"include_common":    true,
				"include_templates": true,
				"exclude_fields":    []string{"id", "created_at", "updated_at"},
			},
		},
	}
}

// ValidateConfig validates the plugin configuration
func (g *GitOpsPlugin) ValidateConfig(config map[string]interface{}) error {
	if outputPath, exists := config["output_path"]; exists {
		if path, ok := outputPath.(string); ok {
			// Check if directory can be created
			if err := os.MkdirAll(path, 0755); err != nil {
				return fmt.Errorf("invalid output_path: cannot create directory %s: %w", path, err)
			}
		} else {
			return fmt.Errorf("output_path must be a string")
		}
	}

	if groupBy, exists := config["group_by"]; exists {
		if gb, ok := groupBy.(string); ok {
			validGroupBy := map[string]bool{
				"location": true,
				"type":     true,
				"custom":   true,
			}
			if !validGroupBy[gb] {
				return fmt.Errorf("invalid group_by: %s", gb)
			}
		} else {
			return fmt.Errorf("group_by must be a string")
		}
	}

	return nil
}

// Export performs the GitOps YAML export
func (g *GitOpsPlugin) Export(ctx context.Context, data *sync.ExportData, config sync.ExportConfig) (*sync.ExportResult, error) {
	startTime := time.Now()

	g.logger.Info("Starting GitOps export",
		"format", config.Format,
		"devices", len(data.Devices),
	)

	// Parse configuration
	outputPath, _ := config.Config["output_path"].(string)
	if outputPath == "" {
		outputPath = "./gitops"
	}

	groupBy, _ := config.Config["group_by"].(string)
	if groupBy == "" {
		groupBy = "location"
	}

	includeCommon, _ := config.Config["include_common"].(bool)
	includeTemplates, _ := config.Config["include_templates"].(bool)
	excludeFields := g.parseExcludeFields(config.Config["exclude_fields"])

	// Clean and create output directory
	if err := os.RemoveAll(outputPath); err != nil {
		g.logger.Warn("Failed to clean output directory", "error", err)
	}
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	recordCount := 0

	// Group devices
	deviceGroups := g.groupDevices(data.Devices, groupBy, config.Config["group_mapping"])

	// Generate common configuration
	if includeCommon {
		commonConfig := g.generateCommonConfig(data)
		commonPath := filepath.Join(outputPath, "common.yaml")
		if err := g.writeYAMLFile(commonPath, commonConfig); err != nil {
			return nil, fmt.Errorf("failed to write common config: %w", err)
		}
		recordCount++
		g.logger.Debug("Generated common configuration", "path", commonPath)
	}

	// Generate group and device configurations
	totalDeviceFiles := 0
	for groupName, devices := range deviceGroups {
		groupPath := filepath.Join(outputPath, "groups", groupName)
		if err := os.MkdirAll(groupPath, 0755); err != nil {
			return nil, fmt.Errorf("failed to create group directory %s: %w", groupPath, err)
		}

		// Generate group-level configuration
		groupConfig := g.generateGroupConfig(groupName, devices, data)
		groupConfigPath := filepath.Join(groupPath, "group.yaml")
		if err := g.writeYAMLFile(groupConfigPath, groupConfig); err != nil {
			return nil, fmt.Errorf("failed to write group config: %w", err)
		}
		recordCount++

		// Group devices by type within the group
		typeGroups := g.groupDevicesByType(devices)

		for deviceType, typeDevices := range typeGroups {
			typePath := filepath.Join(groupPath, strings.ToLower(deviceType))
			if err := os.MkdirAll(typePath, 0755); err != nil {
				return nil, fmt.Errorf("failed to create type directory %s: %w", typePath, err)
			}

			// Generate type-level common configuration
			typeCommonConfig := g.generateTypeCommonConfig(deviceType, groupName)
			typeCommonPath := filepath.Join(typePath, "common.yaml")
			if err := g.writeYAMLFile(typeCommonPath, typeCommonConfig); err != nil {
				return nil, fmt.Errorf("failed to write type common config: %w", err)
			}
			recordCount++

			// Generate individual device configurations
			for _, device := range typeDevices {
				deviceConfig := g.generateDeviceConfig(device, excludeFields)
				deviceFileName := g.sanitizeFilename(device.Name) + ".yaml"
				devicePath := filepath.Join(typePath, deviceFileName)

				if err := g.writeYAMLFile(devicePath, deviceConfig); err != nil {
					return nil, fmt.Errorf("failed to write device config for %s: %w", device.Name, err)
				}
				recordCount++
				totalDeviceFiles++
			}
		}
	}

	// Generate ungrouped devices
	ungroupedDevices := g.getUngroupedDevices(data.Devices, deviceGroups)
	if len(ungroupedDevices) > 0 {
		ungroupedPath := filepath.Join(outputPath, "ungrouped")
		if err := os.MkdirAll(ungroupedPath, 0755); err != nil {
			return nil, fmt.Errorf("failed to create ungrouped directory: %w", err)
		}

		typeGroups := g.groupDevicesByType(ungroupedDevices)
		for deviceType, typeDevices := range typeGroups {
			typePath := filepath.Join(ungroupedPath, strings.ToLower(deviceType))
			if err := os.MkdirAll(typePath, 0755); err != nil {
				return nil, fmt.Errorf("failed to create ungrouped type directory: %w", err)
			}

			for _, device := range typeDevices {
				deviceConfig := g.generateDeviceConfig(device, excludeFields)
				deviceFileName := g.sanitizeFilename(device.Name) + ".yaml"
				devicePath := filepath.Join(typePath, deviceFileName)

				if err := g.writeYAMLFile(devicePath, deviceConfig); err != nil {
					return nil, fmt.Errorf("failed to write ungrouped device config: %w", err)
				}
				recordCount++
				totalDeviceFiles++
			}
		}
	}

	// Generate templates if requested
	if includeTemplates && len(data.Templates) > 0 {
		templatesPath := filepath.Join(outputPath, "templates")
		if err := os.MkdirAll(templatesPath, 0755); err != nil {
			return nil, fmt.Errorf("failed to create templates directory: %w", err)
		}

		for _, template := range data.Templates {
			templateConfig := map[string]interface{}{
				"name":        template.Name,
				"description": template.Description,
				"device_type": template.DeviceType,
				"generation":  template.Generation,
				"config":      template.Config,
				"variables":   template.Variables,
				"is_default":  template.IsDefault,
			}

			templateFileName := g.sanitizeFilename(template.Name) + ".yaml"
			templatePath := filepath.Join(templatesPath, templateFileName)

			if err := g.writeYAMLFile(templatePath, templateConfig); err != nil {
				return nil, fmt.Errorf("failed to write template %s: %w", template.Name, err)
			}
			recordCount++
		}
	}

	// Create summary file
	summary := map[string]interface{}{
		"export_metadata": data.Metadata,
		"structure": map[string]interface{}{
			"total_groups":      len(deviceGroups),
			"total_devices":     len(data.Devices),
			"total_files":       recordCount,
			"device_files":      totalDeviceFiles,
			"template_files":    len(data.Templates),
			"ungrouped_devices": len(ungroupedDevices),
		},
		"generated_at": time.Now(),
		"config":       config.Config,
	}

	summaryPath := filepath.Join(outputPath, "export-summary.yaml")
	if err := g.writeYAMLFile(summaryPath, summary); err != nil {
		g.logger.Warn("Failed to write export summary", "error", err)
	} else {
		recordCount++
	}

	// Calculate total size
	totalSize, err := g.calculateDirectorySize(outputPath)
	if err != nil {
		g.logger.Warn("Failed to calculate directory size", "error", err)
	}

	g.logger.Info("GitOps export completed",
		"path", outputPath,
		"groups", len(deviceGroups),
		"files", recordCount,
		"size", totalSize,
		"duration", time.Since(startTime),
	)

	return &sync.ExportResult{
		Success:     true,
		OutputPath:  outputPath,
		RecordCount: recordCount,
		FileSize:    totalSize,
		Duration:    time.Since(startTime),
		Metadata: map[string]interface{}{
			"output_structure": "hierarchical",
			"group_count":      len(deviceGroups),
			"device_files":     totalDeviceFiles,
			"template_files":   len(data.Templates),
			"grouping_method":  groupBy,
		},
	}, nil
}

// Preview generates a preview of what would be exported
func (g *GitOpsPlugin) Preview(ctx context.Context, data *sync.ExportData, config sync.ExportConfig) (*sync.PreviewResult, error) {
	groupBy, _ := config.Config["group_by"].(string)
	if groupBy == "" {
		groupBy = "location"
	}

	deviceGroups := g.groupDevices(data.Devices, groupBy, config.Config["group_mapping"])
	ungroupedCount := len(g.getUngroupedDevices(data.Devices, deviceGroups))

	// Estimate files
	fileCount := 1 // common.yaml
	for _, devices := range deviceGroups {
		fileCount++ // group.yaml
		typeGroups := g.groupDevicesByType(devices)
		for _, typeDevices := range typeGroups {
			fileCount++                   // type/common.yaml
			fileCount += len(typeDevices) // individual device files
		}
	}

	// Add ungrouped devices
	if ungroupedCount > 0 {
		ungroupedTypeGroups := g.groupDevicesByType(g.getUngroupedDevices(data.Devices, deviceGroups))
		fileCount += len(ungroupedTypeGroups) * ungroupedCount // ungrouped device files
	}

	// Add templates
	if includeTemplates, _ := config.Config["include_templates"].(bool); includeTemplates {
		fileCount += len(data.Templates)
	}

	fileCount++ // export-summary.yaml

	estimatedSize := int64(fileCount) * 2048 // Rough estimate: 2KB per YAML file

	sampleStructure := g.generatePreviewStructure(deviceGroups, data)

	return &sync.PreviewResult{
		Success:       true,
		SampleData:    []byte(sampleStructure),
		RecordCount:   fileCount,
		EstimatedSize: estimatedSize,
	}, nil
}

// Import performs GitOps YAML import
func (g *GitOpsPlugin) Import(ctx context.Context, source sync.ImportSource, config sync.ImportConfig) (*sync.ImportResult, error) {
	// TODO: Implement GitOps import functionality
	return nil, fmt.Errorf("GitOps import functionality not yet implemented")
}

// Capabilities returns plugin capabilities
func (g *GitOpsPlugin) Capabilities() sync.PluginCapabilities {
	return sync.PluginCapabilities{
		SupportsIncremental:    false,
		SupportsScheduling:     true,
		RequiresAuthentication: false,
		SupportedOutputs:       []string{"file"},
		MaxDataSize:            1024 * 1024 * 100, // 100MB
		ConcurrencyLevel:       1,
	}
}

// Initialize initializes the plugin
func (g *GitOpsPlugin) Initialize(logger *logging.Logger) error {
	g.logger = logger
	g.logger.Info("Initialized GitOps exporter plugin")
	return nil
}

// Cleanup cleans up plugin resources
func (g *GitOpsPlugin) Cleanup() error {
	g.logger.Info("Cleaning up GitOps exporter plugin")
	return nil
}

// Helper methods

func (g *GitOpsPlugin) groupDevices(devices []sync.DeviceData, groupBy string, groupMapping interface{}) map[string][]sync.DeviceData {
	groups := make(map[string][]sync.DeviceData)

	for _, device := range devices {
		var groupName string

		switch groupBy {
		case "type":
			groupName = strings.ToLower(device.Type)
		case "location":
			groupName = g.extractLocationFromName(device.Name)
		case "custom":
			groupName = g.getCustomGroup(device.Name, groupMapping)
		default:
			groupName = "default"
		}

		if groupName == "" {
			continue // Will be handled as ungrouped
		}

		groups[groupName] = append(groups[groupName], device)
	}

	return groups
}

func (g *GitOpsPlugin) groupDevicesByType(devices []sync.DeviceData) map[string][]sync.DeviceData {
	groups := make(map[string][]sync.DeviceData)

	for _, device := range devices {
		deviceType := strings.ToLower(device.Type)
		groups[deviceType] = append(groups[deviceType], device)
	}

	return groups
}

func (g *GitOpsPlugin) extractLocationFromName(name string) string {
	// Simple heuristic: look for common location words
	// Order matters - more specific locations first
	name = strings.ToLower(name)
	locations := []string{
		"living", "kitchen", "bedroom", "bathroom", "office",
		"garage", "basement", "attic", "outdoor", "garden", "porch", "room",
	}

	for _, location := range locations {
		if strings.Contains(name, location) {
			return location
		}
	}

	return "" // Will be ungrouped
}

func (g *GitOpsPlugin) getCustomGroup(deviceName string, groupMapping interface{}) string {
	if mapping, ok := groupMapping.(map[string]interface{}); ok {
		if group, exists := mapping[deviceName]; exists {
			if groupStr, ok := group.(string); ok {
				return groupStr
			}
		}
	}
	return ""
}

func (g *GitOpsPlugin) getUngroupedDevices(allDevices []sync.DeviceData, groups map[string][]sync.DeviceData) []sync.DeviceData {
	grouped := make(map[string]bool)
	for _, devices := range groups {
		for _, device := range devices {
			key := fmt.Sprintf("%d-%s", device.ID, device.MAC)
			grouped[key] = true
		}
	}

	var ungrouped []sync.DeviceData
	for _, device := range allDevices {
		key := fmt.Sprintf("%d-%s", device.ID, device.MAC)
		if !grouped[key] {
			ungrouped = append(ungrouped, device)
		}
	}

	return ungrouped
}

func (g *GitOpsPlugin) generateCommonConfig(data *sync.ExportData) map[string]interface{} {
	return map[string]interface{}{
		"# Common configuration applied to all devices": nil,
		"wifi": map[string]interface{}{
			"ssid":     "{{ .Global.wifi_ssid }}",
			"password": "{{ .Global.wifi_password }}",
		},
		"mqtt": map[string]interface{}{
			"enabled": true,
			"server":  "{{ .Global.mqtt_server }}",
			"port":    1883,
		},
		"cloud": map[string]interface{}{
			"enabled": false,
		},
		"sntp": map[string]interface{}{
			"server": "pool.ntp.org",
		},
		"# Export metadata": nil,
		"metadata": map[string]interface{}{
			"exported_at":    data.Timestamp,
			"system_version": data.Metadata.SystemVersion,
			"total_devices":  data.Metadata.TotalDevices,
		},
	}
}

func (g *GitOpsPlugin) generateGroupConfig(groupName string, devices []sync.DeviceData, data *sync.ExportData) map[string]interface{} {
	return map[string]interface{}{
		fmt.Sprintf("# Group configuration for %s", groupName): nil,
		"group": map[string]interface{}{
			"name":         groupName,
			"device_count": len(devices),
		},
		"# Group-specific overrides": nil,
		"mqtt": map[string]interface{}{
			"topic_prefix": fmt.Sprintf("shelly/%s", groupName),
		},
	}
}

func (g *GitOpsPlugin) generateTypeCommonConfig(deviceType, groupName string) map[string]interface{} {
	return map[string]interface{}{
		fmt.Sprintf("# Common configuration for %s devices in %s", deviceType, groupName): nil,
		"device_type":              deviceType,
		"# Type-specific settings": nil,
	}
}

func (g *GitOpsPlugin) generateDeviceConfig(device sync.DeviceData, excludeFields []string) map[string]interface{} {
	config := map[string]interface{}{
		fmt.Sprintf("# Configuration for %s", device.Name): nil,
		"name":     device.Name,
		"mac":      device.MAC,
		"type":     device.Type,
		"firmware": device.Firmware,
	}

	// Add settings if available, excluding specified fields
	if device.Settings != nil {
		filteredSettings := make(map[string]interface{})
		for key, value := range device.Settings {
			excluded := false
			for _, excludeField := range excludeFields {
				if key == excludeField {
					excluded = true
					break
				}
			}
			if !excluded {
				filteredSettings[key] = value
			}
		}
		if len(filteredSettings) > 0 {
			config["settings"] = filteredSettings
		}
	}

	return config
}

func (g *GitOpsPlugin) parseExcludeFields(fieldsInterface interface{}) []string {
	var fields []string
	if fieldSlice, ok := fieldsInterface.([]interface{}); ok {
		for _, field := range fieldSlice {
			if fieldStr, ok := field.(string); ok {
				fields = append(fields, fieldStr)
			}
		}
	}
	return fields
}

func (g *GitOpsPlugin) sanitizeFilename(name string) string {
	// Replace invalid filename characters
	replacer := strings.NewReplacer(
		" ", "-",
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "",
		"?", "",
		"\"", "",
		"<", "",
		">", "",
		"|", "",
	)
	return strings.ToLower(replacer.Replace(name))
}

func (g *GitOpsPlugin) writeYAMLFile(filePath string, data interface{}) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filePath, err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			// Log but continue - file is still usable
			_ = err
		}
	}()

	encoder := yaml.NewEncoder(file)
	encoder.SetIndent(2)
	defer func() {
		if err := encoder.Close(); err != nil {
			// Log but continue - encoder is still functional
			_ = err
		}
	}()

	return encoder.Encode(data)
}

func (g *GitOpsPlugin) calculateDirectorySize(dirPath string) (int64, error) {
	var totalSize int64
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		totalSize += info.Size()
		return nil
	})
	return totalSize, err
}

func (g *GitOpsPlugin) generatePreviewStructure(groups map[string][]sync.DeviceData, data *sync.ExportData) string {
	var preview strings.Builder

	preview.WriteString("GitOps Export Structure Preview:\n\n")
	preview.WriteString("gitops/\n")
	preview.WriteString("├── common.yaml                    # Global settings\n")

	for groupName, devices := range groups {
		preview.WriteString(fmt.Sprintf("├── groups/%s/\n", groupName))
		preview.WriteString(fmt.Sprintf("│   ├── group.yaml              # Group: %s (%d devices)\n", groupName, len(devices)))

		typeGroups := g.groupDevicesByType(devices)
		for deviceType, typeDevices := range typeGroups {
			preview.WriteString(fmt.Sprintf("│   ├── %s/\n", strings.ToLower(deviceType)))
			preview.WriteString(fmt.Sprintf("│   │   ├── common.yaml         # %s settings\n", deviceType))
			for _, device := range typeDevices {
				filename := g.sanitizeFilename(device.Name)
				preview.WriteString(fmt.Sprintf("│   │   └── %s.yaml\n", filename))
			}
		}
	}

	if len(data.Templates) > 0 {
		preview.WriteString("├── templates/\n")
		for _, template := range data.Templates {
			filename := g.sanitizeFilename(template.Name)
			preview.WriteString(fmt.Sprintf("│   └── %s.yaml\n", filename))
		}
	}

	preview.WriteString("└── export-summary.yaml           # Export metadata\n")

	return preview.String()
}
