# Shelly Device Manager

[![CI Status](https://github.com/ginsys/shelly-manager/actions/workflows/test.yml/badge.svg?branch=main)](https://github.com/ginsys/shelly-manager/actions/workflows/test.yml)
[![Docker Publish](https://github.com/ginsys/shelly-manager/actions/workflows/docker-build.yml/badge.svg?branch=main)](https://github.com/ginsys/shelly-manager/actions/workflows/docker-build.yml)

A Golang application for managing Shelly smart home devices. **In development.**

## Architecture

- **Dual-binary design**: API server (containerized) + provisioning agent (host-based)
- **Multi-provider database**: SQLite, PostgreSQL, MySQL
- **Plugin-based export/import**: SMA, Terraform, Ansible, Kubernetes, Docker Compose, JSON, CSV
- **Template engine**: Sprig v3 with security controls and inheritance

## Features

- Device discovery and management (Gen1 & Gen2+ devices)
- Configuration templates with inheritance and validation
- Registry-backed export and preview across multiple formats
- Real-time metrics via WebSocket
- Web UI with configuration wizards and diff tools
- Multi-channel notifications (email, webhook, Slack)

## Export Formats

Shelly Manager supports multiple export formats for backing up and sharing device configurations:

### Database Backup (`.db`)
- **Use Case**: Full system backup including all data
- **Format**: SQLite database file
- **Compression**: GZIP or ZIP
- **Best For**: Disaster recovery, system migration

### JSON Export (`.json`)
- **Use Case**: Structured data export for processing
- **Format**: JSON with devices, templates, and metadata
- **Compression**: None, GZIP, or ZIP
- **Best For**: API integration, data analysis, automation
- **Example**:
  ```bash
  curl -X POST http://localhost:8080/api/v1/export/json \
    -H "Authorization: Bearer $API_KEY" \
    -d '{"compression_algo": "gzip"}'
  ```

### YAML Export (`.yaml`)
- **Use Case**: Human-readable configuration export
- **Format**: YAML with devices and templates
- **Compression**: None, GZIP, or ZIP
- **Best For**: GitOps workflows, manual review, documentation
- **Example**:
  ```bash
  curl -X POST http://localhost:8080/api/v1/export/yaml \
    -H "Authorization: Bearer $API_KEY" \
    -d '{"compression_algo": "none"}'
  ```

### SMA Archive (`.sma`)
- **Use Case**: Shelly Manager Archive format
- **Format**: Strict `2026.1` JSON schema with RFC 8785 SHA-256 integrity
- **Compression**: Always GZIP; import preview also accepts raw JSON
- **Limits**: 100 MiB normalized archive data, 10 MiB HTTP request body,
  7 MiB browser source before base64 encoding
- **Import**: The browser and `POST /api/v1/import/preview` show a dry-run
  preview and write nothing. `POST /api/v1/import` without a preview option
  restores devices, templates and discovered devices into the database in one
  transaction. A backup is not imported this way: `POST /api/v1/import/restore`
  stages a stored SQLite backup and the server applies it at its next start (see
  [export and import](docs/guides/export-import-system.md)), and
  `POST /api/v1/import/restore-preview` only checks that a stored backup file can be opened. See [SMA format](docs/guides/sma-format.md) for matching,
  `force_overwrite` and the limits

### Compression Options

| Algorithm | File Size | Speed | Use Case |
|-----------|-----------|-------|----------|
| `none` | Largest | Fastest | Small datasets, local storage |
| `gzip` | Medium | Medium | General purpose, good balance |
| `zip` | Medium | Medium | Windows compatibility |

**Note**: For hobbyist use, GZIP is recommended for most scenarios.

## Quick Start

```bash
# Build both binaries
make build

# Start server (auto-builds UI if needed)
make start

# Alternative: run API server only
make run

# Run UI dev server (hot reload)
make ui-dev
```

## CLI Commands

```bash
# Device management
./bin/shelly-manager list
./bin/shelly-manager discover 192.168.1.0/24
./bin/shelly-manager add 192.168.1.100 "Living Room Light"

# Export/import
./bin/shelly-manager export --format sma --output /backups/
./bin/shelly-manager import --format sma --file backup.sma --dry-run

# Start server
./bin/shelly-manager server --config /etc/shelly/config.yaml
```

## Deployment

### Kubernetes

```bash
make docker-build
kubectl apply -k deploy/kubernetes/kustomize/overlays/production/
```

See [deploy/kubernetes/README.md](deploy/kubernetes/README.md) for Kustomize, Helm, and other deployment options.

### Docker Compose

```bash
cd deploy/docker-compose
docker-compose up -d
```

## API

REST API with standardized `{success: true/false, data/error}` responses.

To write a device's desired configuration to a Gen1 device, or the system, MQTT, cloud, switch and input settings and the Wi-Fi station, access point and roam threshold, and the login of a Gen2 or Gen3 device, call `POST /api/v1/devices/{id}/config/new/apply`, with the admin key if the server has one configured. To compare without writing, call `POST /api/v1/devices/{id}/config/new/verify`. See [Apply and verify](docs/api/api-overview.md#apply-and-verify-the-desired-configuration) for the supported settings and errors.

See documentation:
- [API overview](docs/api/api-overview.md)
- [Export/Import API](docs/api/API_EXPORT_IMPORT.md)
- [Notification API](docs/API_NOTIFICATION.md)
- [Metrics API](docs/METRICS_API.md)
- [SMA Format](docs/guides/sma-format.md)

## Configuration

### Environment Variables

All settings can be overridden with `SHELLY_` prefix:

```bash
SHELLY_SERVER_PORT=9091
SHELLY_DATABASE_PROVIDER=postgresql
SHELLY_DATABASE_DSN="postgresql://app:secret@localhost:5432/shelly?sslmode=disable"
SHELLY_SECURITY_ADMIN_API_KEY=your-secure-admin-key
SHELLY_SECURITY_AGENT_API_KEY=a-different-key-for-the-provisioner-agent
```

When an admin key is set, every `/api/v1` route needs it (`Authorization: Bearer <key>` or `X-API-Key: <key>`),
including the device and notification routes, except `GET /api/v1/version` and `GET /api/v1/metrics/prometheus`.
The five provisioner agent routes also accept the scoped agent key (`SHELLY_SECURITY_AGENT_API_KEY`); each agent proves
its own identity with a token it receives when it registers.
With no key set the routes are open. The web UI asks for the key in a dialog the first time the server answers 401,
and keeps it for the browser tab (see `ui/README.md`); a build with `VITE_ADMIN_KEY` already holds one.

Provisioner agents can also enrol by key: an agent that sends an Ed25519 `public_key` waits as `pending` until the
admin key holder approves its fingerprint (`POST /api/v1/provisioner/agents/{id}/approve`, or pre-approve one with
`POST /api/v1/provisioner/enrolments`), then signs a challenge to receive its token. `security.agent_enrolment`
(`SHELLY_SECURITY_AGENT_ENROLMENT`) is `open` by default: agents that register without a key work as before. Set it to
`strict` to let in only approved agents: a register without a key is then refused with `403`, and the token of an agent
that is not approved stops working, so every existing keyless agent must be pre-approved and re-enrolled by key first.
`strict` needs an admin key and the manager refuses to start without one (see `docs/api/api-overview.md`, "Strict
enrolment").

Each secret also takes a `_FILE` form, for example `SHELLY_SECURITY_ADMIN_API_KEY_FILE=/run/secrets/admin_key`
(see `docs/security/SECURITY_SECRETS.md`). A `_FILE` that names a file the process cannot read stops the start-up
with an error that names the variable and the path, so a typo in the path cannot leave the routes open.

Drift schedules run only when the scheduler is on: `drift.scheduler_enabled` (`SHELLY_DRIFT_SCHEDULER_ENABLED=true`),
off by default because a run reads every device its schedule selects on a timer. A schedule's `cron_spec` has five
fields (minute, hour, day of month, month, day of week) and is read in the server's time zone; a row with a
six-field spec from an earlier release fails once and is disabled when the scheduler reaches it. With several
replicas on one database each schedule still runs once per due time. A run only reads devices, and its result is
kept in the run history (the newest 100 runs per schedule). On SIGINT or SIGTERM the run in flight stops between
devices; a device read already started is not interrupted, and a run left unfinished is marked `interrupted` by the
first poll after its recorded start is more than ten minutes old, and until then its schedule skips due times.

The PostgreSQL credentials can stay out of the DSN: set `SHELLY_DATABASE_USERNAME` and
`SHELLY_DATABASE_PASSWORD` (`database.username` and `database.password`) and give a DSN without them,
for example `postgresql://db.example:5432/shelly`. The app puts them into the URL with the escaping a
password needs. A DSN that carries credentials as well is refused, and so is a DSN query other than
`sslmode`, `connect_timeout`, `application_name`, `search_path`, `sslcert`, `sslkey`, `sslrootcert`,
`sslcertmode`, `sslsni`, `sslnegotiation`, `channel_binding`, `require_auth`, `target_session_attrs`,
`statement_cache_capacity` and `description_cache_capacity`: the keys that could carry a credential, a
host or a user (`user`, `password`, `passfile`, `service`, `servicefile`, `sslpassword`, `host`) stay refused.
The MySQL provider refuses the two settings, and SQLite ignores them.

### Database connection pool

Leave `database.max_open_conns` and `database.max_idle_conns` unset (or `0`) to get the provider's own pool:

| Provider | Open | Idle |
|---|---|---|
| SQLite | 1 | 1 |
| PostgreSQL, MySQL | 25 | 5 |

A value you set is used as is on PostgreSQL and MySQL; SQLite always runs with one connection, whatever is set. The environment names are `SHELLY_DATABASE_MAX_OPEN_CONNS` and
`SHELLY_DATABASE_MAX_IDLE_CONNS`. Each replica opens its own pool, so keep replicas × `max_open_conns`
below the database server's `max_connections` (PostgreSQL's default is 100), counting the extra replica of a
rolling update and the most the autoscaler can add. A pool of 1 on PostgreSQL or MySQL cannot take the
cross-replica apply lock; keep it at 2 or more when more than one replica runs.

### YAML Configuration

**API Server** (`/etc/shelly/config.yaml`):
```yaml
server:
  port: 8080
  host: 0.0.0.0

database:
  path: /var/lib/shelly/shelly.db

discovery:
  networks:
    - 192.168.1.0/24
  timeout: 5s

security:
  # No key here: set SHELLY_SECURITY_ADMIN_API_KEY and SHELLY_SECURITY_AGENT_API_KEY (see Environment Variables)
  use_proxy_headers: true
  trusted_proxies:
    - 10.0.0.0/8
  cors:
    allowed_origins:
      - https://app.example.com
  # The security monitor and IP blocking are off by default. To turn them on (blocking needs monitoring):
  # monitoring:
  #   enabled: true      # SHELLY_SECURITY_MONITORING_ENABLED
  # ip_blocking:
  #   enabled: true      # SHELLY_SECURITY_IP_BLOCKING_ENABLED; block_seconds and four thresholds also set here
```

The security monitor counts suspicious requests per client IP and serves `GET /api/v1/metrics/security`;
IP blocking refuses a flagged IP for `security.ip_blocking.block_seconds` and starts its counting over. Both
were always on before these keys existed, so a deployment that upgrades stops blocking IPs until it turns
them on. The keys and defaults are listed in `docs/security/API_SECURITY_FRAMEWORK.md`.

**Provisioning Agent** (`/etc/shelly/provisioner.yaml`):
```yaml
api:
  url: http://shelly-api.default.svc.cluster.local:8080
  # No key here: set SHELLY_API_KEY to the manager's agent key

scanner:
  interval: 60s
  interface: wlan0
```

`shelly-provisioner status` checks the host's Wi-Fi device and scanning; what its lines mean is in
[docs/guides/provisioner-status.md](docs/guides/provisioner-status.md).

A `${...}` written in a value of these files is not replaced, so set the keys through the environment
(`SHELLY_SECURITY_ADMIN_API_KEY`, `SHELLY_SECURITY_AGENT_API_KEY`, `SHELLY_API_KEY`, each also in the `_FILE` form).

## Testing

```bash
make test           # Run all tests
make test-coverage  # Run with coverage
make test-ci        # Match CI exactly
```

## Development

**Prerequisites:**
- Go 1.27+ (pinned in `.go-version`, managed via [mise](https://mise.jdx.dev/))
- Node 24 (pinned in `mise.toml`, installed by `mise install`; only the frontend checks and builds need it)
- SQLite3 development libraries
- GCC (for CGO)

`mise install` downloads Node as well as Go. The pre-commit hook runs its tools through `mise exec`, so your first
commit after the pin installs Node once; if that download fails the hook falls back to the tools on your `PATH` and
does not block the commit.

```bash
git clone https://github.com/ginsys/shelly-manager
mise install
make deps
make build
```

See [AGENTS.md](AGENTS.md) for contributor guidelines and [GitHub Issues](https://github.com/ginsys/shelly-manager/issues) for current work.

## Resources

- [Shelly API Documentation](https://shelly-api-docs.shelly.cloud/)
- [OPNSense API Reference](https://docs.opnsense.org/development/api.html)

## License

MIT License - See [LICENSE](LICENSE)
