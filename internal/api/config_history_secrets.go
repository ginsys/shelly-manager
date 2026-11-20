package api

import (
	"encoding/json"
	"strings"

	"github.com/ginsys/shelly-manager/internal/configuration"
	"github.com/ginsys/shelly-manager/internal/redact"
)

// maskConfigHistory returns a copy of the history rows that is safe to send out. A row holds the
// device configuration before and after a change, and the differences between the two, so all
// three follow the rules of maskDeviceConfig. The stored rows are never changed.
func maskConfigHistory(rows []configuration.ConfigHistory) []configuration.ConfigHistory {
	out := make([]configuration.ConfigHistory, len(rows))
	for i, row := range rows {
		row.OldConfig = maskRawJSON(row.OldConfig)
		row.NewConfig = maskRawJSON(row.NewConfig)
		row.Changes = maskChanges(row.Changes)
		out[i] = row
	}
	return out
}

// maskedWhole is a text that cannot be read for secrets, sent as a JSON string.
var maskedWhole = json.RawMessage(`"` + redact.Placeholder + `"`)

// maskChanges masks the values of a list of differences ({path, expected, actual, ...}). A value
// has no key of its own, so it is masked at the place its path names: the value is wrapped in the
// path's own keys and masked by key name, so a secret leaf, a secret ancestor ("api_key.value") and a
// removed or added section that holds a secret are all masked, as the configuration itself is. Any
// other field of a difference is masked by key name too. A shape that is not a list of difference
// objects, each with a text path, cannot be read that way and is masked whole.
func maskChanges(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	doc, ok := redact.DecodeJSON(raw)
	if !ok {
		return maskedWhole
	}
	if doc == nil {
		return raw // the JSON null: no changes were recorded
	}
	list, isList := doc.([]any)
	if !isList {
		return maskedWhole
	}
	for _, item := range list {
		diff, isMap := item.(map[string]any)
		if !isMap {
			return maskedWhole
		}
		path, hasPath := diff["path"].(string)
		if !hasPath {
			return maskedWhole
		}
		// The history producer joins the keys of a path with dots and keeps every key as it is.
		segments := strings.Split(path, ".")
		for _, side := range []string{"expected", "actual"} {
			if value, present := diff[side]; present {
				diff[side] = maskAtPath(segments, value)
			}
		}
		redact.MaskTree(diff, nil)
	}
	masked, err := json.Marshal(list)
	if err != nil {
		return maskedWhole
	}
	return masked
}

// maskAtPath masks value as it would be masked inside a configuration at the path: the value is
// wrapped in one object per segment, and unwrapped after the masking. A secret segment on the way
// replaces everything below it, and that placeholder is what comes back.
func maskAtPath(segments []string, value any) any {
	doc := value
	for i := len(segments) - 1; i >= 0; i-- {
		doc = map[string]any{segments[i]: doc}
	}
	redact.MaskTree(doc, nil)
	for _, segment := range segments {
		node, isMap := doc.(map[string]any)
		if !isMap {
			return doc
		}
		doc = node[segment]
	}
	return doc
}
