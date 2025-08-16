package configuration

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/shelly"
)

// Scheduler manages automated drift detection schedules
type Scheduler struct {
	db           *gorm.DB
	service      *Service
	cron         *cron.Cron
	logger       *logging.Logger
	mu           sync.RWMutex
	scheduleJobs map[uint]cron.EntryID // maps schedule ID to cron job ID
	running      bool
}

// NewScheduler creates a new drift detection scheduler
func NewScheduler(db *gorm.DB, service *Service, logger *logging.Logger) *Scheduler {
	return &Scheduler{
		db:           db,
		service:      service,
		cron:         cron.New(cron.WithSeconds()),
		logger:       logger,
		scheduleJobs: make(map[uint]cron.EntryID),
		running:      false,
	}
}

// Start begins the scheduler and loads existing schedules
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("scheduler is already running")
	}

	s.logger.Info("Starting drift detection scheduler")

	// Load existing schedules from database
	if err := s.loadSchedules(); err != nil {
		return fmt.Errorf("failed to load schedules: %w", err)
	}

	s.cron.Start()
	s.running = true

	s.logger.Info("Drift detection scheduler started successfully")
	return nil
}

// Stop gracefully stops the scheduler
func (s *Scheduler) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.logger.Info("Stopping drift detection scheduler")

	ctx := s.cron.Stop()
	select {
	case <-ctx.Done():
		s.logger.Info("Scheduler stopped gracefully")
	case <-time.After(30 * time.Second):
		s.logger.Warn("Scheduler stop timeout exceeded")
	}

	s.running = false
	s.scheduleJobs = make(map[uint]cron.EntryID)

	s.logger.Info("Drift detection scheduler stopped")
	return nil
}

// loadSchedules loads all active schedules from the database and adds them to cron
func (s *Scheduler) loadSchedules() error {
	var schedules []DriftDetectionSchedule
	if err := s.db.Where("enabled = ?", true).Find(&schedules).Error; err != nil {
		return fmt.Errorf("failed to query schedules: %w", err)
	}

	s.logger.Info("Loading drift detection schedules", "count", len(schedules))

	for _, schedule := range schedules {
		if err := s.addScheduleToCron(schedule); err != nil {
			s.logger.Error("Failed to add schedule to cron", "schedule_id", schedule.ID, "error", err)
			continue
		}
		s.logger.Debug("Added schedule to cron", "schedule_id", schedule.ID, "name", schedule.Name, "cron_spec", schedule.CronSpec)
	}

	return nil
}

// addScheduleToCron adds a single schedule to the cron scheduler
func (s *Scheduler) addScheduleToCron(schedule DriftDetectionSchedule) error {
	job := func() {
		s.executeSchedule(schedule.ID)
	}

	entryID, err := s.cron.AddFunc(schedule.CronSpec, job)
	if err != nil {
		return fmt.Errorf("failed to add cron job: %w", err)
	}

	s.scheduleJobs[schedule.ID] = entryID

	// Update next run time
	nextRun := s.cron.Entry(entryID).Next
	if err := s.db.Model(&schedule).Update("next_run", nextRun).Error; err != nil {
		s.logger.Error("Failed to update next run time", "schedule_id", schedule.ID, "error", err)
	}

	return nil
}

