# SMA support in the frontend

The frontend parser and generator implement the strict Shelly Management
Archive `2026.1` contract. See
[`docs/guides/sma-format.md`](../docs/guides/sma-format.md) for the wire schema.

Key frontend rules:

- generated archives are always gzip;
- raw JSON is accepted only while parsing/importing;
- parser and generator share the local RFC 8785 canonicalizer;
- normalized input defaults to a 100 MiB limit;
- browser import preview rejects files/text over 7 MiB before base64 encoding;
- decoding is fatal UTF-8 and parsing is strict (duplicate keys, lone
  surrogates, unsafe numbers, invalid grammar, and depth over 64 are rejected);
- required collections may not be `null`;
- checksum and record-count validation cannot be disabled;
- browser import is preview-only and is offered only for a registered SMA
  plugin advertising the `sma` format.

`SMAGenerateOptions` contains only `compressionLevel`.
`SMAParseOptions` contains only `maxSizeBytes`, which limits normalized raw or
gzip data.

The registry-backed preview components use backend schema contracts and do not
persist plugin configuration in local storage.
