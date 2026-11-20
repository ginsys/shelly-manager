# Export/Import API

This document describes the Export/Import endpoints, request/response schemas and examples. All responses use the standardized wrapper:

```
{
  "success": true|false,
  "data": { ... },
  "error": { "code": "...", "message": "...", "details": ... },
  "meta": { ... },
  "timestamp": "RFC3339",
  "request_id": "..."
}
```

## Export

- List plugins: `GET /api/v1/export/plugins`
- Plugin info: `GET /api/v1/export/plugins/{name}`
- Plugin schema: `GET /api/v1/export/plugins/{name}/schema`
- Generic export: `POST /api/v1/export`
- Preview export: `POST /api/v1/export/preview`
- Get result: `GET /api/v1/export/{id}` (and `GET /api/v1/export/backup/{id}`). The server keeps a
  result in memory, and after a restart the answer comes from the history record instead, with
  the same fields but no `checksum`, `warnings` or `metadata`, only the first error, and the
  time the record was saved as `created_at`. An id with no result and no record answers `404`.
- Download: `GET /api/v1/export/{id}/download`
- History: `GET /api/v1/export/history` (pagination: `page`, `page_size`; filters: `plugin`, `success`)
- History item: `GET /api/v1/export/history/{export_id}`
- Statistics: `GET /api/v1/export/statistics`
- Backup export: `POST /api/v1/export/backup`
- Backup download: `GET /api/v1/export/backup/{id}/download`
- GitOps export: `POST /api/v1/export/gitops`. `config.output_path` must be a new or empty directory, or one an earlier GitOps export wrote. Such a directory holds a `.shelly-manager-gitops` marker file, or the `export-summary.yaml` file that earlier versions wrote, as a regular file; an export replaces every file in it, so files you add there are removed by the next export. Any other non-empty directory answers `400` and is left untouched. Group and device type names become one directory name each: lower case, with `/`, `\`, spaces and a few other characters replaced. `config.include_common` and `config.include_templates` default to `true` when left out, in the export and in the preview; send `false` to leave the common file or the templates out. A value that is not a boolean answers `400`.
- GitOps download: `GET /api/v1/export/gitops/{id}/download` streams a zip of the export directory (`application/zip`, file name `<directory name>.zip`, files by their relative paths, built as it is sent). It zips only a directory that still holds the `.shelly-manager-gitops` marker file as a regular file, and answers `403` for any other; the marker is left out of the archive. A symlink inside the tree is skipped, never followed, and a path outside the export base directory answers `403`. Files someone added to the directory after the export are in the archive too.
- GitOps list, result and preview use the generic routes: `GET /api/v1/export/history?plugin=gitops`, `GET /api/v1/export/{id}` and `POST /api/v1/export/preview` with `plugin_name: "gitops"`. The history item's `file_size` is the total size of the exported files.
- GitOps delete: `DELETE /api/v1/export/gitops/{id}` removes the history record and answers `204`. It never removes the exported files. An id that is not a GitOps export answers `404`.
- Export scheduling is not available. The former scheduling routes were
  removed because no scheduler executed the stored definitions.

### Request schema (Generic Export)

```
POST /api/v1/export
{
  "plugin_name": "backup|gitops|<plugin>",
  "format": "sma|yaml|sqlite|...",   // the backup plugin takes "sqlite" only
  "config": { ... },
  "filters": { ... },
  "output": { "type": "file|webhook", "destination": "/path/to/file" },
  "options": { "dry_run": false, "validate_only": false }
}
```

### Saved secrets in exports

The generic export, its preview and the GitOps export replace every saved
secret with the marker `[REDACTED]`: the values under a secret-named key (a
name that contains `pass`, `pwd`, `psk`, `secret` or `token`, or ends in `key`)
in the device settings, the device configurations, the templates and the
template variables. This holds for the JSON, YAML, GitOps and SMA plugins. A
secret under a key that holds a list or an object is replaced throughout. An
empty value stays empty, except in the SMA export, whose own wider key rule
replaces a matching field whatever it holds. Set `"exclude_sensitive": false`
in `config` to ask for a full export with the secrets in clear.

Only the SMA import reads the marker, as "keep the stored value". The JSON,
YAML and GitOps plugins have no import, so a file they wrote cannot be
restored.

The `backup` plugin is not covered: it writes a database snapshot, which holds
every secret by design, so protect that file as you protect the database. The
manager has no route that writes a stored import/export record to a device: the
only device write is the apply, `POST /api/v1/devices/{id}/config/new/apply`,
which sends the desired configuration.

### Preview export

```
POST /api/v1/export/preview
{
  ... same body as export ...
}

