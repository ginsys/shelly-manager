package opnsense

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"
)

// DHCPManager manages DHCP reservations in OPNSense
type DHCPManager struct {
	client *Client
}

// NewDHCPManager creates a new DHCP manager
func NewDHCPManager(client *Client) *DHCPManager {
	return &DHCPManager{
		client: client,
	}
}

// GetReservations retrieves all DHCP reservations from OPNSense
func (d *DHCPManager) GetReservations(ctx context.Context, interfaceName string) ([]DHCPReservation, error) {
	d.client.logger.Debug("Fetching DHCP reservations", "interface", interfaceName)

	queryParams := make(map[string]string)
	if interfaceName != "" {
		queryParams["interface"] = interfaceName
	}

	responseBody, err := d.client.makeRequestWithQuery(ctx, "GET", "/api/dhcp/leases/searchReservations", queryParams, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch DHCP reservations: %w", err)
	}

	var response DHCPReservationList
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse DHCP reservations response: %w", err)
	}

	// Convert map to slice
	reservations := make([]DHCPReservation, 0, len(response.Reservations))
	for uuid, reservation := range response.Reservations {
		reservation.UUID = uuid
		reservations = append(reservations, reservation)
	}

	d.client.logger.Info("Retrieved DHCP reservations",
		"count", len(reservations),
		"interface", interfaceName,
	)

	return reservations, nil
}

// GetReservation retrieves a specific DHCP reservation by UUID
func (d *DHCPManager) GetReservation(ctx context.Context, uuid string) (*DHCPReservation, error) {
	d.client.logger.Debug("Fetching DHCP reservation", "uuid", uuid)

	endpoint := fmt.Sprintf("/api/dhcp/leases/getReservation/%s", uuid)
	responseBody, err := d.client.makeRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch DHCP reservation %s: %w", uuid, err)
	}

	var reservation DHCPReservation
	if err := json.Unmarshal(responseBody, &reservation); err != nil {
		return nil, fmt.Errorf("failed to parse DHCP reservation response: %w", err)
	}

	reservation.UUID = uuid
	return &reservation, nil
}

// CreateReservation creates a new DHCP reservation
func (d *DHCPManager) CreateReservation(ctx context.Context, reservation DHCPReservation) (*DHCPReservationResponse, error) {
	d.client.logger.Info("Creating DHCP reservation",
		"mac", reservation.MAC,
		"ip", reservation.IP,
		"hostname", reservation.Hostname,
	)

	// Validate reservation data
	if err := d.validateReservation(reservation); err != nil {
		return nil, fmt.Errorf("invalid reservation data: %w", err)
	}

	responseBody, err := d.client.makeRequest(ctx, "POST", "/api/dhcp/leases/addReservation", reservation)
	if err != nil {
		return nil, fmt.Errorf("failed to create DHCP reservation: %w", err)
	}

	var response DHCPReservationResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse create reservation response: %w", err)
	}

	if response.Status != "ok" {
		return &response, fmt.Errorf("failed to create reservation: %s", response.Message)
	}

	d.client.logger.Info("DHCP reservation created successfully",
		"uuid", response.UUID,
		"mac", reservation.MAC,
		"ip", reservation.IP,
	)

	return &response, nil
}

// UpdateReservation updates an existing DHCP reservation
func (d *DHCPManager) UpdateReservation(ctx context.Context, uuid string, reservation DHCPReservation) (*DHCPReservationResponse, error) {
	d.client.logger.Info("Updating DHCP reservation",
		"uuid", uuid,
		"mac", reservation.MAC,
		"ip", reservation.IP,
		"hostname", reservation.Hostname,
	)

	// Validate reservation data
	if err := d.validateReservation(reservation); err != nil {
		return nil, fmt.Errorf("invalid reservation data: %w", err)
	}

	endpoint := fmt.Sprintf("/api/dhcp/leases/setReservation/%s", uuid)
	responseBody, err := d.client.makeRequest(ctx, "POST", endpoint, reservation)
	if err != nil {
		return nil, fmt.Errorf("failed to update DHCP reservation %s: %w", uuid, err)
	}

	var response DHCPReservationResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse update reservation response: %w", err)
	}

	if response.Status != "ok" {
		return &response, fmt.Errorf("failed to update reservation: %s", response.Message)
	}

	d.client.logger.Info("DHCP reservation updated successfully", "uuid", uuid)
	return &response, nil
}

// DeleteReservation deletes a DHCP reservation
func (d *DHCPManager) DeleteReservation(ctx context.Context, uuid string) (*DHCPReservationResponse, error) {
	d.client.logger.Info("Deleting DHCP reservation", "uuid", uuid)

	endpoint := fmt.Sprintf("/api/dhcp/leases/delReservation/%s", uuid)
	responseBody, err := d.client.makeRequest(ctx, "POST", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to delete DHCP reservation %s: %w", uuid, err)
	}

	var response DHCPReservationResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse delete reservation response: %w", err)
	}

	if response.Status != "ok" {
		return &response, fmt.Errorf("failed to delete reservation: %s", response.Message)
	}

	d.client.logger.Info("DHCP reservation deleted successfully", "uuid", uuid)
	return &response, nil
}

