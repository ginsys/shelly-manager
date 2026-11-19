Fixed
- Drift detection now compares the live device with its desired configuration, the one apply
  writes and verify reads, instead of the stored import record. A device that was just applied no
  longer shows drift against an old import, and a device that left its desired configuration is
  no longer reported "in sync" because it still equals that import. A device with no desired
  configuration cannot be checked: the single check answers 404 and a bulk run marks it skipped
  with the reason, where both used to compare it with whatever the import record held. Fixes
  ginsys/shelly-manager#606.
