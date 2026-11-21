package api

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/redact"
	"github.com/ginsys/shelly-manager/internal/service"
)

// perDevicePaths is what is one device's own, so a template shared by a model or by every device must
// not carry it. A `*` stands for any list index. ui/src/utils/perDevicePaths.json holds the same list
// and a test keeps the two equal.
var perDevicePaths = []string{
	"system.device.name",
	"system.device.hostname",
	"system.device.mac",
	"mqtt.id",
	"mqtt.topic_prefix",
	"auth.realm",
	"wifi.static_ip",
	"wifi.ipv4mode",
	"network.eth.ip",
	"network.eth.ipv4mode",
	"network.wifi.sta.ip",
	"network.wifi.sta.ipv4mode",
	"network.wifi.ap.ssid",
	"wifi.ap.ssid",
	"relay.relays.*.name",
	"input.inputs.*.name",
	"location.lat",
	"location.lng",
	"system.location.lat",
	"system.location.lng",
}

// maxCommonSettingsDevices is the most devices one request names. It reads the database only, so the
// bound is the size of the answer, as for bulk-download.
const maxCommonSettingsDevices = 200

type commonSkipped struct {
	DeviceID   uint   `json:"device_id"`
	DeviceName string `json:"device_name,omitempty"`
	Reason     string `json:"reason"`
}

type commonSettingsAnswer struct {
	Compared []uint          `json:"compared"`
	Skipped  []commonSkipped `json:"skipped"`
	// DeviceType is the model of every compared device, empty when they are not all one model.
	DeviceType string         `json:"device_type"`
	Common     map[string]any `json:"common"`
	// Differing counts, per section, the leaf values that one device holds and another does not or holds
	// differently: what a section that is only partly common leaves out.
	Differing map[string]int `json:"differing"`
}

// CommonSettings handles POST /api/v1/config/common-settings. It answers the settings that are equal
// across the stored imports of the named devices, in the typed shape a template holds. It reads the
// database and never contacts a device. A device with no import, or that cannot be read, is named and
// left out: it never counts as agreeing.
//
// Each import is read in its typed form with every saved secret already masked, and then stripped of
// every secret slot, of what is one device's own, and of the raw part, before any two are compared. No
// unmasked secret is ever compared, and two different passwords, which both read as the mask, are
// dropped by the strip and not reported as agreeing.
func (h *Handler) CommonSettings(w http.ResponseWriter, r *http.Request) {
	selection, ok := h.decodeBulkRequest(w, r)
	if !ok {
		return
	}
	rw := h.responseWriter()
	if selection.StopOnError {
		rw.WriteValidationError(w, r, "options.stop_on_error does not apply to common-settings: a device that cannot be read is skipped and named")
		return
	}
	if len(selection.IDs) > maxCommonSettingsDevices {
		rw.WriteValidationError(w, r, fmt.Sprintf("common-settings takes at most %d devices, got %d", maxCommonSettingsDevices, len(selection.IDs)))
		return
	}

	answer := commonSettingsAnswer{Compared: []uint{}, Skipped: []commonSkipped{}}
	var docs []map[string]any
	models := map[string]bool{}
	for _, id := range selection.IDs {
		doc, model, skipped := h.commonDocument(id)
		if skipped != nil {
			answer.Skipped = append(answer.Skipped, *skipped)
			continue
		}
		answer.Compared = append(answer.Compared, id)
		docs = append(docs, doc)
		models[model] = true
	}
	if len(models) == 1 {
		for model := range models {
			answer.DeviceType = model
		}
	}
	answer.Common, answer.Differing = commonOf(docs)
	rw.WriteSuccess(w, r, answer)
}

// commonDocument reads one device's stored import as a masked, stripped document, or says why the device
// is skipped.
func (h *Handler) commonDocument(id uint) (map[string]any, string, *commonSkipped) {
	device, err := h.DB.GetDevice(id)
	if err != nil {
		reason := "device could not be read"
		if errors.Is(err, gorm.ErrRecordNotFound) {
			reason = "device not found"
		}
		return nil, "", &commonSkipped{DeviceID: id, Reason: reason}
	}
	skip := func(reason string) *commonSkipped {
		return &commonSkipped{DeviceID: id, DeviceName: device.Name, Reason: reason}
	}
	stored, err := h.Service.GetDeviceConfig(id)
	if err != nil || len(stored.Config) == 0 {
		if err == nil || errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, "", skip("no stored import")
		}
		return nil, "", skip("configuration could not be read")
	}
	typed, _, err := h.convertToTypedConfig(stored.Config, device)
	if err != nil {
		return nil, "", skip("configuration could not be read")
	}
	maskTypedSecrets(typed)
	encoded, err := typed.ToJSON()
	if err != nil {
		return nil, "", skip("configuration could not be read")
	}
	decoded, ok := redact.DecodeJSON(encoded)
	doc, isObject := decoded.(map[string]any)
	if !ok || !isObject {
		return nil, "", skip("configuration could not be read")
	}
	stripForCompare(doc)
	return doc, service.DeviceModel(device), nil
}

