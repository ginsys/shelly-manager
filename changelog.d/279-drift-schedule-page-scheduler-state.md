Changed

- The Drift schedules page no longer says schedules are not executed. It reads the scheduler state and shows a banner when the scheduler is off on the server (`drift.scheduler_enabled`), a banner when that state could not be read, and the badge "Disabled", "Scheduler off" or "Enabled" on each schedule. Refs ginsys/shelly-manager#279.
