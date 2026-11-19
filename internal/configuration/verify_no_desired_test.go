package configuration

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// A device with no desired configuration has nothing to compare with. Verify refuses it, as drift
// does, and never reads the device: "match" would read as a device that was checked and found
// right (ginsys/shelly-manager#636).
func TestVerifyDesiredConfig_NoDesiredConfigIsRefused(t *testing.T) {
	for _, desired := range []string{"", "{}", "null", "  {}  ", "{ }", "{\n}", " null\n"} {
		t.Run(fmt.Sprintf("%q", desired), func(t *testing.T) {
			service, _, _ := newDriftService(t, desired)
			client := driftClient("anything", driftTestMAC)

			result, err := service.ConfigurationSvc.VerifyDesiredConfig(context.Background(), 1, client, "SHSW-1")

			assert.Nil(t, result)
			assert.ErrorIs(t, err, ErrNoDesiredConfig)
			client.AssertNotCalled(t, "GetConfig", mock.Anything)
		})
	}
}

// A set desired configuration still verifies: a match when the device holds it, the differences
// when it does not.
func TestVerifyDesiredConfig_SetDesiredConfigStillVerifies(t *testing.T) {
	service, _, _ := newDriftService(t, desiredNamed(t, "tv-office"))

	result, err := service.ConfigurationSvc.VerifyDesiredConfig(context.Background(), 1, driftClient("tv-office", driftTestMAC), "SHSW-1")
	require.NoError(t, err)
	assert.True(t, result.Match)

	result, err = service.ConfigurationSvc.VerifyDesiredConfig(context.Background(), 1, driftClient("other", driftTestMAC), "SHSW-1")
	require.NoError(t, err)
	assert.False(t, result.Match)
	assert.NotEmpty(t, result.Differences)
}
