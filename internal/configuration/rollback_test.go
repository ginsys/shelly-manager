package configuration

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	secrets "github.com/ginsys/shelly-manager/internal/redact"
)

func rollbackService(db *gorm.DB) *ConfigurationService {
	return NewConfigurationService(NewGormConfigRepository(db, nil), Engine{}, nil)
}

// addVersion stores one history row of the device holding text as its new side, and returns its id.
func addVersion(t *testing.T, db *gorm.DB, deviceID uint, text *string) uint {
	t.Helper()
	row := DesiredConfigHistory{DeviceID: deviceID, Action: "recompute", NewDesired: text, ChangedBy: "system"}
	require.NoError(t, db.Create(&row).Error)
	return row.ID
}

func str(s string) *string { return &s }

func rollbackDevice(t *testing.T, db *gorm.DB, id uint, templates, overrides, desired string) {
	t.Helper()
	addHistoryDevice(t, db, id, desired, overrides)
	require.NoError(t, db.Model(&DbDevice{}).Where("id = ?", id).
		Updates(map[string]any{"template_ids": templates, "config_applied": true}).Error)
}

func requireRollbackError(t *testing.T, err error, kind RollbackKind) *RollbackError {
	t.Helper()
	var rerr *RollbackError
	require.True(t, errors.As(err, &rerr), "want a RollbackError, got %v", err)
	require.Equal(t, kind, rerr.Kind)
	return rerr
}

func TestRollbackDesired_ReplacesAndDetachesTemplates(t *testing.T) {
	db := historyDB(t)
	rollbackDevice(t, db, 1, "[7]", `{"wifi":{"ssid":"x"}}`, `{"mqtt":{"server":"a.example"},"wifi":{"ssid":"x"}}`)
	version := addVersion(t, db, 1, str(`{"mqtt":{"server":"b.example"}}`))

	res, err := rollbackService(db).RollbackDesired(1, version, nil)
	require.NoError(t, err)
	assert.False(t, res.NoChange)
	assert.Equal(t, []uint{7}, res.DetachedTemplateIDs)

	device := deviceRow(t, db, 1)
	assert.JSONEq(t, `{"mqtt":{"server":"b.example"}}`, device.DesiredConfig, "a key the version lacks is gone")
	assert.JSONEq(t, `{"mqtt":{"server":"b.example"}}`, device.Overrides)
	assert.Equal(t, "[]", device.TemplateIDs, "the templates are detached")
	assert.False(t, device.ConfigApplied, "the device is pending until an apply")
}

func TestRollbackDesired_SurvivesARecompute(t *testing.T) {
	db := historyDB(t)
	rollbackDevice(t, db, 1, "[7]", `{"wifi":{"ssid":"x"}}`, `{"mqtt":{"server":"a.example"},"wifi":{"ssid":"x"}}`)
	version := addVersion(t, db, 1, str(`{"mqtt":{"server":"b.example"}}`))
	svc := rollbackService(db)
	_, err := svc.RollbackDesired(1, version, nil)
	require.NoError(t, err)

	require.NoError(t, svc.RecomputeDesiredConfig(1))
	assert.JSONEq(t, `{"mqtt":{"server":"b.example"}}`, deviceRow(t, db, 1).DesiredConfig)
}

func TestRollbackDesired_WritesTheRollbackRow(t *testing.T) {
	db := historyDB(t)
	rollbackDevice(t, db, 1, "[7]", `{}`, `{"mqtt":{"server":"a.example"}}`)
	version := addVersion(t, db, 1, str(`{"mqtt":{"server":"b.example"}}`))
	_, err := rollbackService(db).RollbackDesired(1, version, nil)
	require.NoError(t, err)

	rows := historyRows(t, db, 1)
	require.Len(t, rows, 2, "the version and the rollback")
	row := rows[1]
	assert.Equal(t, "rollback", row.Action)
	assert.Equal(t, "admin", row.ChangedBy)
	require.NotNil(t, row.RestoredFrom)
	assert.Equal(t, version, *row.RestoredFrom)
	require.NotNil(t, row.PreviousTemplateIDs)
	assert.Equal(t, "[7]", *row.PreviousTemplateIDs)
	require.NotNil(t, row.OldDesired)
	assert.JSONEq(t, `{"mqtt":{"server":"a.example"}}`, *row.OldDesired)
	require.NotNil(t, row.NewDesired)
	assert.JSONEq(t, `{"mqtt":{"server":"b.example"}}`, *row.NewDesired)
}

