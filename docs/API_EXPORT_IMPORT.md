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

## Import

- Generic import: `POST /api/v1/import`
- Preview import: `POST /api/v1/import/preview`
- Get result: `GET /api/v1/import/{id}`
- Backup restore: `POST /api/v1/import/backup`
- Backup validate: `POST /api/v1/import/backup/validate`
- GitOps import: `POST /api/v1/import/gitops`
- GitOps preview: `POST /api/v1/import/gitops/preview`

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

## Error responses

Common error codes include `VALIDATION_FAILED`, `INTERNAL_SERVER_ERROR`, and `NOT_FOUND`. Details are provided in `error.details` when safe.

