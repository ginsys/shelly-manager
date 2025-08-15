package database

import (
	"time"
)

// Device represents a Shelly device in the database
type Device struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	IP        string    `json:"ip" gorm:"uniqueIndex"`
	MAC       string    `json:"mac"`
	Type      string    `json:"type"`
	Name      string    `json:"name"`
	Firmware  string    `json:"firmware"`
	Status    string    `json:"status"`
	LastSeen  time.Time `json:"last_seen"`
	Settings  string    `json:"settings" gorm:"type:text"` // JSON string
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
