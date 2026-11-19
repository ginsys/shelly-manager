# Design note: bulk apply and bulk download of device configuration

Status: proposal. It decides nothing; the last section lists the decisions and one recommendation
for each. No code, OpenAPI or UI file changes with this note.

It covers two open issues that depend on the same routes and the same dialog:

- ginsys/shelly-manager#594: apply a JSON the operator gives to the selected devices.
- ginsys/shelly-manager#595: download the configuration of the selected devices.

All `file:line` references are at origin/main `37945fcfb1d4`.

## 1. Stale premises in the two issues

The bodies of ginsys/shelly-manager#594 and ginsys/shelly-manager#595 are stale. Read them with
these corrections:

1. **"Bulk export" no longer exists.** ginsys/shelly-manager#604 retired export-to-device in favour
   of the apply and closed ginsys/shelly-manager#592. Only three bulk routes act on devices named by id today:
   `POST /config/bulk-import`, `/config/bulk-drift-detect` and `/config/bulk-drift-detect-enhanced`
   (`internal/api/router_routes.go:104-106`). None of them writes to a device
   (`docs/api/openapi.yaml:2142`, `:2172`). A fourth route, `POST /config/bulk-validate`
   (`router_routes.go:101`, `internal/api/typed_config_handlers.go:452`), takes configurations in
   the body and names no device. The UI says the same about the three
   (`ui/src/api/bulk.ts`, header comment).
2. **The dialog does not promise what the issues say.** It offers only `import`, `drift` and
   `drift-enhanced` (`ui/src/components/bulk/BulkOperationsDialog.vue:259`). It has no field for
   typed JSON. Its "Download Results" button saves the bulk *result* as `bulk-<operation>-result.json`
   (`BulkOperationsDialog.vue:311-322`), not device configuration.
3. **ginsys/shelly-manager#592 did not make a bulk write go through the apply lock.** It removed the
   write path. A bulk write that uses the apply lock is new work, and this note proposes it.
4. **There are two secret-masking rules, not one** (section 3.4).

The dialog is mounted in the Devices page (`ui/src/pages/DevicesPage.vue:171-173`) and receives the
selected devices as `selectedDevices`.

## 2. What exists today

### 2.1 The bulk contract

- Body: `{"device_ids":[...], "options":{"stop_on_error":false}}`
  (`internal/api/bulk_request.go:16`, `:27-33`). Unknown fields are refused, only one JSON value is
  read, the body is limited to 1 MiB (`:21`), ids must be positive, and a repeated id runs once in
  first-seen order (`:48-76`).
- **There is no limit on the number of devices.** Only the body size is limited.
- `runBulkConfig` runs the devices one after another in one HTTP request and reports one entry per
  device (`bulk_request.go:84-131`). An id that names no device is that device's `failed` entry.
  With `stop_on_error`, the devices after the first failure are `skipped` and are not contacted.
- Statuses are `success`, `failed`, `skipped` (`internal/configuration/models.go:144-146`).
- A failed entry carries `err.Error()` as it is (`bulk_request.go:110`). The per-device apply
  route does not do this, on purpose (section 2.3).
- Authorization: the bulk routes are registered *without* the `admin` wrapper
  (`router_routes.go:104-106`). Each handler calls `h.requireAdmin` first, "before any parsing or
  device contact" (`internal/api/handlers.go:897-901`). `requireAdmin` (`handlers.go:107-116`)
  returns true when no admin key is set, otherwise it needs the key in `Authorization` or
  `X-API-Key`. **With no admin key set, every guarded route is open.** The tests are
  `internal/api/bulk_auth_test.go:28` and `internal/api/bulk_contract_test.go:122`.

### 2.2 The UI side

- `ui/src/api/bulk.ts` types the contract (`BulkRequest`, `BulkConfigStatus`, `BulkDriftStatus`).
- Every call uses the shared axios client, which has `timeout: 10000` (10 s) and no per-call
  override (`ui/src/api/client.ts:10`). The dialog calls with `device_ids` from the selection and
  `stop_on_error` (`BulkOperationsDialog.vue:283-292`).
