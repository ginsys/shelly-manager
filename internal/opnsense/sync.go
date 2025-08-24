package opnsense

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// SyncService handles bidirectional synchronization between Shelly Manager and OPNSense
type SyncService struct {
	client          *Client
	dhcpManager     *DHCPManager
	firewallManager *FirewallManager
	logger          *logging.Logger
}

// NewSyncService creates a new sync service
func NewSyncService(client *Client, logger *logging.Logger) *SyncService {
	return &SyncService{
		client:          client,
		dhcpManager:     NewDHCPManager(client),
		firewallManager: NewFirewallManager(client),
		logger:          logger,
	}
}

// BidirectionalSyncConfig defines configuration for bidirectional sync
type BidirectionalSyncConfig struct {
	// Sync settings
	ConflictResolution  ConflictResolution `json:"conflict_resolution"`
	ImportFromOPNSense  bool               `json:"import_from_opnsense"`
	ExportToOPNSense    bool               `json:"export_to_opnsense"`
	SyncFirewallAliases bool               `json:"sync_firewall_aliases"`
	ApplyChanges        bool               `json:"apply_changes"`
	BackupBeforeChanges bool               `json:"backup_before_changes"`

	// Import settings
	ImportInterface   string   `json:"import_interface"`
	ImportOnlyShelly  bool     `json:"import_only_shelly"`
	ShellyIdentifiers []string `json:"shelly_identifiers"` // Keywords to identify Shelly devices

	// Export settings
	DHCPInterface      string   `json:"dhcp_interface"`
	FirewallAliasNames []string `json:"firewall_alias_names"`
	HostnameTemplate   string   `json:"hostname_template"`

	// Validation
	ValidateDevices bool `json:"validate_devices"`
	SkipUnreachable bool `json:"skip_unreachable"`

	// Dry run
	DryRun bool `json:"dry_run"`
}

// BidirectionalSyncResult contains the result of a bidirectional sync operation
type BidirectionalSyncResult struct {
	Success   bool          `json:"success"`
	StartTime time.Time     `json:"start_time"`
	Duration  time.Duration `json:"duration"`

	// Import results
	ImportResult *ImportSyncResult `json:"import_result,omitempty"`

	// Export results
	ExportResult *SyncResult `json:"export_result,omitempty"`

	// Overall statistics
	TotalDevicesProcessed int `json:"total_devices_processed"`
	DevicesAdded          int `json:"devices_added"`
	DevicesUpdated        int `json:"devices_updated"`
	DevicesSkipped        int `json:"devices_skipped"`
	ConflictsResolved     int `json:"conflicts_resolved"`

	// Issues
	Errors    []string       `json:"errors,omitempty"`
	Warnings  []string       `json:"warnings,omitempty"`
	Conflicts []SyncConflict `json:"conflicts,omitempty"`
}

// ImportSyncResult contains the result of importing from OPNSense
type ImportSyncResult struct {
	Success              bool             `json:"success"`
	ReservationsFound    int              `json:"reservations_found"`
	ReservationsImported int              `json:"reservations_imported"`
	ReservationsSkipped  int              `json:"reservations_skipped"`
	ImportedDevices      []ImportedDevice `json:"imported_devices,omitempty"`
	Errors               []string         `json:"errors,omitempty"`
	Warnings             []string         `json:"warnings,omitempty"`
}

// ImportedDevice represents a device imported from OPNSense
type ImportedDevice struct {
	MAC             string  `json:"mac"`
	IP              string  `json:"ip"`
	Hostname        string  `json:"hostname"`
	Description     string  `json:"description"`
	Source          string  `json:"source"` // "dhcp_reservation", "discovered"
	IsShelly        bool    `json:"is_shelly"`
	ConfidenceScore float64 `json:"confidence_score"` // 0.0-1.0
}

// SyncConflict represents a conflict that needs resolution
type SyncConflict struct {
	Type               string      `json:"type"` // "ip_mismatch", "hostname_mismatch", "mac_mismatch"
	DeviceMAC          string      `json:"device_mac"`
	ShellyManagerValue interface{} `json:"shelly_manager_value"`
	OPNSenseValue      interface{} `json:"opnsense_value"`
	Resolution         string      `json:"resolution"` // "manager_wins", "opnsense_wins", "manual", "skipped"
	ResolvedValue      interface{} `json:"resolved_value,omitempty"`
}