// executeSchedule runs drift detection for a specific schedule
func (s *Scheduler) executeSchedule(scheduleID uint) {
	s.logger.Info("Executing drift detection schedule", "schedule_id", scheduleID)

	// Get schedule details
	var schedule DriftDetectionSchedule
	if err := s.db.First(&schedule, scheduleID).Error; err != nil {
		s.logger.Error("Failed to get schedule", "schedule_id", scheduleID, "error", err)
		return
	}

	// Check if schedule is still enabled
	if !schedule.Enabled {
		s.logger.Debug("Schedule is disabled, skipping", "schedule_id", scheduleID)
		return
	}

	// Create drift detection run record
	run := DriftDetectionRun{
		ScheduleID: scheduleID,
		Status:     "running",
		StartedAt:  time.Now(),
	}

	if err := s.db.Create(&run).Error; err != nil {
		s.logger.Error("Failed to create drift detection run record", "schedule_id", scheduleID, "error", err)
		return
	}

	// Execute drift detection
	startTime := time.Now()
	result, err := s.executeDriftDetection(schedule)
	duration := time.Since(startTime)

	// Update run record
	completedAt := startTime.Add(duration)
	run.CompletedAt = &completedAt
	run.Duration = &duration

	if err != nil {
		run.Status = "failed"
		run.Error = err.Error()
		s.logger.Error("Drift detection failed", "schedule_id", scheduleID, "run_id", run.ID, "error", err)
	} else {
		run.Status = "completed"
		if resultJSON, err := json.Marshal(result); err == nil {
			run.Results = resultJSON
		}
		s.logger.Info("Drift detection completed",
			"schedule_id", scheduleID,
			"run_id", run.ID,
			"total", result.Total,
			"drifted", result.Drifted,
			"errors", result.Errors,
			"duration", duration)
	}

	// Save run record
	if err := s.db.Save(&run).Error; err != nil {
		s.logger.Error("Failed to update drift detection run record", "run_id", run.ID, "error", err)
	}

	// Update schedule statistics
	now := time.Now()
	updates := map[string]interface{}{
		"last_run":   now,
		"run_count":  gorm.Expr("run_count + 1"),
		"updated_at": now,
	}

	// Calculate next run time
	if entryID, exists := s.scheduleJobs[scheduleID]; exists {
		if entry := s.cron.Entry(entryID); entry.Valid() {
			updates["next_run"] = entry.Next
		}
	}

	if err := s.db.Model(&schedule).Updates(updates).Error; err != nil {
		s.logger.Error("Failed to update schedule statistics", "schedule_id", scheduleID, "error", err)
	}
}

// executeDriftDetection performs the actual drift detection
func (s *Scheduler) executeDriftDetection(schedule DriftDetectionSchedule) (*BulkDriftResult, error) {
	// Get devices to check
	deviceIDs, err := s.getDevicesForSchedule(schedule)
	if err != nil {
		return nil, fmt.Errorf("failed to get devices for schedule: %w", err)
	}

	if len(deviceIDs) == 0 {
		s.logger.Debug("No devices to check for schedule", "schedule_id", schedule.ID)
		return &BulkDriftResult{
			Total:       0,
			InSync:      0,
			Drifted:     0,
			Errors:      0,
			Results:     []DriftResult{},
			StartedAt:   time.Now(),
			CompletedAt: time.Now(),
			Duration:    0,
		}, nil
	}

	// Use the service to perform bulk drift detection
	clientGetter := func(deviceID uint) (shelly.Client, error) {
		return s.service.createClientForDevice(deviceID)
	}

	return s.service.BulkDetectDrift(deviceIDs, clientGetter)
}

// getDevicesForSchedule determines which devices to check for a given schedule
func (s *Scheduler) getDevicesForSchedule(schedule DriftDetectionSchedule) ([]uint, error) {
	// If specific device IDs are stored in the schedule, use those
	if len(schedule.DeviceIDs) > 0 {
		return schedule.DeviceIDs, nil
	}

	// Parse device filter if present
	if len(schedule.DeviceFilter) > 0 {
		var filter map[string]interface{}
		if err := json.Unmarshal(schedule.DeviceFilter, &filter); err != nil {
			return nil, fmt.Errorf("failed to parse device filter: %w", err)
		}

		// Apply filter to get device IDs
		return s.getDevicesWithFilter(filter)
	}

	// Default: get all devices
	return s.getAllDeviceIDs()
}

// getDevicesWithFilter applies filter criteria to get matching device IDs
func (s *Scheduler) getDevicesWithFilter(filter map[string]interface{}) ([]uint, error) {
	query := s.db.Model(&struct {
		ID uint `json:"id"`
	}{}).Table("devices")

	// Apply filter conditions
	if deviceType, ok := filter["device_type"].(string); ok && deviceType != "" {
		query = query.Where("device_type = ?", deviceType)
	}

	if generation, ok := filter["generation"].(float64); ok {
		query = query.Where("generation = ?", int(generation))
	}

	if enabled, ok := filter["enabled"].(bool); ok {
		query = query.Where("enabled = ?", enabled)
	}

	var deviceIDs []uint
	if err := query.Pluck("id", &deviceIDs).Error; err != nil {
		return nil, fmt.Errorf("failed to query devices with filter: %w", err)
	}

	return deviceIDs, nil
}

// getAllDeviceIDs gets all device IDs
func (s *Scheduler) getAllDeviceIDs() ([]uint, error) {
	var deviceIDs []uint
	if err := s.db.Model(&struct {
		ID uint `json:"id"`
	}{}).Table("devices").Pluck("id", &deviceIDs).Error; err != nil {
		return nil, fmt.Errorf("failed to get all device IDs: %w", err)
	}

	return deviceIDs, nil
}

