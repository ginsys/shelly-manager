package database

import (
	"time"

	"github.com/ginsys/shelly-manager/internal/configuration"
)

// ConfigTemplate is a reusable configuration template. The model lives in the
// configuration package, which this package imports and which therefore cannot
// import this one; the alias keeps the Manager's template methods and the schema
// fixups on the one owner of the table.
type ConfigTemplate = configuration.ConfigTemplate

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
