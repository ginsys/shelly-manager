package sma

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/ginsys/shelly-manager/internal/sync"
)

type normalizationError struct {
	kind string
	err  error
}

func (e *normalizationError) Error() string { return e.kind }
func (e *normalizationError) Unwrap() error { return e.err }

// normalizeSMAInput bounds the normalized JSON representation. It recognizes
// gzip only by its magic bytes; all other input is treated as raw JSON.
func normalizeSMAInput(input io.Reader, limit int64) ([]byte, error) {
	if limit <= 0 {
		return nil, &normalizationError{kind: "invalid normalized-data limit"}
	}
	buffered := bufio.NewReader(input)
	header, err := buffered.Peek(2)
	if err != nil && err != io.EOF && err != bufio.ErrBufferFull {
		return nil, &normalizationError{kind: "failed to read input", err: err}
	}

	var normalized io.Reader = buffered
	var gzipReader *gzip.Reader
	if len(header) == 2 && header[0] == 0x1f && header[1] == 0x8b {
		gzipReader, err = gzip.NewReader(buffered)
		if err != nil {
			return nil, &normalizationError{kind: "malformed gzip input", err: err}
		}
		defer func() { _ = gzipReader.Close() }()
		normalized = gzipReader
	}

	data, readErr := io.ReadAll(io.LimitReader(normalized, limit+1))
	oversized := int64(len(data)) > limit
	if oversized && gzipReader != nil {
		// Do not retain more output, but consume the stream so truncated bodies
		// and bad gzip trailers/checksums are still detected.
		if _, err := io.Copy(io.Discard, gzipReader); err != nil {
			return nil, &normalizationError{kind: "malformed gzip input", err: err}
		}
	}
	if readErr != nil {
		kind := "failed to read input"
		if gzipReader != nil {
			kind = "malformed gzip input"
		}
		return nil, &normalizationError{kind: kind, err: readErr}
	}
	if oversized {
		return nil, &normalizationError{kind: "normalized input exceeds the configured limit"}
	}
	return data, nil
}

func (s *SMAPlugin) ImportFromFile(ctx context.Context, filePath string, config sync.ImportConfig) (*sync.ImportResult, error) {
	source, err := openApprovedRoot(s.importBaseDir, filePath, "import")
	if err != nil {
		return nil, err
	}
	defer func() { _ = source.close() }()
	file, err := source.root.Open(source.relative)
	if err != nil {
		return nil, fmt.Errorf("open SMA input: %w", err)
	}
	defer func() { _ = file.Close() }()
	normalized, err := normalizeSMAInput(file, s.importLimit())
	if err != nil {
		return invalidImportResult("normalize SMA input", err)
	}
	return s.importNormalized(ctx, normalized, config)
}

func (s *SMAPlugin) ImportFromData(ctx context.Context, data []byte, config sync.ImportConfig) (*sync.ImportResult, error) {
	normalized, err := normalizeSMAInput(bytesReader(data), s.importLimit())
	if err != nil {
		return invalidImportResult("normalize SMA input", err)
	}
	return s.importNormalized(ctx, normalized, config)
}

func (s *SMAPlugin) importLimit() int64 {
	if s.normalizedLimit > 0 {
		return s.normalizedLimit
	}
	return defaultNormalizedLimit
}

