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

// gen1ApplyKeys are the Gen1 settings keys that apply writes correctly today: flat parameters
// on POST /settings, the mqtt section, which is flattened to mqtt_* parameters, and the
// relays section, which is written per channel with POST /settings/relay/<n>. Every other
// section (wifi_sta, wifi_ap, cloud, coiot, login, inputs) is refused before the device is
// touched, because the applier would send it in a shape the device ignores. Wi-Fi in
// particular is never written. A relay key outside name, default_state, auto_on and auto_off
// is refused through the round-trip check below.
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
}

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
}

// unsupportedApplySettings returns the sorted top-level Gen1 keys of desired that apply cannot
// write.
func unsupportedApplySettings(converter ConfigConverter, desired *DeviceConfiguration, deviceType string) ([]string, error) {
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
		if !gen1ApplyKeys[key] {
			unsupported = append(unsupported, key)
		}
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

	// The converted configuration holds no channel id: the list position is the channel that
	// apply writes. A stated id that disagrees with its position would change another channel
	// than the one the configuration names, so refuse it before the device is touched. An id
	// left out decodes as 0 and cannot be told from a stated 0, so it is taken as the position.
	if desired.Relay != nil {
		for i, relay := range desired.Relay.Relays {
			if relay.ID != 0 && relay.ID != i {
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
	unsupported, err := unsupportedApplySettings(converter, desired, deviceType)
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
