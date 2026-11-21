package configuration

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	secrets "github.com/ginsys/shelly-manager/internal/redact"
)

var (
	// ErrHistoryNotFound is a version id that is not one of this device's.
	ErrHistoryNotFound = errors.New("no such desired configuration version for this device")
	// ErrVersionNotRestorable is a version whose configuration was not stored, because it could not be
	// masked: there is nothing to roll back to.
	ErrVersionNotRestorable = errors.New("this desired configuration version holds no stored configuration")
)

// DesiredWrite is the template ids, overrides and desired configuration a rollback stores.
type DesiredWrite struct {
	TemplateIDs string
	Overrides   string
	Desired     string
}

// RollbackKind says why a rollback was refused with the paths that caused it.
type RollbackKind int

const (
	// RollbackNeedsSecrets: the version holds a masked value that cannot be kept, and a history row never
	// holds a secret. Paths are the keys the operator answers with.
	RollbackNeedsSecrets RollbackKind = iota + 1
	// RollbackLoss: storing the version and recomputing it would change it.
	RollbackLoss
	// RollbackBadSecrets: the secrets entered name a key off the list, or hold an empty value, the mask
	// or a value of the wrong type. It carries no paths: a refusal never repeats what was sent.
	RollbackBadSecrets
)

// RollbackError is a refusal. Paths name the places in the configuration, never a value, so an error, a
// log line and an answer built from it hold no secret.
type RollbackError struct {
	Kind  RollbackKind
	Paths []string
}

func (e *RollbackError) Error() string {
	switch e.Kind {
	case RollbackNeedsSecrets:
		return fmt.Sprintf("the version holds %d masked value(s) that history cannot restore", len(e.Paths))
	case RollbackBadSecrets:
		return "the secrets entered are not an answer to the list"
	default:
		return fmt.Sprintf("the version would change when the configuration is recomputed at %d place(s)", len(e.Paths))
	}
}

// RollbackResult says what a rollback did. NoChange means the device already held the version, so
// nothing was written.
type RollbackResult struct {
	VersionID           uint
	NoChange            bool
	DetachedTemplateIDs []uint
}

// templateIDsNone reports whether a stored template id text means no templates.
func templateIDsNone(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" || text == "null" {
		return true
	}
	var ids []uint
	return json.Unmarshal([]byte(text), &ids) == nil && len(ids) == 0
}

// RollbackDesired makes a stored version the device's desired configuration: the whole version becomes
// the device's overrides, the templates are detached, and the device is pending until an apply. No device
// is contacted. A secret is never restored from history: the current secret is kept where its destination
// is unchanged, and every other masked value is refused until the operator enters it in supplied, keyed by
// the path the refusal listed. It writes nothing when a masked value is left, when the recompute would
// change the version, or when the device changed since the read. A rollback to what the device already
// holds writes nothing and records no row.
func (s *ConfigurationService) RollbackDesired(deviceID, historyID uint, supplied map[string]any) (*RollbackResult, error) {
	device, err := s.repo.GetDevice(deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get device: %w", err)
	}
	version, err := s.repo.GetDesiredHistory(deviceID, historyID)
	if err != nil {
		return nil, err
	}
	if version.NewDesired == nil {
		return nil, ErrVersionNotRestorable
	}
	raw := []byte(*version.NewDesired)
	doc, ok := secrets.DecodeJSON(raw)
	target, isObject := doc.(map[string]any)
	if !ok || !isObject || secrets.HasRepeatedMember(raw) {
		return nil, ErrVersionNotRestorable
	}

	current, _ := secrets.DecodeJSON([]byte(device.DesiredConfig))
	needed, bad := resolveSecrets(target, current, supplied)
	if bad != nil {
		return nil, bad
	}
	if len(needed) > 0 {
		return nil, &RollbackError{Kind: RollbackNeedsSecrets, Paths: needed}
	}

	encoded, err := secrets.EncodeJSON(target)
	if err != nil {
		return nil, fmt.Errorf("failed to encode the version: %w", err)
	}
	overrides := string(encoded)

	// The recompute's own path, on the target as the only layer.
	var layers []ConfigLayer
	if layer, found := overrideLayer(overrides); found {
		layers = append(layers, layer)
	}
	desired, err := s.desiredFromLayers(layers)
	if err != nil {
		return nil, err
	}
	merged, ok := secrets.DecodeJSON(desired)
	if !ok {
		return nil, errors.New("failed to read the recomputed configuration")
	}
	if paths := treeLoss(target, merged); len(paths) > 0 {
		return nil, &RollbackError{Kind: RollbackLoss, Paths: paths}
	}

	result := &RollbackResult{VersionID: historyID}
	if !templateIDsNone(device.TemplateIDs) {
		if ids, parseErr := s.parseTemplateIDs(device.TemplateIDs); parseErr == nil {
			result.DetachedTemplateIDs = ids
		}
	}
	if string(desired) == device.DesiredConfig && overrides == device.Overrides && templateIDsNone(device.TemplateIDs) {
		result.NoChange = true
		return result, nil
	}

	read := DesiredRead{TemplateIDs: device.TemplateIDs, Overrides: device.Overrides, Desired: device.DesiredConfig}
	write := DesiredWrite{TemplateIDs: "[]", Overrides: overrides, Desired: string(desired)}
	if err := s.repo.RollbackDeviceDesired(deviceID, read, write, historyID); err != nil {
		return nil, fmt.Errorf("failed to roll back the desired config: %w", err)
	}
	s.logger.WithFields(map[string]any{
		"device_id":  deviceID,
		"version_id": historyID,
		"component":  "config_service",
	}).Info("Desired configuration rolled back")
	return result, nil
}