200 OK
{
  "success": true,
  "data": {
    "preview": {
      "success": true,
      "record_count": 123,
      "estimated_size": 45678,
      "warnings": []
    },
    "summary": {
      "record_count": 123,
      "estimated_size": 45678
    }
  },
  "timestamp": "...",
  "request_id": "..."
}
```

### Result structure (Export)

`duration` is a number of nanoseconds, in the import result as well.

```
{
  "success": true,
  "data": {
    "export_id": "exp_...",
    "plugin_name": "...",
    "format": "...",
    "output_path": "/abs/path/file.ext",
    "record_count": 100,
    "file_size": 4096,
    "checksum": "sha256:...",
    "duration": 123000000,
    "warnings": []
  },
  "timestamp": "..."
}
```

### Query parameters (history)

- `page` (int): 1-based page. Invalid or `<=0` defaults to `1`. Non-integer values default to `1`.
- `page_size` (int): page size. Values `>100` default to `20`. `0` or omitted returns a single page with all items for endpoints that support it.
- `plugin` (string): case-sensitive plugin name filter. Unknown names return an empty list.
- `success` (bool): accepts `true/false`, `1/0`, `yes/no` (case-insensitive). Invalid values are treated as no filter (or may be interpreted as false depending on endpoint implementation; see tests for current behavior).

## Import

- Generic import: `POST /api/v1/import`
- Preview import: `POST /api/v1/import/preview`
- Get result: `GET /api/v1/import/{id}`. After a restart the answer comes from the history record,
  with the same fields but no `changes`, `warnings` or `metadata`, only the first error, and the
  time the record was saved as `created_at`. An id with no result and no record answers `404`.
- Check a stored backup: `POST /api/v1/import/restore-preview` with `{"backup_id": "<export id>"}`.
  It takes no path: the file is the one the export history records for that id, and it must
  be a regular file named like a backup inside the export directory. The answer says whether
  the file is present and can be opened, with its size and record count. It checks nothing
  of the file's contents and restores nothing. An empty file, or one the server cannot open,
  answers `200` with `valid: false` and a general message in `errors` (never a path). An
  unknown id, or an id of another plugin, answers `404`; a failed export, or a file that is
  missing, outside the export directory or not a regular backup file, answers `422`; a body
  with any other field answers `400`.
- Restore a stored SQLite backup: `POST /api/v1/import/restore` with
  `{"backup_id": "<export id>", "dry_run": false}`. Like the check above it takes no path, and
  `dry_run` is read from this body field only. The server unpacks a `.sqlite.gz` or `.sqlite.zip`
  backup into a staging directory beside the database, checks the SQLite header, runs
  `PRAGMA integrity_check` and looks for the `devices` table, then answers `202` "applies at the
  next start". It never replaces the live database while the server runs; the operator restarts
  the server (in Kubernetes, restart the pod). A dry run runs the same checks, stages nothing
  and answers `200`. A backup that fails a check, or that needs more free space than the
  database's directory has, answers `422` with the reason (never a path); a body with any other
  field answers `400`. Staging a second backup replaces the first.
  `GET /api/v1/import/restore` answers `{pending, last_result}`: the staged backup, and what
  the last start did with one (`applied`, or `failed` with the reason). `DELETE
  /api/v1/import/restore` cancels a staged restore (`204`, or `404` when none is staged).
  With PostgreSQL or MySQL all three answer `501`. `POST /api/v1/import` and `/import/preview`
  still refuse every source for the backup plugin with `501`.
- GitOps import: `POST /api/v1/import/gitops`
- GitOps preview: `POST /api/v1/import/gitops/preview`
- History: `GET /api/v1/import/history` (pagination: `page`, `page_size`; filters: `plugin`, `success`)
- History item: `GET /api/v1/import/history/{import_id}`
- Statistics: `GET /api/v1/import/statistics`

### Request schema (Generic Import)

```
POST /api/v1/import
{
  "plugin_name": "gitops|<plugin>",   // "backup" is refused with 501: see "Check a stored backup"
  "format": "sma|yaml|...",
  "source": { "type": "file|url|data", "path": "/path" },
  "config": { ... },
  "options": {
    "dry_run": false,
    "validate_only": false,
    "force_overwrite": false,
    "backup_before": false
  }
}
```

### Preview import

The API enforces dry run + validate-only for preview routes.

```
POST /api/v1/import/preview
{
  ... same body as import ...
}

