Fixed
- The normalized configuration of a stored Gen2 device now shows its time zone and position, which it
  left empty, and `raw.additional` lists only the top-level keys that nothing is read from (`ble`,
  `modbus`, scripts, `ws`) instead of a second copy of every section. A setting under a section the typed
  form reads in part, such as `sys.cfg_rev`, is no longer listed there and stays in the `raw` part of
  `GET /devices/{id}/config/typed`. Where a typed configuration holds both a top-level `location` and
  `system.location`, the top-level value wins for each field it sets. Fixes ginsys/shelly-manager#801.
