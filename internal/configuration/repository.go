package configuration

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// GormConfigRepository implements ConfigRepository using GORM
type GormConfigRepository struct {
	db     *gorm.DB
	logger *logging.Logger
	// inTx is true for a repository built on a transaction handle (NewGormConfigRepositoryOn).
	inTx bool

	// The pinned apply-lock connections one process may hold at once (see takeLockSlot).
	lockSlotsOnce sync.Once
	lockSlots     chan struct{}
	lockPoolOfOne bool
}

// DbConfigTemplate is the repository's narrow view of the config_templates table: the
// eight columns a ServiceConfigTemplate carries. ConfigTemplate owns the table and the
// database Manager migrates it, so this type declares no constraint or index and is
// never migrated; saving through it leaves generation, variables and is_default as
// they were.
type DbConfigTemplate struct {
	ID          uint            `gorm:"primaryKey" json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Scope       string          `json:"scope"`
	DeviceType  string          `json:"device_type,omitempty"`
	Config      json.RawMessage `json:"config"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

func (DbConfigTemplate) TableName() string {
	return "config_templates"
}

// DbDeviceTag is the one model of the device_tags table. The database Manager
// migrates it; nothing else does.
type DbDeviceTag struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	DeviceID  uint      `gorm:"not null;index;constraint:OnDelete:CASCADE" json:"device_id"`
	Tag       string    `gorm:"size:191;not null;index" json:"tag"`
	CreatedAt time.Time `json:"created_at"`
}

func (DbDeviceTag) TableName() string {
	return "device_tags"
}

