package opnsense

import (
	"fmt"
	"time"
)

// SystemStatus represents OPNSense system status information
type SystemStatus struct {
	Version     string    `json:"version"`
	ConfigDate  time.Time `json:"config_date"`
	Uptime      string    `json:"uptime"`
	LoadAverage []float64 `json:"load_average"`
	CPUUsage    float64   `json:"cpu_usage"`
	MemoryUsage struct {
		Used  uint64 `json:"used"`
		Total uint64 `json:"total"`
	} `json:"memory_usage"`
}

// DHCPReservation represents a static DHCP reservation
type DHCPReservation struct {
	UUID        string `json:"uuid,omitempty"`
	MAC         string `json:"mac"`
	IP          string `json:"ip"`
	Hostname    string `json:"hostname"`
	Description string `json:"description,omitempty"`
	Disabled    bool   `json:"disabled,omitempty"`
	Interface   string `json:"interface,omitempty"`
}

// DHCPReservationResponse represents the response from DHCP reservation API
type DHCPReservationResponse struct {
	Status      string            `json:"status"`
	Message     string            `json:"message,omitempty"`
	UUID        string            `json:"uuid,omitempty"`
	Validations map[string]string `json:"validations,omitempty"`
}

// DHCPReservationList represents a list of DHCP reservations
type DHCPReservationList struct {
	Reservations map[string]DHCPReservation `json:"reservations"`
}

// FirewallAlias represents a firewall alias
type FirewallAlias struct {
	UUID        string   `json:"uuid,omitempty"`
	Name        string   `json:"name"`
	Type        string   `json:"type"` // host, network, port, url, etc.
	Content     []string `json:"content"`
	Description string   `json:"description,omitempty"`
	Enabled     bool     `json:"enabled"`
	UpdateFreq  string   `json:"updatefreq,omitempty"`
}

// FirewallAliasResponse represents the response from firewall alias API
type FirewallAliasResponse struct {
	Status      string            `json:"status"`
	Message     string            `json:"message,omitempty"`
	UUID        string            `json:"uuid,omitempty"`
	Validations map[string]string `json:"validations,omitempty"`
}

// FirewallAliasList represents a list of firewall aliases
type FirewallAliasList struct {
	Aliases map[string]FirewallAlias `json:"aliases"`
}

// ConfigurationStatus represents the status of configuration changes
type ConfigurationStatus struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Changed bool   `json:"changed"`
}

// SyncResult represents the result of a synchronization operation
type SyncResult struct {
	Success             bool          `json:"success"`
	ReservationsAdded   int           `json:"reservations_added"`
	ReservationsUpdated int           `json:"reservations_updated"`
	ReservationsDeleted int           `json:"reservations_deleted"`
	AliasesUpdated      int           `json:"aliases_updated"`
	Errors              []string      `json:"errors,omitempty"`
	Warnings            []string      `json:"warnings,omitempty"`
	Duration            time.Duration `json:"duration"`
}

// ConflictResolution defines how to handle conflicts during sync
type ConflictResolution string

const (
	ConflictResolutionOPNSenseWins ConflictResolution = "opnsense_wins"
	ConflictResolutionManagerWins  ConflictResolution = "manager_wins"
	ConflictResolutionManual       ConflictResolution = "manual"
	ConflictResolutionSkip         ConflictResolution = "skip"
)

// SyncOptions defines options for synchronization operations
type SyncOptions struct {
	ConflictResolution ConflictResolution `json:"conflict_resolution"`
	DryRun             bool               `json:"dry_run"`
	UpdateFirewall     bool               `json:"update_firewall"`
	BackupBefore       bool               `json:"backup_before"`
	ApplyChanges       bool               `json:"apply_changes"`
}

// DeviceMapping represents how a Shelly device should be mapped in OPNSense
type DeviceMapping struct {
	ShellyMAC        string     `json:"shelly_mac"`
	ShellyIP         string     `json:"shelly_ip"`
	ShellyName       string     `json:"shelly_name"`
	OPNSenseHostname string     `json:"opnsense_hostname"`
	Interface        string     `json:"interface"`
	LastSync         *time.Time `json:"last_sync,omitempty"`
	SyncStatus       string     `json:"sync_status"`
}

// APIError represents an error from the OPNSense API
type APIError struct {
	HTTPStatus int               `json:"http_status"`
	Message    string            `json:"message"`
	Details    map[string]string `json:"details,omitempty"`
}

func (e *APIError) Error() string {
	if len(e.Details) > 0 {
		return fmt.Sprintf("OPNSense API error (HTTP %d): %s, details: %+v", e.HTTPStatus, e.Message, e.Details)
	}
	return fmt.Sprintf("OPNSense API error (HTTP %d): %s", e.HTTPStatus, e.Message)
}
