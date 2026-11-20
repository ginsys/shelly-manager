Added

- `GET /api/v1/config/drift-schedules/{id}/runs` now returns the runs of a schedule instead of 501: newest first, `limit` 1 to 100 (default 20, 400 on any other value), 404 for an unknown schedule, `[]` before the first run, and no `results` document in the list. `GET /api/v1/config/drift-scheduler` returns `{"enabled": bool}`, the `drift.scheduler_enabled` of the server that answers (admin key). Refs ginsys/shelly-manager#279.
