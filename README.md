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
- Export/import with multiple formats and scheduled operations
- Real-time metrics via WebSocket
- Web UI with configuration wizards and diff tools
- Multi-channel notifications (email, webhook, Slack)

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

See documentation:
- [Export/Import API](docs/API_EXPORT_IMPORT.md)
- [Notification API](docs/API_NOTIFICATION.md)
- [Metrics API](docs/METRICS_API.md)
- [SMA Format](docs/sma-format.md)

## Configuration

### Environment Variables

All settings can be overridden with `SHELLY_` prefix:

```bash
SHELLY_SERVER_PORT=9091
SHELLY_DATABASE_PROVIDER=postgresql
SHELLY_DATABASE_DSN="host=localhost user=app dbname=shelly sslmode=disable"
SHELLY_SECURITY_ADMIN_API_KEY=your-secure-admin-key
```

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
  admin_api_key: "${ADMIN_API_KEY}"
  use_proxy_headers: true
  trusted_proxies:
    - 10.0.0.0/8
  cors:
    allowed_origins:
      - https://app.example.com
```

**Provisioning Agent** (`/etc/shelly/provisioner.yaml`):
```yaml
api:
  url: http://shelly-api.default.svc.cluster.local:8080
  key: "${API_KEY}"

scanner:
  interval: 60s
  interface: wlan0
```

## Testing

```bash
make test           # Run all tests
make test-coverage  # Run with coverage
make test-ci        # Match CI exactly
```

## Development

**Prerequisites:**
- Go 1.23+ (managed via [mise](https://mise.jdx.dev/))
- SQLite3 development libraries
- GCC (for CGO)

```bash
git clone https://github.com/ginsys/shelly-manager
mise install
make deps
make build
```

See [AGENTS.md](AGENTS.md) for contributor guidelines and [tasks/README.md](tasks/README.md) for current work.

## Resources

- [Shelly API Documentation](https://shelly-api-docs.shelly.cloud/)
- [OPNSense API Reference](https://docs.opnsense.org/development/api.html)

## License

MIT License - See [LICENSE](LICENSE)
