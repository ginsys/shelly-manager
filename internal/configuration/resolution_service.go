package configuration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
	"gorm.io/gorm"
)

// ResolutionService handles drift resolution workflows
type ResolutionService struct {
	db      *gorm.DB
	service *Service // Configuration service
	logger  *logging.Logger

	// Runtime state
	policies       []ResolutionPolicy
	lastPolicyLoad time.Time
}

// NewResolutionService creates a new resolution service
func NewResolutionService(db *gorm.DB, configService *Service, logger *logging.Logger) *ResolutionService {
	return &ResolutionService{
		db:      db,
		service: configService,
		logger:  logger,
	}
}

// CreatePolicy creates a new resolution policy
func (rs *ResolutionService) CreatePolicy(policy *ResolutionPolicy) error {
	// Serialize JSON fields
	if categoriesJSON, err := json.Marshal(policy.Categories); err == nil {
		policy.CategoriesJSON = categoriesJSON
	}
	if severitiesJSON, err := json.Marshal(policy.Severities); err == nil {
		policy.SeveritiesJSON = severitiesJSON
	}
	if autoFixJSON, err := json.Marshal(policy.AutoFixCategories); err == nil {
		policy.AutoFixCategoriesJSON = autoFixJSON
	}
	if excludedJSON, err := json.Marshal(policy.ExcludedPaths); err == nil {
		policy.ExcludedPathsJSON = excludedJSON
	}

	if err := rs.db.Create(policy).Error; err != nil {
		return fmt.Errorf("failed to create resolution policy: %w", err)
	}

	rs.logger.WithFields(map[string]any{
		"policy_id":   policy.ID,
		"policy_name": policy.Name,
		"component":   "resolution",
	}).Info("Created resolution policy")

	// Reload policies
	if err := rs.loadPolicies(); err != nil {
		rs.logger.WithFields(map[string]any{
			"component": "resolution",
			"error":     err,
		}).Error("Failed to reload policies after creation")
		// Continue execution as the policy was still saved successfully
	}

	return nil
}

// GetPolicies retrieves all resolution policies
func (rs *ResolutionService) GetPolicies() ([]ResolutionPolicy, error) {
	var policies []ResolutionPolicy
	if err := rs.db.Find(&policies).Error; err != nil {
		return nil, fmt.Errorf("failed to get policies: %w", err)
	}

	// Deserialize JSON fields
	for i := range policies {
		if len(policies[i].CategoriesJSON) > 0 {
			if err := json.Unmarshal(policies[i].CategoriesJSON, &policies[i].Categories); err != nil {
				rs.logger.WithFields(map[string]any{
					"component": "resolution",
					"policy_id": policies[i].ID,
					"error":     err,
				}).Error("Failed to unmarshal policy categories")
			}
		}
		if len(policies[i].SeveritiesJSON) > 0 {
			if err := json.Unmarshal(policies[i].SeveritiesJSON, &policies[i].Severities); err != nil {
				rs.logger.WithFields(map[string]any{
					"component": "resolution",
					"policy_id": policies[i].ID,
					"error":     err,
				}).Error("Failed to unmarshal policy severities")
			}
		}
		if len(policies[i].AutoFixCategoriesJSON) > 0 {
			if err := json.Unmarshal(policies[i].AutoFixCategoriesJSON, &policies[i].AutoFixCategories); err != nil {
				rs.logger.WithFields(map[string]any{
					"component": "resolution",
					"policy_id": policies[i].ID,
					"error":     err,
				}).Error("Failed to unmarshal policy auto-fix categories")
			}
		}
		if len(policies[i].ExcludedPathsJSON) > 0 {
			if err := json.Unmarshal(policies[i].ExcludedPathsJSON, &policies[i].ExcludedPaths); err != nil {
				rs.logger.WithFields(map[string]any{
					"component": "resolution",
					"policy_id": policies[i].ID,
					"error":     err,
				}).Error("Failed to unmarshal policy excluded paths")
			}
		}
	}

	return policies, nil
}

// ProcessDriftForResolution analyzes drift and creates resolution requests
func (rs *ResolutionService) ProcessDriftForResolution(ctx context.Context, driftResults []DriftResult) error {
	rs.logger.WithFields(map[string]any{
		"devices":   len(driftResults),
		"component": "resolution",
	}).Info("Processing drift for resolution")

	// Ensure policies are loaded
	if err := rs.loadPolicies(); err != nil {
		return fmt.Errorf("failed to load policies: %w", err)
	}

	for _, result := range driftResults {
		if result.Status != "drift" || result.Drift == nil {
			continue
		}

		for _, diff := range result.Drift.Differences {
			if err := rs.processDifference(ctx, &result, &diff); err != nil {
				rs.logger.WithFields(map[string]any{
					"device_id": result.DeviceID,
					"path":      diff.Path,
					"error":     err.Error(),
					"component": "resolution",
				}).Error("Failed to process difference for resolution")
			}
		}
	}

	return nil
}

