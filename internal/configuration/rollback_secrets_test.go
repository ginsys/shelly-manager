package configuration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	secrets "github.com/ginsys/shelly-manager/internal/redact"
)

const mask = secrets.Placeholder

// currentMQTT is a device whose desired configuration holds a real broker password.
const currentMQTT = `{"mqtt":{"server":"a.example","port":1883,"user":"iot","pass":"real-pass"}}`

func TestRollbackSecrets_KeepsTheCurrentSecretWhereTheBrokerIsUnchanged(t *testing.T) {
	db := historyDB(t)
	rollbackDevice(t, db, 1, "[]", currentMQTT, currentMQTT)
	version := addVersion(t, db, 1, str(`{"mqtt":{"server":"a.example","port":1883,"user":"iot","pass":"`+mask+`"},"led":{"power_indication":false}}`))

	_, err := rollbackService(db).RollbackDesired(1, version, nil)
	require.NoError(t, err)

	device := deviceRow(t, db, 1)
	assert.Contains(t, device.Overrides, `"pass":"real-pass"`, "the current secret is kept")
	assert.NotContains(t, device.Overrides, mask)
	assert.NotContains(t, device.DesiredConfig, mask)
}

func TestRollbackSecrets_ABoundFieldThatDiffersRefusesTheSlot(t *testing.T) {
	cases := map[string]struct{ current, target, want string }{
		"mqtt server": {currentMQTT, `{"mqtt":{"server":"b.example","port":1883,"user":"iot","pass":"` + mask + `"}}`, "mqtt.pass"},
		"mqtt port":   {currentMQTT, `{"mqtt":{"server":"a.example","port":8883,"user":"iot","pass":"` + mask + `"}}`, "mqtt.pass"},
		"mqtt user":   {currentMQTT, `{"mqtt":{"server":"a.example","port":1883,"user":"other","pass":"` + mask + `"}}`, "mqtt.pass"},
		"mqtt ssl_ca": {currentMQTT, `{"mqtt":{"server":"a.example","port":1883,"user":"iot","ssl_ca":true,"pass":"` + mask + `"}}`, "mqtt.pass"},
		"mqtt sni":    {currentMQTT, `{"mqtt":{"server":"a.example","port":1883,"user":"iot","enable_sni":true,"pass":"` + mask + `"}}`, "mqtt.pass"},
		"station ssid": {`{"wifi":{"ssid":"home","password":"real-pass"}}`,
			`{"wifi":{"ssid":"away","password":"` + mask + `"}}`, "wifi.password"},
		"ap ssid": {`{"wifi":{"ap":{"ssid":"ap1","pass":"real-pass"}}}`,
			`{"wifi":{"ap":{"ssid":"ap2","pass":"` + mask + `"}}}`, "wifi.ap.pass"},
		"auth user": {`{"auth":{"user":"admin","pass":"real-pass"}}`,
			`{"auth":{"user":"root","pass":"` + mask + `"}}`, "auth.pass"},
		"network sta": {`{"network":{"wifi":{"sta":{"ssid":"home","pass":"real-pass"}}}}`,
			`{"network":{"wifi":{"sta":{"ssid":"away","pass":"` + mask + `"}}}}`, "network.wifi.sta.pass"},
		"network ap": {`{"network":{"wifi":{"ap":{"ssid":"ap1","pass":"real-pass"}}}}`,
			`{"network":{"wifi":{"ap":{"ssid":"ap2","pass":"` + mask + `"}}}}`, "network.wifi.ap.pass"},
		"the current secret is empty": {`{"mqtt":{"server":"a.example","port":1883,"user":"iot","pass":""}}`,
			`{"mqtt":{"server":"a.example","port":1883,"user":"iot","pass":"` + mask + `"}}`, "mqtt.pass"},
		"the current holds no secret": {`{"mqtt":{"server":"a.example","port":1883,"user":"iot"}}`,
			`{"mqtt":{"server":"a.example","port":1883,"user":"iot","pass":"` + mask + `"}}`, "mqtt.pass"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			db := historyDB(t)
			rollbackDevice(t, db, 1, "[]", c.current, c.current)
			before := deviceRow(t, db, 1)
			version := addVersion(t, db, 1, str(c.target))

			_, err := rollbackService(db).RollbackDesired(1, version, nil)
			rerr := requireRollbackError(t, err, RollbackNeedsSecrets)
			assert.Equal(t, []string{c.want}, rerr.Paths)
			assert.Equal(t, before.DesiredConfig, deviceRow(t, db, 1).DesiredConfig, "nothing was written")
			assert.Empty(t, historyRows(t, db, 1)[1:], "no row")
		})
	}
}