// DbDevice contains only the config-related fields we need
type DbDevice struct {
	ID            uint      `gorm:"primaryKey"`
	MAC           string    `gorm:"column:mac"`
	TemplateIDs   string    `gorm:"column:template_ids"`
	Overrides     string    `gorm:"column:overrides"`
	DesiredConfig string    `gorm:"column:desired_config"`
	ConfigApplied bool      `gorm:"column:config_applied"`
	PendingReboot bool      `gorm:"column:pending_reboot"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

func (DbDevice) TableName() string {
	return "devices"
}

// NewGormConfigRepository creates a new GORM-based config repository
func NewGormConfigRepository(db *gorm.DB, logger *logging.Logger) *GormConfigRepository {
	if logger == nil {
		logger = logging.GetDefault()
	}

	// No migration here: the database Manager creates config_templates and device_tags.
	return &GormConfigRepository{
		db:     db,
		logger: logger,
	}
}

// NewGormConfigRepositoryOn is NewGormConfigRepository for a handle that is already inside a
// transaction. It migrates nothing, because a schema change inside a transaction can commit it on
// some databases; the tables exist once the application has started.
func NewGormConfigRepositoryOn(tx *gorm.DB, logger *logging.Logger) *GormConfigRepository {
	if logger == nil {
		logger = logging.GetDefault()
	}
	return &GormConfigRepository{db: tx, logger: logger, inTx: true}
}

// InTransaction reports whether the repository works inside a transaction its caller owns, where a
// second read can see the same snapshot and miss again.
func (r *GormConfigRepository) InTransaction() bool { return r.inTx }

// Template CRUD

func (r *GormConfigRepository) CreateTemplate(template *ServiceConfigTemplate) error {
	dbTemplate := &DbConfigTemplate{
		Name:        template.Name,
		Description: template.Description,
		Scope:       template.Scope,
		DeviceType:  template.DeviceType,
		Config:      template.Config,
	}

	if err := r.db.Create(dbTemplate).Error; err != nil {
		return fmt.Errorf("failed to create template: %w", err)
	}

	template.ID = dbTemplate.ID
	template.CreatedAt = dbTemplate.CreatedAt
	template.UpdatedAt = dbTemplate.UpdatedAt
	return nil
}

func (r *GormConfigRepository) GetTemplate(id uint) (*ServiceConfigTemplate, error) {
	var dbTemplate DbConfigTemplate
	if err := r.db.First(&dbTemplate, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTemplateNotFound
		}
		return nil, fmt.Errorf("failed to get template: %w", err)
	}

	return dbTemplateToService(&dbTemplate), nil
}

func (r *GormConfigRepository) UpdateTemplate(template *ServiceConfigTemplate) error {
	dbTemplate := &DbConfigTemplate{
		ID:          template.ID,
		Name:        template.Name,
		Description: template.Description,
		Scope:       template.Scope,
		DeviceType:  template.DeviceType,
		Config:      template.Config,
	}

	result := r.db.Save(dbTemplate)
	if result.Error != nil {
		return fmt.Errorf("failed to update template: %w", result.Error)
	}

	template.UpdatedAt = dbTemplate.UpdatedAt
	return nil
}

func (r *GormConfigRepository) DeleteTemplate(id uint) error {
	result := r.db.Delete(&DbConfigTemplate{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete template: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrTemplateNotFound
	}
	return nil
}

func (r *GormConfigRepository) ListTemplates() ([]ServiceConfigTemplate, error) {
	var dbTemplates []DbConfigTemplate
	if err := r.db.Find(&dbTemplates).Error; err != nil {
		return nil, fmt.Errorf("failed to list templates: %w", err)
	}

	templates := make([]ServiceConfigTemplate, len(dbTemplates))
	for i, t := range dbTemplates {
		templates[i] = *dbTemplateToService(&t)
	}
	return templates, nil
}

func (r *GormConfigRepository) GetTemplatesByScope(scope string) ([]ServiceConfigTemplate, error) {
	var dbTemplates []DbConfigTemplate
	if err := r.db.Where("scope = ?", scope).Find(&dbTemplates).Error; err != nil {
		return nil, fmt.Errorf("failed to get templates by scope: %w", err)
	}

	templates := make([]ServiceConfigTemplate, len(dbTemplates))
	for i, t := range dbTemplates {
		templates[i] = *dbTemplateToService(&t)
	}
	return templates, nil
}

func (r *GormConfigRepository) GetTemplatesByDeviceType(deviceType string) ([]ServiceConfigTemplate, error) {
	var dbTemplates []DbConfigTemplate
	if err := r.db.Where("device_type = ?", deviceType).Find(&dbTemplates).Error; err != nil {
		return nil, fmt.Errorf("failed to get templates by device type: %w", err)
	}

	templates := make([]ServiceConfigTemplate, len(dbTemplates))
	for i, t := range dbTemplates {
		templates[i] = *dbTemplateToService(&t)
	}
	return templates, nil
}

// Device operations

func (r *GormConfigRepository) GetDevice(id uint) (*ServiceDevice, error) {
	var dbDevice DbDevice
	if err := r.db.First(&dbDevice, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDeviceNotFound
		}
		return nil, fmt.Errorf("failed to get device: %w", err)
	}

	return &ServiceDevice{
		ID:            dbDevice.ID,
		MAC:           dbDevice.MAC,
		TemplateIDs:   dbDevice.TemplateIDs,
		Overrides:     dbDevice.Overrides,
		DesiredConfig: dbDevice.DesiredConfig,
		ConfigApplied: dbDevice.ConfigApplied,
		PendingReboot: dbDevice.PendingReboot,
		UpdatedAt:     dbDevice.UpdatedAt,
	}, nil
}

func (r *GormConfigRepository) GetDevices() ([]ServiceDevice, error) {
	var dbDevices []DbDevice
	if err := r.db.Find(&dbDevices).Error; err != nil {
		return nil, fmt.Errorf("failed to get devices: %w", err)
	}

	devices := make([]ServiceDevice, len(dbDevices))
	for i, d := range dbDevices {
		devices[i] = dbDeviceToService(d)
	}
	return devices, nil
}

func (r *GormConfigRepository) UpdateDeviceTemplates(deviceID uint, templateIDs []uint) error {
	templateIDsJSON, err := json.Marshal(templateIDs)
	if err != nil {
		return fmt.Errorf("failed to marshal template IDs: %w", err)
	}

	if err := r.db.Model(&DbDevice{}).Where("id = ?", deviceID).Update("template_ids", string(templateIDsJSON)).Error; err != nil {
		return fmt.Errorf("failed to update device templates: %w", err)
	}
	return nil
}

func (r *GormConfigRepository) UpdateDeviceOverrides(deviceID uint, overrides json.RawMessage) error {
	if err := r.db.Model(&DbDevice{}).Where("id = ?", deviceID).Update("overrides", string(overrides)).Error; err != nil {
		return fmt.Errorf("failed to update device overrides: %w", err)
	}
	return nil
}

// SetDeviceDesiredConfigPending stores a new desired configuration and marks the device pending in
// one UPDATE. Two statements would leave a window in which the flag still reads applied against a
// changed configuration, after a failed second write or while an apply's compare-and-set
// (SetDeviceConfigAppliedIfDesired) lands between them. The map form writes config_applied even
// though false is its zero value.
//
// The write applies only while the device still holds what the recompute read (the template ids, the
// overrides and the desired text, compared byte for byte), otherwise nothing is written and
// ErrDesiredChanged says so: a rollback or an import that committed in between is not overwritten. The
// history row is written in the same transaction, when the UPDATE changed a row and the texts differ
// (ginsys/shelly-manager#775).
func (r *GormConfigRepository) SetDeviceDesiredConfigPending(deviceID uint, read DesiredRead, config json.RawMessage) error {
	text := string(config)
	return r.db.Transaction(func(tx *gorm.DB) error {
		dialect := tx.Name()
		updates := map[string]any{"desired_config": text, "config_applied": false}
		res := tx.Model(&DbDevice{}).
			Where(deviceReadCondition(dialect), deviceID, read.TemplateIDs, read.Overrides, read.Desired).
			Updates(updates)
		if res.Error != nil {
			return fmt.Errorf("failed to store device desired config as pending: %w", res.Error)
		}
		if res.RowsAffected > 0 {
			return RecordDesiredChange(tx, deviceID, "recompute", read.Desired, text)
		}

		// MySQL without clientFoundRows reports changed rows, so an UPDATE that changes no column
		// reports none. A row that already holds what was wanted (the read inputs, the new text, pending)
		// is done and is no miss.
		held, err := deviceRowExists(tx, deviceReadCondition(dialect)+" AND config_applied = ?", deviceID, read.TemplateIDs, read.Overrides, text, false)
		if err != nil {
			return fmt.Errorf("failed to store device desired config as pending: %w", err)
		}
		if held {
			return nil
		}

		// A device deleted since the read has nothing to write, as before, and no row to leave behind.
		exists, err := deviceRowExists(tx, "id = ?", deviceID)
		if err != nil {
			return fmt.Errorf("failed to store device desired config as pending: %w", err)
		}
		if !exists {
			return nil
		}
		return ErrDesiredChanged
	})
}

// GetDesiredHistory reads one version of a device's desired configuration. An id that is another
// device's or does not exist is ErrHistoryNotFound.
func (r *GormConfigRepository) GetDesiredHistory(deviceID, historyID uint) (*DesiredConfigHistory, error) {
	var row DesiredConfigHistory
	if err := r.db.Where("id = ? AND device_id = ?", historyID, deviceID).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrHistoryNotFound
		}
		return nil, fmt.Errorf("failed to read the desired configuration version: %w", err)
	}
	return &row, nil
}

// RollbackDeviceDesired is SetDeviceDesiredConfigPending for a rollback, which changes the template ids
// and the overrides as well as the desired text. It applies only while the device still holds read and
// writes the history row in the same transaction. On a miss the recount asks whether the device already
// holds what is being WRITTEN, not what was read: a rollback changes the read values, so recounting them
// would call its own committed write a miss. An unchanged row reports no rows on MySQL without
// clientFoundRows, and is done. A device deleted since the read is ErrDeviceNotFound, with no row.
func (r *GormConfigRepository) RollbackDeviceDesired(deviceID uint, read DesiredRead, write DesiredWrite, restoredFrom uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		dialect := tx.Name()
		res := tx.Model(&DbDevice{}).
			Where(deviceReadCondition(dialect), deviceID, read.TemplateIDs, read.Overrides, read.Desired).
			Updates(map[string]any{
				"template_ids": write.TemplateIDs, "overrides": write.Overrides,
				"desired_config": write.Desired, "config_applied": false,
			})
		if res.Error != nil {
			return fmt.Errorf("failed to store the rolled back desired config: %w", res.Error)
		}
		if res.RowsAffected > 0 {
			return recordRollback(tx, deviceID, restoredFrom, read.TemplateIDs, read.Desired, write.Desired)
		}

		held, err := deviceRowExists(tx, deviceReadCondition(dialect)+" AND config_applied = ?",
			deviceID, write.TemplateIDs, write.Overrides, write.Desired, false)
		if err != nil {
			return fmt.Errorf("failed to store the rolled back desired config: %w", err)
		}
		if held {
			return nil
		}
		exists, err := deviceRowExists(tx, "id = ?", deviceID)
		if err != nil {
			return fmt.Errorf("failed to store the rolled back desired config: %w", err)
		}
		if !exists {
			return ErrDeviceNotFound
		}
		return ErrDesiredChanged
	})
}

// deviceRowExists reports whether a device row matches, reading the latest committed state. FOR UPDATE
// does that: inside a REPEATABLE READ transaction (the MySQL default, and an import runs in one) a plain
// read keeps the snapshot of the transaction's first read, and the UPDATE above tests the current row, so
// the two would disagree. It selects the id and never counts, as PostgreSQL refuses FOR UPDATE with an
// aggregate. SQLite has no row locks and ignores the clause.
func deviceRowExists(tx *gorm.DB, where string, args ...any) (bool, error) {
	var ids []uint
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Model(&DbDevice{}).Where(where, args...).Limit(1).Pluck("id", &ids).Error
	return len(ids) > 0, err
}

// deviceReadCondition is the WHERE clause of SetDeviceDesiredConfigPending: the device, and the three
// columns a recompute reads, byte-exact (see desiredConfigMatchCondition).
func deviceReadCondition(dialect string) string {
	equal := func(column string) string {
		if dialect == "mysql" {
			return "BINARY COALESCE(" + column + ", '') = ?"
		}
		return "COALESCE(" + column + ", '') = ?"
	}
	return "id = ? AND " + equal("template_ids") + " AND " + equal("overrides") + " AND " + equal("desired_config")
}

func (r *GormConfigRepository) SetDeviceConfigApplied(deviceID uint, applied bool) error {
	if err := r.db.Model(&DbDevice{}).Where("id = ?", deviceID).Update("config_applied", applied).Error; err != nil {
		return fmt.Errorf("failed to set device config applied: %w", err)
	}
	return nil
}

// SetDevicePendingReboot is one UPDATE of the pending_reboot column. The database.Device model
// marks that column read-only, so a Save of a device row read before this write cannot undo it.
func (r *GormConfigRepository) SetDevicePendingReboot(deviceID uint, pending bool) error {
	if err := r.db.Model(&DbDevice{}).Where("id = ?", deviceID).Update("pending_reboot", pending).Error; err != nil {
		return fmt.Errorf("failed to set device pending reboot: %w", err)
	}
	return nil
}

// SetDeviceConfigAppliedIfDesired marks the device applied only while its stored desired
// configuration still equals desiredConfig, in one UPDATE. It reports whether the flag was set.
// A template or override edit that landed during an apply changes desired_config, so the
// newer configuration stays pending.
func (r *GormConfigRepository) SetDeviceConfigAppliedIfDesired(deviceID uint, desiredConfig string) (bool, error) {
	res := r.db.Model(&DbDevice{}).
		Where(desiredConfigMatchCondition(r.db.Dialector.Name()), deviceID, desiredConfig).
		Update("config_applied", true)
	if res.Error != nil {
		return false, fmt.Errorf("failed to set device config applied: %w", res.Error)
	}
	return res.RowsAffected > 0, nil
}

// desiredConfigMatchCondition is the WHERE clause of SetDeviceConfigAppliedIfDesired. The
// comparison must be byte-exact: MySQL compares text with the column collation, which is often
// case-insensitive, so a desired name changed from "office" to "Office" during an apply would
// still match. SQLite and PostgreSQL compare text exactly by default.
func desiredConfigMatchCondition(dialect string) string {
	if dialect == "mysql" {
		return "id = ? AND BINARY COALESCE(desired_config, '') = ?"
	}
	return "id = ? AND COALESCE(desired_config, '') = ?"
}

// Tag operations

func (r *GormConfigRepository) AddDeviceTag(deviceID uint, tag string) error {
	deviceTag := &DbDeviceTag{
		DeviceID: deviceID,
		Tag:      tag,
	}

	if err := r.db.Where("device_id = ? AND tag = ?", deviceID, tag).FirstOrCreate(deviceTag).Error; err != nil {
		return fmt.Errorf("failed to add device tag: %w", err)
	}
	return nil
}

func (r *GormConfigRepository) RemoveDeviceTag(deviceID uint, tag string) error {
	if err := r.db.Where("device_id = ? AND tag = ?", deviceID, tag).Delete(&DbDeviceTag{}).Error; err != nil {
		return fmt.Errorf("failed to remove device tag: %w", err)
	}
	return nil
}

func (r *GormConfigRepository) GetDeviceTags(deviceID uint) ([]string, error) {
	var tags []DbDeviceTag
	if err := r.db.Where("device_id = ?", deviceID).Find(&tags).Error; err != nil {
		return nil, fmt.Errorf("failed to get device tags: %w", err)
	}

	tagStrings := make([]string, len(tags))
	for i, t := range tags {
		tagStrings[i] = t.Tag
	}
	return tagStrings, nil
}

func (r *GormConfigRepository) GetDevicesByTag(tag string) ([]ServiceDevice, error) {
	var deviceTags []DbDeviceTag
	if err := r.db.Where("tag = ?", tag).Find(&deviceTags).Error; err != nil {
		return nil, fmt.Errorf("failed to get devices by tag: %w", err)
	}

	if len(deviceTags) == 0 {
		return []ServiceDevice{}, nil
	}

	deviceIDs := make([]uint, len(deviceTags))
	for i, dt := range deviceTags {
		deviceIDs[i] = dt.DeviceID
	}

	var dbDevices []DbDevice
	if err := r.db.Where("id IN ?", deviceIDs).Find(&dbDevices).Error; err != nil {
		return nil, fmt.Errorf("failed to get devices: %w", err)
	}

	devices := make([]ServiceDevice, len(dbDevices))
	for i, d := range dbDevices {
		devices[i] = dbDeviceToService(d)
	}
	return devices, nil
}

func (r *GormConfigRepository) ListAllTags() ([]string, error) {
	var tags []string
	if err := r.db.Model(&DbDeviceTag{}).Distinct("tag").Pluck("tag", &tags).Error; err != nil {
		return nil, fmt.Errorf("failed to list all tags: %w", err)
	}
	return tags, nil
}

// Helper functions

func dbTemplateToService(t *DbConfigTemplate) *ServiceConfigTemplate {
	return &ServiceConfigTemplate{
		ID:          t.ID,
		Name:        t.Name,
		Description: t.Description,
		Scope:       t.Scope,
		DeviceType:  t.DeviceType,
		Config:      t.Config,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}

func dbDeviceToService(d DbDevice) ServiceDevice {
	return ServiceDevice{ //nolint:staticcheck
		ID:            d.ID,
		MAC:           d.MAC,
		TemplateIDs:   d.TemplateIDs,
		Overrides:     d.Overrides,
		DesiredConfig: d.DesiredConfig,
		ConfigApplied: d.ConfigApplied,
		PendingReboot: d.PendingReboot,
		UpdatedAt:     d.UpdatedAt,
	}
}
