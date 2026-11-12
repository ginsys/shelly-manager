package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrUnsupportedGeneration is returned when the device generation has no
// configuration converter yet. Only Gen1 devices are supported for apply and
// verify today.
var ErrUnsupportedGeneration = errors.New("applying configuration is not supported for this device generation")

// ErrUnsupportedSettings is returned when the desired configuration holds settings that apply
// cannot write to the device yet. Nothing is sent to the device in that case.
var ErrUnsupportedSettings = errors.New("desired configuration contains settings that cannot be applied yet")

// LoginHooks connects apply to the owner of a device's stored credentials.
type LoginHooks interface {
	// Store records the login the device accepted, so the manager keeps reaching it.
	Store(ctx context.Context, deviceID uint, change LoginChange) error
	// Client returns a client that sends the device's stored credentials, none when its login
	// is off. Apply calls it once the per-device lock is held, so a queued apply does not keep a
	// client that predates the previous apply's login, and again after Store.
	Client(ctx context.Context, deviceID uint) (ShellyClient, error)
}

// gen1ApplyKeys are the Gen1 settings keys that apply writes correctly today: flat parameters
// on POST /settings, the mqtt section, which is flattened to mqtt_* parameters, and the
// relays section, which is written per channel with POST /settings/relay/<n>, and the cloud
// section, whose enabled flag is written with POST /settings/cloud, the coiot section, which is
// flattened to coiot_* parameters, and the login section, which is written with POST
// /settings/login, but only where LoginHooks is set. Every other section (wifi_sta, wifi_ap,
// inputs) is refused before the device is touched, because the applier would send it in a shape
// the device ignores. Wi-Fi in particular is never written. A relay key outside name,
// default_state, auto_on and auto_off is refused through the round-trip check below, and so is a
// cloud server, which Gen1 neither reports nor accepts.
var gen1ApplyKeys = map[string]bool{
	"name":               true,
	"timezone":           true,
	"lat":                true,
	"lng":                true,
	"eco_mode_enabled":   true,
	"discoverable":       true,
	"max_power":          true,
	"led_power_disable":  true,
	"led_status_disable": true,
	"mqtt":               true,
	"relays":             true,
	"cloud":              true,
	"coiot":              true,
	"login":              true,
}

// gen1CoIoTMinPeriod and gen1CoIoTMaxPeriod bound the CoIoT update period, in seconds, that a
// Gen1 device accepts on POST /settings (coiot_update_period).
const (
	gen1CoIoTMinPeriod = 15
	gen1CoIoTMaxPeriod = 65535
)

// gen1ApplicablePaths are the DeviceConfiguration field paths that reach gen1ApplyKeys through
// the Gen1 converter and survive its round trip, so apply writes them and verify compares them.
// "Applicable" means that apply writes the field. mqtt.pass is the one write-only exception: a
// Gen1 device never returns it, so verify skips it (defaultCompareRules in compare.go) and no
// read-back can confirm it. TestDeviceSchemaApplicableFieldsAreVerifiedOrNamed fails when a
// second applicable path is skipped by verify without being named.
// The device schema marks exactly these fields applicable; TestDeviceSchemaApplicableMatchesApply fails
// when this list and unsupportedApplySettings disagree. mqtt.port reaches the device only
// together with mqtt.server.
var gen1ApplicablePaths = map[string]bool{
	"system.device.name":         true,
	"system.device.eco_mode":     true,
	"system.device.discoverable": true,
	"location.tz":                true,
	"location.lat":               true,
	"location.lng":               true,
	"power_metering.max_power":   true,
	"led.power_indication":       true,
	"led.network_indication":     true,
	"mqtt.enable":                true,
	"mqtt.server":                true,
	"mqtt.port":                  true,
	"mqtt.user":                  true,
	"mqtt.pass":                  true,
	"mqtt.id":                    true,
	"mqtt.clean_session":         true,
	"mqtt.keep_alive":            true,
	"cloud.enable":               true,
	"coiot.enable":               true,
	"coiot.update_period":        true,
	"coiot.peer":                 true,
	"auth.enable":                true,
	"auth.user":                  true,
	"auth.pass":                  true,

	// relays are written per channel with POST /settings/relay/<n> and read back from
	// GET /settings/relay/<n>. The id is an item's position, not a setting.
	"relay.relays[].name":          true,
	"relay.relays[].default_state": true,
	"relay.relays[].auto_on":       true,
	"relay.relays[].auto_off":      true,
}

