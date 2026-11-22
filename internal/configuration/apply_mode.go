package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
)

const (
	gen1ModePath = "system.device.mode"

	// modeLaterSectionsWarning says why the sections after a mode that did not hold were not written.
	modeLaterSectionsWarning = "The device did not confirm the mode change, so no other section was written: the channel sections of the new mode would only be refused."
)

// errModeNotConfirmed is returned when the device accepted the mode write and did not report the mode.
var errModeNotConfirmed = errors.New("the device did not report the mode that was written")

// unsupportedMode returns the paths of a desired device mode that apply cannot write. The model must
// have the switch and take the word, and the channel sections that the other mode owns are refused:
// the device refuses them one by one after the switch, so the whole set is refused before any write.
func unsupportedMode(desired *DeviceConfiguration, deviceType string) []string {
	if desired.System == nil || desired.System.Device == nil || desired.System.Device.Mode == nil {
		return nil
	}
	mode := *desired.System.Device.Mode
	if !slices.Contains(gen1DeviceModes(deviceType), mode) {
		return []string{gen1ModePath}
	}
	switch {
	case mode == "roller" && desired.Relay != nil && len(desired.Relay.Relays) > 0:
		return []string{gen1ModePath, "relay.relays (the mode is roller)"}
	case mode == "relay" && desired.Roller != nil:
		return []string{gen1ModePath, "roller (the mode is relay)"}
	}
	return nil
}

// gen1LiveMode reads the mode from the settings the device reported before the write.
func gen1LiveMode(live json.RawMessage) (string, bool) {
	var settings map[string]interface{}
	if json.Unmarshal(live, &settings) != nil {
		return "", false
	}
	mode, ok := settings["mode"].(string)
	return mode, ok
}

// applyModeSetting writes the device mode before every other section, because the channel endpoints
// of the other mode are refused until it holds: it sends nothing when the device already reports the
// mode, and otherwise reads the mode back. Any error stops the apply (applyConfigSections), so no
// later section is written against a device that may be in the wrong mode.
func (a *ConfigApplier) applyModeSetting(ctx context.Context, client ShellyClient, apiConfig map[string]interface{}, live json.RawMessage, result *ApplyResult) error {
	want, ok := apiConfig["mode"].(string)
	if !ok {
		return nil
	}
	if have, known := gen1LiveMode(live); known && have == want {
		result.AppliedCount++
		return nil
	}

	fail := func(err error) error {
		result.Failures = append(result.Failures, ApplyFailure{Path: gen1ModePath, Value: want, Error: err.Error()})
		result.FailedCount++
		return err
	}
	if err := client.SetConfig(ctx, map[string]interface{}{"mode": want}); err != nil {
		return fail(err)
	}
	current, err := client.GetConfig(ctx)
	if err != nil {
		return fail(fmt.Errorf("could not read the mode back: %w", err))
	}
	if current == nil {
		return fail(errModeNotConfirmed)
	}
	if got, _ := gen1LiveMode(current.Raw); got != want {
		return fail(fmt.Errorf("%w: wrote %q, the device reports %q", errModeNotConfirmed, want, got))
	}
	result.AppliedCount++
	return nil
}
