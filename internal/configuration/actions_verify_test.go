package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/shelly/gen1"
)

// noURLSecrets fails when a marshalled value still holds a part of secretActionURL or of the other
// secret URLs these tests use.
func noURLSecrets(t *testing.T, what string, value interface{}) {
	t.Helper()
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	for _, secret := range []string{"hunter2", "s3cr3t", "pw2", "oldpw"} {
		assert.NotContains(t, string(raw), secret, "%s carries a URL secret", what)
	}
}

func verifyActions(t *testing.T, client ShellyClient, desired *DeviceConfiguration, deviceType string) (*VerifyResult, error) {
	t.Helper()
	return NewConfigVerifier(NewGen1Converter(nil), nil).VerifyConfig(context.Background(), client, desired, deviceType)
}

func clientHolding(device map[string][]gen1.ActionEntry) *actionApplyClient {
	client := newActionApplyClient()
	client.device = device
	return client
}

func TestVerifyActions_MatchWhenTheDeviceHoldsWhatIsDesired(t *testing.T) {
	client := clientHolding(map[string][]gen1.ActionEntry{
		"btn_on_url": {{Index: 0, URLs: []string{secretActionURL}, Enabled: true}},
	})
	desired := actionsConfig(ActionConfig{Name: "btn_on_url", Index: 0, Enabled: BoolPtr(true), SecretURLs: urlList(secretActionURL)})

	result, err := verifyActions(t, client, desired, "SHSW-1")

	require.NoError(t, err)
	assert.True(t, result.Match, "differences: %+v", result.Differences)
}

func TestVerifyActions_ADeviceAnswerInAnotherOrderStillMatches(t *testing.T) {
	client := clientHolding(map[string][]gen1.ActionEntry{
		"out_on_url": {{Index: 1, URLs: []string{"http://b/1"}}, {Index: 0, URLs: []string{"http://b/0"}}},
		"btn_on_url": {{Index: 0, URLs: []string{"http://a/1"}}},
	})
	desired := actionsConfig(actionEntry("btn_on_url", 0, "http://a/1"), actionEntry("out_on_url", 0, "http://b/0"), actionEntry("out_on_url", 1, "http://b/1"))

	result, err := verifyActions(t, client, desired, "SHSW-25")

	require.NoError(t, err)
	assert.True(t, result.Match, "differences: %+v", result.Differences)
}

func TestVerifyActions_AChangedURLIsADifferenceThatHoldsNoURL(t *testing.T) {
	client := clientHolding(map[string][]gen1.ActionEntry{
		"btn_on_url": {{Index: 0, URLs: []string{"http://x:pw2@h/2"}}},
	})

	result, err := verifyActions(t, client, actionsConfig(actionEntry("btn_on_url", 0, secretActionURL)), "SHSW-1")

	require.NoError(t, err)
	assert.False(t, result.Match)
	require.NotEmpty(t, result.Differences)
	noURLSecrets(t, "the differences", result.Differences)
	noURLSecrets(t, "the diff report", NewConfigVerifier(NewGen1Converter(nil), nil).GetDiffReport(result))
}

func TestVerifyActions_AnExtraDeviceURLIsADifference(t *testing.T) {
	client := clientHolding(map[string][]gen1.ActionEntry{
		"btn_on_url": {{Index: 0, URLs: []string{"http://a/1", "http://x:pw2@h/extra"}}},
	})

	result, err := verifyActions(t, client, actionsConfig(actionEntry("btn_on_url", 0, "http://a/1")), "SHSW-1")

	require.NoError(t, err)
	assert.False(t, result.Match, "an extra URL on the device read as a match")
	noURLSecrets(t, "the differences", result.Differences)
}

func TestVerifyActions_ADesiredClearAgainstURLsOnTheDeviceIsADifference(t *testing.T) {
	client := clientHolding(map[string][]gen1.ActionEntry{
		"btn_on_url": {{Index: 0, URLs: []string{"http://x:pw2@h/kept"}}},
	})

	result, err := verifyActions(t, client, actionsConfig(actionEntry("btn_on_url", 0)), "SHSW-1")

	require.NoError(t, err)
	assert.False(t, result.Match, "a clear that the device did not keep read as a match")
	noURLSecrets(t, "the differences", result.Differences)
}

func TestVerifyActions_ADesiredURLAgainstAnEmptyListIsADifference(t *testing.T) {
	client := clientHolding(map[string][]gen1.ActionEntry{
		"btn_on_url": {{Index: 0, URLs: []string{}}},
	})

	result, err := verifyActions(t, client, actionsConfig(actionEntry("btn_on_url", 0, secretActionURL)), "SHSW-1")

	require.NoError(t, err)
	assert.False(t, result.Match)
	noURLSecrets(t, "the differences", result.Differences)
}

func TestVerifyActions_AnEnabledFlagThatDiffersIsADifference(t *testing.T) {
	client := clientHolding(map[string][]gen1.ActionEntry{
		"btn_on_url": {{Index: 0, URLs: []string{"http://a/1"}, Enabled: false}},
	})
	desired := actionsConfig(ActionConfig{Name: "btn_on_url", Index: 0, Enabled: BoolPtr(true)})

	result, err := verifyActions(t, client, desired, "SHSW-1")

	require.NoError(t, err)
	assert.False(t, result.Match)
}

