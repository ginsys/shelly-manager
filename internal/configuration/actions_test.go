package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/shelly/gen1"
)

// The applier and the verifier reach the actions routes through ActionSettingsClient, so the real
// Gen1 client must satisfy it, or apply fails every action on a real device.
var _ ActionSettingsClient = (*gen1.Client)(nil)

// secretActionURL carries a password in its userinfo and a token in its query, the two places a
// URL keeps a secret. Every masking and no-leak test below uses it.
const secretActionURL = "http://admin:hunter2@192.168.1.4/on?token=s3cr3t"

func urlList(urls ...string) *[]string { return &urls }

func actionEntry(name string, index int, urls ...string) ActionConfig {
	return ActionConfig{Name: name, Index: index, SecretURLs: urlList(urls...)}
}

func actionsConfig(entries ...ActionConfig) *DeviceConfiguration {
	return &DeviceConfiguration{Actions: &entries}
}

// actionApplyClient adds the actions surface to the shared apply test client. device is what
// GET /settings/actions returns and a write changes it as the real device does: a urls[] value
// replaces the list, an empty list clears it, and a key that was not sent stays as it is.
type actionApplyClient struct {
	*applyTestClient
	device map[string][]gen1.ActionEntry
	writes []actionWrite
	reads  int
	// ignoreWrites makes the device answer 200 to a write and keep what it held.
	ignoreWrites bool
	setErr       error
	getErr       error
}

type actionWrite struct {
	index   int
	name    string
	enabled *bool
	urls    []string
}

func newActionApplyClient() *actionApplyClient {
	return &actionApplyClient{applyTestClient: newApplyTestClient(), device: map[string][]gen1.ActionEntry{}}
}

func (m *actionApplyClient) SetActionForm(ctx context.Context, index int, name string, enabled *bool, urls []string) error {
	if m.setErr != nil {
		return m.setErr
	}
	m.writes = append(m.writes, actionWrite{index: index, name: name, enabled: enabled, urls: urls})
	if m.ignoreWrites {
		return nil
	}
	entries := m.device[name]
	for len(entries) <= index {
		entries = append(entries, gen1.ActionEntry{Index: len(entries), URLs: []string{}})
	}
	if enabled != nil {
		entries[index].Enabled = *enabled
	}
	if urls != nil {
		entries[index].URLs = append([]string{}, urls...)
	}
	m.device[name] = entries
	return nil
}

func (m *actionApplyClient) GetActions(ctx context.Context) (map[string][]gen1.ActionEntry, error) {
	m.reads++
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.device, nil
}

func TestActionConfig_AClearSurvivesAStoredJSONRoundTrip(t *testing.T) {
	raw := `{"actions":[{"name":"btn_on_url","index":0,"secret_urls":[]},{"name":"out_on_url","index":1,"enabled":false}]}`

	config, err := parseApplicableDesiredConfig(raw)
	require.NoError(t, err)
	require.NotNil(t, config.Actions)
	entries := *config.Actions
	require.Len(t, entries, 2)
	require.NotNil(t, entries[0].SecretURLs, "a clear must stay a value, not become 'left alone'")
	assert.Empty(t, *entries[0].SecretURLs)
	assert.Nil(t, entries[1].SecretURLs, "an omitted list stays omitted")

	stored, err := json.Marshal(config)
	require.NoError(t, err)
	again, err := parseApplicableDesiredConfig(string(stored))
	require.NoError(t, err)
	require.NotNil(t, (*again.Actions)[0].SecretURLs, "the clear was lost by the store round trip: %s", stored)
	assert.Contains(t, string(stored), `"secret_urls":[]`)
}

func TestActionConfig_AnOverrideHoldingOnlyActionsIsNotEmpty(t *testing.T) {
	assert.False(t, isEmptyConfig(actionsConfig(actionEntry("btn_on_url", 0, "http://a/1"))),
		"an override that holds only actions was dropped from the merge")
	assert.True(t, isEmptyConfig(&DeviceConfiguration{}))
}

func TestActionConfig_AMergedLayerReplacesTheListWhole(t *testing.T) {
	template := actionsConfig(actionEntry("out_on_url", 0, "http://t/1"), actionEntry("btn_on_url", 0, "http://t/2"))
	override := actionsConfig(actionEntry("btn_on_url", 0, "http://o/1"))

	result, err := MergeConfigurations([]ConfigLayer{{Name: "template", Config: template}, {Name: "override", Config: override}})
	require.NoError(t, err)

	require.NotNil(t, result.Config.Actions)
	entries := *result.Config.Actions
	require.Len(t, entries, 1, "a layer that names actions names all of them")
	assert.Equal(t, "btn_on_url", entries[0].Name)
	assert.Equal(t, []string{"http://o/1"}, *entries[0].SecretURLs)
}