// ExecuteAutoFix attempts to automatically fix configuration drift
func (rs *ResolutionService) ExecuteAutoFix(ctx context.Context, deviceID uint, path string) (*AutoFixResult, error) {
	result := &AutoFixResult{
		DeviceID: deviceID,
		Path:     path,
	}

	start := time.Now()
	defer func() {
		result.Duration = time.Since(start)
	}()

	rs.logger.WithFields(map[string]any{
		"device_id": deviceID,
		"path":      path,
		"component": "resolution",
	}).Info("Executing auto-fix")

	// Get device information
	var device Device
	if err := rs.db.Table("devices").First(&device, deviceID).Error; err != nil {
		result.Error = fmt.Sprintf("Device not found: %v", err)
		return result, err
	}

	// Get current device configuration
	var deviceConfig DeviceConfig
	if err := rs.db.Where("device_id = ?", deviceID).First(&deviceConfig).Error; err != nil {
		result.Error = fmt.Sprintf("Device configuration not found: %v", err)
		return result, err
	}

	// Check if auto-fix is allowed for this path
	policy := rs.findApplicablePolicy(device, path, "info") // Assume info severity for now
	if policy == nil || !policy.AutoFixEnabled {
		result.Action = "skipped"
		result.Error = "Auto-fix not enabled for this path"
		return result, nil
	}

	// Check if path is excluded
	if rs.isPathExcluded(path, policy.ExcludedPaths) {
		result.Action = "skipped"
		result.Error = "Path is excluded from auto-fix"
		return result, nil
	}

	// For safe mode, only allow metadata fixes
	if policy.SafeMode && !rs.isMetadataPath(path) {
		result.Action = "skipped"
		result.Error = "Safe mode: only metadata paths allowed"
		return result, nil
	}

	// Create client for device
	client, err := rs.service.createClientForDevice(deviceID)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to create client: %v", err)
		return result, err
	}

	// Get current device state
	currentConfig, err := client.GetConfig(ctx)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to get current config: %v", err)
		return result, err
	}

	// Parse stored and current configurations
	var storedData, currentData map[string]interface{}
	if err := json.Unmarshal(deviceConfig.Config, &storedData); err != nil {
		result.Error = fmt.Sprintf("Failed to parse stored config: %v", err)
		return result, err
	}

	if err := json.Unmarshal(currentConfig.Raw, &currentData); err != nil {
		result.Error = fmt.Sprintf("Failed to parse current config: %v", err)
		return result, err
	}

	// Get values at path
	storedValue := rs.getValueAtPath(storedData, path)
	currentValue := rs.getValueAtPath(currentData, path)

	result.OldValue = currentValue
	result.NewValue = storedValue

	// Determine resolution strategy
	strategy := rs.determineAutoFixStrategy(path, storedValue, currentValue, policy)

	switch strategy {
	case StrategyRestore:
		// Export stored configuration to device
		if err := rs.service.ExportToDevice(deviceID, client); err != nil {
			result.Error = fmt.Sprintf("Failed to export configuration: %v", err)
			return result, err
		}
		result.Action = "restored"
		result.Success = true

	case StrategyUpdate:
		// Update stored configuration to match device
		rs.setValueAtPath(storedData, path, currentValue)
		updatedConfig, err := json.Marshal(storedData)
		if err != nil {
			result.Error = fmt.Sprintf("Failed to marshal updated config: %v", err)
			return result, err
		}

		if err := rs.db.Model(&deviceConfig).Update("config", updatedConfig).Error; err != nil {
			result.Error = fmt.Sprintf("Failed to update stored config: %v", err)
			return result, err
		}
		result.Action = "updated"
		result.Success = true

	case StrategyIgnore:
		result.Action = "ignored"
		result.Success = true

	default:
		result.Error = "No applicable auto-fix strategy"
		return result, nil
	}

	// Record resolution history
	if result.Success {
		history := &ResolutionHistory{
			DeviceID:      deviceID,
			DeviceName:    device.Name,
			PolicyID:      &policy.ID,
			Type:          "auto_fix",
			Category:      rs.categorizePathForResolution(path),
			Path:          path,
			ChangeType:    result.Action,
			Method:        "config_export",
			Success:       true,
			Duration:      int(result.Duration.Milliseconds()),
			TriggeredBy:   "system",
			Justification: fmt.Sprintf("Auto-fix via policy: %s", policy.Name),
			ExecutedAt:    time.Now(),
		}

		if oldJSON, err := json.Marshal(result.OldValue); err == nil {
			history.OldValue = oldJSON
		}
		if newJSON, err := json.Marshal(result.NewValue); err == nil {
			history.NewValue = newJSON
		}

		rs.db.Create(history)
	}

	rs.logger.WithFields(map[string]any{
		"device_id": deviceID,
		"path":      path,
		"action":    result.Action,
		"success":   result.Success,
		"duration":  result.Duration,
		"component": "resolution",
	}).Info("Auto-fix completed")

	return result, nil
}

