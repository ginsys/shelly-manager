package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/shelly"
)

// ApplyResult represents the result of applying configuration to a device
type ApplyResult struct {
	Success        bool           `json:"success"`         // All settings applied successfully
	SettingsCount  int            `json:"settings_count"`  // Number of settings sent
	AppliedCount   int            `json:"applied_count"`   // Number of settings accepted
	FailedCount    int            `json:"failed_count"`    // Number of settings rejected
	Failures       []ApplyFailure `json:"failures"`        // Details of failures
	RequiresReboot bool           `json:"requires_reboot"` // Device needs reboot to apply
	Warnings       []string       `json:"warnings"`        // Non-fatal issues
	Duration       time.Duration  `json:"duration"`        // Time taken to apply

	// Unconfirmed is true when a write changed the station network (ssid, enabled, IP settings) and
	// the device then stopped answering: it may now sit on another network. That is neither a
	// failure nor a success, so verify does not run and the device stays pending.
	Unconfirmed bool `json:"unconfirmed"`

	// login is the login change the device accepted in this apply, or nil. It is unexported so
	// the credentials never reach a response or a log.
	login *LoginChange
}

// LoginChange is a login that a device accepted. The password is never printed or marshalled.
type LoginChange struct {
	Enabled  bool   `json:"enabled"`
	Username string `json:"username,omitempty"`
	Password string `json:"-"`
}

// String and GoString keep the password out of every fmt verb, including %+v and %#v.
func (c LoginChange) String() string {
	return fmt.Sprintf("login{enabled=%t user=%q}", c.Enabled, c.Username)
}

func (c LoginChange) GoString() string { return c.String() }

// ApplyFailure represents a single failed setting
type ApplyFailure struct {
	Path  string `json:"path"`  // e.g., "mqtt.server"
	Value string `json:"value"` // What we tried to set
	Error string `json:"error"` // Error from device
}

// ShellyClient defines the interface for communicating with Shelly devices
// This allows for mocking in tests
type ShellyClient interface {
	SetConfig(ctx context.Context, config map[string]interface{}) error
	GetConfig(ctx context.Context) (*shelly.DeviceConfig, error)
	GetInfo(ctx context.Context) (*shelly.DeviceInfo, error)
	Reboot(ctx context.Context) error
	TestConnection(ctx context.Context) error
	GetGeneration() int
	GetIP() string
}

// ConfigApplier handles applying configurations to devices
type ConfigApplier struct {
	converter  ConfigConverter
	comparator *ConfigComparator
	logger     *logging.Logger

	// writeLogin lets apply write the login. It is off unless the caller stores the new
	// credentials and rebuilds its client afterwards (ConfigVerifier.WithLogin), because a
	// login written behind the caller's back locks the manager out of the device.
	writeLogin bool

	// owed carries a reboot an earlier apply left owed and records a new one before any write
	// (ConfigVerifier.WithPendingReboot). Nil keeps the old behaviour: nothing is persisted.
	owed *pendingReboot
}

// NewConfigApplier creates a new configuration applier
func NewConfigApplier(converter ConfigConverter, logger *logging.Logger) *ConfigApplier {
	if logger == nil {
		logger = logging.GetDefault()
	}
	return &ConfigApplier{
		converter:  converter,
		comparator: NewConfigComparator(),
		logger:     logger,
	}
}