- A bulk call that runs longer than 10 s is cut off in the browser while the server goes on. For a
  read this is a nuisance. For a write it is the dangerous case: the operator sees an error and the
  devices may still be written.

### 2.3 The per-device apply

`POST /devices/{id}/config/new/apply` (`router_routes.go:153`, handler `ApplyDeviceNewConfig`,
`internal/api/handlers_config_device.go:529-556`) does, in order: the admin check, the device lookup,
`ShellyService.ApplyDesiredConfig` (`internal/service/service.go:1209`), then
`ConfigurationService.ApplyDesiredConfig` (`internal/configuration/apply_service.go:378`). That
function:

1. takes one apply per device across all manager replicas (`lockApplyAcrossReplicas`; a second apply
   waits at most `DefaultApplyLockWait`, 15 s, `internal/configuration/apply_lock.go:32`, and then
   fails with `ErrApplyInProgress`);
2. snapshots the stored desired configuration;
3. refuses settings the device model does not support (`ErrUnsupportedSettings`);
4. reads the device MAC and compares it with the stored one **before any write**
   (`CheckDeviceIdentity`, `internal/configuration/apply_identity.go`; `ErrDeviceIdentity`);
5. clears `config_applied` before the first write;
6. writes, reads back, and sets `config_applied` only when the read-back matches and the desired
   configuration did not change meanwhile (compare-and-set).

The answer is `ConfigApplyResponse` (`handlers_config_device.go:46-56`): `success`, `applied_count`,
`failed_count`, `requires_reboot`, `unconfirmed`, `failures`, `warnings`. An *unconfirmed* apply
(a Wi-Fi write changed the network and the device stopped answering) is neither success nor failure
and the device stays pending. A refused write names the setting only: "the device's own error text can
echo the value it was sent, such as the MQTT password", so it goes to the log, not to the caller
(`applyFailureMessages`, `handlers_config_device.go:579-595`). Read-back differences pass through
`redactDifference` (`internal/api/config_diff_redact.go:78`), but they still print the expected and
the actual value (`verify %s: expected %v, device has %v`, `handlers_config_device.go:586-590`), with
secrets masked by the divergent rule of section 3.4. So `applyFailureMessages` is not a setting-only
formatter for read-back differences.

A fourth outcome has no failure and no unconfirmed flag: if the desired configuration changed while
the apply ran, the write and read-back may succeed, but `config_applied` is not set and the response
carries a warning, "The desired configuration changed while it was being applied; the device stays
pending." (`apply_service.go:500-510`; test `apply_service_test.go:1000-1017`.)

Errors map to HTTP in `writeConfigDeviceError` (`handlers_config_device.go:597-618`): 501 unsupported
generation, 422 unsupported settings, 409 wrong device at the address, 409 apply in progress,
otherwise `writeDeviceReadError` (502 for a rejected stored login).

### 2.4 What a device applies: desired configuration

The apply writes the *desired configuration*, which is the merge of the device's templates and its
overrides (`RecomputeDesiredConfig`, `internal/configuration/config_service.go:578-637`). Overrides
belong to one device. Saving them (`SetDeviceOverrides`, `config_service.go:360`;
`PatchDeviceOverrides`, `:402`) stores them, recomputes that device's desired configuration and sets
`config_applied` to false. A template edit instead recomputes *every device that uses the template*
(`RecomputeAffectedDevices`, `config_service.go:662`).

The override routes are `GET|PUT|PATCH|DELETE /devices/{id}/overrides/new`
(`router_routes.go:146-149`). They validate the body against the device model
(`writeOverrideFieldErrors`, `handlers_config_device.go:288`; `ValidateConfigBody`,
`internal/configuration/schema_validate.go:30`, which "checks a stored configuration body", not the
merged result), keep a secret sent back as the mask (`restoreMaskedSecrets`,
`handlers_config_templates.go:601`) and treat `null` as "remove this override"
(`PatchDeviceNewOverrides`, `handlers_config_device.go:386`).

## 3. Common ground

### 3.1 What stops a bulk call writing to a device the operator did not select

Five controls. Controls 1, 3 and 4 exist today in the code the proposal reuses. Controls 2 and 5 are
proposed constraints on the new route and dialog.