// treeLoss is the round trip of note 3.2: the paths where the recomputed tree is not the target. A path
// the target holds that the recomputed tree lacks or holds with another value is a loss. A path only the
// recomputed tree holds is no loss when it is a JSON zero (the typed model writes `enable:false`), and
// is one when it holds anything else.
func treeLoss(target, merged any) []string {
	var out []string
	var walk func(t, m any, path []string)
	walk = func(t, m any, path []string) {
		switch tv := t.(type) {
		case map[string]any:
			mv, ok := m.(map[string]any)
			if !ok {
				out = append(out, formatPath(path))
				return
			}
			for k, child := range tv {
				p := append(path[:len(path):len(path)], k)
				other, held := mv[k]
				if !held {
					out = append(out, formatPath(p))
					continue
				}
				walk(child, other, p)
			}
			for k, other := range mv {
				if _, held := tv[k]; !held {
					nonZeroPaths(other, append(path[:len(path):len(path)], k), &out)
				}
			}
		case []any:
			mv, ok := m.([]any)
			if !ok || len(mv) != len(tv) {
				out = append(out, formatPath(path))
				return
			}
			for i, child := range tv {
				walk(child, mv[i], append(path[:len(path):len(path)], strconv.Itoa(i)))
			}
		default:
			if !sameScalar(t, m) {
				out = append(out, formatPath(path))
			}
		}
	}
	walk(target, merged, nil)
	sort.Strings(out)
	return out
}

// nonZeroPaths appends the path of every value under v that is not a JSON zero.
func nonZeroPaths(v any, path []string, out *[]string) {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			nonZeroPaths(child, append(path[:len(path):len(path)], k), out)
		}
	case []any:
		for i, child := range x {
			nonZeroPaths(child, append(path[:len(path):len(path)], strconv.Itoa(i)), out)
		}
	default:
		if !isZeroScalar(v) {
			*out = append(*out, formatPath(path))
		}
	}
}

func isZeroScalar(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case bool:
		return !x
	case string:
		return x == ""
	case interface{ String() string }: // a json.Number
		return secrets.CanonicalNumber(x.String()) == "0"
	}
	return false
}

func sameScalar(a, b any) bool {
	an, aNum := a.(interface{ String() string })
	bn, bNum := b.(interface{ String() string })
	if aNum && bNum {
		return secrets.CanonicalNumber(an.String()) == secrets.CanonicalNumber(bn.String())
	}
	return a == b
}

// formatPath joins a path with dots. A key that holds a dot, a quote or nothing is quoted, so one path
// reads one way.
func formatPath(path []string) string {
	parts := make([]string, len(path))
	for i, p := range path {
		if p == "" || strings.ContainsAny(p, `."`) {
			p = strconv.Quote(p)
		}
		parts[i] = p
	}
	return strings.Join(parts, ".")
}
