package configuration

import (
	"encoding/json"
	"fmt"
)

// The Gen1 Dimmer 1/2 /settings page (read, not confirmed on a device): transition 0 to 5000 ms,
// fade_rate 1 to 5, min_brightness 0 to 50, zcross_debounce 50 to 150 microseconds, pulse_mode 1
// (leading edge) or 2 (trailing edge). The page's example shows fade_rate 0 and pulse_mode 0 on a
// device that has not been set, values outside those ranges, so a desired 0 for either is refused.
const (
	gen1DimmerMaxTransition      = 5000
	gen1DimmerMinFadeRate        = 1
	gen1DimmerMaxFadeRate        = 5
	gen1DimmerMaxMinBrightness   = 50
	gen1DimmerMinZcrossDebounce  = 50
	gen1DimmerMaxZcrossDebounce  = 150
	gen1DimmerLeadingEdge        = 1
	gen1DimmerTrailingEdge       = 2
	gen1DimmerPulseModeDetectKey = "pulse_mode_detected"
)

// gen1DimmerKeys are the flat parameters of POST /settings that a Dimmer's dimming section writes.
// calibrate and calibrate_cancel are actions, not settings, and pulse_mode_detected only accepts
// false to clear the flag, so none of them is a key here. Whether a pulse_mode change needs a reboot
// is unconfirmed on hardware: the page names none, so none is owed.
var gen1DimmerKeys = []string{"min_brightness", "fade_rate", "transition", "zcross_debounce", "pulse_mode"}

// unsupportedDimmingValues returns the paths of a desired dimming section whose value the page does
// not accept. Fields with no Gen1 parameter are reported by the round trip instead.
func unsupportedDimmingValues(d *DimmingConfig) []string {
	var unsupported []string
	outside := func(v *int, min, max int) bool { return v != nil && (*v < min || *v > max) }
	if outside(d.TransitionTime, 0, gen1DimmerMaxTransition) {
		unsupported = append(unsupported, "dimming.transition")
	}
	if outside(d.FadeRate, gen1DimmerMinFadeRate, gen1DimmerMaxFadeRate) {
		unsupported = append(unsupported, "dimming.fade_rate")
	}
	if outside(d.MinBrightness, 0, gen1DimmerMaxMinBrightness) {
		unsupported = append(unsupported, "dimming.min_brightness")
	}
	if outside(d.ZcrossDebounce, gen1DimmerMinZcrossDebounce, gen1DimmerMaxZcrossDebounce) {
		unsupported = append(unsupported, "dimming.zcross_debounce")
	}
	return unsupported
}

// checkPulseModeBeforeWrite refuses, before any section is written, a pulse_mode the device has not
// said it can take. The page does not say what an absent or false pulse_mode_detected permits, so the
// manager does not leave the write to the device: it is allowed only when GET /settings reports
// pulse_mode_detected as true. The page marks that flag "Only Dimmer 2", so a
// Dimmer 1 never reports it and a desired leading_edge is always refused there, even when it equals
// the live pulse_mode. The check reads the one live read apply made and nothing else.
func checkPulseModeBeforeWrite(live json.RawMessage, haveLive bool, readErr error, apiConfig map[string]interface{}) error {
	if _, wanted := apiConfig["pulse_mode"]; !wanted {
		return nil
	}
	if !haveLive {
		return &refusedBeforeWriteError{fmt.Errorf("could not read the device's settings, nothing was written: %w", readErr)}
	}
	var doc map[string]interface{}
	_ = json.Unmarshal(live, &doc)
	if detected, _ := doc[gen1DimmerPulseModeDetectKey].(bool); !detected {
		return &refusedBeforeWriteError{fmt.Errorf("%w: dimming.leading_edge (pulse_mode): the device does not report %s as true, so nothing was written",
			ErrUnsupportedSettings, gen1DimmerPulseModeDetectKey)}
	}
	return nil
}
