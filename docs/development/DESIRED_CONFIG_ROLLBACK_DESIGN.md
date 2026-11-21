# Design note: rolling a device's desired configuration back to an earlier version

Status: proposal. Its last section lists the decisions and one recommendation for each. No code,
OpenAPI or UI file changes with this note; the build is cut into slices (section 4) that are briefed
after it merges.

It answers ginsys/shelly-manager#775, "No way to restore a device's stored config to an earlier
history version". All `file:line` references are at origin/main `82b3985268c4`.

## 1. What the issue asks, and what the code makes of it

The issue asks that the Restore button of the history page put the stored configuration back to an
old version, with replace semantics, a rule for secrets, a history row naming the version, and a
clear statement that the device is unchanged until the configuration is applied.

Three facts in the code change what a "restore" can be:

1. **The history is of the stored import, not of what the device is told.** `ConfigHistory`
   (`internal/configuration/models.go:96`) records changes of the `device_configs` row, the record of
   the last import. Apply never sends that row: `ShellyService.ApplyDesiredConfig`
   (`internal/service/service.go:1243`) calls `ConfigurationService.ApplyDesiredConfig`
   (`internal/configuration/apply_service.go:378`), which snapshots `device.DesiredConfig` (`:401`)
   and sends that (`:406`). Restoring an old version into the stored row would send nothing to the
   device and would make the record of the last import describe a state the device is not in.
2. **No row is written when desired changes.** Desired has three writers, and none writes history:
   `RecomputeDesiredConfig` through `SetDeviceDesiredConfigPending`
   (`internal/configuration/config_service.go:643`, `repository.go:260`), and the archive import
   (`internal/sync/import_store.go:222` for a new device, `:254` for an overwrite).
3. **Desired is derived.** `RecomputeDesiredConfig` rebuilds it from the device's templates and then
   its overrides, the overrides last (`config_service.go:604-625`). A value written straight into
   `desired_config` is lost at the next recompute: any override edit (`:373`, `:433`) or template
   edit (`RecomputeAffectedDevices`, `:673`).

So the real need is the one the parked comment on the issue names: roll back the **desired**
configuration. That needs a history of desired (fact 2) and a way for the rolled-back value to
survive a recompute (fact 3). The history list that exists today stays a record of imports.

## 2. What exists today

### 2.1 The existing history rows and what they hold

| Action | Writer | Holds |
|---|---|---|
| `import` | `createHistory` (`internal/configuration/service.go:104`, `:150`) | the device's own API JSON plus `_metadata` and `device_info` (`service.go:216-236`) |
| `template` | `createHistory` (`service.go:511`, `:526`) | the template's `Config`, typed `DeviceConfiguration` JSON (`service.go:488`; parsed that way at `config_service.go:607`) |
| `manual` | `createHistory` (`service.go:576`) | the stored row with the sent top-level keys merged in |
| `updated` | `UpdateDeviceConfigFromJSON` (`service.go:1140-1155`) | `NewConfig` only, no `OldConfig`, `ConfigID: 1` as a placeholder |

`createHistory` stores both sides as they are read and swallows its own error
(`service.go:892-919`), so a row can silently be missing. `ConfigID` is NOT NULL (`models.go:99`) and
names a `device_configs` row that need not exist.

The route `GET /devices/{id}/config/history` (`internal/api/router_routes.go:81`) answers the rows
masked: `maskConfigHistory` changes the copy, never the stored rows
(`internal/api/config_history_secrets.go:14`, `internal/api/handlers.go:1180-1209`). So the stored
rows can hold real secrets, and the history answer can never bring one back.

### 2.2 How desired is written, and how it is protected

- `SetDeviceDesiredConfigPending` writes `desired_config` and `config_applied = false` in one UPDATE,
  so a flag cannot read applied against a changed configuration (`repository.go:255-266`).
- An apply ends with a compare-and-set that sets `config_applied` only while desired still equals the
  snapshot, byte-exact on MySQL (`apply_service.go:500-511`, `repository.go:284-307`). A rollback that
  lands during an apply makes that compare fail: the answer carries the warning "The desired
  configuration changed while it was being applied; the device stays pending." (`apply_service.go:508-509`).
