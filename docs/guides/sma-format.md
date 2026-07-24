# Shelly Management Archive (SMA) 2026.1

SMA is Shelly Manager's deterministic, integrity-protected exchange format.
`format_version: "2026.1"` is the only supported wire version. There is no
legacy-version migration or compatibility mode.

Generated `.sma` files are gzip-compressed JSON. Imports magic-detect gzip and
also accept raw JSON as a transport representation. In both cases the
normalized JSON is limited to 100 MiB. The HTTP API has its own 10 MiB request
limit, and the browser preview rejects source data over 7 MiB before base64
encoding.

## Root document

The root is closed and requires all of these non-null members:

```json
{
  "format_version": "2026.1",
  "metadata": {},
  "devices": [],
  "templates": [],
  "discovered_devices": [],
  "network_settings": {
    "wifi_networks": [],
    "ntp_servers": []
  },
  "plugin_configurations": [],
  "system_settings": {
    "log_level": "info",
    "api_settings": {},
    "database_settings": {}
  }
}
```

An archive must contain at least one device, template, or discovered device
after filters are applied. Discovered-only archives are valid. Required nil
slices and maps are generated as `[]` and `{}`; a `null` required collection is
invalid. Optional objects are omitted rather than serialized as `null`.

Every modeled object is closed. The only open maps are device `settings`,
device-configuration `config`, template `config` and `variables`,
plugin-configuration `config`, and system `api_settings` and
`database_settings`.

## Metadata and integrity

`metadata` contains:

- a lowercase UUID `export_id`;
- a canonical UTC `created_at`;
- non-empty `created_by`;
- `export_type`, either `manual` or `api`;
- required `system_info` and `integrity` objects.

API exports use `created_by: "api"` and `export_type: "api"`. Internal exports
default to `shelly-manager` and `manual`; browser generation defaults to
`shelly-manager-ui`. Authentication headers and keys never become archive
provenance.

System database providers are normalized to `sqlite`, `postgresql`, or
`mysql`; unknown providers are rejected. `total_size_bytes` and
`compression_ratio` are currently exactly zero.

Integrity is mandatory:

- `checksum` is `sha256:` plus 64 lowercase hexadecimal characters;
- `record_count` equals devices + templates + discovered devices;
- `file_count` is exactly `1`.

The checksum is SHA-256 of RFC 8785 canonical JSON with `checksum` temporarily
set to the empty string. It never covers gzip bytes. Numeric input must be
finite and interoperable as JavaScript binary64; integer-valued numbers must be
within the JavaScript safe-integer range.

## Time and structure rules

Timestamps are real UTC calendar instants matching
`YYYY-MM-DDTHH:mm:ss[.1-9 digits]Z`. Canonical strings are retained exactly.
Go values are normalized with UTC `RFC3339Nano`; JavaScript `Date` values use
`toISOString()`.

Import uses strict JSON parsing: valid UTF-8, no duplicate object names, no lone
surrogates, valid JSON number syntax, one root value, and maximum container
depth 64. Unknown fields are rejected.

Devices, templates, discovered devices, Wi-Fi entries, MQTT configuration,
plugin configuration, and system settings follow the fields exposed by the
`2026.1` schema. IDs, generations, priorities, and configuration references are
non-negative safe integers. Signal is a signed safe integer. MQTT port is
1–65535 and QoS is 0–2.

## Device configurations

Stored standalone device configurations are joined into their device before
preview and generation. Duplicate devices, duplicate or orphan
configurations, nested device-ID mismatches, and conflicting nested/standalone
copies are operational data failures. Equivalent nil and empty `config` maps
both normalize to `{}` and do not conflict. Configurations are not a top-level
archive field and are not counted separately.

## Generation and import

Generation materializes one validated generic tree, computes its integrity
value, serializes it, and always writes gzip. File exports use a same-directory
temporary file, close gzip successfully, sync and close the file, then rename
atomically. A failure removes the temporary file and never publishes a partial
destination.

SMA creation accepts `output_path`, `compression_level`,
`include_discovered`, and `exclude_sensitive` as plugin configuration. The
two boolean fields must be booleans and both default to `true` when omitted, so
server-side exports include discovered devices and redact sensitive keys even
when a caller does not materialize schema defaults. The
required network, plugin-configuration, and system sections currently use
their documented empty defaults; no inclusion toggles are advertised for data
sources that the engine does not yet provide.

Import normalizes raw/gzip input, completes strict lexical and depth checks,
validates the closed schema and counts, then verifies the RFC 8785 checksum.
Dry-run and validate-only imports return a preview. Applying an SMA archive is
not implemented yet and returns HTTP 501 (`ErrImportNotImplemented`) rather
than reporting a false success.

## API example

```json
{
  "plugin_name": "sma",
  "format": "sma",
  "source": {
    "type": "data",
    "data": "<base64 encoded .sma or JSON bytes>"
  },
  "config": {},
  "options": {
    "dry_run": true,
    "validate_only": true
  }
}
```

Send this body to `POST /api/v1/import/preview`. The browser exposes data-based
import preview only for a registered plugin named `sma` that advertises the
`sma` format.