func TestUnsupportedActions_AcceptsWhatTheModelKeeps(t *testing.T) {
	converter := NewGen1Converter(nil)
	cases := []struct {
		deviceType string
		entry      ActionConfig
	}{
		{"SHSW-1", actionEntry("longpush_url", 0, "http://192.168.1.4/lp")},
		{"SHSW-PM", actionEntry("out_off_url", 0, "http://localhost/relay/0?turn=on")},
		{"SHSW-25", actionEntry("btn_off_url", 1, "http://a/1", "http://a/2", "http://a/3", "http://a/4", "http://a/5")},
		{"SHSW-25", actionEntry("shortpush_url", 0)},
		{"SHPLG-S", actionEntry("out_on_url", 0, "http://a/1")},
		{"SHPLG-1", ActionConfig{Name: "btn_on_url", Index: 0, Enabled: BoolPtr(false)}},
	}
	for _, tc := range cases {
		unsupported, err := unsupportedApplySettings(converter, actionsConfig(tc.entry), tc.deviceType, true)
		require.NoError(t, err)
		assert.Empty(t, unsupported, "%s %s was refused", tc.deviceType, tc.entry.Name)
	}
}

func TestUnsupportedActions_RefusesWhatTheDeviceCannotKeep(t *testing.T) {
	converter := NewGen1Converter(nil)
	long := "http://" + strings.Repeat("a", 250)  // 257 bytes
	exact := "http://" + strings.Repeat("a", 249) // 256 bytes
	cases := []struct {
		name       string
		deviceType string
		config     *DeviceConfiguration
		want       []string
	}{
		{"unknown action name", "SHSW-1", actionsConfig(actionEntry("lp_on_url", 0, "http://a/1")), []string{"actions[0].name"}},
		{"name the model lacks", "SHPLG-S", actionsConfig(actionEntry("longpush_url", 0, "http://a/1")), []string{"actions[0].name"}},
		{"roller names are not in this slice", "SHSW-25", actionsConfig(actionEntry("roller_open_url", 0, "http://a/1")), []string{"actions[0].name"}},
		{"index past the model", "SHSW-1", actionsConfig(actionEntry("btn_on_url", 1, "http://a/1")), []string{"actions[0].index"}},
		{"index past a 2.5", "SHSW-25", actionsConfig(actionEntry("btn_on_url", 2, "http://a/1")), []string{"actions[0].index"}},
		{"negative index", "SHSW-25", actionsConfig(actionEntry("btn_on_url", -1, "http://a/1")), []string{"actions[0].index"}},
		{"six URLs", "SHSW-1", actionsConfig(actionEntry("btn_on_url", 0, "http://a/1", "http://a/2", "http://a/3", "http://a/4", "http://a/5", "http://a/6")), []string{"actions[0].secret_urls"}},
		{"257 byte URL", "SHSW-1", actionsConfig(actionEntry("btn_on_url", 0, long)), []string{"actions[0].secret_urls"}},
		{"https", "SHSW-1", actionsConfig(actionEntry("btn_on_url", 0, "https://a/1")), []string{"actions[0].secret_urls"}},
		{"empty URL", "SHSW-1", actionsConfig(actionEntry("btn_on_url", 0, "")), []string{"actions[0].secret_urls"}},
		{"scheme only", "SHSW-1", actionsConfig(actionEntry("btn_on_url", 0, "http://")), []string{"actions[0].secret_urls"}},
		{"space in URL", "SHSW-1", actionsConfig(actionEntry("btn_on_url", 0, "http://a/x y")), []string{"actions[0].secret_urls"}},
		{"control character", "SHSW-1", actionsConfig(actionEntry("btn_on_url", 0, "http://a/x\ny")), []string{"actions[0].secret_urls"}},
		{"duplicate identity", "SHSW-1", actionsConfig(actionEntry("btn_on_url", 0, "http://a/1"), actionEntry("btn_on_url", 0, "http://a/2")), []string{"actions[1].name"}},
		{"i3 has no actions section here", "SHIX3-1", actionsConfig(actionEntry("btn_on_url", 0, "http://a/1")), []string{"actions"}},
		{"unknown model", "SNSW-001X16EU", actionsConfig(actionEntry("btn_on_url", 0, "http://a/1")), []string{"actions"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			unsupported, err := unsupportedApplySettings(converter, tc.config, tc.deviceType, true)
			require.NoError(t, err)
			assert.Equal(t, tc.want, unsupported)
		})
	}

	// The exact limit is kept: the limit is 256 bytes, so 256 passes and 257 does not.
	unsupported, err := unsupportedApplySettings(converter, actionsConfig(actionEntry("btn_on_url", 0, exact)), "SHSW-1", true)
	require.NoError(t, err)
	assert.Empty(t, unsupported)
}