- One apply per device runs at a time across manager replicas (`apply_service.go:387`,
  `internal/configuration/apply_lock.go:118`).
- `RecomputeDesiredConfig` reads the device at `config_service.go:594`, the templates and overrides
  next, and writes at `:643` with no check that those inputs are still the stored ones.
- Overrides can add or replace a value. They cannot delete a value a template gives:
  `PatchDeviceOverrides` removes a path from the overrides, not from the merged result
  (`config_service.go:405-425`).
- Merging one layer into an empty configuration keeps its channel lists as they are
  ("an empty dst takes src as it is", `internal/configuration/merge.go:219-220`, `:264-286`).
  **It keeps a plain, non-pointer scalar field only when its value is not a zero.** `mergeInto` copies
  pointer fields, nested structs, slices and a non-zero plain scalar (`merge.go:116-121`), and records its
  source. `SNTPConfig.Server` is a plain `string` (`internal/configuration/typed_models.go:210-212`), so a
  layer holding `{"system":{"sntp":{"server":"time.example"}}}` merges to the same value, and a layer
  holding `"server":""` merges to nothing, because a plain field cannot tell an empty value from an unset
  one (ginsys/shelly-manager#839 made the merge copy them; before it the merge dropped every plain
  scalar). A desired value that came straight from an archive import (`import_store.go:222`, `:254`) can
  hold a plain value that no layer holds; a recompute never rebuilds it, whatever the field kind, because
  it builds from the layers only (`config_service.go:596-643`).
- Bulk apply already writes a partial configuration into the overrides and applies it
  (`internal/api/bulk_apply.go:292-310`).

### 2.3 Secrets

