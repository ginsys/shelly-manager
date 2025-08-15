package configuration

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
	"gorm.io/gorm"
)

// Reporter handles comprehensive drift analysis and reporting
type Reporter struct {
	db     *gorm.DB
	logger *logging.Logger
}

// NewReporter creates a new drift reporter
func NewReporter(db *gorm.DB, logger *logging.Logger) *Reporter {
	return &Reporter{
		db:     db,
		logger: logger,
	}
}

// GenerateComprehensiveReport creates a detailed drift analysis report
func (r *Reporter) GenerateComprehensiveReport(reportType string, deviceID *uint, scheduleID *uint, driftResults []DriftResult) (*DriftReport, error) {
	r.logger.WithFields(map[string]any{
		"report_type": reportType,
		"device_id":   deviceID,
		"schedule_id": scheduleID,
		"devices":     len(driftResults),
		"component":   "reporter",
	}).Info("Generating comprehensive drift report")

	report := &DriftReport{
		ReportType:  reportType,
		DeviceID:    deviceID,
		ScheduleID:  scheduleID,
		GeneratedAt: time.Now(),
		CreatedAt:   time.Now(),
	}

	// Analyze devices and generate comprehensive analysis
	devices, summary, err := r.analyzeDeviceDrifts(driftResults)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze device drifts: %w", err)
	}

	report.Summary = summary
	report.Devices = devices

	// Generate actionable recommendations
	recommendations := r.generateRecommendations(devices, summary)
	report.Recommendations = recommendations

	// Update trend tracking
	if err := r.updateDriftTrends(devices); err != nil {
		r.logger.WithFields(map[string]any{
			"error":     err.Error(),
			"component": "reporter",
		}).Warn("Failed to update drift trends")
	}

	// Serialize arrays and maps for database storage
	if devicesJSON, err := json.Marshal(devices); err == nil {
		report.DevicesJSON = devicesJSON
	}
	if recommendationsJSON, err := json.Marshal(recommendations); err == nil {
		report.RecommendationsJSON = recommendationsJSON
	}
	if categoriesJSON, err := json.Marshal(summary.CategoriesAffected); err == nil {
		report.Summary.CategoriesAffectedJSON = categoriesJSON
	}
	if driftsJSON, err := json.Marshal(summary.MostCommonDrifts); err == nil {
		report.Summary.MostCommonDriftsJSON = driftsJSON
	}

	// Save report to database
	if err := r.db.Create(report).Error; err != nil {
		return nil, fmt.Errorf("failed to save drift report: %w", err)
	}

	r.logger.WithFields(map[string]any{
		"report_id":       report.ID,
		"devices_drifted": summary.DevicesDrifted,
		"critical_issues": summary.CriticalIssues,
		"recommendations": len(recommendations),
		"component":       "reporter",
	}).Info("Generated comprehensive drift report")

	return report, nil
}