1. **Explicit ids only.** The request names devices by `device_ids`. The parser has no tag, template,
   group or "all" field and refuses unknown fields (`bulk_request.go:48-52`). The test
   `TestBulkRoutes_ActOnlyOnTheNamedDevices` (`internal/api/bulk_contract_test.go:122`) pins this for
   the existing routes.
2. **The JSON is never a template.** Overrides are stored per device. A template edit recomputes the
   stored desired configuration of every device that uses the template and marks each one pending
   (section 2.4), including devices that are not selected; a later apply then writes it to those
   physical devices. So the operator's JSON must become device overrides and nothing wider.
3. **Identity check per device.** The apply compares the MAC of the device that answers at the stored
   address with the stored MAC before the first write (section 2.3, step 4). A stale IP that now
   leads to another device is refused, and nothing is written.
4. **Admin check first**, before parsing and before any device contact (section 2.1).
5. **Proposed: the dialog shows what it will write.** Before the call, the dialog lists the selected
   devices by name and MAC and the JSON, and asks for a confirmation. ginsys/shelly-manager#657
   already makes the page keep its rows and re-check the selection when the dialog opens.

What these controls do **not** solve: an operator who selects the wrong devices, and a stored MAC that
is itself wrong. With no admin key set, anyone who can reach the API can call the route.

### 3.2 Authorization

Use the current model: the handler calls `h.requireAdmin` first, as the three bulk routes do. No
new key, role or scope. This note proposes no change to the admin-key model. If the lead wants a
separate permission for fleet writes, that is a change to an authorization control and needs its own
decision.

### 3.3 Fleet limits

No measurement of time per device exists, so no number in this note is derived from a measurement.
What is known from the code: one apply may wait up to 15 s for the apply lock and then run its own
device I/O, and the browser gives up after 10 s (section 2.2).

### 3.4 Masking: two rules, one to reuse

- `internal/redact` is the shared rule. `IsKey` marks a key as secret when its lowercase name
  contains `pass`, `pwd`, `psk`, `secret` or `token`, or ends in `key`
  (`internal/redact/redact.go:15-26`). `Placeholder` is `********` (`redact.go:13`). Its own comment
  says the API routes and the exports share it.
- `internal/api/config_diff_redact.go:10-14` has a different rule: `pass`, `psk`, `secret`,
  `token`, `key` anywhere in the name (no `pwd`), and a different placeholder, `[redacted]`.
- The device configuration routes mask in two different ways today. `GET /devices/{id}/config` and
  `/config/current` mask with `maskDeviceConfig` (`internal/api/device_secrets.go:126`).
  `GET /devices/{id}/desired-config` **removes** secrets from `config` (`redactSecrets` sets each
  secret to `""`, `handlers_config_templates.go:592`, and the string slot turns that into a missing
  field, `:539-542`; the secret fields have `omitempty`, `internal/configuration/typed_models.go:45`,
  `:64-65`, `:74`, `:92`, so the field is absent from the JSON) and **masks** them in `inherited`
  (`maskSecrets`, `:583`, with `secretMask = redact.Placeholder`, `:515`; both calls in
  `handlers_config_device.go:487-490`).

Recommendation: the new routes use `internal/redact`. The second rule
(`config_diff_redact.go`) is a separate follow-up and this note does not change it.

## 4. ginsys/shelly-manager#594: bulk apply

### 4.1 What is the operator's JSON?

| Option | What it does | Does NOT solve |
|---|---|---|
| A. A partial device configuration, applied as overrides with PATCH rules | Same shape as `PATCH /devices/{id}/overrides/new`. Omitted fields stay, `null` removes an override. Per-device. | The operator must know the schema. A field a model lacks is refused for that device only. |
| B. The same JSON, with PUT rules (replace the overrides) | The device ends with this override set, except that a secret left out of the JSON is kept as stored (`keepOmittedSecrets`, `handlers_config_device.go:345-361`). | One typo drops every other non-secret override on every selected device. Larger blast radius. |
| C. A template | Shared by design. | **Changes the stored desired configuration of devices that are not selected** and marks them pending (section 3.1, control 2). Not acceptable for this feature. |
| D. A full device configuration for the device | Matches the device's own settings. | The apply works from the desired configuration (templates plus overrides). A full document would have to be turned into overrides: a new mapping the repo does not have. |

