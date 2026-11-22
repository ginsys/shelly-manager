package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
)

// Ranges of the Dimmer's night_mode and warm_up endpoint pages. The changelog's 1000 ms warm-up
// maximum is not in the endpoint table, so a desired write stops at 200.
const (
	gen1DimmerMinNightBrightness  = 1
	gen1DimmerMaxNightBrightness  = 100
	gen1DimmerMinWarmupBrightness = 10
	gen1DimmerMaxWarmupBrightness = 100
	gen1DimmerMinWarmupTime       = 20
	gen1DimmerMaxWarmupTime       = 200
)

const gen1ClockPattern = `^([01][0-9]|2[0-3]):[0-5][0-9]$`

var gen1ClockTime = regexp.MustCompile(gen1ClockPattern)

// unsupportedNightWarmup returns the paths of a desired night mode and warm-up the page does not
// accept. A warm-up that is switched off is checked for its enabled flag only.
func unsupportedNightWarmup(d *DimmingConfig) []string {
	var unsupported []string
	outside := func(v *int, min, max int) bool { return v != nil && (*v < min || *v > max) }
	notClock := func(s *string) bool { return s != nil && !gen1ClockTime.MatchString(*s) }
	for path, bad := range map[string]bool{
		"dimming.night_mode_start":      notClock(d.NightModeStart),
		"dimming.night_mode_end":        notClock(d.NightModeEnd),
		"dimming.night_mode_brightness": outside(d.NightModeBrightness, gen1DimmerMinNightBrightness, gen1DimmerMaxNightBrightness),
	} {
		if bad {
			unsupported = append(unsupported, path)
		}
	}
	if d.WarmupEnabled != nil && !*d.WarmupEnabled {
		return unsupported
	}
	if outside(d.WarmupBrightness, gen1DimmerMinWarmupBrightness, gen1DimmerMaxWarmupBrightness) {
		unsupported = append(unsupported, "dimming.warmup_brightness")
	}
	if outside(d.WarmupTime, gen1DimmerMinWarmupTime, gen1DimmerMaxWarmupTime) {
		unsupported = append(unsupported, "dimming.warmup_time")
	}
	return unsupported
}

// DimmerEndpointsClient is POST on /settings/night_mode and /settings/warm_up.
type DimmerEndpointsClient interface {
	SetNightMode(ctx context.Context, settings map[string]interface{}) error
	SetWarmUp(ctx context.Context, settings map[string]interface{}) error
}

var errNoDimmerEndpointsClient = errors.New("client cannot write night mode and warm-up")

// applyDimmerEndpoints posts each object to its endpoint, then reads GET /settings once and compares
// every key it wrote. A warm-up that is switched off writes its enabled flag only.
func (a *ConfigApplier) applyDimmerEndpoints(ctx context.Context, client ShellyClient, apiConfig map[string]interface{}, result *ApplyResult) error {
	night, _ := apiConfig["night_mode"].(map[string]interface{})
	warm, _ := apiConfig["warm_up"].(map[string]interface{})
	if enabled, ok := warm["enabled"].(bool); ok && !enabled {
		result.SettingsCount -= len(warm) - 1 // the keys dropped were counted before any section ran
		warm = map[string]interface{}{"enabled": false}
	}
	if len(night) == 0 && len(warm) == 0 {
		return nil
	}
	endpoints, ok := client.(DimmerEndpointsClient)
	if !ok {
		return failSection(result, "dimming", errNoDimmerEndpointsClient)
	}

	var firstErr error
	written := map[string]map[string]interface{}{}
	for _, w := range []struct {
		object string
		form   map[string]interface{}
		set    func(context.Context, map[string]interface{}) error
	}{{"night_mode", night, endpoints.SetNightMode}, {"warm_up", warm, endpoints.SetWarmUp}} {
		if len(w.form) == 0 {
			continue
		}
		if err := w.set(ctx, w.form); err != nil {
			failObject(result, w.object, w.form, err.Error())
			firstErr = firstError(firstErr, err)
			continue
		}
		written[w.object] = w.form
	}
	if len(written) == 0 {
		return firstErr
	}

	var live map[string]interface{}
	config, err := client.GetConfig(ctx)
	if err == nil {
		err = json.Unmarshal(config.Raw, &live)
	}
	if err != nil {
		for object, form := range written {
			failObject(result, object, form, "read-back failed: "+err.Error())
		}
		return firstError(firstErr, err)
	}
	for _, object := range []string{"night_mode", "warm_up"} {
		reported, _ := live[object].(map[string]interface{})
		for _, key := range sortedKeys(written[object]) {
			if relayValuesEqual(written[object][key], reported[key]) {
				result.AppliedCount++
				continue
			}
			result.Failures = append(result.Failures, ApplyFailure{
				Path:  object + "." + key,
				Value: fmt.Sprintf("%v", written[object][key]),
				Error: fmt.Sprintf("device reports %v after the write", reported[key]),
			})
			result.FailedCount++
		}
	}
	return firstErr
}

// withoutDisabledWarmupValues is desired as verify compares it: a warm-up that is switched off
// wrote its enabled flag only.
func withoutDisabledWarmupValues(desired *DeviceConfiguration) *DeviceConfiguration {
	if desired == nil || desired.Dimming == nil || desired.Dimming.WarmupEnabled == nil || *desired.Dimming.WarmupEnabled {
		return desired
	}
	shown := *desired
	dimming := *desired.Dimming
	dimming.WarmupBrightness, dimming.WarmupTime = nil, nil
	shown.Dimming = &dimming
	return &shown
}

func failObject(result *ApplyResult, object string, form map[string]interface{}, message string) {
	for _, key := range sortedKeys(form) {
		result.Failures = append(result.Failures, ApplyFailure{
			Path:  object + "." + key,
			Value: fmt.Sprintf("%v", form[key]),
			Error: message,
		})
		result.FailedCount++
	}
}
