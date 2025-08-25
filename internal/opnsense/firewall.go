package opnsense

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"
)

// FirewallManager manages firewall aliases in OPNSense
type FirewallManager struct {
	client *Client
}

// NewFirewallManager creates a new firewall manager
func NewFirewallManager(client *Client) *FirewallManager {
	return &FirewallManager{
		client: client,
	}
}

// GetAliases retrieves all firewall aliases from OPNSense
func (f *FirewallManager) GetAliases(ctx context.Context) ([]FirewallAlias, error) {
	f.client.logger.Debug("Fetching firewall aliases")

	responseBody, err := f.client.makeRequest(ctx, "GET", "/api/firewall/alias/searchItem", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch firewall aliases: %w", err)
	}

	var response FirewallAliasList
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse firewall aliases response: %w", err)
	}

	// Convert map to slice
	aliases := make([]FirewallAlias, 0, len(response.Aliases))
	for uuid, alias := range response.Aliases {
		alias.UUID = uuid
		aliases = append(aliases, alias)
	}

	f.client.logger.Info("Retrieved firewall aliases", "count", len(aliases))
	return aliases, nil
}

// GetAlias retrieves a specific firewall alias by UUID
func (f *FirewallManager) GetAlias(ctx context.Context, uuid string) (*FirewallAlias, error) {
	f.client.logger.Debug("Fetching firewall alias", "uuid", uuid)

	endpoint := fmt.Sprintf("/api/firewall/alias/getItem/%s", uuid)
	responseBody, err := f.client.makeRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch firewall alias %s: %w", uuid, err)
	}

	var alias FirewallAlias
	if err := json.Unmarshal(responseBody, &alias); err != nil {
		return nil, fmt.Errorf("failed to parse firewall alias response: %w", err)
	}

	alias.UUID = uuid
	return &alias, nil
}

// FindAliasByName finds a firewall alias by name
func (f *FirewallManager) FindAliasByName(ctx context.Context, name string) (*FirewallAlias, error) {
	aliases, err := f.GetAliases(ctx)
	if err != nil {
		return nil, err
	}

	for _, alias := range aliases {
		if strings.EqualFold(alias.Name, name) {
			return &alias, nil
		}
	}

	return nil, fmt.Errorf("no firewall alias found with name %s", name)
}

// CreateAlias creates a new firewall alias
func (f *FirewallManager) CreateAlias(ctx context.Context, alias FirewallAlias) (*FirewallAliasResponse, error) {
	f.client.logger.Info("Creating firewall alias",
		"name", alias.Name,
		"type", alias.Type,
		"content_count", len(alias.Content),
	)

	// Validate alias data
	if err := f.validateAlias(alias); err != nil {
		return nil, fmt.Errorf("invalid alias data: %w", err)
	}

	responseBody, err := f.client.makeRequest(ctx, "POST", "/api/firewall/alias/addItem", alias)
	if err != nil {
		return nil, fmt.Errorf("failed to create firewall alias: %w", err)
	}

	var response FirewallAliasResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse create alias response: %w", err)
	}

	if response.Status != "ok" {
		return &response, fmt.Errorf("failed to create alias: %s", response.Message)
	}

	f.client.logger.Info("Firewall alias created successfully",
		"uuid", response.UUID,
		"name", alias.Name,
	)

	return &response, nil
}

// UpdateAlias updates an existing firewall alias
func (f *FirewallManager) UpdateAlias(ctx context.Context, uuid string, alias FirewallAlias) (*FirewallAliasResponse, error) {
	f.client.logger.Info("Updating firewall alias",
		"uuid", uuid,
		"name", alias.Name,
		"type", alias.Type,
		"content_count", len(alias.Content),
	)

	// Validate alias data
	if err := f.validateAlias(alias); err != nil {
		return nil, fmt.Errorf("invalid alias data: %w", err)
	}

	endpoint := fmt.Sprintf("/api/firewall/alias/setItem/%s", uuid)
	responseBody, err := f.client.makeRequest(ctx, "POST", endpoint, alias)
	if err != nil {
		return nil, fmt.Errorf("failed to update firewall alias %s: %w", uuid, err)
	}

	var response FirewallAliasResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse update alias response: %w", err)
	}

	if response.Status != "ok" {
		return &response, fmt.Errorf("failed to update alias: %s", response.Message)
	}

	f.client.logger.Info("Firewall alias updated successfully", "uuid", uuid)
	return &response, nil
}

