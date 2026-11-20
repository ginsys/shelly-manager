package configuration

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ErrInvalidDriftSchedule wraps every refusal of a schedule's input, so a caller can tell a bad
// request from a failure of the database.
var ErrInvalidDriftSchedule = errors.New("invalid drift schedule")

// ScheduleInput is what a caller may set on a schedule. A schedule checks the devices in DeviceIDs,
// or the devices DeviceFilter selects, or every device; both at once is refused.
type ScheduleInput struct {
	Name         string             `json:"name"`
	Description  string             `json:"description"`
	CronSpec     string             `json:"cron_spec"`
	Enabled      *bool              `json:"enabled"` // nil: enabled on create, unchanged on update
	DeviceIDs    []uint             `json:"device_ids"`
	DeviceFilter *DriftDeviceFilter `json:"device_filter"`
}

// validScheduleInput is an input after validation: the next instant of its spec, its sorted device ids
// and the stored form of its filter (nil: none).
type validScheduleInput struct {
	next   time.Time
	ids    []uint
	filter json.RawMessage
}

func invalidSchedule(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidDriftSchedule, fmt.Sprintf(format, args...))
}

func (s *Service) validateSchedule(in ScheduleInput, now time.Time) (validScheduleInput, error) {
	var v validScheduleInput
	if strings.TrimSpace(in.Name) == "" {
		return v, invalidSchedule("name is required")
	}
	next, err := NextRun(in.CronSpec, now)
	if err != nil {
		return v, invalidSchedule("%v", err)
	}
	v.next = next

	hasFilter := in.DeviceFilter != nil && *in.DeviceFilter != (DriftDeviceFilter{})
	if len(in.DeviceIDs) > 0 && hasFilter {
		return v, invalidSchedule("device_ids and device_filter cannot both be set")
	}
	if hasFilter {
		if v.filter, err = json.Marshal(in.DeviceFilter); err != nil {
			return v, fmt.Errorf("encode the device filter: %w", err)
		}
	}
	v.ids = slices.Compact(slices.Sorted(slices.Values(in.DeviceIDs)))
	if len(v.ids) > 0 {
		var found []uint
		if err := s.db.Table("devices").Where("id IN ?", v.ids).Pluck("id", &found).Error; err != nil {
			return v, fmt.Errorf("read the devices of the schedule: %w", err)
		}
		var missing []string
		for _, id := range v.ids {
			if !slices.Contains(found, id) {
				missing = append(missing, fmt.Sprint(id))
			}
		}
		if len(missing) > 0 {
			return v, invalidSchedule("unknown device id %s", strings.Join(missing, ", "))
		}
	}
	return v, nil
}

// CreateDriftSchedule stores a schedule and its device list in one transaction. An enabled schedule
// is armed at the next instant of its spec, never at once.
func (s *Service) CreateDriftSchedule(in ScheduleInput) (*DriftDetectionSchedule, error) {
	v, err := s.validateSchedule(in, time.Now())
	if err != nil {
		return nil, err
	}
	enabled := in.Enabled == nil || *in.Enabled
	sc := DriftDetectionSchedule{
		Name: strings.TrimSpace(in.Name), Description: in.Description, CronSpec: in.CronSpec,
		Enabled: true, DeviceFilter: v.filter,
	}
	if enabled {
		sc.NextRun = &v.next
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if res := tx.Create(&sc); res.Error != nil {
			return res.Error
		}
		if !enabled { // a false bool is the column's default (true) to gorm on create
			if res := tx.Model(&sc).Update("enabled", false); res.Error != nil {
				return res.Error
			}
		}
		return replaceScheduleDevices(tx, sc.ID, v.ids)
	})
	if err != nil {
		return nil, fmt.Errorf("create the drift schedule: %w", err)
	}
	sc.DeviceIDs = v.ids
	return &sc, nil
}

// UpdateDriftSchedule replaces the editable fields of a schedule. Its next run is recomputed when the
// spec changes or the schedule becomes enabled, and left alone otherwise, so editing a name never
// moves the next run.
func (s *Service) UpdateDriftSchedule(id uint, in ScheduleInput) (*DriftDetectionSchedule, error) {
	v, err := s.validateSchedule(in, time.Now())
	if err != nil {
		return nil, err
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var sc DriftDetectionSchedule
		if res := tx.First(&sc, id); res.Error != nil {
			return res.Error
		}
		enabled := sc.Enabled
		if in.Enabled != nil {
			enabled = *in.Enabled
		}
		fields := map[string]any{
			"name": strings.TrimSpace(in.Name), "description": in.Description, "cron_spec": in.CronSpec,
			"enabled": enabled, "device_filter": nil,
		}
		if v.filter != nil {
			fields["device_filter"] = []byte(v.filter) // the form Create stores; a string would not scan back
		}
		switch {
		case !enabled:
			fields["next_run"] = nil
		case sc.CronSpec != in.CronSpec || !sc.Enabled || sc.NextRun == nil:
			fields["next_run"] = v.next
		}
		if res := tx.Model(&DriftDetectionSchedule{}).Where("id = ?", id).Updates(fields); res.Error != nil {
			return res.Error
		}
		return replaceScheduleDevices(tx, id, v.ids)
	})
	if err != nil {
		return nil, wrapScheduleError("update the drift schedule", err)
	}
	return s.GetDriftSchedule(id)
}

