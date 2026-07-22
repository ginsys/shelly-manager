package database

import (
	"encoding/json"
	"time"
)

// ConfigTemplate represents a reusable configuration template
type ConfigTemplate struct {
	ID          uint            `gorm:"primaryKey" json:"id"`
	Name        string          `gorm:"size:191;uniqueIndex;not null" json:"name"`
	Description string          `json:"description,omitempty"`
	Scope       string          `gorm:"size:191;not null;index" json:"scope"` // "global", "group", "device_type"
	DeviceType  string          `gorm:"size:191;index" json:"device_type,omitempty"`
	Config      json.RawMessage `gorm:"type:text;not null" json:"config"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// TableName specifies the table name for ConfigTemplate
func (ConfigTemplate) TableName() string {
	return "config_templates"
}

// DeviceTag represents a tag assigned to a device for group templates
type DeviceTag struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	DeviceID  uint      `gorm:"not null;index;constraint:OnDelete:CASCADE" json:"device_id"`
	Tag       string    `gorm:"size:191;not null;index" json:"tag"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName specifies the table name for DeviceTag
func (DeviceTag) TableName() string {
	return "device_tags"
}
