package opnsense

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/opnsense"
	"github.com/ginsys/shelly-manager/internal/sync"
)

// OPNSensePlugin implements the SyncPlugin interface for OPNSense integration
type OPNSensePlugin struct {
	client          *opnsense.Client
	dhcpManager     *opnsense.DHCPManager
	firewallManager *opnsense.FirewallManager
	logger          *logging.Logger
}

// NewPlugin creates a new OPNSense plugin (for registry)
func NewPlugin() sync.SyncPlugin {
	return &OPNSensePlugin{}
}

// NewOPNSenseExporter creates a new OPNSense exporter (backward compatibility)
func NewOPNSenseExporter() *OPNSensePlugin {
	return &OPNSensePlugin{}
}

// Info returns plugin information
func (o *OPNSensePlugin) Info() sync.PluginInfo {
	return sync.PluginInfo{
		Name:        "opnsense",
		Version:     "1.0.0",
		Description: "Export and synchronize Shelly devices with OPNSense DHCP reservations and firewall aliases",
		Author:      "Shelly Manager Team",
		License:     "MIT",
		SupportedFormats: []string{
			"dhcp_reservations",  // Direct DHCP API synchronization
			"firewall_aliases",   // Firewall alias updates
			"bidirectional_sync", // Two-way synchronization
			"xml_config",         // XML configuration export
		},
		Tags:     []string{"opnsense", "dhcp", "firewall", "networking", "synchronization"},
		Category: sync.CategoryNetworking,
	}
}

// ConfigSchema returns the configuration schema
func (o *OPNSensePlugin) ConfigSchema() sync.ConfigSchema {
	return sync.ConfigSchema{
		Version: "1.0",
		Properties: map[string]sync.PropertySchema{
			"host": {
				Type:        "string",
				Description: "OPNSense hostname or IP address",
			},
			"port": {
				Type:        "number",
				Description: "OPNSense port number",
				Default:     443,
				Minimum:     func(v float64) *float64 { return &v }(1),
				Maximum:     func(v float64) *float64 { return &v }(65535),
			},
			"use_https": {
				Type:        "boolean",
				Description: "Use HTTPS for API connections",
				Default:     true,
			},
			"api_key": {
				Type:        "string",
				Description: "OPNSense API key",
				Sensitive:   true,
			},
			"api_secret": {
				Type:        "string",
				Description: "OPNSense API secret",
				Sensitive:   true,
			},
			"insecure_skip_verify": {
				Type:        "boolean",
				Description: "Skip TLS certificate verification",
				Default:     false,
			},
			"timeout": {
				Type:        "number",
				Description: "API request timeout in seconds",
				Default:     30,
				Minimum:     func(v float64) *float64 { return &v }(1),
				Maximum:     func(v float64) *float64 { return &v }(300),
			},
			"dhcp_interface": {
				Type:        "string",
				Description: "DHCP interface name (e.g., 'lan')",
				Default:     "lan",
			},
			"hostname_template": {
				Type:        "string",
				Description: "Template for generating device hostnames",
				Default:     "shelly-{{.Type}}-{{.MAC | last4}}",
			},
			"firewall_alias_name": {
				Type:        "string",
				Description: "Name of firewall alias to update with device IPs",
				Default:     "shelly_devices",
			},
			"sync_mode": {
				Type:        "string",
				Description: "Synchronization mode",
				Default:     "unidirectional",
				Enum:        []interface{}{"unidirectional", "bidirectional"},
			},
			"conflict_resolution": {
				Type:        "string",
				Description: "How to handle conflicts during sync",
				Default:     "manager_wins",
				Enum:        []interface{}{"manager_wins", "opnsense_wins", "skip", "manual"},
			},
			"apply_changes": {
				Type:        "boolean",
				Description: "Automatically apply configuration changes",
				Default:     true,
			},
			"backup_before_changes": {
				Type:        "boolean",
				Description: "Create configuration backup before making changes",
				Default:     true,
			},
			"include_discovered": {
				Type:        "boolean",
				Description: "Include recently discovered devices",
				Default:     false,
			},
		},
		Required: []string{"host", "api_key", "api_secret"},
		Examples: []map[string]interface{}{
			{
				"host":                  "192.168.1.1",
				"port":                  443,
				"use_https":             true,
				"api_key":               "${OPNSENSE_API_KEY}",
				"api_secret":            "${OPNSENSE_API_SECRET}",
				"dhcp_interface":        "lan",
				"hostname_template":     "shelly-{{.Type}}-{{.MAC | last4}}",
				"firewall_alias_name":   "shelly_devices",
				"sync_mode":             "bidirectional",
				"conflict_resolution":   "manager_wins",
				"apply_changes":         true,
				"backup_before_changes": true,
			},
		},
	}
}

