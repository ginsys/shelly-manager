package configuration

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ginsys/shelly-manager/internal/shelly"
)

// Gen2ConfigClient is what the Gen2 apply needs from a client: one <Component>.SetConfig call that
// returns the restart_required answer of the device. *gen2.Client implements it. A client that
// does not stays refused with ErrUnsupportedGeneration, whatever generation it reports.
type Gen2ConfigClient interface {
	CallConfigSetter(ctx context.Context, method string, params map[string]interface{}) (restartRequired bool, err error)
}

// gen2Sections lists the sections of the Gen2 config document that apply writes, in order, with
// the RPC method that writes each. Later slices add their sections here.
var gen2Sections = []struct {
	key    string
	method string
}{
	{"sys", "Sys.SetConfig"},
	{"mqtt", "MQTT.SetConfig"},
	{"cloud", "Cloud.SetConfig"},
}

// gen2ApplyKeys are the fixed top-level keys of a Gen2 config document that apply writes. The
// channel sections switch:N and input:N are written too (gen2KeyApplies).
var gen2ApplyKeys = map[string]bool{"sys": true, "mqtt": true, "cloud": true}

// gen2Write is one SetConfig call of an apply: the section key, the method and its parameters.
type gen2Write struct {
	key    string
	method string
	params map[string]interface{}
	body   map[string]interface{}
}

// gen2WritesOf lists the calls an apply makes, in order: the fixed sections, then each input, then
// each switch, by id. An input comes before the switch it drives, because the type of the input
// must be set before the in_mode of the switch (follow and flip need an input that has a state).
func gen2WritesOf(apiConfig map[string]interface{}) []gen2Write {
	var writes []gen2Write
	for _, section := range gen2Sections {
		if body, ok := apiConfig[section.key].(map[string]interface{}); ok && len(body) > 0 {
			writes = append(writes, gen2Write{key: section.key, method: section.method, params: map[string]interface{}{"config": body}, body: body})
		}
	}
	for _, channel := range []struct{ kind, method string }{{"input", "Input.SetConfig"}, {"switch", "Switch.SetConfig"}} {
		for _, id := range gen2ChannelIDs(apiConfig, channel.kind) {
			key := channel.kind + ":" + strconv.Itoa(id)
			if body, ok := apiConfig[key].(map[string]interface{}); ok && len(body) > 0 {
				writes = append(writes, gen2Write{key: key, method: channel.method, params: map[string]interface{}{"id": id, "config": body}, body: body})
			}
		}
	}
	return writes
}

// applyCheck returns the paths of a desired configuration that apply cannot write.
type applyCheck func(desired *DeviceConfiguration, deviceType string) ([]string, error)

// applyPathFor picks the converter and the refusal check for a client. Generation 1 takes the
// Gen1 path. A later generation takes the Gen2 path only when the client can call the
// <Component>.SetConfig methods; any other client is refused before anything is written.
func (s *ConfigurationService) applyPathFor(client ShellyClient, loginSupported bool) (ConfigConverter, applyCheck, error) {
	switch generation := client.GetGeneration(); {
	case generation == 1:
		converter := NewGen1Converter(s.logger)
		return converter, func(desired *DeviceConfiguration, deviceType string) ([]string, error) {
			return unsupportedApplySettings(converter, desired, deviceType, loginSupported)
		}, nil
	case generation >= 2:
		if _, ok := client.(Gen2ConfigClient); ok {
			converter := NewGen2Converter(s.logger)
			return converter, func(desired *DeviceConfiguration, _ string) ([]string, error) {
				return unsupportedGen2Settings(converter, desired)
			}, nil
		}
	}
	return nil, nil, fmt.Errorf("%w: generation %d", ErrUnsupportedGeneration, client.GetGeneration())
}

