package configuration

import (
	"encoding/json"
	"fmt"
	"strings"
)

// checkInputModeBeforeWrite refuses, before any write, a desired input on a Shelly 2.5 whose mode is
// roller. The page lists btn_type and btn_reverse of /settings/relay/<n> under relay mode, says input
// events are only detected there, and gives the roller its own btn_type, btn_reverse, swap_inputs and
// input_mode, so an input written to the relay endpoint is not the roller's input. What the device
// does with it is unconfirmed on hardware. The mode is the desired one, else the reported one.
func checkInputModeBeforeWrite(live json.RawMessage, apiConfig map[string]interface{}, deviceType string) error {
	inputs := relayMaps(apiConfig["inputs"])
	if !HasRollerMode(deviceType) || len(inputs) == 0 {
		return nil
	}
	if mode, _ := effectiveLightMode(apiConfig, live); mode != "roller" {
		return nil
	}
	paths := make([]string, len(inputs))
	for i := range inputs {
		paths[i] = fmt.Sprintf("input.inputs[%d]", i)
	}
	return &refusedBeforeWriteError{fmt.Errorf("%w: %s (the mode is roller: the inputs belong to the roller, set roller.button_type, roller.button_reverse, roller.swap_inputs and roller.input_mode instead), so nothing was written",
		ErrUnsupportedSettings, strings.Join(paths, ", "))}
}
