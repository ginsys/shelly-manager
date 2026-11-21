Fixed
- Adding a device by hand stores the generation the device reports even when the client hangs up while
  the server asks it: the generation probe now has its own 2 s deadline instead of the request's. A
  device that reports a generation below 1 is stored as Gen 1, as one that does not answer is, and the
  log now says whether the device did not answer, is not a Shelly, or was given an address that is not
  a valid URL host. Fixes ginsys/shelly-manager#820.
