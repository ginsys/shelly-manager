# Sync Plugin Contract

Sync plugins are registered with `SyncEngine` and implement metadata,
configuration schema, validation, export, preview, import, lifecycle, and
capability methods. Export/import scheduling is not part of this contract.

## Metadata and formats

`PluginInfo.SupportedFormats` is authoritative. Engine operations validate in
this order:

1. look up `PluginName`;
2. require `Format` in `SupportedFormats`;
3. call `ValidateConfig`;
4. validate and normalize engine-owned paths;
5. load data and execute, when applicable.

`ValidateExport` performs stages 1–4 only. A plugin validator must be
side-effect free. File plugins repeat security validation during execution
before creating a directory or file.

The list endpoint represents supported formats as the `capabilities` string
array. The detail endpoint's `capabilities` value is the structured
`PluginCapabilities` object; these two response shapes intentionally differ.

## Configuration schema

Schemas contain:

- `version`;
- recursive `properties`;
- nullable `required`;
- optional `examples`.

Every property supplies `type` and `description`. Optional constraints are
`default`, `enum`, `pattern`, `minimum`, `maximum`, `items`, `properties`, and
`sensitive`. Supported types are string, number, boolean, array, and object.

The frontend adapter applies present defaults, including `false`, `0`, and the
empty string. Untouched optional fields are omitted; touched falsy fields are
retained. Arrays and objects are edited as validated JSON.

## Data and preview

An export plugin receives `ExportData`, `ExportConfig`, filters, output, and
generic options. Preview must inspect the same data and configuration as
generation and must not mutate stored data.

SMA additionally joins standalone device configurations into devices before
filtering, preview, validation, or generation. Its preview counts devices,
templates, and included discovered devices only. A post-filter empty SMA
archive returns `ErrInvalidExportData`.

## Stable errors

Plugins and the engine wrap stable sentinel errors with `%w`:

- `ErrInvalidPluginConfig` only for `ValidateConfig` failures;
- `ErrPluginNotFound`;
- `ErrUnsupportedFormat`;
- `ErrInvalidImportData`;
- `ErrInvalidExportData`;
- `ErrInvalidExportPath`;
- `ErrImportNotImplemented`.

Stored-data conflicts and execution failures must remain ordinary operational
errors so the API returns HTTP 500 rather than misclassifying them as client
configuration failures.

## Security and publication

Paths are constrained by the engine and, where applicable, by a plugin base
directory. Validation never creates the path. SMA uses atomic publication:
same-directory temporary file, gzip close, file sync, file close, then rename.
Every failure removes the temporary file.

Audit history uses explicit `X-User-ID` or `X-User` headers only, defaulting to
`api`. Plugins must not copy credentials into result data or archive metadata.

For the full SMA wire and checksum contract, see
[SMA 2026.1](../guides/sma-format.md).
