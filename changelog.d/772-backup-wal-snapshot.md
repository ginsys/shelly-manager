Fixed

- A SQLite backup is now a consistent snapshot of every committed write. It was a copy of the database file, which misses writes that still sit in the `-wal` file, so a restore could bring back an older database than the one that was backed up. Backups keep their file names and `.gz` and `.zip` handling. While a backup runs it needs free space for one full copy of the database in the directory of the database's DSN path (beside the live database file for a plain path), and it holds the database's one connection, so other queries wait for it. Fixes ginsys/shelly-manager#772.
