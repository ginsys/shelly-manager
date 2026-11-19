Fixed
- Verify now compares a struct that has no exported field as one value. Before, it compared nothing
  inside it, so two different `power_metering.energy_reset` times were reported as a match; the same
  instant written in two time zones is still a match. A desired `energy_reset` against a read-back
  without it stays a `removed` difference, which is the true answer: no device reports a reset time and
  apply refuses one. Fixes ginsys/shelly-manager#567.