func TestRollbackDesired_AVersionIsNoSourceWhenNullOrAnotherDevices(t *testing.T) {
	db := historyDB(t)
	rollbackDevice(t, db, 1, "[]", `{}`, `{"mqtt":{"server":"a.example"}}`)
	rollbackDevice(t, db, 2, "[]", `{}`, `{}`)
	svc := rollbackService(db)

	null := addVersion(t, db, 1, nil)
	_, err := svc.RollbackDesired(1, null, nil)
	assert.ErrorIs(t, err, ErrVersionNotRestorable)

	other := addVersion(t, db, 2, str(`{"mqtt":{"server":"z.example"}}`))
	_, err = svc.RollbackDesired(1, other, nil)
	assert.ErrorIs(t, err, ErrHistoryNotFound, "another device's version")
	_, err = svc.RollbackDesired(1, 9999, nil)
	assert.ErrorIs(t, err, ErrHistoryNotFound)
	_, err = svc.RollbackDesired(77, null, nil)
	assert.ErrorIs(t, err, ErrDeviceNotFound)

	assert.JSONEq(t, `{"mqtt":{"server":"a.example"}}`, deviceRow(t, db, 1).DesiredConfig)
	assert.Len(t, historyRows(t, db, 1), 1, "no rollback row")
}

func TestRollbackDesired_AnyPlaceholderRefusesAndNamesItsPath(t *testing.T) {
	cases := map[string]struct {
		target string
		paths  []string
		kind   RollbackKind
	}{
		"a secret slot":         {`{"mqtt":{"server":"a","pass":"` + secrets.Placeholder + `"}}`, []string{"mqtt.pass"}, RollbackNeedsSecrets},
		"a list entry":          {`{"actions":[{"name":"x","secret_urls":"` + secrets.Placeholder + `"}]}`, []string{`actions."x".0.secret_urls`}, RollbackNeedsSecrets},
		"a key that is no slot": {`{"system":{"device":{"name":"` + secrets.Placeholder + `"}}}`, []string{"system.device.name"}, RollbackNotRestorable},
		"several, sorted": {`{"wifi":{"password":"` + secrets.Placeholder + `"},"auth":{"pass":"` + secrets.Placeholder + `"}}`,
			[]string{"auth.pass", "wifi.password"}, RollbackNeedsSecrets},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			db := historyDB(t)
			rollbackDevice(t, db, 1, "[7]", `{}`, `{"mqtt":{"server":"a.example"}}`)
			version := addVersion(t, db, 1, str(tc.target))

			_, err := rollbackService(db).RollbackDesired(1, version, nil)
			rerr := requireRollbackError(t, err, tc.kind)
			assert.Equal(t, tc.paths, rerr.Paths)

			device := deviceRow(t, db, 1)
			assert.JSONEq(t, `{"mqtt":{"server":"a.example"}}`, device.DesiredConfig)
			assert.Equal(t, "[7]", device.TemplateIDs)
			assert.True(t, device.ConfigApplied)
			assert.Len(t, historyRows(t, db, 1), 1, "no rollback row")
		})
	}
}