Recommendation: **A.** It reuses the validation, the secret handling and the `null` rule of the
existing override route.

### 4.2 Which route owns the write?

Candidate: the per-device apply (section 2.3). It owns the lock, the identity check, the read-back
and the unconfirmed state. A bulk call must not write to a device by any other path.

Proposed route: `POST /api/v1/config/bulk-apply`. For each device, in order:

1. validate the JSON against that device's model (`ValidateConfigBody`);
2. patch that device's overrides (`PatchDeviceOverrides`, which recomputes the desired configuration);
3. call `ApplyDesiredConfig`.

What the bulk call adds to the per-device route, and nothing more: the device list; one validation
pass over every selected device *before the first write*; ordered per-device results in one answer;
the `stop_on_error` option. It adds no new write path.

Request: `{"device_ids":[...], "config":{...}, "options":{"stop_on_error":false}}`, with the existing
body rules (1 MiB, unknown fields refused, duplicates dropped). The `config` object is the JSON from
4.1.

Does NOT solve: a bulk call is not atomic. Device 3 can be applied while device 4 is refused. There is
no rollback across devices (4.4).

### 4.3 Result shape

One entry per device, in request order, with a status the operator can act on:

| Status | Meaning | Where it comes from |
|---|---|---|
| `applied` | Written and read back equal. | `ConfigApplyResponse.success` |
| `pending` | Written and read back, but the desired configuration changed during the apply, so the device stays pending. The entry keeps the warning. | `success` false, no failures, a warning (section 2.3) |
| `unconfirmed` | Written; the device stopped answering. Not verified, not refused; stays pending. | `unconfirmed` |
| `refused` | Nothing was written: invalid JSON for the model, unsupported setting, wrong device at the address, apply already running. | `ErrUnsupportedSettings`, `ErrDeviceIdentity`, `ErrApplyInProgress`, validation |
| `failed` | The call did not end in a recorded, verified write, for a reason that is not a refusal before the write: a write the device refused, a read-back difference, a device that rejected the stored login, a storage error while saving the overrides (4.4), or a storage error after a verified write, when the applied flag could not be saved ("configuration applied but could not record it", `apply_service.go:500-503`; the device was written and read back equal, and stays pending). Reason is one fixed text per cause, such as "the device refused a write", "the device rejected the stored login", "could not store the configuration", "applied, but could not record it". | `failures`; an auth error (`handlers.go:774-780`); an error from `SetDeviceOverrides`; an error from the apply after the read-back |
| `skipped` | Not contacted because of `stop_on_error`. | as today |

Each entry has `device_id`, `device_name`, `device_ip`, `status`, a short `reason`, the `warnings` and
a `failures` list. **The reason and the failures name the setting, never the device's own error text
and never a value.** Two things follow from section 2.3. The new route must not copy the
`entry.Error = err.Error()` line of `runBulkConfig`, because a device error can echo a secret. And it
cannot reuse `applyFailureMessages` as it is, because that function prints values for read-back
differences; the route needs a small formatter that gives the setting path only. Each entry also says
whether the overrides were stored (`stored`), which 4.4 needs.

How the per-device route reports each case, and how the bulk entry maps it. A write the device
refused and a read-back difference are *results*: the per-device route answers 200 with them in
`failures` (`configApplyResponse`, `handlers_config_device.go:549-555`, `:561-590`), and they become
`failed`. Five conditions are *handler errors* with their own HTTP code: a body that fails validation
(400, `handlers_config_device.go:320-324`), a wrong device at the address (409), an apply already
running (409), unsupported settings (422), and an unsupported generation (501)
(`writeConfigDeviceError`, section 2.3). They become `refused`. A device that rejects the stored login
is a 502 (`handlers.go:774-780`) and becomes `failed` with the reason "the device rejected the stored
login". In the bulk answer all of these are a status inside a 200 answer, as bulk-import does today.
The operator sees, per device: what happened, whether the device was touched, and whether it is still
pending.

OpenAPI: the status list is new. The existing `BulkConfigResult` has `success|failed|skipped` and
cannot carry `pending`, `unconfirmed` and `refused`, so the new route needs its own result schema.