// unsupportedGen2Settings returns the sorted paths of desired that the Gen2 apply cannot write
// and read back. Only the sys, mqtt, cloud, switch and input sections have a mapping so far, so
// every other populated setting is reported through the round trip: the converter drops it, and it
// is missing on the way back. An MQTT password with no user is refused too: the device takes the
// password only together with the user.
func unsupportedGen2Settings(converter ConfigConverter, desired *DeviceConfiguration) ([]string, error) {
	// A setting with no key, or a value no Gen2 word names, is named on its own: left in, the round
	// trip reports the whole section.
	unsupported, mapped := unmappedMQTTSettings(desired.MQTT)
	channelPaths, checkedChannels := unmappedChannelSettings(desired)
	unsupported = append(unsupported, channelPaths...)
	checked := *checkedChannels
	checked.MQTT = mapped

	apiJSON, err := converter.ToAPIConfig(&checked, "")
	if err != nil {
		return nil, fmt.Errorf("failed to convert desired config: %w", err)
	}
	var apiConfig map[string]interface{}
	if err = json.Unmarshal(apiJSON, &apiConfig); err != nil {
		return nil, fmt.Errorf("failed to parse converted config: %w", err)
	}

	for key := range apiConfig {
		if !gen2KeyApplies(key) {
			unsupported = append(unsupported, key)
		}
	}

	back, err := converter.FromAPIConfig(apiJSON, "")
	if err != nil {
		return nil, fmt.Errorf("failed to check converted config: %w", err)
	}
	// Channels pair by id, as verify does: a desired list may skip an id or sit in any order.
	for _, diff := range NewChannelIDConfigComparator().Compare(&checked, back).Differences {
		if diff.Actual == nil {
			unsupported = append(unsupported, diff.Path)
		}
	}

	// The device takes any string as a time server, so a typo would silently stop its clock.
	// The Gen1 rule applies: a plain host name or IPv4 address. A host:port form is refused too.
	if desired.System != nil && desired.System.SNTP != nil && desired.System.SNTP.Server != "" && !validTimeServer(desired.System.SNTP.Server) {
		unsupported = append(unsupported, "system.sntp.server")
	}

	if desired.MQTT != nil && gen2PasswordIsSet(desired.MQTT) && desired.MQTT.User == nil {
		unsupported = append(unsupported, "mqtt.pass")
	}
	// The device stores one host:port string. A server that has a port and a separate port would be
	// written as host:port:port. A bracketed IPv6 endpoint in one string is read back split, and the
	// verify step shared with Gen1 does not split it, so IPv6 takes a host and a port apart.
	if m := desired.MQTT; m != nil && m.Server != nil {
		if host, _, splitErr := net.SplitHostPort(*m.Server); splitErr == nil && (m.Port != nil || strings.Contains(host, ":")) {
			unsupported = append(unsupported, "mqtt.server")
		}
	}

	sort.Strings(unsupported)
	out := unsupported[:0]
	for i, path := range unsupported {
		if i == 0 || path != unsupported[i-1] {
			out = append(out, path)
		}
	}
	return out, nil
}

// unmappedMQTTSettings returns the paths of the MQTT settings that MQTT.SetConfig has no key for
// here, and the rest of the section, or nil when nothing else is set.
func unmappedMQTTSettings(m *MQTTConfiguration) ([]string, *MQTTConfiguration) {
	if m == nil {
		return nil, nil
	}
	rest := *m
	var paths []string
	if rest.CleanSession != nil {
		paths, rest.CleanSession = append(paths, "mqtt.clean_session"), nil
	}
	if rest.KeepAlive != nil {
		paths, rest.KeepAlive = append(paths, "mqtt.keep_alive"), nil
	}
	if rest.MaxQueuedMessages != nil {
		paths, rest.MaxQueuedMessages = append(paths, "mqtt.max_qos_msgs"), nil
	}
	if rest.RetainPeriod != nil {
		paths, rest.RetainPeriod = append(paths, "mqtt.retain"), nil
	}
	if rest.UpdatePeriod != nil {
		paths, rest.UpdatePeriod = append(paths, "mqtt.update_period"), nil
	}
	if rest.UseClientCert != nil {
		paths, rest.UseClientCert = append(paths, "mqtt.ssl_ca"), nil
	}
	if rest.EnableSNI != nil {
		paths, rest.EnableSNI = append(paths, "mqtt.enable_sni"), nil
	}
	if rest == (MQTTConfiguration{}) {
		return paths, nil
	}
	return paths, &rest
}

// applyGen2Config applies a configuration to a Gen2 or later device. It has its own flow because
// the device decides whether a write needs a reboot: the Gen1 reboot rules do not apply, and a
// reboot happens only when an answer says restart_required.
func (a *ConfigApplier) applyGen2Config(ctx context.Context, client ShellyClient, config *DeviceConfiguration, deviceType string, result *ApplyResult, startTime time.Time) (*ApplyResult, error) {
	setter, ok := client.(Gen2ConfigClient)
	if !ok {
		return nil, fmt.Errorf("%w: the client cannot write Gen2 settings", ErrUnsupportedGeneration)
	}

	apiJSON, err := a.converter.ToAPIConfig(config, deviceType)
	if err != nil {
		return nil, fmt.Errorf("failed to convert config to API format: %w", err)
	}
	var apiConfig map[string]interface{}
	if err := json.Unmarshal(apiJSON, &apiConfig); err != nil {
		return nil, fmt.Errorf("failed to parse API config: %w", err)
	}
	for _, write := range gen2WritesOf(apiConfig) {
		result.SettingsCount += len(gen2Leaves(write.key, write.body))
	}

	// A reboot an earlier apply left owed stays owed until a reboot succeeds.
	if a.owed != nil && a.owed.pending {
		result.RequiresReboot = true
		result.Warnings = append(result.Warnings, "Device reboot still owed from an earlier apply")
	}

	if err := a.applyGen2Sections(ctx, setter, apiConfig, result); err != nil {
		return nil, fmt.Errorf("device rejected the login while applying: %w", err)
	}

	result.Duration = time.Since(startTime)
	result.Success = result.FailedCount == 0

	a.logger.WithFields(map[string]any{
		"device_ip":       client.GetIP(),
		"device_type":     deviceType,
		"settings_count":  result.SettingsCount,
		"applied_count":   result.AppliedCount,
		"failed_count":    result.FailedCount,
		"requires_reboot": result.RequiresReboot,
		"duration_ms":     result.Duration.Milliseconds(),
		"component":       "config_applier",
	}).Info("Gen2 configuration apply completed")

	return result, nil
}

