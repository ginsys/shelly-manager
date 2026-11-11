package configuration

import (
	"encoding/json"
	"fmt"
	"strings"
)

// NullPaths lists the path of every field that a raw JSON body sets to null. A decoded struct
// cannot tell null from an omitted field, so the raw text is read. Keys are lower case, because the
// decoder matches keys without regard to case. A null inside an array is not a path.
func NullPaths(raw []byte) [][]string {
	var tree map[string]any
	if json.Unmarshal(raw, &tree) != nil {
		return nil
	}
	var paths [][]string
	var walk func(node map[string]any, prefix []string)
	walk = func(node map[string]any, prefix []string) {
		for key, value := range node {
			path := append(append([]string{}, prefix...), strings.ToLower(key))
			switch v := value.(type) {
			case nil:
				paths = append(paths, path)
			case map[string]any:
				walk(v, path)
			}
		}
	}
	walk(tree, nil)
	return paths
}

// RemovePaths returns a copy of config without the fields at the given paths. A section that the
// removal leaves empty is removed too. A path that is not stored changes nothing.
func RemovePaths(config *DeviceConfiguration, paths [][]string) (*DeviceConfiguration, error) {
	if len(paths) == 0 {
		return config, nil
	}
	data, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal configuration: %w", err)
	}
	var tree map[string]any
	if err = json.Unmarshal(data, &tree); err != nil {
		return nil, fmt.Errorf("failed to read configuration: %w", err)
	}
	for _, path := range paths {
		removePath(tree, path)
	}
	data, err = json.Marshal(tree)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal configuration: %w", err)
	}
	var result DeviceConfiguration
	if err = json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to read configuration: %w", err)
	}
	return &result, nil
}

// removePath deletes the key at path from node. It reports whether node is empty afterwards, so the
// caller can drop a section that the removal emptied.
func removePath(node map[string]any, path []string) bool {
	if len(path) == 0 {
		return false
	}
	for key, value := range node {
		if !strings.EqualFold(key, path[0]) {
			continue
		}
		if len(path) == 1 {
			delete(node, key)
			continue
		}
		if child, ok := value.(map[string]any); ok && removePath(child, path[1:]) {
			delete(node, key)
		}
	}
	return len(node) == 0
}