// DeleteAlias deletes a firewall alias
func (f *FirewallManager) DeleteAlias(ctx context.Context, uuid string) (*FirewallAliasResponse, error) {
	f.client.logger.Info("Deleting firewall alias", "uuid", uuid)

	endpoint := fmt.Sprintf("/api/firewall/alias/delItem/%s", uuid)
	responseBody, err := f.client.makeRequest(ctx, "POST", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to delete firewall alias %s: %w", uuid, err)
	}

	var response FirewallAliasResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse delete alias response: %w", err)
	}

	if response.Status != "ok" {
		return &response, fmt.Errorf("failed to delete alias: %s", response.Message)
	}

	f.client.logger.Info("Firewall alias deleted successfully", "uuid", uuid)
	return &response, nil
}

// UpdateShellyDeviceAlias updates a firewall alias with Shelly device IP addresses
func (f *FirewallManager) UpdateShellyDeviceAlias(ctx context.Context, aliasName string, devices []DeviceMapping, createIfNotExists bool) (*FirewallAliasResponse, error) {
	f.client.logger.Info("Updating Shelly device alias",
		"alias_name", aliasName,
		"device_count", len(devices),
		"create_if_not_exists", createIfNotExists,
	)

	// Extract IP addresses from devices
	ipAddresses := make([]string, 0, len(devices))
	for _, device := range devices {
		if device.ShellyIP != "" {
			// Validate IP address
			if ip := net.ParseIP(device.ShellyIP); ip != nil {
				ipAddresses = append(ipAddresses, device.ShellyIP)
			} else {
				f.client.logger.Warn("Skipping invalid IP address", "ip", device.ShellyIP, "device", device.ShellyName)
			}
		}
	}

	if len(ipAddresses) == 0 {
		return nil, fmt.Errorf("no valid IP addresses found in device list")
	}

	// Check if alias exists
	existingAlias, err := f.FindAliasByName(ctx, aliasName)
	if err != nil && !createIfNotExists {
		return nil, fmt.Errorf("alias %s not found and create_if_not_exists is false", aliasName)
	}

	if existingAlias != nil {
		// Update existing alias
		existingAlias.Content = ipAddresses
		existingAlias.Description = fmt.Sprintf("Shelly devices auto-updated by Shelly Manager (last update: %s)", time.Now().Format("2006-01-02 15:04:05"))
		return f.UpdateAlias(ctx, existingAlias.UUID, *existingAlias)
	} else {
		// Create new alias
		newAlias := FirewallAlias{
			Name:        aliasName,
			Type:        "host",
			Content:     ipAddresses,
			Description: fmt.Sprintf("Shelly devices managed by Shelly Manager (created: %s)", time.Now().Format("2006-01-02 15:04:05")),
			Enabled:     true,
		}
		return f.CreateAlias(ctx, newAlias)
	}
}

// SyncShellyDeviceAliases synchronizes multiple firewall aliases with Shelly device groups
func (f *FirewallManager) SyncShellyDeviceAliases(ctx context.Context, aliasConfigs map[string][]DeviceMapping, options SyncOptions) (*SyncResult, error) {
	startTime := time.Now()

	f.client.logger.Info("Starting firewall alias synchronization",
		"alias_count", len(aliasConfigs),
		"dry_run", options.DryRun,
	)

	result := &SyncResult{
		Success:  true,
		Errors:   []string{},
		Warnings: []string{},
	}

	// Process each alias configuration
	for aliasName, devices := range aliasConfigs {
		f.client.logger.Debug("Processing alias", "alias_name", aliasName, "device_count", len(devices))

		if !options.DryRun {
			response, err := f.UpdateShellyDeviceAlias(ctx, aliasName, devices, true)
			if err != nil {
				result.Success = false
				result.Errors = append(result.Errors, fmt.Sprintf("failed to update alias %s: %v", aliasName, err))
				continue
			}

			if response.Status != "ok" {
				result.Warnings = append(result.Warnings, fmt.Sprintf("alias %s update completed with warnings: %s", aliasName, response.Message))
			}
		}

		result.AliasesUpdated++
	}

	// Apply firewall configuration if requested and not a dry run
	if !options.DryRun && options.ApplyChanges {
		if err := f.ApplyConfiguration(ctx); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("failed to apply firewall configuration: %v", err))
		}
	}

	result.Duration = time.Since(startTime)

	f.client.logger.Info("Firewall alias synchronization completed",
		"success", result.Success,
		"aliases_updated", result.AliasesUpdated,
		"duration", result.Duration,
		"errors", len(result.Errors),
		"warnings", len(result.Warnings),
	)

	return result, nil
}