// ApplyConfig applies the given configuration to a device via the provided client
func (a *ConfigApplier) ApplyConfig(ctx context.Context, client ShellyClient, config *DeviceConfiguration, deviceType string) (*ApplyResult, error) {
	startTime := time.Now()

	result := &ApplyResult{
		Success:  true,
		Failures: []ApplyFailure{},
		Warnings: []string{},
		Duration: 0,
	}

	if a.converter.Generation() >= 2 {
		return a.applyGen2Config(ctx, client, config, deviceType, result, startTime)
	}

	// Get current config for comparison (to detect reboot requirements)
	currentConfig, readErr := client.GetConfig(ctx)
	if readErr != nil {
		a.logger.WithFields(map[string]any{
			"error":     readErr.Error(),
			"device_ip": client.GetIP(),
			"component": "config_applier",
		}).Warn("Could not get current config for comparison")
	}

	// Convert internal config to API format
	apiJSON, err := a.converter.ToAPIConfig(config, deviceType)
	if err != nil {
		return nil, fmt.Errorf("failed to convert config to API format: %w", err)
	}

	// Parse API JSON to map
	var apiConfig map[string]interface{}
	if err := json.Unmarshal(apiJSON, &apiConfig); err != nil {
		return nil, fmt.Errorf("failed to parse API config: %w", err)
	}

	var liveRaw json.RawMessage
	if currentConfig != nil {
		liveRaw = currentConfig.Raw
	}
	routeGen1MaxPower(apiConfig, liveRaw, deviceType)

	// Count settings
	result.SettingsCount = a.countSettings(apiConfig)

	// Refuse a Wi-Fi change that could take the device off its network, before the first write of
	// any section.
	if err := checkWiFiBeforeWrite(liveRaw, currentConfig != nil, readErr, apiConfig); err != nil {
		return nil, err
	}

	// Decide the reboot from the settings as they are before any write, and record it before the
	// first one. A write that fails can still have landed (the answer may be lost), and a later
	// failure returns before the reboot, so the obligation has to be stored first: a retry reads
	// the written value as already stored and would see nothing to reboot for. A setting that was
	// refused for good costs one reboot too many, which is cheap.
	if currentConfig != nil {
		result.RequiresReboot = a.detectRebootRequired(currentConfig.Raw, apiJSON, config)
	}
	if a.owed != nil {
		if currentConfig == nil {
			// Without the stored settings the reboot cannot be decided, and a later write that fails
			// would leave nothing owed. Nothing has been written yet, so stopping costs nothing.
			return nil, fmt.Errorf("could not read the device's settings, nothing was written: %w", readErr)
		}
		if a.owed.pending && !result.RequiresReboot {
			result.RequiresReboot = true
			result.Warnings = append(result.Warnings, "Device reboot still owed from an earlier apply")
		}
		if result.RequiresReboot && !a.owed.pending {
			if storeErr := a.owed.store(true); storeErr != nil {
				return nil, fmt.Errorf("could not record the pending reboot, nothing was written: %w", storeErr)
			}
			a.owed.pending = true
		}
	}
	if result.RequiresReboot {
		result.Warnings = append(result.Warnings,
			"Device reboot required for some settings to take effect")
	}

	// Apply configuration sections. A rejected login is an error, not a failed setting: the caller
	// maps it to "the device rejected the stored credentials".
	if err := a.applyConfigSections(ctx, client, apiConfig, liveRaw, deviceType, result); err != nil {
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
	}).Info("Configuration apply completed")

	return result, nil
}

// applyConfigSections applies each section of the configuration. A section the device refuses
// is recorded in result and the next section is tried, except when the device rejects the login:
// every later write would be rejected too, so that error stops the apply and is returned.
func (a *ConfigApplier) applyConfigSections(ctx context.Context, client ShellyClient, apiConfig map[string]interface{}, live json.RawMessage, deviceType string, result *ApplyResult) error {
	// For Gen1 devices, we can send settings to /settings endpoint
	// Different setting groups go to different endpoints
	sections := []struct {
		name  string
		apply func(context.Context, ShellyClient, map[string]interface{}, *ApplyResult) error
	}{
		{"main", a.applyMainSettings}, // name, timezone, location, etc.
		{"WiFi", func(ctx context.Context, client ShellyClient, apiConfig map[string]interface{}, result *ApplyResult) error {
			return a.applyWiFiSettings(ctx, client, apiConfig, live, result)
		}},
		{"MQTT", a.applyMQTTSettings},
		{"SNTP", a.applySNTPSettings},
		{"Cloud", a.applyCloudSettings},
		{"CoIoT", a.applyCoIoTSettings},
		{"Relay", a.applyRelaySettings},
		{"Roller", a.applyRollerSettings},
		// Inputs follow the relays: on a relay model they are keys of the same channel.
		{"Input", func(ctx context.Context, client ShellyClient, apiConfig map[string]interface{}, result *ApplyResult) error {
			return a.applyInputSettings(ctx, client, apiConfig, deviceType, result)
		}},
		// The login goes last: it changes the credentials every later request needs, and an
		// earlier failure then never leaves a changed login behind.
		{"Login", a.applyLoginSettings},
	}
	for _, section := range sections {
		if err := section.apply(ctx, client, apiConfig, result); err != nil {
			a.logger.WithFields(map[string]any{
				"error":     err.Error(),
				"component": "config_applier",
			}).Warn("Failed to apply " + section.name + " settings")
			if shelly.IsAuthError(err) {
				return err
			}
		}
		if result.Unconfirmed {
			// The device may be on another network, so the later sections would only fail against it.
			result.Warnings = append(result.Warnings, wifiLaterSectionsWarning)
			break
		}
	}
	return nil
}

