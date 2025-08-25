package sync

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"plugin"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// ExternalPluginLoader handles loading and management of external export plugins
type ExternalPluginLoader struct {
	pluginDir     string
	loadedPlugins map[string]*LoadedPlugin
	manifests     map[string]*PluginManifest
	logger        *logging.Logger
	mutex         sync.RWMutex
}

// LoadedPlugin represents a loaded external plugin
type LoadedPlugin struct {
	Plugin   ExportPlugin
	Manifest *PluginManifest
	FilePath string
	Loaded   bool
}

// PluginManifest describes an external plugin's metadata
type PluginManifest struct {
	Name             string         `yaml:"name" json:"name"`
	Version          string         `yaml:"version" json:"version"`
	Description      string         `yaml:"description" json:"description"`
	Author           string         `yaml:"author" json:"author"`
	Website          string         `yaml:"website,omitempty" json:"website,omitempty"`
	License          string         `yaml:"license" json:"license"`
	Category         PluginCategory `yaml:"category" json:"category"`
	SupportedFormats []string       `yaml:"supported_formats" json:"supported_formats"`
	Tags             []string       `yaml:"tags,omitempty" json:"tags,omitempty"`

	// Plugin file information
	PluginFile string `yaml:"plugin_file" json:"plugin_file"`
	EntryPoint string `yaml:"entry_point" json:"entry_point"`

	// Dependencies and requirements
	Dependencies []PluginDependency `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`
	MinVersion   string             `yaml:"min_version,omitempty" json:"min_version,omitempty"`
	MaxVersion   string             `yaml:"max_version,omitempty" json:"max_version,omitempty"`

	// Configuration schema
	ConfigSchema *ConfigSchema `yaml:"config_schema,omitempty" json:"config_schema,omitempty"`

	// Template-based plugin content
	Templates map[string]string      `yaml:"templates,omitempty" json:"templates,omitempty"`
	Variables map[string]interface{} `yaml:"variables,omitempty" json:"variables,omitempty"`

	// Security and validation
	Checksum  string `yaml:"checksum,omitempty" json:"checksum,omitempty"`
	Signature string `yaml:"signature,omitempty" json:"signature,omitempty"`
	Trusted   bool   `yaml:"trusted,omitempty" json:"trusted,omitempty"`

	// Runtime settings
	Capabilities   *PluginCapabilities `yaml:"capabilities,omitempty" json:"capabilities,omitempty"`
	ResourceLimits *ResourceLimits     `yaml:"resource_limits,omitempty" json:"resource_limits,omitempty"`
}

// PluginDependency describes a plugin dependency
type PluginDependency struct {
	Name       string `yaml:"name" json:"name"`
	Version    string `yaml:"version" json:"version"`
	Optional   bool   `yaml:"optional,omitempty" json:"optional,omitempty"`
	Repository string `yaml:"repository,omitempty" json:"repository,omitempty"`
}

// ResourceLimits defines resource constraints for a plugin
type ResourceLimits struct {
	MaxMemoryMB      int      `yaml:"max_memory_mb,omitempty" json:"max_memory_mb,omitempty"`
	MaxCPUPercent    int      `yaml:"max_cpu_percent,omitempty" json:"max_cpu_percent,omitempty"`
	MaxExecutionTime string   `yaml:"max_execution_time,omitempty" json:"max_execution_time,omitempty"`
	MaxFileSize      int64    `yaml:"max_file_size,omitempty" json:"max_file_size,omitempty"`
	AllowedPaths     []string `yaml:"allowed_paths,omitempty" json:"allowed_paths,omitempty"`
	NetworkAccess    bool     `yaml:"network_access,omitempty" json:"network_access,omitempty"`
}

