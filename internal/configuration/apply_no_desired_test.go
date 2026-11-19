package configuration

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The refusal that verify gives a device with no desired configuration does not reach apply. Apply
// verifies after its writes with its own verifier, where an empty desired configuration writes
// nothing and compares nothing, and it answers as it always did (ginsys/shelly-manager#636).
func TestApplyDesiredConfig_EmptyDesiredConfigStaysANoOp(t *testing.T) {
	for _, desired := range []string{"", "{}", "null"} {
		t.Run(fmt.Sprintf("%q", desired), func(t *testing.T) {
			svc, repo := newServiceWithDesiredName(t, "x")
			repo.devices[1].DesiredConfig = desired
			dev := newFakeGen1Device()

			result, err := svc.ApplyDesiredConfig(context.Background(), 1, dev, "SHPLG-S")

			require.NoError(t, err)
			assert.True(t, result.ConfigApplied)
			assert.True(t, result.VerifyResult.Match)
			assert.Equal(t, 0, dev.setCalls, "nothing is written to the device")
		})
	}
}