func (a *ConfigApplier) applyMainSettings(ctx context.Context, client ShellyClient, apiConfig map[string]interface{}, result *ApplyResult) error {
	mainSettings := make(map[string]interface{})

	// Extract main settings
	mainKeys := []string{"name", "timezone", "lat", "lng", "eco_mode_enabled", "discoverable", "max_power"}
	for _, key := range mainKeys {
		if val, ok := apiConfig[key]; ok {
			mainSettings[key] = val
		}
	}

	// LED settings (inverted booleans)
	if val, ok := apiConfig["led_power_disable"]; ok {
		mainSettings["led_power_disable"] = val
	}
	if val, ok := apiConfig["led_status_disable"]; ok {
		mainSettings["led_status_disable"] = val
	}

	if len(mainSettings) == 0 {
		return nil
	}

	// A custom timezone or location only takes effect once the device's automatic detection
	// is off. The prerequisite is sent with them but is not a desired setting, so it is not
	// counted.
	write := make(map[string]interface{})
	for key, val := range mainSettings {
		write[key] = val
	}
	for _, key := range []string{"timezone", "lat", "lng"} {
		if _, ok := mainSettings[key]; ok {
			write["tzautodetect"] = false
			break
		}
	}

	if err := client.SetConfig(ctx, write); err != nil {
		for key := range mainSettings {
			result.Failures = append(result.Failures, ApplyFailure{
				Path:  key,
				Value: fmt.Sprintf("%v", mainSettings[key]),
				Error: err.Error(),
			})
			result.FailedCount++
		}
		return err
	}

	result.AppliedCount += len(mainSettings)
	return nil
}

func (a *ConfigApplier) applyMQTTSettings(ctx context.Context, client ShellyClient, apiConfig map[string]interface{}, result *ApplyResult) error {
	mqtt, ok := apiConfig["mqtt"].(map[string]interface{})
	if !ok || len(mqtt) == 0 {
		return nil
	}

	form := gen1MQTTForm(mqtt)
	if len(form) == 0 {
		return nil
	}

	if err := client.SetConfig(ctx, form); err != nil {
		// Value names the keys only: the form can hold the broker password.
		result.Failures = append(result.Failures, ApplyFailure{
			Path:  "mqtt",
			Value: strings.Join(sortedKeys(form), ","),
			Error: err.Error(),
		})
		result.FailedCount += len(form)
		return err
	}

	result.AppliedCount += len(form)
	return nil
}

// applySNTPSettings writes the time server. The device reports it as the nested sntp.server and
// takes it as the flat sntp_server parameter. The parameter name is unverified until the
// hardware readback (the reference was read through a summary only).
func (a *ConfigApplier) applySNTPSettings(ctx context.Context, client ShellyClient, apiConfig map[string]interface{}, result *ApplyResult) error {
	sntp, ok := apiConfig["sntp"].(map[string]interface{})
	if !ok {
		return nil
	}
	server, ok := sntp["server"].(string)
	if !ok || server == "" {
		return nil
	}

	if err := client.SetConfig(ctx, map[string]interface{}{"sntp_server": server}); err != nil {
		result.Failures = append(result.Failures, ApplyFailure{
			Path:  "sntp",
			Value: server,
			Error: err.Error(),
		})
		result.FailedCount++
		return err
	}

	result.AppliedCount++
	return nil
}

// gen1MQTTKeys maps the nested "mqtt" document that Gen1 returns from GET /settings to the
// flat form parameters that POST /settings accepts. The device reads nested maps on output but
// only understands flat mqtt_* parameters on input.
var gen1MQTTKeys = map[string]string{
	"enable":        "mqtt_enable",
	"server":        "mqtt_server",
	"user":          "mqtt_user",
	"pass":          "mqtt_pass",
	"id":            "mqtt_id",
	"clean_session": "mqtt_clean_session",
	"keep_alive":    "mqtt_keep_alive",
	// The key name is unverified until the hardware readback (the reference was read through a
	// summary only).
	"update_period": "mqtt_update_period",
}

