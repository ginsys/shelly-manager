package configuration

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	secrets "github.com/ginsys/shelly-manager/internal/redact"
)

// secretBindings names, for each secret slot, the fields that say where the secret is sent (note 3.3).
// A rollback keeps the current secret at a slot only while every bound field is equal in the target and
// in the current desired configuration. A slot with no bound field counts as changed. The keys are the
// JSON paths of secretSlots in internal/api, and a test there fails for a slot with no row here.
var secretBindings = map[string][]string{
	"wifi.password":         {"wifi.ssid"},
	"wifi.ap.pass":          {"wifi.ap.ssid"},
	"wifi.ap.key":           nil, // apply refuses it: no destination can be named
	"mqtt.pass":             {"mqtt.server", "mqtt.port", "mqtt.user", "mqtt.ssl_ca", "mqtt.enable_sni"},
	"auth.pass":             {"auth.user"},
	"network.wifi.sta.pass": {"network.wifi.sta.ssid"},
	"network.wifi.ap.pass":  {"network.wifi.ap.ssid"},
}

// SecretBindingPaths lists the slots that have a binding row, sorted.
func SecretBindingPaths() []string {
	paths := make([]string, 0, len(secretBindings))
	for path := range secretBindings {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

// DuplicateActionSlotMark follows the base key of an action whose identity another entry of the same
// configuration shares, then the entry's list position.
const DuplicateActionSlotMark = ".secret_urls#"

// ActionSlotKeys returns the slot key of each action entry, by position: `actions."<name>".<index>.
// secret_urls`, with the name quoted so a dotted name cannot read as another key. Where two entries
// share an identity (stored data can; only apply refuses it) each gets the key with its list position.
func ActionSlotKeys(entries []ActionConfig) []string {
	base := func(e ActionConfig) string { return fmt.Sprintf("actions.%q.%d.secret_urls", e.Name, e.Index) }
	count := map[string]int{}
	for _, entry := range entries {
		count[base(entry)]++
	}
	keys := make([]string, len(entries))
	for i, entry := range entries {
		keys[i] = base(entry)
		if count[keys[i]] > 1 {
			keys[i] = strings.TrimSuffix(keys[i], ".secret_urls") + DuplicateActionSlotMark + strconv.Itoa(i)
		}
	}
	return keys
}

// secretRef is a place in the target that holds the mask. key is what the operator sees and answers
// with: the JSON path, or the slot key of an action's URL list. list says the slot takes a list of URLs.
type secretRef struct {
	key  string
	path []string
	list bool
}

// maskedRefs walks the target for every value that is the mask, whatever its path, in key order.
func maskedRefs(target map[string]any) []secretRef {
	var refs []secretRef
	var walk func(v any, path []string)
	walk = func(v any, path []string) {
		switch x := v.(type) {
		case map[string]any:
			for k, child := range x {
				walk(child, append(path[:len(path):len(path)], k))
			}
		case []any:
			for i, child := range x {
				walk(child, append(path[:len(path):len(path)], strconv.Itoa(i)))
			}
		case string:
			if x == secrets.Placeholder {
				refs = append(refs, secretRef{key: formatPath(path), path: path})
			}
		}
	}
	walk(target, nil)

	// A mask inside an action's URL list stands for the whole list: one slot, named by the entry's key.
	keys := treeActionKeys(target)
	seen := map[string]bool{}
	out := refs[:0]
	for _, ref := range refs {
		if p := ref.path; len(p) >= 3 && p[0] == "actions" && p[2] == "secret_urls" {
			if n, err := strconv.Atoi(p[1]); err == nil && n >= 0 && n < len(keys) {
				ref = secretRef{key: keys[n], path: p[:3], list: true}
			}
		}
		if !seen[ref.key] {
			seen[ref.key] = true
			out = append(out, ref)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].key < out[j].key })
	return out
}

// treeActionKeys reads the identity of each action entry of a tree decoded by redact.DecodeJSON, which
// keeps a number as json.Number: a tree decoded another way reads every index as 0.
func treeActionKeys(tree map[string]any) []string {
	list, _ := tree["actions"].([]any)
	entries := make([]ActionConfig, len(list))
	for i, item := range list {
		if m, ok := item.(map[string]any); ok {
			entries[i].Name, _ = m["name"].(string)
			if n, ok := m["index"].(interface{ Int64() (int64, error) }); ok {
				idx, _ := n.Int64()
				entries[i].Index = int(idx)
			}
		}
	}
	return ActionSlotKeys(entries)
}

// valueAt returns the value at a dotted path of a decoded tree.
func valueAt(tree any, path []string) (any, bool) {
	for _, step := range path {
		switch node := tree.(type) {
		case map[string]any:
			next, ok := node[step]
			if !ok {
				return nil, false
			}
			tree = next
		case []any:
			n, err := strconv.Atoi(step)
			if err != nil || n < 0 || n >= len(node) {
				return nil, false
			}
			tree = node[n]
		default:
			return nil, false
		}
	}
	return tree, true
}

// setAt stores v at a path of the target, which holds the mask there.
func setAt(tree any, path []string, v any) {
	parent, _ := valueAt(tree, path[:len(path)-1])
	last := path[len(path)-1]
	switch node := parent.(type) {
	case map[string]any:
		node[last] = v
	case []any:
		if n, err := strconv.Atoi(last); err == nil && n >= 0 && n < len(node) {
			node[n] = v
		}
	}
}

// sameBinding reports whether every bound field holds the same value in both trees, compared exactly:
// a number keeps its spelling, and a list or an object compares whole. A field that is absent from both
// is equal; absent from one and present in the other is not.
func sameBinding(target, current any, fields []string) bool {
	for _, field := range fields {
		path := strings.Split(field, ".")
		a, aOK := valueAt(target, path)
		b, bOK := valueAt(current, path)
		if aOK != bOK || (aOK && !reflect.DeepEqual(a, b)) {
			return false
		}
	}
	return true
}

// suppliedSecret checks what the operator entered for a slot: a non-empty string that is not the mask,
// or for a URL list a non-empty list of such strings. An empty value is never a clear: the target
// holds a secret there, and a rollback to a version that held one with none is not that version.
func suppliedSecret(v any, list bool) (any, bool) {
	valid := func(s any) bool {
		str, ok := s.(string)
		return ok && str != "" && str != secrets.Placeholder
	}
	if !list {
		return v, valid(v)
	}
	items, ok := v.([]any)
	if !ok || len(items) == 0 {
		return nil, false
	}
	for _, item := range items {
		if !valid(item) {
			return nil, false
		}
	}
	return items, true
}

// resolveSecrets replaces each mask of the target (note 3.3). A secret stays out of history, so no mask
// is ever restored: the current secret is kept only where the slot has a binding and every bound field
// is equal in the target and the current desired configuration; every other mask goes on the list the
// operator answers with supplied, keyed by that path. It returns the keys still without a value, and a
// RollbackBadSecrets error when supplied names a key off the list or holds a value that is empty, the
// mask or of the wrong type. Nothing it returns holds a secret.
func resolveSecrets(target map[string]any, current any, supplied map[string]any) ([]string, *RollbackError) {
	var listed []secretRef
	var stuck []string
	for _, ref := range maskedRefs(target) {
		bound, hasRow := secretBindings[ref.key]
		if !hasRow && !ref.list {
			// A mask where no secret slot is: history cannot give it back and nothing can be entered.
			stuck = append(stuck, ref.key)
			continue
		}
		held, _ := valueAt(current, ref.path)
		if str, isStr := held.(string); hasRow && len(bound) > 0 && isStr && str != "" &&
			str != secrets.Placeholder && sameBinding(target, current, bound) {
			setAt(target, ref.path, str)
			continue
		}
		listed = append(listed, ref)
	}

	if len(stuck) > 0 {
		return nil, &RollbackError{Kind: RollbackNotRestorable, Paths: stuck}
	}

	onList := map[string]secretRef{}
	for _, ref := range listed {
		onList[ref.key] = ref
	}
	for key, value := range supplied {
		ref, ok := onList[key]
		if _, valid := suppliedSecret(value, ref.list); !ok || !valid {
			return nil, &RollbackError{Kind: RollbackBadSecrets}
		}
	}

	var needed []string
	for _, ref := range listed {
		value, given := supplied[ref.key]
		if !given {
			needed = append(needed, ref.key)
			continue
		}
		setAt(target, ref.path, value)
	}
	return needed, nil
}