// PerformBidirectionalSync performs a complete bidirectional synchronization
func (s *SyncService) PerformBidirectionalSync(ctx context.Context, shellyDevices []DeviceMapping, config BidirectionalSyncConfig) (*BidirectionalSyncResult, error) {
	startTime := time.Now()

	s.logger.Info("Starting bidirectional synchronization",
		"shelly_devices", len(shellyDevices),
		"import_enabled", config.ImportFromOPNSense,
		"export_enabled", config.ExportToOPNSense,
		"conflict_resolution", config.ConflictResolution,
		"dry_run", config.DryRun,
	)

	result := &BidirectionalSyncResult{
		Success:   true,
		StartTime: startTime,
		Errors:    []string{},
		Warnings:  []string{},
		Conflicts: []SyncConflict{},
	}

	// Step 1: Import from OPNSense if enabled
	var importedDevices []ImportedDevice
	if config.ImportFromOPNSense {
		importResult, err := s.importFromOPNSense(ctx, config)
		result.ImportResult = importResult

		if err != nil {
			result.Success = false
			result.Errors = append(result.Errors, fmt.Sprintf("import failed: %v", err))
		} else if importResult.Success {
			importedDevices = importResult.ImportedDevices
		}
	}

	// Step 2: Resolve conflicts between Shelly Manager and OPNSense data
	resolvedDevices, conflicts, err := s.resolveConflicts(ctx, shellyDevices, importedDevices, config)
	if err != nil {
		result.Success = false
		result.Errors = append(result.Errors, fmt.Sprintf("conflict resolution failed: %v", err))
	}

	result.Conflicts = conflicts
	result.ConflictsResolved = len(conflicts)

	// Step 3: Export to OPNSense if enabled
	if config.ExportToOPNSense && len(resolvedDevices) > 0 {
		syncOptions := SyncOptions{
			ConflictResolution: config.ConflictResolution,
			DryRun:             config.DryRun,
			ApplyChanges:       config.ApplyChanges,
			BackupBefore:       config.BackupBeforeChanges,
		}

		exportResult, err := s.dhcpManager.SyncReservations(ctx, resolvedDevices, syncOptions)
		result.ExportResult = exportResult

		if err != nil {
			result.Success = false
			result.Errors = append(result.Errors, fmt.Sprintf("export failed: %v", err))
		} else {
			result.DevicesAdded = exportResult.ReservationsAdded
			result.DevicesUpdated = exportResult.ReservationsUpdated
		}

		// Sync firewall aliases if enabled
		if config.SyncFirewallAliases && len(config.FirewallAliasNames) > 0 {
			for _, aliasName := range config.FirewallAliasNames {
				aliasConfigs := map[string][]DeviceMapping{
					aliasName: resolvedDevices,
				}

				_, err := s.firewallManager.SyncShellyDeviceAliases(ctx, aliasConfigs, syncOptions)
				if err != nil {
					result.Warnings = append(result.Warnings, fmt.Sprintf("failed to sync firewall alias %s: %v", aliasName, err))
				}
			}
		}
	}

	// Calculate final statistics
	result.TotalDevicesProcessed = len(shellyDevices) + len(importedDevices)
	result.Duration = time.Since(startTime)

	s.logger.Info("Bidirectional synchronization completed",
		"success", result.Success,
		"duration", result.Duration,
		"total_processed", result.TotalDevicesProcessed,
		"added", result.DevicesAdded,
		"updated", result.DevicesUpdated,
		"conflicts", result.ConflictsResolved,
		"errors", len(result.Errors),
		"warnings", len(result.Warnings),
	)

	return result, nil
}

