# Export and Import

Shelly Manager routes exports and imports through the runtime sync-plugin
registry. The frontend reads the same registry and plugin schemas rather than
maintaining a separate list of formats or mock previews.

## Export

`GET /api/v1/export/plugins` lists registered plugins. Its `capabilities` array
is the plugin's supported format list. The plugin detail endpoint exposes the
structured runtime capability object, while
`GET /api/v1/export/plugins/{name}/schema` returns the recursive configuration
schema. Each listed plugin has a `status` with `available` and `enabled`, both
`true`, because plugins are compiled in. Plugin configuration is not stored: it
travels with each export or import request.

Use `POST /api/v1/export/preview` to validate and preview any registered
plugin/format pair:

```json
{
  "plugin_name": "sma",
  "format": "sma",
  "config": {},
  "filters": {},
  "output": {"type": "response"},
  "options": {
    "dry_run": true,
    "validate_only": true,
    "include_metadata": true
  }
}
```

Validation order is stable: plugin lookup, supported format, plugin
configuration, then engine-owned path validation and normalization. Preview
loads actual database data and invokes the selected plugin.

Preview is how to test a plugin: there is no separate test route. For the
OPNsense plugin it does not contact the OPNsense host, so it does not check the
API key and secret. An export does, and fails with the connection error when
they are wrong.

File-producing plugins validate paths without creating directories during
configuration validation. Execution repeats path/security checks and creates
the destination only when needed. SMA publishes atomically through a
same-directory temporary file.

Generic result routes accept lowercase UUIDs only:

```text
GET /api/v1/export/{lowercase-uuid}
GET /api/v1/export/{lowercase-uuid}/download
```

Export scheduling is not available. The previous API and UI were removed
because stored definitions had no execution engine. Drift, notification, and
device/relay schedules are separate features and remain unchanged.

## Import preview

Use `POST /api/v1/import/preview`. The handler always forces dry-run and
validate-only behavior. Browser data import currently exposes only a registered
SMA plugin advertising format `sma`; it sends:

```json
{
  "plugin_name": "sma",
  "format": "sma",
  "source": {"type": "data", "data": "<base64>"},
  "config": {},
  "options": {"dry_run": true, "validate_only": true}
}
```

The browser accepts `.sma` or `.json` data up to 7 MiB before base64 encoding.
SMA then applies its 100 MiB normalized-data limit. An SMA import without a
preview option restores devices, templates and discovered devices in one
transaction. Preview and validation do not persist.

A backup is not imported through these routes: both routes answer `501` for the
backup plugin. `POST /api/v1/import/restore-preview` with `{"backup_id": "..."}` only
checks that the stored backup file is present and can be opened.

### Restoring a backup (SQLite)

A backup is restored in two steps, because the server never replaces its database
while it runs: services hold the database handle they took at start, and nothing holds
their writers back.

1. `POST /api/v1/import/restore` with `{"backup_id": "..."}` checks the stored backup and
   stages a copy. The server unpacks a `.sqlite.gz` or `.sqlite.zip` backup into
   `<database file>.restore/`, beside the database, checks the SQLite header, runs
   `PRAGMA integrity_check` and looks for the `devices` table. It answers `202` and changes
   nothing else. `"dry_run": true` runs the checks and stages nothing.
2. Restart the server (in Kubernetes, restart the pod; run one replica on the data volume).
   Before the database is opened, the start keeps one copy of the live file as
   `<database file>.pre-restore` (and its `-wal` as `.pre-restore-wal`), removes the stale
   `-wal` and `-shm` files, moves the staged file into place, then connects and migrates.

If the restored file cannot be opened or migrated, the start puts the `.pre-restore` copy
back (it is moved, so it is used up) and runs on it. Any failed apply keeps the old database and records the reason:
`GET /api/v1/import/restore` shows the staged restore and the result of the last start.
`DELETE /api/v1/import/restore` cancels a staged restore. To undo a restore by hand, stop the
server and copy `.pre-restore` (and `.pre-restore-wal`) back over the database file.

Space: staging needs room for the unpacked backup, and the apply needs room for one copy
of the live database and its `-wal`, all beside the database file. The check refuses a
backup that does not fit, with the numbers. The `.pre-restore` copy stays until the next
restore replaces it, or a rollback uses it. PostgreSQL and MySQL answer `501`: restore is for SQLite only.

Not detected: a backup made by a newer version. The schema has no version number; extra
columns in a newer backup are ignored and missing ones are added at start. A backup made
while the database had uncheckpointed writes can miss them, because the backup copies the
file as it is.

Not covered: a process killed in the middle of the start-up apply. A kill between two of its
steps can leave the previous database without its `-wal` in the `.pre-restore` copy, or leave
a restored file that cannot be migrated with no rollback at the next start. The staged file is
checked before this point, so the cases need a kill and a bad restore together. Restore the
copy by hand as described above.

## Errors

Stable engine error identities are classified with `errors.Is`:

- invalid plugin configuration, missing operational plugin, unsupported
  format, invalid import data, invalid export data, or invalid export path:
  HTTP 400;
- missing plugin detail/schema route: HTTP 404;
- unsupported import persistence: HTTP 501;
- stored-data inconsistency or execution failure: HTTP 500.

For SMA, an archive emptied by filtering is invalid export data (400).
Duplicate devices, orphan/duplicate/conflicting configurations, and nested
configuration ID mismatches are internal-data failures (500).

## History and audit identity

Successful and recorded failed operations are stored in export/import history.
The requester identity is the trimmed `X-User-ID` header, then trimmed
`X-User`, or `api`. Authorization headers, API keys, and remote addresses are
never used as audit identities.

Archive provenance is independent of this audit identity. API-created SMA
archives use `created_by: "api"` and `export_type: "api"`.

See [SMA 2026.1](sma-format.md) for its wire schema and integrity rules.
