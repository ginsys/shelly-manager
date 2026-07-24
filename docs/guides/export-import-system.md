# Export and Import

Shelly Manager routes exports and imports through the runtime sync-plugin
registry. The frontend reads the same registry and plugin schemas rather than
maintaining a separate list of formats or mock previews.

## Export

`GET /api/v1/export/plugins` lists registered plugins. Its `capabilities` array
is the plugin's supported format list. The plugin detail endpoint exposes the
structured runtime capability object, while
`GET /api/v1/export/plugins/{name}/schema` returns the recursive configuration
schema.

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
SMA then applies its 100 MiB normalized-data limit. Applying an SMA import is
not implemented and returns HTTP 501; preview and validation do not persist.

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
