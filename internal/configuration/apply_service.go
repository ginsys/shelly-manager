package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
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

// LoginHooks connects apply to the owner of a device's stored credentials.
type LoginHooks interface {
	// Store records the login the device accepted, so the manager keeps reaching it.
	Store(ctx context.Context, deviceID uint, change LoginChange) error
	// Client returns a client that sends the device's stored credentials, none when its login
	// is off. Apply calls it once the per-device lock is held, so a queued apply does not keep a
	// client that predates the previous apply's login, and again after Store.
	Client(ctx context.Context, deviceID uint) (ShellyClient, error)
}

// gen1ApplyKeys are the Gen1 settings keys that apply writes correctly today: flat parameters
// on POST /settings, the mqtt section, which is flattened to mqtt_* parameters, and the
// relays section, which is written per channel with POST /settings/relay/<n>, and the cloud
// section, whose enabled flag is written with POST /settings/cloud, the coiot section, which is
// flattened to coiot_* parameters, and the login section, which is written with POST
// /settings/login, but only where LoginHooks is set, and the sntp section, whose server is
// written as the flat sntp_server parameter, and the inputs section, whose btn_type and
// btn_reverse are written per input (POST /settings/relay/<n> on a Shelly 1, 1PM or 2.5, POST
// /settings/input/<n> on a Shelly i3), and the wifi_sta and wifi_ap sections, which are written
// with POST /settings/sta and /settings/ap (unsupportedWiFi names what stays refused). Every other
// section is refused before the device is touched, because the applier would send it in a shape the
// device ignores. A relay key outside gen1RelayKeys
// is refused through the round-trip check below, and so is a cloud server, which Gen1 neither
// reports nor accepts.
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
	"rollers":            true,
	"cloud":              true,
	"coiot":              true,
	"login":              true,
	"sntp":               true,
	"inputs":             true,
	"wifi_sta":           true,
	"wifi_ap":            true,
}

// gen1CoIoTMinPeriod and gen1CoIoTMaxPeriod bound the CoIoT update period, in seconds, that a
// Gen1 device accepts on POST /settings (coiot_update_period).
const (
	gen1CoIoTMinPeriod = 15
	gen1CoIoTMaxPeriod = 65535
)

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
	"mqtt.update_period":         true,
	"system.sntp.server":         true,
	"cloud.enable":               true,
	"coiot.enable":               true,
	"coiot.update_period":        true,
	"coiot.peer":                 true,
	"auth.enable":                true,
	"auth.user":                  true,
	"auth.pass":                  true,

	// relays are written per channel with POST /settings/relay/<n> and read back from
	// GET /settings/relay/<n>. The id is an item's position, not a setting.
	"relay.relays[].name":           true,
	"relay.relays[].default_state":  true,
	"relay.relays[].auto_on":        true,
	"relay.relays[].auto_off":       true,
	"relay.relays[].appliance_type": true,
	"relay.relays[].schedule":       true,
	"relay.relays[].schedule_rules": true,
	"relay.relays[].max_power":      true,

	// The roller is written with POST /settings/roller/0 and read back from the same path.
	// Only a model with roller mode has the section.
	"roller.max_open_time":   true,
	"roller.max_close_time":  true,
	"roller.motor_direction": true,
	"roller.swap_inputs":     true,
	"roller.input_mode":      true,

	// The remaining Shelly 2.5 roller keys, written to the same path. Positioning and its
	// calibration are not applicable: they need a separate action.
	"roller.default_state":             true,
	"roller.button_type":               true,
	"roller.button_reverse":            true,
	"roller.schedule":                  true,
	"roller.schedule_rules":            true,
	"roller.obstacle_mode":             true,
	"roller.obstacle_action":           true,
	"roller.obstacle_power":            true,
	"roller.obstacle_delay":            true,
	"roller.safety_mode":               true,
	"roller.safety_action":             true,
	"roller.safety_allowed_on_trigger": true,
	"roller.off_power":                 true,
	"roller.ends_delay":                true,

	// Inputs: the stored mode is the Gen1 button type and the stored inverted flag is btn_reverse.
	// The input name, type and push actions are not written.
	"input.inputs[].mode":     true,
	"input.inputs[].inverted": true,

	// Wi-Fi: the station is written with POST /settings/sta and the access point with POST
	// /settings/ap, and both are read back from GET /settings. wifi.password and wifi.ap.pass are
	// write-only like mqtt.pass: the device never reports a key, so verify skips them. The access
	// point SSID, the roam threshold and the access point key field are not applicable.
	"wifi.enable":               true,
	"wifi.ssid":                 true,
	"wifi.password":             true,
	"wifi.ipv4mode":             true,
	"wifi.static_ip.ip":         true,
	"wifi.static_ip.netmask":    true,
	"wifi.static_ip.gw":         true,
	"wifi.static_ip.nameserver": true,
	"wifi.ap.enable":            true,
	"wifi.ap.pass":              true,
}