func TestUnsupportedActions_ARefusalNamesNoURL(t *testing.T) {
	unsupported, err := unsupportedApplySettings(NewGen1Converter(nil), actionsConfig(actionEntry("btn_on_url", 0, secretActionURL+" bad")), "SHSW-1", true)
	require.NoError(t, err)
	require.NotEmpty(t, unsupported)
	for _, path := range unsupported {
		assert.NotContains(t, path, "hunter2")
		assert.NotContains(t, path, "s3cr3t")
	}
}

func TestApplyActions_WritesEachEntryAndReadsItBack(t *testing.T) {
	client := newActionApplyClient()
	applier := NewConfigApplier(NewGen1Converter(nil), nil)
	config := actionsConfig(
		actionEntry("btn_on_url", 0, "http://192.168.1.4/on", "http://192.168.1.5/on"),
		ActionConfig{Name: "out_on_url", Index: 1, Enabled: BoolPtr(true), SecretURLs: urlList("http://192.168.1.6/x")},
	)

	result, err := applier.ApplyConfig(context.Background(), client, config, "SHSW-25")

	require.NoError(t, err)
	assert.True(t, result.Success, "failures: %+v", result.Failures)
	require.Len(t, client.writes, 2, "one write per entry")
	assert.Equal(t, actionWrite{index: 0, name: "btn_on_url", urls: []string{"http://192.168.1.4/on", "http://192.168.1.5/on"}}, client.writes[0])
	assert.Equal(t, "out_on_url", client.writes[1].name)
	assert.Equal(t, 1, client.writes[1].index)
	require.NotNil(t, client.writes[1].enabled)
	assert.True(t, *client.writes[1].enabled)
	assert.Equal(t, 2, client.reads, "one read-back per entry")
	assert.Equal(t, 3, result.SettingsCount, "one setting per written key")
	assert.Equal(t, 3, result.AppliedCount)
}

func TestApplyActions_AClearWritesAnEmptyListAndReadsItBack(t *testing.T) {
	client := newActionApplyClient()
	client.device["btn_on_url"] = []gen1.ActionEntry{{Index: 0, URLs: []string{"http://old/1"}, Enabled: true}}
	applier := NewConfigApplier(NewGen1Converter(nil), nil)

	result, err := applier.ApplyConfig(context.Background(), client, actionsConfig(actionEntry("btn_on_url", 0)), "SHSW-1")

	require.NoError(t, err)
	assert.True(t, result.Success, "failures: %+v", result.Failures)
	require.Len(t, client.writes, 1)
	require.NotNil(t, client.writes[0].urls, "a clear is an empty list, not 'no list'")
	assert.Empty(t, client.writes[0].urls)
	assert.Empty(t, client.device["btn_on_url"][0].URLs)
}

func TestApplyActions_AnOmittedActionIsNeitherWrittenNorCleared(t *testing.T) {
	client := newActionApplyClient()
	client.device["out_on_url"] = []gen1.ActionEntry{{Index: 0, URLs: []string{"http://kept/1"}, Enabled: true}}
	applier := NewConfigApplier(NewGen1Converter(nil), nil)

	result, err := applier.ApplyConfig(context.Background(), client, actionsConfig(actionEntry("btn_on_url", 0, "http://a/1")), "SHSW-1")

	require.NoError(t, err)
	assert.True(t, result.Success, "failures: %+v", result.Failures)
	require.Len(t, client.writes, 1)
	assert.Equal(t, "btn_on_url", client.writes[0].name)
	assert.Equal(t, []string{"http://kept/1"}, client.device["out_on_url"][0].URLs)
}

func TestApplyActions_AnEntryWithNothingToSetWritesNothing(t *testing.T) {
	client := newActionApplyClient()
	applier := NewConfigApplier(NewGen1Converter(nil), nil)

	result, err := applier.ApplyConfig(context.Background(), client, actionsConfig(ActionConfig{Name: "btn_on_url", Index: 0}), "SHSW-1")

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Empty(t, client.writes)
	assert.Equal(t, 0, client.reads)
}

