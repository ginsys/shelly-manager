package configuration

import (
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/ginsys/shelly-manager/internal/logging"
	secrets "github.com/ginsys/shelly-manager/internal/redact"
)

// desiredHistoryKeep is how many rows a device keeps. A rollback undoes recent edits, and a count (not an
// age) leaves an idle device its history. Its newest row is never pruned.
const desiredHistoryKeep = 50

// DesiredConfigHistory is one version of a device's desired configuration: a row per real change,
// written in the transaction that changed it (ginsys/shelly-manager#775). Both texts have every secret
// masked at write, so no row holds a secret. A text that could not be masked is NULL, and a row whose
// NewDesired is NULL is not a version anyone can roll back to. The rollback-only columns exist now so the
// slice that writes them needs no second migration.
type DesiredConfigHistory struct {
	ID                  uint      `gorm:"primaryKey"`
	DeviceID            uint      `gorm:"index;not null"`
	Action              string    `gorm:"not null"` // recompute, import or rollback
	OldDesired          *string   // masked, NULL when empty or not JSON
	NewDesired          *string   // masked, NULL when not JSON
	PreviousTemplateIDs *string   // a rollback's detached template ids
	RestoredFrom        *uint     // the version a rollback restored
	ChangedBy           string    `gorm:"not null"` // "system": no per-person source exists
	CreatedAt           time.Time `gorm:"not null"`
}

func (DesiredConfigHistory) TableName() string {
	return "desired_config_history"
}

// RecordDesiredChange stores the row for a change of a device's desired configuration, in the caller's
// transaction, and prunes the device to its newest rows. Equal texts write no row. The texts are compared
// as they are, byte for byte: a change to a secret alone is a real change, whose two masked sides are equal.
func RecordDesiredChange(tx *gorm.DB, deviceID uint, action, oldText, newText string) error {
	if oldText == newText {
		return nil
	}
	row := DesiredConfigHistory{
		DeviceID:   deviceID,
		Action:     action,
		OldDesired: maskedDesired(deviceID, action, oldText),
		NewDesired: maskedDesired(deviceID, action, newText),
		ChangedBy:  "system",
		CreatedAt:  time.Now().UTC(),
	}
	if err := tx.Create(&row).Error; err != nil {
		return fmt.Errorf("failed to record the desired configuration change: %w", err)
	}
	return pruneDesiredHistory(tx, deviceID)
}

// maskedDesired returns the text with every secret masked, as the API masks a desired configuration. An
// empty text is NULL. A text that is not JSON cannot be masked and is never stored raw: it is NULL too,
// and the row still records that the change happened.
func maskedDesired(deviceID uint, action, text string) *string {
	if text == "" {
		return nil
	}
	doc, ok := secrets.DecodeJSON([]byte(text))
	if ok {
		secrets.MaskTree(doc, nil)
		if out, err := secrets.EncodeJSON(doc); err == nil {
			masked := string(out)
			return &masked
		}
	}
	logging.GetDefault().WithFields(map[string]any{
		"device_id": deviceID,
		"action":    action,
		"component": "config_history",
	}).Warn("A desired configuration text could not be masked and is not stored in the history")
	return nil
}

// pruneDesiredHistory deletes the rows of a device older than its newest desiredHistoryKeep. The cutoff
// is read first, so MySQL needs no subquery on the table it deletes from, and under FOR UPDATE so it sees
// the latest committed rows: a plain read in the import's REPEATABLE READ transaction keeps its first
// snapshot, misses a row a concurrent recompute added and pruned, and leaves more than the newest 50.
func pruneDesiredHistory(tx *gorm.DB, deviceID uint) error {
	var cutoff []uint
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Model(&DesiredConfigHistory{}).Where("device_id = ?", deviceID).
		Order("id DESC").Offset(desiredHistoryKeep).Limit(1).Pluck("id", &cutoff).Error
	if err != nil {
		return fmt.Errorf("failed to prune the desired configuration history: %w", err)
	}
	if len(cutoff) == 0 {
		return nil
	}
	if err := tx.Where("device_id = ? AND id <= ?", deviceID, cutoff[0]).Delete(&DesiredConfigHistory{}).Error; err != nil {
		return fmt.Errorf("failed to prune the desired configuration history: %w", err)
	}
	return nil
}