// stripForCompare removes from a masked typed configuration everything that must never be a common
// setting: the raw part, every secret slot whatever it holds (the server masks only a secret that is set,
// and an empty one would clear the password of every device a template reaches), what is one device's
// own, and any value that is still the mask.
func stripForCompare(doc map[string]any) {
	delete(doc, "raw")
	for _, path := range allSecretPaths() {
		deletePath(doc, strings.Split(path, "."))
	}
	for _, path := range perDevicePaths {
		deletePath(doc, strings.Split(path, "."))
	}
	dropMasks(doc)
}

// deletePath removes the member a dotted path names; a `*` segment stands for every element of a list.
func deletePath(node any, segments []string) {
	if len(segments) == 0 {
		return
	}
	switch n := node.(type) {
	case map[string]any:
		if len(segments) == 1 {
			delete(n, segments[0])
		} else if child, ok := n[segments[0]]; ok {
			deletePath(child, segments[1:])
		}
	case []any:
		if segments[0] == "*" {
			for _, element := range n {
				deletePath(element, segments[1:])
			}
		}
	}
}

func dropMasks(node any) {
	switch n := node.(type) {
	case map[string]any:
		for key, child := range n {
			if child == secretMask {
				delete(n, key)
				continue
			}
			dropMasks(child)
		}
	case []any:
		for _, element := range n {
			dropMasks(element)
		}
	}
}

// commonOf returns what every document holds equally, and per top-level section how many leaf values
// are left out. Objects keep the keys all documents have with a common value, a list is kept when all
// lists have one length and every element has something in common (elements pair by position), and a
// scalar is kept when all are equal. With no document nothing is common.
func commonOf(docs []map[string]any) (map[string]any, map[string]int) {
	common := map[string]any{}
	differing := map[string]int{}
	if len(docs) == 0 {
		return common, differing
	}
	values := make([]any, len(docs))
	for i, doc := range docs {
		values[i] = doc
	}
	if merged, ok := commonValue(values); ok {
		common = merged.(map[string]any)
	}
	held := map[string]bool{}
	for _, doc := range docs {
		collectLeaves("", doc, held)
	}
	kept := map[string]bool{}
	collectLeaves("", common, kept)
	for leaf := range held {
		if !kept[leaf] {
			differing[strings.SplitN(leaf, ".", 2)[0]]++
		}
	}
	return common, differing
}

// commonValue returns the part of the values that all share, and false when they share nothing.
func commonValue(values []any) (any, bool) {
	switch first := values[0].(type) {
	case map[string]any:
		maps := make([]map[string]any, len(values))
		for i, v := range values {
			m, isMap := v.(map[string]any)
			if !isMap {
				return nil, false
			}
			maps[i] = m
		}
		out := map[string]any{}
		for key := range first {
			members := make([]any, 0, len(maps))
			for _, m := range maps {
				if member, has := m[key]; has {
					members = append(members, member)
				}
			}
			if len(members) != len(maps) {
				continue
			}
			if shared, ok := commonValue(members); ok {
				out[key] = shared
			}
		}
		return out, len(out) > 0
	case []any:
		out := make([]any, len(first))
		for i := range first {
			column := make([]any, 0, len(values))
			for _, v := range values {
				list, isList := v.([]any)
				if !isList || len(list) != len(first) {
					return nil, false
				}
				column = append(column, list[i])
			}
			shared, ok := commonValue(column)
			if !ok {
				return nil, false
			}
			out[i] = shared
		}
		return out, len(out) > 0
	default:
		for _, v := range values[1:] {
			if v != first {
				return nil, false
			}
		}
		return first, true
	}
}

// collectLeaves records the dotted path of every scalar under node; a list index is a path segment.
func collectLeaves(prefix string, node any, into map[string]bool) {
	join := func(key string) string {
		if prefix == "" {
			return key
		}
		return prefix + "." + key
	}
	switch n := node.(type) {
	case map[string]any:
		keys := make([]string, 0, len(n))
		for key := range n {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			collectLeaves(join(key), n[key], into)
		}
	case []any:
		for i, element := range n {
			collectLeaves(join(fmt.Sprint(i)), element, into)
		}
	default:
		into[prefix] = true
	}
}