// gen1MQTTForm converts a nested mqtt document into Gen1 form parameters. Keys it does not know
// are dropped, never sent.
func gen1MQTTForm(mqtt map[string]interface{}) map[string]interface{} {
	form := make(map[string]interface{}, len(mqtt))
	for key, value := range mqtt {
		if formKey, ok := gen1MQTTKeys[key]; ok {
			form[formKey] = value
		}
	}
	return form
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// LoginSettingsClient is the login surface of a Gen1 client: POST /settings/login. SetAuth turns
// the login on with a username and password, ResetAuth turns it off.
type LoginSettingsClient interface {
	SetAuth(ctx context.Context, username, password string) error
	ResetAuth(ctx context.Context) error
}

var errNoLoginClient = errors.New("client cannot write the login")

// redactedError hides a secret from an error's text and keeps the wrapped error for errors.Is.
type redactedError struct {
	msg string
	err error
}

func (e *redactedError) Error() string { return e.msg }
func (e *redactedError) Unwrap() error { return e.err }

func redact(err error, secret string) error {
	if secret == "" || !strings.Contains(err.Error(), secret) {
		return err
	}
	return &redactedError{msg: strings.ReplaceAll(err.Error(), secret, "***"), err: err}
}

// applyLoginSettings writes the login with POST /settings/login, never as part of POST /settings.
// The device never returns the password, so verify cannot confirm it. A rejected write returns
// the error with the password removed, and records no login change.
func (a *ConfigApplier) applyLoginSettings(ctx context.Context, client ShellyClient, apiConfig map[string]interface{}, result *ApplyResult) error {
	// A section that failed earlier means the apply as a whole failed: the login then stays
	// unwritten, so the stored credentials and the cached client never fall behind the device.
	if !a.writeLogin || result.FailedCount > 0 {
		return nil
	}
	login, ok := apiConfig["login"].(map[string]interface{})
	if !ok {
		return nil
	}
	enabled, ok := login["enabled"].(bool)
	if !ok {
		return nil
	}
	username, _ := login["username"].(string)
	password, _ := login["password"].(string)
	failure := ApplyFailure{Path: "login", Value: strings.Join(sortedKeys(login), ",")}

	loginClient, ok := client.(LoginSettingsClient)
	if !ok {
		failure.Error = errNoLoginClient.Error()
		result.Failures = append(result.Failures, failure)
		result.FailedCount += len(login)
		return errNoLoginClient
	}

	var err error
	if enabled {
		err = loginClient.SetAuth(ctx, username, password)
	} else {
		err = loginClient.ResetAuth(ctx)
	}
	if err != nil {
		err = redact(err, password)
		failure.Error = err.Error()
		result.Failures = append(result.Failures, failure)
		result.FailedCount += len(login)
		return err
	}

	result.AppliedCount += len(login)
	result.login = &LoginChange{Enabled: enabled}
	if enabled {
		result.login.Username, result.login.Password = username, password
	}
	return nil
}

// CloudSettingsClient is the cloud surface of a Gen1 client: POST /settings/cloud. A client
// without it cannot write the cloud flag, and apply reports that as a failure instead of falling
// back to a nested "cloud" document on POST /settings, which Gen1 ignores.
type CloudSettingsClient interface {
	SetCloudConfig(ctx context.Context, enabled bool, server string) error
}

var errNoCloudClient = errors.New("client cannot write the cloud setting")

// applyCloudSettings writes the cloud flag. Gen1 reports no cloud server and POST /settings/cloud
// takes none, so only the flag is sent; a desired server is refused earlier (see
// unsupportedApplySettings). The final read-back of the whole configuration confirms the flag.
func (a *ConfigApplier) applyCloudSettings(ctx context.Context, client ShellyClient, apiConfig map[string]interface{}, result *ApplyResult) error {
	cloud, ok := apiConfig["cloud"].(map[string]interface{})
	if !ok {
		return nil
	}
	enabled, ok := cloud["enabled"].(bool)
	if !ok {
		return nil
	}

	failure := ApplyFailure{Path: "cloud.enabled", Value: fmt.Sprintf("%t", enabled)}
	cloudClient, ok := client.(CloudSettingsClient)
	if !ok {
		failure.Error = errNoCloudClient.Error()
		result.Failures = append(result.Failures, failure)
		result.FailedCount++
		return errNoCloudClient
	}
	if err := cloudClient.SetCloudConfig(ctx, enabled, ""); err != nil {
		failure.Error = err.Error()
		result.Failures = append(result.Failures, failure)
		result.FailedCount++
		return err
	}

	result.AppliedCount++
	return nil
}

func (a *ConfigApplier) applyCoIoTSettings(ctx context.Context, client ShellyClient, apiConfig map[string]interface{}, result *ApplyResult) error {
	coiot, ok := apiConfig["coiot"].(map[string]interface{})
	if !ok || len(coiot) == 0 {
		return nil
	}

	form := gen1CoIoTForm(coiot)
	if len(form) == 0 {
		return nil
	}

	if err := client.SetConfig(ctx, form); err != nil {
		result.Failures = append(result.Failures, ApplyFailure{
			Path:  "coiot",
			Value: strings.Join(sortedKeys(form), ","),
			Error: err.Error(),
		})
		result.FailedCount += len(form)
		return err
	}

	result.AppliedCount += len(form)
	return nil
}

// gen1CoIoTKeys maps the nested "coiot" document that Gen1 returns from GET /settings to the
// flat parameters that POST /settings accepts (there is no /settings/coiot endpoint).
var gen1CoIoTKeys = map[string]string{
	"enabled":       "coiot_enable",
	"update_period": "coiot_update_period",
	"peer":          "coiot_peer",
}

// gen1CoIoTMulticast is the peer value that switches CoIoT to multicast. The device reports
// multicast as an empty peer.
const gen1CoIoTMulticast = "mcast"

// gen1CoIoTForm converts a nested coiot document into Gen1 form parameters. Keys it does not know
// are dropped, never sent. An empty peer is written as "mcast".
func gen1CoIoTForm(coiot map[string]interface{}) map[string]interface{} {
	form := make(map[string]interface{}, len(coiot))
	for key, value := range coiot {
		formKey, ok := gen1CoIoTKeys[key]
		if !ok {
			continue
		}
		if peer, isString := value.(string); key == "peer" && isString && peer == "" {
			value = gen1CoIoTMulticast
		}
		form[formKey] = value
	}
	return form
}

// RelaySettingsClient is the per-channel relay surface of a Gen1 client: POST and GET on
// /settings/relay/<n>. A client without it cannot apply relay settings, and apply reports that
// as a failure instead of falling back to the nested "relays" document, which Gen1 ignores.
type RelaySettingsClient interface {
	SetRelaySettings(ctx context.Context, channel int, settings map[string]interface{}) error
	GetRelaySettings(ctx context.Context, channel int) (map[string]interface{}, error)
}

var errNoRelayClient = errors.New("client cannot write per-channel relay settings")

// gen1RelayKeys are the relay settings apply writes: the flat form parameters of
// POST /settings/relay/<n>. Anything else in a relay document is refused before the device is
// touched (see unsupportedApplySettings).
var gen1RelayKeys = []string{"name", "default_state", "auto_on", "auto_off", "appliance_type", "schedule", "schedule_rules", "max_power"}

func (a *ConfigApplier) applyRelaySettings(ctx context.Context, client ShellyClient, apiConfig map[string]interface{}, result *ApplyResult) error {
	relays := relayMaps(apiConfig["relays"])
	if len(relays) == 0 {
		return nil
	}

	relayClient, ok := client.(RelaySettingsClient)
	if !ok {
		return failSection(result, "relays", errNoRelayClient)
	}
	return applyChannels(ctx, result, "relays", relays, gen1RelayKeys, relayClient.SetRelaySettings, relayClient.GetRelaySettings)
}

// InputSettingsClient is the per-input surface of a Shelly i3: POST and GET on
// /settings/input/<n>. The relay models keep their button type in the relay channel instead, so
// they go through RelaySettingsClient. A client without the route a model needs cannot apply
// inputs, and apply reports that as a failure instead of sending a nested "inputs" document, which
// Gen1 ignores.
type InputSettingsClient interface {
	SetInputSettings(ctx context.Context, input int, settings map[string]interface{}) error
	GetInputSettings(ctx context.Context, input int) (map[string]interface{}, error)
}

var errNoInputClient = errors.New("client cannot write per-input settings")

// gen1InputKeys are the input settings apply writes: the flat form parameters btn_type and
// btn_reverse. Anything else in an input document is refused before the device is touched (see
// unsupportedApplySettings).
var gen1InputKeys = []string{"btn_type", "btn_reverse"}

func (a *ConfigApplier) applyInputSettings(ctx context.Context, client ShellyClient, apiConfig map[string]interface{}, deviceType string, result *ApplyResult) error {
	inputs := relayMaps(apiConfig["inputs"])
	if len(inputs) == 0 {
		return nil
	}

	if inputsLiveInRelays(deviceType) {
		relayClient, ok := client.(RelaySettingsClient)
		if !ok {
			return failSection(result, "inputs", errNoRelayClient)
		}
		return applyChannels(ctx, result, "inputs", inputs, gen1InputKeys, relayClient.SetRelaySettings, relayClient.GetRelaySettings)
	}
	inputClient, ok := client.(InputSettingsClient)
	if !ok {
		return failSection(result, "inputs", errNoInputClient)
	}
	return applyChannels(ctx, result, "inputs", inputs, gen1InputKeys, inputClient.SetInputSettings, inputClient.GetInputSettings)
}

// failSection records a section that cannot be written at all as one failed path.
func failSection(result *ApplyResult, section string, err error) error {
	result.Failures = append(result.Failures, ApplyFailure{Path: section, Error: err.Error()})
	result.FailedCount++
	return err
}

// applyChannels writes the listed keys of each item with one POST per channel, then reads the
// channel back and compares every key just written. A device can answer 200 and keep the old
// value, so a mismatch is a failure of that path.
func applyChannels(
	ctx context.Context,
	result *ApplyResult,
	section string,
	items []map[string]interface{},
	keys []string,
	set func(context.Context, int, map[string]interface{}) error,
	get func(context.Context, int) (map[string]interface{}, error),
) error {
	var firstErr error
	for channel, item := range items {
		form := make(map[string]interface{}, len(keys))
		for _, key := range keys {
			if value, present := item[key]; present {
				form[key] = value
			}
		}
		if len(form) == 0 {
			continue
		}

		if err := set(ctx, channel, form); err != nil {
			failChannel(result, section, channel, form, err.Error())
			firstErr = firstError(firstErr, err)
			continue
		}

		live, err := get(ctx, channel)
		if err != nil {
			failChannel(result, section, channel, form, "read-back failed: "+err.Error())
			firstErr = firstError(firstErr, err)
			continue
		}
		for _, key := range sortedKeys(form) {
			if relayValuesEqual(form[key], live[key]) {
				result.AppliedCount++
				continue
			}
			result.Failures = append(result.Failures, ApplyFailure{
				Path:  fmt.Sprintf("%s[%d].%s", section, channel, key),
				Value: fmt.Sprintf("%v", form[key]),
				Error: fmt.Sprintf("device reports %v after the write", live[key]),
			})
			result.FailedCount++
		}
	}
	return firstErr
}

// failChannel records every key of one channel's form as failed.
func failChannel(result *ApplyResult, section string, channel int, form map[string]interface{}, message string) {
	for _, key := range sortedKeys(form) {
		result.Failures = append(result.Failures, ApplyFailure{
			Path:  fmt.Sprintf("%s[%d].%s", section, channel, key),
			Value: fmt.Sprintf("%v", form[key]),
			Error: message,
		})
		result.FailedCount++
	}
}

// RollerSettingsClient is the per-channel roller surface of a Gen1 client: POST and GET on
// /settings/roller/<n>. A client without it cannot apply roller settings, and apply reports that
// as a failure instead of sending the nested "rollers" document, which Gen1 ignores.
type RollerSettingsClient interface {
	SetRollerSettingsForm(ctx context.Context, channel int, settings map[string]interface{}) error
	GetRollerSettingsForm(ctx context.Context, channel int) (map[string]interface{}, error)
}

var errNoRollerClient = errors.New("client cannot write per-channel roller settings")

// gen1RollerKeys are the roller settings apply writes: the flat form parameters of
// POST /settings/roller/<n>. Anything else in a roller document is refused before the device is
// touched (see unsupportedApplySettings). The key names and their spellings are unverified until
// the hardware readback, and a device in relay mode refuses the endpoint.
var gen1RollerKeys = []string{"maxtime_open", "maxtime_close", "swap", "swap_inputs", "input_mode"}

func (a *ConfigApplier) applyRollerSettings(ctx context.Context, client ShellyClient, apiConfig map[string]interface{}, result *ApplyResult) error {
	rollers := relayMaps(apiConfig["rollers"])
	if len(rollers) == 0 {
		return nil
	}

	rollerClient, ok := client.(RollerSettingsClient)
	if !ok {
		result.Failures = append(result.Failures, ApplyFailure{
			Path:  "rollers",
			Error: errNoRollerClient.Error(),
		})
		result.FailedCount++
		return errNoRollerClient
	}

	var firstErr error
	for channel, roller := range rollers {
		form := make(map[string]interface{}, len(gen1RollerKeys))
		for _, key := range gen1RollerKeys {
			if value, present := roller[key]; present {
				form[key] = value
			}
		}
		if len(form) == 0 {
			continue
		}

		if err := rollerClient.SetRollerSettingsForm(ctx, channel, form); err != nil {
			a.failRoller(result, channel, form, err.Error())
			firstErr = firstError(firstErr, err)
			continue
		}

		// Read back every key just written: a device can answer 200 and keep the old value.
		live, err := rollerClient.GetRollerSettingsForm(ctx, channel)
		if err != nil {
			a.failRoller(result, channel, form, "read-back failed: "+err.Error())
			firstErr = firstError(firstErr, err)
			continue
		}
		for _, key := range sortedKeys(form) {
			if relayValuesEqual(form[key], live[key]) {
				result.AppliedCount++
				continue
			}
			result.Failures = append(result.Failures, ApplyFailure{
				Path:  fmt.Sprintf("rollers[%d].%s", channel, key),
				Value: fmt.Sprintf("%v", form[key]),
				Error: fmt.Sprintf("device reports %v after the write", live[key]),
			})
			result.FailedCount++
		}
	}

	return firstErr
}

// failRoller records every key of one channel's form as failed.
func (a *ConfigApplier) failRoller(result *ApplyResult, channel int, form map[string]interface{}, message string) {
	for _, key := range sortedKeys(form) {
		result.Failures = append(result.Failures, ApplyFailure{
			Path:  fmt.Sprintf("rollers[%d].%s", channel, key),
			Value: fmt.Sprintf("%v", form[key]),
			Error: message,
		})
		result.FailedCount++
	}
}

func firstError(current, next error) error {
	if current != nil {
		return current
	}
	return next
}

// relayMaps returns the relay documents of the converted configuration, in channel order.
func relayMaps(raw interface{}) []map[string]interface{} {
	switch relays := raw.(type) {
	case []map[string]interface{}:
		return relays
	case []interface{}:
		out := make([]map[string]interface{}, 0, len(relays))
		for _, item := range relays {
			relay, _ := item.(map[string]interface{})
			out = append(out, relay)
		}
		return out
	}
	return nil
}

// relayValuesEqual compares a written form value with the JSON the device returns. JSON numbers
// arrive as float64, while the converter holds ints. A string must come back as a string: a
// null or a number that prints the same as the written text is a mismatch. A written 0 or 1 matches
// a live false or true, and the reverse, and nothing else about a boolean changes.
func relayValuesEqual(written, live interface{}) bool {
	if wf, ok := toFloat(written); ok {
		lf, lok := toFloat(live)
		if !lok {
			lf, lok = flagAsFloat(live)
		}
		return lok && wf == lf
	}
	if wf, ok := flagAsFloat(written); ok {
		if lf, lok := toFloat(live); lok {
			return wf == lf
		}
	}
	if ws, ok := written.(string); ok {
		ls, lok := live.(string)
		return lok && ws == ls
	}
	return fmt.Sprintf("%v", written) == fmt.Sprintf("%v", live)
}

func toFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	}
	return 0, false
}

