# Provisioner registry in the database, and agent enrolment

Status: proposal for ginsys/shelly-manager#638. Nothing in this note is built.
Read at origin/main 147080796e83 (cites are `file:line` at that commit). Every
cited file is the same at 6343b21c3fda, the base this note sits on.

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

This note proposes three slices, each of which ships alone:

| Slice | What it does | Fixes | Changes who may enrol an agent? |
|---|---|---|---|
| 1 | The whole registry moves to the database: agents, hashed tokens, tasks. | 1, 4, and the empty-registry window | No new right. It closes a window (see "The empty-registry window") and opens a smaller one: a request that began before a token was replaced may finish after it, for one request (see "The store", detail 2, and decision 11). |
| 2 | Enrolment: an agent id can only be created by the admin key holder. | 2 | **Yes.** Today the agent key is enough. |
| 3 | An id is no longer freed by 5 minutes of silence. Stale rows are removed on purpose. | 3 | **Yes.** Today the agent key frees an id after 5 minutes of silence. |

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
| Manager restarts, one replica | Its next poll gets 401 (the registry is empty), it registers again at its next poll. No 409, because the manager has no record. `TestProvisionerClient_RegistersAgainAfterAManagerRestart` (`internal/api/provisioner_client_e2e_test.go:117`) proves this. | One poll interval (30 s in the shipped agent config, 600 s in the shipped Kubernetes agent). Tasks in memory are gone. |
| Agent restarts and keeps its id (container), one replica | It has no token. It registers with the agent key only. If the manager's record was seen less than 5 minutes ago, the answer is 409. It retries at each poll. | The rest of the 5 minutes counted from its last poll before the restart, plus up to one poll interval for the retry. With a 600-second interval the record is often already more than 5 minutes old, and then there is no wait at all. Tasks assigned to the old process stay `assigned` and nobody polls them again. |
| Agent restarts with a new id (host) | It registers a new id. No wait. The old row stays as an offline agent. | None. |
| Agent holds the admin key | `holdsAdminKey` is true, so the 409 never applies (`:153`) and tokens are not checked. | None. This is the pre-token behaviour, and the reason the docs say to send the agent key and never the admin key (`docs/security/SECURITY_SECRETS.md:15`). |

`docs/api/api-overview.md:433` says "a manager restart drops them, and a
restarted agent waits until its old record is 5 minutes silent". Read together,
the two clauses suggest a manager restart causes the wait. It does not: the
wait follows an **agent** restart. The build slice corrects the sentence.

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
  the collation `utf8mb4_0900_bin` (MySQL 8.0 and later, which CI runs) or a
  binary string type. The provider test proves, on every provider, that
  `alpha`, `Alpha` and `alpha ` are three agents (and the unique index accepts
  all three), that a task owned by one is not claimed by another, and that a
  `"PENDING"` task is not claimed. The `token_hash` is lowercase hex that the
  manager writes itself, so it needs the collation only for strictness.
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
   writer at a time). The note leaves that choice to the build plan.
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

### Slice 2: enrolment

See the next section. It adds an admin route and an agent setting.

### Slice 3: no takeover by silence, stale rows

Removes the 5-minute rule from `RegisterAgent` and adds the cleanup in "Stale
rows". It ships only after slice 2 is the required mode, or after the agent
keeps its token (see "Option D").

## Enrolment

The question: who may create an agent id, and how does an agent prove it is the
one that was created. Today, anyone with the agent key.

All options need an admin key. With no admin key configured, the provisioner
routes are open and no token is checked, so no enrolment can be enforced. A
manager set to require enrolment and having no admin key must refuse to start.

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

### Which options change who may enrol an agent

State this plainly, because it moves a security boundary:

- **A (slice 1):** does not change who may enrol. It closes one window: after
  a manager restart an id the manager has already seen is no longer free to
  take at once. That is a tightening. It is not a pure one: without the row
  lock of decision 11, a claim, a status update or a report that began before a
  token was replaced can finish after it, for one request. Today one mutex
  covers the token check and the claim, so that is a small loosening of one
  control. Decision 11 settles it.
- **B, when the manager requires enrolment:** an agent id can only be created by
  the **admin key holder** (through an enrolment). The agent key alone no
  longer creates an id. Today it does.
- **C:** an agent id only gets a token after the **admin key holder** approves
  it. The agent key alone no longer yields a token.
