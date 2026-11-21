package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/shelly"
)

// A saved login that the device refuses, followed by an identity read that fails, used to return the
// identity error alone, and the refusal was lost (ginsys/shelly-manager#759). Both clear sites keep
// it: getClientWithRetry (reached by status, energy and control through getClient) and
// getClientWithAuthRetry (reached by the configuration reads).
func TestGetClient_AFailedIdentityReadKeepsTheRefusedLogin(t *testing.T) {
	const saved = `{"gen":1,"auth_enabled":true,"auth_user":"old","auth_pass":"oldpw"}`
	sites := map[string]func(s *ShellyService, d *database.Device) error{
		"getClientWithRetry": func(s *ShellyService, d *database.Device) error {
			_, err := s.getClientWithRetry(d, true)
			return err
		},
		"getClientWithAuthRetry": func(s *ShellyService, d *database.Device) error {
			_, err := s.getClientWithAuthRetry(d)
			return err
		},
	}
	for name, call := range sites {
		t.Run(name, func(t *testing.T) {
			dev := &identityDevice{mac: identityReportedMAC, login: "other:otherpw", strictInfo: true,
				probeArrived: make(chan struct{}), probeHold: make(chan struct{})}
			service, device := identityFixture(t, dev, identityStoredMAC, saved)
			t.Cleanup(func() { close(dev.probeHold) })

			err := call(service, device)

			if !shelly.IsAuthError(err) {
				t.Errorf("got %v, want the refused login to stay in the error", err)
			}
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("got %v, want the identity read's timeout to stay in the error too", err)
			}
			if err != nil && !strings.HasPrefix(err.Error(), "the login was rejected and is left as it is") {
				t.Errorf("got %q, want the refusal named first", err)
			}
			if user, _ := storedLogin(t, service, device.ID); user != "old" {
				t.Errorf("the stored login is %q, want it kept while the identity is unknown", user)
			}
		})
	}
}

// ErrDeviceIdentity has three causes: another MAC answered, the record has no MAC, the device
// reported none. Each keeps today's text and is not a refused login: the identity handlers own that
// answer, and the text already says the login was rejected and names the MAC as the fix.
func TestGetClient_AnIdentityRefusalKeepsItsOwnAnswer(t *testing.T) {
	const saved = `{"gen":1,"auth_enabled":true,"auth_user":"old","auth_pass":"oldpw"}`
	cases := []struct {
		name      string
		storedMAC string
		reported  string
	}{
		{"another MAC", identityStoredMAC, identityOtherMAC},
		{"no stored MAC", "", identityReportedMAC},
		{"the device reports no MAC", identityStoredMAC, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dev := &identityDevice{mac: tc.reported, login: "other:otherpw", strictInfo: true}
			service, device := identityFixture(t, dev, tc.storedMAC, saved)

			_, err := service.getClientWithRetry(device, true)

			if !errors.Is(err, configuration.ErrDeviceIdentity) {
				t.Fatalf("got %v, want an error wrapping ErrDeviceIdentity", err)
			}
			if shelly.IsAuthError(err) {
				t.Errorf("got %v, want no refused-login answer for an identity refusal", err)
			}
			if want := "the login was rejected and is left as it is: "; !strings.HasPrefix(err.Error(), want) {
				t.Errorf("got %q, want it to start with %q", err, want)
			}
		})
	}
}

func TestLoginRefusedBy_NamesTheRefusalFirst(t *testing.T) {
	refusal := shelly.ErrAuthFailed
	idErr := context.DeadlineExceeded

	err := loginRefusedBy(refusal, idErr)

	if !errors.Is(err, refusal) || !errors.Is(err, idErr) {
		t.Fatalf("got %v, want both errors reachable", err)
	}
	const want = "the login was rejected and is left as it is (" // the refusal, then the identity failure
	if !strings.HasPrefix(err.Error(), want) {
		t.Errorf("got %q, want it to start with %q", err, want)
	}
	if !strings.Contains(err.Error(), "the identity check then failed: ") {
		t.Errorf("got %q, want the identity failure named after the refusal", err)
	}
}