// flagAsFloat reads a JSON boolean as 0 or 1, because a device may report a 0/1 flag as true/false.
func flagAsFloat(v interface{}) (float64, bool) {
	b, ok := v.(bool)
	if !ok {
		return 0, false
	}
	if b {
		return 1, true
	}
	return 0, true
}

// countSettings counts the number of settings being applied
func (a *ConfigApplier) countSettings(config map[string]interface{}) int {
	count := 0
	for key, val := range config {
		if key == "relays" || key == "inputs" {
			// One setting per key apply writes, as applyChannels counts them.
			keys := gen1RelayKeys
			if key == "inputs" {
				keys = gen1InputKeys
			}
			for _, item := range relayMaps(val) {
				for _, itemKey := range keys {
					if _, ok := item[itemKey]; ok {
						count++
					}
				}
			}
			continue
		}
		if key == "wifi_sta" || key == "wifi_ap" {
			continue // counted below, as the form keys apply writes
		}
		if key == "rollers" {
			// One setting per roller key apply writes, as applyRollerSettings counts them.
			for _, roller := range relayMaps(val) {
				for _, rollerKey := range gen1RollerKeys {
					if _, ok := roller[rollerKey]; ok {
						count++
					}
				}
			}
			continue
		}
		switch v := val.(type) {
		case map[string]interface{}:
			count += len(v)
		case []interface{}:
			count += len(v)
		default:
			count++
		}
		_ = key
	}
	sta, ap := gen1WiFiForms(config)
	return count + len(sta) + len(ap)
}

