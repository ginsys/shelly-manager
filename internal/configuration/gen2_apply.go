package configuration

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
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
// the RPC method that writes each. S1 writes sys only; later slices add their sections here.
var gen2Sections = []struct {
	key    string
	method string
}{
	{"sys", "Sys.SetConfig"},
}

// gen2ApplyKeys are the top-level keys of a Gen2 config document that apply writes.
var gen2ApplyKeys = map[string]bool{"sys": true}

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
// and read back. Only the sys section has a mapping so far, so every other populated setting is
// reported through the round trip: the converter drops it, and it is missing on the way back.
func unsupportedGen2Settings(converter ConfigConverter, desired *DeviceConfiguration) ([]string, error) {
	apiJSON, err := converter.ToAPIConfig(desired, "")
	if err != nil {
		return nil, fmt.Errorf("failed to convert desired config: %w", err)
	}
	var apiConfig map[string]interface{}
	if err = json.Unmarshal(apiJSON, &apiConfig); err != nil {
		return nil, fmt.Errorf("failed to parse converted config: %w", err)
	}

	var unsupported []string
	for key := range apiConfig {
		if !gen2ApplyKeys[key] {
			unsupported = append(unsupported, key)
		}
	}

	back, err := converter.FromAPIConfig(apiJSON, "")
	if err != nil {
		return nil, fmt.Errorf("failed to check converted config: %w", err)
	}
	for _, diff := range NewConfigComparator().Compare(desired, back).Differences {
		if diff.Actual == nil {
			unsupported = append(unsupported, diff.Path)
		}
	}

	// The device takes any string as a time server, so a typo would silently stop its clock.
	// The Gen1 rule applies: a plain host name or IPv4 address. A host:port form is refused too.
	if desired.System != nil && desired.System.SNTP != nil && desired.System.SNTP.Server != "" && !validTimeServer(desired.System.SNTP.Server) {
		unsupported = append(unsupported, "system.sntp.server")
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
	for _, section := range gen2Sections {
		if body, ok := apiConfig[section.key].(map[string]interface{}); ok {
			result.SettingsCount += len(gen2Leaves(section.key, body))
		}
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
	for _, section := range gen2Sections {
		body, ok := apiConfig[section.key].(map[string]interface{})
		if !ok || len(body) == 0 {
			continue
		}
		leaves := gen2Leaves(section.key, body)

		restart, err := setter.CallConfigSetter(ctx, section.method, map[string]interface{}{"config": body})
		if err != nil {
			a.logger.WithFields(map[string]any{
				"error":     err.Error(),
				"method":    section.method,
				"component": "config_applier",
			}).Warn("Failed to apply " + section.key + " settings")
			for _, leaf := range leaves {
				result.Failures = append(result.Failures, ApplyFailure{Path: leaf.path, Value: fmt.Sprintf("%v", leaf.value), Error: err.Error()})
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
		// from rebooting or verifying, and the failure names what is still to do.
		result.Failures = append(result.Failures, ApplyFailure{
			Path:  "reboot",
			Value: method,
			Error: "the device asked for a restart but it could not be recorded as owed; reboot the device and apply again: " + err.Error(),
		})
		result.FailedCount++
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
