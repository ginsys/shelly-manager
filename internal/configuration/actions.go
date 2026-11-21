package configuration

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/ginsys/shelly-manager/internal/shelly/gen1"
)

// ActionConfig is one Gen1 action: the HTTP calls a device makes when a button is pressed or an
// output changes. Name and Index identify it. Enabled and SecretURLs are each optional: an omitted
// one is neither written nor compared, and an empty SecretURLs clears the list.
//
// The URL list is named secret_urls on purpose. A URL can carry a password or a token, and every
// masking rule in the manager matches a key by its name: a key that contains "secret" is masked on
// every read path (history, export, drift, the typed config). The device's own key is urls[], used
// on the wire only. SecretURLs is a pointer so that an empty list survives omitempty: a clear that
// vanished at the first store would turn into "left alone".
type ActionConfig struct {
	Name       string    `json:"name"`
	Index      int       `json:"index"`
	Enabled    *bool     `json:"enabled,omitempty"`
	SecretURLs *[]string `json:"secret_urls,omitempty"`
}

// The limits of a Gen1 action: at most 5 URLs per action, each at most 256 bytes (the reference
// states both since firmware 1.8.0). The URL scheme rule is not on the reference: every example is
// http, and the device has no stated https support, so only http is accepted until a hardware read
// shows otherwise.
const (
	gen1ActionMaxURLs      = 5
	gen1ActionMaxURLLength = 256
	gen1ActionScheme       = "http://"
)

// gen1ActionRow is what a model keeps: the action names it has, and how many indexes each name has
// (one per channel).
type gen1ActionRow struct {
	names   []string
	indexes int
}

var (
	gen1RelayActionNames = []string{"btn_on_url", "btn_off_url", "out_on_url", "out_off_url", "longpush_url", "shortpush_url"}
	// The plug names are unverified: the page the reference gives lists them in summary only.
	gen1PlugActionNames = []string{"btn_on_url", "out_on_url", "out_off_url"}
)

// gen1ActionTable returns the actions a supported model keeps. The lp_* and ext_* names need an
// add-on and roller_* names belong to roller mode, so neither is in this table. A model with no row
// (the Shelly i3 included: its inputs have their own names and counts) has no actions section here.
func gen1ActionTable(deviceType string) (gen1ActionRow, bool) {
	switch deviceType {
	case "SHSW-1", "SHSW-PM":
		return gen1ActionRow{names: gen1RelayActionNames, indexes: 1}, true
	case "SHSW-25":
		return gen1ActionRow{names: gen1RelayActionNames, indexes: 2}, true
	case "SHPLG-S", "SHPLG-1":
		return gen1ActionRow{names: gen1PlugActionNames, indexes: 1}, true
	default:
		return gen1ActionRow{}, false
	}
}

// gen1AllActionRow is the union of every row, for a template that can reach any model.
func gen1AllActionRow() gen1ActionRow {
	names := slices.Clone(gen1RelayActionNames)
	for _, name := range gen1PlugActionNames {
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	return gen1ActionRow{names: names, indexes: 2}
}

// validActionURL reports whether the device can keep url: http, at least one byte after the
// scheme, at most 256 bytes in all (the stricter reading of the limit for a multibyte character),
// and no space or control character.
func validActionURL(url string) bool {
	if len(url) <= len(gen1ActionScheme) || len(url) > gen1ActionMaxURLLength || !strings.HasPrefix(url, gen1ActionScheme) {
		return false
	}
	return !strings.ContainsFunc(url, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) })
}

// unsupportedActions returns the action paths of desired that the device could not keep. A model
// with no actions row refuses the whole section. A path names the entry and the field, never a URL.
func unsupportedActions(desired *DeviceConfiguration, deviceType string) []string {
	if desired.Actions == nil {
		return nil
	}
	row, ok := gen1ActionTable(deviceType)
	if !ok {
		return []string{"actions"}
	}
	var unsupported []string
	seen := map[string]bool{}
	for i, entry := range *desired.Actions {
		path := fmt.Sprintf("actions[%d].", i)
		if !slices.Contains(row.names, entry.Name) {
			unsupported = append(unsupported, path+"name")
		}
		if entry.Index < 0 || entry.Index >= row.indexes {
			unsupported = append(unsupported, path+"index")
		}
		// Two entries for one action would write twice and read one back: the later one is refused.
		identity := fmt.Sprintf("%q/%d", entry.Name, entry.Index)
		if seen[identity] {
			unsupported = append(unsupported, path+"name")
		}
		seen[identity] = true
		if entry.SecretURLs != nil && (len(*entry.SecretURLs) > gen1ActionMaxURLs || slices.ContainsFunc(*entry.SecretURLs, func(u string) bool { return !validActionURL(u) })) {
			unsupported = append(unsupported, path+"secret_urls")
		}
	}
	return unsupported
}

