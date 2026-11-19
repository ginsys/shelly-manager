Added
- `POST /api/v1/config/bulk-download` downloads the desired configuration of up to 10 named devices as one JSON file, with every secret masked and an `error` entry for a device that cannot be read. It reads the database and contacts no device; the web UI does not offer it yet. Refs ginsys/shelly-manager#595.