// FindReservationByMAC finds a DHCP reservation by MAC address
func (d *DHCPManager) FindReservationByMAC(ctx context.Context, mac, interfaceName string) (*DHCPReservation, error) {
	reservations, err := d.GetReservations(ctx, interfaceName)
	if err != nil {
		return nil, err
	}

	// Normalize MAC address for comparison
	normalizedMAC := d.normalizeMAC(mac)

	for _, reservation := range reservations {
		if d.normalizeMAC(reservation.MAC) == normalizedMAC {
			return &reservation, nil
		}
	}

	return nil, fmt.Errorf("no reservation found for MAC address %s", mac)
}

// FindReservationByIP finds a DHCP reservation by IP address
func (d *DHCPManager) FindReservationByIP(ctx context.Context, ip, interfaceName string) (*DHCPReservation, error) {
	reservations, err := d.GetReservations(ctx, interfaceName)
	if err != nil {
		return nil, err
	}

	for _, reservation := range reservations {
		if reservation.IP == ip {
			return &reservation, nil
		}
	}

	return nil, fmt.Errorf("no reservation found for IP address %s", ip)
}

// SyncReservations synchronizes Shelly device data with OPNSense DHCP reservations
func (d *DHCPManager) SyncReservations(ctx context.Context, devices []DeviceMapping, options SyncOptions) (*SyncResult, error) {
	startTime := time.Now()

	d.client.logger.Info("Starting DHCP reservation synchronization",
		"device_count", len(devices),
		"dry_run", options.DryRun,
		"conflict_resolution", options.ConflictResolution,
	)

	result := &SyncResult{
		Success:  true,
		Errors:   []string{},
		Warnings: []string{},
	}

	// Get existing reservations
	existingReservations, err := d.GetReservations(ctx, "")
	if err != nil {
		result.Success = false
		result.Errors = append(result.Errors, fmt.Sprintf("failed to fetch existing reservations: %v", err))
		return result, err
	}

	// Create lookup map for existing reservations by MAC
	existingByMAC := make(map[string]DHCPReservation)
	for _, reservation := range existingReservations {
		existingByMAC[d.normalizeMAC(reservation.MAC)] = reservation
	}

	// Process each device
	for _, device := range devices {
		if err := d.syncSingleDevice(ctx, device, existingByMAC, options, result); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("failed to sync device %s: %v", device.ShellyMAC, err))
		}
	}

	// Apply configuration changes if requested and not a dry run
	if !options.DryRun && options.ApplyChanges {
		if err := d.ApplyConfiguration(ctx); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("failed to apply configuration: %v", err))
		}
	}

	result.Duration = time.Since(startTime)

	d.client.logger.Info("DHCP reservation synchronization completed",
		"success", result.Success,
		"added", result.ReservationsAdded,
		"updated", result.ReservationsUpdated,
		"deleted", result.ReservationsDeleted,
		"duration", result.Duration,
		"errors", len(result.Errors),
		"warnings", len(result.Warnings),
	)

	return result, nil
}

// syncSingleDevice synchronizes a single device's DHCP reservation
func (d *DHCPManager) syncSingleDevice(ctx context.Context, device DeviceMapping, existingByMAC map[string]DHCPReservation, options SyncOptions, result *SyncResult) error {
	normalizedMAC := d.normalizeMAC(device.ShellyMAC)
	existing, exists := existingByMAC[normalizedMAC]

	if exists {
		// Check if update is needed
		if existing.IP != device.ShellyIP || existing.Hostname != device.OPNSenseHostname {
			switch options.ConflictResolution {
			case ConflictResolutionManagerWins:
				return d.updateExistingReservation(ctx, existing.UUID, device, options, result)
			case ConflictResolutionOPNSenseWins:
				result.Warnings = append(result.Warnings, fmt.Sprintf("skipping update for %s due to conflict resolution policy", device.ShellyMAC))
			case ConflictResolutionSkip:
				result.Warnings = append(result.Warnings, fmt.Sprintf("skipping conflicted device %s", device.ShellyMAC))
			default:
				result.Warnings = append(result.Warnings, fmt.Sprintf("manual conflict resolution required for device %s", device.ShellyMAC))
			}
		}
	} else {
		// Create new reservation
		return d.createNewReservation(ctx, device, options, result)
	}

	return nil
}