func (s *SMAPlugin) importNormalized(_ context.Context, data []byte, config sync.ImportConfig) (*sync.ImportResult, error) {
	start := time.Now()
	treeValue, parseErr := parseStrictJSON(data, 64)
	if parseErr != nil {
		return invalidImportResult("invalid JSON", parseErr)
	}
	root, ok := treeValue.(map[string]interface{})
	if !ok {
		return invalidImportResult("invalid SMA structure", fmt.Errorf("root must be an object"))
	}
	version, ok := root["format_version"].(string)
	if !ok || version != FormatVersion {
		return invalidImportResult("unsupported SMA format", fmt.Errorf("format_version must be %q", FormatVersion))
	}
	if validationErr := validateArchiveTree(root); validationErr != nil {
		return invalidImportResult("invalid SMA structure", validationErr)
	}

	metadata := root["metadata"].(map[string]interface{})
	integrity := metadata["integrity"].(map[string]interface{})
	suppliedChecksum, _ := integrity["checksum"].(string)
	if !checksumPattern.MatchString(suppliedChecksum) {
		return invalidImportResult("integrity failure", fmt.Errorf("checksum is missing or malformed"))
	}
	devices := root["devices"].([]interface{})
	templates := root["templates"].([]interface{})
	discovered := root["discovered_devices"].([]interface{})
	expectedRecords, err := requireSafeInteger(integrity["record_count"], "record_count", false)
	if err != nil || expectedRecords != int64(len(devices)+len(templates)+len(discovered)) {
		return invalidImportResult("integrity failure", fmt.Errorf("record_count does not match archive contents"))
	}
	fileCount, err := requireSafeInteger(integrity["file_count"], "file_count", false)
	if err != nil || fileCount != 1 {
		return invalidImportResult("integrity failure", fmt.Errorf("file_count must be 1"))
	}

	integrity["checksum"] = ""
	canonical, err := canonicalizeTree(root)
	integrity["checksum"] = suppliedChecksum
	if err != nil {
		return invalidImportResult("integrity failure", err)
	}
	if checksumBytes(canonical) != suppliedChecksum {
		return invalidImportResult("integrity failure", fmt.Errorf("checksum mismatch"))
	}

	restored, err := json.Marshal(root)
	if err != nil {
		return invalidImportResult("invalid SMA structure", err)
	}
	var archive SMAArchive
	if err := json.Unmarshal(restored, &archive); err != nil {
		return invalidImportResult("invalid SMA structure", err)
	}
	importID := fmt.Sprintf("sma-import-%d", time.Now().UnixNano())
	if config.Options.DryRun {
		return s.generateDryRunResult(importID, &archive, time.Since(start))
	}

	notImplemented := fmt.Errorf(
		"SMA import persistence is not yet implemented; re-run with dry_run to preview (#284): %w",
		sync.ErrImportNotImplemented,
	)
	return &sync.ImportResult{
		Success:    false,
		ImportID:   importID,
		PluginName: "sma",
		Format:     "sma",
		Duration:   time.Since(start),
		Errors:     []string{notImplemented.Error()},
		CreatedAt:  time.Now(),
	}, notImplemented
}

func invalidImportResult(kind string, cause error) (*sync.ImportResult, error) {
	err := fmt.Errorf("%w: %s: %v", sync.ErrInvalidImportData, kind, cause)
	return &sync.ImportResult{
		Success:   false,
		ImportID:  fmt.Sprintf("sma-import-%d", time.Now().UnixNano()),
		Errors:    []string{err.Error()},
		CreatedAt: time.Now(),
	}, err
}

func (s *SMAPlugin) generateDryRunResult(importID string, archive *SMAArchive, duration time.Duration) (*sync.ImportResult, error) {
	changeCapacity, err := checkedIntSum(
		len(archive.Devices),
		len(archive.Templates),
		len(archive.Discovered),
	)
	if err != nil {
		return nil, fmt.Errorf("prepare dry-run changes: %w", err)
	}
	changes := make([]sync.ImportChange, 0, changeCapacity)
	for _, device := range archive.Devices {
		changes = append(changes, sync.ImportChange{
			Type: "create", Resource: "device", ResourceID: device.MAC,
			NewValue: fmt.Sprintf("Device: %s (%s)", device.Name, device.Type),
		})
	}
	for _, template := range archive.Templates {
		changes = append(changes, sync.ImportChange{
			Type: "create", Resource: "template", ResourceID: template.Name,
			NewValue: fmt.Sprintf("Template: %s for %s", template.Name, template.DeviceType),
		})
	}
	for _, discovered := range archive.Discovered {
		changes = append(changes, sync.ImportChange{
			Type: "create", Resource: "discovered_device", ResourceID: discovered.MAC,
			NewValue: fmt.Sprintf("Discovered: %s (%s)", discovered.Model, discovered.MAC),
		})
	}
	return &sync.ImportResult{
		Success:         true,
		ImportID:        importID,
		PluginName:      "sma",
		Format:          "sma",
		RecordsImported: len(changes),
		Changes:         changes,
		Warnings:        []string{"This is a dry run - no actual changes were made"},
		Metadata: map[string]interface{}{
			"format_version": archive.FormatVersion,
			"source_system":  archive.Metadata.SystemInfo.Hostname,
			"dry_run":        true,
		},
		Duration:  duration,
		CreatedAt: time.Now(),
	}, nil
}

func checkedIntSum(values ...int) (int, error) {
	total := 0
	for _, value := range values {
		if value < 0 || total > math.MaxInt-value {
			return 0, fmt.Errorf("integer addition overflow")
		}
		total += value
	}
	return total, nil
}

// bytesReader avoids importing bytes solely at call sites and keeps the
// normalizer's reader contract explicit.
type sliceReader struct {
	data []byte
	at   int
}

func bytesReader(data []byte) *sliceReader { return &sliceReader{data: data} }
func (r *sliceReader) Read(target []byte) (int, error) {
	if r.at >= len(r.data) {
		return 0, io.EOF
	}
	count := copy(target, r.data[r.at:])
	r.at += count
	return count, nil
}
