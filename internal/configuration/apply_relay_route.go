package configuration

import (
	"encoding/json"
	"fmt"
	"strings"
)

// gen1MaxScheduleRules is the most schedule rules a channel keeps. The Gen1 reference says "Shelly1/PM
// supports up to 18 schedule rules" and "Shelly2.5 supports up to 20 schedule rules"; the larger
// bound is used for every model, so a rule above 18 on a Shelly 1 is left to the device to refuse.
const gen1MaxScheduleRules = 20

// unsupportedRelayValues returns the paths of one relay channel whose value no device keeps: a
// negative max power, an empty schedule rule, a rule with a comma (rules are sent comma-separated)
// and more rules than gen1MaxScheduleRules.
func unsupportedRelayValues(channel int, relay SingleRelayConfig) []string {
	path := fmt.Sprintf("relay.relays[%d].", channel)
	var unsupported []string
	if relay.MaxPower != nil && *relay.MaxPower < 0 {
		unsupported = append(unsupported, path+"max_power")
	}
	bad := len(relay.ScheduleRules) > gen1MaxScheduleRules
	for _, rule := range relay.ScheduleRules {
		if rule == "" || strings.Contains(rule, ",") {
			bad = true
		}
	}
	if bad {
		unsupported = append(unsupported, path+"schedule_rules")
	}
	return unsupported
}

// routeGen1MaxPower moves the model-wide max_power of a Shelly 2.5 in relay mode onto its channels.
// The Gen1 reference says of POST /settings max_power: "Overpower threshold in Watts for roller
// mode only. For relay mode the device keeps separate, per-channel configurations", and of
// POST /settings/relay/<n> max_power: "Set per-channel overpower threshold (used in relay mode)".
// A channel that states its own value keeps it. The mode is read from the settings the device
// reported before the write; every other model, and the roller mode, is left as it is.
func routeGen1MaxPower(apiConfig map[string]interface{}, live json.RawMessage, deviceType string) {
	maxPower, ok := apiConfig["max_power"]
	if !ok || deviceType != "SHSW-25" {
		return
	}
	var liveConfig map[string]interface{}
	if err := json.Unmarshal(live, &liveConfig); err != nil || !gen1MaxPowerIsPerChannel(deviceType, liveConfig) {
		return
	}

	desired := relayMaps(apiConfig["relays"])
	channels := len(relayMaps(liveConfig["relays"]))
	if len(desired) > channels {
		channels = len(desired)
	}
	if channels == 0 {
		return
	}

	relays := make([]interface{}, channels)
	for i := range relays {
		channel := map[string]interface{}{}
		if i < len(desired) {
			for key, value := range desired[i] {
				channel[key] = value
			}
		}
		if _, own := channel["max_power"]; !own {
			channel["max_power"] = maxPower
		}
		relays[i] = channel
	}
	apiConfig["relays"] = relays
	delete(apiConfig, "max_power")
}