// gen1LoginMaxLength is the longest username or password POST /settings/login accepts. The
// shortest is 1. The limit is in bytes, which is the stricter reading for a multibyte character.
const gen1LoginMaxLength = 50

// unsupportedLogin returns the auth paths of a desired login that the device could not keep.
// The device takes a username and a password together, 1 to 50 characters each, and never
// returns the password. A switched-off login carries no credentials: the device keeps or drops a
// stored username as it likes, so a desired one could not be read back as written.
func unsupportedLogin(auth *AuthConfiguration) []string {
	var unsupported []string
	badLength := func(s *string) bool { return s == nil || len(*s) < 1 || len(*s) > gen1LoginMaxLength }
	switch {
	case auth.Enable == nil:
		if auth.Username != nil || auth.Password != nil {
			unsupported = append(unsupported, "auth.enable")
		}
	case *auth.Enable:
		if badLength(auth.Username) {
			unsupported = append(unsupported, "auth.user")
		}
		if badLength(auth.Password) {
			unsupported = append(unsupported, "auth.pass")
		}
	default:
		if auth.Username != nil {
			unsupported = append(unsupported, "auth.user")
		}
		if auth.Password != nil {
			unsupported = append(unsupported, "auth.pass")
		}
	}
	return unsupported
}

// validTimeServer reports whether s is an IPv4 address or a DNS host name: labels of letters,
// digits and hyphens, no label empty, over 63 bytes or starting or ending with a hyphen, and at
// most 253 bytes in all. A dotted IPv4 address is such a name; an IPv6 literal has colons, so it is
// refused (Gen1 devices are IPv4 only, so one could never be used as a time source). A name whose
// last label is all digits is a mistyped address, never a name (RFC 3696: a top-level label is not
// all-numeric), so it must be a valid IPv4 address: 192.168.1.300 is refused.
// The config form (ui/src/utils/config-form.ts) holds the same rule; both read
// testdata/config/hosts.json.
func validTimeServer(s string) bool {
	// The optional terminal dot is not part of the 253-byte limit.
	name := strings.TrimSuffix(s, ".")
	if len(name) > 253 {
		return false
	}
	labels := strings.Split(name, ".")
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			switch {
			case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
			default:
				return false
			}
		}
	}
	if last := labels[len(labels)-1]; strings.Trim(last, "0123456789") == "" {
		return isIPv4(name)
	}
	return true
}