// detectRebootRequired determines if a reboot is needed
func (a *ConfigApplier) detectRebootRequired(oldConfigJSON json.RawMessage, newConfigJSON json.RawMessage, newConfig *DeviceConfiguration) bool {
	// Settings that require reboot. The Wi-Fi routes are not here: the reference says their
	// parameters are applied immediately and names no reboot (unverified until the hardware
	// readback), and a station change followed by a reboot of a device that may be on another
	// network is the worse mistake.
	rebootSettings := []string{
		"login.enabled",
		"login.username",
		"login.password",
	}
	// The Gen1 reference says an MQTT change needs a reboot (read through a summary, so unverified
	// until the hardware readback). Every key apply writes counts, so a new key in gen1MQTTKeys is
	// covered at once. The password is never read back, so a configured one always counts as changed.
	for key := range gen1MQTTKeys {
		rebootSettings = append(rebootSettings, "mqtt."+key)
	}

	// Check if any reboot-required settings changed
	var oldConfig, newAPIConfig map[string]interface{}
	if err := json.Unmarshal(oldConfigJSON, &oldConfig); err != nil {
		return false
	}
	if err := json.Unmarshal(newConfigJSON, &newAPIConfig); err != nil {
		return false
	}

	for _, path := range rebootSettings {
		parts := strings.Split(path, ".")
		if len(parts) != 2 {
			continue
		}

		section, field := parts[0], parts[1]

		oldSection, _ := oldConfig[section].(map[string]interface{})
		newSection, _ := newAPIConfig[section].(map[string]interface{})

		if oldSection == nil || newSection == nil {
			continue
		}

		oldVal := oldSection[field]
		newVal := newSection[field]

		// A value the device does not report (a write-only password) counts as changed. Comparing
		// formatted strings alone would read a password of "<nil>" as equal to a missing one.
		if newVal != nil && (oldVal == nil || fmt.Sprintf("%v", oldVal) != fmt.Sprintf("%v", newVal)) {
			return true
		}
	}

	// Check auth enable change
	if newConfig != nil && newConfig.Auth != nil && newConfig.Auth.Enable != nil {
		return true
	}

	return false
}

// RebootAndWait reboots the device and waits for it to come back online
func (a *ConfigApplier) RebootAndWait(ctx context.Context, client ShellyClient, timeout time.Duration) error {
	a.logger.WithFields(map[string]any{
		"device_ip": client.GetIP(),
		"timeout":   timeout.String(),
		"component": "config_applier",
	}).Info("Rebooting device")

	// Send reboot command
	if err := client.Reboot(ctx); err != nil {
		return fmt.Errorf("failed to send reboot command: %w", err)
	}

	// Wait a bit for device to start rebooting
	time.Sleep(2 * time.Second)

	// Poll for device to come back online
	deadline := time.Now().Add(timeout)
	pollInterval := 3 * time.Second

	for time.Now().Before(deadline) {
		if err := client.TestConnection(ctx); err == nil {
			a.logger.WithFields(map[string]any{
				"device_ip": client.GetIP(),
				"component": "config_applier",
			}).Info("Device back online after reboot")
			return nil
		}
		time.Sleep(pollInterval)
	}

	return fmt.Errorf("device did not come back online within %s", timeout)
}