func TestRollbackSecrets_ASlotWithNoBindingIsAlwaysRefused(t *testing.T) {
	current := `{"wifi":{"ap":{"ssid":"ap1","key":"real-key"}}}`
	db := historyDB(t)
	rollbackDevice(t, db, 1, "[]", current, current)
	version := addVersion(t, db, 1, str(`{"wifi":{"ap":{"ssid":"ap1","key":"`+mask+`"}}}`))

	_, err := rollbackService(db).RollbackDesired(1, version, nil)
	rerr := requireRollbackError(t, err, RollbackNeedsSecrets)
	assert.Equal(t, []string{"wifi.ap.key"}, rerr.Paths)
}

func TestRollbackSecrets_TheOperatorsSecretSatisfiesTheSlot(t *testing.T) {
	db := historyDB(t)
	rollbackDevice(t, db, 1, "[]", currentMQTT, currentMQTT)
	version := addVersion(t, db, 1, str(`{"mqtt":{"server":"b.example","port":1883,"user":"iot","pass":"`+mask+`"}}`))

	_, err := rollbackService(db).RollbackDesired(1, version, map[string]any{"mqtt.pass": "new-pass"})
	require.NoError(t, err)

	device := deviceRow(t, db, 1)
	assert.Contains(t, device.Overrides, `"pass":"new-pass"`)
	assert.Contains(t, device.Overrides, `"server":"b.example"`, "the broker is the version's, the secret the operator's")
	assert.NotContains(t, device.Overrides, "real-pass", "the current secret is not kept for a changed broker")
	for _, row := range historyRows(t, db, 1) {
		for _, side := range []*string{row.OldDesired, row.NewDesired} {
			if side != nil {
				assert.NotContains(t, *side, "new-pass", "no history row holds the secret")
			}
		}
	}
}

func TestRollbackSecrets_ABadAnswerIsRefusedAndWritesNothing(t *testing.T) {
	target := `{"mqtt":{"server":"b.example","port":1883,"user":"iot","pass":"` + mask + `"}}`
	cases := map[string]map[string]any{
		"a key off the list":    {"mqtt.pass": "x", "auth.pass": "y"},
		"an empty value":        {"mqtt.pass": ""},
		"the mask":              {"mqtt.pass": mask},
		"a number":              {"mqtt.pass": 5},
		"a list for a string":   {"mqtt.pass": []any{"x"}},
		"an empty list":         {"mqtt.pass": []any{}},
		"a null":                {"mqtt.pass": nil},
		"an unlisted key alone": {"nope": "x"},
	}
	for name, supplied := range cases {
		t.Run(name, func(t *testing.T) {
			db := historyDB(t)
			rollbackDevice(t, db, 1, "[]", currentMQTT, currentMQTT)
			version := addVersion(t, db, 1, str(target))

			_, err := rollbackService(db).RollbackDesired(1, version, supplied)
			_ = requireRollbackError(t, err, RollbackBadSecrets)
			assert.Equal(t, currentMQTT, deviceRow(t, db, 1).DesiredConfig)
			assert.NotContains(t, err.Error(), "x")
		})
	}
}

func TestRollbackSecrets_AnActionURLListIsAnOperatorSlot(t *testing.T) {
	current := `{"actions":[{"name":"btn_on_url","index":0,"secret_urls":["http://real.example/a"]}]}`
	target := `{"actions":[{"name":"btn_on_url","index":0,"secret_urls":["` + mask + `"]}]}`
	const key = `actions."btn_on_url".0.secret_urls`

	t.Run("it is listed under its slot key, though the current holds the same list", func(t *testing.T) {
		db := historyDB(t)
		rollbackDevice(t, db, 1, "[]", current, current)
		version := addVersion(t, db, 1, str(target))
		_, err := rollbackService(db).RollbackDesired(1, version, nil)
		rerr := requireRollbackError(t, err, RollbackNeedsSecrets)
		assert.Equal(t, []string{key}, rerr.Paths)
	})
	t.Run("a list of URLs answers it", func(t *testing.T) {
		db := historyDB(t)
		rollbackDevice(t, db, 1, "[]", current, current)
		version := addVersion(t, db, 1, str(target))
		_, err := rollbackService(db).RollbackDesired(1, version, map[string]any{key: []any{"http://new.example/a", "http://new.example/b"}})
		require.NoError(t, err)
		assert.Contains(t, deviceRow(t, db, 1).Overrides, "http://new.example/b")
	})
	for name, bad := range map[string]any{
		"a string":             "http://x",
		"an empty list":        []any{},
		"a list with the mask": []any{mask},
		"a list with an empty": []any{""},
		"a list of numbers":    []any{1},
	} {
		t.Run("refuses "+name, func(t *testing.T) {
			db := historyDB(t)
			rollbackDevice(t, db, 1, "[]", current, current)
			version := addVersion(t, db, 1, str(target))
			_, err := rollbackService(db).RollbackDesired(1, version, map[string]any{key: bad})
			_ = requireRollbackError(t, err, RollbackBadSecrets)
		})
	}
}

