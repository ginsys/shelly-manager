package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/testutil"
)

// An action URL can carry a user name, a password or a token, so every read path that sends a
// configuration out must hide it (ginsys/shelly-manager#381). Each test first proves the fixture
// holds the secret, so a path that hides nothing cannot pass for a path that hides it.

const (
	actionSecretURL  = "http://admin:hunter2@192.168.1.4/on?token=s3cr3t"
	actionSecretURL2 = "http://x:pw2@192.168.1.4/off"
)

var actionURLSecrets = []string{"hunter2", "s3cr3t", "pw2"}

func requireHoldsActionSecrets(t *testing.T, what string, raw []byte) {
	t.Helper()
	for _, secret := range actionURLSecrets {
		require.Contains(t, string(raw), secret, "%s: the fixture must hold the secret, or the test proves nothing", what)
	}
}

func requireNoActionSecrets(t *testing.T, what string, raw []byte) {
	t.Helper()
	for _, secret := range actionURLSecrets {
		assert.NotContains(t, string(raw), secret, "%s carries a URL secret", what)
	}
}

func actionsDesired(urls ...string) (*configuration.DeviceConfiguration, string) {
	first := []string{urls[0]}
	second := urls[1:]
	list := []configuration.ActionConfig{
		{Name: "btn_on_url", Index: 0, SecretURLs: &first},
		{Name: "out_on_url", Index: 0, SecretURLs: &second},
	}
	cfg := &configuration.DeviceConfiguration{Actions: &list}
	raw, _ := json.Marshal(cfg)
	return cfg, string(raw)
}

// The bulk download sends the desired configuration, which holds the action URLs.
func TestBulkDownload_MasksActionURLs(t *testing.T) {
	db, cleanup := testutil.TestDatabase(t)
	defer cleanup()
	handler := exportHandler(t, db)
	fleet := bulkFleet(t, db, 1)
	_, stored := actionsDesired(actionSecretURL, actionSecretURL2)
	requireHoldsActionSecrets(t, "stored desired config", []byte(stored))
	require.NoError(t, db.GetDB().Exec("UPDATE devices SET desired_config = ? WHERE id = ?",
		stored, fleet[0].device.ID).Error)

	w := postDownload(handler, idsBody(fleet[0].device.ID))
	file := decodeDownload(t, w)

	requireNoActionSecrets(t, "bulk download", w.Body.Bytes())
	require.Len(t, file.Devices, 1)
	assert.Contains(t, string(file.Devices[0].Config), "btn_on_url", "the action is kept, only its URLs are masked")
	assert.Contains(t, string(file.Devices[0].Config), secretMask)
}

// The history rows hold the configuration before and after a change, and the differences.
func TestMaskConfigHistory_MasksActionURLs(t *testing.T) {
	_, raw := actionsDesired(actionSecretURL, actionSecretURL2)
	_, otherRaw := actionsDesired("http://admin:hunter2@h/new", "http://x:pw2@h/s3cr3t")
	// The comparator already hides URLs in its differences; a row written by another producer
	// may not, so the history mask must hide them in the raw configurations and in a raw change.
	row := configuration.ConfigHistory{
		OldConfig: json.RawMessage(raw),
		NewConfig: json.RawMessage(otherRaw),
		Changes: json.RawMessage(`[{"path":"actions.0.secret_urls","expected":["` + actionSecretURL +
			`"],"actual":["` + actionSecretURL2 + `"]}]`),
	}
	requireHoldsActionSecrets(t, "old and new config", append(append([]byte{}, row.OldConfig...), row.NewConfig...))
	requireHoldsActionSecrets(t, "changes", row.Changes)

	masked := maskConfigHistory([]configuration.ConfigHistory{row})

	require.Len(t, masked, 1)
	requireNoActionSecrets(t, "old_config", masked[0].OldConfig)
	requireNoActionSecrets(t, "new_config", masked[0].NewConfig)
	requireNoActionSecrets(t, "changes", masked[0].Changes)
	assert.Contains(t, string(row.Changes), "hunter2", "the stored row is not changed")
}

// A difference the comparator reports, passed through the API's own redaction, holds no URL.
func TestRedactDifference_HoldsNoActionURL(t *testing.T) {
	desired, _ := actionsDesired(actionSecretURL, actionSecretURL2)
	changed, _ := actionsDesired("http://admin:hunter2@h/new", "http://x:pw2@h/s3cr3t")

	// Control: the same values written under an ordinary key are visible to the comparator,
	// so the masking below is the scrub's work and not an empty comparison.
	server, serverChanged := actionSecretURL, actionSecretURL2
	plainDesired := &configuration.DeviceConfiguration{MQTT: &configuration.MQTTConfiguration{Server: &server}}
	plainChanged := &configuration.DeviceConfiguration{MQTT: &configuration.MQTTConfiguration{Server: &serverChanged}}
	control := configuration.NewConfigComparator().Compare(plainDesired, plainChanged)
	require.False(t, control.Match)
	controlRaw, err := json.Marshal(control.Differences)
	require.NoError(t, err)
	requireHoldsActionSecrets(t, "control comparison", controlRaw)

	for name, pair := range map[string][2]*configuration.DeviceConfiguration{
		"changed":      {desired, changed},
		"section gone": {desired, {}},
		"device nil":   {desired, nil},
		"desired nil":  {nil, desired},
	} {
		result := configuration.NewConfigComparator().Compare(pair[0], pair[1])
		require.False(t, result.Match, name)
		for _, d := range result.Differences {
			path, expected, actual := redactDifference(d)
			raw, err := json.Marshal([]any{path, expected, actual})
			require.NoError(t, err)
			requireNoActionSecrets(t, name+" "+path, raw)
		}
	}
}
