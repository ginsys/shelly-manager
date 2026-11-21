Fixed

- `docs/api/openapi.yaml` parses again: the description of the common-settings `400` answer held an unquoted `: ` that made the whole file invalid YAML for a viewer, a client generator or a lint. A test now parses the file, so the next break fails CI. Fixes ginsys/shelly-manager#816.
