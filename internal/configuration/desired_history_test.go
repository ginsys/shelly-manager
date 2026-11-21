package configuration

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// historyDB is a SQLite database with the two tables a desired write touches. A single connection keeps
// the in-memory database one database.
func historyDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := bareDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&DbDevice{}, &DesiredConfigHistory{}))
	return db
}

func mqttOverride(server, password string) string {
	return fmt.Sprintf(`{"mqtt":{"server":%q,"pass":%q}}`, server, password)
}

func addHistoryDevice(t *testing.T, db *gorm.DB, id uint, desired, overrides string) {
	t.Helper()
	require.NoError(t, db.Create(&DbDevice{
		ID: id, MAC: fmt.Sprintf("AA:BB:CC:00:00:%02X", id), TemplateIDs: "[]", Overrides: overrides, DesiredConfig: desired,
	}).Error)
}

func historyRows(t *testing.T, db *gorm.DB, deviceID uint) []DesiredConfigHistory {
	t.Helper()
	var rows []DesiredConfigHistory
	require.NoError(t, db.Where("device_id = ?", deviceID).Order("id").Find(&rows).Error)
	return rows
}

func setOverrides(t *testing.T, db *gorm.DB, id uint, overrides string) {
	t.Helper()
	require.NoError(t, db.Model(&DbDevice{}).Where("id = ?", id).Update("overrides", overrides).Error)
}

func deviceRow(t *testing.T, db *gorm.DB, id uint) DbDevice {
	t.Helper()
	var device DbDevice
	require.NoError(t, db.First(&device, id).Error)
	return device
}

func TestRecomputeDesiredConfig_HistoryRowPerRealChange(t *testing.T) {
	db := historyDB(t)
	addHistoryDevice(t, db, 1, "{}", mqttOverride("a.example", ""))
	svc := NewConfigurationService(NewGormConfigRepository(db, nil), Engine{}, nil)

	require.NoError(t, svc.RecomputeDesiredConfig(1))
	first := deviceRow(t, db, 1).DesiredConfig
	rows := historyRows(t, db, 1)
	require.Len(t, rows, 1)
	assert.Equal(t, "recompute", rows[0].Action)
	assert.Equal(t, "system", rows[0].ChangedBy)
	require.NotNil(t, rows[0].OldDesired)
	assert.JSONEq(t, "{}", *rows[0].OldDesired)
	require.NotNil(t, rows[0].NewDesired)
	assert.JSONEq(t, first, *rows[0].NewDesired)
	assert.False(t, rows[0].CreatedAt.IsZero())

	setOverrides(t, db, 1, mqttOverride("b.example", ""))
	require.NoError(t, svc.RecomputeDesiredConfig(1))
	rows = historyRows(t, db, 1)
	require.Len(t, rows, 2)
	assert.JSONEq(t, first, *rows[1].OldDesired, "the old side is the text the write replaced")
	assert.JSONEq(t, deviceRow(t, db, 1).DesiredConfig, *rows[1].NewDesired)
}

func TestRecomputeDesiredConfig_HistoryNoRowWhenNothingChanged(t *testing.T) {
	db := historyDB(t)
	addHistoryDevice(t, db, 1, "{}", mqttOverride("a.example", ""))
	svc := NewConfigurationService(NewGormConfigRepository(db, nil), Engine{}, nil)
	require.NoError(t, svc.RecomputeDesiredConfig(1))
	require.Len(t, historyRows(t, db, 1), 1)

	// The same text again: no row, and the device stays pending.
	require.NoError(t, svc.RecomputeDesiredConfig(1))
	assert.Len(t, historyRows(t, db, 1), 1)
	assert.False(t, deviceRow(t, db, 1).ConfigApplied)

	// The same text on an applied device: the flag is cleared as before, and still no row.
	require.NoError(t, db.Model(&DbDevice{}).Where("id = ?", 1).Update("config_applied", true).Error)
	require.NoError(t, svc.RecomputeDesiredConfig(1))
	assert.Len(t, historyRows(t, db, 1), 1)
	assert.False(t, deviceRow(t, db, 1).ConfigApplied, "a recompute still marks the device pending")
}