// ToggleDriftSchedule flips enabled. Turning it on arms the schedule from now, so a long pause never
// causes a catch-up run; turning it off clears the next run.
func (s *Service) ToggleDriftSchedule(id uint) (*DriftDetectionSchedule, error) {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var sc DriftDetectionSchedule
		if err := tx.First(&sc, id).Error; err != nil {
			return err
		}
		fields := map[string]any{"enabled": !sc.Enabled, "next_run": nil}
		if !sc.Enabled {
			next, err := NextRun(sc.CronSpec, time.Now())
			if err != nil {
				return invalidSchedule("cannot enable: %v", err)
			}
			fields["next_run"] = next
		}
		return tx.Model(&DriftDetectionSchedule{}).Where("id = ?", id).Updates(fields).Error
	})
	if err != nil {
		return nil, wrapScheduleError("toggle the drift schedule", err)
	}
	return s.GetDriftSchedule(id)
}

// DeleteDriftSchedule removes a schedule with its device list and its runs. A schedule that does not
// exist is gorm.ErrRecordNotFound.
func (s *Service) DeleteDriftSchedule(id uint) error {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Delete(&DriftDetectionSchedule{}, id)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Where("schedule_id = ?", id).Delete(&DriftScheduleDevice{}).Error; err != nil {
			return err
		}
		return tx.Where("schedule_id = ?", id).Delete(&DriftDetectionRun{}).Error
	})
	return wrapScheduleError("delete the drift schedule", err)
}

// GetDriftSchedule returns one schedule with its device list.
func (s *Service) GetDriftSchedule(id uint) (*DriftDetectionSchedule, error) {
	var sc DriftDetectionSchedule
	if err := s.db.First(&sc, id).Error; err != nil {
		return nil, fmt.Errorf("drift schedule not found: %w", err)
	}
	lists, err := s.scheduleDeviceIDs(sc.ID)
	if err != nil {
		return nil, err
	}
	sc.DeviceIDs = lists[sc.ID]
	return &sc, nil
}

// ListDriftSchedules returns every schedule with its device list.
func (s *Service) ListDriftSchedules() ([]DriftDetectionSchedule, error) {
	var list []DriftDetectionSchedule
	if err := s.db.Order("id").Find(&list).Error; err != nil {
		return nil, fmt.Errorf("failed to get drift schedules: %w", err)
	}
	if len(list) == 0 {
		return list, nil
	}
	ids := make([]uint, len(list))
	for i, sc := range list {
		ids[i] = sc.ID
	}
	lists, err := s.scheduleDeviceIDs(ids...)
	if err != nil {
		return nil, err
	}
	for i := range list {
		list[i].DeviceIDs = lists[list[i].ID]
	}
	return list, nil
}

// scheduleDeviceIDs reads the stored device lists of the given schedules, each sorted.
func (s *Service) scheduleDeviceIDs(scheduleIDs ...uint) (map[uint][]uint, error) {
	var rows []DriftScheduleDevice
	if err := s.db.Where("schedule_id IN ?", scheduleIDs).Order("device_id").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("read the devices of the schedules: %w", err)
	}
	lists := map[uint][]uint{}
	for _, r := range rows {
		lists[r.ScheduleID] = append(lists[r.ScheduleID], r.DeviceID)
	}
	return lists, nil
}

func replaceScheduleDevices(tx *gorm.DB, scheduleID uint, ids []uint) error {
	if err := tx.Where("schedule_id = ?", scheduleID).Delete(&DriftScheduleDevice{}).Error; err != nil {
		return err
	}
	for _, id := range ids {
		if err := tx.Create(&DriftScheduleDevice{ScheduleID: scheduleID, DeviceID: id}).Error; err != nil {
			return err
		}
	}
	return nil
}

// wrapScheduleError adds what was being done to a database failure and passes a refusal of the input
// or a missing schedule through unchanged, so errors.Is still finds them.
func wrapScheduleError(what string, err error) error {
	if err == nil || errors.Is(err, ErrInvalidDriftSchedule) || errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return fmt.Errorf("%s: %w", what, err)
}
