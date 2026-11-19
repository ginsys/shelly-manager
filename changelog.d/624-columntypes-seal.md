Security
- The start-up schema check no longer quotes the database server's text when it fails. Reading a
  table's columns before the migration could return a driver error (server address, identifiers)
  that the manager printed as it was; it now reports `database operation failed: <kind>` like the
  other connection errors. Fixes ginsys/shelly-manager#624.
