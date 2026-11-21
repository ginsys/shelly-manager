Fixed
- A device's generation is now read from its `gen` setting, then from the shape of its firmware, and is `0`
  when neither says, where it was guessed from the firmware alone: Gen2 firmware `1.x` was read as Gen1 and an
  empty firmware string as Gen2. The device page lists `ble` and `ethernet` for generation 2 and later only,
  so a device with no `gen` and Gen2 firmware `1.x` now lists both, and one with no `gen` and no recognisable
  firmware lists neither. The typed validator reads `gen` too. A `gen` of 0 or less is no value, so the firmware decides. Fixes ginsys/shelly-manager#798.
