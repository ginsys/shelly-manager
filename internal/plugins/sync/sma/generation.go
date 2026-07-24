package sma

import (
	"compress/gzip"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"

	jsoncanonicalizer "github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
	"github.com/google/uuid"

	"github.com/ginsys/shelly-manager/internal/sync"
)

type preparedArchive struct {
	exportID        string
	deviceCount     int
	templateCount   int
	discoveredCount int
	recordCount     int
}

func (s *SMAPlugin) prepareArchive(data *sync.ExportData, config sync.ExportConfig) (map[string]interface{}, preparedArchive, error) {
	if data == nil {
		return nil, preparedArchive{}, fmt.Errorf("export data is nil")
	}
	devices, err := joinConfigurations(data.Devices, data.Configurations)
	if err != nil {
		return nil, preparedArchive{}, err
	}
	includeDiscovered, err := effectiveBool(config.Config, "include_discovered", true)
	if err != nil {
		return nil, preparedArchive{}, err
	}
	excludeSensitive, err := effectiveBool(config.Config, "exclude_sensitive", true)
	if err != nil {
		return nil, preparedArchive{}, err
	}
	templates := append([]sync.TemplateData(nil), data.Templates...)
	discovered := []sync.DiscoveredDeviceData{}
	if includeDiscovered {
		discovered = append(discovered, data.DiscoveredDevices...)
	}
	if len(devices)+len(templates)+len(discovered) == 0 {
		return nil, preparedArchive{}, fmt.Errorf("%w: archive is empty after filtering", sync.ErrInvalidExportData)
	}

	exportID := data.Metadata.ExportID
	if exportID == "" {
		exportID = uuid.NewString()
	}
	if !isLowerUUID(exportID) {
		return nil, preparedArchive{}, fmt.Errorf("export_id must be a lowercase UUID")
	}
	created := data.Timestamp
	if created.IsZero() {
		created = time.Now()
	}
	createdBy := strings.TrimSpace(data.Metadata.RequestedBy)
	if createdBy == "" {
		createdBy = "shelly-manager"
	}
	exportType := data.Metadata.ExportType
	if exportType == "" {
		exportType = "manual"
	}
	if exportType != "manual" && exportType != "api" {
		return nil, preparedArchive{}, fmt.Errorf("invalid export_type %q", exportType)
	}
	databaseType, err := normalizeDatabaseType(data.Metadata.DatabaseType)
	if err != nil {
		return nil, preparedArchive{}, err
	}
	version := strings.TrimSpace(data.Metadata.SystemVersion)
	if version == "" {
		version = "unknown"
	}
	hostname, err := os.Hostname()
	if err != nil || strings.TrimSpace(hostname) == "" {
		hostname = "localhost"
	}

	deviceTrees := make([]interface{}, len(devices))
	for i := range devices {
		deviceTrees[i], err = buildDevice(devices[i])
		if err != nil {
			return nil, preparedArchive{}, fmt.Errorf("device %d: %w", i, err)
		}
	}
	templateTrees := make([]interface{}, len(templates))
	for i := range templates {
		templateTrees[i], err = buildTemplate(templates[i])
		if err != nil {
			return nil, preparedArchive{}, fmt.Errorf("template %d: %w", i, err)
		}
	}
	discoveredTrees := make([]interface{}, len(discovered))
	for i := range discovered {
		discoveredTrees[i], err = buildDiscovered(discovered[i])
		if err != nil {
			return nil, preparedArchive{}, fmt.Errorf("discovered device %d: %w", i, err)
		}
	}

	network := map[string]interface{}{
		"wifi_networks": []interface{}{},
		"ntp_servers":   []interface{}{},
	}
	pluginConfigs := []interface{}{}
	systemSettings := map[string]interface{}{
		"log_level":         "info",
		"api_settings":      map[string]interface{}{},
		"database_settings": map[string]interface{}{},
	}

	recordCount := len(devices) + len(templates) + len(discovered)
	tree := map[string]interface{}{
		"format_version": FormatVersion,
		"metadata": map[string]interface{}{
			"export_id":   exportID,
			"created_at":  canonicalTime(created),
			"created_by":  createdBy,
			"export_type": exportType,
			"system_info": map[string]interface{}{
				"version":           version,
				"database_type":     databaseType,
				"hostname":          hostname,
				"total_size_bytes":  int64(0),
				"compression_ratio": float64(0),
			},
			"integrity": map[string]interface{}{
				"checksum":     "",
				"record_count": recordCount,
				"file_count":   1,
			},
		},
		"devices":               deviceTrees,
		"templates":             templateTrees,
		"discovered_devices":    discoveredTrees,
		"network_settings":      network,
		"plugin_configurations": pluginConfigs,
		"system_settings":       systemSettings,
	}
	if excludeSensitive {
		redactSensitive(tree, s.isSensitiveField)
	}
	if err := validateGeneratedTree(tree, 64); err != nil {
		return nil, preparedArchive{}, fmt.Errorf("generated SMA data is invalid: %w", err)
	}
	if err := validateArchiveTree(tree); err != nil {
		return nil, preparedArchive{}, fmt.Errorf("generated SMA structure is invalid: %w", err)
	}
	return tree, preparedArchive{
		exportID:        exportID,
		deviceCount:     len(devices),
		templateCount:   len(templates),
		discoveredCount: len(discovered),
		recordCount:     recordCount,
	}, nil
}

