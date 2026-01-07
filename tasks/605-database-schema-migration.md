# Database Schema Migration

**Priority**: HIGH
**Status**: completed
**Effort**: 4 hours
**Completed**: 2026-01-06
**Depends On**: 601

## Context

Add new database tables and columns to support the redesigned configuration system:
- Templates table for storing reusable partial configs
- Device tags for group-based template assignment
- New device columns for template assignment, overrides, and desired config

## Current State

The existing `device_configs` table stores `imported_config` (raw API JSON). This will be kept as-is for the "last known device state" snapshot.

## New Schema

### config_templates Table

```sql
CREATE TABLE config_templates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    scope TEXT NOT NULL CHECK (scope IN ('global', 'group', 'device_type')),
    
    -- For scope='device_type': the model this template is intended for (e.g., 'SHPLG-S').
    device_type TEXT,
    
    -- Store JSON as TEXT for SQLite portability.
    config TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Scope constraints
    CHECK (scope != 'device_type' OR device_type IS NOT NULL)
);

-- Index for filtering by scope
CREATE INDEX idx_config_templates_scope ON config_templates(scope);
-- Index for device_type lookup
CREATE INDEX idx_config_templates_device_type ON config_templates(device_type);
```

### device_tags Table

```sql
CREATE TABLE device_tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    device_id INTEGER NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    tag TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(device_id, tag)
);

-- Index for finding devices by tag
CREATE INDEX idx_device_tags_tag ON device_tags(tag);
-- Index for finding tags by device
CREATE INDEX idx_device_tags_device_id ON device_tags(device_id);
```

### devices Table Additions

```sql
-- New columns for devices table
ALTER TABLE devices ADD COLUMN template_ids TEXT DEFAULT '[]';
-- Ordered list of template IDs (manual assignment): [tmpl1, tmpl2, ...]

ALTER TABLE devices ADD COLUMN overrides TEXT DEFAULT '{}';
-- DeviceConfiguration in internal format (device-specific values)

ALTER TABLE devices ADD COLUMN desired_config TEXT DEFAULT '{}';
-- DeviceConfiguration in internal format (computed from templates + overrides)

ALTER TABLE devices ADD COLUMN config_applied BOOLEAN DEFAULT FALSE;
-- Whether desired_config has been successfully applied to the device
```

## GORM Models

```go
// ConfigTemplate represents a reusable configuration template
type ConfigTemplate struct {
    ID          uint      `gorm:"primaryKey" json:"id"`
    Name        string    `gorm:"uniqueIndex;not null" json:"name"`
    Description string    `json:"description,omitempty"`
    Scope       string    `gorm:"not null" json:"scope"` // "global", "group", "device_type"

    // For scope="device_type": device model (e.g., "SHPLG-S").
    DeviceType string `json:"device_type,omitempty"`

    // Use JSON stored as TEXT in SQLite; in Go prefer json.RawMessage or datatypes.JSON.
    Config json.RawMessage `gorm:"type:text;not null" json:"config"`

    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

// DeviceTag represents a tag assigned to a device for group templates
type DeviceTag struct {
    ID        uint      `gorm:"primaryKey" json:"id"`
    DeviceID  uint      `gorm:"not null;index" json:"device_id"`
    Tag       string    `gorm:"not null;index" json:"tag"`
    CreatedAt time.Time `json:"created_at"`
    
    // Unique constraint on (device_id, tag)
    // Defined via GORM index
}

func (DeviceTag) TableName() string {
    return "device_tags"
}

// Add to existing Device model
type Device struct {
    // ... existing fields ...

    // Store JSON as TEXT for SQLite portability.
    // In Go prefer json.RawMessage or datatypes.JSON.
    TemplateIDs   json.RawMessage `gorm:"type:text;default:'[]'" json:"template_ids"`
    Overrides     json.RawMessage `gorm:"type:text;default:'{}'" json:"overrides"`
    DesiredConfig json.RawMessage `gorm:"type:text;default:'{}'" json:"desired_config"`
    ConfigApplied bool            `gorm:"default:false" json:"config_applied"`
}
```

## Success Criteria

- [ ] Create `ConfigTemplate` GORM model
- [ ] Create `DeviceTag` GORM model
- [ ] Add new columns to `Device` model
- [ ] GORM auto-migration runs on startup
- [ ] Existing devices get sensible defaults (empty template_ids, empty overrides)
- [ ] Unit tests for new model CRUD operations
- [ ] Test unique constraints work correctly

## Migration Strategy

GORM's AutoMigrate will handle adding new tables and columns. Existing data is preserved:
- Existing devices keep their current fields
- New columns get default values (empty JSON, false boolean)
- No data migration needed - templates start empty

## Files to Create/Modify

- `internal/database/models.go` (modify - add Device fields)
- `internal/database/models_config.go` (NEW - ConfigTemplate, DeviceTag models)
- `internal/database/manager.go` (modify - register new models for migration)
- `internal/database/manager_config.go` (NEW - CRUD methods for templates/tags)
- `internal/database/manager_config_test.go` (NEW - tests)

## Validation

```bash
make test-ci
go test -v ./internal/database/...

# Verify migration works on fresh database
rm -f data/test.db
go test -v ./internal/database/... -run TestMigration
```

## Notes

The schema is designed to be backward compatible. Existing functionality continues to work while new features are added incrementally.