func TestRecomputeDesiredConfig_HistoryMasksSecrets(t *testing.T) {
	db := historyDB(t)
	addHistoryDevice(t, db, 1, "{}", mqttOverride("a.example", "canary-one"))
	svc := NewConfigurationService(NewGormConfigRepository(db, nil), Engine{}, nil)
	require.NoError(t, svc.RecomputeDesiredConfig(1))

	// A change to the secret alone is a real change: one row, whose two masked sides are equal.
	setOverrides(t, db, 1, mqttOverride("a.example", "canary-two"))
	require.NoError(t, svc.RecomputeDesiredConfig(1))
	assert.Contains(t, deviceRow(t, db, 1).DesiredConfig, "canary-two", "the real desired text keeps the secret")

	rows := historyRows(t, db, 1)
	require.Len(t, rows, 2)
	require.NotNil(t, rows[1].OldDesired)
	require.NotNil(t, rows[1].NewDesired)
	assert.Equal(t, *rows[1].OldDesired, *rows[1].NewDesired)

	for _, row := range rows {
		for _, text := range []*string{row.OldDesired, row.NewDesired} {
			if text != nil {
				assert.NotContains(t, *text, "canary", "a history text holds no secret")
			}
		}
	}
}

// A Gen1 action's URLs can hold a password or a token, so no history row holds one, on either side.
func TestRecomputeDesiredConfig_HistoryMasksActionURLs(t *testing.T) {
	actionOverride := func(url string) string {
		return `{"actions":[{"name":"btn_on_url","index":0,"secret_urls":["` + url + `"]}]}`
	}
	db := historyDB(t)
	addHistoryDevice(t, db, 1, "{}", actionOverride("http://a:canary-one@h/1"))
	svc := NewConfigurationService(NewGormConfigRepository(db, nil), Engine{}, nil)
	require.NoError(t, svc.RecomputeDesiredConfig(1))
	assert.Contains(t, deviceRow(t, db, 1).DesiredConfig, "canary-one", "the real desired text keeps the URL")

	setOverrides(t, db, 1, actionOverride("http://a:canary-two@h/1"))
	require.NoError(t, svc.RecomputeDesiredConfig(1))
	assert.Contains(t, deviceRow(t, db, 1).DesiredConfig, "canary-two")

	rows := historyRows(t, db, 1)
	require.Len(t, rows, 2)
	for _, row := range rows {
		for _, text := range []*string{row.OldDesired, row.NewDesired} {
			if text != nil {
				assert.NotContains(t, *text, "canary", "a history text holds an action URL")
			}
		}
	}
	require.NotNil(t, rows[1].NewDesired)
	assert.Contains(t, *rows[1].NewDesired, "btn_on_url", "the action itself is kept, only its URLs are masked")
}

// A text that cannot be masked is never stored raw, and the write that succeeds today still succeeds.
func TestRecomputeDesiredConfig_HistoryUnmaskableOldSideIsNull(t *testing.T) {
	db := historyDB(t)
	addHistoryDevice(t, db, 1, "token=canary-raw not json", mqttOverride("a.example", ""))
	svc := NewConfigurationService(NewGormConfigRepository(db, nil), Engine{}, nil)

	require.NoError(t, svc.RecomputeDesiredConfig(1))
	rows := historyRows(t, db, 1)
	require.Len(t, rows, 1)
	assert.Nil(t, rows[0].OldDesired)
	require.NotNil(t, rows[0].NewDesired)
	assert.NotContains(t, *rows[0].NewDesired, "canary-raw")
}

func TestSetDeviceDesiredConfigPending_StaleReadWritesNothing(t *testing.T) {
	db := historyDB(t)
	addHistoryDevice(t, db, 1, `{"a":1}`, "{}")
	repo := NewGormConfigRepository(db, nil)
	read := DesiredRead{TemplateIDs: "[]", Overrides: "{}", Desired: `{"a":1}`}

	for name, stale := range map[string]DesiredRead{
		"overrides":     {TemplateIDs: "[]", Overrides: `{"x":1}`, Desired: `{"a":1}`},
		"template ids":  {TemplateIDs: "[1]", Overrides: "{}", Desired: `{"a":1}`},
		"desired":       {TemplateIDs: "[]", Overrides: "{}", Desired: `{"a":0}`},
		"desired case":  {TemplateIDs: "[]", Overrides: "{}", Desired: `{"A":1}`},
		"empty vs null": {TemplateIDs: "", Overrides: "{}", Desired: `{"a":1}`},
	} {
		t.Run(name, func(t *testing.T) {
			err := repo.SetDeviceDesiredConfigPending(1, stale, json.RawMessage(`{"a":2}`))
			assert.ErrorIs(t, err, ErrDesiredChanged)
			assert.Equal(t, `{"a":1}`, deviceRow(t, db, 1).DesiredConfig)
			assert.Empty(t, historyRows(t, db, 1))
		})
	}

	require.NoError(t, repo.SetDeviceDesiredConfigPending(1, read, json.RawMessage(`{"a":2}`)))
	assert.Equal(t, `{"a":2}`, deviceRow(t, db, 1).DesiredConfig)
	assert.Len(t, historyRows(t, db, 1), 1)
}

