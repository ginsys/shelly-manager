Added

- `GET /api/v1/config/drift-schedules/{id}/runs` now returns the runs of a schedule instead of 501: newest first, `limit` 1 to 100 (default 20, 400 on any other value), 404 for an unknown schedule, `[]` before the first run, and no `results` document in the list. `GET /api/v1/config/drift-scheduler` returns `{"enabled": bool}`, the `drift.scheduler_enabled` of the server that answers (admin key). Refs ginsys/shelly-manager#279.

Fixed

- A drift schedule now carries a revision that every edit, toggle and disable bumps, and the scheduler claims a tick only at the revision it read. A tick read before a change of the device selection, or before a disable and enable, is no longer run from the old read. An edit costs the tick in flight one claim; the next poll takes it. Refs ginsys/shelly-manager#279.
