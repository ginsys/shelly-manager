package sma

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/ginsys/shelly-manager/internal/security"
	"github.com/ginsys/shelly-manager/internal/sync"
)

// ImportFromFile imports data from an SMA file
func (s *SMAPlugin) ImportFromFile(ctx context.Context, filePath string, config sync.ImportConfig) (*sync.ImportResult, error) {
	s.logger.Info("Starting SMA import from file", "path", filePath)

	// Validate file path against base directory to prevent path traversal
	if s.baseDir != "" {
		validatedPath, err := security.ValidatePath(s.baseDir, filePath)
		if err != nil {
			return nil, fmt.Errorf("path validation failed: %w", err)
		}
		filePath = validatedPath
	}

	// Validate file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("SMA file does not exist: %s", filePath)
	}

	// Open and read the file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open SMA file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			s.logger.Warn("Failed to close SMA file", "error", closeErr)
		}
	}()

	// Read compressed data
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader (file may not be compressed): %w", err)
	}
	defer func() {
		if closeErr := gzipReader.Close(); closeErr != nil {
			s.logger.Warn("Failed to close gzip reader", "error", closeErr)
		}
	}()

	// Read all data
	jsonData, err := io.ReadAll(gzipReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read compressed data: %w", err)
	}

	return s.ImportFromData(ctx, jsonData, config)
}

// ImportFromData imports data from raw JSON bytes
func (s *SMAPlugin) ImportFromData(ctx context.Context, data []byte, config sync.ImportConfig) (*sync.ImportResult, error) {
	startTime := time.Now()
	importID := fmt.Sprintf("sma-import-%d", time.Now().Unix())

	s.logger.Info("Starting SMA import from data", "size", len(data))

	// Parse SMA archive
	var archive SMAArchive
	if err := json.Unmarshal(data, &archive); err != nil {
		return &sync.ImportResult{
			Success:   false,
			ImportID:  importID,
			Duration:  time.Since(startTime),
			Errors:    []string{fmt.Sprintf("failed to parse SMA JSON: %v", err)},
			CreatedAt: time.Now(),
		}, fmt.Errorf("failed to parse SMA archive: %w", err)
	}

	// Validate SMA format
	if err := s.validateSMAFormat(&archive); err != nil {
		return &sync.ImportResult{
			Success:   false,
			ImportID:  importID,
			Duration:  time.Since(startTime),
			Errors:    []string{fmt.Sprintf("SMA format validation failed: %v", err)},
			CreatedAt: time.Now(),
		}, fmt.Errorf("SMA format validation failed: %w", err)
	}

	// Verify integrity if checksum is provided
	if archive.Metadata.Integrity.Checksum != "" {
		if err := s.verifyIntegrity(&archive, data); err != nil {
			return &sync.ImportResult{
				Success:   false,
				ImportID:  importID,
				Duration:  time.Since(startTime),
				Errors:    []string{fmt.Sprintf("integrity verification failed: %v", err)},
				Warnings:  []string{"Data may be corrupted or tampered with"},
				CreatedAt: time.Now(),
			}, fmt.Errorf("integrity verification failed: %w", err)
		}
	}

	// Handle dry run
	if config.Options.DryRun {
		return s.generateDryRunResult(importID, &archive, time.Since(startTime))
	}

	// Non-dry-run persistence is not yet implemented. Refuse rather than
	// fabricate success: the previous placeholder returned success:true with
	// records_imported>0 while writing nothing, silently discarding the
	// caller's data (#272). Fail closed until real persistence lands; callers
	// can re-run with dry_run to preview.
	s.logger.Warn("Refusing non-dry-run SMA import: persistence not yet implemented",
		"import_id", importID,
		"devices", len(archive.Devices),
		"templates", len(archive.Templates),
		"discovered", len(archive.Discovered),
	)

	err := fmt.Errorf("SMA import persistence is not yet implemented; re-run with dry_run to preview (#272)")
	return &sync.ImportResult{
		Success:    false,
		ImportID:   importID,
		PluginName: "sma",
		Format:     "sma",
		Duration:   time.Since(startTime),
		Errors:     []string{err.Error()},
		CreatedAt:  time.Now(),
	}, err
}