// ValidateConfig validates the plugin configuration
func (o *OPNSensePlugin) ValidateConfig(config map[string]interface{}) error {
	// Required fields validation
	requiredFields := []string{"host", "api_key", "api_secret"}
	for _, field := range requiredFields {
		if _, exists := config[field]; !exists {
			return fmt.Errorf("required field '%s' is missing", field)
		}
		if value, ok := config[field].(string); !ok || value == "" {
			return fmt.Errorf("field '%s' must be a non-empty string", field)
		}
	}

	// Validate sync_mode
	if syncMode, exists := config["sync_mode"]; exists {
		if mode, ok := syncMode.(string); ok {
			validModes := map[string]bool{
				"unidirectional": true,
				"bidirectional":  true,
			}
			if !validModes[mode] {
				return fmt.Errorf("invalid sync_mode: %s", mode)
			}
		}
	}

	// Validate conflict_resolution
	if conflictRes, exists := config["conflict_resolution"]; exists {
		if res, ok := conflictRes.(string); ok {
			validResolutions := map[string]bool{
				"manager_wins":  true,
				"opnsense_wins": true,
				"skip":          true,
				"manual":        true,
			}
			if !validResolutions[res] {
				return fmt.Errorf("invalid conflict_resolution: %s", res)
			}
		}
	}

	// Validate port range
	if port, exists := config["port"]; exists {
		if portNum, ok := port.(float64); ok {
			if portNum < 1 || portNum > 65535 {
				return fmt.Errorf("port must be between 1 and 65535")
			}
		}
	}

	// Validate timeout
	if timeout, exists := config["timeout"]; exists {
		if timeoutNum, ok := timeout.(float64); ok {
			if timeoutNum < 1 || timeoutNum > 300 {
				return fmt.Errorf("timeout must be between 1 and 300 seconds")
			}
		}
	}

	return nil
}

// Export performs the export operation
func (o *OPNSensePlugin) Export(ctx context.Context, data *sync.ExportData, config sync.ExportConfig) (*sync.ExportResult, error) {
	startTime := time.Now()

	o.logger.Info("Starting OPNSense export",
		"format", config.Format,
		"devices", len(data.Devices),
	)

	// Initialize OPNSense client
	if err := o.initializeClient(config.Config); err != nil {
		return nil, fmt.Errorf("failed to initialize OPNSense client: %w", err)
	}

	// Test connection
	if err := o.client.TestConnection(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to OPNSense: %w", err)
	}

	// Convert export data to device mappings
	deviceMappings := o.convertToDeviceMappings(data, config.Config)

	var result *sync.ExportResult
	var err error

	// Perform export based on format
	switch config.Format {
	case "dhcp_reservations":
		result, err = o.exportDHCPReservations(ctx, deviceMappings, config)
	case "firewall_aliases":
		result, err = o.exportFirewallAliases(ctx, deviceMappings, config)
	case "bidirectional_sync":
		result, err = o.exportBidirectionalSync(ctx, deviceMappings, config)
	case "xml_config":
		result, err = o.exportXMLConfig(ctx, deviceMappings, config)
	default:
		return nil, fmt.Errorf("unsupported export format: %s", config.Format)
	}

	if err != nil {
		return &sync.ExportResult{
			Success:  false,
			Duration: time.Since(startTime),
			Errors:   []string{err.Error()},
		}, err
	}

	// Update result with common fields
	result.Duration = time.Since(startTime)

	o.logger.Info("OPNSense export completed",
		"format", config.Format,
		"success", result.Success,
		"duration", result.Duration,
		"records", result.RecordCount,
	)

	return result, nil
}