func TestRollbackDesired_AStaleReadLosesAndWritesNoRow(t *testing.T) {
	db := historyDB(t)
	rollbackDevice(t, db, 1, "[]", `{"mqtt":{"server":"a.example"}}`, `{"mqtt":{"server":"a.example"}}`)
	version := addVersion(t, db, 1, str(`{"mqtt":{"server":"b.example"}}`))
	repo := &editsBeforeRollback{GormConfigRepository: NewGormConfigRepository(db, nil), edit: func() {
		setOverrides(t, db, 1, `{"mqtt":{"server":"edited.example"}}`)
	}}
	svc := NewConfigurationService(repo, Engine{}, nil)

	_, err := svc.RollbackDesired(1, version, nil)
	assert.ErrorIs(t, err, ErrDesiredChanged)
	assert.Equal(t, 1, repo.writes, "one attempt, no retry")
	assert.JSONEq(t, `{"mqtt":{"server":"edited.example"}}`, deviceRow(t, db, 1).Overrides)
	assert.JSONEq(t, `{"mqtt":{"server":"a.example"}}`, deviceRow(t, db, 1).DesiredConfig)
	assert.Len(t, historyRows(t, db, 1), 1, "no rollback row")
}

// editsBeforeRollback edits the device between the service's read and the repository's write.
type editsBeforeRollback struct {
	*GormConfigRepository
	edit   func()
	writes int
}

func (r *editsBeforeRollback) RollbackDeviceDesired(deviceID uint, read DesiredRead, write DesiredWrite, restoredFrom uint) error {
	r.writes++
	r.edit()
	return r.GormConfigRepository.RollbackDeviceDesired(deviceID, read, write, restoredFrom)
}

func TestRollbackDesired_TheRoundTrip(t *testing.T) {
	t.Run("a non-zero typed value passes", func(t *testing.T) {
		db := historyDB(t)
		rollbackDevice(t, db, 1, "[]", `{}`, `{}`)
		version := addVersion(t, db, 1, str(`{"system":{"sntp":{"server":"time.example"}}}`))
		_, err := rollbackService(db).RollbackDesired(1, version, nil)
		require.NoError(t, err)
		assert.JSONEq(t, `{"system":{"sntp":{"server":"time.example"}}}`, deviceRow(t, db, 1).DesiredConfig)
	})

	t.Run("an empty string the typed model drops is refused with its path", func(t *testing.T) {
		db := historyDB(t)
		rollbackDevice(t, db, 1, "[7]", `{}`, `{"mqtt":{"server":"a.example"}}`)
		version := addVersion(t, db, 1, str(`{"system":{"sntp":{"server":""}}}`))
		_, err := rollbackService(db).RollbackDesired(1, version, nil)
		rerr := requireRollbackError(t, err, RollbackLoss)
		assert.Equal(t, []string{"system.sntp.server"}, rerr.Paths)
		device := deviceRow(t, db, 1)
		assert.JSONEq(t, `{"mqtt":{"server":"a.example"}}`, device.DesiredConfig)
		assert.Equal(t, "[7]", device.TemplateIDs)
		assert.Len(t, historyRows(t, db, 1), 1)
	})

	t.Run("an unknown field the typed model drops is refused", func(t *testing.T) {
		db := historyDB(t)
		rollbackDevice(t, db, 1, "[]", `{}`, `{}`)
		version := addVersion(t, db, 1, str(`{"mqtt":{"server":"a"},"made_up":{"x":1}}`))
		_, err := rollbackService(db).RollbackDesired(1, version, nil)
		rerr := requireRollbackError(t, err, RollbackLoss)
		assert.Equal(t, []string{"made_up"}, rerr.Paths)
	})

	t.Run("a version the recompute wrote passes", func(t *testing.T) {
		db := historyDB(t)
		addHistoryDevice(t, db, 1, "{}", mqttOverride("a.example", ""))
		svc := rollbackService(db)
		require.NoError(t, svc.RecomputeDesiredConfig(1))
		written := historyRows(t, db, 1)[0]
		require.NotNil(t, written.NewDesired)

		setOverrides(t, db, 1, mqttOverride("b.example", ""))
		require.NoError(t, svc.RecomputeDesiredConfig(1))
		_, err := svc.RollbackDesired(1, written.ID, nil)
		require.NoError(t, err)
		assert.JSONEq(t, *written.NewDesired, deviceRow(t, db, 1).DesiredConfig)
	})

	t.Run("a version in another key order passes", func(t *testing.T) {
		db := historyDB(t)
		rollbackDevice(t, db, 1, "[]", `{}`, `{}`)
		version := addVersion(t, db, 1, str(`{"system":{"sntp":{"server":"t"}},"mqtt":{"server":"m"},"wifi":{"ssid":"s"}}`))
		_, err := rollbackService(db).RollbackDesired(1, version, nil)
		require.NoError(t, err)
	})

	t.Run("a path only the merge holds passes with a zero value", func(t *testing.T) {
		db := historyDB(t)
		rollbackDevice(t, db, 1, "[]", `{}`, `{}`)
		// The typed model writes system.sleep_mode.enable without omitempty.
		version := addVersion(t, db, 1, str(`{"system":{"sleep_mode":{"period":5}}}`))
		_, err := rollbackService(db).RollbackDesired(1, version, nil)
		require.NoError(t, err)
		assert.Contains(t, deviceRow(t, db, 1).DesiredConfig, `"enable":false`)
	})

	t.Run("a path only the merge holds is refused with any other value", func(t *testing.T) {
		db := historyDB(t)
		rollbackDevice(t, db, 1, "[7]", `{}`, `{}`)
		version := addVersion(t, db, 1, str(`{"mqtt":{"server":"m"}}`))
		svc := NewConfigurationService(NewGormConfigRepository(db, nil), injectingMerger{}, nil)
		_, err := svc.RollbackDesired(1, version, nil)
		rerr := requireRollbackError(t, err, RollbackLoss)
		assert.Equal(t, []string{"wifi.ssid"}, rerr.Paths)
		assert.Equal(t, "[7]", deviceRow(t, db, 1).TemplateIDs)
	})
}

