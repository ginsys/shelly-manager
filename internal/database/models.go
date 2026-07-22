package database

import (
	"time"

	"gorm.io/gorm"
)

// Indexed string columns carry `size:191`. Without an explicit size GORM maps a
// Go string to an unbounded text type, and MySQL cannot index that ("BLOB/TEXT
// column used in key specification without a key length"). 191 is the largest
// utf8mb4 prefix that fits MySQL's 767-byte index limit, so it works on old and
// new servers alike; SQLite and PostgreSQL are unaffected in practice.

// Device represents a Shelly device in the database
type Device struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	IP        string    `json:"ip" gorm:"size:191;uniqueIndex"`
	MAC       string    `json:"mac" gorm:"size:191;uniqueIndex;not null"`
	Type      string    `json:"type"`
	Name      string    `json:"name"`
	Firmware  string    `json:"firmware"`
	Status    string    `json:"status" gorm:"size:191;index"`
	LastSeen  time.Time `json:"last_seen" gorm:"index"`
	Settings  string    `json:"settings" gorm:"type:text"` // JSON string
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// These hold JSON documents and are seeded by BeforeSave rather than by a
	// column DEFAULT: MySQL rejects defaults on TEXT columns, which made
	// AutoMigrate fail outright there ("BLOB, TEXT, GEOMETRY or JSON column
	// 'template_ids' can't have a default value").
	TemplateIDs   string `json:"template_ids" gorm:"type:text"`
	Overrides     string `json:"overrides" gorm:"type:text"`
	DesiredConfig string `json:"desired_config" gorm:"type:text"`
	ConfigApplied bool   `json:"config_applied" gorm:"default:false"`
}

// BeforeSave seeds the JSON text columns so an unset field is stored as an
// empty document rather than an empty string. This replaces the column
// defaults these fields used to carry, and applies on every provider.
func (d *Device) BeforeSave(*gorm.DB) error {
	if d.TemplateIDs == "" {
		d.TemplateIDs = "[]"
	}
	if d.Overrides == "" {
		d.Overrides = "{}"
	}
	if d.DesiredConfig == "" {
		d.DesiredConfig = "{}"
	}
	return nil
}

// DiscoveredDevice represents a temporarily discovered Shelly device from provisioning scans
type DiscoveredDevice struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	MAC        string    `json:"mac" gorm:"size:191;not null;index"`
	SSID       string    `json:"ssid"`
	Model      string    `json:"model"`
	Generation int       `json:"generation"`
	IP         string    `json:"ip"`
	Signal     int       `json:"signal"`
	AgentID    string    `json:"agent_id" gorm:"size:191;not null;index"`
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
	ExportID     string    `json:"export_id" gorm:"size:191;uniqueIndex"`
	PluginName   string    `json:"plugin_name" gorm:"size:191;index"`
	Format       string    `json:"format"`
	Name         string    `json:"name" gorm:"type:text"`
	Description  string    `json:"description" gorm:"type:text"`
	RequestedBy  string    `json:"requested_by"`
	Success      bool      `json:"success" gorm:"index"`
	RecordCount  int       `json:"record_count"`
	FileSize     int64     `json:"file_size"`
	FilePath     string    `json:"file_path,omitempty" gorm:"type:text"`
	DurationMs   int64     `json:"duration_ms"`
	ErrorMessage string    `json:"error_message,omitempty" gorm:"type:text"`
	CreatedAt    time.Time `json:"created_at" gorm:"index"`
}

// ImportHistory stores audit records for import operations
type ImportHistory struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	ImportID        string    `json:"import_id" gorm:"size:191;uniqueIndex"`
	PluginName      string    `json:"plugin_name" gorm:"size:191;index"`
	Format          string    `json:"format"`
	RequestedBy     string    `json:"requested_by"`
	Success         bool      `json:"success" gorm:"index"`
	RecordsImported int       `json:"records_imported"`
	RecordsSkipped  int       `json:"records_skipped"`
	DurationMs      int64     `json:"duration_ms"`
	ErrorMessage    string    `json:"error_message,omitempty" gorm:"type:text"`
	CreatedAt       time.Time `json:"created_at" gorm:"index"`
}