// PluginLoadResult contains the result of a plugin loading operation
type PluginLoadResult struct {
	Success    bool     `json:"success"`
	PluginName string   `json:"plugin_name"`
	Version    string   `json:"version"`
	FilePath   string   `json:"file_path"`
	Errors     []string `json:"errors,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
}

// NewExternalPluginLoader creates a new external plugin loader
func NewExternalPluginLoader(pluginDir string, logger *logging.Logger) *ExternalPluginLoader {
	return &ExternalPluginLoader{
		pluginDir:     pluginDir,
		loadedPlugins: make(map[string]*LoadedPlugin),
		manifests:     make(map[string]*PluginManifest),
		logger:        logger,
	}
}

// LoadAllPlugins loads all plugins from the plugin directory
func (epl *ExternalPluginLoader) LoadAllPlugins() ([]*PluginLoadResult, error) {
	epl.mutex.Lock()
	defer epl.mutex.Unlock()

	epl.logger.Info("Loading all plugins from directory", "dir", epl.pluginDir)

	if err := epl.ensurePluginDir(); err != nil {
		return nil, fmt.Errorf("failed to ensure plugin directory: %w", err)
	}

	// First, scan for manifest files
	manifests, err := epl.scanManifests()
	if err != nil {
		return nil, fmt.Errorf("failed to scan plugin manifests: %w", err)
	}

	var results []*PluginLoadResult

	// Load each plugin
	for name, manifest := range manifests {
		result := epl.loadSinglePlugin(name, manifest)
		results = append(results, result)
	}

	epl.logger.Info("Plugin loading completed",
		"total_plugins", len(results),
		"successful", epl.countSuccessfulLoads(results),
	)

	return results, nil
}

// LoadPlugin loads a specific plugin by name
func (epl *ExternalPluginLoader) LoadPlugin(pluginName string) (*PluginLoadResult, error) {
	epl.mutex.Lock()
	defer epl.mutex.Unlock()

	epl.logger.Info("Loading specific plugin", "plugin", pluginName)

	manifest, exists := epl.manifests[pluginName]
	if !exists {
		// Try to find and load manifest
		manifests, err := epl.scanManifests()
		if err != nil {
			return nil, fmt.Errorf("failed to scan manifests: %w", err)
		}

		manifest, exists = manifests[pluginName]
		if !exists {
			return &PluginLoadResult{
				Success:    false,
				PluginName: pluginName,
				Errors:     []string{"plugin manifest not found"},
			}, fmt.Errorf("plugin %s not found", pluginName)
		}
	}

	result := epl.loadSinglePlugin(pluginName, manifest)
	return result, nil
}

// UnloadPlugin unloads a specific plugin
func (epl *ExternalPluginLoader) UnloadPlugin(pluginName string) error {
	epl.mutex.Lock()
	defer epl.mutex.Unlock()

	epl.logger.Info("Unloading plugin", "plugin", pluginName)

	loadedPlugin, exists := epl.loadedPlugins[pluginName]
	if !exists {
		return fmt.Errorf("plugin %s is not loaded", pluginName)
	}

	// Call plugin cleanup
	if err := loadedPlugin.Plugin.Cleanup(); err != nil {
		epl.logger.Warn("Plugin cleanup failed", "plugin", pluginName, "error", err)
	}

	// Remove from loaded plugins
	delete(epl.loadedPlugins, pluginName)

	epl.logger.Info("Plugin unloaded successfully", "plugin", pluginName)
	return nil
}

// GetLoadedPlugins returns all currently loaded plugins
func (epl *ExternalPluginLoader) GetLoadedPlugins() map[string]*LoadedPlugin {
	epl.mutex.RLock()
	defer epl.mutex.RUnlock()

	// Return a copy to prevent external modification
	result := make(map[string]*LoadedPlugin)
	for name, plugin := range epl.loadedPlugins {
		result[name] = plugin
	}
	return result
}

// GetPluginManifests returns all discovered plugin manifests
func (epl *ExternalPluginLoader) GetPluginManifests() map[string]*PluginManifest {
	epl.mutex.RLock()
	defer epl.mutex.RUnlock()

	// Return a copy to prevent external modification
	result := make(map[string]*PluginManifest)
	for name, manifest := range epl.manifests {
		result[name] = manifest
	}
	return result
}

// ReloadPlugin reloads a specific plugin
func (epl *ExternalPluginLoader) ReloadPlugin(pluginName string) (*PluginLoadResult, error) {
	epl.logger.Info("Reloading plugin", "plugin", pluginName)

	// Unload if currently loaded
	if _, exists := epl.loadedPlugins[pluginName]; exists {
		if err := epl.UnloadPlugin(pluginName); err != nil {
			return nil, fmt.Errorf("failed to unload plugin for reload: %w", err)
		}
	}

	// Reload manifest and plugin
	return epl.LoadPlugin(pluginName)
}

// ValidatePlugin validates a plugin without loading it
func (epl *ExternalPluginLoader) ValidatePlugin(pluginName string) (*PluginLoadResult, error) {
	epl.mutex.RLock()
	defer epl.mutex.RUnlock()

	epl.logger.Info("Validating plugin", "plugin", pluginName)

	manifest, exists := epl.manifests[pluginName]
	if !exists {
		return &PluginLoadResult{
			Success:    false,
			PluginName: pluginName,
			Errors:     []string{"plugin manifest not found"},
		}, nil
	}

	result := &PluginLoadResult{
		Success:    true,
		PluginName: pluginName,
		Version:    manifest.Version,
		FilePath:   filepath.Join(epl.pluginDir, manifest.PluginFile),
		Warnings:   []string{},
	}

	// Validate manifest
	if err := epl.validateManifest(manifest); err != nil {
		result.Success = false
		result.Errors = append(result.Errors, fmt.Sprintf("manifest validation failed: %v", err))
	}

	// Check plugin file exists
	pluginPath := filepath.Join(epl.pluginDir, manifest.PluginFile)
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		result.Success = false
		result.Errors = append(result.Errors, "plugin file not found")
	}

	// Validate dependencies
	if err := epl.validateDependencies(manifest.Dependencies); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("dependency check: %v", err))
	}

	return result, nil
}

// Private helper methods

// ensurePluginDir ensures the plugin directory exists
func (epl *ExternalPluginLoader) ensurePluginDir() error {
	if _, err := os.Stat(epl.pluginDir); os.IsNotExist(err) {
		if err := os.MkdirAll(epl.pluginDir, 0755); err != nil {
			return fmt.Errorf("failed to create plugin directory: %w", err)
		}
	}
	return nil
}

// scanManifests scans the plugin directory for manifest files
func (epl *ExternalPluginLoader) scanManifests() (map[string]*PluginManifest, error) {
	manifests := make(map[string]*PluginManifest)

	err := filepath.WalkDir(epl.pluginDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Look for manifest files
		if !d.IsDir() && (strings.HasSuffix(d.Name(), ".yaml") || strings.HasSuffix(d.Name(), ".yml") || strings.HasSuffix(d.Name(), ".json")) {
			// Try to parse as manifest
			manifest, err := epl.parseManifestFile(path)
			if err != nil {
				epl.logger.Warn("Failed to parse manifest file", "file", path, "error", err)
				return nil // Continue with other files
			}

			if manifest != nil {
				manifests[manifest.Name] = manifest
				epl.manifests[manifest.Name] = manifest
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan plugin directory: %w", err)
	}

	return manifests, nil
}

// parseManifestFile parses a manifest file
func (epl *ExternalPluginLoader) parseManifestFile(path string) (*PluginManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var manifest PluginManifest

	// Try YAML first, then JSON
	if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
		err = yaml.Unmarshal(data, &manifest)
	} else if strings.HasSuffix(path, ".json") {
		err = json.Unmarshal(data, &manifest)
	} else {
		return nil, fmt.Errorf("unsupported manifest format")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifest: %w", err)
	}

	// Validate that this is actually a plugin manifest
	if manifest.Name == "" || manifest.PluginFile == "" {
		return nil, nil // Not a plugin manifest, skip silently
	}

	return &manifest, nil
}

// loadSinglePlugin loads a single plugin
func (epl *ExternalPluginLoader) loadSinglePlugin(name string, manifest *PluginManifest) *PluginLoadResult {
	result := &PluginLoadResult{
		PluginName: name,
		Version:    manifest.Version,
		FilePath:   filepath.Join(epl.pluginDir, manifest.PluginFile),
		Warnings:   []string{},
	}

	// Validate manifest
	if err := epl.validateManifest(manifest); err != nil {
		result.Success = false
		result.Errors = append(result.Errors, fmt.Sprintf("manifest validation failed: %v", err))
		return result
	}

	// Check if already loaded
	if _, exists := epl.loadedPlugins[name]; exists {
		result.Success = false
		result.Errors = append(result.Errors, "plugin already loaded")
		return result
	}

	// Load the plugin file
	pluginPath := filepath.Join(epl.pluginDir, manifest.PluginFile)

	// For Go plugins, use plugin.Open
	if strings.HasSuffix(pluginPath, ".so") {
		return epl.loadGoPlugin(name, manifest, pluginPath, result)
	}

	// For template-based plugins, create a template plugin wrapper
	if epl.isTemplatePlugin(manifest) {
		return epl.loadTemplatePlugin(name, manifest, result)
	}

	result.Success = false
	result.Errors = append(result.Errors, "unsupported plugin type")
	return result
}

// loadGoPlugin loads a Go plugin (.so file)
func (epl *ExternalPluginLoader) loadGoPlugin(name string, manifest *PluginManifest, pluginPath string, result *PluginLoadResult) *PluginLoadResult {
	// Load the Go plugin
	p, err := plugin.Open(pluginPath)
	if err != nil {
		result.Success = false
		result.Errors = append(result.Errors, fmt.Sprintf("failed to open plugin: %v", err))
		return result
	}

	// Look for the entry point
	entryPoint := manifest.EntryPoint
	if entryPoint == "" {
		entryPoint = "NewPlugin"
	}

	symbol, err := p.Lookup(entryPoint)
	if err != nil {
		result.Success = false
		result.Errors = append(result.Errors, fmt.Sprintf("failed to find entry point '%s': %v", entryPoint, err))
		return result
	}

	// Try to cast to plugin constructor function
	constructor, ok := symbol.(func() ExportPlugin)
	if !ok {
		result.Success = false
		result.Errors = append(result.Errors, "entry point does not match expected signature")
		return result
	}

	// Create plugin instance
	pluginInstance := constructor()

	// Initialize the plugin
	if err := pluginInstance.Initialize(epl.logger); err != nil {
		result.Success = false
		result.Errors = append(result.Errors, fmt.Sprintf("plugin initialization failed: %v", err))
		return result
	}

	// Store the loaded plugin
	epl.loadedPlugins[name] = &LoadedPlugin{
		Plugin:   pluginInstance,
		Manifest: manifest,
		FilePath: pluginPath,
		Loaded:   true,
	}

	result.Success = true
	epl.logger.Info("Go plugin loaded successfully", "plugin", name, "version", manifest.Version)
	return result
}

// loadTemplatePlugin loads a template-based plugin
func (epl *ExternalPluginLoader) loadTemplatePlugin(name string, manifest *PluginManifest, result *PluginLoadResult) *PluginLoadResult {
	// Create a template plugin wrapper
	templatePlugin := NewTemplatePlugin(manifest, epl.logger)

	// Initialize the plugin
	if err := templatePlugin.Initialize(epl.logger); err != nil {
		result.Success = false
		result.Errors = append(result.Errors, fmt.Sprintf("template plugin initialization failed: %v", err))
		return result
	}

	// Store the loaded plugin
	epl.loadedPlugins[name] = &LoadedPlugin{
		Plugin:   templatePlugin,
		Manifest: manifest,
		FilePath: "",
		Loaded:   true,
	}

	result.Success = true
	epl.logger.Info("Template plugin loaded successfully", "plugin", name, "version", manifest.Version)
	return result
}

// isTemplatePlugin checks if a manifest describes a template-based plugin
func (epl *ExternalPluginLoader) isTemplatePlugin(manifest *PluginManifest) bool {
	return manifest.PluginFile == "" && manifest.ConfigSchema != nil
}

// validateManifest validates a plugin manifest
func (epl *ExternalPluginLoader) validateManifest(manifest *PluginManifest) error {
	if manifest.Name == "" {
		return fmt.Errorf("plugin name is required")
	}
	if manifest.Version == "" {
		return fmt.Errorf("plugin version is required")
	}
	if manifest.PluginFile == "" && !epl.isTemplatePlugin(manifest) {
		return fmt.Errorf("plugin file is required for non-template plugins")
	}
	return nil
}

// validateDependencies validates plugin dependencies
func (epl *ExternalPluginLoader) validateDependencies(deps []PluginDependency) error {
	for _, dep := range deps {
		if !dep.Optional {
			// Check if dependency is available
			if _, exists := epl.loadedPlugins[dep.Name]; !exists {
				return fmt.Errorf("required dependency '%s' not available", dep.Name)
			}
		}
	}
	return nil
}

// countSuccessfulLoads counts successful plugin loads
func (epl *ExternalPluginLoader) countSuccessfulLoads(results []*PluginLoadResult) int {
	count := 0
	for _, result := range results {
		if result.Success {
			count++
		}
	}
	return count
}