// Preview generates a preview of what would be exported
func (o *OPNSensePlugin) Preview(ctx context.Context, data *sync.ExportData, config sync.ExportConfig) (*sync.PreviewResult, error) {
	deviceMappings := o.convertToDeviceMappings(data, config.Config)

	var sampleData strings.Builder
	sampleData.WriteString(fmt.Sprintf("OPNSense Export Preview - Format: %s\n", config.Format))
	sampleData.WriteString(fmt.Sprintf("Total Devices: %d\n\n", len(deviceMappings)))

	switch config.Format {
	case "dhcp_reservations":
		sampleData.WriteString("DHCP Reservations:\n")
		for i, device := range deviceMappings {
			if i >= 5 { // Limit preview to 5 devices
				sampleData.WriteString("...\n")
				break
			}
			sampleData.WriteString(fmt.Sprintf("- %s -> %s (%s)\n", device.ShellyMAC, device.ShellyIP, device.OPNSenseHostname))
		}
	case "firewall_aliases":
		aliasName := o.getStringConfig(config.Config, "firewall_alias_name", "shelly_devices")
		sampleData.WriteString(fmt.Sprintf("Firewall Alias: %s\n", aliasName))
		sampleData.WriteString("IP Addresses:\n")
		for i, device := range deviceMappings {
			if i >= 10 { // Limit preview to 10 IPs
				sampleData.WriteString("...\n")
				break
			}
			sampleData.WriteString(fmt.Sprintf("- %s\n", device.ShellyIP))
		}
	case "bidirectional_sync":
		sampleData.WriteString("Bidirectional Sync Preview:\n")
		sampleData.WriteString("- DHCP reservations will be synchronized\n")
		sampleData.WriteString("- Firewall aliases will be updated\n")
		sampleData.WriteString("- Existing OPNSense data will be imported\n")
	case "xml_config":
		sampleData.WriteString("XML Configuration Export:\n")
		sampleData.WriteString("- DHCP static mappings\n")
		sampleData.WriteString("- Firewall aliases\n")
		sampleData.WriteString("- System configuration\n")
	}

	estimatedSize := int64(len(deviceMappings) * 200) // Rough estimate

	return &sync.PreviewResult{
		Success:       true,
		SampleData:    []byte(sampleData.String()),
		RecordCount:   len(deviceMappings),
		EstimatedSize: estimatedSize,
	}, nil
}

// Import performs OPNSense configuration import
func (o *OPNSensePlugin) Import(ctx context.Context, source sync.ImportSource, config sync.ImportConfig) (*sync.ImportResult, error) {
	// TODO: Implement OPNSense import functionality
	return nil, fmt.Errorf("OPNSense import functionality not yet implemented")
}

// Capabilities returns plugin capabilities
func (o *OPNSensePlugin) Capabilities() sync.PluginCapabilities {
	return sync.PluginCapabilities{
		SupportsIncremental:    true,
		SupportsScheduling:     true,
		RequiresAuthentication: true,
		SupportedOutputs:       []string{"api", "file"},
		MaxDataSize:            1024 * 1024 * 100, // 100MB
		ConcurrencyLevel:       1,                 // OPNSense API calls should be sequential
	}
}

// Initialize initializes the plugin
func (o *OPNSensePlugin) Initialize(logger *logging.Logger) error {
	o.logger = logger
	o.logger.Info("Initialized OPNSense exporter plugin")
	return nil
}

// Cleanup cleans up plugin resources
func (o *OPNSensePlugin) Cleanup() error {
	if o.client != nil {
		o.client.Close()
	}
	o.logger.Info("Cleaning up OPNSense exporter plugin")
	return nil
}

// initializeClient initializes the OPNSense client from configuration
func (o *OPNSensePlugin) initializeClient(config map[string]interface{}) error {
	clientConfig := opnsense.ClientConfig{
		Host:               o.getStringConfig(config, "host", ""),
		Port:               int(o.getFloatConfig(config, "port", 443)),
		UseHTTPS:           o.getBoolConfig(config, "use_https", true),
		APIKey:             o.getStringConfig(config, "api_key", ""),
		APISecret:          o.getStringConfig(config, "api_secret", ""),
		Timeout:            time.Duration(o.getFloatConfig(config, "timeout", 30)) * time.Second,
		InsecureSkipVerify: o.getBoolConfig(config, "insecure_skip_verify", false),
	}

	var err error
	o.client, err = opnsense.NewClient(clientConfig, o.logger)
	if err != nil {
		return err
	}

	o.dhcpManager = opnsense.NewDHCPManager(o.client)
	o.firewallManager = opnsense.NewFirewallManager(o.client)

	return nil
}

