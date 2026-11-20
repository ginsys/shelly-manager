package configuration

import "fmt"

// ListDriftScheduleRuns returns the newest runs of a schedule first, at most limit of them, without
// their results (a result document holds a whole fleet). A schedule that does not exist is
// gorm.ErrRecordNotFound; one with no run yet gives an empty list.
func (s *Service) ListDriftScheduleRuns(scheduleID uint, limit int) ([]DriftDetectionRun, error) {
	if err := s.db.Select("id").First(&DriftDetectionSchedule{}, scheduleID).Error; err != nil {
		return nil, err
	}
	runs := []DriftDetectionRun{}
	err := s.db.Omit("results").Where("schedule_id = ?", scheduleID).
		Order("started_at DESC, id DESC").Limit(limit).Find(&runs).Error
	if err != nil {
		return nil, fmt.Errorf("read the runs of the drift schedule: %w", err)
	}
	return runs, nil
}