// importFromOPNSense imports devices from OPNSense DHCP reservations
func (s *SyncService) importFromOPNSense(ctx context.Context, config BidirectionalSyncConfig) (*ImportSyncResult, error) {
	s.logger.Info("Importing devices from OPNSense", "interface", config.ImportInterface)

	result := &ImportSyncResult{
		Success:         true,
		ImportedDevices: []ImportedDevice{},
		Errors:          []string{},
		Warnings:        []string{},
	}

	// Get DHCP reservations from OPNSense
	reservations, err := s.dhcpManager.GetReservations(ctx, config.ImportInterface)
	if err != nil {
		result.Success = false
		result.Errors = append(result.Errors, fmt.Sprintf("failed to fetch DHCP reservations: %v", err))
		return result, err
	}

	result.ReservationsFound = len(reservations)

	// Process each reservation
	for _, reservation := range reservations {
		if reservation.MAC == "" || reservation.IP == "" {
			result.ReservationsSkipped++
			continue
		}

		// Check if this looks like a Shelly device
		isShelly, confidence := s.identifyShelly(reservation, config.ShellyIdentifiers)

		if config.ImportOnlyShelly && !isShelly {
			result.ReservationsSkipped++
			continue
		}

		importedDevice := ImportedDevice{
			MAC:             reservation.MAC,
			IP:              reservation.IP,
			Hostname:        reservation.Hostname,
			Description:     reservation.Description,
			Source:          "dhcp_reservation",
			IsShelly:        isShelly,
			ConfidenceScore: confidence,
		}

		result.ImportedDevices = append(result.ImportedDevices, importedDevice)
		result.ReservationsImported++
	}

	s.logger.Info("Import from OPNSense completed",
		"found", result.ReservationsFound,
		"imported", result.ReservationsImported,
		"skipped", result.ReservationsSkipped,
	)

	return result, nil
}

// identifyShelly determines if a device is likely a Shelly device
func (s *SyncService) identifyShelly(reservation DHCPReservation, identifiers []string) (bool, float64) {
	// Default identifiers if none provided
	if len(identifiers) == 0 {
		identifiers = []string{"shelly", "allterco", "shellyplus", "shelly1", "shelly2"}
	}

	confidence := 0.0
	matchCount := 0

	// Check hostname
	if reservation.Hostname != "" {
		hostname := strings.ToLower(reservation.Hostname)
		for _, identifier := range identifiers {
			if strings.Contains(hostname, strings.ToLower(identifier)) {
				matchCount++
				confidence += 0.4
			}
		}
	}

	// Check description
	if reservation.Description != "" {
		description := strings.ToLower(reservation.Description)
		for _, identifier := range identifiers {
			if strings.Contains(description, strings.ToLower(identifier)) {
				matchCount++
				confidence += 0.3
			}
		}
	}

	// Check MAC address patterns (Allterco/Shelly OUIs)
	mac := strings.ToUpper(strings.ReplaceAll(reservation.MAC, ":", ""))
	shellyOUIs := []string{
		"8CAAB5", // Allterco Robotics Ltd
		"C45BBE", // Another common Shelly OUI
		"84CCA8", // Another Shelly OUI
		"3CDBBC", // Another Shelly OUI
	}

	for _, oui := range shellyOUIs {
		if strings.HasPrefix(mac, oui) {
			confidence += 0.6
			matchCount++
			break
		}
	}

	// Normalize confidence to 0.0-1.0 range
	if confidence > 1.0 {
		confidence = 1.0
	}

	// Consider it a Shelly device if confidence > 0.5 or we have multiple matches
	isShelly := confidence > 0.5 || matchCount >= 2

	return isShelly, confidence
}

// resolveConflicts resolves conflicts between Shelly Manager and OPNSense data
func (s *SyncService) resolveConflicts(ctx context.Context, shellyDevices []DeviceMapping, importedDevices []ImportedDevice, config BidirectionalSyncConfig) ([]DeviceMapping, []SyncConflict, error) {
	s.logger.Info("Resolving conflicts between Shelly Manager and OPNSense data",
		"shelly_devices", len(shellyDevices),
		"imported_devices", len(importedDevices),
		"strategy", config.ConflictResolution,
	)

	var resolvedDevices []DeviceMapping
	var conflicts []SyncConflict

	// Create lookup map for imported devices by MAC
	importedByMAC := make(map[string]ImportedDevice)
	for _, imported := range importedDevices {
		normalizedMAC := s.normalizeMAC(imported.MAC)
		importedByMAC[normalizedMAC] = imported
	}

	// Process each Shelly device
	for _, shellyDevice := range shellyDevices {
		normalizedMAC := s.normalizeMAC(shellyDevice.ShellyMAC)
		imported, exists := importedByMAC[normalizedMAC]

		if !exists {
			// No conflict - device only exists in Shelly Manager
			resolvedDevices = append(resolvedDevices, shellyDevice)
			continue
		}

		// Device exists in both - check for conflicts
		conflictsFound, resolvedDevice := s.resolveDeviceConflicts(shellyDevice, imported, config.ConflictResolution)
		conflicts = append(conflicts, conflictsFound...)
		resolvedDevices = append(resolvedDevices, resolvedDevice)

		// Remove from imported map so we don't process it again
		delete(importedByMAC, normalizedMAC)
	}

	// Add remaining imported devices (exist only in OPNSense)
	for _, imported := range importedByMAC {
		if imported.IsShelly || !config.ImportOnlyShelly {
			deviceMapping := DeviceMapping{
				ShellyMAC:        imported.MAC,
				ShellyIP:         imported.IP,
				ShellyName:       imported.Hostname,
				OPNSenseHostname: imported.Hostname,
				Interface:        config.DHCPInterface,
				SyncStatus:       "imported_from_opnsense",
			}
			resolvedDevices = append(resolvedDevices, deviceMapping)
		}
	}

	s.logger.Info("Conflict resolution completed",
		"resolved_devices", len(resolvedDevices),
		"conflicts_found", len(conflicts),
	)

	return resolvedDevices, conflicts, nil
}

