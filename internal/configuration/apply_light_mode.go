package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// RGBW2SettingsClient is POST and GET on /settings/color/<n> and /settings/white/<n>.
type RGBW2SettingsClient interface {
	SetColorSettings(ctx context.Context, channel int, settings map[string]interface{}) error
	GetColorSettings(ctx context.Context, channel int) (map[string]interface{}, error)
	SetWhiteSettings(ctx context.Context, channel int, settings map[string]interface{}) error
	GetWhiteSettings(ctx context.Context, channel int) (map[string]interface{}, error)
}

var errNoRGBW2Client = errors.New("client cannot write per-channel color and white settings")

// A white channel has no effect.
var (
	gen1RGBW2ColorKeys = []string{"name", "effect", "transition", "default_state", "auto_on", "auto_off", "schedule", "schedule_rules", "btn_type", "btn_reverse"}
	gen1RGBW2WhiteKeys = []string{"name", "transition", "default_state", "auto_on", "auto_off", "schedule", "schedule_rules", "btn_type", "btn_reverse"}
)

// effectiveLightMode is the desired mode (written first), else the reported one.
func effectiveLightMode(apiConfig map[string]interface{}, live json.RawMessage) (string, bool) {
	if want, ok := apiConfig["mode"].(string); ok {
		return want, true
	}
	return gen1LiveMode(live)
}

// checkLightModeBeforeWrite refuses, before any write, lights the mode cannot take: color has one
// channel; white has no effect, and btn_type and btn_reverse (shared) on channel 0 only.
func checkLightModeBeforeWrite(live json.RawMessage, apiConfig map[string]interface{}, deviceType string) error {
	lights := relayMaps(apiConfig["lights"])
	if deviceType != "SHRGBW2" || len(lights) == 0 {
		return nil
	}
	mode, known := effectiveLightMode(apiConfig, live)
	if !known || (mode != "color" && mode != "white") {
		return &refusedBeforeWriteError{fmt.Errorf("%w: %s: the device's mode is neither desired nor reported as color or white, so the channel endpoint is unknown and nothing was written", ErrUnsupportedSettings, gen1ModePath)}
	}
	var paths []string
	switch mode {
	case "color":
		for i := 1; i < len(lights); i++ {
			paths = append(paths, fmt.Sprintf("light.lights[%d] (the mode is color: one channel)", i))
		}
	case "white":
		for i, light := range lights {
			if _, has := light["effect"]; has {
				paths = append(paths, fmt.Sprintf("light.lights[%d].effect (the mode is white)", i))
			}
			for _, key := range []string{"btn_type", "btn_reverse"} {
				if _, has := light[key]; has && i > 0 {
					paths = append(paths, fmt.Sprintf("light.lights[%d].%s (shared by the four white channels, written on channel 0 only)", i, key))
				}
			}
		}
	}
	if len(paths) == 0 {
		return nil
	}
	return &refusedBeforeWriteError{fmt.Errorf("%w: %s", ErrUnsupportedSettings, strings.Join(paths, ", "))}
}

func applyRGBW2Lights(ctx context.Context, client ShellyClient, lights []map[string]interface{}, apiConfig map[string]interface{}, live json.RawMessage, result *ApplyResult) error {
	rgbw2, ok := client.(RGBW2SettingsClient)
	if !ok {
		return failSection(result, "lights", errNoRGBW2Client)
	}
	if mode, _ := effectiveLightMode(apiConfig, live); mode == "white" {
		return applyChannels(ctx, result, "lights", lights, gen1RGBW2WhiteKeys, rgbw2.SetWhiteSettings, rgbw2.GetWhiteSettings)
	}
	return applyChannels(ctx, result, "lights", lights, gen1RGBW2ColorKeys, rgbw2.SetColorSettings, rgbw2.GetColorSettings)
}