func effectiveBool(config map[string]interface{}, name string, fallback bool) (bool, error) {
	value, exists := config[name]
	if !exists {
		return fallback, nil
	}
	typed, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("%s must be a boolean", name)
	}
	return typed, nil
}

func joinConfigurations(input []sync.DeviceData, standalone []sync.ConfigurationData) ([]sync.DeviceData, error) {
	devices := append([]sync.DeviceData(nil), input...)
	index := make(map[uint]int, len(devices))
	for i := range devices {
		id := devices[i].ID
		if _, exists := index[id]; exists {
			return nil, fmt.Errorf("duplicate device id %d", id)
		}
		index[id] = i
		if nested := devices[i].Configuration; nested != nil && nested.DeviceID != id {
			return nil, fmt.Errorf("device %d has configuration for device %d", id, nested.DeviceID)
		}
	}
	standaloneByDevice := make(map[uint]sync.ConfigurationData, len(standalone))
	for _, configuration := range standalone {
		if _, exists := standaloneByDevice[configuration.DeviceID]; exists {
			return nil, fmt.Errorf("duplicate standalone configuration for device %d", configuration.DeviceID)
		}
		if _, exists := index[configuration.DeviceID]; !exists {
			return nil, fmt.Errorf("orphan standalone configuration for device %d", configuration.DeviceID)
		}
		standaloneByDevice[configuration.DeviceID] = configuration
	}
	for deviceID, configuration := range standaloneByDevice {
		i := index[deviceID]
		if devices[i].Configuration == nil {
			copy := configuration
			devices[i].Configuration = &copy
			continue
		}
		nestedTree, err := buildConfiguration(*devices[i].Configuration)
		if err != nil {
			return nil, fmt.Errorf("invalid nested configuration for device %d: %w", deviceID, err)
		}
		standaloneTree, err := buildConfiguration(configuration)
		if err != nil {
			return nil, fmt.Errorf("invalid standalone configuration for device %d: %w", deviceID, err)
		}
		nestedCanonical, err := canonicalizeTree(nestedTree)
		if err != nil {
			return nil, err
		}
		standaloneCanonical, err := canonicalizeTree(standaloneTree)
		if err != nil {
			return nil, err
		}
		if string(nestedCanonical) != string(standaloneCanonical) {
			return nil, fmt.Errorf("conflicting configurations for device %d", deviceID)
		}
	}
	return devices, nil
}

