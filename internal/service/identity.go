package service

import (
	"context"
	"errors"
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
// the caller is about to clear or save. It is one request with no retry, and it costs that request
// only where a stored login is about to change or a command is about to be sent: an ordinary status
// read gains nothing. It never runs under clientMu, which every device shares.
//
// The client is built for this one read and closes its connection when the read is done: its
// transport is private, and an idle connection would outlive the call.
func (s *ShellyService) checkStoredIdentity(device *database.Device) error {
	settings, err := parseClientSettings(device.Settings)
	if err != nil {
		return err
	}

	var probe configuration.IdentityReader
	switch settings.Gen {
	case 1:
		client := gen1.NewClient(device.IP, gen1.WithUserAgent(identityProbeUserAgent), gen1.WithRetry(0, 0))
		defer client.CloseIdleConnections()
		probe = client
	case 2, 3:
		client := gen2.NewClient(device.IP, gen2.WithUserAgent(identityProbeUserAgent), gen2.WithRetry(0, 0))
		defer client.CloseIdleConnections()
		probe = client
	default:
		return fmt.Errorf("unsupported device generation: %d", settings.Gen)
	}

	ctx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
	defer cancel()
	return configuration.CheckDeviceIdentity(ctx, probe, device.MAC)
}

// loginRefusedBy is the error of a refused saved login whose identity read then failed. The refusal
// comes first, so a caller that classifies by it (shelly.IsAuthError) still sees a device that
// rejects the login, and the failed read stays reachable with errors.Is.
//
// A configuration.ErrDeviceIdentity keeps the identity text alone. It has three causes: another
// MAC answered, the record has no MAC, the device reported none. The text already says the login
// was rejected and names the fix, setting the MAC. The identity handlers own that answer, and only
// the first cause means that some other device refused the login.
func loginRefusedBy(refusal, idErr error) error {
	if errors.Is(idErr, configuration.ErrDeviceIdentity) {
		return fmt.Errorf("the login was rejected and is left as it is: %w", idErr)
	}
	return fmt.Errorf("the login was rejected and is left as it is (%w); the identity check then failed: %w", refusal, idErr)
}

// mayAdoptConfigLogin reports whether a client built for this record could end by saving the global
// login into it: the device wants a login, the record holds none and the configuration supplies one.
// The builder reads the identity before it takes clientMu when this is true, and saves nothing
// without that read.
func (s *ShellyService) mayAdoptConfigLogin(device *database.Device) bool {
	settings, err := parseClientSettings(device.Settings)
	if err != nil {
		return false
	}
	return settings.AuthEnabled && (settings.AuthUser == "" || settings.AuthPass == "") && s.Config.Provisioning.AuthEnabled
}