// createNewReservation creates a new DHCP reservation for a device
func (d *DHCPManager) createNewReservation(ctx context.Context, device DeviceMapping, options SyncOptions, result *SyncResult) error {
	reservation := DHCPReservation{
		MAC:         device.ShellyMAC,
		IP:          device.ShellyIP,
		Hostname:    device.OPNSenseHostname,
		Description: fmt.Sprintf("Shelly device: %s", device.ShellyName),
		Interface:   device.Interface,
		Disabled:    false,
	}

	if !options.DryRun {
		response, err := d.CreateReservation(ctx, reservation)
		if err != nil {
			return err
		}
		if response.Status != "ok" {
			return fmt.Errorf("creation failed: %s", response.Message)
		}
	}

	result.ReservationsAdded++
	return nil
}

// updateExistingReservation updates an existing DHCP reservation
func (d *DHCPManager) updateExistingReservation(ctx context.Context, uuid string, device DeviceMapping, options SyncOptions, result *SyncResult) error {
	reservation := DHCPReservation{
		MAC:         device.ShellyMAC,
		IP:          device.ShellyIP,
		Hostname:    device.OPNSenseHostname,
		Description: fmt.Sprintf("Shelly device: %s", device.ShellyName),
		Interface:   device.Interface,
		Disabled:    false,
	}

	if !options.DryRun {
		response, err := d.UpdateReservation(ctx, uuid, reservation)
		if err != nil {
			return err
		}
		if response.Status != "ok" {
			return fmt.Errorf("update failed: %s", response.Message)
		}
	}

	result.ReservationsUpdated++
	return nil
}

// ApplyConfiguration applies pending DHCP configuration changes
func (d *DHCPManager) ApplyConfiguration(ctx context.Context) error {
	d.client.logger.Info("Applying DHCP configuration changes")

	responseBody, err := d.client.makeRequest(ctx, "POST", "/api/dhcp/service/reconfigure", nil)
	if err != nil {
		return fmt.Errorf("failed to apply DHCP configuration: %w", err)
	}

	var response ConfigurationStatus
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return fmt.Errorf("failed to parse configuration response: %w", err)
	}

	if response.Status != "ok" {
		return fmt.Errorf("failed to apply configuration: %s", response.Message)
	}

	d.client.logger.Info("DHCP configuration applied successfully")
	return nil
}

// validateReservation validates a DHCP reservation
func (d *DHCPManager) validateReservation(reservation DHCPReservation) error {
	// Validate MAC address
	if reservation.MAC == "" {
		return fmt.Errorf("MAC address is required")
	}
	if _, err := net.ParseMAC(reservation.MAC); err != nil {
		return fmt.Errorf("invalid MAC address format: %w", err)
	}

	// Validate IP address
	if reservation.IP == "" {
		return fmt.Errorf("IP address is required")
	}
	if ip := net.ParseIP(reservation.IP); ip == nil {
		return fmt.Errorf("invalid IP address format: %s", reservation.IP)
	}

	// Validate hostname
	if reservation.Hostname == "" {
		return fmt.Errorf("hostname is required")
	}
	if len(reservation.Hostname) > 63 {
		return fmt.Errorf("hostname too long (max 63 characters)")
	}

	return nil
}

// normalizeMAC normalizes MAC address format for comparison
func (d *DHCPManager) normalizeMAC(mac string) string {
	// Remove all separators and convert to lowercase
	normalized := strings.ReplaceAll(mac, ":", "")
	normalized = strings.ReplaceAll(normalized, "-", "")
	normalized = strings.ToLower(normalized)
	return normalized
}

// GenerateHostname generates a hostname for a Shelly device
func (d *DHCPManager) GenerateHostname(device DeviceMapping, template string) string {
	if template == "" {
		template = "shelly-{{.Type}}-{{.MAC | last4}}"
	}

	// Simple template replacement
	hostname := template
	hostname = strings.ReplaceAll(hostname, "{{.Type}}", strings.ToLower(device.ShellyName))
	hostname = strings.ReplaceAll(hostname, "{{.MAC | last4}}", d.getLastFourMAC(device.ShellyMAC))
	hostname = strings.ReplaceAll(hostname, "{{.Name}}", strings.ToLower(device.ShellyName))

	// Ensure hostname is valid
	hostname = d.sanitizeHostname(hostname)
	return hostname
}

// getLastFourMAC gets the last 4 characters of a MAC address
func (d *DHCPManager) getLastFourMAC(mac string) string {
	normalized := d.normalizeMAC(mac)
	if len(normalized) >= 4 {
		return normalized[len(normalized)-4:]
	}
	return normalized
}

// sanitizeHostname ensures hostname meets DNS requirements
func (d *DHCPManager) sanitizeHostname(hostname string) string {
	// Convert to lowercase
	hostname = strings.ToLower(hostname)

	// Replace invalid characters with hyphens
	var result strings.Builder
	for _, r := range hostname {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		} else {
			result.WriteRune('-')
		}
	}

	hostname = result.String()

	// Remove leading/trailing hyphens
	hostname = strings.Trim(hostname, "-")

	// Ensure maximum length
	if len(hostname) > 63 {
		hostname = hostname[:63]
	}

	// Ensure not empty
	if hostname == "" {
		hostname = "shelly-device"
	}

	return hostname
}
