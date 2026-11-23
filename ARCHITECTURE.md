# Architecture

This file describes the code as it is on `main`. Each claim names the file it comes from.

## 1. What this is

Shelly Manager keeps track of Shelly smart-home devices (Gen1 and Gen2) and sets their configuration from one place. It has two programs: an API server that also serves the web UI, and a provisioning agent that runs on a host next to new devices.

## 2. Map

Programs:

- `cmd/shelly-manager`: the API server (the `server` command) and a few command-line commands such as `discover` and `add`. `main.go` builds the services and starts the server.
- `cmd/shelly-provisioner`: the agent. It registers with the server, polls it for tasks, scans for devices that are not on the network yet, and gives them Wi-Fi settings. It talks to the server only over HTTP.
- `cmd/shelly-test`: a small manual tool for one Gen1 device.

Packages in `internal/`:

- `shelly`: the device client and the factory that finds the generation of a device. `gen1` speaks HTTP (`/settings`, `/status`). `gen2` speaks JSON-RPC (`POST /rpc`). It does not know the database.
- `discovery`: finds devices by HTTP scan and mDNS. It finds and does not store.
- `service`: the business layer for devices. It stores discovered devices, builds and caches a client for each device, and keeps the stored login of a device working.
- `configuration`: the desired configuration of each device, the templates it is built from, apply, verify, drift detection, rollback and the drift scheduler.
- `database`: models, migrations and one provider for each database (sqlite, postgresql, mysql). `database/restore` swaps in a stored SQLite backup at the next start, never under a running server.
- `sync` and `plugins/sync/*`: export and import. The engine runs a plugin (`sma`, `json`, `yaml`, `gitops`, `opnsense`, `backup`). The `opnsense` plugin uses the OPNsense client in `internal/opnsense`.
- `api`: the HTTP routes (`router_routes.go`), handlers and security middleware. `metrics` and `notification` hold the metrics stream and the notification channels and rules.
- `redact`, `security`, `agentkey`: shared guards. `redact` is the one rule for which JSON keys are secrets, used by the routes and the exports. `security` compares API keys and reads secrets. `agentkey` defines the agent identity and uses the standard library only, so the agent does not link the database drivers.
- `netguard` and `testutil/dialguard`: the dial function that device clients use, and the test guard that replaces it. Production never replaces it.
- `config`, `logging`, `version`: settings, logs and the one version string.

`ui/` is the web UI, a Vue single-page app. It calls the server under `/api/v1` (`ui/src/api/client.ts`).

## 3. How data moves

Flow A. Discover devices and read their configuration

1. A user starts discovery in the UI, or runs `shelly-manager discover`. The handler or the command calls `service.DiscoverDevicesReport`.
2. `discovery.CombinedDiscoveryWith` scans the configured networks over HTTP, and over mDNS when `EnableMDNS` is on.
3. `service.storeDiscovered` writes each device to the database, or updates its row.
4. The handler calls `ImportDeviceConfig` for each device it found. That reads the live configuration and stores it. A request cannot turn this off.
5. The agent reports devices it found with `POST /api/v1/provisioner/discovered-devices`.

Flow B. Apply the desired configuration (Gen1 and Gen2)

1. A change to a template or to the overrides of a device calls `RecomputeDesiredConfig`. It merges the template layers, then the device overrides (`merge.go`), and stores the result with `SetDeviceDesiredConfigPending`. The device is now "pending".
2. `ConfigurationService.ApplyDesiredConfig` picks the Gen1 path or the Gen2 path (`applyPathFor`). It takes a lock for the device. In one server, the lock is in memory. With PostgreSQL or MySQL and more than one connection in the pool, it is also a database lock, so it holds across server copies. SQLite has one server and no database lock.
3. It refuses a setting that it cannot write, before it sends anything. It compares the MAC address of the device with the stored one. A different MAC means a different device, so nothing is written.
4. It writes the settings one section at a time, then reads the device back and compares.
5. It marks the device "applied" only when the read-back matches and the desired configuration did not change meanwhile.

Flow C. Drift detection

1. `cmd/shelly-manager/drift_scheduler.go` starts `configuration.Scheduler` when `drift.scheduler_enabled` is on.
2. The scheduler claims each due run with one database update. Only one server copy wins a claim, so a schedule does not run twice.
3. `BulkDetectDrift` calls `DetectDrift` for each device. That calls `VerifyDesiredConfig`, the same comparison as apply, and changes nothing on the device. A device with no desired configuration is an error, never "no drift".
4. When a device differs, a notifier (set up in `main.go`) sends a `drift_detected` event to `notification`.

Flow D. Import an SMA archive