func TestRollbackSecrets_TwoActionsOfOneIdentityAreTwoSlots(t *testing.T) {
	current := `{}`
	target := `{"actions":[{"name":"a","index":0,"secret_urls":["` + mask + `"]},{"name":"a","index":0,"secret_urls":["` + mask + `"]}]}`
	db := historyDB(t)
	rollbackDevice(t, db, 1, "[]", current, current)
	version := addVersion(t, db, 1, str(target))
	_, err := rollbackService(db).RollbackDesired(1, version, nil)
	rerr := requireRollbackError(t, err, RollbackNeedsSecrets)
	assert.Equal(t, []string{`actions."a".0.secret_urls#0`, `actions."a".0.secret_urls#1`}, rerr.Paths)
}

func TestRollbackSecrets_AMaskAnywhereElseIsRefusedByItsPath(t *testing.T) {
	db := historyDB(t)
	rollbackDevice(t, db, 1, "[]", `{}`, `{}`)
	version := addVersion(t, db, 1, str(`{"custom":{"token":"`+mask+`"}}`))
	_, err := rollbackService(db).RollbackDesired(1, version, nil)
	rerr := requireRollbackError(t, err, RollbackNeedsSecrets)
	assert.Equal(t, []string{"custom.token"}, rerr.Paths)
}

func TestSecretBindingPaths_NameEverySlot(t *testing.T) {
	assert.Equal(t, []string{
		"auth.pass", "mqtt.pass", "network.wifi.ap.pass", "network.wifi.sta.pass",
		"wifi.ap.key", "wifi.ap.pass", "wifi.password",
	}, SecretBindingPaths())
}

func TestRollbackSecrets_AKeptSlotTakesNoAnswer(t *testing.T) {
	current := `{"wifi":{"ssid":"home","password":"real-wifi"},"mqtt":{"server":"a.example","pass":"real-pass"}}`
	target := `{"wifi":{"ssid":"home","password":"` + mask + `"},"mqtt":{"server":"b.example","pass":"` + mask + `"}}`
	db := historyDB(t)
	rollbackDevice(t, db, 1, "[]", current, current)
	version := addVersion(t, db, 1, str(target))
	svc := rollbackService(db)

	_, err := svc.RollbackDesired(1, version, map[string]any{"wifi.password": "other", "mqtt.pass": "new"})
	_ = requireRollbackError(t, err, RollbackBadSecrets)

	_, err = svc.RollbackDesired(1, version, map[string]any{"mqtt.pass": "new"})
	require.NoError(t, err)
	stored := deviceRow(t, db, 1).Overrides
	assert.Contains(t, stored, `"password":"real-wifi"`, "the Wi-Fi secret is kept: its network is unchanged")
	assert.Contains(t, stored, `"pass":"new"`)
}

// A bound field is compared exactly: another spelling of a number, or a list or an object where the
// other side holds a scalar, is a change, and no comparison panics.
func TestRollbackSecrets_BoundFieldsCompareExactly(t *testing.T) {
	cases := map[string]struct{ current, target string }{
		"a number spelled another way": {
			`{"mqtt":{"server":"a.example","port":1883.0,"pass":"real-pass"}}`,
			`{"mqtt":{"server":"a.example","port":1883,"pass":"` + mask + `"}}`},
		"a list on both sides": {
			`{"wifi":{"ssid":["home"],"password":"real-pass"}}`,
			`{"wifi":{"ssid":["away"],"password":"` + mask + `"}}`},
		"an object on both sides": {
			`{"wifi":{"ssid":{"a":1},"password":"real-pass"}}`,
			`{"wifi":{"ssid":{"a":2},"password":"` + mask + `"}}`},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			db := historyDB(t)
			rollbackDevice(t, db, 1, "[]", c.current, c.current)
			version := addVersion(t, db, 1, str(c.target))
			_, err := rollbackService(db).RollbackDesired(1, version, nil)
			_ = requireRollbackError(t, err, RollbackNeedsSecrets)
		})
	}
}

// Only a secret slot takes an answer. A mask at another path (a stored value that merely equals the
// mask) is listed and has no way through, so an answer cannot put a secret into a field that history
// does not mask, and cannot rename an action to make another key.
func TestRollbackSecrets_OnlyASlotTakesAnAnswer(t *testing.T) {
	cases := map[string]struct {
		target   string
		supplied map[string]any
		want     []string
	}{
		"a plain field": {`{"system":{"device":{"name":"` + mask + `"}}}`,
			map[string]any{"system.device.name": "sentinel"}, nil},
		"a masked action name": {`{"actions":[{"name":"` + mask + `","index":0,"secret_urls":["` + mask + `"]}]}`,
			map[string]any{"actions.0.name": "sentinel"}, nil},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			db := historyDB(t)
			rollbackDevice(t, db, 1, "[]", `{}`, `{}`)
			version := addVersion(t, db, 1, str(c.target))
			_, err := rollbackService(db).RollbackDesired(1, version, c.supplied)
			_ = requireRollbackError(t, err, RollbackBadSecrets)
			assert.Equal(t, `{}`, deviceRow(t, db, 1).Overrides)
		})
	}
}