// readActions fills imported.Actions from GET /settings/actions, for the entries desired lists
// only, in desired's order: one entry per desired (name, index) pair, carrying enabled and the URL
// list only where desired has them. A pair the device does not report is an entry with nothing set,
// so the comparator reports it at its own position instead of shifting the next entry. A model with
// no actions row is not read: its imported section stays empty, and the comparator reports the
// section that the device cannot keep.
func readActions(ctx context.Context, client ShellyClient, desired *DeviceConfiguration, deviceType string, imported *DeviceConfiguration) error {
	if _, ok := gen1ActionTable(deviceType); !ok {
		return nil
	}
	actionClient, ok := client.(ActionSettingsClient)
	if !ok {
		return errors.New("failed to read the device actions: " + errNoActionClient.Error())
	}
	live, err := actionClient.GetActions(ctx)
	if err != nil {
		return &deviceReadError{fmt.Errorf("failed to read the device actions: %w", err)}
	}
	entries := make([]ActionConfig, 0, len(*desired.Actions))
	for _, want := range *desired.Actions {
		entry := ActionConfig{Name: want.Name, Index: want.Index}
		if reported, found := liveAction(live, want.Name, want.Index); found {
			if want.Enabled != nil {
				enabled := reported.Enabled
				entry.Enabled = &enabled
			}
			if want.SecretURLs != nil {
				urls := append([]string{}, reported.URLs...)
				entry.SecretURLs = &urls
			}
		}
		entries = append(entries, entry)
	}
	imported.Actions = &entries
	return nil
}

// redactedActionURL replaces an action URL in a difference. A URL can hold a password or a token.
const redactedActionURL = "[redacted]"

// scrubActionValue returns value as a ConfigDifference may hold it: no action URL in it. A value at
// a path with a secret_urls segment becomes the fixed text, and an ActionConfig, a list of them, or
// a whole DeviceConfiguration (a difference of a config with a nil side) is returned as a copy whose
// URL lists are the one-element list holding that text. nil stays nil, which means "absent".
func scrubActionValue(path string, value interface{}) interface{} {
	if value == nil {
		return nil
	}
	if slices.Contains(strings.Split(path, "."), "secret_urls") {
		return redactedActionURL
	}
	scrubEntry := func(entry ActionConfig) ActionConfig {
		if entry.SecretURLs != nil {
			entry.SecretURLs = &[]string{redactedActionURL}
		}
		return entry
	}
	scrubList := func(entries []ActionConfig) []ActionConfig {
		out := make([]ActionConfig, len(entries))
		for i, entry := range entries {
			out[i] = scrubEntry(entry)
		}
		return out
	}
	switch v := value.(type) {
	case ActionConfig:
		return scrubEntry(v)
	case *ActionConfig:
		if v == nil {
			return v
		}
		scrubbed := scrubEntry(*v)
		return &scrubbed
	case []ActionConfig:
		return scrubList(v)
	case *[]ActionConfig:
		if v == nil {
			return v
		}
		scrubbed := scrubList(*v)
		return &scrubbed
	case *DeviceConfiguration:
		if v == nil || v.Actions == nil {
			return v
		}
		cp := *v
		scrubbed := scrubList(*v.Actions)
		cp.Actions = &scrubbed
		return &cp
	}
	return value
}

// getActionsSchema describes DeviceConfiguration.Actions for one model: the names and indexes it
// keeps, up to 5 http URLs of at most 256 characters each. A template with no model gets every name.
func getActionsSchema(deviceType string) map[string]interface{} {
	row, ok := gen1ActionTable(deviceType)
	if !ok {
		row = gen1AllActionRow()
	}
	return map[string]interface{}{
		"type":        "array",
		"title":       "Actions",
		"description": "HTTP calls the device makes when a button is pressed or an output changes. A layer that lists actions names all of them: an action it leaves out is not written, and an empty URL list clears one.",
		"items": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"title":       "Action",
					"description": "The device's own action name",
					"enum":        slices.Clone(row.names),
				},
				"index": map[string]interface{}{
					"type":        "integer",
					"title":       "Channel",
					"description": "The channel the action belongs to, from 0",
					"minimum":     0,
					"maximum":     row.indexes - 1,
				},
				"enabled": map[string]interface{}{
					"type":  "boolean",
					"title": "Enabled",
				},
				"secret_urls": map[string]interface{}{
					"type":        "array",
					"title":       "URLs",
					"description": "Up to 5 http URLs, each up to 256 characters. A URL can hold a password or a token, so it is masked everywhere it is shown.",
					"maxItems":    gen1ActionMaxURLs,
					"items": map[string]interface{}{
						"type":      "string",
						"maxLength": gen1ActionMaxURLLength,
					},
				},
			},
		},
	}
}

// withoutActions returns a shallow copy of config with no actions. The converter does not carry
// them (GET /settings lists action names, not values), so the round trip that finds the settings a
// model drops must not see them.
func withoutActions(config *DeviceConfiguration) *DeviceConfiguration {
	if config.Actions == nil {
		return config
	}
	cp := *config
	cp.Actions = nil
	return &cp
}

// ActionSettingsClient is the actions surface of a Gen1 client: POST and GET on /settings/actions.
// A client without it cannot apply or verify actions, and apply reports that as a failure.
type ActionSettingsClient interface {
	SetActionForm(ctx context.Context, index int, name string, enabled *bool, urls []string) error
	GetActions(ctx context.Context) (map[string][]gen1.ActionEntry, error)
}

