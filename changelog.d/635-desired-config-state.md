Fixed

- Verify of a device with no desired configuration now answers 404 `NOT_FOUND`, as drift does, instead of reporting a match, and the configuration status route no longer words an old stored `drift` value as drift detected: `sync_status` is import and restore state only. Fixes ginsys/shelly-manager#635 and ginsys/shelly-manager#636.