// convertToDeviceMappings converts export data to OPNSense device mappings
func (o *OPNSensePlugin) convertToDeviceMappings(data *sync.ExportData, config map[string]interface{}) []opnsense.DeviceMapping {
	hostnameTemplate := o.getStringConfig(config, "hostname_template", "shelly-{{.Type}}-{{.MAC | last4}}")
	dhcpInterface := o.getStringConfig(config, "dhcp_interface", "lan")
	includeDiscovered := o.getBoolConfig(config, "include_discovered", false)

	var mappings []opnsense.DeviceMapping

	// Add configured devices
	for _, device := range data.Devices {
		if device.MAC == "" || device.IP == "" {
			continue
		}

		hostname := o.generateHostname(device, hostnameTemplate)
		mapping := opnsense.DeviceMapping{
			ShellyMAC:        device.MAC,
			ShellyIP:         device.IP,
			ShellyName:       device.Name,
			OPNSenseHostname: hostname,
			Interface:        dhcpInterface,
			SyncStatus:       "pending",
		}
		mappings = append(mappings, mapping)
	}

	// Add discovered devices if requested
	if includeDiscovered {
		for _, device := range data.DiscoveredDevices {
			if device.MAC == "" || device.IP == "" {
				continue
			}

			// Create a pseudo device for hostname generation
			pseudoDevice := sync.DeviceData{
				MAC:  device.MAC,
				IP:   device.IP,
				Name: fmt.Sprintf("%s-%s", device.Model, device.MAC[len(device.MAC)-4:]),
				Type: device.Model,
			}

			hostname := o.generateHostname(pseudoDevice, hostnameTemplate)
			mapping := opnsense.DeviceMapping{
				ShellyMAC:        device.MAC,
				ShellyIP:         device.IP,
				ShellyName:       fmt.Sprintf("Discovered %s", device.Model),
				OPNSenseHostname: hostname,
				Interface:        dhcpInterface,
				SyncStatus:       "discovered",
			}
			mappings = append(mappings, mapping)
		}
	}

	return mappings
}

// generateHostname generates a hostname for a device using a template
func (o *OPNSensePlugin) generateHostname(device sync.DeviceData, template string) string {
	// Simple template replacement
	hostname := template
	hostname = strings.ReplaceAll(hostname, "{{.Type}}", strings.ToLower(device.Type))
	hostname = strings.ReplaceAll(hostname, "{{.Name}}", strings.ToLower(device.Name))
	hostname = strings.ReplaceAll(hostname, "{{.MAC | last4}}", o.getLastFourMAC(device.MAC))

	// Sanitize hostname
	return o.sanitizeHostname(hostname)
}

// getLastFourMAC gets the last 4 characters of a MAC address
func (o *OPNSensePlugin) getLastFourMAC(mac string) string {
	normalized := strings.ReplaceAll(strings.ToLower(mac), ":", "")
	if len(normalized) >= 4 {
		return normalized[len(normalized)-4:]
	}
	return normalized
}

// sanitizeHostname ensures hostname meets DNS requirements
func (o *OPNSensePlugin) sanitizeHostname(hostname string) string {
	hostname = strings.ToLower(hostname)
	var result strings.Builder
	for _, r := range hostname {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		} else {
			result.WriteRune('-')
		}
	}
	hostname = strings.Trim(result.String(), "-")
	if len(hostname) > 63 {
		hostname = hostname[:63]
	}
	if hostname == "" {
		hostname = "shelly-device"
	}
	return hostname
}