func buildDevice(device sync.DeviceData) (map[string]interface{}, error) {
	settings, err := materializeOpenMap(device.Settings, 4)
	if err != nil {
		return nil, fmt.Errorf("settings: %w", err)
	}
	result := map[string]interface{}{
		"id":         device.ID,
		"mac":        device.MAC,
		"ip":         device.IP,
		"type":       device.Type,
		"name":       device.Name,
		"model":      device.Model,
		"firmware":   device.Firmware,
		"status":     device.Status,
		"last_seen":  canonicalTime(device.LastSeen),
		"settings":   settings,
		"created_at": canonicalTime(device.CreatedAt),
		"updated_at": canonicalTime(device.UpdatedAt),
	}
	if device.Configuration != nil {
		configuration, err := buildConfiguration(*device.Configuration)
		if err != nil {
			return nil, err
		}
		result["configuration"] = configuration
	}
	return result, nil
}

func buildConfiguration(configuration sync.ConfigurationData) (map[string]interface{}, error) {
	config, err := materializeOpenMap(configuration.Config, 5)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	result := map[string]interface{}{
		"device_id":   configuration.DeviceID,
		"config":      config,
		"sync_status": configuration.SyncStatus,
		"updated_at":  canonicalTime(configuration.UpdatedAt),
	}
	if configuration.TemplateID != nil {
		result["template_id"] = *configuration.TemplateID
	}
	if configuration.LastSynced != nil {
		result["last_synced"] = canonicalTime(*configuration.LastSynced)
	}
	return result, nil
}

func buildTemplate(template sync.TemplateData) (map[string]interface{}, error) {
	config, err := materializeOpenMap(template.Config, 4)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	variables, err := materializeOpenMap(template.Variables, 4)
	if err != nil {
		return nil, fmt.Errorf("variables: %w", err)
	}
	return map[string]interface{}{
		"id":          template.ID,
		"name":        template.Name,
		"description": template.Description,
		"device_type": template.DeviceType,
		"generation":  template.Generation,
		"config":      config,
		"variables":   variables,
		"is_default":  template.IsDefault,
		"created_at":  canonicalTime(template.CreatedAt),
		"updated_at":  canonicalTime(template.UpdatedAt),
	}, nil
}

func buildDiscovered(device sync.DiscoveredDeviceData) (map[string]interface{}, error) {
	return map[string]interface{}{
		"mac":        device.MAC,
		"ssid":       device.SSID,
		"model":      device.Model,
		"generation": device.Generation,
		"ip":         device.IP,
		"signal":     device.Signal,
		"agent_id":   device.AgentID,
		"discovered": canonicalTime(device.Discovered),
	}, nil
}

func canonicalTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func normalizeDatabaseType(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "sqlite", "sqlite3":
		return "sqlite", nil
	case "postgres", "postgresql":
		return "postgresql", nil
	case "mysql":
		return "mysql", nil
	default:
		return "", fmt.Errorf("unknown database provider %q", value)
	}
}

func materializeOpenMap(value map[string]interface{}, depth int) (map[string]interface{}, error) {
	if value == nil {
		return map[string]interface{}{}, nil
	}
	materialized, err := materializeValue(reflect.ValueOf(value), make(map[visit]bool), depth)
	if err != nil {
		return nil, err
	}
	return materialized.(map[string]interface{}), nil
}

type visit struct {
	kind reflect.Kind
	ptr  uintptr
}