func TestApplyActions_ADeviceThatKeepsTheOldURLsFailsTheSectionWithoutPrintingThem(t *testing.T) {
	client := newActionApplyClient()
	client.ignoreWrites = true
	client.device["btn_on_url"] = []gen1.ActionEntry{{Index: 0, URLs: []string{"http://old:pw@h/1"}, Enabled: true}}
	applier := NewConfigApplier(NewGen1Converter(nil), nil)

	result, err := applier.ApplyConfig(context.Background(), client, actionsConfig(actionEntry("btn_on_url", 0, secretActionURL)), "SHSW-1")

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Equal(t, 1, result.FailedCount)
	require.Len(t, result.Failures, 1)
	assert.Equal(t, "actions[0].secret_urls", result.Failures[0].Path)
	assert.Contains(t, result.Failures[0].Value, "btn_on_url")
	assert.Contains(t, result.Failures[0].Value, "1 URL")
	raw, marshalErr := json.Marshal(result)
	require.NoError(t, marshalErr)
	for _, secret := range []string{"hunter2", "s3cr3t", "old:pw"} {
		assert.NotContains(t, string(raw), secret, "the apply result carries a URL secret")
	}
}

func TestApplyActions_ADeviceThatKeepsTheOldEnabledFlagFailsThatKey(t *testing.T) {
	client := newActionApplyClient()
	client.ignoreWrites = true
	client.device["btn_on_url"] = []gen1.ActionEntry{{Index: 0, URLs: []string{}, Enabled: false}}
	applier := NewConfigApplier(NewGen1Converter(nil), nil)

	result, err := applier.ApplyConfig(context.Background(), client, actionsConfig(ActionConfig{Name: "btn_on_url", Index: 0, Enabled: BoolPtr(true)}), "SHSW-1")

	require.NoError(t, err)
	assert.False(t, result.Success)
	require.Len(t, result.Failures, 1)
	assert.Equal(t, "actions[0].enabled", result.Failures[0].Path)
	assert.Contains(t, result.Failures[0].Error, "false")
}

func TestApplyActions_AMissingEntryInTheReadBackFails(t *testing.T) {
	client := newActionApplyClient()
	client.ignoreWrites = true
	applier := NewConfigApplier(NewGen1Converter(nil), nil)

	result, err := applier.ApplyConfig(context.Background(), client, actionsConfig(ActionConfig{Name: "out_on_url", Index: 0, Enabled: BoolPtr(true)}), "SHSW-1")

	require.NoError(t, err)
	assert.False(t, result.Success)
	require.Len(t, result.Failures, 1)
	assert.Equal(t, "actions[0].enabled", result.Failures[0].Path)
}

func TestApplyActions_AWriteErrorFailsEveryKeyAndHidesTheURL(t *testing.T) {
	client := newActionApplyClient()
	client.setErr = errors.New("POST failed for " + secretActionURL)
	applier := NewConfigApplier(NewGen1Converter(nil), nil)
	config := actionsConfig(ActionConfig{Name: "btn_on_url", Index: 0, Enabled: BoolPtr(true), SecretURLs: urlList(secretActionURL)})

	result, err := applier.ApplyConfig(context.Background(), client, config, "SHSW-1")

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Equal(t, 2, result.FailedCount)
	assert.Equal(t, 0, result.AppliedCount)
	raw, marshalErr := json.Marshal(result)
	require.NoError(t, marshalErr)
	assert.NotContains(t, string(raw), "hunter2")
	assert.NotContains(t, string(raw), "s3cr3t")
}

func TestApplyActions_AReadBackErrorFailsTheEntry(t *testing.T) {
	client := newActionApplyClient()
	client.getErr = errors.New("timeout")
	applier := NewConfigApplier(NewGen1Converter(nil), nil)

	result, err := applier.ApplyConfig(context.Background(), client, actionsConfig(actionEntry("btn_on_url", 0, "http://a/1")), "SHSW-1")

	require.NoError(t, err)
	assert.False(t, result.Success)
	require.NotEmpty(t, result.Failures)
	assert.Contains(t, result.Failures[0].Error, "read-back failed")
}

// A client without the actions surface cannot write an action, and apply says so instead of
// dropping the section silently.
func TestApplyActions_AClientWithoutTheSurfaceFails(t *testing.T) {
	client := newApplyTestClient()
	applier := NewConfigApplier(NewGen1Converter(nil), nil)

	result, err := applier.ApplyConfig(context.Background(), client, actionsConfig(actionEntry("btn_on_url", 0, "http://a/1")), "SHSW-1")

	require.NoError(t, err)
	assert.False(t, result.Success)
	require.Len(t, result.Failures, 1)
	assert.Equal(t, "actions", result.Failures[0].Path)
}

// Every masking rule in the manager matches a key by its name, and the typed key of an action's URL
// list must be one all of them match (the other rules are tested in their own packages).
func TestActionConfig_TheTypedKeyIsASensitiveField(t *testing.T) {
	assert.True(t, isSensitiveField("secret_urls"), "the configuration service would show the URLs")
}
