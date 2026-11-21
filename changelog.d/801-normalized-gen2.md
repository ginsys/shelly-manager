Fixed
- The normalized configuration of a stored Gen2 device now shows its time zone and position, which it
  left empty, and `raw.additional` lists only the keys that nothing is read from (`ble`, `modbus`,
  scripts, `ws`) instead of a second copy of every section the typed form holds. Fixes ginsys/shelly-manager#801.