// The history row is part of the desired write: a row that cannot be written fails the write.
func TestSetDeviceDesiredConfigPending_RowAndWriteAreOneTransaction(t *testing.T) {
	db := historyDB(t)
	addHistoryDevice(t, db, 1, `{"a":1}`, "{}")
	require.NoError(t, db.Migrator().DropTable(&DesiredConfigHistory{}))
	repo := NewGormConfigRepository(db, nil)

	err := repo.SetDeviceDesiredConfigPending(1, DesiredRead{TemplateIDs: "[]", Overrides: "{}", Desired: `{"a":1}`}, json.RawMessage(`{"a":2}`))
	require.Error(t, err)
	assert.Equal(t, `{"a":1}`, deviceRow(t, db, 1).DesiredConfig, "the desired write rolled back with the row")
}

// MySQL without clientFoundRows counts changed rows, so an UPDATE that changes no column reports 0. The
// callback gives SQLite that count: the case that follows is the MySQL one, run here.
func zeroRowsAffected(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Callback().Update().After("gorm:update").Register("test:mysql_changed_rows", func(tx *gorm.DB) {
		if tx.Statement.Table == "devices" {
			tx.RowsAffected = 0
		}
	}))
}

func TestSetDeviceDesiredConfigPending_UnchangedRowIsNotAMiss(t *testing.T) {
	db := historyDB(t)
	addHistoryDevice(t, db, 1, `{"a":1}`, "{}")
	require.NoError(t, db.Model(&DbDevice{}).Where("id = ?", 1).Update("config_applied", false).Error)
	zeroRowsAffected(t, db)
	repo := NewGormConfigRepository(db, nil)
	read := DesiredRead{TemplateIDs: "[]", Overrides: "{}", Desired: `{"a":1}`}

	require.NoError(t, repo.SetDeviceDesiredConfigPending(1, read, json.RawMessage(`{"a":1}`)),
		"a pending device with the same text holds what was wanted")
	assert.Empty(t, historyRows(t, db, 1))

	// A real miss still reads as a miss when the count is zero.
	stale := DesiredRead{TemplateIDs: "[]", Overrides: `{"x":1}`, Desired: `{"a":1}`}
	assert.ErrorIs(t, repo.SetDeviceDesiredConfigPending(1, stale, json.RawMessage(`{"a":1}`)), ErrDesiredChanged)
}

// A device deleted after the read is not a miss and leaves no row.
func TestSetDeviceDesiredConfigPending_DeletedDeviceIsSuccessWithoutRow(t *testing.T) {
	db := historyDB(t)
	zeroRowsAffected(t, db)
	repo := NewGormConfigRepository(db, nil)

	err := repo.SetDeviceDesiredConfigPending(7, DesiredRead{TemplateIDs: "[]", Overrides: "{}", Desired: `{"a":1}`}, json.RawMessage(`{"a":2}`))
	require.NoError(t, err)
	assert.Empty(t, historyRows(t, db, 7))
}

// deletingRepository removes the device after the recompute read it.
type deletingRepository struct {
	*GormConfigRepository
	db    *gorm.DB
	reads int
}

// The recompute reads the device twice (the device, then its templates), so the delete lands after both.
func (r *deletingRepository) GetDevice(id uint) (*ServiceDevice, error) {
	device, err := r.GormConfigRepository.GetDevice(id)
	if r.reads++; err == nil && r.reads == 2 {
		r.db.Delete(&DbDevice{}, id)
	}
	return device, err
}