// CreateResolutionRequest creates a manual resolution request
func (rs *ResolutionService) CreateResolutionRequest(req *ResolutionRequest) error {
	// Set default values
	if req.Status == "" {
		req.Status = string(StatusPending)
	}
	if req.Priority == "" {
		req.Priority = string(PriorityMedium)
	}

	if err := rs.db.Create(req).Error; err != nil {
		return fmt.Errorf("failed to create resolution request: %w", err)
	}

	rs.logger.WithFields(map[string]any{
		"request_id": req.ID,
		"device_id":  req.DeviceID,
		"path":       req.Path,
		"priority":   req.Priority,
		"component":  "resolution",
	}).Info("Created resolution request")

	return nil
}

// GetPendingRequests retrieves pending resolution requests
func (rs *ResolutionService) GetPendingRequests(limit int) ([]ResolutionRequest, error) {
	var requests []ResolutionRequest
	query := rs.db.Where("status = ?", StatusPending).Order("priority DESC, created_at ASC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&requests).Error; err != nil {
		return nil, fmt.Errorf("failed to get pending requests: %w", err)
	}

	return requests, nil
}

// ApproveRequest approves a resolution request
func (rs *ResolutionService) ApproveRequest(ctx context.Context, requestID uint, reviewedBy, notes string) error {
	var request ResolutionRequest
	if err := rs.db.First(&request, requestID).Error; err != nil {
		return fmt.Errorf("request not found: %w", err)
	}

	now := time.Now()
	updates := map[string]interface{}{
		"status":       StatusApproved,
		"reviewed_by":  reviewedBy,
		"reviewed_at":  &now,
		"review_notes": notes,
	}

	if err := rs.db.Model(&request).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to approve request: %w", err)
	}

	rs.logger.WithFields(map[string]any{
		"request_id":  requestID,
		"reviewed_by": reviewedBy,
		"component":   "resolution",
	}).Info("Approved resolution request")

	// Execute the resolution if not scheduled
	if request.ScheduledAt == nil {
		go rs.executeResolutionRequest(context.Background(), requestID)
	}

	return nil
}

// RejectRequest rejects a resolution request
func (rs *ResolutionService) RejectRequest(requestID uint, reviewedBy, notes string) error {
	now := time.Now()
	updates := map[string]interface{}{
		"status":       StatusRejected,
		"reviewed_by":  reviewedBy,
		"reviewed_at":  &now,
		"review_notes": notes,
	}

	result := rs.db.Model(&ResolutionRequest{}).Where("id = ?", requestID).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to reject request: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("request not found")
	}

	rs.logger.WithFields(map[string]any{
		"request_id":  requestID,
		"reviewed_by": reviewedBy,
		"component":   "resolution",
	}).Info("Rejected resolution request")

	return nil
}

// Helper methods

func (rs *ResolutionService) loadPolicies() error {
	// Cache policies for 5 minutes
	if time.Since(rs.lastPolicyLoad) < 5*time.Minute && len(rs.policies) > 0 {
		return nil
	}

	policies, err := rs.GetPolicies()
	if err != nil {
		return err
	}

	rs.policies = policies
	rs.lastPolicyLoad = time.Now()
	return nil
}

func (rs *ResolutionService) processDifference(ctx context.Context, result *DriftResult, diff *ConfigDifference) error {
	// Get device info
	var device Device
	if err := rs.db.Table("devices").First(&device, result.DeviceID).Error; err != nil {
		return err
	}

	// Find applicable policy
	policy := rs.findApplicablePolicy(device, diff.Path, diff.Severity)

	if policy != nil && policy.AutoFixEnabled && rs.canAutoFix(diff, policy) {
		// Attempt auto-fix
		autoFixResult, err := rs.ExecuteAutoFix(ctx, result.DeviceID, diff.Path)
		if err != nil || !autoFixResult.Success {
			// Auto-fix failed, create manual request
			return rs.createManualRequest(result, diff, "auto_fix_failed")
		}
		return nil
	}

	// Create manual resolution request
	return rs.createManualRequest(result, diff, "manual_review")
}

