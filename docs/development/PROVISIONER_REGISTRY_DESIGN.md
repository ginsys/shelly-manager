# Provisioner registry in the database, and agent enrolment

Status: proposal for ginsys/shelly-manager#638. Slice 1 is built: the tables and the store
(merged as ginsys/shelly-manager#724), the handlers that use them, the admin delete route and the
hourly purge of old rows. Slice 2, the manager half of Option F, is built: the columns and the keyless refusals
(ginsys/shelly-manager#845, ginsys/shelly-manager#851), the purge by state (ginsys/shelly-manager#854), the admin routes
(ginsys/shelly-manager#857), the keyed register (ginsys/shelly-manager#862) and the `strict` mode. Slice 3, the agent
half of Option F (the seed, the key, the fingerprint and the two-step register, in `internal/agentkey` and the
provisioner agent), is built. Slice 4, the UI, is built: the agents page
shows each agent's enrolment state and fingerprint and approves, rejects and pre-approves. Serge chose the enrolment design on
2026-11-21 (14:13 CET, typed "F"): **Option F, fingerprint approval of a seeded agent key**. Options
A to E stay below as the options that were weighed. Sections below that describe the registry
"today" describe the code before slice 1.
Read at origin/main 147080796e83 (cites are `file:line` at that commit), except the Option F,
slice, upgrade and decision text, whose cites are `file:line` read at origin/main 82b3985, after
slice 1. Every cited file is the same at 6343b21c3fda, the base this note sits on, for the first
set of cites.

## Summary

The provisioner registry holds agents, tasks and per-agent tokens. It is one
in-memory global in each manager process. That causes four limits. Three are in
the issue. One is not.

1. **A manager restart loses all of it.** Agents, tokens and tasks.
2. **The agent key alone can register a new agent id** and claim every task that
   has no agent.
3. **The agent key alone can take over an id** that has been silent for 5 minutes.
4. **Each replica has its own registry.** The production overlay runs 3 to 10
   replicas, so an agent's token is known only to the replica it registered with.

This note proposes four slices. Each ships alone, in this order:

| Slice | What it does | Fixes | Changes who may enrol an agent? |
|---|---|---|---|
| 1 | The whole registry moves to the database: agents, hashed tokens, tasks. | 1, 4, and the empty-registry window | No new right. It closes a window (see "The empty-registry window") and opens a smaller one: a request that began before a token was replaced may finish after it, for one request (see "The store", detail 2, and decision 11). Slice 1 took the row lock of decision 11 (`internal/database/provisioner_store.go:43-48`), so that window is closed; the text stays as the alternative that was rejected. |
| 2 | The manager: enrolment states, the challenge, the approve, reject and pre-approve routes, the `open` and `strict` modes, the purge rules. | 2 and 3, for every agent that has a key; for every agent once `strict` is on | **Yes, in `strict`:** only an operator-approved key gets a token and the agent key alone creates no id. In `open` (the default) the keyless path stays as today (two refusals are new: a keyless id that starts with `keyed-` in any case, and a new keyless id that is not printable ASCII without a space unless it starts with `provisioner-`; an existing row keeps its id), and a keyed id needs an approval. |
| 3 | The agent: a seed, the fingerprint, the signed register and an id taken from the key. | The agent half of 2 and 3 | No new right. An agent with no seed behaves as today. |
| 4 | The UI: a pending list with approve, reject and pre-approve. | Nothing new | No. |

The decisions that need an owner are at the end ("Decisions needed").

## The registry today

### Where things live

- `registry` is a package-level variable: a mutex and two maps, `agents` and
  `tasks` (`internal/api/provisioner_handlers.go:54-64`).
- A `ProvisionerAgent` holds its token in an unexported field, so it is never
  serialised (`provisioner_handlers.go:16-33`, `token` at `:32`). A
  `ProvisioningTask` holds its type, device MAC, target SSID, `Config`, status,
  agent id and priority (`:39-50`).
- A token is 32 random bytes in hex (`newAgentToken`, `:68-74`). The manager
  finds the agent for a token by scanning every agent with a constant-time
  compare (`agentOf`, `:83-91`; `apikey.Equal`, `internal/security/apikey/apikey.go:34`).
- Two keys exist. The admin key opens every route. The agent key
  (`Handler.AgentAPIKey`, `internal/api/handlers.go:45`) opens only the five
  routes the agent calls: register, poll, task status, discovered devices and
  health (`requireAgentOrAdmin`, `handlers.go:122`; routes at
  `internal/api/router_routes.go:241-249`). With no admin key configured, these
  routes are open and no token is checked.
- `authorizeAgent` (`provisioner_handlers.go:98`) decides who may act for an
  agent: the admin key (or no admin key) may act for any agent; anyone else
  needs the agent's own token. No known token is 401. A token of another agent
  is 403.

### Register, takeover and claim

- `RegisterAgent` (`:115`) looks the id up (`:149`). If the id exists, is online
  (seen within `agentOfflineAfter` = 5 minutes, `:35-37`), and the caller holds
  neither the admin key nor that agent's current token, the answer is
  409 "Agent id in use" (`:153-157`). In every other case the register succeeds
  and **replaces the token** (`:193`).
- So an id is free to take in two cases: it has been silent for 5 minutes
  (limit 3), or the manager has no record of it at all (limit 1 and 4, below).
- `PollTasks` (`:226`) gives the caller every task whose status is `pending`
  and whose agent is empty or the caller, and marks it `assigned` to the caller
  (`:254-259`). A task with no agent is claimable by any registered agent. That
  is by design, and the UI creates tasks without an agent (`createTaskLocked`
  sets no `AgentID`: `internal/api/provisioning_ui_handlers.go:227-235`, used
  by `CreateProvisioningTaskUI` at `:215` and `BulkProvisionUI` at `:307`).
  `CreateProvisioningTask` stores the task in memory (`:337`, store at `:376`).
- **A second set of handlers uses the same `registry`.** The admin UI routes
  under `/api/v1/provisioning/` (`internal/api/router_routes.go:227-233`) list,
  read, create, cancel and bulk-create tasks, and list and read agents
  (`provisioning_ui_handlers.go:142-361`). Each checks the admin key itself. The
  store of slice 1 serves them too; see "The store".
- The discovered devices an agent reports are already in the database
  (`database.DiscoveredDevice`, `internal/database/models.go:62`). They carry
  the agent id as a plain string, with no foreign key.

### What the agent does

- The agent id is `provisioner-<hostname>-<pid>` (`generateAgentID`,
  `cmd/shelly-provisioner/main.go:403`), computed at each start. On a host the
  pid changes on each start, so a restarted agent has a new id. In the shipped
  image the agent is the container's first process (exec-form `CMD`,
  `deploy/docker/Dockerfile.provisioner:60`), so the pid is 1. The shipped
  Kubernetes agent is a DaemonSet with `hostNetwork: true`
  (`deploy/kubernetes/kustomize/base/deployment.yaml:128`, `:149`), so the
  hostname is the node's. A restarted container there keeps its id.
- The token is kept in memory only (`internal/provisioning/api_client.go:25`).
  A restarted agent has no token.
- The agent polls every `provisioning.provision_interval` seconds
  (`main.go:130-133`). The code falls back to 30 seconds only when that value is
  zero or less. The configuration default is **600 seconds**
  (`internal/config/config.go:276`), and the shipped Kubernetes agent sets 600
  (the DaemonSet,
  `deploy/kubernetes/kustomize/base/deployment.yaml:183-184`). Only the shipped
  agent config file sets 30 (`configs/shelly-provisioner.yaml:12`). Before each
  poll the agent calls `EnsureRegistered` (`main.go:447`; `api_client.go:171`).
  On a 401 or 404 from a poll it drops its registration, so the next poll
  registers again (`dropRegistration`, `api_client.go:180`, called at `:208`).
- **A 600-second agent is silent for longer than the 5-minute rule between two
  polls.** At that interval the manager lists it as offline for about half of
  each cycle, and the takeover rule (`:153`) allows a register of its id by any
  agent-key holder for about half of each cycle. So for the shipped Kubernetes
  agent, limit 3 is open for about half of the time, not only after an outage.
  Only a poll, a register or a discovery report moves `last_seen`
  (`provisioner_handlers.go:191`, `:249`, `:491`), so a discovery report in the
  middle of the cycle shortens the open part.
- The agent sends the agent key as `Authorization: Bearer` and the token as
  `X-Agent-Token` (`api_client.go`, `makeRequest`).

### What each restart costs

| Event | What the agent sees | Wait |
|---|---|---|
| Manager restarts, one replica | Its next poll gets 401 (the registry is empty), it registers again at its next poll. No 409, because the manager has no record. Since slice 1 a restart keeps the registry, so this row now describes an agent the manager lost (a restored backup), which `TestProvisionerClient_RegistersAgainWhenTheManagerForgetsIt` proves. | One poll interval (30 s in the shipped agent config, 600 s in the shipped Kubernetes agent). Tasks in memory are gone. |
| Agent restarts and keeps its id (container), one replica | It has no token. It registers with the agent key only. If the manager's record was seen less than 5 minutes ago, the answer is 409. It retries at each poll. | The rest of the 5 minutes counted from its last poll before the restart, plus up to one poll interval for the retry. With a 600-second interval the record is often already more than 5 minutes old, and then there is no wait at all. Tasks assigned to the old process stay `assigned` and nobody polls them again. |
| Agent restarts with a new id (host) | It registers a new id. No wait. The old row stays as an offline agent. | None. |
| Agent holds the admin key | `holdsAdminKey` is true, so the 409 never applies (`:153`) and tokens are not checked. | None. This is the pre-token behaviour, and the reason the docs say to send the agent key and never the admin key (`docs/security/SECURITY_SECRETS.md:15`). |

`docs/api/api-overview.md:433` says "a manager restart drops them, and a
restarted agent waits until its old record is 5 minutes silent". Read together,
the two clauses suggest a manager restart causes the wait. It does not: the
wait follows an **agent** restart. Slice 1 corrected the sentence: it now says
that a restarted agent waits, and that the registry itself survives a manager
restart.

### Limit 4: one registry per replica

`deploy/kubernetes/kustomize/overlays/production/hpa.yaml:13-14` sets 3 to 10
replicas, `replica-patch.yaml:6` sets 3, and staging sets 2
(`overlays/staging/replica-patch.yaml:6`). The kustomize tree sets no session
affinity on the Service or the Ingress. The kustomize README names PostgreSQL
for staging and production (`deploy/kubernetes/kustomize/README.md:321-324`).

Each replica has its own `registry`. With more than one replica:

- A token is valid only on the replica that issued it. A poll that lands on
  another replica gets 401 or 404, the agent drops its registration and
  registers again there. The agent churns, and a discovery result can be lost:
  a discovery task claimed on replica A and reported to replica B is refused
  with 401, the client drops its registration
  (`api_client.go:311-321`), and the agent logs the failed report and goes on
  without retrying it (`cmd/shelly-provisioner/main.go:655-672`). The
  discovered devices of that task are discarded, and the task status update
  cannot run while the agent is unregistered.
- A task created on replica A is visible and claimable only by polls that land
  on A. A status report that lands on replica B is refused: 401 when B does not
  know the agent's token, 403 when it does (an unknown task is nobody's task,
  `provisioner_handlers.go:303-310`). An admin-key caller gets 404.
- The wait in the table above shows only when the register request lands on the
  replica that holds the old record. On any other replica there is no record
  and the register succeeds at once.

### The empty-registry window

`RegisterAgent` refuses an id only if the id **exists** (`:149`, `:153`). On a
manager that has just restarted, or on a replica that never saw the id, the
registry is empty. Any holder of the agent key can register any id at once and
receive that id's tasks. This is the takeover of limit 3 with no 5-minute
silence needed. It is the strongest reason to persist the agents.

## Storage

### What goes in the database

Both tables are needed in slice 1.

- **Agents, with the hash of the token.** This fixes limit 1 for agents, and it
  makes a token valid on every replica.
- **Tasks.** Agents without tasks do not ship alone under replicas: a task
  created on replica A would still be claimable only on A, and a status report
  on B would still find no task. One registry split across two stores is worse
  than either.

Not stored: the plain token (never), and a computed `status`. Today
`GetProvisionerAgents` rewrites `Status` to `offline` from `LastSeen`
(`provisioner_handlers.go:206-215`). With a database, the status is computed
from `last_seen` when read, not stored.

### Table shapes

The models go in `internal/database`, next to the others, and join the list in
`Manager` (`internal/database/manager.go:68-83`). Names follow the existing
models: a `uint` primary key, and a separate unique string column for the
public id (as `ExportHistory.ExportID` does).

`provisioner_agents` (model `database.ProvisionerAgent`):

| Column | Type | Notes |
|---|---|---|
| `id` | uint, primary key | |
| `agent_id` | string, `size:191`, unique index, no-pad binary comparison | The id the agent chooses. |
| `hostname`, `ip`, `version` | string | Not indexed. |
| `capabilities`, `metadata` | text, size to decide | JSON documents. No column default; seeded by `BeforeSave`. See "Column types". |
| `token_hash` | string, `size:64`, unique index | Hex SHA-256 of the token. |
| `token_issued_at`, `registered_at`, `last_seen` | time; `last_seen` indexed | |

`provisioning_tasks` (model `database.ProvisioningTask`):

| Column | Type | Notes |
|---|---|---|
| `id` | uint, primary key | |
| `task_id` | string, `size:191`, unique index | Today `task_<unixnano>`, made in `CreateProvisioningTask` (`provisioner_handlers.go:358`) and in `createTaskLocked` (`provisioning_ui_handlers.go:222`). The API keeps its string id. |
| `type`, `status` | string, `size:191`; `status` indexed | |
| `device_mac`, `target_ssid` | string | |
| `config` | text, size to decide | JSON. See "Column types" for the size, and "Risk". |
| `agent_id` | string, `size:191`, index | No foreign key, as `DiscoveredDevice.AgentID`. |
| `priority` | int | |
| `created_at`, `updated_at` | time | |

### Column types on the three providers

- `internal/database/models.go:9-13` records the first rule every model here
  follows: an indexed string needs `size:191`, because MySQL cannot index an
  unbounded text type. `models.go:29-32` and `:45-47` record the second: a JSON
  document column is `type:text` with **no column default**, because MySQL
  rejects defaults on TEXT, and a `BeforeSave` hook seeds the empty value.
- With those rules, every column above maps on SQLite, PostgreSQL and MySQL:
  strings become `varchar(191)` or `text`, times become the provider's
  timestamp, ints and bools are plain. **No provider lacks a column type this
  design needs**, so there is no stop point here.
- **A column must hold every value the API accepts today, or the build slice
  says it adds a limit.** The handlers check very little. `RegisterAgent` checks
  only that `id` and `hostname` are not empty (`provisioner_handlers.go:116-133`)
  and `CreateProvisioningTask` checks only the task type (`:338-354`). The
  request limit is 10 MiB (the default, `internal/api/middleware/security.go:80`,
  applied at `:473-481` and built in `cmd/shelly-manager/main.go:366`). Two kinds
  of column follow from that:
  - **Free text the client supplies**: task `config`, and agent `capabilities`
    and `metadata`. `TEXT` on MySQL holds about 64 KiB, so a plain `type:text`
    column rejects values the API accepts today (a 70 KiB `metadata`, for
    example). The build slice sizes these for the request limit (`MEDIUMTEXT` or
    `LONGTEXT` on MySQL; `text` on the other two providers already holds it) and
    proves it with a provider test that stores and reads back a value near the
    limit. The other unindexed strings (`hostname`, `ip`, `version`,
    `device_mac`, `target_ssid`) use the GORM default for a string, which is an
    unbounded text type on all three providers. This note recommends this over
    adding limits.
  - **Indexed strings** (`agent_id`, `task_id`, `token_hash`, and the task
    `status`, `type` and `agent_id`): `size:191` is a real limit on MySQL and,
    through `varchar(191)`, on PostgreSQL (the pinned GORM PostgreSQL driver,
    `go.mod:26`, maps a sized string to `varchar(<size>)`: `postgres.go:254-257`
    in `gorm.io/driver/postgres` v1.6.3; SQLite does not enforce a length). These are
    **new restrictions** on what today's handlers accept. The build slice
    validates them on every provider (a 400 with a clear message for an id, task
    type or status longer than 191 characters) and lists them in the API doc
    change. A value that long is not expected, but the restriction must be
    stated, not left to a database error.
- **Identity and status comparisons must be byte-exact on every provider.**
  Today the handlers compare ids and statuses as Go strings
  (`provisioner_handlers.go:255`, the map key at `:149`, and the status test
  against exactly `"pending"` at `:255`). So `alpha`, `Alpha` and `alpha ` (a
  trailing space, which the handlers accept: `:130`) are three different
  agents, and a task whose status was set to `"PENDING"` through
  `UpdateTaskStatus` (`:317`) is not claimable. SQLite compares strings
  byte for byte by default, and so does PostgreSQL for `varchar` and `text`.
  **MySQL does not**: its default collation compares case-insensitively (the
  repository already documents that hazard, `migrations.go:365-370`), and even
  the binary collation `utf8mb4_bin` pads with spaces, so `alpha` equals
  `alpha `. The build slice therefore gives `agent_id` (in both tables),
  `task_id`, `status` and `token_hash` a **no-pad binary** comparison on MySQL:
  the collation `utf8mb4_0900_bin` (MySQL 8.0.17 and later, which CI runs) or a
  binary string type. The provider test proves, on every provider, that
  `alpha`, `Alpha` and `alpha ` are three agents (and the unique index accepts
  all three), that a task owned by one is not claimed by another, and that a
  `"PENDING"` task is not claimed. The `token_hash` is lowercase hex that the
  manager writes itself, so it needs the collation only for strictness.
  **As built, the requirement is met only where the collation exists.** The start-up step
  (`migrations.go:56-82`) converts the tables on MySQL 8.0.17 and later (the collation does not
  exist before that). On MariaDB, MySQL 5.7 and MySQL 8.0.0
  to 8.0.16 it logs a warning and leaves the folding collation, so there the byte-exact guarantee
  above is **not met** and the binary string type is not built. Option F does not depend on it: see
  "How an id is compared, per database" under "The agent id".
- The hash column is exactly 64 hex characters, so `size:64` is a hard fit and
  needs no 191 workaround.

### The migration

There are no numbered migration files. The schema is GORM `AutoMigrate` over
one model list (`manager.go:68-83`). `runPreMigrationFixups`
(`internal/database/migrations.go:43`, called at `manager.go:60`) exists only
for one case: adding a NOT NULL column to a table that already has rows, which
SQLite refuses and PostgreSQL rejects (`migrations.go:17-37`).

So for slice 1:

- Two new tables, no existing rows: **no fixup is needed**. Adding the two
  models to the list is the whole migration.
- Later slices add columns to a table that then holds rows (slice 2 adds
  enrolment state). Those columns **must be nullable**, or the build slice must
  add a fixup that adds the column nullable, backfills it, and then tightens it,
  as the existing `config_templates` fixup does (`migrations.go:161-178`,
  `:335-375`). A Go-side default is not enough: a `BeforeSave` hook runs on a
  row operation and never touches the rows that already exist when `AutoMigrate`
  runs. This is the one trap to name in the slice 2 brief.

Tests the build slice adds:

- A model test on SQLite, as the other model tests do.
- A provider test. CI runs PostgreSQL 16 and MySQL 8 in the job "Database
  Provider Migration" (`.github/workflows/test.yml:140-200`), but only the test
  names listed in its `-run` patterns. Outside that job, the provider tests
  skip unless `POSTGRES_TEST_HOST` or `MYSQL_TEST_HOST` is set
  (`internal/database/migrations_provider_test.go:245`, `:293`). **A new
  provider test must be added to that `-run` list**, or it never runs on the two
  providers that matter.

### The store, and what it costs

The registry becomes a store behind an interface on `Handler`. The handler
keeps the authentication rules (`authorizeAgent` stays where it is). The store
holds the rows. Rough methods: register or replace an agent, find an agent by
token hash, touch `last_seen`, list and get agents, create, list and get tasks,
claim tasks for an agent, update a task's status, cancel a task, delete an
agent. The admin UI handlers (`provisioning_ui_handlers.go:142-361`) use the
same store. The cancel today sets the status to `failed` and writes
`config["_error"] = "canceled"` on a task that is `pending`, `assigned` or
`in_progress` (`:269-276`). That edits the JSON in `config`, so in the store it
is a read and a write of the row in one transaction, with the status test in
the write.

Three details matter:

1. **The register is one conditional write, and keeps the admin path.** The 409
   rule and the token replace must be atomic, because two replicas can register
   one id at the same moment. Today the replace is allowed in three cases: the
   caller holds the current token, the id has been silent for more than 5
   minutes, or the caller holds the admin key (or no admin key is configured:
   `holdsAdminKey`, `provisioner_handlers.go:77-80`, rule at `:153`). The store
   operation takes the third case as an explicit "privileged" input from the
   handler, so "same controls" stays true. For the first two, update the row
   `WHERE agent_id = ? AND (token_hash = ? OR last_seen < now - 5 min)` and read
   the rows affected. Zero rows and an existing row is the 409. A first register
   inserts, and the unique index on `agent_id` makes a concurrent second insert
   fail; the loser retries as an update. `clause.OnConflict` works on all three
   providers. `last_seen` and "now" come from the manager processes, so a clock
   difference between replicas moves the 5-minute rule by that difference, which
   is small next to 5 minutes on hosts that keep their clocks in sync.
2. **The claim is one conditional update per task, with the owner check.**
   `PollTasks` today tests both the owner and the status (`:254-255`), and the
   status of a task can be set back to `pending` by `UpdateTaskStatus`, which
   accepts any status string (`:317`). So the update keeps both tests. The
   in-process mutex no longer serialises replicas. Select the candidate ids,
   then `UPDATE provisioning_tasks SET status = 'assigned', agent_id = ? WHERE
   id = ? AND status = 'pending' AND (agent_id = '' OR agent_id = ?)` for each,
   and keep only the rows where the rows affected is 1. A single-row
   conditional update is atomic on all three providers. Without the owner test,
   a stale candidate list could take a task that another agent claimed and an
   update then set back to `pending`. **The token check goes into the same
   statement.** Today one mutex covers the token check and the assignment
   (`PollTasks` takes `registry.mu` at `:235` and `authorizeAgent` runs under it).
   With replicas, a handler-side check followed by a task-only update leaves a
   window in which another replica replaces the token, and the old request still
   claims tasks and receives their `config`. So the claim and the task status
   update add `AND EXISTS (SELECT 1 FROM provisioner_agents WHERE agent_id = ?
   AND token_hash = ?)` (skipped for a caller that holds the admin key), and a
   token replaced a moment earlier updates zero rows. This **narrows the window
   to one statement and does not close it**. On PostgreSQL the `EXISTS` reads a
   statement snapshot, so a replacement that commits while the update runs can
   still be missed. The discovered-devices report inserts into a table that has
   no such link, so a request that passed the check just before a replacement
   can still finish. So for the claim, the status update and the report, the
   proposal changes today's behaviour: a request that began before a token was
   replaced may finish after it, for the length of one request. Closing that
   needs a row lock on the agent row shared by the register and the claim
   (`SELECT ... FOR UPDATE`, which SQLite does not have, but SQLite has one
   writer at a time). Decision 11 asks for that choice.
   The number of tasks per poll is small. A side effect: the database can order
   the result by `priority` and `created_at`, where the map walk today is in
   random order.
3. **`last_seen` is written on every poll.** One agent polling every 30 seconds
   is 2,880 writes a day, and every 600 seconds is 144. For a few agents this is
   negligible, and the note does not propose a write throttle.

The cost: the tests that today reset or fill `registry` directly move to the
store. Five test files in `internal/api` use `registry.`
(`provisioner_client_e2e_test.go:23`, `provisioner_handlers_test.go`,
`provisioning_ui_handlers_test.go`, `provisioner_discovered_devices_test.go`,
`provisioner_identity_test.go`). That is a test change in the files that
already touch the global, and its size is for the build plan to count.

A SQLite database file is for one manager process, so a SQLite deployment has
one registry, as today. That is a deployment requirement and not a guarantee:
the development overlay sets one replica, but it inherits the base rolling
update (`maxSurge: 1`, `maxUnavailable: 0`;
`deploy/kubernetes/kustomize/base/deployment.yaml:9-14`,
`overlays/development/kustomization.yaml:36-38`), so two manager pods overlap
during an update. Replicas need PostgreSQL or MySQL, which the production and
staging overlays already name.

## The slices, and the first one

### Slice 1: the registry in the database (ships alone)

Same API. Same controls. Same enrolment (the shared agent key). The agent binary
is unchanged. What changes for an operator:

- A manager restart no longer drops agents or tokens. The agent's next poll
  works. There is no re-register, no 409 wait and no lost task.
- Every replica knows every token and every task. Limit 4 is closed.
- The empty-registry window is closed for every id the manager has seen.

**All three hold only while the database is durable.** That is a separate
requirement from the slice. The shipped base overlay keeps SQLite on a volume
(`SHELLY_DATABASE_PATH: "/data/shelly.db"`,
`deploy/kubernetes/kustomize/base/configmap.yaml:20`; the `data` volume is a
PersistentVolumeClaim, `base/deployment.yaml:119-121`). The shipped development
overlay overrides the path to `/tmp/shelly.db`
(`overlays/development/kustomization.yaml:25`), and `/tmp` is an `emptyDir`
(`base/deployment.yaml:122-123`). A development pod that is replaced loses the
registry, and the empty-registry window is open again, as it is today. The
build slice states the durable database as a requirement in its docs and leaves
the development overlay as it is.

Why it ships alone: no route, no header and no field the agent sees changes.
An old agent works with a new manager and the other way round.

What it does **not** fix: limit 2 (a new id by the agent key) and limit 3 (an
id after 5 minutes of silence). It makes both permanent in one sense: the rows
now outlive a restart, so a dead agent stays on the list (see "Stale rows").

If the current registry could not be left as it is for a slice that ships
alone, this note would stop and say so. It can be left as it is: slice 1
replaces the storage behind the handler and leaves the handler's rules intact.

### Slice 2: the manager (Option F)

Read "Option F" for the design. This slice builds the manager half, in `internal/database` and
`internal/api`:

- the enrolment columns and states, and a fixup that backfills every existing row as `legacy`;
- the register that answers `pending`, `challenge` or a token, and the signature check;
- the keyless register's own three rules: the `keyed-` prefix refused in any case, a 409 for a
  row that is not `legacy`, for every actor, before the silence test (see "The register protocol",
  item 6), and the ASCII rule for a new id, run in the store's created branch (see "The agent
  id");
- the admin routes to approve, reject and pre-approve, and the enrolment state and fingerprint in
  the admin agent list;
- the manager setting `security.agent_enrolment` with `open` (default) and `strict`, and the check
  in `agentByToken` that strict adds;
- the purge rules for the new states (see "Stale rows");
- tests on SQLite, and a provider test on PostgreSQL and MySQL added to the `-run` list of the
  provider job (see "The migration"), and the two doc updates of decision 10. The proof for the
  keyless rules is three tests. A store-level test: a keyless register of a keyed row's id is
  refused for a silent row and for the admin key, on every provider. A second store-level test: a
  new id with a space or a non-ASCII byte is a 400 and leaves no row, an id that starts with
  `provisioner-` is accepted whatever follows, and an existing row with a non-ASCII id registers
  as before. A handler-level test: a keyless id of `KEYED-<fingerprint>` or `Keyed-…` is a 400 before
  the store is reached. CI runs no case-folding database (MySQL 8 there compares with the binary
  collation), so the folded lookup itself is not exercised; the prefix test and the state check
  close the takeover and the ASCII rule closes the squat without it, and the note says so here.

Size: about 200 lines of code plus tests (the lead's estimate, not measured). It ships alone. No
agent sends a key until slice 3, so every agent is keyless and `open` behaves as today, except
that a keyless id that starts with `keyed-` in any case is refused, and a new keyless id must be
printable ASCII without a space unless it starts with `provisioner-` (see "The agent id").

### Slice 3: the agent

Read "Option F" for the design. This slice builds the agent half, in `cmd/shelly-provisioner` and
`internal/provisioning`: the seed setting, the `seed` and `fingerprint` commands, the key and the
fingerprint, the two-step register with its pending and rejected states, and the agent id taken from
the key. It uses the standard library (`crypto/ed25519`). Size: about 80 lines of code plus tests
(the lead's estimate, not measured).

It ships after slice 2. Against a manager without slice 2, a register that carries a `public_key`
is read as a keyless register: the handler decodes the body without refusing unknown fields
(`internal/api/provisioner_handlers.go:96-108`), so the manager answers with a token and does no
enrolment. An agent in `strict` refuses a manager that answers a keyed register with a token. An
agent in `open` carries on as today, under its new id.

### Slice 4: the UI

The admin agents page (`ui/src/pages/ProvisioningAgentsPage.vue:17-35`) lists the fields of
`uiProvisioningAgent` (`internal/api/provisioning_ui_handlers.go:38`, filled by `toUIAgent` at
`:112`), which carries no state and no fingerprint. This slice adds the state and the fingerprint,
the approve, reject and pre-approve actions that call the routes of slice 2, and the list of
pending agents that the operator compares with the fingerprint in the agent's log. Until it ships
the operator uses the routes of slice 2 directly.

Built. `uiProvisioningAgent` gains `enrolmentState` and `fingerprint` (the readable form, absent for a
keyless row), and the page reads the same list as before. The manager stores the state name in an agent's
`status` until an enrolled agent registers (a pending row has `pending`, a just approved one `approved`), so
the page's Activity column shows `online`, `busy` or `offline` only and the summary counts keyless and
approved agents only. Reject and approving a rejected row ask
first. The page does not show the manager's `security.agent_enrolment` mode: no route returns it.

## Enrolment

The question: who may create an agent id, and how does an agent prove it is the
one that was created. Today, anyone with the agent key.

All options need an admin key. With no admin key configured, the provisioner
routes are open and no token is checked, so no enrolment can be enforced. A
manager set to require enrolment and having no admin key must refuse to start.

Options A to E are the options that were weighed, kept as they were written. In the texts of B and
D, "slice 2" means the build of B and "slice 3" means the build of D, as the note first planned
them. **Option F, which Serge chose, replaces both**: see "Option F" below and "The slices" above.

### Option A: the shared key only (today, plus slice 1)

- Protects: a token binds an id to the agent that registered it. Other agents
  cannot act for it.
- Does not solve: limits 2 and 3. Any agent-key holder creates ids.
- Operator and agent: nothing to do.

### Option B: a one-time enrolment secret per agent

The admin creates an enrolment: `POST` to a new admin-key route, with an
optional agent id and an expiry. The response shows a secret **once**. The
manager stores only its hash, an expiry and a used flag. The agent is started
with the secret. Its first register presents it (new header or body field), the
manager marks it used and issues the agent token. Later registers of the same id
need the current token.

An enrolment may name an id that already exists. Redeeming it **replaces that
id's token**. This is the admin-authorised reset: it is the route for an agent
that lost its token or its rotation response, and it needs the admin key at
creation, so it is no weaker than the admin delete followed by a new
enrolment.

- Protects: limit 2. A new id needs an enrolment from the admin, so a caller
  with only the agent key cannot create an id. **Until slice 3 this protects
  new ids only.** The 5-minute silence rule stays in slice 2
  (`provisioner_handlers.go:153-157`), so an agent-key holder can still take an
  enrolled id once it has been silent for 5 minutes, and then claim tasks.
  Only after slice 3 (option D) is "only enrolled agents can claim" true. Slice
  2 does not remove the silence rule, so it changes no control beyond the
  enrolment of new ids.
- Operator: one secret per agent, passed into the agent's secret store.
- Agent: **must keep its id and its token across restarts.** A used secret
  cannot be used again, so without a kept token each agent restart needs a new
  secret. The token belongs to one id. A host agent computes a new id on each
  start, because the pid is in it (`generateAgentID`, `main.go:403`), so a kept
  token alone would belong to an id the restarted agent no longer uses. So the
  agent writes **the id and the token together** to a file (mode 0600) at a
  configured path, and uses the stored id when the file exists. (A stable id set
  in the agent's configuration is the other way. The recommendation is the
  file.) The agent image already has `USER shellyuser` and `VOLUME ["/data"]`
  (`deploy/docker/Dockerfile.provisioner:47`, `:50`), so a file under `/data`
  survives a container restart when `/data` is a volume.
- Cost on a restart: with a kept id and token, none. Manager or replica
  restart: none (slice 1). A container without a persistent volume, or a host
  agent that loses its file, costs one new secret per restart.
- Does not solve: an agent host that is already enrolled and compromised. A leaked
  agent token still acts as that agent.

### Option C: admin-approved registration

An agent registers with the agent key. The manager creates a **pending** row and
returns no token, but a **claim secret** (shown once, only its hash is stored).
The agent keeps polling its register, presenting the claim secret, until an
admin approves the id (admin-key route, or later a UI button). Then the next
register that presents the claim secret returns the token. Without the claim
secret, any agent-key holder who knows the id could win that request after the
approval and collect the token meant for the original agent. So the claim
secret is part of C, not an extra.

- Protects: limit 2 for the claim. A pending agent claims nothing, and only the
  applicant that holds the claim secret collects the token.
- Operator: approve each new id. No secret to hand out.
- Agent: handle a "pending" answer, keep the claim secret in memory while it
  waits, and retry. (It already retries a failed register at each poll.) A
  restart during the wait loses the claim secret, so the agent registers again
  and the admin approves again.
- Cost on a restart: an agent that lost its token registers again with an id
  that is already approved. It needs a rule, and the rule is the same as for
  B: an existing id needs its token, so the admin must reset it.
- Does not solve: the approval is weak. The id, hostname and IP are all chosen
  or reported by the registering caller, so the admin cannot tell a real agent
  from an attacker unless the admin already knows the id to expect. A busy
  admin may approve the wrong one. B has no such gap, because the secret is
  given out by the admin.

### Option D: an id is bound to its token (smaller)

Remove the 5-minute silence rule. An existing id needs its token. Only the admin
key frees an id: a new `DELETE` route, plus the cleanup in "Stale rows".

- Protects: limit 3, and the empty-registry window for ids the manager knows.
- Does not solve: limit 2. A new id is still free to anyone with the agent key.
- Cost on a restart: an agent that lost its token (container without a volume)
  can never register that id again until an admin deletes the row or creates an
  enrolment that names it (Option B). So D is safe only **with B**, or with an
  agent that keeps its id and token.

### Option E: unassigned tasks are not claimable (smaller)

A task must name an agent, or the claim is limited to agents an admin marked.

- Protects: the claim half of limit 2, with no enrolment at all.
- Does not solve: an id created by the agent key, which still receives its own
  assigned tasks if an operator names it.
- Cost: it changes the UI default, because the UI creates tasks without an
  agent today. The UI would have to pick an agent. That is a UI change, which
  this note does not propose.

### Option F: fingerprint approval of a seeded agent key (chosen)

Serge, typed 2026-11-21 12:15 CET (as the lead recorded his words): an agent generates or is
preconfigured with a code, reports it, the server shows it, and the operator accepts or rejects
it; "once approved, agents keep working with their code and remain stateless"; the "operator
doesnt have to juggle codes, at most preconfigure one per agent". He chose this option on
2026-11-21 at 14:13 CET.

Each agent has an Ed25519 key pair that it rebuilds from a configured seed at every start. It
prints a short fingerprint of the public key in its log. The manager lists unknown keys as
pending, with their fingerprints. The operator compares a fingerprint with the one in the agent's
log and approves it, or pre-approves a fingerprint before the agent first starts. From then on
the agent proves that it holds the key by signing a challenge at each start, and receives a
session token that it keeps in memory. Nothing is written to disk, and no token or code is handed
out by the operator.

| Serge's requirement | Where F meets it |
|---|---|
| The agent reports a code, the server shows it, the operator accepts or rejects it | "The fingerprint", "The register protocol" (pending), "What the operator does" |
| Once approved, agents keep working with their code and stay stateless | "The seed" (nothing on disk), "The challenge" (a restart needs no operator step), "Cost on a restart" |
| The operator does not juggle codes, at most one preconfigured per agent | "The seed" (one value per agent, or none), "What the operator does" (compare once, or pre-approve) |

**The seed.**

- 32 random bytes as 64 lowercase hex characters, in `provisioning.agent_seed`. That is the one
  preconfigured value per agent. It takes the routes the other secrets take: the environment
  variable that the usual rule gives the key (the prefix `SHELLY`, an underscore, then the key in
  upper case with each `.` written as `_`) and its `_FILE` form, the latter added to the table
  that applies secrets at start (`internal/security/secrets/apply.go:38` lists the existing
  entries), so Compose and Kubernetes supply it the way they supply the agent key. The build
  slice writes the variable names into the docs once the app reads them: a name the app does not
  read yet fails the docs name check. A `shelly-provisioner seed` command
  prints a new seed from `crypto/rand`.
- It is **never logged**, never printed by the `status` command or any configuration dump, and
  never sent. Only the public key and signatures leave the agent. The build slice adds a test that
  captures the agent's log and output at every level and asserts that the seed is not in them.
- Why a seed and not a key file: the same seed gives the same key at every start, so a restart
  needs nothing from disk and no operator step. That is "stateless".
- No seed: the agent stays keyless and registers as today (the pid-based id, the shared key). So
  installing the new binary changes nothing until a seed is set.
- Two hosts with one seed are one identity. Each register replaces the other's token, so they
  churn. One seed per agent.

**The fingerprint.**

- The first 16 bytes (128 bits) of the SHA-256 of the 32 raw bytes of the public key. An attacker
  who wants a key that shows the fingerprint of a real agent needs about 2^128 work, so no
  operator can be shown a look-alike. A shorter value is easier to read and cheaper to forge.
- Readable form: 32 lowercase hex characters in eight groups of four joined by `-`
  (`a1b2-c3d4-...`). Input accepts any case and ignores the `-`. The agent prints it at start, in
  its log and on stdout, and `shelly-provisioner fingerprint` prints it from the seed alone.
- The manager derives the fingerprint from the public key. It never trusts one that is sent.

**The agent id.** `keyed-` plus the 32 hex characters of the fingerprint (38 characters, under the
191 limit at `internal/database/provisioner_store.go:31`). The pid-based id cannot stay for a keyed
agent: `generateAgentID` (`cmd/shelly-provisioner/main.go:421`, used at `:755`) gives a host agent
a new id at each start, and an agent that must come back as itself needs an id that follows from
what it holds. The agent sends this id and the manager refuses a register whose id is not the one
it derives from the key (400). An id that nobody chooses cannot be asked for by someone else. The
keyless path refuses an id that starts with `keyed-` **in any case** (`KEYED-`, `Keyed-`, and so on), a
new restriction on keyless ids added to the validation at
`internal/api/provisioner_handlers.go:110-117`; the test folds case in Go, on the request, so it
does not depend on the database. A new seed is a new id. The hostname stays display text. The prefix is
`keyed-` and not `key-`, because `key-` and 32 hex characters is the shape of a Mailgun key, and secret
scanners (GitHub push protection among them) would flag every keyed agent id.

**How an id is compared, per database, and why the design does not depend on it.** SQLite and
PostgreSQL compare `agent_id` byte for byte. MySQL 8.0.17 and later does too, after the start-up
step that converts the provisioner tables to `utf8mb4_0900_bin`. MariaDB, MySQL 5.7 and MySQL 8.0.0
to 8.0.16 have no such collation: the tables keep a case-folding one and the start-up only logs a
warning (`internal/database/migrations.go:66-82`). There `KEYED-<fingerprint>` finds the row of
`keyed-<fingerprint>`, and today's register code would then take that row over: the insert is a
no-op, the row is read by `agent_id`, the silence test (`provisioner_store.go:166`) passes for a
silent agent and the token is replaced. The fingerprint is no secret, so a holder of the shared key
could do this to a silent approved agent. Two rules close that takeover whatever the collation
says, and a third closes a squat that follows from it:

- the case-folded prefix test above;
- a **state check on the locked row** (item 6 of "The register protocol"), which does not look at
  the id string at all;
- **a new keyless id must be printable ASCII with no space: bytes 0x21 to 0x7E** (lead ruling
  RULING-20261121-1539-638f). Today nothing limits it: `RegisterAgent` checks only that the id is
  not empty and not too long (`provisioner_handlers.go:111-116`). The prefix test cannot copy a
  collation: one that also folds accents or ignores control characters could match `kéyed-<fp>` to
  `keyed-<fp>`. With only those bytes a folding collation can do nothing but fold case, which the
  prefix test already covers. **This rule does not close the takeover; the first two do.** On a
  folding server `kéyed-<fp>` hits the unique index, the insert is a no-op and the locked read finds
  `keyed-<fp>`: that is not the created branch, so the ASCII rule never runs, and the state check
  refuses it. What the rule closes is a **squat**: with no `keyed-<fp>` row yet, `kéyed-<fp>` would
  become a `legacy` row, and the keyed agent's own register would then get a 409 until an admin
  deletes it. The rule runs **in the store**, in the branch that finds the row was
  created by this request (`provisioner_store.go:146-162`: the insert-first step, the locked read,
  and the test that the row holds this request's token hash), and an id outside the set is a 400
  that rolls the insert back. It cannot run in the handler, which cannot tell a new id from an
  existing row; only the `keyed-` prefix test stays there. A register that finds a row skips the
  rule, so **an existing row keeps its id**, even a non-ASCII one, and keeps registering as today.
  **One exemption: an id that starts with the exact bytes `provisioner-`** is not tested. That is
  the shape the shipped agent sends (`provisioner-<hostname>-<pid>`, new at each restart, from
  `cmd/shelly-provisioner/main.go:421-424`), so a host whose name holds a space or a non-ASCII
  byte would otherwise be refused at its next start. The exemption opens nothing: no keyed id
  starts with `provisioner-`, so no collation can match such an id to a keyed row.

What is left, plainly. **The squat closure is unmeasured on MariaDB, MySQL 5.7 and MySQL 8.0.0 to
8.0.16.** The claim that a folding collation does no more than fold case on the bytes 0x21 to 0x7E
is recalled from how those collations work and was not run: no such server was reachable through
the verify route, and CI has none. Slice 2 measures it before it ships. It reads the `agent_id`
column's collation from `information_schema.columns` on each of those servers (the tables keep
whatever collation the migration gave them; `migrations.go:61-62` names `utf8mb4_0900_ai_ci` as
MySQL's default), and under that collation compares every pair of the 94 bytes 0x21 to 0x7E. Only
a letter and its other case may be equal. Any other equal pair means the ASCII rule does not close
the squat, and slice 2 stops there.

Two more limits. On a folding server a `legacy` row of **any** id, plain ASCII included, can be
matched by a variant of that id (another case, an accent, or a trailing space where the collation
pads), because a register that finds a row skips the ASCII rule. Only the 5-minute silence rule
then stands in the way. That is today's behaviour and no new power, since the same key can send
the exact id; the same holds for any new id that starts with `provisioner-` and holds other bytes.
And a keyless caller that sends some other id with a space or a non-ASCII byte is refused from
slice 2 on and must change its id (nothing in the shipped agent does). `strict` removes the
keyless path altogether.

**The register protocol.** The route is today's `POST /api/v1/provisioner/agents/register`
(`internal/api/router_routes.go:247`), with the shared agent key or the admin key as now. The body
gains `public_key` (64 lowercase hex characters, 32 bytes) and `signature` (128 lowercase hex
characters, 64 bytes). The manager decodes `public_key` and refuses any other length, any non-hex
and any upper case with a 400 before it uses the value, because `ed25519.Verify` panics on a key
of the wrong length and every caller controls the field.

1. **Unsigned register (no `signature`).** The manager derives the fingerprint and the id.
   - No row: insert a `pending` row, unless the pending cap is reached (see "Limits"), and answer
     `pending` with the fingerprint. The insert follows the store's insert-first pattern
     (`provisioner_store.go:146-150`), so two replicas registering one new key do not deadlock.
   - `pending`: refresh the row's details and answer `pending`.
   - `approved` or `preapproved`: store a new challenge on the row and answer `challenge`. Nothing
     else on the row is written: no hostname, IP, version, capabilities, metadata, status or
     `last_seen`. Otherwise anyone who knows a public key could rewrite what the operator sees and
     keep a silent row from ever reaching the 30-day token end of "Stale rows".
   - `rejected`: answer `rejected`. No challenge, nothing written.
   - A `legacy` row with that id (it exists only from before the upgrade): 409.
2. **Signed register.** The agent signs the message below with its key and sends it with the
   `public_key`. The manager first checks the format of both fields, whatever the row's state: a
   `public_key` or `signature` of the wrong length or not lower-case hex is a 400, even on a
   `pending` row. It then reads the row's state as it does for an unsigned register: no
   row creates a `pending` row (under the cap) and answers `pending`, `pending` answers `pending`,
   `rejected` answers `rejected`, `legacy` answers 409. Only on an `approved` or `preapproved` row
   does it read the signature. It locks the agent row, as the store does for every agent operation
   (`provisioner_store.go:43-58`), and **issues a token only when the row, read under that lock,
   is `approved` or `preapproved`**, the challenge is stored and not expired, the key is the row's
   key (a `preapproved` row has none yet: the key must hash to the row's fingerprint), and the
   signature verifies. A good signature clears the challenge, replaces the token, moves a
   `preapproved` row to `approved`, stores the key of a `preapproved` row, and updates the
   hostname, IP, version, capabilities, metadata and `last_seen`, in one transaction. The answer is
   today's register answer: 201 with `agent_id`, `agent_token`, `registered_at` and `status`
   (`provisioner_handlers.go:162-169`). A missing, expired or replaced challenge, or a signature
   that does not verify, is a 401 that writes nothing; a bad signature does not consume the
   challenge. The agent then starts again with an unsigned register, which stores and returns a
   fresh challenge.
3. **The three normal enrolment answers are a 200 with a `status`**: `pending`, `challenge` or
   `rejected`. A client reads them as answers and not as errors. **The 200 bodies**, in the same
   response envelope as today's answer (`provisioner_handlers.go:162-169`); the request keeps its
   `id` field, and `public_key` and `signature` sit beside it:
   - `pending`: `status` (`"pending"`), `agent_id`, `fingerprint` (the readable form), `message`.
   - `challenge`: `status` (`"challenge"`), `agent_id`, `fingerprint`, `challenge` (64 lowercase
     hex characters, the 32 random bytes), `message`. The agent has 60 seconds. It signs the full
     message defined below, whose challenge part is the 32 **decoded bytes**, never the hex text.
   - `rejected`: `status` (`"rejected"`), `agent_id`, `fingerprint`, `message`.

   None of the three carries a token, a public key or a hash. A token is issued only in the 201
   answer of a good signed register. The errors keep their usual codes:
   400 for a malformed `public_key` or `signature`, for an id that is not the one derived from the
   key, for a keyless register of an id that starts with `keyed-` in any case, and for a new keyless
   id that is not printable ASCII without a space unless it starts with `provisioner-`; 401 for a failed
   signed register; 403 for a keyless register in `strict`; 409 for a `legacy` row hit by a keyed
   register, for a keyless register that finds a row that is not `legacy` (item 6, the admin key
   included), and for any act the transitions below refuse; 429 for the pending cap.
4. **A valid signature replaces the token with no silence test.** Today a register of an id that
   was seen less than 5 minutes ago and has neither the admin key nor the current token is a 409
   (`provisioner_store.go:166`). A restarted agent has no token, so with that test a keyed agent
   would wait up to 5 minutes at each restart. For a keyed agent possession of the key is the
   authorization, so the test is skipped. It stays on the keyless path.
5. **The token is today's.** 32 random bytes in hex (`newAgentToken`,
   `provisioner_handlers.go:35`), the manager stores the SHA-256 (`HashAgentToken`,
   `provisioner_store.go:69-75`), the agent keeps it in memory (`internal/provisioning/api_client.go:23-25`)
   and sends it in `X-Agent-Token` (`:372-374`). Poll, status and report code does not change.
6. **A keyless register reads the locked row's state, for every actor.** After the insert-first
   step the store reads the row under its lock (`provisioner_store.go:146-150`). If a row exists
   and its state is not `legacy` (or NULL), the register is a 409 **before** the silence test at
   `:166`, whoever asks: the shared agent key, a silent agent, and the admin key too. A keyed row
   is never replaced by a keyless request. The check reads the row the database returned and not
   the id string, so it holds on a case-folding MySQL or MariaDB as well. The admin key still frees
   a keyed id the way it does for every id, through the delete route.

The signed message is the ASCII text `shelly-manager provisioner register v1`, a zero byte, the
agent id, a zero byte and the 32 challenge bytes. The text keeps the key from being used to sign
anything else, the id binds the signature to the agent, and the zero bytes keep the fields from
shifting into each other.

**The challenge.**

- It lives **on the agent row**, in `challenge` (32 random bytes as hex: it is not a credential,
  and the manager must hold the bytes to verify a signature over them) and `challenge_expires_at`.
  Not in the memory of a replica: the manager runs 3 to 10 replicas (Limit 4, above), and the
  unsigned request and the signed one can land on different replicas.
- **Expiry: 60 seconds**, long enough for the second request on a slow link, short enough that a
  challenge nobody used does not stay valid.
- **Single use**: a good signature clears the challenge in the transaction that replaces the
  token, under the agent-row lock, so two replicas cannot both spend it. A reject clears any
  outstanding challenge, so a challenge held by a stolen seed is dead the moment the operator
  rejects. One challenge is outstanding per agent. A new unsigned request replaces it.
- Anyone who knows a public key and holds the agent key can keep replacing that agent's
  challenge. That delays the real agent to its next retry (an availability limit, not an
  authorization one), and no signature means no token. No route that the agent key opens returns
  a public key: the register answer carries the fingerprint of the caller's own key, and the list
  route takes the admin key alone and shows only the fingerprint.

**Enrolment states and transitions.** A new column `enrolment_state` on `provisioner_agents`:

| State | Meaning | Key |
|---|---|---|
| `legacy` | Every row from before slice 2 and every keyless row. A NULL state counts as `legacy`. | Never; it refuses any `public_key` |
| `pending` | A key was seen; the operator has not decided | Yes |
| `preapproved` | The operator named this fingerprint ahead of the agent | None until the first signed register |
| `approved` | The operator accepted it | Yes |
| `rejected` | The operator refused it, or revoked it | Yes, or the fingerprint alone |

Transitions: **approve** takes `pending` or `rejected` to `approved`, except that a `rejected` row
that never had a key (it was `preapproved`) goes back to `preapproved`. **Reject** takes `pending`,
`preapproved` or `approved` to `rejected`. **Pre-approve** takes no row to `preapproved`, or a
`pending` row with that fingerprint to `approved`. The **first valid signature** takes
`preapproved` to `approved`: without it a pre-approved agent could never leave its state. Anything
else is a 409, every act on a `legacy` row included. The `legacy` state is why an upgrade enrols
nobody: a keyless caller of today can register `keyed-<fingerprint of its own seed>`, and that row
is `legacy` and never accepts a key, so the first signed register cannot fill it.

`token_hash` is `not null` and unique (`internal/database/provisioner_models.go:21`). Each
`pending`, `preapproved` and `rejected` row therefore gets a never-issued value: the hash of a
fresh `newAgentToken` that is discarded. A second such row inserts, and a reject revokes the token
of an `approved` row at once.

**What the operator does.** All admin key only, under `/api/v1/provisioner/` beside the routes that
exist (`router_routes.go:247-256`):

| Route | Effect |
|---|---|
| `GET /agents` (exists, `:248`) | Gains `enrolment_state` and `fingerprint` per agent; never a key, a token hash or a challenge |
| `POST /agents/{id}/approve` | `pending` or `rejected` to `approved`; a `rejected` row that never had a key goes to `preapproved` |
| `POST /agents/{id}/reject` | To `rejected`; clears the challenge, replaces the token, fails the agent's `assigned` and `in_progress` tasks with a stated reason |
| `POST /enrolments` with `{fingerprint}` | Creates a `preapproved` row, or approves the `pending` row with that fingerprint |
| `DELETE /agents/{id}` (exists, `:249`) | Removes the row and frees the id; a keyed agent that returns is `pending` again |

Reject shares the step of `removeAgent` (`provisioner_store.go:372-392`) that fails an agent's
`assigned` and `in_progress` tasks, so none of those stays stranded. That step is split out of
`removeAgent`, which keeps the delete. `removeAgent` deliberately keeps `pending` tasks, and so
does reject: nobody receives the `pending` tasks of a rejected id until the operator cancels them
(`POST /api/v1/provisioning/tasks/{id}/cancel`, `router_routes.go:236`) or approves the key again.

- **A new agent.** Generate a seed, put it in the agent's secret store, start the agent, read the
  fingerprint in its log, find the `pending` row in the list, compare, approve. Or read the
  fingerprint with `shelly-provisioner fingerprint` first, pre-approve it, and start the agent;
  the first register then takes it to `approved` with nothing to compare.
- **Revoke an agent**: reject it. The row stays, so the agent does not return as `pending`.
- **A lost seed**: delete the row, generate a new seed, approve the new fingerprint. The old id's
  `assigned` and `in_progress` tasks fail, as for any delete. Its `pending` tasks stay under the
  old id and cannot reach the new one (a different id): cancel them and create them again,
  naming the new id or no agent.
- **A stolen seed**: reject the old row, never delete it. A deleted key returns as `pending`; a
  rejected one stays out. Then generate a new seed.
- With no admin key configured, these routes are open, as every admin route is today
  (`internal/api/handlers.go:115-139`; `provisioner_handlers.go:44-46` lets any caller act for
  any agent). That is why `strict` refuses to start without one.

**Modes.** The manager setting `security.agent_enrolment` (its environment variable follows the
usual rule above) is `open` or `strict`. Any other value stops the start.

| | `open` (default) | `strict` |
|---|---|---|
| Keyless register | As today: the agent key creates an id, the 5-minute rule stays. Three exceptions: an id that starts with `keyed-` in any case is refused (400); a row that is not `legacy` is never replaced (409, the admin key included); a new id of other bytes than printable ASCII without a space is refused (400) unless it starts with `provisioner-` | Refused (403) |
| Keyed register | The flow above | The flow above |
| Token of a `legacy` row | Authorizes as today | Never authorizes: in `strict` only an `approved` row does, and a NULL state counts as `legacy` |
| Start-up | No new requirement | Refuses to start without an admin key |

In `strict` the one check sits in `agentByToken` (`provisioner_store.go:129`). `lockAgent`'s
non-admin path calls it (`:155`), so poll, task status and report are all covered, on every
replica, whatever the age of the token. Its admin path stays unchecked: the admin key acts for any
row. Without that check a keyless row from before `strict` would keep polling and claiming
unassigned tasks with a token that now survives restarts.

The agent setting `provisioning.enrolment` is `open` or `strict`. `strict` refuses to start
without a seed: a seedless agent would register keyless, and `strict` refuses that. This is where
"a strict mode that requires a seed" lives. The agent cannot ask the manager which mode it runs.

**What the agent does while it is not approved.** `EnsureRegistered`
(`internal/provisioning/api_client.go:172`) expects a token from its register today, so the
client's register step changes:

- `pending` is not an error. The agent logs the fingerprint and "waiting for approval" once per
  change, polls no tasks this cycle, and retries the register every 30 seconds, whatever
  `provision_interval` says (the default is 600 seconds, `internal/config/config.go:283`, and the
  loop falls back to 30 seconds only when the value is zero or less, `main.go:130-133`). An
  operator who is watching for the agent must not wait ten minutes. Once approved, the agent
  polls at the normal pace.
- `rejected` logs an error with the fingerprint and retries at the poll interval, so an operator
  who undoes a reject is picked up.
- A 401 on poll, task status or report drops the registration as today, and so does a 404 on
  poll (`api_client.go:209`, `:242`, `:314`). The next poll registers again through the same two
  steps.

`EnsureRegistered` returns a typed "not enrolled yet" result that the poll loop treats as a wait
and not a failure.

**Limits.**

- **Pending cap.** At most 20 `pending` rows. A register of a new key at the cap is refused (429)
  until the operator decides one. The count and the insert are separate statements, so concurrent
  registers, on one replica or several, can overshoot by as many rows as there are requests in
  flight: the cap is a soft limit and the overshoot has no fixed bound. A `preapproved` row is made by the admin
  and bypasses it. A holder of the agent key can fill the cap and so block new unknown agents; it
  cannot enrol, claim or take over anything in a keyed id. The operator rejects the pending rows,
  or pre-approves the fingerprints it expects.
- **Pending rows are display text.** The hostname, IP, version and metadata of a `pending` row are
  what the caller sent, and a `pending` row has not proved that it holds the key. The operator
  trusts only the fingerprint, compared with the agent's own log.

**Cost on a restart.**

| Event | What the agent sees | Wait |
|---|---|---|
| Agent restarts | Two requests: the challenge, then the signature | None; no operator step |
| Manager or replica restarts | Nothing: the registry is in the database (slice 1) and the token is valid | None |
| A register answer is lost | The next register answers a new challenge and issues a new token | One retry |
| The token ended after 30 days of silence (see "Stale rows") | A 401, then the two requests | None; no operator step |
| The seed is lost or stolen | See "What the operator does" | One approval |

- Protects: a holder of the agent key cannot produce a valid signature for an approved key, and
  so cannot get a token for a key it does not hold (knowing a public key, and so its fingerprint,
  proves nothing). A keyed id has no takeover: the 5-minute rule is not on its path, and a keyless
  register that finds a keyed row, by any id the database matches to it, is a 409 for every actor
  (item 6 of "The register protocol").
  In `strict` the agent key alone creates no id and opens no claim.
- Operator: compare one fingerprint per agent, or pre-approve it; one seed per agent at most.
- Agent: one configured value, nothing on disk.
- Does not solve: see "What this does not solve" (a copied seed, a rogue manager, a hasty
  approval, and the keyless path that `open` keeps).

### Which options change who may enrol an agent

State this plainly, because it moves a security boundary:

- **A (slice 1):** does not change who may enrol. It closes one window: after
  a manager restart an id the manager has already seen is no longer free to
  take at once. That is a tightening. It is not a pure one: without the row
  lock of decision 11, a claim, a status update or a report that began before a
  token was replaced can finish after it, for one request. Today one mutex
  covers the token check and the claim, so that is a small loosening of one
  control. Decision 11 settles it, and slice 1 took the lock, so the window is
  closed.
- **B, when the manager requires enrolment:** an agent id can only be created by
  the **admin key holder** (through an enrolment). The agent key alone no
  longer creates an id. Today it does.
- **C:** an agent id only gets a token after the **admin key holder** approves
  it. The agent key alone no longer yields a token.
- **D:** an id is only freed by its token or by the **admin key holder**. The
  agent key alone no longer frees an id after 5 minutes of silence.
- **E:** changes who may claim a task, not who may enrol.
- **F, in `strict`:** an agent id gets a token only after the **admin key holder** approves the
  key that the agent proves it holds. The agent key alone creates no id, frees none and yields no
  token. **In `open` (the default) nothing changes for a keyless agent**, except that a keyless
  id that starts with `keyed-` in any case is refused, and so is a new keyless id that is not
  printable ASCII without a space unless it starts with `provisioner-` (a caller that uses either
  today must change its id; an existing row keeps its id): the
  agent key still creates other ids and the 5-minute rule stays on them, so a shared-key holder
  keeps today's powers until `strict` is on. A keyed id is safer from the first day: it needs an approval and has no
  takeover. Nothing loosens a control that exists today. The one narrowing of an earlier decision
  is the purge (decision 7, below).

### Recommendation

**Option F**, which Serge chose on 2026-11-21 at 14:13 CET (typed "F"). Build slice 1 (built),
then slice 2 (the manager), slice 3 (the agent) and slice 4 (the UI), each alone, starting in
`open` so that no running setup breaks. B, C and D are not built, and E stays as it is: with F,
once `strict` is on, only approved agents can claim, which removes most of E's exposure.

Why F over B: B hands the operator a secret per agent and makes the agent keep a token file, so a
lost file costs a new secret and the agent is not stateless. F needs one seed per agent at most,
keeps nothing on disk, and approves by comparing a fingerprint. Why F over C: C asks the admin to
judge a caller that named itself. In F the identity is a key whose fingerprint the agent itself
printed, so the admin compares and does not guess. Why F covers D: an id belongs to its key, so the
5-minute takeover has no path to a keyed id, and `strict` removes the keyless path that still has it.

## Stale rows

Storage makes a dead agent permanent. Today a restart forgets it. With slice 1
every id ever registered stays in `provisioner_agents`, and a host agent that
gets a new id on each start (pid in the id) leaves one row per start.

Two things can remove a row:

1. **An admin delete route** (admin key). It frees an id and revokes its token.
   Needed for D in any case.
2. **A purge of rows silent for N days**, with no `pending` task. Proposed
   N is 30. It runs in the manager, on any replica, as a conditional `DELETE`.

The 5-minute silence rule removes no row. It lets a register **replace** the
token of an existing row, which keeps the row and its `registered_at`
(`provisioner_handlers.go:149-193`); the list only shows such a row as
`offline` (`:211-215`). D removes that way of freeing an id.

D and the cleanup interact. With D, only (1) and (2) free an id. An agent that
lost its token waits for a purge or an admin. So D needs B or a kept token, as
above.

### Stranded `assigned` tasks

`PollTasks` hands out only tasks whose status is `pending`
(`provisioner_handlers.go:254-255`). A task assigned to an agent process that
died is therefore never delivered again, to the same id or to a new one (the
restart table above says so). Today a manager restart wipes such tasks. After
slice 1 they stay forever, and a purge that waited for "no non-terminal task"
would never remove that agent's row. The purge therefore waits only for
`pending` tasks (decision 12).

Manual recovery exists today, under the existing ownership checks: the admin
key may act for any agent, and the agent that owns the task may act for it
(`authorizeAgent`, `UpdateTaskStatus` at `:303-317`). Either can set the task
back to `pending`, because the handler accepts any status. An agent can do this
only while it holds a token for that id. Decision 12 asks whether to automate
it.

### Rows under Option F

The purge of slice 1 (`Purge`, `internal/database/provisioner_store.go:421`; 30 days at
`internal/api/provisioner_handlers.go:178-179`) removes any agent silent for 30 days that has no
`pending` task. Under F that would undo an approval: an approved agent that was off for a month
would return as `pending`, and a rejected stolen key could return too. So the purge goes by state:

| State | Silent for | What the purge does |
|---|---|---|
| `legacy` | 30 days | As today: removes the row if it has no `pending` task, and fails its `assigned` and `in_progress` tasks as `removeAgent` does (`:375`) |
| `pending` | 7 days | Removes the row if it has no `pending` task |
| `approved` | 30 days | Keeps the row, the key and the approval. Replaces the session token with a never-issued value and fails the agent's `assigned` and `in_progress` tasks with the stated reason, through the step shared with reject. No `pending`-task condition applies, because the row stays. The agent's next start answers a signed challenge, with no operator step |
| `preapproved`, `rejected` | never | Nothing. Only an admin delete removes the row |

A row in an operator-decided state (`approved`, `preapproved`, `rejected`; a `preapproved` row has
no key yet) is never deleted for silence. A purged `rejected` row would let the same key return
as `pending`. The token end for a silent `approved` agent keeps the control that exists today: the
purge ends a silent agent's token after 30 days. **This narrows decision 7**, which removed any
agent silent for 30 days with no `pending` task.

## Compatibility at upgrade

### Slice 1

- The tables are created at startup. They are empty. Agents that are running
  with the shared key hold an in-memory token the manager does not know, so
  their next poll gets 401, they drop the registration and register again at
  the next poll. This is the same as a manager restart today: one poll
  interval, no 409 (the manager has no record).
- Tasks that were only in memory are lost, as in any restart today. Drain the
  queue before the upgrade.
- A rolling update (`maxSurge: 1`, `maxUnavailable: 0` in the production
  overlay) runs old and new pods together. Requests that land on an old pod use
  its memory, and on a new pod use the database. During the roll an agent may
  move between the two and register more than once. It can then be locked out:
  it registers on the database, loses that token when a poll lands on an old
  pod (the client clears the token on a 401 or 404, `api_client.go:180-188`) and
  registers there instead. After the last old pod stops, its token from the old
  pod's memory is unknown (401), and its next register finds the online
  database row and gets 409 (`provisioner_handlers.go:153-157`). It then waits
  for the rest of the 5 minutes since that row was last seen, plus up to one
  poll interval for the retry (600 seconds in the shipped Kubernetes agent).
  Slice 1 does not remove this wait, because the silence rule stays. Under
  Option F it stays for keyless ids only: a keyed agent answers a signed
  challenge and waits for nothing.
- An old agent binary works with the new manager. A new agent binary works with
  an old manager. No API field changes.

### Slice 2 (the manager, Option F)

- The new columns are nullable, as "The migration" requires for a table that holds rows:
  `public_key`, `fingerprint` (a unique index; every provider allows many NULLs in one),
  `enrolment_state`, `challenge`, `challenge_expires_at` and `approved_at`. The fixup adds them
  nullable and backfills `enrolment_state` as `legacy` for every existing row. A NULL state also
  counts as `legacy`, so a row that an old replica writes during a rolling update, before the
  backfill, is safe.
- `security.agent_enrolment` defaults to `open`, so the manager behaves as today for every
  keyless agent, and an upgrade changes nothing until a slice 3 agent sends a key. There are two
  exceptions, both refused from slice 2 on: a keyless id that starts with `keyed-` in any case, and a
  new keyless id that is not printable ASCII without a space unless it starts with `provisioner-`.
  An agent that uses such an id today must change it (nothing in the shipped agent does,
  `main.go:421`: its id starts with `provisioner-`). An existing row keeps its id and keeps
  registering as before.
- A rolling update runs old and new pods together. An old pod ignores `public_key` (see "Slice
  3") and answers a keyed register with a token and a `legacy` row under the `keyed-` id. A new pod
  then answers 409 for that id until an admin deletes the row. And an old pod does not enforce
  `strict`. So set `strict`, and start the first keyed agent, only after the roll has finished.
- The manager does not need the database to be on a new provider or a new version: the columns
  are plain strings and times.

### Slice 3 (the agent, Option F)

- An agent with no seed is unchanged, and so is an old agent binary. Upgrading the binary
  changes nothing until a seed is set.
- A keyed agent has a new id, `keyed-<fingerprint>`, so the row of its old pid-based id stays as a
  `legacy` row and its `assigned` tasks stay stranded (decision 12) until the 30-day purge or an
  admin delete. Drain the agent's tasks before its seed is set.
- Order for an operator: (1) upgrade the manager (slice 2), still `open`; (2) per agent, generate
  a seed, set it, start the agent, compare the fingerprint in its log with the pending row, and
  approve (or pre-approve the fingerprint before the first start); (3) when every agent shows
  `approved`, set the manager to `strict` and the agents to `strict`; (4) delete the old `legacy`
  rows, or let the purge remove them.
- An agent in `strict` with no seed refuses to start; an agent in `strict` also refuses a manager
  that answers a keyed register with a token (a manager without slice 2).

### Slice 4 (the UI)

- Nothing to migrate. The operator can use the agents page or the routes of slice 2.

## Risk

A token is a credential. These are the rules for it.

- **Hashing.** The manager stores the hex SHA-256 of the token. The token is
  256 bits of random data, so an attacker cannot guess it or reverse the hash.
  A slow hash such as bcrypt adds cost on every poll and gives nothing here: it
  cannot be indexed, because each hash has a random salt, and the lookup would
  go back to scanning every row. A fast hash with a unique index finds the agent
  in one lookup, which also removes the linear scan in `agentOf` (`:83-91`).
- **Where the plain token is shown.** Only in the body of the register response,
  once, to the agent that registered. The manager does not log it. The admin
  list must not return the hash either, and the build slice adds a test that
  asserts it. TLS in front of the manager is required for this to hold
  (`docs/security/SECURITY_TLS_PROXY.md`).
- **Rotation.** (This bullet and the next describe the keyless path, which is
  the only path before Option F and stays in `open`. A keyed agent differs: only
  a valid signature of its approved key issues it a token, a holder of the
  current token or the admin key alone cannot, and recovery from a lost
  response is the next signed register. See Option F.) A register with the
  current token issues a new token, and the old one stops working at once. This
  is how it works today (`:193`), and the note keeps it for keyless ids. A lost response strands the agent with a dead token. The
  note does **not** add a grace period for the previous token: a grace period
  would let a token that leaked and was rotated away still register for a
  while, which weakens revocation. Recovery from a lost response is the 5-minute
  rule in `open`, or an admin delete. (Under Options B and D it would have been an
  enrolment that names the existing id; F builds neither, so that route is history.)
  That is rare (the response is lost after the manager has committed the new token)
  and the recovery costs one silent wait or an operator step, not a loss of data. For a keyed agent
  (Option F) a lost response costs nothing: the next register answers a new
  challenge and issues a new token, and no previous token is honoured.
- **What a leaked token lets a holder do.** Every provisioner agent route first
  needs the agent key or the admin key (`agentOrAdmin`,
  `internal/api/router_routes.go:241-249`; `requireAgentOrAdmin`,
  `handlers.go:122`), and the token is a second credential on top. So the
  holder described here has the token **and** the agent key. That holder can act
  as that one agent: poll the tasks assigned to it and the unassigned ones it
  claims, read their `config` (which may hold Wi-Fi settings or device
  credentials), set the status of its own tasks, and report discovered devices
  as that agent. The holder can also register with the token and so **replace**
  it, which locks the real agent out. It opens no admin route, and it cannot act
  for another agent. Revoking the leaked token already works today: a register
  by the admin key replaces the token without presenting the current one
  (`provisioner_handlers.go:153-157`, `:193`), and the proposal keeps that path.
  That revokes the token only. Restoring the legitimate agent's credentials is
  a separate step: the agent registers again (today) or an enrolment that names
  its id is redeemed (Option B). The admin delete route is for freeing an id,
  not for revocation. A token alone, without the agent key, opens nothing. None of this
  changes with the proposal for keyless agents: the shared key stays the outer control until
  the operator chooses to retire it, which this note does not propose. **For a keyed agent
  (Option F) the register paths above do not exist:** a register by the holder of the current
  token, or by the admin key, issues no token, because only a valid signature of the approved
  key does. So a leaked token of a keyed agent cannot replace itself or lock the real agent out,
  and it ends at the next signed register of the real agent, or when the operator rejects the
  agent (reject replaces the token at once). The agent key is still needed to reach the routes.
- **A leaked agent key** is limit 2 and 3 for keyless agents until `strict` is on
  (slices 2 and 3). It cannot enrol, claim or take over a keyed id from the day slice 2
  lands, because a token needs a signature of the key that the operator approved. In
  `open` it can still create keyless ids and take a silent keyless id, as today.
- **What a database read leaks.** Hashes, which cannot be replayed as tokens,
  because the header carries the token and the manager hashes it. It also leaks
  task `config` in plain text. `Device.Settings` and `Device.DesiredConfig`
  are already plain-text columns in the same database (`models.go:25`, `:35`)
  and the database package has no field encryption (`rg -i encrypt` over
  `internal/database` finds one hit, the `Encryption` option of a backup
  config, `provider/interface.go:165`), so a task `config` adds no new class of data
  at rest. It is still a decision (below).

## What this does not solve

- It builds nothing. No store, no table, no route, no setting.
- It does not decide who may enrol an agent. Serge decided that (Option F); the note says
  which options change it and how slices 2 to 4 build F.
- It does not change the agent key itself. There is still no API to rotate it
  (`docs/security/SECURITY_SECRETS.md:151`).
- A compromised enrolled agent still acts as itself until an admin revokes it. With F, a
  **copied seed is a copied identity**: whoever holds it signs as that agent until the operator
  rejects its fingerprint and the agent gets a new seed.
- In `open`, the default, the keyless path stays: a holder of the agent key can still create
  keyless ids and take a silent keyless id, as today. Only `strict` closes that.
- F trusts a fingerprint on first use, and only the operator's comparison makes that trust
  sound. An operator who approves a `pending` row without comparing approves whatever is there.
- A **rogue manager**, or a man in the middle of an unencrypted link, can ask an agent to sign.
  The signed text names no manager, because a manager does not know its own external address
  behind a proxy. TLS in front of the manager and the agent's configured URL are the control
  (`docs/security/SECURITY_TLS_PROXY.md`).
- Two agents started with one seed churn each other's tokens, and nothing detects it.
- A holder of the agent key can fill the pending list to its cap (see "Limits") and can keep
  replacing a known agent's challenge. Both delay agents and enrol nobody.
- It does not encrypt task `config` at rest.
- Wi-Fi provisioning itself, and discovered devices, are unchanged.

## Decisions needed

One recommendation per question.

Decisions 3 to 6 were answered by Serge's "F" (typed 2026-11-21 14:13 CET) and are rewritten
below. Decisions 1, 2 and 7 to 12 were taken by the lead for slice 1; they stay as they were
asked, amended where F changes them (7, 9, 10). One question is open for Serge: decision 13.

1. **Build slice 1 (the whole registry in the database) now?** Recommended:
   yes. It fixes limits 1 and 4 and the empty-registry window, changes no API,
   and needs no agent change. Its one change to a control would have been the
   one-request window of decision 11, which the row lock closed.
2. **Store a task's `config` in the database as plain text?** Recommended: yes,
   like `Device.DesiredConfig`. Field encryption is a separate, larger decision.
3. **Which enrolment option?** Answered: **F**, fingerprint approval of a seeded agent key
   (Serge, 2026-11-21 14:13 CET). In `strict` it **changes who may enrol an agent**: only a
   key that the admin key holder approved gets a token.
4. **Does the agent keep its id and token in a file?** Answered by F: no. The agent keeps
   nothing on disk. Its id follows from its key, and its token lives in memory.
5. **Make `strict` the only mode eventually, or keep `open`?** Answered: keep `open` as the
   default, add `strict`, and refuse to start `strict` with no admin key (manager) or with no
   seed (agent). Whether `strict` ever becomes the only mode is decision 13.
6. **Remove the 5-minute takeover (D)?** Answered by F: a keyed id has no takeover, and
   `strict` removes the keyless path that still has it. No separate removal is built, so the
   old slice 3 is gone.
7. **Stale rows.** Recommended: an admin delete route, and a purge by state (see "Rows under
   Option F"). A `legacy` row is removed after 30 days silent with no `pending` task, as the
   decision first read; a `pending` row after 7 days; an `approved` row keeps its key and
   approval and only loses its session token after 30 days; a `preapproved` or `rejected` row
   is never removed for silence. **F narrows this decision**: it no longer removes every agent
   silent for 30 days (see decision 12 for `assigned` tasks).
8. **Rotation grace for the previous token?** Recommended: no. A grace period
   weakens revocation. For a keyless agent (a `legacy` row, which F's enrolment routes cannot
   reach) a lost register response is recovered in `open` by the 5-minute silence rule of
   today's register, or at once by an admin delete; the "new enrolment" of Options B and D is
   history and F builds none. For a keyed agent (Option F) it is recovered by its next signed
   register, with no operator step.
9. **Unassigned tasks (E).** Recommended: leave claimable for now. Revisit once `strict` is
   in use, when only approved agents can claim.
10. **Docs.** Slice 2 updates `docs/security/SECURITY_SECRETS.md:21`
    and the provisioner paragraphs of `docs/api/api-overview.md` (identity, registry and
    purge, `:495-499`), and adds its provider test to the `-run` list in
    `.github/workflows/test.yml:197`. Slice 3 adds the seed
    to `docs/security/SECURITY_SECRETS.md`, and the `seed` and `fingerprint` commands and the agent
    settings to `docs/guides/provisioner-status.md` or a guide beside it.
11. **Close the one-request window with a row lock, or accept it?** The choices:
    accept the window (a request that began before a token was replaced may
    finish after it), or take the agent row `FOR UPDATE` in the register and in
    the claim, status update and report transactions on PostgreSQL and MySQL
    (SQLite has one writer at a time). Recommended: take the lock. There are
    few agents, each polls rarely, so the cost is small, and slice 1 then
    loosens no control. Without it, slice 1 loosens one control slightly, and
    the lead and Serge should know that before slice 1 is built.
12. **Stranded `assigned` tasks.** Choices: keep the manual recovery as it is
    (the admin key or the owning agent sets the task back to `pending`), or
    release the `assigned` tasks of an agent silent for N. Recommended: the
    manual recovery in slice 1, which keeps today's behaviour and adds no
    permission. Automatic release can run a provisioning step twice on a device,
    so it needs its own decision. The purge (decision 7) ignores `assigned`
    tasks of the agent it removes, and marks them `failed` with a stated
    reason, so the UI shows them. Under F it does the same for the `assigned`
    and `in_progress` tasks of an `approved` agent whose token it ends.
13. **Open for Serge: make `strict` the only mode in a later release?** Choices: keep `open`
    as a permanent default that leaves the keyless path alive, or remove the keyless path
    after one release in which `strict` was available. Recommended: keep `open` for now. It
    is the only mode in which an operator's current setup, one shared key and no seed, keeps
    working, and `strict` already gives every operator who wants the closed boundary a switch.
    Decide again once the operator has moved every agent to a seed.
