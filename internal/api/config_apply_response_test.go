package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/configuration"
)

func TestConfigApplyResponse_UnconfirmedIsItsOwnState(t *testing.T) {
	resp := configApplyResponse(&configuration.ApplyVerifyResult{
		ConfigApplied: false,
		ApplyResult: &configuration.ApplyResult{
			Success:     true,
			Unconfirmed: true,
			Warnings:    []string{"Applied, unconfirmed: the device may have changed network and has not answered since"},
		},
	})

	assert.True(t, resp.Unconfirmed)
	assert.False(t, resp.Success, "nothing was verified, so it is not a success")
	assert.Zero(t, resp.FailedCount)
	assert.Empty(t, resp.Failures, "and nothing was refused, so it is not a failure")

	raw, err := json.Marshal(resp)
	require.NoError(t, err)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &body))
	assert.Equal(t, true, body["unconfirmed"])
	assert.Equal(t, false, body["success"])
}

func TestConfigApplyResponse_AVerifiedApplyIsNotUnconfirmed(t *testing.T) {
	resp := configApplyResponse(&configuration.ApplyVerifyResult{
		ConfigApplied: true,
		ApplyResult:   &configuration.ApplyResult{Success: true, AppliedCount: 3},
	})

	assert.False(t, resp.Unconfirmed)
	assert.True(t, resp.Success)
	assert.Equal(t, 3, resp.AppliedCount)
}

// A login write whose answer never arrived is told apart from a refusal by a warning, because a
// failure reaches the caller as a fixed text that names the setting only.
func TestConfigApplyResponse_ALostLoginAnswerIsAWarningNotAFailureText(t *testing.T) {
	const warning = "The device gave no answer to the login write; it may have switched its login, so the stored login may now be stale"
	resp := configApplyResponse(&configuration.ApplyVerifyResult{
		ApplyResult: &configuration.ApplyResult{
			FailedCount: 1,
			Failures:    []configuration.ApplyFailure{{Path: "auth", Value: "enable", Error: "no answer; the stored login may now be stale"}},
			Warnings:    []string{warning},
		},
	})

	assert.Equal(t, []string{"auth: the device refused the write"}, resp.Failures)
	assert.Equal(t, []string{warning}, resp.Warnings)
}