func TestVerifyActions_AMissingPairDoesNotShiftTheNextEntry(t *testing.T) {
	client := clientHolding(map[string][]gen1.ActionEntry{
		"out_on_url": {{Index: 0, URLs: []string{"http://right/1"}}},
	})
	desired := actionsConfig(
		ActionConfig{Name: "btn_on_url", Index: 0, Enabled: BoolPtr(true)}, // the device reports none
		actionEntry("out_on_url", 0, "http://wrong/1"),                     // and holds another URL here
	)

	result, err := verifyActions(t, client, desired, "SHSW-1")

	require.NoError(t, err)
	assert.False(t, result.Match)
	paths := map[string]bool{}
	for _, diff := range result.Differences {
		paths[diff.Path] = true
	}
	assert.True(t, paths["actions.0.enabled"], "the missing pair is reported at its own position: %v", paths)
	assert.True(t, paths["actions.1.secret_urls.0"], "the next entry is compared with its own device entry: %v", paths)
}

func TestVerifyActions_AModelWithNoActionsReportsTheSectionWithoutItsURLs(t *testing.T) {
	for _, deviceType := range []string{"SHIX3-1", "SNSW-001X16EU"} {
		client := newApplyTestClient()
		result, err := verifyActions(t, client, actionsConfig(actionEntry("btn_on_url", 0, secretActionURL)), deviceType)
		if err != nil {
			continue // a Gen2 model verified with a Gen1 converter may refuse the document: no URL in it either
		}
		assert.False(t, result.Match, deviceType)
		noURLSecrets(t, deviceType+" differences", result.Differences)
	}
}

func TestVerifyActions_AClientWithoutTheSurfaceIsAnError(t *testing.T) {
	_, err := verifyActions(t, newApplyTestClient(), actionsConfig(actionEntry("btn_on_url", 0, "http://a/1")), "SHSW-1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "actions")
}

func TestVerifyActions_ADeviceReadFailureIsADeviceReadError(t *testing.T) {
	client := newActionApplyClient()
	client.getErr = errors.New("timeout")

	_, err := verifyActions(t, client, actionsConfig(actionEntry("btn_on_url", 0, "http://a/1")), "SHSW-1")

	var read *deviceReadError
	require.ErrorAs(t, err, &read)
}

func TestVerifyActions_NoActionsDesiredReadsNoActions(t *testing.T) {
	client := newActionApplyClient()

	result, err := verifyActions(t, client, &DeviceConfiguration{}, "SHSW-1")

	require.NoError(t, err)
	assert.True(t, result.Match)
	assert.Equal(t, 0, client.reads, "a config with no actions must not read the route")
}

func TestCompareActions_NoDifferenceHoldsAURL(t *testing.T) {
	two := []ActionConfig{actionEntry("btn_on_url", 0, secretActionURL), actionEntry("out_on_url", 0, "http://x:pw2@h/2")}
	one := []ActionConfig{actionEntry("btn_on_url", 0, "http://x:oldpw@h/other")}
	cases := map[string][2]*DeviceConfiguration{
		"a changed URL":           {{Actions: &two}, {Actions: &one}},
		"a removed entry":         {{Actions: &two}, {Actions: &[]ActionConfig{two[0]}}},
		"the section removed":     {{Actions: &two}, {}},
		"the whole config absent": {{Actions: &two}, nil},
		"the desired config nil":  {nil, {Actions: &two}},
	}
	for name, pair := range cases {
		result := NewConfigComparator().Compare(pair[0], pair[1])
		assert.False(t, result.Match, name)
		noURLSecrets(t, name, result.Differences)
	}
}

// An action the device holds and the desired config does not list is left alone: it is not drift.
func TestCompareActions_AnEntryOnlyTheDeviceHoldsIsNotDrift(t *testing.T) {
	desired := actionsConfig(actionEntry("btn_on_url", 0, "http://a/1"))
	device := actionsConfig(actionEntry("btn_on_url", 0, "http://a/1"), actionEntry("out_on_url", 0, "http://b/1"))

	result := NewConfigComparator().Compare(desired, device)

	assert.True(t, result.Match, "differences: %+v", result.Differences)
}

func TestCompareActions_AnExactListReportsEveryURLThatDiffers(t *testing.T) {
	cases := map[string][2][]string{
		"extra device URL":       {{"http://a/1"}, {"http://a/1", "http://a/2"}},
		"a clear not kept":       {{}, {"http://a/1"}},
		"a URL the device lacks": {{"http://a/1", "http://a/2"}, {"http://a/1"}},
	}
	for name, pair := range cases {
		want, have := pair[0], pair[1]
		result := NewConfigComparator().Compare(
			actionsConfig(ActionConfig{Name: "btn_on_url", Index: 0, SecretURLs: &want}),
			actionsConfig(ActionConfig{Name: "btn_on_url", Index: 0, SecretURLs: &have}))
		assert.False(t, result.Match, name)
	}
}
