package discovery

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ginsys/shelly-manager#707: discovery.enable_mdns was read nowhere, so every discovery sent an
// mDNS multicast query whatever the setting said.
func TestCombinedDiscoveryWith_MDNSSwitch(t *testing.T) {
	calls := 0
	original := mdnsDiscover
	mdnsDiscover = func(context.Context, time.Duration) ([]ShellyDevice, error) {
		calls++
		return nil, nil
	}
	t.Cleanup(func() { mdnsDiscover = original })

	// No network to scan: only the mDNS step can run.
	_, err := CombinedDiscoveryWith(context.Background(), nil, time.Second, false)
	assert.NoError(t, err)
	assert.Equal(t, 0, calls, "mDNS ran although it is switched off")

	_, err = CombinedDiscoveryWith(context.Background(), nil, time.Second, true)
	assert.NoError(t, err)
	assert.Equal(t, 1, calls, "mDNS did not run although it is switched on")
}
