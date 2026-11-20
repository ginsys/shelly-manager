Added

- Drift schedules can now be created, updated and enabled or disabled through the API (`POST`, `PUT` and `POST .../toggle` on `/api/v1/config/drift-schedules`). A body that names a bad five-field `cron_spec`, an unknown device, an unknown filter key, or both a device list and a filter is a 400 and writes nothing. Deleting a schedule now answers 404 for a missing one and removes its device list and runs, and list and get return each schedule's `device_ids`. The run-history route still answers 501. Refs ginsys/shelly-manager#279.