// unsupportedApplySettings returns the sorted top-level Gen1 keys of desired that apply cannot
// write. loginSupported says whether the caller can store new credentials (LoginHooks).
func unsupportedApplySettings(converter ConfigConverter, desired *DeviceConfiguration, deviceType string, loginSupported bool) ([]string, error) {
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
		if !gen1ApplyKeys[key] || (key == "login" && !loginSupported) {
			unsupported = append(unsupported, key)
		}
	}
	if loginSupported && desired.Auth != nil {
		unsupported = append(unsupported, unsupportedLogin(desired.Auth)...)
	}

	// The converter drops desired settings it has no Gen1 mapping for (for example system.sntp
	// or mqtt.update_period), so they never reach apiConfig. Convert back and report every
	// populated desired field that did not survive the round trip.
	back, err := converter.FromAPIConfig(apiJSON, deviceType)
	if err != nil {
		return nil, fmt.Errorf("failed to check converted config: %w", err)
	}
	for _, diff := range NewPositionalConfigComparator().Compare(desired, back).Differences {
		if diff.Actual == nil {
			unsupported = append(unsupported, diff.Path)
		}
	}

	// The converter keeps a cloud server, but the device reports none and POST /settings/cloud takes
	// none, so apply could neither write it nor read it back.
	if desired.Cloud != nil && desired.Cloud.Server != nil {
		unsupported = append(unsupported, "cloud.server")
	}

	// The device keeps a CoIoT update period of 15 to 65535 seconds only, and reports multicast
	// as an empty peer, so the literal "mcast" could never read back as written.
	if desired.CoIoT != nil {
		if period := desired.CoIoT.UpdatePeriod; period != nil && (*period < gen1CoIoTMinPeriod || *period > gen1CoIoTMaxPeriod) {
			unsupported = append(unsupported, "coiot.update_period")
		}
		if peer := desired.CoIoT.Peer; peer != nil && *peer == gen1CoIoTMulticast {
			unsupported = append(unsupported, "coiot.peer")
		}
	}

	// A time server must be a host the device can resolve. The device takes any string, so a typo
	// would silently stop its clock and every schedule that depends on it. The accepted length and
	// the MQTT period range are unverified until the hardware readback (the reference gives none),
	// so the period is only refused where it cannot mean anything: below zero.
	if desired.System != nil && desired.System.SNTP != nil && desired.System.SNTP.Server != "" && !validTimeServer(desired.System.SNTP.Server) {
		unsupported = append(unsupported, "system.sntp.server")
	}
	if desired.MQTT != nil && desired.MQTT.UpdatePeriod != nil && *desired.MQTT.UpdatePeriod < 0 {
		unsupported = append(unsupported, "mqtt.update_period")
	}
	// A Gen1 device stores the broker as one host:port string, which the write joins with a colon
	// and the read-back splits at the first one. An IPv6 address (or any server with a second colon)
	// would be stored as "fd00::1:1883" and read back as "fd00". The reference documents no
	// bracketed form, so the value is refused and no form is guessed.
	if m := desired.MQTT; m != nil && m.Server != nil && (strings.Count(*m.Server, ":") > 1 || strings.HasPrefix(*m.Server, "[")) {
		unsupported = append(unsupported, "mqtt.server")
	}

	// The converted configuration holds no channel id: the list position is the channel that
	// apply writes. A stated id that disagrees with its position would change another channel
	// than the one the configuration names, so refuse it before the device is touched. An id
	// left out decodes as 0 and cannot be told from a stated 0, so the rule is strict: every
	// id equals its position. That also keeps the final verification honest, because the live
	// configuration numbers its channels by position.
	if desired.Relay != nil {
		for i, relay := range desired.Relay.Relays {
			if relay.ID != i {
				unsupported = append(unsupported, fmt.Sprintf("relay.relays[%d].id", i))
			}
			unsupported = append(unsupported, unsupportedRelayValues(i, relay)...)
		}
	}
	if desired.PowerMetering != nil && desired.PowerMetering.MaxPower != nil && *desired.PowerMetering.MaxPower < 0 {
		unsupported = append(unsupported, "power_metering.max_power")
	}

	// The device keeps a roller travel time of one second or more only. Zero or below is a value it
	// cannot run a motor on, whatever it echoes back.
	if desired.Roller != nil {
		if t := desired.Roller.MaxOpenTime; t != nil && *t < 1 {
			unsupported = append(unsupported, "roller.max_open_time")
		}
		if t := desired.Roller.MaxCloseTime; t != nil && *t < 1 {
			unsupported = append(unsupported, "roller.max_close_time")
		}
		unsupported = append(unsupported, unsupportedRollerValues(desired.Roller)...)
	}

	unsupported = append(unsupported, unsupportedInputs(desired, deviceType)...)
	unsupported = append(unsupported, unsupportedWiFi(desired.WiFi)...)

	sort.Strings(unsupported)
	return slices.Compact(unsupported), nil
}