### 4.4 What stays stored when the apply does not succeed

The route writes the overrides first, then applies. A successful `PatchDeviceOverrides` stores them,
recomputes the desired configuration and marks the device pending (section 2.4). The table assumes the
patch succeeded; the last row is the case where it did not. For each outcome:

| Outcome | Overrides after the call | Device after the call | Operator sees |
|---|---|---|---|
| `refused` before the write (identity, in progress, unsupported) | New overrides **stored** | Untouched; pending | `refused`, `stored: true` |
| `refused` because the JSON is invalid for the model | Not stored (validation comes first) | Untouched | `refused`, `stored: false` |
| `failed` | New overrides stored | Partly written, not written, or fully written and verified but not recorded (the applied flag could not be saved); pending | `failed`, `stored: true`, setting names or the fixed reason |
| `pending` | New overrides stored | Written and read back; pending | `pending`, `stored: true`, the warning |
| `unconfirmed` | New overrides stored | Written, not verified; pending | `unconfirmed`, `stored: true` |
| the patch itself fails (storage error) | Depends on which write failed. `SetDeviceOverrides` makes three separate writes (`config_service.go:366-371`, `:628-633`; `repository.go:255-264`): the overrides, then the desired configuration, then the applied flag. If the first fails: old overrides, old desired configuration, old flag. If the second fails: new overrides, old desired configuration, old flag. If the third fails: new overrides, **new desired configuration, old flag**, so a device that was applied can still read as applied although its desired configuration changed | Not contacted | `failed`, reason "could not store the configuration", `stored: unknown` |

For the last row the route never reports the device as pending or applied. The operator re-reads the
overrides and the status route, and sends the call again; a repeat patch of the same JSON rewrites all
three values. This is rare (a database error) and the note proposes no repair for it. The third-write
case is tracked in ginsys/shelly-manager#716.

Option 1 (recommended): **keep the new overrides stored** and report `stored: true`. They are the
operator's intent. The device shows as pending in the status route
(`GET /devices/{id}/config/new/status`), and a later per-device apply or a retry of the same bulk call
finishes the job. It adds no second write path.

Option 2: restore the previous overrides when the apply is refused or fails. It needs the previous
overrides read before the patch and a second write after a failure. That restore can itself fail, and
for `unconfirmed` there is no safe answer: the device may already carry the new values. The device
would then disagree with the stored overrides.

Does NOT solve with option 1: a device can stay pending with overrides the operator no longer wants.
The operator corrects it with the per-device override routes or a second bulk call.

Validation scope: `ValidateConfigBody` checks the *typed JSON* against the device model, not the
merged result (`schema_validate.go:20-30`). The unsupported-settings check in the apply runs on the
*merged desired configuration*, after the overrides are stored (`apply_service.go:378`, `:411`). A true
pre-check of the merged result needs a merge that does not write; `RecomputeDesiredConfig` always
writes (`config_service.go:578-637`) and no dry variant exists.

- First slice: validate the typed JSON per device before the first write. A merged-result problem
  appears as `refused` at the apply, with `stored: true`.
- Later: add a dry merge so the pre-check covers the merged result. This is new code in the
  configuration service.

### 4.5 A limit on devices per call

No number is derived from a measurement: time per device is not measured. A synchronous call can run
for the sum of the per-device times, each up to the 15 s lock wait plus device I/O, and the browser
cuts the request at 10 s (section 2.2).

| Option | Does NOT solve |
|---|---|
| A. A small hard cap (proposal: 10 devices; over the cap is a 400 that names the cap), with a longer per-call timeout in the UI | The 10 is a proposal, not a measurement. A slow device can still exceed any timeout. |
| B. No cap, the UI sends in chunks | A direct API caller can send any number. A long call can still be cut. |
| C. An asynchronous job: the call answers 202 with a job id, the UI polls | New state, storage and a job lifecycle. The largest change. |

Recommendation: **A for the first slice**, with the cap as one named constant, and **C as the next
slice** when the fleet size or measured times show A is too small. The first real runs on fake devices
give the time per device (merge fast, fix fast).

### 4.6 One device fails: continue or stop?

