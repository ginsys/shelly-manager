package configuration

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// GormConfigRepository implements ConfigRepository using GORM
type GormConfigRepository struct {
	db     *gorm.DB
	logger *logging.Logger
}

// DbConfigTemplate is the database model for config templates
// This mirrors database.ConfigTemplate but is defined here to avoid import cycles
type DbConfigTemplate struct {
	ID          uint            `gorm:"primaryKey" json:"id"`
	Name        string          `gorm:"uniqueIndex;not null" json:"name"`
	Description string          `json:"description,omitempty"`
	Scope       string          `gorm:"not null;index" json:"scope"`
	DeviceType  string          `gorm:"index" json:"device_type,omitempty"`
	Config      json.RawMessage `gorm:"type:text;not null" json:"config"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

func (DbConfigTemplate) TableName() string {
	return "config_templates"
}

// DbDeviceTag is the database model for device tags
type DbDeviceTag struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	DeviceID  uint      `gorm:"not null;index;constraint:OnDelete:CASCADE" json:"device_id"`
	Tag       string    `gorm:"not null;index" json:"tag"`
	CreatedAt time.Time `json:"created_at"`
}

func (DbDeviceTag) TableName() string {
	return "device_tags"
}

// DbDevice contains only the config-related fields we need
type DbDevice struct {
	ID            uint      `gorm:"primaryKey"`
	TemplateIDs   string    `gorm:"column:template_ids"`
	Overrides     string    `gorm:"column:overrides"`
	DesiredConfig string    `gorm:"column:desired_config"`
	ConfigApplied bool      `gorm:"column:config_applied"`
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

	// Auto-migrate tables
	if err := db.AutoMigrate(&DbConfigTemplate{}, &DbDeviceTag{}); err != nil {
		logger.Error("Failed to auto-migrate config tables", "error", err)
	}

	return &GormConfigRepository{
		db:     db,
		logger: logger,
	}
}

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
		TemplateIDs:   dbDevice.TemplateIDs,
		Overrides:     dbDevice.Overrides,
		DesiredConfig: dbDevice.DesiredConfig,
		ConfigApplied: dbDevice.ConfigApplied,
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

func (r *GormConfigRepository) UpdateDeviceDesiredConfig(deviceID uint, config json.RawMessage) error {
	if err := r.db.Model(&DbDevice{}).Where("id = ?", deviceID).Update("desired_config", string(config)).Error; err != nil {
		return fmt.Errorf("failed to update device desired config: %w", err)
	}
	return nil
}

func (r *GormConfigRepository) SetDeviceConfigApplied(deviceID uint, applied bool) error {
	if err := r.db.Model(&DbDevice{}).Where("id = ?", deviceID).Update("config_applied", applied).Error; err != nil {
		return fmt.Errorf("failed to set device config applied: %w", err)
	}
	return nil
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
		TemplateIDs:   d.TemplateIDs,
		Overrides:     d.Overrides,
		DesiredConfig: d.DesiredConfig,
		ConfigApplied: d.ConfigApplied,
		UpdatedAt:     d.UpdatedAt,
	}
}