// analyzeDeviceDrifts performs detailed analysis of device drift results
func (r *Reporter) analyzeDeviceDrifts(driftResults []DriftResult) ([]DeviceDriftAnalysis, DriftSummary, error) {
	devices := make([]DeviceDriftAnalysis, 0, len(driftResults))
	summary := DriftSummary{
		TotalDevices:       len(driftResults),
		DevicesInSync:      0,
		DevicesDrifted:     0,
		DevicesErrored:     0,
		CriticalIssues:     0,
		WarningIssues:      0,
		InfoIssues:         0,
		CategoriesAffected: make(map[string]int),
		SecurityConcerns:   0,
		NetworkChanges:     0,
	}

	// Get device information from database
	deviceMap, err := r.getDeviceMap()
	if err != nil {
		return nil, summary, fmt.Errorf("failed to get device information: %w", err)
	}

	// Track common drift patterns
	driftPatterns := make(map[string]*CommonDrift)

	for _, result := range driftResults {
		device := r.analyzeDeviceResult(result, deviceMap)
		devices = append(devices, device)

		// Update summary statistics
		switch device.Status {
		case "synced":
			summary.DevicesInSync++
		case "drift":
			summary.DevicesDrifted++
		case "error":
			summary.DevicesErrored++
		}

		// Count issues by severity
		summary.CriticalIssues += device.CriticalCount
		summary.WarningIssues += device.WarningCount
		summary.InfoIssues += device.InfoCount

		// Track categories and patterns
		for _, diff := range device.Differences {
			summary.CategoriesAffected[diff.Category]++

			// Track security and network changes
			if diff.Category == "security" {
				summary.SecurityConcerns++
			}
			if diff.Category == "network" {
				summary.NetworkChanges++
			}

			// Track common drift patterns
			key := fmt.Sprintf("%s:%s", diff.Path, diff.Type)
			if pattern, exists := driftPatterns[key]; exists {
				pattern.Count++
			} else {
				driftPatterns[key] = &CommonDrift{
					Path:        diff.Path,
					Count:       1,
					Severity:    diff.Severity,
					Category:    diff.Category,
					Description: diff.Description,
				}
			}
		}
	}

	// Calculate percentages for common drifts and get top patterns
	totalDrifted := float64(summary.DevicesDrifted)
	if totalDrifted > 0 {
		for _, pattern := range driftPatterns {
			pattern.Percentage = (float64(pattern.Count) / totalDrifted) * 100
		}
	}

	// Sort and get top 10 most common drifts
	summary.MostCommonDrifts = r.getTopCommonDrifts(driftPatterns, 10)

	return devices, summary, nil
}

// analyzeDeviceResult performs detailed analysis of a single device's drift result
func (r *Reporter) analyzeDeviceResult(result DriftResult, deviceMap map[uint]*Device) DeviceDriftAnalysis {
	device := DeviceDriftAnalysis{
		DeviceID:          result.DeviceID,
		DeviceName:        result.DeviceName,
		DeviceIP:          result.DeviceIP,
		Status:            result.Status,
		DriftDetectedTime: time.Now(),
		Error:             result.Error,
	}

	// Get additional device information
	if dbDevice, exists := deviceMap[result.DeviceID]; exists {
		device.DeviceType = dbDevice.Type
		// Parse generation from settings
		if dbDevice.Settings != "" {
			var settings struct {
				Gen int `json:"gen"`
			}
			if err := json.Unmarshal([]byte(dbDevice.Settings), &settings); err == nil {
				device.Generation = settings.Gen
			}
		}
	}

	// If there's drift, analyze the differences
	if result.Drift != nil {
		device.Differences = r.enhanceDifferences(result.Drift.Differences)
		device.TotalDifferences = len(device.Differences)

		// Count by severity and calculate health metrics
		for _, diff := range device.Differences {
			switch diff.Severity {
			case "critical":
				device.CriticalCount++
			case "warning":
				device.WarningCount++
			case "info":
				device.InfoCount++
			}
		}

		// Calculate health score and risk level
		device.HealthScore = r.calculateHealthScore(device)
		device.RiskLevel = r.determineRiskLevel(device)
		device.DriftSeverity = r.calculateDriftSeverity(device)
	} else {
		device.HealthScore = 100.0
		device.RiskLevel = "low"
		device.DriftSeverity = "none"
	}

	return device
}

// enhanceDifferences adds detailed analysis to configuration differences
func (r *Reporter) enhanceDifferences(differences []ConfigDifference) []ConfigDifference {
	enhanced := make([]ConfigDifference, len(differences))

	for i, diff := range differences {
		enhanced[i] = diff

		// Enhance with category, severity, description, impact, and suggestion
		category, severity := r.categorizeDifference(diff.Path, diff.Type)
		enhanced[i].Category = category
		enhanced[i].Severity = severity
		enhanced[i].Description = r.generateDescription(diff)
		enhanced[i].Impact = r.assessImpact(diff, category, severity)
		enhanced[i].Suggestion = r.generateSuggestion(diff, category, severity)
	}

	return enhanced
}

