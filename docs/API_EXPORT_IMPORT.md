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
- Get result: `GET /api/v1/export/{id}`
- Download: `GET /api/v1/export/{id}/download`
- History: `GET /api/v1/export/history` (pagination: `page`, `page_size`; filters: `plugin`, `success`)
- History item: `GET /api/v1/export/history/{export_id}`
- Statistics: `GET /api/v1/export/statistics`
- Backup export: `POST /api/v1/export/backup`
- Backup download: `GET /api/v1/export/backup/{id}/download`
- GitOps export: `POST /api/v1/export/gitops`
- GitOps download: `GET /api/v1/export/gitops/{id}/download`
- Schedules: CRUD + run under `/api/v1/export/schedules`

### Request schema (Generic Export)

```
POST /api/v1/export
{
  "plugin_name": "backup|gitops|<plugin>",
  "format": "sma|yaml|...",
  "config": { ... },
  "filters": { ... },
  "output": { "type": "file|webhook", "destination": "/path/to/file" },
  "options": { "dry_run": false, "validate_only": false }
}
```

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
    "duration": "123ms",
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
- Get result: `GET /api/v1/import/{id}`
- Backup restore: `POST /api/v1/import/backup`
- Backup validate: `POST /api/v1/import/backup/validate`
- GitOps import: `POST /api/v1/import/gitops`
- GitOps preview: `POST /api/v1/import/gitops/preview`
- History: `GET /api/v1/import/history` (pagination: `page`, `page_size`; filters: `plugin`, `success`)
- History item: `GET /api/v1/import/history/{import_id}`
- Statistics: `GET /api/v1/import/statistics`

### Request schema (Generic Import)

```
POST /api/v1/import
{
  "plugin_name": "backup|gitops|<plugin>",
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
    "duration": "250ms",
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