// gen1LoginMaxLength is the longest username or password POST /settings/login accepts. The
// shortest is 1. The limit is in bytes, which is the stricter reading for a multibyte character.
const gen1LoginMaxLength = 50

// unsupportedLogin returns the auth paths of a desired login that the device could not keep.
// The device takes a username and a password together, 1 to 50 characters each, and never
// returns the password. A switched-off login carries no credentials: the device keeps or drops a
// stored username as it likes, so a desired one could not be read back as written.
func unsupportedLogin(auth *AuthConfiguration) []string {
	var unsupported []string
	badLength := func(s *string) bool { return s == nil || len(*s) < 1 || len(*s) > gen1LoginMaxLength }
	switch {
	case auth.Enable == nil:
		if auth.Username != nil || auth.Password != nil {
			unsupported = append(unsupported, "auth.enable")
		}
	case *auth.Enable:
		if badLength(auth.Username) {
			unsupported = append(unsupported, "auth.user")
		}
		if badLength(auth.Password) {
			unsupported = append(unsupported, "auth.pass")
		}
	default:
		if auth.Username != nil {
			unsupported = append(unsupported, "auth.user")
		}
		if auth.Password != nil {
			unsupported = append(unsupported, "auth.pass")
		}
	}
	return unsupported
}

// unsupportedApplySettings returns the sorted top-level Gen1 keys of desired that apply cannot
// write. loginSupported says whether the caller can store new credentials (LoginHooks).
func unsupportedApplySettings(converter ConfigConverter, desired *DeviceConfiguration, deviceType string, loginSupported bool) ([]string, error) {
	apiJSON, err := converter.ToAPIConfig(desired, deviceType)
	if err != nil {
		return nil, fmt.Errorf("failed to convert desired config: %w", err)
	}
	var apiConfig map[string]interface{}
	if err = json.Unmarshal(apiJSON, &apiConfig); err != nil {
		return nil, fmt.Errorf("failed to parse converted config: %w", err)
	}

	var unsupported []string
	for key := range apiConfig {
		if !gen1ApplyKeys[key] || (key == "login" && !loginSupported) {
			unsupported = append(unsupported, key)
		}
	}
	if loginSupported && desired.Auth != nil {
		unsupported = append(unsupported, unsupportedLogin(desired.Auth)...)
	}

	// The converter drops desired settings it has no Gen1 mapping for (for example system.sntp
	// or mqtt.update_period), so they never reach apiConfig. Convert back and report every
	// populated desired field that did not survive the round trip.
	back, err := converter.FromAPIConfig(apiJSON, deviceType)
	if err != nil {
		return nil, fmt.Errorf("failed to check converted config: %w", err)
	}
	for _, diff := range NewConfigComparator().Compare(desired, back).Differences {
		if diff.Actual == nil {
			unsupported = append(unsupported, diff.Path)
		}
	}

	// The converter keeps a cloud server, but the device reports none and POST /settings/cloud takes
	// none, so apply could neither write it nor read it back.
	if desired.Cloud != nil && desired.Cloud.Server != nil {
		unsupported = append(unsupported, "cloud.server")
	}

	// The device keeps a CoIoT update period of 15 to 65535 seconds only, and reports multicast
	// as an empty peer, so the literal "mcast" could never read back as written.
	if desired.CoIoT != nil {
		if period := desired.CoIoT.UpdatePeriod; period != nil && (*period < gen1CoIoTMinPeriod || *period > gen1CoIoTMaxPeriod) {
			unsupported = append(unsupported, "coiot.update_period")
		}
		if peer := desired.CoIoT.Peer; peer != nil && *peer == gen1CoIoTMulticast {
			unsupported = append(unsupported, "coiot.peer")
		}
	}

	// The converted configuration holds no channel id: the list position is the channel that
	// apply writes. A stated id that disagrees with its position would change another channel
	// than the one the configuration names, so refuse it before the device is touched. An id
	// left out decodes as 0 and cannot be told from a stated 0, so the rule is strict: every
	// id equals its position. That also keeps the final verification honest, because the live
	// configuration numbers its channels by position.
	if desired.Relay != nil {
		for i, relay := range desired.Relay.Relays {
			if relay.ID != i {
				unsupported = append(unsupported, fmt.Sprintf("relay.relays[%d].id", i))
			}
		}
	}

	sort.Strings(unsupported)
	return unsupported, nil
}