func TestRecomputeDesiredConfig_DeviceDeletedMeanwhileLeavesNoRow(t *testing.T) {
	db := historyDB(t)
	addHistoryDevice(t, db, 1, "{}", mqttOverride("a.example", ""))
	svc := NewConfigurationService(&deletingRepository{GormConfigRepository: NewGormConfigRepository(db, nil), db: db}, Engine{}, nil)

	require.NoError(t, svc.RecomputeDesiredConfig(1))
	assert.Empty(t, historyRows(t, db, 1), "a row for a device that is gone would be an orphan")
}

func TestRecordDesiredChange_KeepsTheNewest50(t *testing.T) {
	db := historyDB(t)
	for i := 0; i < 51; i++ {
		require.NoError(t, RecordDesiredChange(db, 1, "recompute", fmt.Sprintf(`{"v":%d}`, i), fmt.Sprintf(`{"v":%d}`, i+1)))
	}
	require.NoError(t, RecordDesiredChange(db, 2, "recompute", `{"v":0}`, `{"v":1}`))

	rows := historyRows(t, db, 1)
	require.Len(t, rows, 50)
	assert.JSONEq(t, `{"v":2}`, *rows[0].NewDesired, "the oldest row went")
	assert.JSONEq(t, `{"v":51}`, *rows[49].NewDesired, "the newest stayed")
	assert.Len(t, historyRows(t, db, 2), 1, "another device's rows are not touched")
}

func TestRecordDesiredChange_EmptySidesAndEqualTexts(t *testing.T) {
	db := historyDB(t)

	require.NoError(t, RecordDesiredChange(db, 1, "import", "", `{"a":1}`))
	require.NoError(t, RecordDesiredChange(db, 1, "import", `{"a":1}`, `{"a":1}`))
	require.NoError(t, RecordDesiredChange(db, 1, "import", `{"a":1}`, "not json"))

	rows := historyRows(t, db, 1)
	require.Len(t, rows, 2, "equal texts write no row")
	assert.Nil(t, rows[0].OldDesired, "an empty side is NULL")
	assert.NotNil(t, rows[0].NewDesired)
	assert.NotNil(t, rows[1].OldDesired)
	assert.Nil(t, rows[1].NewDesired, "an unmaskable side is NULL, never raw")
}

// Service-level retry, with a repository that reports a miss: the count of write attempts is the proof.
type missingRepository struct {
	*mockRepository
	misses int
	writes int
	inTx   bool
}

func (r *missingRepository) InTransaction() bool { return r.inTx }

func (r *missingRepository) SetDeviceDesiredConfigPending(id uint, read DesiredRead, config json.RawMessage) error {
	r.writes++
	if r.writes <= r.misses {
		return ErrDesiredChanged
	}
	return r.mockRepository.SetDeviceDesiredConfigPending(id, read, config)
}

func newMissingRepository(misses int, inTx bool) (*missingRepository, *ConfigurationService) {
	repo := &missingRepository{mockRepository: newMockRepository(), misses: misses, inTx: inTx}
	repo.addDevice(1)
	return repo, NewConfigurationService(repo, Engine{}, nil)
}

func TestRecomputeDesiredConfig_MissRetriesThenWrites(t *testing.T) {
	repo, svc := newMissingRepository(1, false)
	require.NoError(t, svc.RecomputeDesiredConfig(1))
	assert.Equal(t, 2, repo.writes)
}

func TestRecomputeDesiredConfig_MissIsBoundedOutsideATransaction(t *testing.T) {
	repo, svc := newMissingRepository(100, false)
	err := svc.RecomputeDesiredConfig(1)
	assert.ErrorIs(t, err, ErrDesiredChanged)
	assert.Equal(t, 3, repo.writes)
}

func TestRecomputeDesiredConfig_MissInsideATransactionReturnsItsErrorAndDoesNotLoop(t *testing.T) {
	repo, svc := newMissingRepository(100, true)
	err := svc.RecomputeDesiredConfig(1)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrDesiredChanged))
	assert.Equal(t, 1, repo.writes, "a second read inside a transaction can see the same snapshot")
}

// Only a repository built on a caller's transaction says so: the recompute's retry reads it.
func TestGormConfigRepository_InTransaction(t *testing.T) {
	db := historyDB(t)
	assert.False(t, NewGormConfigRepository(db, nil).InTransaction())
	assert.True(t, NewGormConfigRepositoryOn(db, nil).InTransaction())
}
