package service

import (
	"context"
	"fmt"
	"time"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/shelly/gen1"
	"github.com/ginsys/shelly-manager/internal/shelly/gen2"
)

// identityProbeUserAgent names the identity read in a device's own log.
const identityProbeUserAgent = "shelly-manager/identity-probe"

// checkStoredIdentity refuses unless the device that answers at the record's address is the stored
// one (configuration.ErrDeviceIdentity). A stale address, for example a moved DHCP lease, leads to
// another device, and what that device does or says must not change this device's record or
// receive its commands.
//
// The read sends no login. A Gen1 /shelly and a Gen2 Shelly.GetDeviceInfo are served without one,
// even when the device requires a login elsewhere, so the answer does not depend on the login that
// the caller is about to clear or save. It costs one request, and only where a stored login is about
// to change or a command is about to be sent: an ordinary status read gains nothing.
func (s *ShellyService) checkStoredIdentity(device *database.Device) error {
	settings, err := parseClientSettings(device.Settings)
	if err != nil {
		return err
	}

	var probe configuration.IdentityReader
	switch settings.Gen {
	case 1:
		probe = gen1.NewClient(device.IP, gen1.WithUserAgent(identityProbeUserAgent))
	case 2, 3:
		probe = gen2.NewClient(device.IP, gen2.WithUserAgent(identityProbeUserAgent))
	default:
		return fmt.Errorf("unsupported device generation: %d", settings.Gen)
	}

	ctx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
	defer cancel()
	return configuration.CheckDeviceIdentity(ctx, probe, device.MAC)
}