func materializeValue(value reflect.Value, active map[visit]bool, depth int) (interface{}, error) {
	if !value.IsValid() {
		return nil, nil
	}
	if value.Kind() == reflect.Interface {
		if value.IsNil() {
			return nil, nil
		}
		return materializeValue(value.Elem(), active, depth)
	}
	if value.CanInterface() {
		if timestamp, ok := value.Interface().(time.Time); ok {
			return canonicalTime(timestamp), nil
		}
		if number, ok := value.Interface().(json.Number); ok {
			parsed, err := number.Float64()
			if err != nil {
				return nil, fmt.Errorf("invalid JSON number %q", number)
			}
			if math.IsInf(parsed, 0) || math.IsNaN(parsed) {
				return nil, fmt.Errorf("number must be finite")
			}
			if math.Trunc(parsed) == parsed && math.Abs(parsed) > float64(maxSafeInteger) {
				return nil, fmt.Errorf("integer-valued number is outside the safe range")
			}
			return parsed, nil
		}
		if marshaler, ok := value.Interface().(json.Marshaler); ok {
			raw, err := marshaler.MarshalJSON()
			if err != nil {
				return nil, fmt.Errorf("custom JSON marshaler: %w", err)
			}
			tree, err := parseStrictJSON(raw, 65-depth)
			if err != nil {
				return nil, fmt.Errorf("custom JSON marshaler returned invalid JSON: %w", err)
			}
			return tree, nil
		}
	}

	switch value.Kind() {
	case reflect.Pointer:
		if value.IsNil() {
			return nil, nil
		}
		key := visit{kind: value.Kind(), ptr: value.Pointer()}
		if active[key] {
			return nil, fmt.Errorf("cycle detected")
		}
		active[key] = true
		result, err := materializeValue(value.Elem(), active, depth)
		delete(active, key)
		return result, err
	case reflect.Map:
		if depth > 64 {
			return nil, fmt.Errorf("maximum JSON depth 64 exceeded")
		}
		if value.IsNil() {
			return map[string]interface{}{}, nil
		}
		if value.Type().Key().Kind() != reflect.String {
			return nil, fmt.Errorf("map keys must be strings")
		}
		key := visit{kind: value.Kind(), ptr: value.Pointer()}
		if active[key] {
			return nil, fmt.Errorf("cycle detected")
		}
		active[key] = true
		result := make(map[string]interface{}, value.Len())
		iter := value.MapRange()
		for iter.Next() {
			name := iter.Key().String()
			if !utf8.ValidString(name) {
				return nil, fmt.Errorf("invalid UTF-8 object name")
			}
			child, err := materializeValue(iter.Value(), active, depth+1)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			result[name] = child
		}
		delete(active, key)
		return result, nil
	case reflect.Slice:
		if depth > 64 {
			return nil, fmt.Errorf("maximum JSON depth 64 exceeded")
		}
		if value.IsNil() {
			return []interface{}{}, nil
		}
		key := visit{kind: value.Kind(), ptr: value.Pointer()}
		if active[key] {
			return nil, fmt.Errorf("cycle detected")
		}
		active[key] = true
		result := make([]interface{}, value.Len())
		for i := range result {
			child, err := materializeValue(value.Index(i), active, depth+1)
			if err != nil {
				return nil, fmt.Errorf("index %d: %w", i, err)
			}
			result[i] = child
		}
		delete(active, key)
		return result, nil
	case reflect.Array:
		if depth > 64 {
			return nil, fmt.Errorf("maximum JSON depth 64 exceeded")
		}
		result := make([]interface{}, value.Len())
		for i := range result {
			child, err := materializeValue(value.Index(i), active, depth+1)
			if err != nil {
				return nil, fmt.Errorf("index %d: %w", i, err)
			}
			result[i] = child
		}
		return result, nil
	case reflect.String:
		if !utf8.ValidString(value.String()) {
			return nil, fmt.Errorf("invalid UTF-8 string")
		}
		return value.String(), nil
	case reflect.Bool:
		return value.Bool(), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		number := value.Int()
		if number < -maxSafeInteger || number > maxSafeInteger {
			return nil, fmt.Errorf("integer is outside the safe range")
		}
		return number, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		number := value.Uint()
		if number > uint64(maxSafeInteger) {
			return nil, fmt.Errorf("integer is outside the safe range")
		}
		return number, nil
	case reflect.Float32, reflect.Float64:
		number := value.Float()
		if math.IsInf(number, 0) || math.IsNaN(number) {
			return nil, fmt.Errorf("number must be finite")
		}
		if math.Trunc(number) == number && math.Abs(number) > float64(maxSafeInteger) {
			return nil, fmt.Errorf("integer-valued number is outside the safe range")
		}
		return number, nil
	case reflect.Invalid:
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported JSON value %s", value.Type())
	}
}