var errNoActionClient = errors.New("client cannot write device actions")

// actionKeys counts the keys an entry writes: enabled and the URL list.
func actionKeys(entry ActionConfig) int {
	n := 0
	if entry.Enabled != nil {
		n++
	}
	if entry.SecretURLs != nil {
		n++
	}
	return n
}

// countActionSettings counts the settings the actions section writes.
func countActionSettings(config *DeviceConfiguration) int {
	if config == nil || config.Actions == nil {
		return 0
	}
	count := 0
	for _, entry := range *config.Actions {
		count += actionKeys(entry)
	}
	return count
}

// liveAction finds the entry a device reports for an action name and index.
func liveAction(live map[string][]gen1.ActionEntry, name string, index int) (gen1.ActionEntry, bool) {
	for _, entry := range live[name] {
		if entry.Index == index {
			return entry, true
		}
	}
	return gen1.ActionEntry{}, false
}

// actionSummary is what a failure says about an entry: the action and the number of URLs, never a
// URL (a URL can hold a password or a token).
func actionSummary(entry ActionConfig) string {
	summary := fmt.Sprintf("%s/%d", entry.Name, entry.Index)
	if entry.SecretURLs != nil {
		summary += fmt.Sprintf(", %d URL", len(*entry.SecretURLs))
		if len(*entry.SecretURLs) != 1 {
			summary += "s"
		}
	}
	return summary
}

// failActionEntry records every key of one entry as failed.
func failActionEntry(result *ApplyResult, position int, entry ActionConfig, message string) {
	add := func(field string) {
		result.Failures = append(result.Failures, ApplyFailure{
			Path:  fmt.Sprintf("actions[%d].%s", position, field),
			Value: actionSummary(entry),
			Error: message,
		})
		result.FailedCount++
	}
	if entry.Enabled != nil {
		add("enabled")
	}
	if entry.SecretURLs != nil {
		add("secret_urls")
	}
}

// hideActionURLs removes every URL of the entry from an error's text.
func hideActionURLs(err error, entry ActionConfig) error {
	if entry.SecretURLs != nil {
		for _, u := range *entry.SecretURLs {
			err = redact(err, u)
		}
	}
	return err
}

// applyActionSettings writes each listed action with POST /settings/actions and reads it back with
// GET /settings/actions, comparing every key just written: a device can answer 200 and keep the old
// value. An action the desired config does not list is never written and never cleared. A failure
// names the action, the field and the number of URLs, never a URL.
func (a *ConfigApplier) applyActionSettings(ctx context.Context, client ShellyClient, config *DeviceConfiguration, result *ApplyResult) error {
	if countActionSettings(config) == 0 {
		return nil
	}
	actionClient, ok := client.(ActionSettingsClient)
	if !ok {
		return failSection(result, "actions", errNoActionClient)
	}

	var firstErr error
	for position, entry := range *config.Actions {
		if actionKeys(entry) == 0 {
			continue
		}
		var urls []string
		if entry.SecretURLs != nil {
			urls = *entry.SecretURLs
			if urls == nil {
				urls = []string{}
			}
		}
		if err := actionClient.SetActionForm(ctx, entry.Index, entry.Name, entry.Enabled, urls); err != nil {
			err = hideActionURLs(err, entry)
			failActionEntry(result, position, entry, err.Error())
			firstErr = firstError(firstErr, err)
			continue
		}

		live, err := actionClient.GetActions(ctx)
		if err != nil {
			err = hideActionURLs(err, entry)
			failActionEntry(result, position, entry, "read-back failed: "+err.Error())
			firstErr = firstError(firstErr, err)
			continue
		}
		reported, found := liveAction(live, entry.Name, entry.Index)
		for _, key := range []string{"enabled", "secret_urls"} {
			switch {
			case key == "enabled" && entry.Enabled == nil, key == "secret_urls" && entry.SecretURLs == nil:
				continue
			case !found:
				result.Failures = append(result.Failures, ApplyFailure{
					Path:  fmt.Sprintf("actions[%d].%s", position, key),
					Value: actionSummary(entry),
					Error: "device reports no such action after the write",
				})
				result.FailedCount++
			case key == "enabled" && reported.Enabled == *entry.Enabled,
				key == "secret_urls" && slices.Equal(reported.URLs, urls):
				result.AppliedCount++
			case key == "enabled":
				result.Failures = append(result.Failures, ApplyFailure{
					Path:  fmt.Sprintf("actions[%d].enabled", position),
					Value: actionSummary(entry),
					Error: fmt.Sprintf("device reports %t after the write", reported.Enabled),
				})
				result.FailedCount++
			default:
				result.Failures = append(result.Failures, ApplyFailure{
					Path:  fmt.Sprintf("actions[%d].secret_urls", position),
					Value: actionSummary(entry),
					Error: fmt.Sprintf("device reports %d URLs after the write", len(reported.URLs)),
				})
				result.FailedCount++
			}
		}
	}
	return firstErr
}
