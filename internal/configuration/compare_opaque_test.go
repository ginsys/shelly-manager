package configuration

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A struct with no exported field, such as time.Time, has nothing the comparator can walk into.
// It is one value: ginsys/shelly-manager#567.

func configWithEnergyReset(reset *time.Time) *DeviceConfiguration {
	return &DeviceConfiguration{PowerMetering: &PowerMeteringConfig{EnergyReset: reset}}
}

func TestCompare_TwoDifferentEnergyResetTimesAreADifference(t *testing.T) {
	first := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	second := first.Add(24 * time.Hour)

	result := NewConfigComparator().Compare(configWithEnergyReset(&first), configWithEnergyReset(&second))

	assert.False(t, result.Match)
	assert.Equal(t, []string{"power_metering.energy_reset (modified)"}, differencePaths(result.Differences))
}

// The kept half of the issue: no device reports a reset time and apply refuses one, so a desired
// reset time against a read-back without it stays a difference.
func TestCompare_ADesiredEnergyResetAgainstNoneStaysRemoved(t *testing.T) {
	reset := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	result := NewConfigComparator().Compare(configWithEnergyReset(&reset), configWithEnergyReset(nil))

	assert.False(t, result.Match)
	assert.Equal(t, []string{"power_metering.energy_reset (removed)"}, differencePaths(result.Differences))
}

// The same instant is not a difference, whatever location it is written in. A DeepEqual of the two
// times would say otherwise, so this fails a comparison by DeepEqual alone.
func TestCompare_TheSameInstantInTwoLocationsIsNoDifference(t *testing.T) {
	utc := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	plusFive := utc.In(time.FixedZone("plus5", 5*60*60))
	require.False(t, reflect.DeepEqual(utc, plusFive), "the fixture must differ for DeepEqual, or this proves nothing")

	result := NewConfigComparator().Compare(configWithEnergyReset(&utc), configWithEnergyReset(&plusFive))

	assert.True(t, result.Match)
	assert.Empty(t, result.Differences)
}

// A struct with no exported field that is not a time is compared as one value too.
func TestCompareFields_AnOpaqueStructIsOneValue(t *testing.T) {
	type opaque struct{ n int }

	compare := func(expected, actual opaque) *CompareResult {
		result := &CompareResult{Match: true, Differences: []ConfigDifference{}}
		NewConfigComparator().compareFields(reflect.ValueOf(expected), reflect.ValueOf(actual), "value", nil, result)
		return result
	}

	different := compare(opaque{n: 1}, opaque{n: 2})
	assert.False(t, different.Match)
	assert.Equal(t, []string{"value (modified)"}, differencePaths(different.Differences))

	same := compare(opaque{n: 1}, opaque{n: 1})
	assert.True(t, same.Match)
	assert.Empty(t, same.Differences)
}