// AddSchedule creates a new drift detection schedule
func (s *Scheduler) AddSchedule(schedule DriftDetectionSchedule) (*DriftDetectionSchedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate cron expression
	if _, err := cron.ParseStandard(schedule.CronSpec); err != nil {
		return nil, fmt.Errorf("invalid cron expression: %w", err)
	}

	// Save to database
	if err := s.db.Create(&schedule).Error; err != nil {
		return nil, fmt.Errorf("failed to create schedule: %w", err)
	}

	// Add to cron if enabled and scheduler is running
	if schedule.Enabled && s.running {
		if err := s.addScheduleToCron(schedule); err != nil {
			s.logger.Error("Failed to add new schedule to cron", "schedule_id", schedule.ID, "error", err)
		}
	}

	s.logger.Info("Created new drift detection schedule", "schedule_id", schedule.ID, "name", schedule.Name)
	return &schedule, nil
}

// UpdateSchedule updates an existing drift detection schedule
func (s *Scheduler) UpdateSchedule(scheduleID uint, updates DriftDetectionSchedule) (*DriftDetectionSchedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get existing schedule
	var schedule DriftDetectionSchedule
	if err := s.db.First(&schedule, scheduleID).Error; err != nil {
		return nil, fmt.Errorf("schedule not found: %w", err)
	}

	// Validate cron expression if changed
	if updates.CronSpec != "" && updates.CronSpec != schedule.CronSpec {
		if _, err := cron.ParseStandard(updates.CronSpec); err != nil {
			return nil, fmt.Errorf("invalid cron expression: %w", err)
		}
	}

	// Remove from cron if it exists
	if entryID, exists := s.scheduleJobs[scheduleID]; exists {
		s.cron.Remove(entryID)
		delete(s.scheduleJobs, scheduleID)
	}

	// Update database record
	if err := s.db.Model(&schedule).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update schedule: %w", err)
	}

	// Reload the updated schedule
	if err := s.db.First(&schedule, scheduleID).Error; err != nil {
		return nil, fmt.Errorf("failed to reload updated schedule: %w", err)
	}

	// Add back to cron if enabled and scheduler is running
	if schedule.Enabled && s.running {
		if err := s.addScheduleToCron(schedule); err != nil {
			s.logger.Error("Failed to re-add updated schedule to cron", "schedule_id", schedule.ID, "error", err)
		}
	}

	s.logger.Info("Updated drift detection schedule", "schedule_id", schedule.ID, "name", schedule.Name)
	return &schedule, nil
}

// DeleteSchedule removes a drift detection schedule
func (s *Scheduler) DeleteSchedule(scheduleID uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove from cron if it exists
	if entryID, exists := s.scheduleJobs[scheduleID]; exists {
		s.cron.Remove(entryID)
		delete(s.scheduleJobs, scheduleID)
	}

	// Delete from database
	if err := s.db.Delete(&DriftDetectionSchedule{}, scheduleID).Error; err != nil {
		return fmt.Errorf("failed to delete schedule: %w", err)
	}

	s.logger.Info("Deleted drift detection schedule", "schedule_id", scheduleID)
	return nil
}

// GetSchedules returns all drift detection schedules
func (s *Scheduler) GetSchedules() ([]DriftDetectionSchedule, error) {
	var schedules []DriftDetectionSchedule
	if err := s.db.Find(&schedules).Error; err != nil {
		return nil, fmt.Errorf("failed to get schedules: %w", err)
	}
	return schedules, nil
}

// GetSchedule returns a specific drift detection schedule
func (s *Scheduler) GetSchedule(scheduleID uint) (*DriftDetectionSchedule, error) {
	var schedule DriftDetectionSchedule
	if err := s.db.First(&schedule, scheduleID).Error; err != nil {
		return nil, fmt.Errorf("schedule not found: %w", err)
	}
	return &schedule, nil
}

// GetScheduleRuns returns the execution history for a schedule
func (s *Scheduler) GetScheduleRuns(scheduleID uint, limit int) ([]DriftDetectionRun, error) {
	var runs []DriftDetectionRun
	query := s.db.Where("schedule_id = ?", scheduleID).Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&runs).Error; err != nil {
		return nil, fmt.Errorf("failed to get schedule runs: %w", err)
	}

	return runs, nil
}

// IsRunning returns whether the scheduler is currently running
func (s *Scheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}