// unsupportedInputs returns the input paths of desired that the round trip cannot see. A model
// with no inputs refuses the whole section. A mode must be one the model's button types can
// carry, and an id must equal its position for the same reason as a relay's: the list position is
// the channel that gets written. The stored name, type and push actions need no rule here: the
// converter drops them, so the round trip reports them.
func unsupportedInputs(desired *DeviceConfiguration, deviceType string) []string {
	if desired.Input == nil {
		return nil
	}
	if !hasInputSettings(deviceType) {
		return []string{"input"}
	}
	var unsupported []string
	for i, input := range desired.Input.Inputs {
		path := fmt.Sprintf("input.inputs[%d].", i)
		if input.ID != i {
			unsupported = append(unsupported, path+"id")
		}
		if input.Mode != nil {
			if _, known := gen1ButtonTypes[*input.Mode]; !known || (!inputsLiveInRelays(deviceType) && !gen1ButtonTypesOfI3[*input.Mode]) {
				unsupported = append(unsupported, path+"mode")
			}
		}
	}
	return unsupported
}

// ApplyDesiredConfig sends the device's desired configuration to the device,
// then reads it back and compares. The device is marked config_applied only
// when the read-back matches and the desired configuration did not change
// meanwhile, so a partial, failed or outdated apply stays pending.
//
// The applied flag is cleared before the first write, so a failed re-apply of a previously
// applied device reports pending, not applied.
func (s *ConfigurationService) ApplyDesiredConfig(ctx context.Context, deviceID uint, client ShellyClient, deviceType string) (*ApplyVerifyResult, error) {
	converter, check, err := s.applyPathFor(client, s.loginHooks != nil)
	if err != nil {
		return nil, err
	}

	// One apply per device at a time, from the snapshot to the flag update: the compare-and-set
	// below guards the stored configuration, this lock guards the order of writes to hardware.
	defer s.lockApply(deviceID)()

	hooks := s.loginHooks

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

	unsupported, err := check(desired, deviceType)
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
	verifier.WithPendingReboot(device.PendingReboot, func(pending bool) error {
		return s.repo.SetDevicePendingReboot(deviceID, pending)
	})
	if hooks != nil {
		// The client handed in may predate a login that an apply queued ahead of this one
		// changed, so the lock holder builds its own from the stored login.
		fresh, clientErr := hooks.Client(ctx, deviceID)
		if clientErr != nil {
			return nil, fmt.Errorf("could not build a client for the device: %w", clientErr)
		}
		client = fresh

		// Order on success: the device accepted the write, then the new credentials are stored,
		// then the client is rebuilt. A failed write reaches none of it.
		verifier.WithLogin(func(ctx context.Context, client ShellyClient, change LoginChange) (ShellyClient, error) {
			if storeErr := hooks.Store(ctx, deviceID, change); storeErr != nil {
				return nil, fmt.Errorf("the device accepted the new login but it could not be stored, so the stored login is now stale; enter it again: %w", storeErr)
			}
			rebuilt, reconnectErr := hooks.Client(ctx, deviceID)
			if reconnectErr != nil {
				return nil, fmt.Errorf("the new login was stored but the client for it could not be built: %w", reconnectErr)
			}
			return rebuilt, nil
		})
	}
	result, err := verifier.ApplyAndVerify(ctx, client, desired, deviceType)
	if err != nil {
		var refused *refusedBeforeWriteError
		if errors.As(err, &refused) && device.ConfigApplied {
			// Nothing was written, so the device still holds what the flag said. Only the snapshot
			// that was applied against is restored: an edit made meanwhile stays pending.
			if _, restoreErr := s.repo.SetDeviceConfigAppliedIfDesired(deviceID, snapshot); restoreErr != nil {
				s.logger.WithFields(map[string]any{"error": restoreErr.Error(), "device_id": deviceID}).
					Warn("Could not restore config_applied after a refused apply")
			}
		}
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
	converter, _, err := s.applyPathFor(client, s.loginHooks != nil)
	if err != nil {
		return nil, err
	}

	device, err := s.repo.GetDevice(deviceID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDeviceNotFound, err)
	}
	desired, err := parseApplicableDesiredConfig(device.DesiredConfig)
	if err != nil {
		return nil, err
	}

	verifier := NewConfigVerifier(converter, s.logger)
	return verifier.VerifyOnly(ctx, client, desired, deviceType)
}
