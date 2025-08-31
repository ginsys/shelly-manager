package database

import (
	"time"
)

// Device represents a Shelly device in the database
type Device struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	IP        string    `json:"ip" gorm:"uniqueIndex"`
	MAC       string    `json:"mac" gorm:"uniqueIndex;not null"`
	Type      string    `json:"type"`
	Name      string    `json:"name"`
	Firmware  string    `json:"firmware"`
	Status    string    `json:"status" gorm:"index"`
	LastSeen  time.Time `json:"last_seen" gorm:"index"`
	Settings  string    `json:"settings" gorm:"type:text"` // JSON string
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DiscoveredDevice represents a temporarily discovered Shelly device from provisioning scans
type DiscoveredDevice struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	MAC        string    `json:"mac" gorm:"not null;index"`
	SSID       string    `json:"ssid"`
	Model      string    `json:"model"`
	Generation int       `json:"generation"`
	IP         string    `json:"ip"`
	Signal     int       `json:"signal"`
	AgentID    string    `json:"agent_id" gorm:"not null;index"`
	TaskID     string    `json:"task_id,omitempty"`
	Discovered time.Time `json:"discovered" gorm:"not null;index"`
	ExpiresAt  time.Time `json:"expires_at" gorm:"not null;index"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// DiscoveryUpdate contains fields that should be updated during discovery
type DiscoveryUpdate struct {
	IP       string
	Type     string
	Firmware string
	Status   string
	LastSeen time.Time
}

// ExportHistory stores audit records for export operations
type ExportHistory struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	ExportID     string    `json:"export_id" gorm:"uniqueIndex"`
	PluginName   string    `json:"plugin_name" gorm:"index"`
	Format       string    `json:"format"`
	RequestedBy  string    `json:"requested_by"`
	Success      bool      `json:"success" gorm:"index"`
	RecordCount  int       `json:"record_count"`
	FileSize     int64     `json:"file_size"`
	DurationMs   int64     `json:"duration_ms"`
	ErrorMessage string    `json:"error_message,omitempty" gorm:"type:text"`
	CreatedAt    time.Time `json:"created_at" gorm:"index"`
}

// ImportHistory stores audit records for import operations
type ImportHistory struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	ImportID        string    `json:"import_id" gorm:"uniqueIndex"`
	PluginName      string    `json:"plugin_name" gorm:"index"`
	Format          string    `json:"format"`
	RequestedBy     string    `json:"requested_by"`
	Success         bool      `json:"success" gorm:"index"`
	RecordsImported int       `json:"records_imported"`
	RecordsSkipped  int       `json:"records_skipped"`
	DurationMs      int64     `json:"duration_ms"`
	ErrorMessage    string    `json:"error_message,omitempty" gorm:"type:text"`
	CreatedAt       time.Time `json:"created_at" gorm:"index"`
}