// Helper functions for configuration access
func (o *OPNSensePlugin) getStringConfig(config map[string]interface{}, key, defaultValue string) string {
	if value, exists := config[key]; exists {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return defaultValue
}

func (o *OPNSensePlugin) getBoolConfig(config map[string]interface{}, key string, defaultValue bool) bool {
	if value, exists := config[key]; exists {
		if b, ok := value.(bool); ok {
			return b
		}
	}
	return defaultValue
}

func (o *OPNSensePlugin) getFloatConfig(config map[string]interface{}, key string, defaultValue float64) float64 {
	if value, exists := config[key]; exists {
		if f, ok := value.(float64); ok {
			return f
		}
	}
	return defaultValue
}

// Export format implementations

// exportDHCPReservations exports devices as DHCP reservations
func (o *OPNSensePlugin) exportDHCPReservations(ctx context.Context, devices []opnsense.DeviceMapping, config sync.ExportConfig) (*sync.ExportResult, error) {
	syncOptions := opnsense.SyncOptions{
		ConflictResolution: opnsense.ConflictResolution(o.getStringConfig(config.Config, "conflict_resolution", "manager_wins")),
		DryRun:             config.Options.DryRun,
		ApplyChanges:       o.getBoolConfig(config.Config, "apply_changes", true),
		BackupBefore:       o.getBoolConfig(config.Config, "backup_before_changes", true),
	}

	result, err := o.dhcpManager.SyncReservations(ctx, devices, syncOptions)
	if err != nil {
		return nil, err
	}

	return &sync.ExportResult{
		Success:     result.Success,
		RecordCount: result.ReservationsAdded + result.ReservationsUpdated,
		Errors:      result.Errors,
		Warnings:    result.Warnings,
		Metadata: map[string]interface{}{
			"reservations_added":   result.ReservationsAdded,
			"reservations_updated": result.ReservationsUpdated,
			"reservations_deleted": result.ReservationsDeleted,
		},
	}, nil
}

// exportFirewallAliases exports devices as firewall aliases
func (o *OPNSensePlugin) exportFirewallAliases(ctx context.Context, devices []opnsense.DeviceMapping, config sync.ExportConfig) (*sync.ExportResult, error) {
	aliasName := o.getStringConfig(config.Config, "firewall_alias_name", "shelly_devices")
	aliasConfigs := map[string][]opnsense.DeviceMapping{
		aliasName: devices,
	}

	syncOptions := opnsense.SyncOptions{
		DryRun:       config.Options.DryRun,
		ApplyChanges: o.getBoolConfig(config.Config, "apply_changes", true),
	}

	result, err := o.firewallManager.SyncShellyDeviceAliases(ctx, aliasConfigs, syncOptions)
	if err != nil {
		return nil, err
	}

	return &sync.ExportResult{
		Success:     result.Success,
		RecordCount: len(devices),
		Errors:      result.Errors,
		Warnings:    result.Warnings,
		Metadata: map[string]interface{}{
			"aliases_updated": result.AliasesUpdated,
			"alias_name":      aliasName,
		},
	}, nil
}

// exportBidirectionalSync performs bidirectional synchronization
func (o *OPNSensePlugin) exportBidirectionalSync(ctx context.Context, devices []opnsense.DeviceMapping, config sync.ExportConfig) (*sync.ExportResult, error) {
	// First sync DHCP reservations
	dhcpResult, err := o.exportDHCPReservations(ctx, devices, config)
	if err != nil {
		return dhcpResult, err
	}

	// Then sync firewall aliases
	firewallResult, err := o.exportFirewallAliases(ctx, devices, config)
	if err != nil {
		// Combine errors from both operations
		if dhcpResult != nil {
			dhcpResult.Errors = append(dhcpResult.Errors, firewallResult.Errors...)
			dhcpResult.Warnings = append(dhcpResult.Warnings, firewallResult.Warnings...)
		}
		return dhcpResult, err
	}

	// Combine results
	combinedResult := &sync.ExportResult{
		Success:     dhcpResult.Success && firewallResult.Success,
		RecordCount: dhcpResult.RecordCount,
		Errors:      append(dhcpResult.Errors, firewallResult.Errors...),
		Warnings:    append(dhcpResult.Warnings, firewallResult.Warnings...),
		Metadata: map[string]interface{}{
			"dhcp":     dhcpResult.Metadata,
			"firewall": firewallResult.Metadata,
		},
	}

	return combinedResult, nil
}

// exportXMLConfig exports XML configuration (placeholder implementation)
func (o *OPNSensePlugin) exportXMLConfig(ctx context.Context, devices []opnsense.DeviceMapping, config sync.ExportConfig) (*sync.ExportResult, error) {
	// This would generate OPNSense XML configuration for import
	// For now, this is a placeholder that could be implemented later

	xmlContent := o.generateXMLConfig(devices, config.Config)

	result := &sync.ExportResult{
		Success:     true,
		RecordCount: len(devices),
		FileSize:    int64(len(xmlContent)),
		Metadata: map[string]interface{}{
			"format":          "xml",
			"config_sections": []string{"dhcp", "firewall"},
		},
	}

	// If file output is requested, save the XML
	if config.Output.Type == "file" && config.Output.Destination != "" {
		// This would save the XML file
		result.OutputPath = config.Output.Destination
	}

	return result, nil
}

// generateXMLConfig generates OPNSense XML configuration (placeholder)
func (o *OPNSensePlugin) generateXMLConfig(devices []opnsense.DeviceMapping, config map[string]interface{}) string {
	// This is a simplified XML generation
	// A complete implementation would generate proper OPNSense XML configuration
	var xml strings.Builder
	xml.WriteString("<?xml version=\"1.0\"?>\n")
	xml.WriteString("<opnsense>\n")
	xml.WriteString("  <dhcp>\n")

	for _, device := range devices {
		xml.WriteString("    <staticmap>\n")
		xml.WriteString(fmt.Sprintf("      <mac>%s</mac>\n", device.ShellyMAC))
		xml.WriteString(fmt.Sprintf("      <ipaddr>%s</ipaddr>\n", device.ShellyIP))
		xml.WriteString(fmt.Sprintf("      <hostname>%s</hostname>\n", device.OPNSenseHostname))
		xml.WriteString(fmt.Sprintf("      <descr>Shelly device: %s</descr>\n", device.ShellyName))
		xml.WriteString("    </staticmap>\n")
	}

	xml.WriteString("  </dhcp>\n")
	xml.WriteString("</opnsense>\n")

	return xml.String()
}
