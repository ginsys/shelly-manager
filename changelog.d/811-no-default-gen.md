Fixed
- A device added by hand keeps the generation it really has. Adding a device with no `gen` in its
  settings used to store Gen 1, so a Gen2 or Gen3 device added from the UI read as Gen1. The server
  now asks the device once at the address in the request and stores the generation it reports;
  a device that does not answer is stored as Gen 1, as before. Updating a device keeps its stored
  `gen` and never adds one. A Gen2 device added by hand before this fix keeps `gen: 1` until its
  settings are edited or it is discovered again. Fixes ginsys/shelly-manager#811.