// categorizeDifference determines the category and severity of a configuration difference
func (r *Reporter) categorizeDifference(path, diffType string) (category, severity string) {
	path = strings.ToLower(path)

	// Security-related paths
	if strings.Contains(path, "auth") || strings.Contains(path, "password") ||
		strings.Contains(path, "login") || strings.Contains(path, "user") {
		return "security", "critical"
	}

	// Network configuration
	if strings.Contains(path, "wifi") || strings.Contains(path, "ip") ||
		strings.Contains(path, "network") || strings.Contains(path, "mqtt") ||
		strings.Contains(path, "cloud") {
		if strings.Contains(path, "ip") || strings.Contains(path, "wifi.sta.ssid") {
			return "network", "warning"
		}
		return "network", "info"
	}

	// Device configuration
	if strings.Contains(path, "relay") || strings.Contains(path, "switch") ||
		strings.Contains(path, "dimmer") || strings.Contains(path, "roller") ||
		strings.Contains(path, "components") {
		return "device", "warning"
	}

	// System configuration
	if strings.Contains(path, "sys") || strings.Contains(path, "device.name") ||
		strings.Contains(path, "timezone") || strings.Contains(path, "debug") {
		return "system", "info"
	}

	// Metadata (usually informational)
	if strings.Contains(path, "_metadata") || strings.Contains(path, "device_info") {
		return "metadata", "info"
	}

	// Default categorization
	if diffType == "removed" {
		return "system", "warning"
	}

	return "system", "info"
}

// generateDescription creates a human-readable description of the difference
func (r *Reporter) generateDescription(diff ConfigDifference) string {
	path := diff.Path

	switch diff.Type {
	case "added":
		return fmt.Sprintf("New configuration added at '%s'", path)
	case "removed":
		return fmt.Sprintf("Configuration removed from '%s'", path)
	case "modified":
		return fmt.Sprintf("Configuration changed at '%s'", path)
	default:
		return fmt.Sprintf("Configuration difference at '%s'", path)
	}
}

// assessImpact evaluates the potential impact of a configuration change
func (r *Reporter) assessImpact(diff ConfigDifference, category, severity string) string {
	switch category {
	case "security":
		return "May affect device security and access control"
	case "network":
		if severity == "warning" {
			return "May affect device connectivity and network communication"
		}
		return "Minor network configuration change"
	case "device":
		return "May affect device functionality and behavior"
	case "system":
		if severity == "warning" {
			return "May affect system stability or configuration"
		}
		return "Minor system configuration change"
	case "metadata":
		return "Informational change, no functional impact"
	default:
		return "Configuration change with potential unknown impact"
	}
}

// generateSuggestion provides actionable recommendations for addressing the difference
func (r *Reporter) generateSuggestion(diff ConfigDifference, category, severity string) string {
	switch severity {
	case "critical":
		return "Review immediately and verify if change is authorized"
	case "warning":
		return "Review change and update stored configuration if intended"
	case "info":
		return "Monitor for consistency, update if needed"
	default:
		return "Review and take action if necessary"
	}
}

// calculateHealthScore computes a health score based on drift severity
func (r *Reporter) calculateHealthScore(device DeviceDriftAnalysis) float64 {
	if device.TotalDifferences == 0 {
		return 100.0
	}

	// Weight different severity levels
	criticalWeight := 20.0
	warningWeight := 10.0
	infoWeight := 2.0

	totalPenalty := float64(device.CriticalCount)*criticalWeight +
		float64(device.WarningCount)*warningWeight +
		float64(device.InfoCount)*infoWeight

	// Cap the penalty to ensure score doesn't go below 0
	maxPenalty := 100.0
	if totalPenalty > maxPenalty {
		totalPenalty = maxPenalty
	}

	return math.Max(0, 100.0-totalPenalty)
}