// resolveDeviceConflicts resolves conflicts for a single device
func (s *SyncService) resolveDeviceConflicts(shellyDevice DeviceMapping, imported ImportedDevice, strategy ConflictResolution) ([]SyncConflict, DeviceMapping) {
	var conflicts []SyncConflict
	resolvedDevice := shellyDevice // Start with Shelly Manager data

	// Check IP address conflict
	if shellyDevice.ShellyIP != imported.IP {
		conflict := SyncConflict{
			Type:               "ip_mismatch",
			DeviceMAC:          shellyDevice.ShellyMAC,
			ShellyManagerValue: shellyDevice.ShellyIP,
			OPNSenseValue:      imported.IP,
		}

		switch strategy {
		case ConflictResolutionManagerWins:
			conflict.Resolution = "manager_wins"
			conflict.ResolvedValue = shellyDevice.ShellyIP
			// Keep Shelly Manager IP
		case ConflictResolutionOPNSenseWins:
			conflict.Resolution = "opnsense_wins"
			conflict.ResolvedValue = imported.IP
			resolvedDevice.ShellyIP = imported.IP
		case ConflictResolutionSkip:
			conflict.Resolution = "skipped"
			// Keep original values, mark as skipped
		default:
			conflict.Resolution = "manual"
			// Keep Shelly Manager value but flag for manual resolution
		}

		conflicts = append(conflicts, conflict)
	}

	// Check hostname conflict
	if shellyDevice.OPNSenseHostname != imported.Hostname {
		conflict := SyncConflict{
			Type:               "hostname_mismatch",
			DeviceMAC:          shellyDevice.ShellyMAC,
			ShellyManagerValue: shellyDevice.OPNSenseHostname,
			OPNSenseValue:      imported.Hostname,
		}

		switch strategy {
		case ConflictResolutionManagerWins:
			conflict.Resolution = "manager_wins"
			conflict.ResolvedValue = shellyDevice.OPNSenseHostname
			// Keep Shelly Manager hostname
		case ConflictResolutionOPNSenseWins:
			conflict.Resolution = "opnsense_wins"
			conflict.ResolvedValue = imported.Hostname
			resolvedDevice.OPNSenseHostname = imported.Hostname
		case ConflictResolutionSkip:
			conflict.Resolution = "skipped"
		default:
			conflict.Resolution = "manual"
		}

		conflicts = append(conflicts, conflict)
	}

	return conflicts, resolvedDevice
}

// normalizeMAC normalizes MAC address for comparison
func (s *SyncService) normalizeMAC(mac string) string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(mac), ":", ""), "-", "")
}

// ValidateDevices validates that devices are reachable (placeholder)
func (s *SyncService) ValidateDevices(ctx context.Context, devices []DeviceMapping) ([]DeviceMapping, error) {
	// This could be implemented to ping devices or perform other validation
	// For now, return all devices as valid
	s.logger.Debug("Validating device reachability", "count", len(devices))
	return devices, nil
}

// GetSyncStatus gets the current synchronization status
func (s *SyncService) GetSyncStatus(ctx context.Context, mac string) (*DeviceMapping, error) {
	// This could query the current sync status for a device
	// Implementation would depend on how sync status is stored
	return nil, fmt.Errorf("not implemented")
}

// ScheduleSync schedules automatic synchronization (placeholder)
func (s *SyncService) ScheduleSync(ctx context.Context, config BidirectionalSyncConfig, interval time.Duration) error {
	// This would implement scheduled synchronization
	// Could use cron jobs or similar scheduling mechanism
	s.logger.Info("Scheduling automatic sync", "interval", interval)
	return fmt.Errorf("not implemented - scheduled sync would be implemented here")
}
