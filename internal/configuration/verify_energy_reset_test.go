package configuration

import (
	"context"
	"encoding/json"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func differencePaths(diffs []ConfigDifference) []string {
	paths := make([]string, 0, len(diffs))
	for _, d := range diffs {
		paths = append(paths, d.Path+" ("+d.Type+")")
	}
	sort.Strings(paths)
	return paths
}

// desiredWithEnergyReset merges one layer the way the service does, so the configuration verify sees
// is the merge's own output.
func desiredWithEnergyReset(t *testing.T, reset *time.Time) *DeviceConfiguration {
	t.Helper()
	merged, err := MergeConfigurations([]ConfigLayer{{
		Name: "template",
		Config: &DeviceConfiguration{
			PowerMetering: &PowerMeteringConfig{MaxPower: IntPtr(2000), EnergyReset: reset},
		},
	}})
	require.NoError(t, err)
	return merged.Config
}

// No device reports a reset time, so a desired configuration that sets one differs from every read-back.
// That holds whether the merge keeps the time or leaves a zero time in its place: the set of difference
// paths is the same, and only the expected value in the difference changes. A desired configuration with
// none matches, which shows the difference comes from this field alone.
func TestVerify_ADesiredEnergyResetIsTheSameDifferenceWhateverTheMergeKeeps(t *testing.T) {
	reset := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	const readBack = `{"max_power": 2000}`

	verify := func(t *testing.T, desired *DeviceConfiguration) *VerifyResult {
		t.Helper()
		client := newVerifyTestClient()
		client.currentRaw = json.RawMessage(readBack)
		verifier := NewConfigVerifier(NewGen1Converter(nil), nil)
		result, err := verifier.VerifyConfig(context.Background(), client, desired, "SHPLG-S")
		require.NoError(t, err)
		return result
	}

	t.Run("verify of a configuration that sets it", func(t *testing.T) {
		result := verify(t, desiredWithEnergyReset(t, &reset))

		assert.False(t, result.Match)
		assert.Equal(t, []string{"power_metering.energy_reset (removed)"}, differencePaths(result.Differences))
		t.Logf("differences: %v", differencePaths(result.Differences))
		for _, d := range result.Differences {
			t.Logf("expected value of %s: %v", d.Path, d.Expected)
		}
	})

	t.Run("apply and verify of a configuration that sets it", func(t *testing.T) {
		client := newVerifyTestClient()
		client.currentRaw = json.RawMessage(readBack)
		verifier := NewConfigVerifier(NewGen1Converter(nil), nil)

		result, err := verifier.ApplyAndVerify(context.Background(), client, desiredWithEnergyReset(t, &reset), "SHPLG-S")

		require.NoError(t, err)
		require.NotNil(t, result.VerifyResult)
		assert.False(t, result.ConfigApplied)
		assert.Equal(t, []string{"power_metering.energy_reset (removed)"}, differencePaths(result.VerifyResult.Differences))
	})

	t.Run("a configuration that does not set it matches", func(t *testing.T) {
		result := verify(t, desiredWithEnergyReset(t, nil))

		assert.True(t, result.Match)
		assert.Empty(t, result.Differences)
	})
}