1. `SyncEngine.Import` hands the request to the plugin it names, here `sma`. The plugin limits the size of the input (gzip or plain JSON) and parses it strictly.
2. It checks the format version, the structure, the record count and the checksum. Any failure refuses the whole archive.
3. It checks the identity of each record with `sync.ValidateImportIdentities`. A preview and a real import use the same check.
4. A dry run or a validate-only request stops here and writes nothing.
5. A real import writes the devices, templates and discovered devices in one database transaction. If it fails, nothing is written. Records that already exist stay as they are unless the request sets `force_overwrite`. Network, plugin and system settings in the archive are not restored, and the result says so.

Export runs the other way: `SyncEngine.Export` loads the data, masks secrets unless the request sets `exclude_sensitive` to false, and the plugin writes under the export root. The agent enrolment and notification delivery are not shown here.

## 4. Rules the code keeps

"Enforced by" names the test or CI job that fails when the rule breaks.

| Rule | Enforced by |
|---|---|
| Apply refuses a setting it cannot write, before it touches the device. | `TestApplyDesiredConfig_RefusesUnsupportedSettingsBeforeWriting`, `TestDeviceSchemaApplicableMatchesApply` |
| Apply writes nothing to a device whose MAC differs. | `TestApplyDesiredConfig_RefusesADeviceWithAnotherMAC` |
| One apply per device at a time in a server. Across server copies, only with PostgreSQL or MySQL and a pool of more than one connection. | `TestApplyLockProviders_*` in `internal/configuration/apply_lock_test.go` |
| A device is "applied" only when the read-back matches and nothing changed meanwhile. | `TestApplyDesiredConfig_CloudStaysPendingWhenDeviceKeepsTheOldFlag` (read-back), `TestApplyDesiredConfig_ConfigChangedDuringApplyStaysPending` (change meanwhile) |
| With an admin key set, a request with no key is refused on every `/api/v1` route except the open reads the test lists with a reason and CORS preflight (`OPTIONS`) requests, which the test skips. The agent key opens only the agent register and health routes. One key serves every guard. | `TestRouteTable_EveryRouteRequiresTheAdminKey` and `TestRouteTable_AgentKeyOpensOnlyTheAgentRoutes` in `internal/api/route_guards_test.go`, `TestAdminKey_OneKeyForEveryGuard` |
| Every call the UI makes has a registered route. | `TestRouteContract` in `internal/api/route_contract_test.go` |
| WebSocket message types are the same in Go and TypeScript. | `TestMessageTypeManifestParity` in `internal/metrics/contract_test.go` |
| A backup cannot be restored through the generic import routes. | `TestImportOfABackupIsRefusedOnBothRoutes` |
| A package's tests make no call that leaves the host through the default HTTP transport, the default resolver or `netguard`. Raw dials, multicast and database drivers are not covered. | `dialguard.Main` in the package's `TestMain`, and `TestEveryTestPackageCallsMain` in `internal/testutil/dialguard/dialguard_test.go`, which fails when a test package does not call it |
| TypeScript type errors never go up. | CI job "Frontend Unit Tests": `npm run typecheck:baseline` against `ui/typecheck-baseline.json` |
| `.go-version` is the one Go version. `go.mod`, `mise.toml` and the Dockerfiles match it at major and minor, and the workflows do not hardcode a version. A different `toolchain` line in `go.mod` only warns. | CI job "Validate Go Version": `scripts/check-go-version.sh` |
| Which package may import which. | By review only. The `depguard` linter is off in `.golangci.yml`. |

## 5. Boundaries

- HTTP API: `internal/api/router_routes.go`. The OpenAPI file is `docs/api/openapi.yaml`.
- Devices: Gen1 over HTTP (`internal/shelly/gen1/client.go`), Gen2 over JSON-RPC (`internal/shelly/gen2/client.go`).
- Agent and server: the client is `internal/provisioning/api_client.go`, the routes are under `/api/v1/provisioner/` (`internal/api/provisioner_handlers.go`), and the key format is `internal/agentkey`.
- Databases: `internal/database/provider/factory.go` chooses sqlite, postgresql or mysql.
- Export and import formats: the plugins in `internal/plugins/sync/`. The SMA archive is written by `sma/generation.go` and read by `sma/importer.go`.
- Metrics stream: `internal/metrics/websocket.go` and `ui/src/api/metricsMessages.ts`.
- Container images: `deploy/docker/Dockerfile.manager` (server and built UI) and `deploy/docker/Dockerfile.provisioner` (agent). Compose, Helm and Kubernetes files are under `deploy/`.

## 6. Where to start reading

1. `cmd/shelly-manager/main.go`: how the server is wired.
2. `internal/api/router_routes.go`: what the server offers.
3. `internal/configuration/apply_service.go`: the main flow, apply and verify.
4. `internal/shelly/factory.go`: how a device generation is chosen.
5. `ui/src/router/routes.ts`: which pages the UI shows.