// ApplyDesiredConfig sends the device's desired configuration to the device,
// then reads it back and compares. The device is marked config_applied only
// when the read-back matches and the desired configuration did not change
// meanwhile, so a partial, failed or outdated apply stays pending.
//
// The applied flag is cleared before the first write, so a failed re-apply of a previously
// applied device reports pending, not applied.
func (s *ConfigurationService) ApplyDesiredConfig(ctx context.Context, deviceID uint, client ShellyClient, deviceType string) (*ApplyVerifyResult, error) {
	if client.GetGeneration() != 1 {
		return nil, fmt.Errorf("%w: generation %d", ErrUnsupportedGeneration, client.GetGeneration())
	}

	// One apply per device at a time, from the snapshot to the flag update: the compare-and-set
	// below guards the stored configuration, this lock guards the order of writes to hardware.
	defer s.lockApply(deviceID)()

	hooks := s.loginHooks

	// Snapshot first: if the desired configuration changes after this read, the final
	// compare-and-set below fails and the newer configuration stays pending.
	device, err := s.repo.GetDevice(deviceID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDeviceNotFound, err)
	}
	snapshot := device.DesiredConfig

	// Apply exactly the snapshot that the compare-and-set checks. A second read could see an
	// edit that is undone before the check, so the device would hold a configuration the
	// check never saw.
	desired, err := parseApplicableDesiredConfig(snapshot)
	if err != nil {
		return nil, err
	}

	converter := NewGen1Converter(s.logger)
	unsupported, err := unsupportedApplySettings(converter, desired, deviceType, hooks != nil)
	if err != nil {
		return nil, err
	}
	if len(unsupported) > 0 {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedSettings, strings.Join(unsupported, ", "))
	}

	if err = s.repo.SetDeviceConfigApplied(deviceID, false); err != nil {
		return nil, fmt.Errorf("could not mark configuration pending before applying: %w", err)
	}

	verifier := NewConfigVerifier(converter, s.logger)
	if hooks != nil {
		// The client handed in may predate a login that an apply queued ahead of this one
		// changed, so the lock holder builds its own from the stored login.
		fresh, clientErr := hooks.Client(ctx, deviceID)
		if clientErr != nil {
			return nil, fmt.Errorf("could not build a client for the device: %w", clientErr)
		}
		client = fresh

		// Order on success: the device accepted the write, then the new credentials are stored,
		// then the client is rebuilt. A failed write reaches none of it.
		verifier.WithLogin(func(ctx context.Context, client ShellyClient, change LoginChange) (ShellyClient, error) {
			if storeErr := hooks.Store(ctx, deviceID, change); storeErr != nil {
				return nil, fmt.Errorf("the device accepted the new login but it could not be stored, so the stored login is now stale; enter it again: %w", storeErr)
			}
			rebuilt, reconnectErr := hooks.Client(ctx, deviceID)
			if reconnectErr != nil {
				return nil, fmt.Errorf("the new login was stored but the client for it could not be built: %w", reconnectErr)
			}
			return rebuilt, nil
		})
	}
	result, err := verifier.ApplyAndVerify(ctx, client, desired, deviceType)
	if err != nil {
		return nil, err
	}

	if result.ConfigApplied {
		set, err := s.repo.SetDeviceConfigAppliedIfDesired(deviceID, snapshot)
		if err != nil {
			return nil, fmt.Errorf("configuration applied but could not record it: %w", err)
		}
		if !set {
			result.ConfigApplied = false
			if result.ApplyResult != nil {
				result.ApplyResult.Warnings = append(result.ApplyResult.Warnings,
					"The desired configuration changed while it was being applied; the device stays pending.")
			}
		}
	}

	return result, nil
}

// VerifyDesiredConfig reads the live configuration from the device and
// compares it with the desired configuration. It changes nothing on the device.
func (s *ConfigurationService) VerifyDesiredConfig(ctx context.Context, deviceID uint, client ShellyClient, deviceType string) (*VerifyResult, error) {
	if client.GetGeneration() != 1 {
		return nil, fmt.Errorf("%w: generation %d", ErrUnsupportedGeneration, client.GetGeneration())
	}

	device, err := s.repo.GetDevice(deviceID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDeviceNotFound, err)
	}
	desired, err := parseApplicableDesiredConfig(device.DesiredConfig)
	if err != nil {
		return nil, err
	}

	verifier := NewConfigVerifier(NewGen1Converter(s.logger), s.logger)
	return verifier.VerifyOnly(ctx, client, desired, deviceType)
}