200 OK
{
  "success": true,
  "data": {
    "preview": {
      "success": true,
      "import_id": "imp_...",
      "records_imported": 0,
      "records_skipped": 0,
      "changes": [ { "type": "create|update|delete", "resource": "device|config|template", ... } ],
      "warnings": []
    },
    "summary": {
      "will_create": 2,
      "will_update": 3,
      "will_delete": 0
    }
  },
  "timestamp": "..."
}
```

### Result structure (Import)

```
{
  "success": true,
  "data": {
    "success": true,
    "import_id": "imp_...",
    "plugin_name": "...",
    "format": "...",
    "records_imported": 10,
    "records_skipped": 1,
    "duration": 250000000,
    "changes": [ ... ],
    "warnings": []
  }
}
```

### Query parameters (history)

Same semantics as Export history: `page`, `page_size`, `plugin` (case-sensitive), and `success` value parsing.

## Error responses

Common error codes include `VALIDATION_FAILED`, `INTERNAL_SERVER_ERROR`, and `NOT_FOUND`. Details are provided in `error.details` when safe.

## Security and RBAC

- Admin-only endpoints: All export/import operations, downloads, previews, results, schedules, history and statistics require admin authorization when an admin API key is configured.
- Provide either header `Authorization: Bearer <ADMIN_KEY>` or `X-API-Key: <ADMIN_KEY>`.
- If no admin key is configured, endpoints remain open (development mode). Configure `security.admin_api_key` to enable protection.

## Safe Downloads

- To prevent path traversal and accidental exposure, downloads can be restricted to a base directory configured via `export.output_directory`.
- When set, any requested `output_path` must resolve under this directory; otherwise the API returns `403 FORBIDDEN`.
### Backup export (Provider snapshot)

The Backup plugin takes a database snapshot via the active DB provider.

- SQLite behavior:
  - `config.compression = false` → output file `*.sqlite`
  - `config.compression = true`  → output file `*.sqlite.gz`
  - No `.tar.gz` container; single-file outputs only.
  - The snapshot is one consistent copy of every committed write, including writes that are
    still in the `-wal` file (SQLite `VACUUM INTO`).
  - While it runs, the backup needs free space for one full, uncompressed copy of the database in
    a temporary `.backup-*` directory beside the output file, even when compression is on. The
    directory is removed afterwards. A crash can leave it behind; delete it by hand.
  - The database has one connection, so every other query waits until the snapshot is written.
    For a small database that is seconds.
- Backup directory: a backup is written only inside one directory. That is `sync.export_base_dir` when it is set, and `./data/backups` (relative to the working directory) when it is not. `config.output_path` is optional; empty means that directory. A relative `output_path` joins under `sync.export_base_dir` when it is set, and is relative to the working directory when it is not. An `output_path` that is not inside the backup directory answers `400` with the directory in the reason, and nothing is created. The check reads the path text only: a symbolic link inside the backup directory is followed, so only the host's own administrator should create one. The result's `output_path` is an absolute path. To keep backups elsewhere, set `sync.export_base_dir`; the download and restore routes check `export.output_directory`, so set both to the same directory.

Request example (SQLite):

```
POST /api/v1/export/backup
{
  "name": "Pre-maintenance snapshot",
  "description": "DB snapshot before changes",
  "format": "sqlite",
  "config": {
    "output_path": "./data/backups",
    "compression": true
  }
}
```

Note: JSON/YAML/SMA live under “Content Exports” and use the generic export endpoints. The Backup endpoint is for provider-level snapshots only.
