Fixed
- A SQLite database opened with a `file:` DSN can be backed up again: the snapshot directory and the directory the provider creates no longer start with `file:`, and the database size in the stats follows the file the DSN names, for a `file:` DSN and for any DSN with a `?` query. Fixes ginsys/shelly-manager#837.