// applyGen2Sections writes each section the configuration sets. A section the device refuses is
// recorded and the next one is tried, except when the device rejects the login: every later write
// would be rejected too, so that error stops the apply and is returned.
//
// An answer that asks for a restart is recorded as owed right away, when the owed flag is
// persisted. Unlike the Gen1 flow, a write whose answer was lost is not recorded beforehand,
// because the device, not the manager, decides whether it needs a restart.
func (a *ConfigApplier) applyGen2Sections(ctx context.Context, setter Gen2ConfigClient, apiConfig map[string]interface{}, result *ApplyResult) error {
	// The MQTT password stays out of every error, log line and failure, whichever section fails.
	var secret string
	if mqtt, ok := apiConfig["mqtt"].(map[string]interface{}); ok {
		secret, _ = mqtt["pass"].(string)
	}
	for _, section := range gen2WritesOf(apiConfig) {
		leaves := gen2Leaves(section.key, section.body)

		restart, err := setter.CallConfigSetter(ctx, section.method, section.params)
		if err != nil {
			if secret != "" {
				// A device may echo a request in any encoding, and no list of encodings is
				// complete, so while the apply holds a password the text of every error is hidden
				// whole. The cause stays for errors.Is.
				err = &redactedError{
					msg: "the device refused the " + section.method + " write; its error text is hidden because this apply holds a password",
					err: err,
				}
			}
			a.logger.WithFields(map[string]any{
				"error":     err.Error(),
				"method":    section.method,
				"component": "config_applier",
			}).Warn("Failed to apply " + section.key + " settings")
			for _, leaf := range leaves {
				value := fmt.Sprintf("%v", leaf.value)
				if strings.HasSuffix(leaf.path, ".pass") {
					value = redactedSecret
				}
				result.Failures = append(result.Failures, ApplyFailure{Path: leaf.path, Value: value, Error: err.Error()})
			}
			result.FailedCount += len(leaves)
			if shelly.IsAuthError(err) {
				return err
			}
			continue
		}
		result.AppliedCount += len(leaves)

		if restart {
			a.markGen2RestartOwed(section.method, result)
		}
	}
	return nil
}

// markGen2RestartOwed records that the device asked for a restart.
func (a *ConfigApplier) markGen2RestartOwed(method string, result *ApplyResult) {
	if !result.RequiresReboot {
		result.RequiresReboot = true
		result.Warnings = append(result.Warnings,
			"Device reboot required for some settings to take effect ("+method+" asked for a restart)")
	}
	if a.owed == nil || a.owed.pending {
		return
	}
	if err := a.owed.store(true); err != nil {
		// A reboot that fails after this would leave nothing owed. Failing the apply keeps it
		// from rebooting or verifying. The API shows a failure by its path only, so the warning
		// carries what is still to do.
		result.Failures = append(result.Failures, ApplyFailure{
			Path:  "reboot",
			Value: method,
			Error: "the restart the device asked for could not be recorded as owed: " + err.Error(),
		})
		result.FailedCount++
		result.Warnings = append(result.Warnings,
			"The device asked for a restart that could not be recorded as owed. Reboot the device by hand, then apply again.")
		return
	}
	a.owed.pending = true
}

type gen2Leaf struct {
	path  string
	value interface{}
}

// gen2Leaves returns the settings of one section as dotted paths, sorted, such as sys.device.name.
func gen2Leaves(prefix string, body map[string]interface{}) []gen2Leaf {
	var leaves []gen2Leaf
	for key, value := range body {
		path := prefix + "." + key
		if nested, ok := value.(map[string]interface{}); ok {
			leaves = append(leaves, gen2Leaves(path, nested)...)
			continue
		}
		leaves = append(leaves, gen2Leaf{path: path, value: value})
	}
	sort.Slice(leaves, func(i, j int) bool { return leaves[i].path < leaves[j].path })
	return leaves
}