Keep `stop_on_error` as the bulk contract has it (`bulk_request.go:27-33`, `:99`), default `false`.
For a write, `true` is the safe choice when the JSON is risky, and `false` is the useful choice when
one device is simply offline. The dialog already has the checkbox. Skipped devices are not contacted
and are reported `skipped`. The option does NOT undo devices already written.

### 4.7 What the UI dialog sends

- A new operation `apply` with a JSON text area for `config`.
- It parses the JSON in the browser first and shows a parse error before any call.
- A confirmation step lists each selected device by name and MAC, and shows the JSON with secrets as
  they will be sent. The operator confirms once.
- It calls `POST /config/bulk-apply` with `{device_ids, config, options:{stop_on_error}}`.
- It uses a per-call timeout longer than 10 s (section 4.5) and shows each status with the tone helper
  already in `ui/src/api/bulk.ts` (`BulkTone`, `viewOf`).
- Secrets in the answer are never echoed. A secret in the operator's JSON goes to every selected
  device and is stored in each device's overrides; the stored value is masked by the existing
  override read routes.
- "Download Results" keeps saving the result.

Does NOT solve: a double click or a retry after a browser timeout can send the same JSON twice. A
second identical call repeats the writes to every device. This note did not check whether a repeat
is harmless for each setting. The first slice does not guard against it.

### 4.8 Smallest first slice for ginsys/shelly-manager#594 that ships alone

`POST /config/bulk-apply` with option A (partial config as overrides), the cap in 4.5, continue by
default, the status table in 4.3 with `stored`, and `requireAdmin` first. Tests, all on fake devices
(no live device, no live fleet):

- admin refusal before any parsing (as `bulk_auth_test.go:28`);
- the call acts only on the named devices (as `bulk_contract_test.go:122`);
- a refused identity writes nothing;
- a device error text never appears in the answer;
- over the cap is a 400 and no device is contacted;
- `stop_on_error` skips the rest.

The UI dialog change can follow in its own slice, because the route is usable from the API without it.

## 5. ginsys/shelly-manager#595: download the configuration of selected devices

### 5.1 What is downloaded?

| Source | Existing read | Contacts the device? | Does NOT solve |
|---|---|---|---|
| Desired (templates plus overrides) | `GET /devices/{id}/desired-config` (`handlers_config_device.go:460`) | No: a database read | It is what the manager *wants*, not what the device *has*. |
| Stored baseline (last import) | `GET /devices/{id}/config` (`handlers.go:746`) | No | Can be old. |
| Live | `GET /devices/{id}/config/current` (`handlers.go:785`) | **Yes**: one device read per device | An offline device has no data. A fleet read at once is slower and can hit the 10 s timeout. |

Recommendation: **desired configuration for the first slice.** It needs no device contact, so it
has no fleet risk and cannot fail on an offline device. Add a `source` field later (`desired`,
`stored`, `live`) once a live fleet read has its own timeout answer (4.5, option C).

### 5.2 File format

One JSON file, `Content-Disposition: attachment; filename="..."`, as the existing downloads do
(`internal/api/sync_handlers.go:687`, `:711`). The file is an object with a header (`source`,
`generated_at`) and one entry per device:
`{"device_id", "device_name", "mac", "status", "config", "error"}`. The route is `POST
/api/v1/config/bulk-download` with the device list in the body, because a device list does not fit a
query string and the contract already reads ids from a body. It takes no options.

Alternatives and what they do not solve:

- A ZIP with one file per device: easy to hand to a person, but the manager then builds an archive
  in memory and a name per device. Later.
- The SMA archive (`docs/guides/sma-format.md`): an integrity-protected exchange format with its own
  import rules. This note did not evaluate it for this purpose; the choice is left to the lead.

### 5.3 Secrets