// determineRiskLevel calculates risk level based on health score and issue types
func (r *Reporter) determineRiskLevel(device DeviceDriftAnalysis) string {
	if device.CriticalCount > 0 {
		return "critical"
	}

	if device.HealthScore < 50 {
		return "high"
	}

	if device.WarningCount > 0 || device.HealthScore < 80 {
		return "medium"
	}

	return "low"
}

// calculateDriftSeverity determines overall drift severity for the device
func (r *Reporter) calculateDriftSeverity(device DeviceDriftAnalysis) string {
	if device.CriticalCount > 0 {
		return "critical"
	}

	if device.WarningCount > 3 {
		return "high"
	}

	if device.WarningCount > 0 {
		return "medium"
	}

	if device.InfoCount > 5 {
		return "low"
	}

	if device.TotalDifferences > 0 {
		return "low"
	}

	return "none"
}

// generateRecommendations creates actionable recommendations based on drift analysis
func (r *Reporter) generateRecommendations(devices []DeviceDriftAnalysis, summary DriftSummary) []DriftRecommendation {
	recommendations := []DriftRecommendation{}

	// Security recommendations
	if summary.SecurityConcerns > 0 {
		securityDevices := r.getDevicesWithCategory(devices, "security")
		if len(securityDevices) > 0 {
			recommendations = append(recommendations, DriftRecommendation{
				Priority:        "high",
				Category:        "security",
				Title:           "Security Configuration Drift Detected",
				Description:     fmt.Sprintf("Security-related configuration changes detected on %d device(s). These changes may affect device access control and security.", len(securityDevices)),
				AffectedDevices: securityDevices,
				Actions: []RecommendedAction{
					{
						Type:        "manual-review",
						Description: "Review authentication and security settings",
						Automated:   false,
					},
					{
						Type:        "manual-action",
						Description: "Verify credentials and access controls",
						Automated:   false,
					},
				},
				Impact: "High - Security vulnerabilities may exist",
			})
		}
	}

	// Network recommendations
	if summary.NetworkChanges > 0 {
		networkDevices := r.getDevicesWithCategory(devices, "network")
		if len(networkDevices) > 0 {
			recommendations = append(recommendations, DriftRecommendation{
				Priority:        "medium",
				Category:        "network",
				Title:           "Network Configuration Changes",
				Description:     fmt.Sprintf("Network configuration changes detected on %d device(s). Verify connectivity settings.", len(networkDevices)),
				AffectedDevices: networkDevices,
				Actions: []RecommendedAction{
					{
						Type:        "auto-fix",
						Description: "Synchronize network settings from stored configuration",
						Automated:   true,
					},
					{
						Type:        "monitor",
						Description: "Monitor device connectivity after synchronization",
						Automated:   true,
					},
				},
				Impact: "Medium - May affect device connectivity",
			})
		}
	}

	// High drift device recommendations
	criticalDevices := r.getDevicesWithRiskLevel(devices, "critical")
	if len(criticalDevices) > 0 {
		recommendations = append(recommendations, DriftRecommendation{
			Priority:        "high",
			Category:        "maintenance",
			Title:           "Critical Configuration Drift",
			Description:     fmt.Sprintf("%d device(s) have critical configuration drift requiring immediate attention.", len(criticalDevices)),
			AffectedDevices: criticalDevices,
			Actions: []RecommendedAction{
				{
					Type:        "manual-review",
					Description: "Immediate review of critical configuration changes",
					Automated:   false,
				},
				{
					Type:        "manual-action",
					Description: "Restore known-good configuration or validate changes",
					Automated:   false,
				},
			},
			Impact: "Critical - Device functionality may be compromised",
		})
	}

	// Bulk synchronization recommendation
	if summary.DevicesDrifted > 3 {
		recommendations = append(recommendations, DriftRecommendation{
			Priority:        "medium",
			Category:        "maintenance",
			Title:           "Bulk Configuration Synchronization",
			Description:     fmt.Sprintf("Multiple devices (%d) have configuration drift. Consider bulk synchronization.", summary.DevicesDrifted),
			AffectedDevices: r.getAllDriftedDevices(devices),
			Actions: []RecommendedAction{
				{
					Type:        "auto-fix",
					Description: "Perform bulk configuration export to synchronize devices",
					Command:     "bulk-export",
					Automated:   true,
				},
			},
			Impact: "Medium - Improves overall configuration consistency",
		})
	}

	return recommendations
}