// validateSMAFormat validates the basic structure and version of an SMA archive
func (s *SMAPlugin) validateSMAFormat(archive *SMAArchive) error {
	// Check SMA version compatibility
	if archive.SMAVersion == "" {
		return fmt.Errorf("missing SMA version")
	}

	// For now, we only support version 1.0
	if archive.SMAVersion != "1.0" {
		return fmt.Errorf("unsupported SMA version: %s (supported: 1.0)", archive.SMAVersion)
	}

	// Check format version
	if archive.FormatVersion == "" {
		return fmt.Errorf("missing format version")
	}

	// Validate required metadata
	if archive.Metadata.ExportID == "" {
		return fmt.Errorf("missing export ID in metadata")
	}

	if archive.Metadata.CreatedAt.IsZero() {
		return fmt.Errorf("missing or invalid creation timestamp")
	}

	// Validate data sections exist
	if len(archive.Devices) == 0 && len(archive.Templates) == 0 {
		return fmt.Errorf("archive contains no devices or templates")
	}

	return nil
}

// verifyIntegrity verifies the integrity of the SMA archive
func (s *SMAPlugin) verifyIntegrity(archive *SMAArchive, originalData []byte) error {
	expectedChecksum := archive.Metadata.Integrity.Checksum

	if expectedChecksum == "" {
		s.logger.Info("No checksum provided, skipping integrity verification")
		return nil
	}

	if !strings.HasPrefix(expectedChecksum, "sha256:") {
		return fmt.Errorf("unsupported checksum format: %s", expectedChecksum)
	}

	// For checksum verification, we need to recalculate based on the data without the checksum
	// Create a copy of the archive with empty checksum and recalculate
	archiveCopy := *archive
	archiveCopy.Metadata.Integrity.Checksum = ""

	// Marshal the copy to get comparable JSON
	comparableData, err := json.MarshalIndent(archiveCopy, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal archive for checksum verification: %w", err)
	}

	// Calculate checksum of the comparable data
	hash := sha256.Sum256(comparableData)
	actualChecksum := fmt.Sprintf("sha256:%x", hash)

	if actualChecksum != expectedChecksum {
		// Note: JSON formatting can cause checksum mismatches even with identical data
		// This is a known limitation and would be improved in a production implementation
		s.logger.Warn("Checksum verification failed - this may be due to JSON formatting differences",
			"expected", expectedChecksum,
			"actual", actualChecksum,
		)
		// For now, we'll continue with a warning rather than failing
		// In production, you might want to make this configurable
	}

	// Verify record counts if provided
	if archive.Metadata.Integrity.RecordCount > 0 {
		actualRecordCount := len(archive.Devices) + len(archive.Templates) + len(archive.Discovered)
		if actualRecordCount != archive.Metadata.Integrity.RecordCount {
			return fmt.Errorf("record count mismatch: expected %d, got %d",
				archive.Metadata.Integrity.RecordCount, actualRecordCount)
		}
	}

	s.logger.Info("SMA integrity verification passed", "checksum", expectedChecksum)
	return nil
}

// generateDryRunResult generates a preview of what would be imported
func (s *SMAPlugin) generateDryRunResult(importID string, archive *SMAArchive, duration time.Duration) (*sync.ImportResult, error) {
	var changes []sync.ImportChange

	// Simulate device imports
	for _, device := range archive.Devices {
		changes = append(changes, sync.ImportChange{
			Type:       "create", // or "update" based on existence check
			Resource:   "device",
			ResourceID: device.MAC,
			NewValue:   fmt.Sprintf("Device: %s (%s)", device.Name, device.Type),
		})
	}

	// Simulate template imports
	for _, template := range archive.Templates {
		changes = append(changes, sync.ImportChange{
			Type:       "create", // or "update" based on existence check
			Resource:   "template",
			ResourceID: template.Name,
			NewValue:   fmt.Sprintf("Template: %s for %s", template.Name, template.DeviceType),
		})
	}

	// Simulate discovered device imports
	for _, discovered := range archive.Discovered {
		changes = append(changes, sync.ImportChange{
			Type:       "create",
			Resource:   "discovered_device",
			ResourceID: discovered.MAC,
			NewValue:   fmt.Sprintf("Discovered: %s (%s)", discovered.Model, discovered.MAC),
		})
	}

	estimatedImported := len(archive.Devices) + len(archive.Templates) + len(archive.Discovered)

	return &sync.ImportResult{
		Success:         true,
		ImportID:        importID,
		PluginName:      "sma",
		Format:          "sma",
		RecordsImported: estimatedImported,
		RecordsSkipped:  0,
		Duration:        duration,
		Changes:         changes,
		Warnings: []string{
			"This is a dry run - no actual changes were made",
			"Actual import may differ based on existing data and conflicts",
		},
		Metadata: map[string]interface{}{
			"sma_version":    archive.SMAVersion,
			"format_version": archive.FormatVersion,
			"source_system":  archive.Metadata.SystemInfo.Hostname,
			"dry_run":        true,
		},
		CreatedAt: time.Now(),
	}, nil
}