- Existing rule to reuse: `internal/redact` (section 3.4). The download must not carry a real secret.
- **The masking helper depends on the configuration type.** The desired configuration is a
  `DeviceConfiguration`, which has no `raw` part (`internal/configuration/device_config.go:3-20`;
  `GetDesiredConfig` returns it, `config_service.go:442`), so `maskSecrets` over its fixed list of
  secret slots (`handlers_config_templates.go:552-595`) covers it. The typed configuration is a
  different type, `TypedConfiguration` (`internal/configuration/typed_models.go:14`), and it has
  `raw`, arbitrary JSON "for unsupported settings" (`:38`), which the slot list does not hold. The
  typed routes mask it with `maskTypedSecrets` (`internal/api/typed_config_secrets.go:48-52`, which
  also masks `raw` through `maskRawJSON`; called at `typed_config_handlers.go:112`, `:164`, `:248`).
  So a download built on the desired configuration uses `maskSecrets`, and one built on the typed
  configuration must use `maskTypedSecrets`, never `maskSecrets` alone. A gap in an existing read
  route is not assumed; decision 12 proves it by test for every route the download reads.
- **Which masking?** `desired-config` omits secrets from `config` and masks them in `inherited`
  (section 3.4). Recommendation: **mask** (`maskSecrets`, `********`), not omit. A masked secret says
  "this secret is set", while an omitted one cannot be told from a secret that was never set. The
  override routes accept the mask back and keep the stored secret
  (`restoreMaskedSecrets`, `handlers_config_templates.go:601`). An omitted secret is also kept by the
  PUT route (`keepOmittedSecrets`); only an explicit `""` clears a secret there
  (`handlers_config_device.go:344-346`). So the reason for masking is clarity for the operator, not
  protection against clearing: a file with omitted secrets would not clear them. The PATCH rule for an
  explicit empty string was not checked for this note.
- Which fields: the ones `redact.IsKey` names, as the shared walker applies them
  (`internal/redact/walker.go`). This note adds no list of its own.
- An unmasked download is a change to a security control. It is **not** proposed and is not part of any
  slice here. If wanted, it goes to the lead as its own question.

Does NOT solve: a masked file cannot restore a device after loss, because the secrets are not in it.

### 5.4 A device that cannot be read

For desired configuration the possible causes are: an id that names no device, a database or decoding
error, and a device with no desired configuration. The last one needs care. The read
`GetDesiredConfig` returns an *empty configuration without an error* when the stored text is `""` or
`"{}"` (`internal/configuration/config_service.go:442-454`). `ErrNoDesiredConfig`
(`config_service.go:22-25`) is raised by the verify path (`apply_service.go:534`) and by
`RequireDesiredConfig` (`config_service.go:547-554`), not by that read. So a download built on
`GetDesiredConfig` alone would show an empty `config` for a device that has none, which looks like a
device with no settings. To report it, the download route calls `RequireDesiredConfig` first.

The file then holds an entry with `status: "error"`, no `config` and a short fixed `error` text such
as "device not found", "no desired configuration" or "could not read the configuration". The call
itself answers 200, as bulk-import does for a failed device. A fixed text, never `err.Error()`, for the
reason in section 2.3. With a live source (later) an offline device gets the same entry; the manager
does not retry.

### 5.5 Authorization

`requireAdmin` first, as the bulk routes do. Desired configuration, even masked, is a map of the
whole fleet, so the route must not be open to an unauthenticated caller when an admin key is set.

### 5.6 Smallest first slice for ginsys/shelly-manager#595 that ships alone

`POST /config/bulk-download`, desired configuration, masked, one JSON file, the same device cap as
4.5, admin check first. No device contact, so it ships before ginsys/shelly-manager#594 and carries no
fleet-write risk. The UI adds a "Download configuration" action next to "Download Results"; that
dialog change can follow in its own slice.

## 6. Effect on the OpenAPI file (`docs/api/openapi.yaml`)

- ginsys/shelly-manager#594: a new path `/api/v1/config/bulk-apply`; a new request schema (`device_ids`, `config`,
  `options`, referencing the existing `BulkDeviceRequest` rules); a new result schema with the six
  statuses and `stored`. The comment at `:95-96`, which lists "bulk-import, bulk-drift-detect,
  bulk-drift-detect-enhanced" as the bulk routes that "act on the named devices only", gains the new
  route. The route-specific statements "It never writes to a device" (`:2142`, `:2172`) stay correct
  for their own routes and are not touched. `BulkConfigResult` (`:122`) stays for bulk-import.