// Helper functions for generating recommendations

func (r *Reporter) getDevicesWithCategory(devices []DeviceDriftAnalysis, category string) []uint {
	deviceIDs := []uint{}
	for _, device := range devices {
		for _, diff := range device.Differences {
			if diff.Category == category {
				deviceIDs = append(deviceIDs, device.DeviceID)
				break
			}
		}
	}
	return deviceIDs
}

func (r *Reporter) getDevicesWithRiskLevel(devices []DeviceDriftAnalysis, riskLevel string) []uint {
	deviceIDs := []uint{}
	for _, device := range devices {
		if device.RiskLevel == riskLevel {
			deviceIDs = append(deviceIDs, device.DeviceID)
		}
	}
	return deviceIDs
}

func (r *Reporter) getAllDriftedDevices(devices []DeviceDriftAnalysis) []uint {
	deviceIDs := []uint{}
	for _, device := range devices {
		if device.Status == "drift" {
			deviceIDs = append(deviceIDs, device.DeviceID)
		}
	}
	return deviceIDs
}

// getTopCommonDrifts returns the most common drift patterns
func (r *Reporter) getTopCommonDrifts(patterns map[string]*CommonDrift, limit int) []CommonDrift {
	drifts := make([]CommonDrift, 0, len(patterns))
	for _, pattern := range patterns {
		drifts = append(drifts, *pattern)
	}

	// Sort by count (descending) then by severity
	sort.Slice(drifts, func(i, j int) bool {
		if drifts[i].Count == drifts[j].Count {
			// If counts are equal, prioritize by severity
			severityOrder := map[string]int{"critical": 3, "warning": 2, "info": 1}
			return severityOrder[drifts[i].Severity] > severityOrder[drifts[j].Severity]
		}
		return drifts[i].Count > drifts[j].Count
	})

	if len(drifts) > limit {
		drifts = drifts[:limit]
	}

	return drifts
}

// updateDriftTrends tracks drift patterns over time
func (r *Reporter) updateDriftTrends(devices []DeviceDriftAnalysis) error {
	now := time.Now()

	for _, device := range devices {
		for _, diff := range device.Differences {
			// Check if trend already exists
			var trend DriftTrend
			result := r.db.Where("device_id = ? AND path = ? AND resolved = ?",
				device.DeviceID, diff.Path, false).First(&trend)

			if result.Error == gorm.ErrRecordNotFound {
				// Create new trend
				trend = DriftTrend{
					DeviceID:    device.DeviceID,
					Path:        diff.Path,
					Category:    diff.Category,
					Severity:    diff.Severity,
					FirstSeen:   now,
					LastSeen:    now,
					Occurrences: 1,
					Resolved:    false,
					CreatedAt:   now,
					UpdatedAt:   now,
				}
				if err := r.db.Create(&trend).Error; err != nil {
					r.logger.WithFields(map[string]any{
						"device_id": device.DeviceID,
						"path":      diff.Path,
						"error":     err.Error(),
						"component": "reporter",
					}).Error("Failed to create drift trend")
				}
			} else if result.Error == nil {
				// Update existing trend
				trend.LastSeen = now
				trend.Occurrences++
				trend.Severity = diff.Severity // Update severity in case it changed
				trend.UpdatedAt = now

				if err := r.db.Save(&trend).Error; err != nil {
					r.logger.WithFields(map[string]any{
						"trend_id":  trend.ID,
						"device_id": device.DeviceID,
						"path":      diff.Path,
						"error":     err.Error(),
						"component": "reporter",
					}).Error("Failed to update drift trend")
				}
			}
		}
	}

	return nil
}