- **D:** an id is only freed by its token or by the **admin key holder**. The
  agent key alone no longer frees an id after 5 minutes of silence.
- **E:** changes who may claim a task, not who may enrol.

### Recommendation

Do slice 1 first. Then B, with a token file in the agent. Then D, once enrolment
is required. Keep C as the fallback if the operator does not want to hand out
secrets, and leave E until after B and D: once only enrolled agents can claim, most of
E's exposure is gone.

Why B over C: the secret is given out by the admin and reaches only the agent
meant to have it. C asks the admin to judge a caller who named itself.

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
  Slice 1 does not remove this wait, because the silence rule stays until
  slice 3.
- An old agent binary works with the new manager. A new agent binary works with
  an old manager. No API field changes.

### Slice 2

- Add a setting with two values, `open` and `required`, **default `open`**. In
  `open` the manager behaves as today. So an upgrade changes nothing until an
  operator turns it on.
- The operator enrols every running agent, then sets `required`. An agent whose
  binary cannot keep a token or present a secret cannot register under
  `required`, so upgrade the agents first.
- `required` with no admin key: refuse to start (see "Enrolment").

### Slice 3

- Ships once `required` is the mode in use, or once the agent keeps its id and
  token. Until then the 5-minute rule is the only way a lost token is recovered.

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
- **Rotation.** A register with the current token issues a new token, and the
  old one stops working at once. This is how it works today (`:193`), and the
  note keeps it. A lost response strands the agent with a dead token. The
  note does **not** add a grace period for the previous token: a grace period
  would let a token that leaked and was rotated away still register for a
  while, which weakens revocation. Recovery from a lost response is the 5-minute
  rule until D, and after D an enrolment that names the existing id (Option B,
  which replaces its token) or an admin delete followed by a new enrolment. That is
  rare (the response is lost after the manager has committed the new token)
  and the recovery is an operator step, not a loss of data.
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
  changes with the proposal: the shared key stays the outer control until the
  operator chooses to retire it, which this note does not propose.
- **A leaked agent key** is limit 2 and 3 until slice 2 and 3 land.
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
- It does not decide who may enrol an agent. It recommends, and says which
  options change that.
- It does not change the agent key itself. There is still no API to rotate it
  (`docs/security/SECURITY_SECRETS.md:151`).
- A compromised enrolled agent still acts as itself until an admin revokes it.
- It does not encrypt task `config` at rest.
- Wi-Fi provisioning itself, and discovered devices, are unchanged.

## Decisions needed

One recommendation per question.

1. **Build slice 1 (the whole registry in the database) now?** Recommended:
   yes. It fixes limits 1 and 4 and the empty-registry window, changes no API,
   and needs no agent change. Its one change to a control is the one-request
   window of decision 11.
2. **Store a task's `config` in the database as plain text?** Recommended: yes,
   like `Device.DesiredConfig`. Field encryption is a separate, larger decision.
3. **Which enrolment option?** Recommended: B (a one-time enrolment secret per
   agent). C is the fallback. This **changes who may enrol an agent**: when
   required, only the admin key holder. It needs the lead's and Serge's
   decision before it is built.
4. **Does the agent keep its id and token in a file?** Recommended: yes, opt-in
   by a configured path, mode 0600, under `/data` in the image, and the agent
   uses the stored id. Without it, B costs one new secret per agent restart, and
   a host agent (new id per start) cannot reuse a kept token.
5. **Make `required` the only mode eventually, or keep `open`?** Recommended:
   keep `open` as the default until the operator turns on `required`, and refuse
   to start with `required` and no admin key.
6. **Remove the 5-minute takeover (D)?** Recommended: yes, in slice 3, only after
   B is required or the agent keeps its token. This **changes who may free an
   id**.
7. **Stale rows.** Recommended: an admin delete route, and a purge of rows
   silent for 30 days with no `pending` task (see decision 12 for `assigned`
   tasks).
8. **Rotation grace for the previous token?** Recommended: no. A grace period
   weakens revocation. A lost register response is recovered by a new
   enrolment or an admin delete.
9. **Unassigned tasks (E).** Recommended: leave claimable for now. Revisit after
   B and D, when only enrolled agents can claim.
10. **Docs.** The build slice updates `docs/security/SECURITY_SECRETS.md:21`
    and `docs/api/api-overview.md:433` (including the restart sentence), and adds
    its provider test to the `-run` list in `.github/workflows/test.yml`.
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
    reason, so the UI shows them.