func (rs *ResolutionService) findApplicablePolicy(device Device, path, severity string) *ResolutionPolicy {
	for _, policy := range rs.policies {
		if !policy.Enabled {
			continue
		}

		// Check categories
		if len(policy.Categories) > 0 {
			category := rs.categorizePathForResolution(path)
			found := false
			for _, cat := range policy.Categories {
				if cat == category {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Check severities
		if len(policy.Severities) > 0 {
			found := false
			for _, sev := range policy.Severities {
				if sev == severity {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Check device filter (simplified)
		// This would be expanded to check device types, generations, etc.

		return &policy
	}

	return nil
}

func (rs *ResolutionService) canAutoFix(diff *ConfigDifference, policy *ResolutionPolicy) bool {
	// Check if category is allowed for auto-fix
	category := rs.categorizePathForResolution(diff.Path)
	for _, allowedCat := range policy.AutoFixCategories {
		if category == allowedCat {
			return true
		}
	}

	// Check if path is excluded
	if rs.isPathExcluded(diff.Path, policy.ExcludedPaths) {
		return false
	}

	// In safe mode, only allow metadata
	if policy.SafeMode && !rs.isMetadataPath(diff.Path) {
		return false
	}

	return false
}

func (rs *ResolutionService) createManualRequest(result *DriftResult, diff *ConfigDifference, requestType string) error {
	request := &ResolutionRequest{
		DeviceID:    result.DeviceID,
		DeviceName:  result.DeviceName,
		RequestType: requestType,
		Priority:    rs.determinePriority(diff.Severity),
		Category:    diff.Category,
		Severity:    diff.Severity,
		Path:        diff.Path,
		Description: diff.Description,
		Impact:      diff.Impact,
		Strategy:    string(StrategyRestore), // Default strategy
		Status:      string(StatusPending),
	}

	// Set values
	if currentJSON, err := json.Marshal(diff.Actual); err == nil {
		request.CurrentValue = currentJSON
	}
	if expectedJSON, err := json.Marshal(diff.Expected); err == nil {
		request.ExpectedValue = expectedJSON
		request.ProposedValue = expectedJSON // Default to expected
	}

	return rs.CreateResolutionRequest(request)
}

func (rs *ResolutionService) executeResolutionRequest(ctx context.Context, requestID uint) {
	var request ResolutionRequest
	if err := rs.db.First(&request, requestID).Error; err != nil {
		rs.logger.WithFields(map[string]any{
			"request_id": requestID,
			"error":      err.Error(),
			"component":  "resolution",
		}).Error("Failed to get resolution request")
		return
	}

	// Implementation would execute the actual resolution
	// This is a placeholder for the full implementation
	rs.logger.WithFields(map[string]any{
		"request_id": requestID,
		"device_id":  request.DeviceID,
		"component":  "resolution",
	}).Info("Executing resolution request")

	// Update status to completed
	now := time.Now()
	rs.db.Model(&request).Updates(map[string]interface{}{
		"status":       StatusCompleted,
		"completed_at": &now,
	})
}

func (rs *ResolutionService) categorizePathForResolution(path string) string {
	path = strings.ToLower(path)

	if strings.Contains(path, "_metadata") || strings.Contains(path, "device_info") {
		return "metadata"
	}
	if strings.Contains(path, "auth") || strings.Contains(path, "password") || strings.Contains(path, "login") {
		return "security"
	}
	if strings.Contains(path, "wifi") || strings.Contains(path, "network") || strings.Contains(path, "ip") {
		return "network"
	}
	if strings.Contains(path, "relay") || strings.Contains(path, "switch") || strings.Contains(path, "dimmer") {
		return "device"
	}

	return "system"
}

func (rs *ResolutionService) determinePriority(severity string) string {
	switch severity {
	case "critical":
		return string(PriorityCritical)
	case "warning":
		return string(PriorityHigh)
	case "info":
		return string(PriorityMedium)
	default:
		return string(PriorityLow)
	}
}

func (rs *ResolutionService) isPathExcluded(path string, excludedPaths []string) bool {
	for _, excluded := range excludedPaths {
		if strings.Contains(path, excluded) {
			return true
		}
	}
	return false
}

func (rs *ResolutionService) isMetadataPath(path string) bool {
	return strings.Contains(path, "_metadata") || strings.Contains(path, "device_info")
}

func (rs *ResolutionService) determineAutoFixStrategy(path string, storedValue, currentValue interface{}, policy *ResolutionPolicy) ResolutionStrategy {
	// For metadata, always update stored to match current
	if rs.isMetadataPath(path) {
		return StrategyUpdate
	}

	// For security paths, always restore stored configuration
	if rs.categorizePathForResolution(path) == "security" {
		return StrategyRestore
	}

	// Default: restore stored configuration
	return StrategyRestore
}

func (rs *ResolutionService) getValueAtPath(data map[string]interface{}, path string) interface{} {
	// Simplified path traversal - would need proper implementation
	return data[path]
}

func (rs *ResolutionService) setValueAtPath(data map[string]interface{}, path string, value interface{}) {
	// Simplified path setting - would need proper implementation
	data[path] = value
}