// getDeviceMap creates a map of device ID to device information
func (r *Reporter) getDeviceMap() (map[uint]*Device, error) {
	var devices []Device
	if err := r.db.Find(&devices).Error; err != nil {
		return nil, err
	}

	deviceMap := make(map[uint]*Device)
	for i := range devices {
		deviceMap[devices[i].ID] = &devices[i]
	}

	return deviceMap, nil
}

// GetReports retrieves drift reports with optional filtering
func (r *Reporter) GetReports(reportType string, deviceID *uint, limit int) ([]DriftReport, error) {
	query := r.db.Model(&DriftReport{})

	if reportType != "" {
		query = query.Where("report_type = ?", reportType)
	}

	if deviceID != nil {
		query = query.Where("device_id = ?", *deviceID)
	}

	if limit > 0 {
		query = query.Limit(limit)
	}

	query = query.Order("created_at DESC")

	var reports []DriftReport
	if err := query.Find(&reports).Error; err != nil {
		return nil, fmt.Errorf("failed to get drift reports: %w", err)
	}

	// Deserialize JSON fields
	for i := range reports {
		if len(reports[i].DevicesJSON) > 0 {
			if err := json.Unmarshal(reports[i].DevicesJSON, &reports[i].Devices); err != nil {
				r.logger.WithFields(map[string]any{
					"report_id": reports[i].ID,
					"error":     err.Error(),
					"component": "reporter",
				}).Warn("Failed to deserialize devices JSON")
			}
		}

		if len(reports[i].RecommendationsJSON) > 0 {
			if err := json.Unmarshal(reports[i].RecommendationsJSON, &reports[i].Recommendations); err != nil {
				r.logger.WithFields(map[string]any{
					"report_id": reports[i].ID,
					"error":     err.Error(),
					"component": "reporter",
				}).Warn("Failed to deserialize recommendations JSON")
			}
		}

		// Deserialize summary JSON fields
		if len(reports[i].Summary.CategoriesAffectedJSON) > 0 {
			if err := json.Unmarshal(reports[i].Summary.CategoriesAffectedJSON, &reports[i].Summary.CategoriesAffected); err != nil {
				r.logger.WithFields(map[string]any{
					"report_id": reports[i].ID,
					"error":     err.Error(),
					"component": "reporter",
				}).Warn("Failed to deserialize categories affected JSON")
			}
		}

		if len(reports[i].Summary.MostCommonDriftsJSON) > 0 {
			if err := json.Unmarshal(reports[i].Summary.MostCommonDriftsJSON, &reports[i].Summary.MostCommonDrifts); err != nil {
				r.logger.WithFields(map[string]any{
					"report_id": reports[i].ID,
					"error":     err.Error(),
					"component": "reporter",
				}).Warn("Failed to deserialize most common drifts JSON")
			}
		}
	}

	return reports, nil
}

// GetDriftTrends retrieves drift trends with optional filtering
func (r *Reporter) GetDriftTrends(deviceID *uint, resolved *bool, limit int) ([]DriftTrend, error) {
	query := r.db.Model(&DriftTrend{})

	if deviceID != nil {
		query = query.Where("device_id = ?", *deviceID)
	}

	if resolved != nil {
		query = query.Where("resolved = ?", *resolved)
	}

	if limit > 0 {
		query = query.Limit(limit)
	}

	query = query.Order("last_seen DESC")

	var trends []DriftTrend
	if err := query.Find(&trends).Error; err != nil {
		return nil, fmt.Errorf("failed to get drift trends: %w", err)
	}

	return trends, nil
}

// MarkTrendResolved marks a drift trend as resolved
func (r *Reporter) MarkTrendResolved(trendID uint) error {
	now := time.Now()
	result := r.db.Model(&DriftTrend{}).Where("id = ?", trendID).Updates(map[string]interface{}{
		"resolved":    true,
		"resolved_at": &now,
		"updated_at":  now,
	})

	if result.Error != nil {
		return fmt.Errorf("failed to mark trend as resolved: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("trend not found")
	}

	return nil
}
