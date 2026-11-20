# Design note: plugin configuration, testing and enablement

Status: proposal. It decides nothing; the last section lists the decisions and one recommendation
for each. No code, OpenAPI or UI file changes with this note.

It answers the four questions of ginsys/shelly-manager#290, which was opened when
ginsys/shelly-manager#264 removed the plugin config, test and enable controls from the UI because
no route backed them. The related ginsys/shelly-manager#266 and ginsys/shelly-manager#253 are closed
and need nothing here.

All `file:line` references are at origin/main `8545e201c677`.

## 1. What the issue leaves out

Read ginsys/shelly-manager#290 with these corrections. Section 2 gives the detail.

1. **The three plugin routes are admin-guarded.** The issue lists them as read-only routes. They sit
   at `internal/api/sync_handlers.go:92-94` (the issue's line range has moved) and each is wrapped by
   `guard`, which calls the admin check (`:84-91`).
2. **No plugin sends a webhook.** The issue asks about "sink plugins such as a webhook".
   `OutputConfig.Webhook` exists (`internal/sync/plugin.go:134-152`), but the only code that reads it is
   `TemplatePlugin.handleOutput` (`internal/sync/template_plugin.go:328-338`), and `sendWebhook`
   returns "webhook sending not implemented" (`:357-360`). `TemplatePlugin` is built only by the
   external plugin loader (`internal/sync/loader.go:455`), and nothing calls
   `NewExternalPluginLoader`: the name occurs in `loader.go` and in no other Go file. The one plugin
   that talks to another system is OPNsense.
3. **A stored, masked, testable configuration already exists in the product, for notifications.**
   `NotificationChannel` has an `Enabled` flag and a `Config` (`internal/notification/models.go:9-18`).
   Its routes need the admin key, and one of them is `POST /notifications/channels/{id}/test`
   (`internal/api/router_routes.go:159-167`). So "no model behind them" is true of the sync plugins and
   false of the product. Section 2.6 says why it is still not a plugin config.
4. **The OPNsense schema example promises something the code does not do.** Its example uses
   `"api_key": "${OPNSENSE_API_KEY}"` (`internal/plugins/sync/opnsense/opnsense.go:143-144`). Nothing in
   the OPNsense package or in `internal/sync/engine.go` expands such a reference, so a client that
   copies the example sends the literal text as the key. This was a separate defect, fixed by
   ginsys/shelly-manager#736: the example now shows a plain placeholder and the field descriptions say
   the value is sent as written.
5. **`Preview` does not contact OPNsense; `Export` does.** The OPNsense `Preview` builds a text from
   local data and never creates the client (`opnsense.go:280-328`). `Export` creates the client and
   calls `TestConnection` before it writes anything (`:229-236`).

## 2. What exists today

### 2.1 The plugins

`SyncPlugin` (`internal/sync/plugin.go:11-34`) has `ConfigSchema()` (`:16`) and
`ValidateConfig(config)` (`:17`). It has no `Test()` and no stored configuration.

Six plugins are compiled in and registered at start-up (`cmd/shelly-manager/main.go:686-696`):
`backup`, `gitops`, `opnsense`, `sma`, `jsonexport`, `yamlexport`. There is no other way to add
one: the external loader has no caller (section 1, item 2).

Only OPNsense talks to another system. It is the only plugin whose schema marks fields `Sensitive`
(`api_key`, `api_secret`, `opnsense.go:75,80`). Its import is not implemented
(`opnsense.go:331-334`), and neither is the GitOps import (`internal/plugins/sync/gitops/gitops.go:385-388`).

### 2.2 How a plugin's configuration reaches a run

The configuration is part of each request: `ExportRequest.Config` (`internal/sync/data.go:104`),
`ImportRequest.Config` (`:121`), and the copies a plugin receives, `ExportConfig.Config`
(`internal/sync/plugin.go:113-120`) and `ImportConfig.Config` (`data.go:126-129`). The engine hands
`request.Config` to the plugin (`internal/sync/engine.go:386`, `:501`) after `ValidateConfig`
(`:574` for export, `:474` for import). The export handler validates, then runs
(`internal/api/sync_handlers.go:529-536`).

`ExportHistory` and `ImportHistory` have no config column (`internal/database/models.go:88-118`);
the export row copies only the config's `name` and `description` keys (`engine.go:632-651`). The
application config has no plugin key: a search of `internal/config` for "plugin" finds nothing, and the
same search for `ExportBaseDir` finds its two lines (`internal/config/config.go:154,156`).

### 2.3 Secrets

- **The schema flag.** `PropertySchema.Sensitive` (`plugin.go:109`). The UI shows such a field as a
  password input (`ui/src/components/shared/SchemaForm.vue:68`) and the schema viewer prints
  "Sensitive" (`ui/src/components/PluginSchemaViewer.vue:54`).
- **Nothing masks a plugin configuration today.** It is not stored, so there is nothing to mask. The
  masking that exists is for other data: device secrets inside an export (`engine.go:314`, `:410`,
  with the `exclude_sensitive` option), and notification channels (section 2.6).
- **A separate defect.** GitOps writes the request config, unmasked, into `export-summary.yaml` in its
  output directory (`internal/plugins/sync/gitops/gitops.go:298-302`). No schema marks a GitOps
  field `Sensitive`, so it is no argument for or against storing a config.

### 2.4 Authorization

One model: the admin key. `requireAdmin` returns true when no admin key is set and checks the key
otherwise (`internal/api/handlers.go:110-120`). The plugin routes, `POST /export` and
`POST /export/preview` call it (`sync_handlers.go:84-94`, `:515`, `:551`). So with no admin key set,
every one of these routes is open to any caller. That default is the same for every route and is not
this note's subject, but it matters in section 4: a stored credential sits behind it.

### 2.5 The UI

The plugin page is read-only: list, schema viewer, details. The card says the configure, test and
enable controls were removed (`ui/src/components/plugin/PluginCard.vue:40`). The list handler hardcodes
`Available`, `Configured` and `Enabled` to true (`sync_handlers.go:195`), and the UI knows it and does
not read them (`PluginCard.vue:15`, `ui/src/api/plugin.ts`, comment above `PluginStatus`). The OpenAPI
file names the three plugin operations and does not spell the list item, so it says nothing about
these fields (`docs/api/openapi.yaml:2520-2574`).

Nothing in `ui/src` mentions OPNsense (a search finds no file), and the export calls of the UI client
(`ui/src/api/export.ts:203,212,221,248,522`) post to the backup, JSON, YAML, SMA and GitOps routes,
so no UI path runs an OPNsense export.

### 2.6 The notification channels, and why they are not a plugin config

A channel is a named row with a type, an `Enabled` flag and a `Config` kept as JSON text
(`internal/notification/models.go:9-18`). Reads return the config with credentials replaced by the
placeholder `********` (`internal/notification/config_secrets.go:98`, `internal/redact/redact.go:13`),
and a save that sends the placeholder back keeps the stored value (`internal/notification/service.go:99-111`).
`TestChannel` sends a real test notification through the channel (`service.go:269-299`).

It is the nearest model, and a plugin config could copy its shape. It is not a plugin config because
nothing in a channel is read by `SyncEngine`: a channel is delivery for alerts, chosen by rules, and a
sync plugin's config is an input of one request. Two things carry over and two do not. The masking
walk (`internal/redact`) and the admin guard carry over. The credentials sit in the database as clear
text (the `Config` column has no encryption), and that is the part to decide on purpose and not copy by
accident (section 4).

## 3. Question 1: is a stored global plugin config desired?

**Option A: keep it per request (do nothing).** The UI stays read-only for configuration. The
operator can still export through the API with the config in each request. What the operator cannot
do: save the OPNsense host, key and secret, so each OPNsense export carries them again; and learn that
a credential is wrong before an export fails at
`failed to connect to OPNSense` (`opnsense.go:235`). No scheduled export exists to need a stored config
(the repo's project notes record the export scheduling as removed, `.claude/CLAUDE.md:121`). Cost: none.
Authorization and OpenAPI: unchanged.

**Option B: a stored singleton per plugin.** One row per plugin name holding the complete config.
Routes `GET` and `PUT /export/plugins/{name}/config`, admin-guarded (the routes the old UI called).
A request merges its own config over the stored one. First slice that ships alone, for OPNsense only:
my estimate, not a measurement, from copying the notification shape:

| Part | Non-test lines |
|---|---|
| model and migration | 40 |
| service with masking and placeholder restore (the `redact` walk is shared) | 90 |
| two handlers and routes | 90 |
| merge into the export, preview and import requests | 40 |
| OpenAPI: two paths and one schema | 80 |
| UI form on the existing `SchemaForm` | 120 |

About 460 lines plus roughly as many of tests. Authorization: any holder of the admin key may store a
config, and with no admin key set, anyone (section 2.4). OpenAPI: grows by the two paths.
It does not solve: two OPNsense hosts, or a second plugin with a different credential for each
use.

**Option C: named profiles** (the notification shape: id, name, plugin, config; create, list, update,
delete; a request names the profile). It solves two OPNsense hosts. With one plugin that needs
credentials it buys nothing over B, and it costs about twice as much, because it adds list and delete
and the request must name a profile.

**Recommendation: A.** One plugin needs saved credentials, and its import is not implemented
(`opnsense.go:331-334`). There is no webhook sink and no scheduled export. Build B when a second plugin
needs saved credentials or when scheduled exports return, and build it with the answers of section 4.
If the maintainer wants OPNsense credentials saved now, B is the smallest slice.
**This decision is the maintainer's:** it changes what an operator can do.

## 4. Question 2: if stored, scope, secrets and validation

**Scope.** Singleton (B) over profiles (C), for the reason in section 3.

**Secret handling.** This is the part that changes which secret reaches a stored row, so it is the
maintainer's to decide.

| Option | What it does | What it does not solve |
|---|---|---|
| (i) store the secret in the row, mask it on read, restore the placeholder on save | the notification-channel behaviour; about 0 new code beyond the table, because `internal/redact` already does the walk | the secret is clear text in the database, and the `backup` plugin writes a raw database snapshot (`docs/api/API_EXPORT_IMPORT.md:235`), which the export masking does not cover (`:68`). The OPNsense secret would travel inside every backup. |
| (ii) store only an environment reference such as `${OPNSENSE_API_SECRET}` and read the process environment when the plugin runs | the secret never reaches the database or a backup; the schema example showed this form before ginsys/shelly-manager#736 (section 1, item 4) | needs expansion code (none today, about 60 lines) and a rule for which names may be read: without one, whoever may store the config could point a field at any variable of the process, such as the database password, and send it to a host of their choosing. That is an admin-key holder, and anyone at all when no admin key is set (section 2.4). A fixed name prefix closes it in both cases. |
| (iii) store the non-secret fields only; send the secret with each request | a stored host and port | the request still carries the secret every time, so it solves little. |

**Validation surface.** `ValidateConfig` is per plugin and cannot check a partial config: OPNsense
requires `host`, `api_key` and `api_secret` together (`opnsense.go:158-170`). So a stored config must be
complete, and `PUT` validates the whole config with the plugin's own `ValidateConfig`, the same call an
export makes (`engine.go:574`). No new validator is needed.

**Recommendation, if Q1 is "stored":** singleton, (ii) with a fixed name prefix, and complete-config
validation on `PUT`. (i) is smaller but puts the secret into backups, which is a decision to make
openly and not by copying the channel code. **This decision is the maintainer's.**

## 5. Question 3: what does "test" mean for a plugin with no live connection?

Of the six plugins, five touch only local files or the database: a search of `internal/plugins/sync`
and `internal/sync` for `net/http`, `http.Client`, `http.NewRequest`, `exec.Command` and `net.Dial` finds
nothing, and the OPNsense client in `internal/opnsense/client.go` is where the network code is. For
those five, "test" can only mean a dry run.

**Option (a): test is a dry run.** It exists: `POST /export/preview` (`sync_handlers.go:550`). Cost:
none. It does not solve: for OPNsense, `Preview` cannot tell a wrong credential, because it never
contacts the host (section 1, item 5). This note makes no claim about what any plugin does with the
`DryRun` and `ValidateOnly` options of an export request (`internal/sync/plugin.go:154-160`): the
recommendation does not rest on it.

**Option (b): a connectivity check for plugins that have a connection.** An optional interface,
`ConnectionTester` with `TestConnection(ctx, config) error`, so `SyncPlugin` does not change and five
plugins need nothing. OPNsense implements it by calling its existing client
(`opnsense.go:367-388`, `internal/opnsense/client.go:103-114`). A route `POST
/export/plugins/{name}/test` takes the config in its body, so it works without Option B of section 3.
Cost: about 175 lines with tests (an estimate). Authorization: no new power, because `Export` already
makes the same call to the same host with the same credentials (`opnsense.go:229-236`); the route is
admin-guarded like the others. OpenAPI: one path.

**Option (c): neither.** The first export fails with the connection error (`opnsense.go:235`).

**Recommendation: (a), and write down that `Preview` does not check OPNsense credentials.** Build (b)
when operators ask; it is independent of storage. **This decision is the lead's:** (b) adds a route and
no new power.

## 6. Question 4: what do enable and disable mean?

Plugins are compiled in (section 2.1), so a registered plugin is always available and always enabled:
the handler's value for those two is accurate, if redundant. "Configured" is the one that says
something untrue, because it has no meaning without a stored config.

**Option (a): not applicable.** Stop returning `configured`, and the `error` and `last_used` the DTO
declares and never sets (`sync_handlers.go:164-170`); keep available and `enabled`. The UI does not
read any of them (section 2.5) and the OpenAPI file does not spell them. Cost: about 15 lines of Go and
a test, and the `PluginStatus` type in `ui/src/api/plugin.ts` (an estimate). It does not solve: an
operator who wants `backup` or `gitops` switched off.

**Option (b): an operator flag stored per plugin.** The engine refuses a request for a disabled plugin.
It needs a table, so it shares the cost of Option B in section 3, and it gives an admin a new power: to
switch off a plugin at run time through the API.

**Option (c): a start-up key**, for example `sync.disabled_plugins`, next to `ExportBaseDir`
(`internal/config/config.go:154`), read where the plugins are registered (`main.go:696`). Restart-only,
no table, no UI, no new API power. Cost: about 40 lines and a doc line.

**Recommendation: (a).** If someone needs to switch a plugin off, for example `backup`, which writes a raw
database snapshot, build (c). **This decision is the lead's:** it changes a response field no client reads.

## 7. What this note does not solve

- The `${OPNSENSE_API_KEY}` example (section 1, item 4) was a defect whatever was decided;
  ginsys/shelly-manager#736 removed it.
- No plugin sends a webhook, and the GitOps and OPNsense imports are not implemented. The note does not
  propose building them.
- A second registry exists (`cmd/shelly-manager/main.go:724`, `internal/plugins/sync/registry`) and is
  out of scope: the engine's `SyncPlugin` is what `main.go:686-696` registers and what every route
  reads.
- The size figures are estimates from the notification code's shape, not measurements.

## 8. Decisions needed

| Decision | Question | Recommendation | Whose |
|---|---|---|---|
| 1 | Is a stored plugin config desired? | No: keep it per request (A). Revisit when a second plugin needs saved credentials or scheduled exports return. | the maintainer |
| 2 | If stored: scope, secrets, validation | Singleton; environment reference with a fixed name prefix for secrets; validate the complete config on `PUT`. Only if decision 1 is "stored". | the maintainer (which secret reaches a stored row) |
| 3 | What does "test" mean? | A dry run; document that `Preview` does not check OPNsense credentials. Build the connectivity check (b) on request. | the lead |
| 4 | What do enable and disable mean? | Not applicable: stop returning `configured`, error and last_used; keep available and `enabled`. A start-up key (c) if a plugin must be switched off. | the lead |

ginsys/shelly-manager#290 stays open until decision 1 is recorded, and until what it decides is built.