func redactSensitive(value interface{}, sensitive func(string) bool) {
	switch current := value.(type) {
	case map[string]interface{}:
		for key, child := range current {
			if sensitive(key) {
				current[key] = "[REDACTED]"
			} else {
				redactSensitive(child, sensitive)
			}
		}
	case []interface{}:
		for _, child := range current {
			redactSensitive(child, sensitive)
		}
	}
}

func canonicalizeTree(tree interface{}) ([]byte, error) {
	raw, err := json.Marshal(tree)
	if err != nil {
		return nil, err
	}
	return jsoncanonicalizer.Transform(raw)
}

func finalizeArchiveTree(tree map[string]interface{}) ([]byte, string, error) {
	metadata := tree["metadata"].(map[string]interface{})
	integrity := metadata["integrity"].(map[string]interface{})
	integrity["checksum"] = ""
	canonicalForHash, err := canonicalizeTree(tree)
	if err != nil {
		return nil, "", err
	}
	checksum := checksumBytes(canonicalForHash)
	integrity["checksum"] = checksum
	final, err := canonicalizeTree(tree)
	return final, checksum, err
}

type atomicFile interface {
	io.Writer
	Sync() error
	Close() error
	Stat() (os.FileInfo, error)
}

type fileOperations interface {
	CreateTemp(root *os.Root, dir, pattern string) (atomicFile, string, error)
	Rename(root *os.Root, oldPath, newPath string) error
	Remove(root *os.Root, path string) error
}

type osFileOperations struct{}

func (osFileOperations) CreateTemp(root *os.Root, dir, pattern string) (atomicFile, string, error) {
	for range 100 {
		var random [8]byte
		if _, err := rand.Read(random[:]); err != nil {
			return nil, "", err
		}
		name := pattern
		token := fmt.Sprintf("%x", random[:])
		if wildcard := strings.LastIndexByte(name, '*'); wildcard >= 0 {
			name = name[:wildcard] + token + name[wildcard+1:]
		} else {
			name += token
		}
		tempPath := filepath.Join(dir, name)
		file, err := root.OpenFile(tempPath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return nil, "", err
		}
		return file, tempPath, nil
	}
	return nil, "", fmt.Errorf("create unique temporary archive: too many collisions")
}
func (osFileOperations) Rename(root *os.Root, oldPath, newPath string) error {
	return root.Rename(oldPath, newPath)
}
func (osFileOperations) Remove(root *os.Root, path string) error { return root.Remove(path) }

func (s *SMAPlugin) publishAtomic(root *os.Root, finalPath string, data []byte, level int) (size int64, err error) {
	temp, tempPath, err := s.files.CreateTemp(root, filepath.Dir(finalPath), ".sma-*.tmp")
	if err != nil {
		return 0, fmt.Errorf("create temporary archive: %w", err)
	}
	renamed := false
	closed := false
	defer func() {
		if !closed {
			_ = temp.Close()
		}
		if !renamed {
			_ = s.files.Remove(root, tempPath)
		}
	}()

	writer, err := gzip.NewWriterLevel(temp, level)
	if err != nil {
		return 0, fmt.Errorf("create gzip writer: %w", err)
	}
	writer.ModTime = time.Unix(0, 0)
	writer.OS = 255
	if _, err = writer.Write(data); err != nil {
		return 0, fmt.Errorf("write gzip archive: %w", err)
	}
	if err = writer.Close(); err != nil {
		return 0, fmt.Errorf("close gzip archive: %w", err)
	}
	if err = temp.Sync(); err != nil {
		return 0, fmt.Errorf("sync temporary archive: %w", err)
	}
	info, err := temp.Stat()
	if err != nil {
		return 0, fmt.Errorf("stat temporary archive: %w", err)
	}
	if err = temp.Close(); err != nil {
		closed = true
		return 0, fmt.Errorf("close temporary archive: %w", err)
	}
	closed = true
	if err = s.files.Rename(root, tempPath, finalPath); err != nil {
		return 0, fmt.Errorf("publish archive: %w", err)
	}
	renamed = true
	return info.Size(), nil
}