// ApplyConfiguration applies pending firewall configuration changes
func (f *FirewallManager) ApplyConfiguration(ctx context.Context) error {
	f.client.logger.Info("Applying firewall configuration changes")

	responseBody, err := f.client.makeRequest(ctx, "POST", "/api/firewall/alias/reconfigure", nil)
	if err != nil {
		return fmt.Errorf("failed to apply firewall configuration: %w", err)
	}

	var response ConfigurationStatus
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return fmt.Errorf("failed to parse configuration response: %w", err)
	}

	if response.Status != "ok" {
		return fmt.Errorf("failed to apply configuration: %s", response.Message)
	}

	f.client.logger.Info("Firewall configuration applied successfully")
	return nil
}

// validateAlias validates a firewall alias
func (f *FirewallManager) validateAlias(alias FirewallAlias) error {
	// Validate name
	if alias.Name == "" {
		return fmt.Errorf("alias name is required")
	}
	if len(alias.Name) > 32 {
		return fmt.Errorf("alias name too long (max 32 characters)")
	}

	// Validate name format (alphanumeric and underscores only)
	for _, r := range alias.Name {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return fmt.Errorf("alias name contains invalid characters (only alphanumeric and underscores allowed)")
		}
	}

	// Validate type
	validTypes := map[string]bool{
		"host":         true,
		"network":      true,
		"port":         true,
		"url":          true,
		"url_ports":    true,
		"urltable":     true,
		"geoip":        true,
		"networkgroup": true,
		"mac":          true,
		"dynipv6host":  true,
		"openvpngroup": true,
	}
	if !validTypes[alias.Type] {
		return fmt.Errorf("invalid alias type: %s", alias.Type)
	}

	// Validate content
	if len(alias.Content) == 0 {
		return fmt.Errorf("alias content is required")
	}

	// Validate content based on type
	switch alias.Type {
	case "host", "network":
		for _, content := range alias.Content {
			if err := f.validateIPOrNetwork(content); err != nil {
				return fmt.Errorf("invalid %s content '%s': %w", alias.Type, content, err)
			}
		}
	case "port":
		for _, content := range alias.Content {
			if err := f.validatePortOrRange(content); err != nil {
				return fmt.Errorf("invalid port content '%s': %w", content, err)
			}
		}
	}

	return nil
}

// validateIPOrNetwork validates an IP address or network CIDR
func (f *FirewallManager) validateIPOrNetwork(content string) error {
	// Check if it's a CIDR network
	if strings.Contains(content, "/") {
		_, _, err := net.ParseCIDR(content)
		return err
	}

	// Check if it's an IP address
	if ip := net.ParseIP(content); ip == nil {
		return fmt.Errorf("invalid IP address or network")
	}

	return nil
}

// validatePortOrRange validates a port number or port range
func (f *FirewallManager) validatePortOrRange(content string) error {
	// This is a simplified validation - OPNSense supports complex port expressions
	if strings.Contains(content, "-") {
		// Port range
		parts := strings.SplitN(content, "-", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid port range format")
		}
		// Additional port range validation could be added here
	}

	// For now, accept any non-empty content for ports
	if content == "" {
		return fmt.Errorf("empty port content")
	}

	return nil
}

// GetAliasUsage gets information about where a firewall alias is used
func (f *FirewallManager) GetAliasUsage(ctx context.Context, aliasName string) (map[string]interface{}, error) {
	f.client.logger.Debug("Checking firewall alias usage", "alias_name", aliasName)

	queryParams := map[string]string{
		"alias": aliasName,
	}

	responseBody, err := f.client.makeRequestWithQuery(ctx, "GET", "/api/firewall/alias/getAliasUUID", queryParams, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get alias usage: %w", err)
	}

	var usage map[string]interface{}
	if err := json.Unmarshal(responseBody, &usage); err != nil {
		return nil, fmt.Errorf("failed to parse alias usage response: %w", err)
	}

	return usage, nil
}