// injectingMerger adds a value the target never held, as a merge that supplied a default would.
type injectingMerger struct{}

func (injectingMerger) Merge(layers []ConfigLayer) (*MergeResult, error) {
	res, err := Engine{}.Merge(layers)
	if err != nil {
		return nil, err
	}
	ssid := "injected"
	res.Config.WiFi = &WiFiConfiguration{SSID: &ssid}
	return res, nil
}

func TestRollbackDesired_AnEqualRollbackWritesNothing(t *testing.T) {
	target := `{"mqtt":{"server":"a.example"}}`
	for _, templates := range []string{"", "null", "[]", " [] ", "[ ]", "[\n]"} {
		t.Run("templates "+templates, func(t *testing.T) {
			db := historyDB(t)
			rollbackDevice(t, db, 1, templates, target, target)
			version := addVersion(t, db, 1, str(target))
			res, err := rollbackService(db).RollbackDesired(1, version, nil)
			require.NoError(t, err)
			assert.True(t, res.NoChange)
			device := deviceRow(t, db, 1)
			assert.Equal(t, templates, device.TemplateIDs, "not even the spelling is rewritten")
			assert.True(t, device.ConfigApplied, "no pending flag for a change of nothing")
			assert.Len(t, historyRows(t, db, 1), 1, "no rollback row")
		})
	}

	t.Run("equal text with templates attached still detaches and records", func(t *testing.T) {
		db := historyDB(t)
		rollbackDevice(t, db, 1, "[3]", target, target)
		version := addVersion(t, db, 1, str(target))
		res, err := rollbackService(db).RollbackDesired(1, version, nil)
		require.NoError(t, err)
		assert.False(t, res.NoChange)
		assert.Equal(t, "[]", deviceRow(t, db, 1).TemplateIDs)
		rows := historyRows(t, db, 1)
		require.Len(t, rows, 2)
		require.NotNil(t, rows[1].PreviousTemplateIDs)
		assert.Equal(t, "[3]", *rows[1].PreviousTemplateIDs)
		assert.Equal(t, *rows[1].OldDesired, *rows[1].NewDesired, "the texts are equal and the row is still a change")
	})
}