- ginsys/shelly-manager#595: a new path `/api/v1/config/bulk-download`; the answer is `application/json` with a
  `Content-Disposition` header; no change to existing schemas.
- Both: `security` is `BearerAuth` and `ApiKeyAuth`, as the bulk routes have
  (`openapi.yaml:2136-2166`); a 400 for the cap and for a bad body.

## 7. Order of work

1. ginsys/shelly-manager#595 slice (read only, no fleet risk).
2. ginsys/shelly-manager#594 route slice, with fake-device tests.
3. UI slices for both.
4. Later: a dry merge for pre-checking, an asynchronous job, a `source` field.

## 8. Decisions needed

Each has one recommendation. The lead decides.

1. **What is the JSON for ginsys/shelly-manager#594?** Recommend: a partial device configuration applied as overrides with
   PATCH rules (4.1, option A). Never a template.
2. **Which route owns the write?** Recommend: a new `POST /config/bulk-apply` that calls the existing
   per-device apply per device and adds no new write path (4.2).
3. **Result statuses?** Recommend: `applied`, `pending`, `unconfirmed`, `refused`, `failed`,
   `skipped`, with setting names only (a new setting-only formatter, not `applyFailureMessages`) and a
   `stored` flag (4.3).
4. **What stays stored when the apply does not succeed?** Recommend: the new overrides stay stored and
   the entry says `stored: true`; the device stays pending when the patch succeeded and the apply did
   not (4.4, option 1). A storage error in the patch itself is the last row of the 4.4 table and has
   no such guarantee. **And what does validation
   cover?** Recommend: the typed JSON per device model before the first write, in the first slice; a
   dry merge for the merged result later.
5. **A device limit and time.** Recommend: a hard cap of 10, a named constant, with a longer UI
   timeout, as a proposal that no measurement supports; an asynchronous job as the next slice (4.5).
6. **One device fails.** Recommend: continue by default, keep `stop_on_error` (4.6).
7. **What the dialog sends.** Recommend: `{device_ids, config, options}` after a confirmation step
   that lists device names and MACs (4.7).
8. **What ginsys/shelly-manager#595 downloads.** Recommend: desired configuration first; `source` later (5.1).
9. **File format.** Recommend: one JSON file with an entry per device, from a `POST` route
   (5.2). The SMA archive is not evaluated here.
10. **Secrets in the download.** Recommend: masked, not omitted, with `maskSecrets` over the desired configuration
    (`maskTypedSecrets` if the typed configuration is ever the source); no unmasked option (5.3).
11. **An unreadable device.** Recommend: an entry with `status: "error"` and a fixed text; the call
    answers 200; the route calls `RequireDesiredConfig` so an empty configuration is not mistaken
    for a device with no settings (5.4).
12. **The second masking rule** (`internal/api/config_diff_redact.go`) **and a test of the masking.**
    Recommend: a separate follow-up issue that moves the diff rule to `internal/redact`, and a test
    that proves every route the download reads masks every secret, including `raw` where the type has
    it; neither is part of either slice. A gap in an existing route is not assumed. If the test finds
    one, it is a change to a masking control and goes back to the lead.
13. **Order.** Recommend: the ginsys/shelly-manager#595 slice, then the ginsys/shelly-manager#594 route
    slice, then UI (section 7).

If the lead wants an unmasked download, a separate fleet-write permission, or a change to how the
apply lock works, the design needs a change to a security control and goes back to the lead before any
build.

## 9. What this note does not solve

- It measures nothing: the cap of 10 and the timeout are proposals, not results.
- It does not make a bulk write atomic, and it does not roll back devices already written.
- It does not close the open admin-key gap: with no key set, every route in this note is open.
- It does not change the second masking rule, the apply lock or any authorization.
- It does not repair the last write of `RecomputeDesiredConfig` failing after the overrides and the
  desired configuration were stored (ginsys/shelly-manager#716).
- It does not test that every route the download reads masks every secret; decision 12 proposed the
  test, and the download slice added it (`TestBulkDownload_ReturnsNoSecretValue`).
- It does not evaluate the SMA archive or a ZIP format.
- It builds nothing. ginsys/shelly-manager#594 and ginsys/shelly-manager#595 stay open until built.