`secretSlots` lists every secret of a `DeviceConfiguration` by its JSON path
(`internal/api/handlers_config_templates.go:552-578`); `allSecretPaths` lists them sorted (`:627`).
The key rule that masks a document is `redact.IsKey` (`internal/redact/redact.go:18-26`).
`redact.MaskTree` masks by key name (`internal/redact/walker.go:125`); `redact.RestoreTree` gives a
placeholder the stored value at the same key, drops it when none is stored, and for a list entry
keeps the stored secret only when the entry agrees with the stored one on everything but its secrets
(`walker.go:171-196`, `:224-234`: `SameWithoutSecrets` compares the entry's other fields). A secret in a
map, such as `wifi.password` or `mqtt.pass`, gets the stored value at the same key with no look at its
sibling fields. It reports no path it dropped.
The PUT routes use it so a client that sends the mask back keeps the stored secret
(`internal/api/handlers.go:1231`, `internal/api/handlers_config_device.go:355-368`).

## 3. Design

### 3.1 Which rows can be a rollback source

**Only rows of a new `desired_config_history` table.** The existing `ConfigHistory` rows are refused as
sources, each with its reason:

- An `import` row is the device's own JSON. The converters read a part of it: Gen2 reads only `sys`,
  `mqtt`, `cloud`, channels and `wifi` (`internal/configuration/gen2_converter.go:39-105`). They build a
  typed struct, so an unknown field is gone before `parseApplicableDesiredConfig` (`config_service.go:565`)
  can refuse it, and the check passes on a row that lost settings. A lossy rollback reads as a full one.
  An import already has its own route to a desired value: ginsys/shelly-manager#795 creates a template
  from it.
- A `template` row is a template's text, not what the device had. A `manual` row is a merge into the
  stored import. An `updated` row has no old side, a wrong `ConfigID` and a shape its caller chose.
- Any of them can hold real secrets. A rule that reads no stored secret needs a source that holds none
  (section 3.3).

**Why a new table and not more rows in `ConfigHistory`:** its `ConfigID` is bound to a `device_configs`
row that may not exist (`models.go:99`); its route and the card read device-JSON rows
(`service.go:616`); and a rollback row needs a field for the template ids it detached. Desired is
owned by `ConfigurationService` and its repository (`config_service.go`, `repository.go`), so the
table and its writer live there.

A row holds: the device id, an action (`recompute`, `import`, `rollback`), `old_desired` and
`new_desired` (both with the secrets already masked at write, `redact.MaskTree`), the
`previous_template_ids` of a rollback, a nullable `restored_from` (the id of the version a rollback
restored), a `changed_by` text, and when. **`changed_by` has no per-person source:** the hook sees only
`SetDeviceDesiredConfigPending(deviceID, config)` (`repository.go:260`), which takes no actor, and
neither do its recompute callers, and the only identity the API checks is the one shared admin key (`internal/api/router_routes.go:12-19`).
So it holds
the fixed text `system` for `recompute` and `import`, and `admin` for `rollback`. No caller's signature
changes. **A version is a row's `new_desired`.** The table is created with
the others by the database manager's migration (`internal/database/manager.go:122`), nullable
columns only.

**Writer rules.**

- One hook in the repository, `SetDeviceDesiredConfigPending`, plus the two import sites.
- A row only when the new text differs from the stored one. `RecomputeDesiredConfig` writes even when
  nothing changed (`config_service.go:643`), so a row per write would fill the table with duplicates.
- The row is inserted in the same transaction as the desired write
  (`NewGormConfigRepositoryOn`, `repository.go:88-96`, the handle `recomputeTemplateDevices` already
  uses, `import_store.go:364`). `createHistory` swallows its error (`service.go:911-918`); a desired
  history that can silently miss a version is not a history.
- **Deleting a device deletes its rows.** `DeleteDevice` deletes only the device row
  (`internal/database/manager.go:527-529`, called from `internal/api/handlers.go:491`), so the new rows
  would keep SSIDs and broker hosts with no owner. The delete takes them in the same transaction. The
  existing `ConfigHistory` rows of a deleted device are an orphan of the same kind today; the note
  names it and leaves it to its own issue.

**Cost.** Nothing can be rolled back to until a device's desired has changed once after the first
slice is deployed. The first row's `old_desired` is the state before it, but a version is a row's
`new_desired`, so that earlier state is not offered. Retention (how many rows, how long) is not
decided here.

### 3.2 How a rollback survives a recompute

Three ways were compared.

| | Idea | What it cannot do |
|---|---|---|
| (a) | Write the difference into the overrides | Overrides cannot delete a key a template gives (section 2.2), so the result is not "exactly the old version". |
| (b) | Detach the templates; store the whole target as the overrides | Later template edits stop reaching that device until the operator undoes it. |
| (c) | A stored pin that recompute returns | A new column that every edit path must clear; a silent override of the templates with no owner. |

**Recommendation: (b).** The recompute of a device with no templates is the merge of one layer into an
empty configuration (section 2.2), so the target survives every later recompute, **provided the merge
returns it unchanged**. It does not for a plain `omitempty` scalar field that holds a zero
(`"server":""`, `"level":0`): the merge does not copy a zero, and the field is dropped when written, so
the merged tree lacks that path. A zero in a field without `omitempty` (the `enable` of `SleepConfig` and
`BLEConfig`) is written anyway and passes. The rollback checks that before it writes: it merges the target as the only layer and compares the two **decoded JSON trees**, never their
text, so a version whose keys are in another order, such as one written by an archive import, passes. A
path the target holds that the merged tree lacks, or holds with another value, is a loss. A path only the
merged tree holds counts as equal when its value is a JSON zero (`false`, `0`, `""`, `null`, an empty
object or list), because a plain field without `omitempty` can appear as its zero value; any other extra
value is a difference. Two kinds of field are known to appear this way, and the list is not claimed
complete. The `enable` of `SleepConfig` and `BLEConfig` (`typed_models.go:216`, `:223`): Apply reads
neither, and the only other reader is the validator's presence check on `BleConfig` (`validator.go:508`).
The `id` of a channel entry (`configs.go:18`, `:123`): Apply does read it (`gen2_channels.go:219`,
`:231`), and it reads 0 from a target that has no `id` as well, so the added zero sends nothing new. A
change that makes Apply send a plain field it does not read today must revisit this rule, because an
added zero would then be sent. A loss refuses the rollback, names each path that would be lost, and
writes nothing. It adds no column to `devices` and no second mechanism. The rollback row keeps `previous_template_ids`.

**The way back is two steps: re-attach the templates and clear the overrides**
(`ClearDeviceOverrides`, `config_service.go:428`). Re-attaching alone is not harmless:
`SetDeviceTemplates` recomputes at once (`config_service.go:243-254`), the overrides win only where the
target holds a value, and a template value shows through wherever the target holds none, so desired
changes and the device is pending again. Clearing the overrides then leaves the templates alone as the
source. Each step is a recompute, so each may add a history row; neither touches the device. The
confirm dialog and the answer say so.

### 3.3 Secrets

**Rule.** A rollback keeps a *current* secret only where the place it is sent to is unchanged.
Otherwise it is refused until the operator enters the secret. A stored old secret is never restored,
even when its old destination comes back. History secrets stay out of desired.

**Why "keep the current secret" is not enough.** The restore helper takes a map secret by key and
never compares its sibling fields (section 2.3). Apply posts the MQTT form built from desired
(`internal/configuration/apply.go:327-345`). A rollback to an old broker with today's password kept
would post today's password to whatever answers at the old address; a rollback to an old SSID would send
today's Wi-Fi key to that network. That is a secret reaching a place it does not reach now.

**The stored old secret is not offered either.** The new table stores secrets masked (section 3.1), so
a rollback never reads a real secret from history. That removes the case where an old, rotated
credential is written back to a device and strands it.

**Where each secret goes.** The slots are those of `secretSlots`
(`handlers_config_templates.go:552-578`); the bound fields say where the secret is sent.

| Secret slot | Where it goes | Bound fields, compared exactly |
|---|---|---|
| `wifi.password` | the station join; Gen1 `wifi_sta.key` (`gen1_converter.go:545`) | `wifi.ssid` |
| `wifi.ap.pass` | the device's own access point; Gen1 `wifi_ap.key` (`gen1_converter.go:577-578`); it leaves nothing | `wifi.ap.ssid` |
| `wifi.ap.key` | Apply refuses it (`apply_wifi.go:296-302`); no destination can be named | none: counts as changed |
| `mqtt.pass` | the broker (`apply.go:327-345`) | `mqtt.server`, `mqtt.port`, `mqtt.user`, `mqtt.ssl_ca`, `mqtt.enable_sni` |
| `auth.pass` | the device login | `auth.user` |
| `network.wifi.sta.pass` | the station join | `network.wifi.sta.ssid` |
| `network.wifi.ap.pass` | the device's own access point | `network.wifi.ap.ssid` |

Notes on the table:

- The MQTT row goes past the fields a server-and-user rule names. `port` is part of where the broker
  is. `ssl_ca` and `enable_sni` (`internal/configuration/typed_models.go:84-85`) change how the password
  travels to the same broker. Apply cannot change them today: the Gen1 form carries only `enable`,
  `server`, `user`, `pass`, `id`, `clean_session`, `keep_alive` and `update_period` and drops every
  other key (`apply.go:383-406`), and Gen2 apply lists both as unmapped (`gen2_apply.go:188-216`). They
  are bound anyway: an exact compare costs nothing, and a later mapping must not reopen the hole.
- The Gen1 apply cannot change the access point's SSID (`apply_wifi.go:47-48`), so `wifi.ap.ssid` is
  fixed on Gen1 and the access point rows pass there. They are bound for the other generations.
- A search of the non-test files of `internal/configuration` for `.Network` finds no converter or apply
  read of that section (the validator, the typed model, the template context and the emptiness check
  at `config_service.go:747` only). The two `network.*` rows follow the same rule for the day it is
  read.

**How the check runs.**

1. Read the target (a version's masked `new_desired`) and the current desired once.
2. For every slot where the target holds a masked secret: keep the current secret only when the
   current desired holds one **and** every bound field is equal in both. A secret with no nameable
   binding counts as changed.
3. A target with no secret in a slot gets none there: replace semantics.
4. Every slot that is not kept goes on the refusal list as its JSON path.
5. **Fail closed.** After the substitution the rollback refuses if a placeholder (`redact.Placeholder`,
   `redact.go:13`) remains anywhere in the target, whatever its path, and names that path. A slot with
   no row in the table counts as changed. A test loops over every path `allSecretPaths` returns and
   fails for one with neither a binding row nor an explicit "no binding" row, so a new secret field
   cannot be added unbound. A list slot has no key to bind, so it counts as changed: ginsys/shelly-manager#381
   plans such a slot (`actions.<name>.<index>.secret_urls`); that plan is not read here.

The refusal list is **computed** from the table. The restore helper cannot give it: it drops a list
entry's secret when any other field differs, can return `ErrAmbiguousEntry`, and reports no dropped
path (section 2.3).

**The operator's way through.** A refusal the operator cannot clear is a dead end: entering the secret
through the overrides leaves the current broker or SSID in desired, so the binding still differs and
the next try is refused again. So the route takes the secrets (section 3.6). A secret entered for a
path on the refusal list is the operator's own, goes into the target and satisfies that slot.

**Cost.** A rollback across a broker or SSID change always needs the operator to type the secret.
That is the point of the rule.

### 3.4 The rollback's own write, its history row and concurrency

The rollback is one transaction: the template ids, the overrides (the whole target), `desired_config`
and the pending flag in one UPDATE as `SetDeviceDesiredConfigPending` does, and the history row
(`action = rollback`, `restored_from`, `previous_template_ids`, both desired texts masked). Today
`ConfigurationService` writes template ids, overrides and desired in separate steps
(`config_service.go:363-375`); a new repository method does the three together.

- **No apply lock.** The rollback writes no device, so it does not take the apply lock
  (`apply_service.go:387`). An apply in flight fails its compare-and-set and reports that the
  configuration changed (`apply_service.go:500-511`).
- **The rollback's write is conditional on what it read.** It is built from one read of the desired
  text, the template ids and the overrides, and from the secrets the operator entered against that
  read. The UPDATE carries the `desiredConfigMatchCondition` shape on all three
  (`repository.go:302-307`, byte-exact on MySQL). A miss answers 409, writes nothing and records no
  row. So a rollback built on an older read can never overwrite a secret the operator entered in
  between. Two racing rollbacks: one wins, the other gets 409.
- **A recompute racing a rollback.** The recompute reads at `config_service.go:594-604` and writes at
  `:643` with no check, so a rollback that commits between could be overwritten by a recompute built
  on the old inputs, and Apply would send the old desired. The recompute's write becomes conditional on
  the template ids, the overrides and the desired text it read (the same shape; the desired text so the
  history row's old side is what the write replaced). On a miss it re-reads and tries again, a
  bounded number of times (three), then returns the error. A write that changes no column is no miss:
  MySQL without `clientFoundRows` reports 0 rows for it, so a zero count is recounted before it is read as
  a miss.
- **Inside a transaction the retry does not run.** `recomputeTemplateDevices` calls
  `RecomputeDesiredConfig` inside the import transaction (`import_store.go:356-382`), where a second
  read can see the same snapshot and miss again. There a miss returns the error at once, which fails
  the import, as any recompute error does there. A test pins this: a conditional recompute write that
  misses inside a transaction returns its error and does not loop.
- **What slice 1 stores.** A row is written when the UPDATE changed a row and the two real texts differ
  byte for byte, and only then; a device deleted after the read leaves no row. An empty side, and a text
  that is not JSON and so cannot be masked, are stored NULL and never raw, so a row whose `new_desired` is
  NULL is not a rollback source and the rollback must skip it. The three text columns are unsized
  (`text` on PostgreSQL and SQLite, `longtext` on MySQL), never narrower than `devices.desired_config`.
  A device keeps its newest 50 rows (a rollback undoes recent edits; at about 5 KB a text that is about
  0.5 MB a device), pruned at insert, so a pruned row can be the one a later `restored_from` names.
- **The history row.** A rollback row names its version through `restored_from`, so the card can say
  "restored to version N" without a free-text field.

### 3.5 No device is written until Apply

The only sender is the existing Apply (`service.go:1243` to `apply_service.go:378`). The rollback
writes desired and clears `config_applied`; the device is untouched, and the page already shows the
state as Pending (`ui/src/pages/DeviceConfigPage.vue:44-45`) with the Apply Config button
(`:9-14`).

The answer and the card say, in these words: "Desired configuration set to version N. The device is
unchanged until you apply it." When secrets were entered, they are never repeated in the answer.

### 3.6 Route, permission and UI

- `GET /devices/{id}/desired-config/history` lists the versions (id, time, action, who, `restored_from`,
  and the masked `new_desired`; the text is masked again on the way out as `maskJSONText` does,
  `internal/api/device_secrets.go:35`).
- `POST /devices/{id}/config/new/rollback`, body `{history_id, secrets}`, next to the other desired
  routes (`router_routes.go:156-159`), under `admin(...)` like its neighbours (`router_routes.go:156`,
  `:158`, `:159`; apply checks the key in its own handler, `:155`). `secrets` is an optional object
  keyed by secret path (`mqtt.pass`).
- 404 for an id that is not a version of this device.
- A first call that needs secrets gets **409** with the computed list of paths to enter, echoes no
  value and writes nothing. The operator retries with `secrets` for exactly those paths. A key of
  `secrets` that is not on that list gets 400. A value that is empty or the mask gets 400. `secrets`
  values are never logged, echoed, written to the history row or put in an error.
- 409 also for the conditional write (section 3.4). No device is contacted.
- The card, next to the History card (`DeviceConfigPage.vue:228-254`): a list of versions, a Roll back
  button per version, and a confirm that shows the difference with the current desired, the templates
  that will be detached, the way back and that it is two recomputes (section 3.2), and the fields that
  need a secret. A target the merge cannot return unchanged is refused with its paths (section 3.2).

## 4. Slices, one pull request each

Each is under about 350 non-test lines.

1. **The table and the writers.** The `desired_config_history` table and its migration; the hook in
   `SetDeviceDesiredConfigPending` and the two import sites, in one transaction with the write; a row only
   on a change; deletion with the device; the conditional recompute write and its bounded retry outside a
   transaction. Red first: a recompute that changes nothing writes no row; a row's secrets are masked; a
   conditional write that misses inside a transaction returns its error and does not loop; a device
   delete takes its rows.
2. **The service rollback, the list route and the rollback route.** The transaction, the conditional
   write, the secret table and the computed refusal list. Red first: replace semantics (a key the version
   lacked is gone, template ids detached); a secret kept only when its bound fields are equal; a refusal
   names every path and echoes no value; a placeholder left anywhere refuses; the loop over `allSecretPaths`
   finds a row for each; 404 for another device's version; a non-admin is refused; a rollback built on a
   stale read gets 409; a target with a non-zero plain scalar field (`system.sntp.server`) passes, and one
   with a zero in a plain `omitempty` field (`"server":""`) is refused with that path and writes nothing; after a rollback, a recompute leaves
   desired equal to the target; a version written by a recompute always passes the round-trip check; an
   import-written version that differs only in key order passes it; a path only the merged tree holds, with
   a zero value, passes, and with any other value is refused.
3. **The card on `DeviceConfigPage`.** Versions, the confirm, the secret prompts, the notice.
4. **Retire `ui/src/pages/DeviceConfigHistoryPage.vue`.** Re-point the link at `DeviceDetailPage.vue:142`
   to the card and remove the route at `ui/src/router/routes.ts:28-30`, as the comment on the issue asks.

## 5. Decisions needed

None is expected to reach Serge: the secrets rule tightens a control that today would send a current secret
to an old address and loosens none.

Candidates, each with one recommendation:

1. **Detach the templates on rollback** (section 3.2). Recommended: yes, shown in the confirm, with the
   way back.
2. **Retention of the desired history** (section 3.1). Decided in slice 1: the newest 50 rows per device
   (section 3.4, "What slice 1 stores").

## 6. What this does not solve

- No code, OpenAPI or UI change, and no device is touched.
- Versions that existed before the first slice is deployed: they are not recorded and cannot be rolled
  back to.
- The orphaned `ConfigHistory` rows of a deleted device.
- A zero in a plain `omitempty` scalar field (`"server":""`, `"level":0`): the merge does not copy it, so a version holding one
  cannot be rolled back to, and the refusal says which path.
- Restoring the stored `device_configs` copy: it is a record of the last import, and the issue's old
  Restore button is not rebuilt.
- A list slot among the secrets (ginsys/shelly-manager#381): counted as changed until its plan is read.
- Retention, and a history of the typed configuration.