func TestRollbackDesired_ASecretStaysOutOfEveryOutput(t *testing.T) {
	const sentinel = "sentinel-secret-9f3a"
	db := historyDB(t)
	current := `{"mqtt":{"server":"a.example","pass":"` + sentinel + `"}}`
	rollbackDevice(t, db, 1, "[]", current, current)
	svc := rollbackService(db)
	maskedVersion := addVersion(t, db, 1, str(`{"mqtt":{"server":"b.example","pass":"`+secrets.Placeholder+`"}}`))
	cleanVersion := addVersion(t, db, 1, str(`{"mqtt":{"server":"c.example"}}`))

	_, err := svc.RollbackDesired(1, maskedVersion, nil)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), sentinel)
	rerr := requireRollbackError(t, err, RollbackNeedsSecrets)
	assert.NotContains(t, strings.Join(rerr.Paths, " "), sentinel)

	res, err := svc.RollbackDesired(1, cleanVersion, nil)
	require.NoError(t, err)
	assert.NotContains(t, fmt.Sprintf("%+v", res), sentinel)
	for _, row := range historyRows(t, db, 1) {
		for _, side := range []*string{row.OldDesired, row.NewDesired} {
			if side != nil {
				assert.NotContains(t, *side, sentinel, "no history row holds the secret")
			}
		}
	}
}

// The write's recount asks about the values being written. Here the device already holds a rollback's
// write (an identical rollback committed first) while this one's read is stale, so the UPDATE matches no
// row: it is done, not a miss, and writes no row. Recounting the READ values would call it a miss.
func TestRollbackDeviceDesired_RecountsTheWrittenValues(t *testing.T) {
	db := historyDB(t)
	rollbackDevice(t, db, 1, "[]", `{"mqtt":{"server":"b.example"}}`, `{"mqtt":{"server":"b.example"}}`)
	require.NoError(t, db.Model(&DbDevice{}).Where("id = ?", 1).Update("config_applied", false).Error)
	repo := NewGormConfigRepository(db, nil)
	stale := DesiredRead{TemplateIDs: "[7]", Overrides: `{"wifi":{"ssid":"x"}}`, Desired: `{"wifi":{"ssid":"x"}}`}
	write := DesiredWrite{TemplateIDs: "[]", Overrides: `{"mqtt":{"server":"b.example"}}`, Desired: `{"mqtt":{"server":"b.example"}}`}

	require.NoError(t, repo.RollbackDeviceDesired(1, stale, write, 5))
	assert.Empty(t, historyRows(t, db, 1), "nothing changed, so no row")
}

func TestRollbackDeviceDesired_AMissAndADeletedDevice(t *testing.T) {
	db := historyDB(t)
	rollbackDevice(t, db, 1, "[]", `{}`, `{"a":1}`)
	repo := NewGormConfigRepository(db, nil)
	write := DesiredWrite{TemplateIDs: "[]", Overrides: `{"b":2}`, Desired: `{"b":2}`}
	stale := DesiredRead{TemplateIDs: "[]", Overrides: `{"x":1}`, Desired: `{"a":1}`}

	assert.ErrorIs(t, repo.RollbackDeviceDesired(1, stale, write, 5), ErrDesiredChanged)
	assert.ErrorIs(t, repo.RollbackDeviceDesired(404, stale, write, 5), ErrDeviceNotFound)
	assert.Empty(t, historyRows(t, db, 1))
	assert.JSONEq(t, `{"a":1}`, deviceRow(t, db, 1).DesiredConfig)
}

// A write whose three texts equal what the device holds writes nothing, as the service's NoChange does:
// no row, and an applied device stays applied, on every server.
func TestRollbackDeviceDesired_AnUnchangedWriteWritesNothing(t *testing.T) {
	db := historyDB(t)
	text := `{"mqtt":{"server":"b.example"}}`
	rollbackDevice(t, db, 1, "[]", text, text) // config_applied is true
	same := DesiredWrite{TemplateIDs: "[]", Overrides: text, Desired: text}

	require.NoError(t, NewGormConfigRepository(db, nil).RollbackDeviceDesired(1, DesiredRead(same), same, 5))
	assert.Empty(t, historyRows(t, db, 1))
	assert.True(t, deviceRow(t, db, 1).ConfigApplied, "an applied device is not turned pending for a change of nothing")
}
