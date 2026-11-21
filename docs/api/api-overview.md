# Shelly Manager API Overview Report

## Executive Summary

The Shelly Manager exposes **112+ REST API endpoints** organized across 6 handler modules, with a comprehensive security middleware stack and standardized response format. The API follows REST conventions with a single version (`/api/v1`) and supports real-time WebSocket communication for metrics.

---

## API Base Structure

| Base Path | Purpose |
|-----------|---------|
| `/healthz`, `/readyz`, `/version` | Health & status checks (no auth) |
| `/api/v1/*` | All protected API endpoints |
| `/metrics/*` | Metrics and monitoring endpoints |

**Admin key.** If the server has an admin key configured (`security.admin_api_key`), every `/api/v1` route
(the CORS `OPTIONS` preflight answer aside) needs it, as `Authorization: Bearer <key>` or `X-API-Key: <key>`, and answers 401 `UNAUTHORIZED` without it.
The five provisioner agent routes also accept the scoped agent key (`security.agent_api_key`), described with them below. Two routes stay open by design: `GET /api/v1/version` and `GET /api/v1/metrics/prometheus` (the scrape
endpoint). The device and tag routes, the notification routes, the export plugin reads,
`GET /api/v1/provisioning/status` and `GET /api/v1/dhcp/reservations` need the key too
(ginsys/shelly-manager#611); they were open before. If no admin key is configured, every route is open.
**A write request with no key at all is refused before its body is read.** With an admin key set, a `POST`, `PUT`,
`PATCH` or `DELETE` that has a body and sends neither `Authorization: Bearer` nor `X-API-Key` answers 401
`UNAUTHORIZED` ("Authorization required") at once, so a malformed JSON body no longer draws a 400 first
(ginsys/shelly-manager#720). The limits: a request that sends a wrong key still has its JSON body checked before the
route refuses it, as does a `GET` with a body, and the header, content-type and query checks still run first, so
a request that fails one of them can still answer 400 without the key. Nothing changes when no admin key is
configured.
**The web UI sends the key only when it holds one** (entered in its key dialog and kept for the browser tab, a
build-time `VITE_ADMIN_KEY`, or a `window.__ADMIN_KEY__` set by hand): with a key set on the server and none in the
UI, the device list answers 401 and the UI opens the key dialog.

---

## Endpoint Categories

### 1. Health & Version (3 endpoints)

| Method | Endpoint | Description | Input | Output |
|--------|----------|-------------|-------|--------|
| GET | `/healthz` | Liveness probe | - | `{"status": "ok"}` |
| GET | `/readyz` | Readiness probe | - | `{"status": "ready"}` |
| GET | `/version`, `/api/v1/version` | API version info. The UI uses the `/api/v1` path. | - | Standard envelope; `data` holds `api_version`, `server_started_at`, `git_sha` (the revision the server was built from, empty when the build has none) and, when known, `database_provider_name` and `database_provider_version` |

---

### 2. Device Management (8 endpoints)

| Method | Endpoint | Description | Input | Output |
|--------|----------|-------------|-------|--------|
| GET | `/api/v1/devices` | List all devices | Query: `page`, `page_size` | Paginated device list |
| POST | `/api/v1/devices` | Add new device. When `settings` holds no `gen`, the server asks the device at `ip` once (up to 2 s) and stores the generation it reports; a device that does not answer, is not a Shelly, reports a generation below 1, or has an address that is not a valid URL host is stored as Gen 1. `PUT /devices/{id}` keeps the stored `gen` and never adds one | `{ip, mac, type, name, firmware, settings}` | Created device |
| GET | `/api/v1/devices/{id}` | Get single device | Path: `id` | Device object |
| PUT | `/api/v1/devices/{id}` | Update device | Path: `id`, Body: device fields | Updated device |
| DELETE | `/api/v1/devices/{id}` | Delete device | Path: `id` | Success confirmation |
| POST | `/api/v1/devices/{id}/control` | Control device; 409 when the device's MAC is not the stored one; 503 `DEVICE_OFFLINE` for a device marked offline unless the optional `force` is true (see the error-code table) | `{action, params, force}` | Action result |
| GET | `/api/v1/devices/{id}/status` | Get device status | Path: `id` | Status object |
| GET | `/api/v1/devices/{id}/energy` | Get energy metrics | Path: `id` | Energy data |

**Read-only fields in a device `PUT`:** `template_ids`, `overrides`, `desired_config` and `config_applied` each have an owner, and a `PUT /devices/{id}` never writes them: a body that carries them changes none of them, and the answer shows the stored values. Set the templates with `PUT /devices/{id}/templates/new`, the overrides with `PUT` or `PATCH /devices/{id}/overrides/new` and the applied flag with `POST /devices/{id}/config/new/apply`. `desired_config` is derived from the templates and the overrides, and `GET /devices/{id}/desired-config` returns it. Before this change a `PUT` wrote these four columns from the body.

**Login in a device `PUT`:** A `PUT /devices/{id}` saves the device's own fields. The login that the manager uses to reach the device sits in `settings` (`auth_enabled`, `auth_user`, `auth_pass` and `auth`). The server reads the stored device when the request starts. If the request leaves the login as it is in that stored device, the save keeps the login that is stored at the moment of the save. This holds when another request saved a new login after the request started. If the request changes the login, the new login is saved. A missing `auth_enabled` counts as `false`. Limit: a client that sends a copy that it read before another save, and that leaves out `updated_at`, is not protected. The server can take the older login in that copy for a change, and then the save writes it, with the older values of every other field. A password that comes back as the mask keeps the stored password.

**Version check in a device `PUT`:** The device carries `updated_at`, the time of its last save. A `PUT /devices/{id}` that sends the `updated_at` that it read is saved. If the stored `updated_at` is a different instant, the server saves nothing and answers `409` with the code `CONFLICT`. The client then reads the device again and sends its change again. The answer to a saved `PUT` carries the new `updated_at`. A body without `updated_at` is saved against the row the server reads when the save starts, so a writer that saves between that read and the save makes it answer `409` too. A writer that saved before that read is not seen, which is why a client sends `updated_at`. The server compares instants, so a time zone offset does not matter. The status and the last seen time are not part of the version. A status read and a control command record them without a change to `updated_at`, so a device page that polls the status keeps a version that the next `PUT` accepts. A change of the stored login does move `updated_at`, including the one-time save of a working login that a status read finds and the one-time removal of a login that the device rejected. Both change the settings that a client edits, so one `409` after such a change is correct. A discovery refresh moves `updated_at` only when it changes the address, the type, the firmware or the settings that a discovery stores. A discovery that finds the same values writes the status and the last seen time only, so it keeps the version. An import and a configuration change move `updated_at`. The save is one conditional SQL update that matches the `updated_at` that the server read, so a writer that saves between the compare and the save makes the `PUT` answer `409` and save nothing. Limits: First, MySQL stores `updated_at` in milliseconds (`datetime(3)`). On a legacy column that holds whole seconds, two saves in the same second can carry the same value, and the check does not tell them apart. Second, a forced archive restore moves the address of a device to a placeholder for a short time inside its transaction, and this step does not change `updated_at`. The import of a discovered device changes `updated_at` only when the address, the type or the firmware is new. The check covers `PUT /devices/{id}` only.

**Device Model:**
```json
{
  "id": 1,
  "ip": "192.168.1.100",
  "mac": "AA:BB:CC:DD:EE:FF",
  "type": "SHSW-1",
  "name": "Living Room Switch",
  "firmware": "1.0.0",
  "status": "online",
  "last_seen": "2025-11-30T12:00:00Z",
  "settings": "{...}",
  "created_at": "...",
  "updated_at": "..."
}
```

---

### 3. Device Configuration (14 endpoints)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/devices/{id}/config` | Get stored device config (what an import wrote, and what `PUT /devices/{id}/config` changed since: `last_synced` is the import time, `updated_at` the last change). 404 `NOT_FOUND` when nothing was imported for the device; it was a 500 before |
| PUT | `/api/v1/devices/{id}/config` | Update stored config |
| GET | `/api/v1/devices/{id}/config/current` | Get live config from device (stores nothing; drift keeps its baseline) |
| GET | `/api/v1/devices/{id}/config/current/normalized` | Get normalized live config (stores nothing) |
| GET | `/api/v1/devices/{id}/config/typed/normalized` | Get typed normalized config. `data.configuration` holds the normalized configuration, as on `/config/current/normalized`. Wherever the typed configuration holds a top-level `location` (a stored Gen2 import does, and so can a Gen1 one), the time zone and position come from it, and it wins field by field over `system.location` when both are set. For a Gen2 device `raw.additional` lists only the top-level keys that nothing is read from, such as `ble`, `modbus`, `script:N` and `ws`. A setting under a key that is read in part, such as `sys.cfg_rev`, is not listed here and stays in the `raw` part of `GET /devices/{id}/config/typed` |
| POST | `/api/v1/devices/{id}/config/import` | Import config to device |
| GET | `/api/v1/devices/{id}/config/status` | Get import status: the state that an import or restore last stored (`synced` or `pending`), not drift. Use `/config/drift` for drift |
| GET | `/api/v1/devices/{id}/config/drift` | Detect configuration drift |
| POST | `/api/v1/devices/{id}/config/apply-template` | Apply template to device |
| GET | `/api/v1/devices/{id}/config/history` | Get config change history: a list of rows (`action`, `old_config`, `new_config`, `changes`, `changed_by`, `created_at`), newest first, `?limit=` (default 50). Secrets are masked in all three of `old_config`, `new_config` and `changes` |
| GET | `/api/v1/devices/{id}/desired-config` | Get the desired configuration and what the device inherits from its templates |
| GET | `/api/v1/devices/{id}/config/new/status` | Get whether the desired configuration is applied |
| POST | `/api/v1/devices/{id}/config/new/apply` | Write the desired configuration to the device (admin key) |
| POST | `/api/v1/devices/{id}/config/new/verify` | Compare the desired configuration with the device; 409 when the device's MAC is not the stored one |

#### The desired configuration and its inherited part

`GET /devices/{id}/desired-config` returns `config` and `sources`: the composed result of the device's templates and overrides, with the layer each value comes from. It also returns `inherited` and `inherited_sources`: the template layers alone, which is what a field falls back to when its override is removed. A field that a template and an override both set shows only the override in `config`, and the template value in `inherited`. A set secret is removed from `config`. In `inherited` it shows as the mask `********`, never as its value, so a client can see that a password is set. `PUT /devices/{id}/templates/new` returns the same two fields beside its `desired_config` and `sources`.

The device overrides routes (`GET`, `PUT` and `PATCH /devices/{id}/overrides/new`) show a saved secret as the same mask `********`. The secrets are the Wi-Fi password, the access point password and key, the MQTT password, the authentication password, and the station and access point passwords of the `network.wifi` part. The desired-config, template and overrides routes remove the secrets (`config`, `desired_config`) or mask them (`inherited`, `overrides`). A write that sends the mask back keeps the stored secret. If no secret is stored, the field is dropped. The mask is never stored as a value. Any other value replaces the stored secret.

`PUT` and `PATCH /devices/{id}/overrides/new` check the body against the schema of the device model, as the template routes do (the model of the device, or every stored section when the device has no model). A value outside its schema (range, length, option, or an `ipv4` field that is not four decimal parts) is refused with 400 `VALIDATION_FAILED`, one entry per field in `error.details.fields`, and nothing is stored. A section the model does not have (`roller` on a plug) is refused with its name. An empty string passes an `ipv4` field, because an empty address reads as not set. The same check covers `system.sntp.server` (an IPv4 address or a host name, the rule apply uses), `mqtt.server` (the same, with an optional `:port`, because a Gen1 device stores the broker that way and apply accepts it, or a bare IPv6 address in colon-hex form, which a Gen2 apply writes with the port from its own field; the bracketed `[addr]:port` form is refused, as Gen2 apply refuses it) and `coiot.peer` (`host:port` with a port from 1 to 65535; empty means multicast), and an empty value passes each. A key that differs from the stored key only in case is checked as the field it stores. A value of the wrong JSON type (`"port": "1883"`) fails when the body is decoded, before this check, and answers 400 `VALIDATION_FAILED` with the text `Invalid JSON request body` in `error.details` and no field list. A `PATCH` null removes a value and is not checked, and a section that holds no value after the nulls is not checked either, so the last value of a section the model lacks can still be removed. `GET` still returns an override that was stored before this check, even when it holds a bad value; a `PUT` of that body is refused until the value is fixed or removed.

#### Secrets in the device routes

The device routes show a saved secret as the mask `********` and never return its value: `GET /devices`, `GET`, `POST` and `PUT /devices/{id}`, and the four configuration reads `GET /devices/{id}/config`, `POST /devices/{id}/config/import`, `GET /devices/{id}/config/current` and `GET /devices/{id}/config/history`. The history read masks the configuration before and after a change by key name, and masks the `expected` and `actual` value of each difference as it would be masked inside the configuration at its `path`: a secret key anywhere on the path (`api_key.value` as much as `mqtt.pass`) shows the mask on both sides, and a removed or added section keeps its shape with its secrets masked. Any other field of a difference is masked by key name, and a `changes` text that is not a list of differences with a text `path` each is returned as the mask whole. The stored history rows keep the real values. A device holds its secrets inside JSON text (`settings`, `overrides`, `desired_config`), and the configuration reads return the device's own raw settings, so these routes mask by key name, with the rule the notification channels use: a value under a key that contains `pass`, `pwd`, `psk`, `secret` or `token`, or that ends in `key`, where names are matched the way the server's JSON decoder matches them (without regard to case, so `ſecret` is `secret`). A string or number is replaced by the mask. A non-empty list or object under such a key is one secret and is replaced whole by the mask, and the mask brings the whole stored value back. Examples are the Gen1 `wifi_sta.key`, `wifi_ap.key`, `login.password` and `mqtt.pass`, or the Gen2 `wifi.sta.pass`, `wifi.ap.pass` and `mqtt.pass`. An empty value stays empty. A text that is not valid JSON cannot be read for key names, so it is returned as the mask whole. The stored record keeps the real values, so drift detection and apply are unchanged. `PUT /devices/{id}`, `POST /devices` and `PUT /devices/{id}/config` treat a value that comes back as the mask like the overrides routes do: it takes the stored value, or is dropped when none is stored. The stored key is found without regard to case, and two stored keys that differ only in case restore neither. A whole text that is the mask as a JSON string takes the stored text. A text that repeats a member name, with names matched as above and at any depth, is refused with 400 (`device configuration text repeats a member name: send each name once`), because the masked read of such a text would not show what is stored. The answer to `PUT /devices/{id}/config` shows a secret as the mask too. `PUT /devices/{id}/config/auth` takes the stored login password when the password is the mask, and refuses it with 400 when none is stored. `GET /devices/{id}/config/current/normalized` carries no secret field. The typed routes `GET /devices/{id}/config/typed` and `.../typed/normalized` mask the same way, in the typed sections and, by key name, in the `raw` part. `PUT /devices/{id}/config/typed` follows the rules of the overrides routes: a secret that is the mask or is left out keeps its stored value, an empty string or `null` removes it, and the mask is never stored. The same holds for a secret inside `raw`. A configuration stored in the device's own (Gen1) shape converts `wifi_sta.key` to the typed Wi-Fi password and `login.password` to the typed login password, so the typed read masks them and a save keeps them; a `pass` that is present, even as `null`, takes precedence over these spellings. A save stores the typed shape. In an array, a masked or omitted secret returns only to the stored entry at the same position that matches it on everything but its secrets. This holds for an array inside `raw` and for a `raw` part that is an array itself. Numbers match by value, so a stored `1.0` matches the `1` of a client that writes its JSON again. Where more than one stored entry matches, the request is refused with 400 and asks for the secret value only if those entries disagree at a secret the entry gets back: entries that share the password and differ in a token the request replaces are not ambiguous for the password. The answer is masked too, validation messages included. Limit: the `network.wifi.*.pass` fields are plain strings, so an empty value there keeps the stored password; send `null` to clear one.

#### Apply and verify the desired configuration

The desired configuration is the composed result of the device's templates and overrides. Neither route takes a request body.

`POST /devices/{id}/config/new/apply` writes the desired configuration to the device. Then it reads the configuration back and compares it. If the server has an admin key configured, the route needs that key, as `Authorization: Bearer <key>` or `X-API-Key: <key>`. Without the key it then returns 401 `UNAUTHORIZED`. If no admin key is configured, the route is open, like the other admin routes. Before it marks or writes anything, apply reads the MAC the device at the stored address reports and compares it with the stored MAC (case, colons, dashes and dots are ignored). A different MAC, a device that reports none, or a record with no MAC is refused with 409 `CONFLICT`, nothing is written and no flag changes: set a missing MAC with `PUT /api/v1/devices/{id}`, and run discovery (`POST /api/v1/discover`) when the address now leads to another device. Apply clears `config_applied` before the write. It sets the flag again only if the read-back matches and the desired configuration did not change during the apply. Applies to one device run one at a time, from the snapshot to the flag update and the pending-reboot record, in one server process and, on PostgreSQL and MySQL, across manager replicas: the apply holds a lock in the database for that span. A second apply to the same device, on this server or on another replica, waits up to 15 seconds in all, well inside the 30 second request timeout (the wait for the in-process lock counts in those 15 seconds), and is then refused with 409 `CONFLICT` (an apply that then gets the lock has about 15 seconds of the request left, so a long device apply, for example one with a reboot, can still end in 408); nothing was written, so the call can be repeated. An apply that runs longer than 15 seconds therefore makes a second apply to the same device fail with 409 where it used to queue behind it. The lock belongs to the database session of the apply, so a replica that stops frees it when the server sees its connection close; an abrupt loss of the replica (no TCP reset) frees it only when the server notices the dead client, and that is not tested. A connection pooler in transaction mode (PgBouncer) breaks the lock, so connect the manager to the database directly or in session mode. Each running apply holds one pooled connection for the lock and needs another for its own queries, so at most half of `max_open_conns` can hold locks at once: more applies wait for a slot inside the same 15 seconds. A pool of one connection (`max_open_conns: 1`, which is not the PostgreSQL or MySQL default of 25 and has to be set on purpose or kept from an older example config) cannot hold the lock and run the apply's own queries: below 2 there is no cross-replica lock, only the in-process one, and the server logs a warning once per process. Set `database.max_open_conns` to at least 2 (the PostgreSQL and MySQL pool default is 25) to get the lock; ginsys/shelly-manager#570 made that the default. Right before its first write (to the device or to the stored reboot flag), the apply checks that it still holds the lock. If the session was lost by then, the apply writes nothing to the device, forces no `pending_reboot` and makes no later flag change, and fails with the lost-lock error, so the result of the replica that took over stays. The failed apply does not restore the `config_applied` flag it cleared at its start: the device stays pending unless another apply has set the flag since. If the session is lost later, during the apply, the device may hold a mix of two applies: the apply then fails, sets `config_applied` to false and `pending_reboot` to true, and the next apply writes the configuration again and reboots once more. Usually the session is lost because the database became unreachable, and then those two writes fail too: the log names them, and the other replica's flags stay until the next apply or verify. Three cases stay open, and all are accepted. First, an apply that stalls before the check (reading the device identity, for example) and clears `config_applied` after the replica that took over has set it leaves the device shown as pending until the next apply. Second, the check and the device write are not one step: if the apply stalls after the check and loses its lock, and the replica that took over reads the configuration back and then waits, the first apply can still write its older configuration, and the replica that took over then sets `config_applied` to true over a device that holds the older one. Third, the unreachable database above. The cost of each is at most one extra reboot, or one stale flag until the next apply or verify. Two designs were weighed and not built (ginsys/shelly-manager#564). A per-device epoch carried to the flag writes: a fencing token only works where the guarded resource checks it, and a Shelly device checks nothing, so the epoch would only reorder the flag writes and the outcome would be the same. A check before every section: a slow or briefly unreachable database would then stop a healthy apply halfway and leave a half-written device, which happens more often than the case it guards. SQLite runs in one process and needs no database lock.

The response `data` holds these fields:

| Field | Meaning |
|-------|---------|
| `success` | `true` if the read-back matched and `config_applied` is set |
| `applied_count`, `failed_count` | Settings the device accepted and refused |
| `requires_reboot` | The device needs a reboot for a change to take effect. A reboot owed by an earlier apply counts too: if a reboot-needing setting was written and the apply then failed, it stays owed until a later apply has rebooted the device. On Gen1 the reboot is recorded before the first write, so a write the device refused or never answered counts too: it may have landed. On Gen2 and Gen3 the device decides: the apply reboots only when an answer says `restart_required`, and records that reboot as owed when it reads the answer. A write whose answer is lost is not recorded beforehand. If the restart cannot be recorded, the apply fails with a warning to reboot by hand |
| `unconfirmed` | A Wi-Fi write changed the station network and the device stopped answering (Gen1, Gen2 and Gen3). Neither a success nor a failure: the device stays pending |
| `failures` | Each refused setting by path, and each read-back difference. Secret values are masked |
| `warnings` | Problems that did not stop the apply |

`POST /devices/{id}/config/new/verify` reads the device and compares it with the desired configuration. It writes nothing. A device with no desired configuration (empty, `{}` or `null`) has nothing to compare with: verify answers `404` `NOT_FOUND` with that reason, as `/config/drift` does, and does not contact the device. Apply is not affected. Before it compares, it reads the MAC the device reports, as apply does, and answers `409` `CONFLICT` when that is not the stored MAC, when the device reports none or when the record has none: another device at the stored address is never reported as in sync or as drifted. The four live reads (`GET /devices/{id}/status`, `/energy`, `/config/current` and `/config/current/normalized`) answer the same `409` `CONFLICT` for the same three causes. They reach it when the device refuses the stored login and the identity read that follows then finds another MAC, none stored or none reported: the message keeps the identity text, which says the login was rejected. A refused login whose identity read fails in any other way keeps the `502` `EXTERNAL_SERVICE_ERROR`. The same check guards `POST /devices/{id}/control` (on, off, toggle and reboot): a device that is not the stored one gets no command, and the service neither clears nor saves a stored login on its word. The identity is read without a login, because a Gen1 `/shelly` and a Gen2 `Shelly.GetDeviceInfo` are served without one (Shelly Gen1 reference, `/shelly`; Gen2 reference, Authentication). A status read sends the extra request only when it is about to save or clear a stored login. The response `data` holds `match` and `differences`. Each difference has `path`, `expected` and `actual`. Secret values (passwords, keys, tokens) are masked.

Supported devices and settings:

- Gen1, and Gen2 and Gen3 for the system, MQTT, cloud, switch and input sections and the Wi-Fi station, access point and roam threshold, and the login only. Any other generation, or a client that cannot write Gen2 settings, returns 501 `NOT_IMPLEMENTED`.
- Gen2 and Gen3 settings: name, eco mode, discoverable, timezone, latitude, longitude and SNTP server, written with `Sys.SetConfig`; MQTT enable, server, port, user, password, client ID, topic prefix and the RPC and status notification flags, written with `MQTT.SetConfig`; cloud enable and server, written with `Cloud.SetConfig`. All are read back from `Shelly.GetConfig` except the MQTT password, which is write-only: it is sent only with a user, never when it is empty or the mask, and it never appears in a failure or a log, and while an apply holds a password the text of every device error is replaced by a fixed message. Only the keys the desired configuration sets are sent, because the Shelly documentation does not say that `SetConfig` merges. Any other setting, an MQTT password with no user, an MQTT server that already carries a port beside a separate port, an IPv6 MQTT endpoint in one string, and an SNTP server written as `host:port`, return 422 `VALIDATION_FAILED` and nothing is written. Unconfirmed on hardware: the read-back of each key.
- Gen2 and Gen3 switches and inputs: each relay channel is written with `Switch.SetConfig` and each input with `Input.SetConfig`, one call per channel with `{"id": N, "config": ...}`, after the system, MQTT and cloud sections, inputs first and then switches, by id. Relay `name`, `default_state` (as `initial_state`), `auto_on` and `auto_off` are written, and the `mode` of input N is written as the `in_mode` of switch N. Input `name`, `type` (`switch` or `button`) and `inverted` (as `invert`) are written. The Gen2 words are `on`, `off`, `restore_last` and `match_input` for the initial state, and `momentary`, `follow`, `flip`, `detached`, `cycle` and `activate` for the mode. A Gen1 word with the same meaning is accepted: `last` is `restore_last`, `switch` is `match_input`, `toggle` is `follow` and `edge` is `flip`, and verify treats each pair as one value. `auto_on` and `auto_off` are seconds: above 0 sends the timer on with that delay, and 0 sends it off with no delay. Only the keys the desired configuration sets are sent. Any other value (such as the Gen1 mode `action`, an input type `analog` or `count`, or a negative timer), an input push action or timing, a relay schedule, the top-level relay and input settings, a channel id that a list repeats, whether or not the entries set anything (named at the second entry, such as `relay.relays.1.id`), and a negative channel id, on any entry (named at its own place in the desired list, such as `input.inputs.1.id`) return 422 `VALIDATION_FAILED` and nothing is written. An input mode returns 422 too, when the device's switch with the id of that input runs another input (its `input_id` is not that id), because the mode is written to the switch. The path is `input.inputs.I.mode`, where I is the position of the input in the desired list, not its id. Apply reads the device before writing only when the desired list sets an input mode. A device that does not have the mode (`cycle` needs a Plus 2PM, Pro 2PM or Pro 2, and `activate` needs a physical input) answers with its own error. The device decides whether a write needs a restart. Unconfirmed on hardware: every key and the read-back.
- Gen2 and Gen3 Wi-Fi station: `wifi.enable`, `wifi.ssid`, `wifi.password`, `wifi.ipv4mode` and the static address (`wifi.static_ip.ip`, `netmask`, `gw`, `nameserver`) are written with one `WiFi.SetConfig` call, `{"config": {"sta": ...}}`, after the system, MQTT, cloud, switch and input sections, because the device reconnects about a second after the write. They are read back from `wifi.sta` in `Shelly.GetConfig`, except the password, which the device never returns and which never appears in a failure or a log (the text of every error of a failed write is replaced by a fixed message while an apply holds a password). A password may be sent alone, to change the password of the network the device is on. Only the keys the desired configuration sets are sent. A `ssid` is written only with a password; where the desired configuration names the network the device is already on and sets no password, the `ssid` is left out of the write. The device is read before a write that sets any of `wifi.enable`, `wifi.ssid`, `wifi.ipv4mode` or the static `ip`, `netmask` or `gw`; a password or a name server alone is written without a read. These return 422 `VALIDATION_FAILED` and nothing is written: a new `ssid` with no password that can be sent (an empty one and the mask `********` are not passwords), and `wifi.enable` set to false (the manager reaches the device through the station). A device that cannot be read before such a write is refused too, with the error of a failed device read, and nothing is written, as for Gen1. A wrong static address strands the device: the apply writes what the configuration says and no check can tell a wrong address from a right one. A write that changes the network (a different `ssid`, `enable`, `ipv4mode`, `ip`, `netmask` or `gw` than the device reported before the write; the password and the name server never count) can make the device answer from another address. If the device then gives no answer to the reboot or to the read-back, the apply response has `unconfirmed: true` with the same warning as Gen1: neither a success nor a failure, and the device stays pending. A write that gets no answer is not counted as a failure: it may have landed, so the apply goes on to the read-back, and no reboot is requested for it. A silent read-back then makes the apply `unconfirmed`, a read-back that matches makes it a success, and one that differs reports the differences. A device that answers with an error (also a reboot that ends with the device still answering with errors), or a reply that cannot be read as a configuration, keeps its error. The stored address of the device is not changed: to reach it after a static address change, re-discover the device (discovery matches by MAC address) or correct its address, then apply again. Unconfirmed on hardware: every key, the read-back and what `restart_required` means for these keys.
- Gen2 and Gen3 Wi-Fi access point and roaming: `wifi.ap.enable`, `wifi.ap.ssid` and `wifi.ap.pass` are written with `WiFi.SetConfig`, `{"config": {"ap": ...}}`, and `wifi.roam_threshold` as `{"config": {"roam": {"rssi_thr": ...}}}`, each in its own call before the station call. They are read back from `wifi.ap` (`enable`, `ssid`) and `wifi.roam.rssi_thr`. The access point password is write-only like the station password (never returned, never in a failure or a log) and may be sent alone. `wifi.ap.key` has no Gen2 meaning (the access point has one secret, `wifi.ap.pass`) and returns 422 `VALIDATION_FAILED`. Two more rules return 422 and write nothing. Switching the access point off (`wifi.ap.enable` false, while the device reports it on or does not report it) needs a steady station: the device reports its station enabled with a `ssid`, and the same apply changes neither the station network (a different `ssid`, `enable`, `ipv4mode`, `ip`, `netmask` or `gw`) nor its password, so the device keeps a way back. Switching an access point on (while the device does not report it on) needs a `wifi.ap.pass` in the same configuration, or a device that reports the access point closed (`is_open` false), because a Shelly access point is open until it has a password. The device is read before such a write, and one that cannot be read is refused. An access point that already runs open needs no password to stay enabled, and the apply leaves it open (a password in the configuration is still sent). A station network change on a device whose access point is off is not checked: it carries the risk of a wrong static address described above. `roam.interval` is not written. Unconfirmed on hardware: every key, what `ap.enable` false does, and whether the access point shuts itself when the station connects.
- Gen2 and Gen3 login: `auth.enable`, `auth.user` and `auth.pass` are written last, after every other setting, with `Shelly.SetAuth`, which takes the user, the `realm` (the device id, read with `Shelly.GetDeviceInfo`) and `ha1`, the lowercase hex SHA-256 of `user:realm:password` (`ha1` null switches the login off), never the password. A Gen2 device has one user, `admin`. The password is write-only: it never appears in a failure, a log or a response. The new login is stored once the device accepted it, and the client is rebuilt with it for the reboot and the read-back. The read-back is `enable` from `auth_en` in `Shelly.GetDeviceInfo`; the device reports neither the user nor the password. These return 422 `VALIDATION_FAILED` and nothing is written: a user other than `admin`, `auth.enable` true with no password that can be sent (an empty one and the mask `********` are not passwords), `auth.enable` false with a user or a password, `auth.user` or `auth.pass` with no `auth.enable`, `auth.realm`, a login change in the same apply as a change of the Wi-Fi station network (apply them one at a time), and any login setting where the server cannot store the new login. `SetAuth` is sent once and never retried, and a failure carries only the fixed text `auth: the device refused the write` (the device's own text can echo `ha1`). A `SetAuth` answer that never arrives, or that ends early or is not JSON, is a failure, not a success: the response adds a warning that the device may have switched its login, so the stored login may now be stale, and nothing is stored. A device that rejects the stored login still returns 502 `EXTERNAL_SERVICE_ERROR`: store the right login with `PUT /devices/{id}/config/auth`. Digest authentication answers the SHA-256 challenge of Gen2 devices (RFC 7616) and resends the request body on the authenticated retry. Unconfirmed on hardware: whether `Shelly.SetAuth` restarts the device or drops open connections, and what a device does with an absent `ha1`.
- Supported Gen1 settings (the rest of this list is Gen1): name, timezone, location, eco mode, discoverable, max power, LED, MQTT, relays, rollers, input button types, cloud, CoIoT, login, SNTP server, MQTT update period and Wi-Fi (station and access point). A MQTT server given as `host:port` matches the device's split read-back. The MQTT password is write-only on Gen1, so verify does not compare it.
- Relays: for each entry of `relays`, apply writes `default_state`, `auto_on`, `auto_off`, `name`, `appliance_type`, `schedule`, `schedule_rules` and `max_power` to that channel with `POST /settings/relay/<n>`, then reads the channel back. A value the device did not keep fails with the path, for example `relays[0].auto_off`. Each relay `id` must equal its position in the list, because the position is the channel written. An `id` that differs from its position, a `default_state` of `restore_last` or `match_input` (the Gen2 words for `last` and `switch`), a negative `max_power`, an empty schedule rule, a rule with a comma and more than 20 rules return 422 `VALIDATION_FAILED`. `schedule_rules` goes out as one comma-separated parameter and is read back as a list. An omitted `schedule_rules` leaves the rules alone, and an empty list clears them; verify compares the whole list, so a rule left on the device is a difference. A Shelly 2.5 in relay mode (its reported `mode`) keeps its overpower threshold per channel, because the reference says `max_power` on `POST /settings` is "for roller mode only": there a desired `power_metering.max_power` is written to every channel that states no `max_power` of its own, and `POST /settings` is not used for it; verify compares it channel by channel. In roller mode, and on every other model, `power_metering.max_power` goes to `POST /settings` as before. Unconfirmed on hardware (read from the Shelly reference): the comma-separated form of `schedule_rules`, the 20-rule bound (18 on a Shelly 1 and 1PM), and that no reboot is needed.
- Relays in the typed conversion: when a raw (Gen1 shaped) configuration is converted to the typed shape, `GET /devices/{id}/config/typed` and `POST /config/convert-to-typed` keep `appliance_type`, `schedule_rules` and `max_power` for each relay channel. `GET /devices/{id}/config/typed/normalized` converts the same way, but its relay entry holds only the state, the timers and the default state, so it shows none of the three. This holds for the `relays` array and for the `relay_0` and `relay_1` keys. `appliance_type` is no longer taken as the relay name. A channel that states none of the three leaves them unset. An omitted `schedule_rules` stays unset, and an empty list stays an empty list, which means no rules. A fractional `max_power` (for example 7.5) reads as no value and is never cut to 7. Other whole-number reads in this conversion still cut a fraction, and ginsys/shelly-manager#532 tracks them.
- Rollers: only a model with roller mode (Shelly 2.5, `SHSW-25`) has the `roller` section; on any other model it returns 422 `VALIDATION_FAILED`. Apply writes the keys that are set to channel 0 with `POST /settings/roller/0`, then reads each one back with `GET /settings/roller/0`. A value the device did not keep fails with the path, for example `rollers[0].maxtime_close`. The first five fields are `max_open_time` and `max_close_time` (whole seconds, written as `maxtime_open` and `maxtime_close`), `motor_direction` (`normal` or `reversed`, written as `swap`), `swap_inputs` and `input_mode` (`single` or `dual`, written as `onebutton` or `openclose`). Apply also writes `default_state` (`stop`, `open` or `close`), `button_type` (`momentary`, `toggle`, `detached` or `action`; sent as `btn_type` and read back as `button_type`), `button_reverse` (sent as `btn_reverse`, 0 or 1), `schedule`, `schedule_rules`, `obstacle_mode` and `safety_mode` (`disabled`, `while_opening`, `while_closing` or `while_moving`), `obstacle_action` (`stop` or `reverse`), `safety_action` (`stop`, `pause` or `reverse`), `safety_allowed_on_trigger` (`none`, `open`, `close` or `all`), `obstacle_power` and `off_power` (watts), `obstacle_delay` (seconds) and `ends_delay` (milliseconds, 0 to 10000 in steps of 100). An omitted `schedule_rules` leaves the rules alone and an empty list clears them; verify compares the whole list. A travel time below 1 second, a value outside those lists, a negative power or delay, an `ends_delay` outside its range or step, an empty schedule rule, a rule with a comma, more than 20 rules, and any other roller field (such as `positioning_enabled` or `safety_switch`) return 422 `VALIDATION_FAILED`. Unconfirmed on hardware: the key names and values, read from the Gen1 reference for the Shelly 2.5, and the types the reference gives for `btn_reverse` and `positioning`, which differ between its read and write tables. A device that is in relay mode refuses the endpoint, and the failure is reported per key.
- Cloud: apply writes the `enable` flag with `POST /settings/cloud` and the read-back confirms it. A cloud `server` returns 422 `VALIDATION_FAILED`, because Gen1 reports no cloud server and accepts none.
- CoIoT: apply writes `coiot_enable`, `coiot_update_period` and `coiot_peer` as flat parameters on `POST /settings`, then reads the nested `coiot` document back. An empty `peer` is the multicast value, and apply writes it as `mcast`. An `update_period` outside 15 to 65535 seconds, and the literal peer `mcast` (the device reads multicast back as an empty peer), return 422 `VALIDATION_FAILED`. The Shelly documentation does not say whether a CoIoT change needs a reboot, so the response does not report one.
- Login: apply writes `auth` with `POST /settings/login`, last, after every other setting. A username and password of 1 to 50 bytes each (a multibyte character counts as several) are needed together with `enable: true`; with `enable: false` neither may be set. Anything else returns 422 `VALIDATION_FAILED` and nothing is written. When the device accepts the write, the manager stores the new login (as `PUT /devices/{id}/config/auth` does), then reboots and reads back with it. A write the device rejects stores nothing. The password is write-only: verify does not compare it and no result echoes it. If the response is lost after the device accepted the write, the stored login is stale until it is entered again. Unconfirmed on hardware: the field names (read from a summary of the Shelly reference) and whether the change needs a reboot.
- SNTP and MQTT update period: apply writes `system.sntp.server` as `sntp_server` and `mqtt.update_period` as `mqtt_update_period`, flat on `POST /settings`, then reads `sntp.server` and `mqtt.update_period` back. A server that is not an IPv4 address or a valid host name (letters, digits and hyphens in labels of 1 to 63 bytes, 253 bytes in all; an IPv6 literal is refused because Gen1 is IPv4 only; a name whose last label is all digits must be a valid IPv4 address, so `192.168.1.300` is refused) and a negative period return 422 `VALIDATION_FAILED`. An MQTT server with a second colon or a leading `[` (an IPv6 address, bracketed or not) returns 422 `VALIDATION_FAILED` with `mqtt.server`, because Gen1 stores the broker as one `host:port` string and no bracketed form is documented; the form still accepts a bare IPv6 address, which a Gen2 apply writes. A period of `0` is written as it is. An empty server cannot be told from an unset one, so it is not written. The same holds when templates and overrides are merged: an empty, `false` or `0` value of a plain setting (a field that is not optional, such as `system.sntp.server`) does not clear a value set earlier, neither a template's nor the override's own. For `system.sntp.server` apply then writes that earlier value; the other plain settings (debug, RPC, sleep, UI data and BLE) stay refused. A `PATCH` with `null` removes it from the override, as does a `PUT` that leaves it out or clearing the overrides, and the inherited value then returns. A change to any MQTT setting apply writes (enable, server, user, password, id, clean session, keep alive, update period) reports `requires_reboot`, because the Shelly reference says an MQTT change needs one; the password is never read back, so a configured one always counts as changed. Unconfirmed on hardware (all read from a summary of the Shelly reference): the parameter names, the period range and that reboot.
- Inputs: for each entry of `input.inputs`, apply writes `mode` as the Gen1 button type `btn_type` and `inverted` as `btn_reverse` (0 or 1), then reads the input back. The stored modes map as `momentary` to `momentary`, `follow` to `toggle`, `flip` to `edge` and `detached` to `detached`. A Shelly 1, 1PM or 2.5 keeps both in its relay channel (`POST` and `GET /settings/relay/<n>`) and accepts all four modes. A Shelly i3 has its own route (`POST` and `GET /settings/input/<n>`) and accepts `momentary` and `follow` only. Each input `id` must equal its position in the list. The input `name`, `type` and push actions, the section-wide input settings, a mode the model does not accept, an `id` that differs from its position and any `input` section on a model with no inputs (a plug) return 422 `VALIDATION_FAILED`. Unconfirmed on hardware (read from a summary of the Shelly reference): the parameter names, the toggle and edge mapping, the i3 mode set and whether a button type change needs a reboot, which apply does not report.
- Wi-Fi: apply writes the station with `POST /settings/sta` and the access point with `POST /settings/ap`, only for the keys the desired configuration sets. The desired-configuration keys map to these form parameters. Station: `wifi.enable` as `enabled`, `wifi.ssid` as `ssid`, `wifi.password` as `key`, `wifi.ipv4mode` as `ipv4_method` (`dhcp` or `static`) and, for a static address, `wifi.static_ip.ip` as `ip`, `netmask` as `netmask`, `gw` as `gateway` and `nameserver` as `dns`. Access point: `wifi.ap.enable` as `enabled` and `wifi.ap.pass` as `key` only, because a Gen1 access point name cannot be changed. A flag goes out as 1 or 0. Apply reads `GET /settings` before any write. If that read fails, nothing is written and the response is the read's own error (500, or 502 `EXTERNAL_SERVICE_ERROR` if the device rejects the stored login). A new `ssid` with an empty or redacted password returns 422 `VALIDATION_FAILED`, and nothing is written. A refused apply leaves `config_applied` as it was. A write that keeps the same `ssid` may omit the password. The password is write-only: it is never sent empty or redacted, never logged, and neither verify nor any result echoes it. A Wi-Fi change never reports `requires_reboot`. Apply reads the station and access point back from `GET /settings`. If the write changed the station network (`ssid`, `enable`, `ipv4mode`, or a static `ip`, `netmask` or `gw`; a name server alone is not a network change) and the device then gives no answer at all to the read-back, the response has `unconfirmed: true`, `success: false` and no failure. A station write whose own response is lost (a transport error, not an HTTP status) counts the same way: apply reads it back, and silence there is `unconfirmed`, while a read-back that answers with the old value is a failure. That is a state of its own: the device may now be on another network. Apply then skips verify, the reboot and the sections after Wi-Fi (MQTT, SNTP, cloud, CoIoT, relays, rollers, inputs and login), with a warning, and the device stays pending. Apply again once the device answers. Find the device on its new network and verify it. If the write did not change the network and the device stays silent, that is a failure. A device that answers the read-back with an error (an HTTP status or bad JSON) is on its network, so that is a failure too. `ap.ssid`, `ap.key`, `roam_threshold` and a station and an access point both enabled return 422. Unconfirmed on hardware (read from a summary of the Shelly reference): the route behaviour, the key names, `enabled` as 1 or 0, that no reboot is needed and that `GET /settings` never returns the key.
- On apply, any other section returns 422 `VALIDATION_FAILED`, and nothing is written. A section the converter cannot map, such as the debug settings, returns 422 too. The error names the sections. Verify does not refuse these sections. It compares them with the device like the supported ones.
- On both routes, a stored desired configuration with fields the configuration model does not know returns 422. An example is a legacy row from an SMA import.
- If the device rejects the stored login, both routes return 502 `EXTERNAL_SERVICE_ERROR`. Store the right login with `PUT /devices/{id}/config/auth`.
- An unknown device returns 404.

---

### 4. Device Login (1 endpoint)

| Method | Endpoint | Description |
|--------|----------|-------------|
| PUT | `/api/v1/devices/{id}/config/auth` | Update device authentication |

The per-capability routes `PUT /devices/{id}/config/relay`, `/dimming`, `/roller` and `/power-metering` were removed. They wrote a legacy stored key that apply and the override editor never read. Use the override editor (`/devices/{id}/overrides/new`) and apply instead. `GET /devices/{id}/capabilities` reports what a device supports.

---

### 5. Configuration Templates (4 endpoints)

| Method | Endpoint | Description | Input |
|--------|----------|-------------|-------|
| GET | `/api/v1/config/templates` | List templates (admin key) | Query: pagination |
| POST | `/api/v1/config/templates` | Create template (admin key) | `{name, description, device_type, generation, config, variables}` |
| PUT | `/api/v1/config/templates/{id}` | Update template (admin key) | Template fields |
| DELETE | `/api/v1/config/templates/{id}` | Delete template (admin key) | Path: `id` |

**Admin key.** If the server has an admin key configured, these routes need it, as `Authorization: Bearer <key>` or `X-API-Key: <key>`. Without it they return 401 `UNAUTHORIZED`. The same holds for every route under `/api/v1/config/templates/new`, the four routes under `/api/v1/devices/{id}/templates/new` and `GET /api/v1/config/schema`. If no admin key is configured, the routes are open, like the other admin routes. The web UI sends the key it holds.

`POST` and `PUT` on `/api/v1/config/templates/new` check the `config` body against the schema of the template (the model for a `device_type` template, every stored section for a `global` or `group` template) and refuse it with 400 `VALIDATION_FAILED`, one entry per field in `error.details.fields`, and store nothing. The check covers the value of each field and the keys: an unknown key, a key that differs from the stored key only in case (`Relay` for `relay`), and a repeated key are refused with the key named, because the stored body is decoded without regard to case and merges repeated keys. A request key that is repeated or differs in case is refused the same way. The response returns a saved password as `has_wifi_password`, `has_mqtt_password` or `has_auth_password` and leaves the value out. It also returns `stored_secrets`, the path of each secret the template holds (for example `wifi.ap.pass`), which the three flags cannot give: `has_wifi_password` covers both the station and the access point password. A `PUT` without `config` keeps the stored config.

A `PUT` with a `config` replaces the config, except for secrets. A secret the body leaves out keeps its stored value, whether or not the section around it is in the body. A secret sent as the mask `********` takes the stored value, and the mask is never stored (a `POST` drops it). An empty string clears the secret. JSON `null` removes it (the JSON merge patch convention, RFC 7396): on the device overrides, the device then inherits the secret of its templates again, which is what the editor's Reset sends. So a secret path takes four values: omitted keeps the stored value, `""` clears it, the mask restores it and `null` removes it. The device overrides `PUT` follows the same rule, and an empty override secret is kept as empty, so it still beats the secret of a template. The station and access point passwords of the `network.wifi` part follow the same rule. The fields of `network.ethernet` and `network.wifi` are optional in the JSON: a field that is not set is left out, and a layer merge treats a missing field as inherited and an empty or `false` value as a real one. `PATCH` on the device overrides merges into the stored overrides, so it keeps every secret it leaves out. A secret path takes the same four values on `PATCH`. JSON `null` removes any field, not only a secret: `{"mqtt":{"server":null}}` removes the stored `mqtt.server`, and a `null` section such as `{"mqtt":null}` removes the whole section. A section that the removal leaves empty is removed too. A `null` inside an array element removes that field of the element at the same position. In the channel lists `relay.relays` and `input.inputs` it removes that field of the channel with the same `id`, wherever the stored list holds it, because the merge pairs channels by `id`; a list that repeats an `id` is merged by position, and the `null` keeps its position there. A `null` for a field that is not stored changes nothing, and so does a `null` nested deeper than 16 levels, because no configuration field sits that deep. A key that appears twice in one object is refused with 400, because the two readings of the body could differ. The same holds for the three `PUT` routes that read a secret sent as `null`: the device overrides `PUT`, the template `PUT` and `PUT /devices/{id}/config/typed`. A key repeated in the configuration they read, or in the `config` or `configuration` member that wraps it, in any casing, gets 400 and nothing is saved. The web UI never sends such a body. A few fields are plain values that the API always writes, such as `system.device.ble.enable` and `system.sleep_mode.enable`. A `null` sets such a field to `false`. A `null` for the whole section removes it.

A `device_type` template names a model from `GET /api/v1/config/device-types`. `POST /api/v1/config/templates/new` refuses a `device_type` that is not in that list with 400 `VALIDATION_FAILED` and a `device_type` entry in `error.details.fields`, and stores nothing. An empty `device_type` on a `device_type` template is refused as before, and a `global` or `group` template ignores the field. `PUT /api/v1/config/templates/new/{id}` never changes the type of a template. It refuses a `device_type` in the body only when it differs from the stored type and is not in the list, so a template stored with another type stays editable. Only these two routes check the type. The older `/api/v1/config/templates` routes, the startup migration and the SMA import only require a `device_type` template to have a type, and accept any non-empty one.

**Template Model:**
```json
{
  "id": 1,
  "name": "Office Lights Standard",
  "description": "Standard config for office light switches",
  "device_type": "SHSW-1",
  "generation": 2,
  "config": {...},
  "variables": {...},
  "is_default": false
}
```

---

### 6. Template Operations (4 endpoints)

| Method | Endpoint | Description | Input |
|--------|----------|-------------|-------|
| POST | `/api/v1/configuration/preview-template` | Preview template rendering | `{template, variables}` |
| POST | `/api/v1/configuration/validate-template` | Validate template syntax | `{template}` |
| POST | `/api/v1/configuration/templates` | Save template | `{name, template, variables}` |
| GET | `/api/v1/configuration/template-examples` | Get example templates | - |

---

### 7. Typed Configuration (9 endpoints)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/devices/{id}/config/typed` | Get typed config. `data` holds `configuration`, `validation` and `conversion_info`. A stored second-generation import (a document with a `sys` object) is read into `system`, `location`, `wifi`, `mqtt`, `cloud`, `relay` and `input`; `configuration.raw` holds the whole imported document (secrets masked), and `conversion_info.warnings` names each top-level key that nothing is read from (`ble`, `ws`, `script:1`), never a value and never a sub-key such as `sys.cfg_rev`. The raw part is a copy of the import: a typed save stores the typed sections as sent and keeps the copy, and the copy never overrides them |
| PUT | `/api/v1/devices/{id}/config/typed` | Update typed config. The body is `{"configuration": {...}, "validation_level": "basic"}`; `validation_level` is optional (`basic`, `strict` or `production`). `data` holds `configuration` (secrets masked) and `validation`. A configuration that fails validation gets 400 `VALIDATION_FAILED` with the validation result in `error.details`, and nothing is saved |
| GET | `/api/v1/devices/{id}/capabilities` | Get device capabilities. The body is `{device_id, device_model, generation, capabilities}`, with `capabilities` a list of names. Every generation lists `wifi`, `mqtt`, `cloud` and `auth`, plus the names the model adds (`relay`, `power_metering`, `dimming`, `rgbw`, `roller`, ...); `ble` and `ethernet` are listed for generation 2 and later only. `generation` is the `gen` setting of the device. Where that is missing or is not a whole number of 1 or more, it is read from the shape of the firmware (a Gen1 `v1.` version is 1, a bare version such as `1.4.4` is 2), and it is `0` when neither says, which lists neither `ble` nor `ethernet`. The list is guessed from the model name. The typed validation of a stored device, and of a request that names a `device_model`, reads the same list, so a Gen1 device with MQTT enabled gets no "may not support MQTT" warning. A validation request without `device_model` uses the `capabilities` it sends |
| POST | `/api/v1/config/validate-typed` | Validate typed config (admin key). The body is `{"configuration": {...}, "device_model": "SHPLG-S", "generation": 1, "capabilities": [...], "validation_level": "basic"}`; everything but `configuration` is optional. `data` is the validation result: `valid`, and `errors`, `warnings` and `info` when there are any |
| POST | `/api/v1/config/convert-to-typed` | Convert raw to typed (admin key). The body is `{"configuration": {...}, "device_model": "SHPLG-S", "generation": 1}`. `data` holds `configuration` and `conversion_info` |
| POST | `/api/v1/config/convert-to-raw` | Convert typed to raw (admin key). The body is `{"configuration": {...}}`. `data` holds `configuration` and `conversion_info` |
| GET | `/api/v1/config/schema` | Get configuration schema (admin key). With `?device_type=<model>` or `?device_id=<id>`: the stored-configuration schema for that model, with each field marked `x-applicable` when apply can write it. A field carries a `default` only when apply can write it and a source states one: `auth.enable` false, `mqtt.update_period` 30, `system.sntp.server` `time.google.com` and `mqtt.port` 1883. A default is a form hint, a placeholder and a "Use default" link, and apply never supplies it. With `?scope=global` or `?scope=group`: the schema of such a template, every stored section |
| GET | `/api/v1/config/device-types` | List the models a `device_type` template can name. `data.device_types` holds `{device_type, name, generation}` for each of the six supported Gen1 models (`SHPLG-S`, `SHPLG-1`, `SHSW-1`, `SHSW-PM`, `SHSW-25`, `SHIX3-1`) (admin key) |
| POST | `/api/v1/config/common-settings` | The settings that the stored imports of several devices have in common (admin key). The body is `{"device_ids": [1, 2, 3]}`: at least one positive id, at most 200 distinct ids (a repeated id counts once), and no field other than `device_ids` and `options`, where `options.stop_on_error: true` is refused. It reads the database and never contacts a device. `data` holds `compared` (the ids it read), `skipped` (`{device_id, device_name, reason}` for a device that does not exist or has no stored import, which is never counted as agreeing), `device_type` (the model all compared devices have, empty for a mix), `common` (the equal settings in the typed shape a template holds) and `differing` (per section, how many values the devices disagree on). Each import is read in its typed form with its secrets masked, and then every secret slot, every per-device value (name, hostname, MAC, login realm, MQTT client id and topic prefix, access-point name, channel names, latitude and longitude, static address) and the raw part are dropped before any two are compared, so a password that is the same on every device is never returned. A list is common only when all lists have one length and every element has something in common, and then element by element. One compared device answers its whole stripped configuration with `differing` empty; no compared device answers an empty `common`. |
| POST | `/api/v1/config/bulk-validate` | Bulk validate configs (admin key). The body is `{"configurations": [{"configuration": {...}, "device_model": "...", "generation": 1, "capabilities": [...]}], "validation_level": "basic", "stop_on_error": false}`. `data` holds `summary` (`total`, `valid`, `invalid`) and `results`. Each result is `{index, validation}` for a validated entry, or `{index, validation: null, error}` for an entry that has no `configuration` or cannot be serialized (that entry does not fail the request). With `stop_on_error` the run ends at the first invalid entry, so later entries have no result: `total` counts every entry sent, and `valid` and `invalid` count only the entries that ran |

Every typed-configuration route answers with the standard envelope, on success and on failure, except the success body of `GET /devices/{id}/capabilities`, which stays the bare object described in its row. A failure is `{"success": false, "error": {"code", "message"}}`, with `error.details` only where there is something to add (a refused `PUT`). A malformed body, a missing `configuration` (for `bulk-validate`, an empty or missing `configurations` list) or a non-numeric device id is 400 `BAD_REQUEST`, an unknown device is 404 `NOT_FOUND`, and an unexpected failure is 500 `INTERNAL_SERVER_ERROR` with no internal detail. The request keys are exactly the ones above, and `config`, `deviceType` and `configs` are not read. A body that carries only the old keys therefore gets 400 ("Configuration is required", or "No configurations provided" for `bulk-validate`); an old key beside a valid new one is ignored.

**Changed answers.** Before this change `PUT /devices/{id}/config/typed`, `POST /config/validate-typed`, `POST /config/bulk-validate` and `GET /devices/{id}/config/typed/normalized` answered a bare object, and a client that read it now reads it from `data`. The failures of `PUT`, `validate-typed` and `GET /devices/{id}/capabilities`, and a malformed `convert-to-typed` body, were plain text and are now JSON. The failures of `GET /devices/{id}/config/typed/normalized`, and a failed conversion on `GET /devices/{id}/config/typed`, were JSON with a string `error` and are now the structured `error`. A refused `PUT` used to answer `{"error", "validation"}` and now answers `error.details`. The `convert-to-raw` and `bulk-validate` request failures were 400 `VALIDATION_FAILED` and are now 400 `BAD_REQUEST` with the specific message. `convert-to-typed` without a `configuration` is 400, where it was a 500 with the same envelope.

---

### 8. Bulk Operations (5 endpoints)

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/config/bulk-import` | For each named device, read its live configuration into the manager (the per-device `POST /devices/{id}/config/import`). Never writes to a device |
| POST | `/api/v1/config/bulk-drift-detect` | For each named device, compare its stored and live configuration. Never writes to a device |
| POST | `/api/v1/config/bulk-drift-detect-enhanced` | The same selection as `bulk-drift-detect`, answered with the comprehensive drift report |
| POST | `/api/v1/config/bulk-download` | For each named device, the desired configuration with every secret masked, in one JSON file. Reads the database only. See "Download" below |
| POST | `/api/v1/config/bulk-apply` | Store one partial configuration as the overrides of each named device and apply it through the per-device apply. Writes to the named devices only. See "Apply" below |

**One request contract (all three routes, admin key).** The body is `{"device_ids": [1, 2, 3], "options": {"stop_on_error": false}}`, and the routes act on exactly those devices. `device_ids` is required: no body, invalid JSON, a missing or empty list, or an id that is not a positive integer is 400 `VALIDATION_FAILED`, and no device is contacted. A repeated id runs once, in the order first seen. A field or option the server does not read is refused with 400 naming it, so `deviceIds`, `configurations` and `validate_only` are errors, not ignored. An id that names no device is that device's `failed` entry (`error`: "device not found") and the other devices still run. With `stop_on_error`, the devices after the first failure are reported `skipped` and are not contacted.

**Answers.** Import answers `data` as `{total, success, failed, skipped, results}`, with one `results` entry per named device, in request order: `{device_id, device_name, device_ip, status, error}`, where `status` is `success`, `failed` or `skipped` (an import also carries `config_id`). Drift answers `{total, in_sync, drifted, errors, skipped, results, started_at, completed_at, duration}`, where each result is `{device_id, device_name, device_ip, status, error, drift_summary, difference_count, drift}` and `status` is `synced`, `drift`, `error` or `skipped`. The enhanced route answers the comprehensive drift report instead, and the report covers the devices that ran: with `stop_on_error` the devices after the first error are not contacted and are not in it, so its totals add up. If the report cannot be generated or stored, that route still answers 200 with the plain drift result above. The calls are synchronous: there is no operation id, status or progress.

**Time limit of a read.** Import and both drift routes read the devices one after another and have no device cap, under the 30 second request deadline. They stop starting devices 10 seconds before that deadline (a dead device takes about 9 seconds to give up on, so a device started just before the cut ends in time) or when the client has left. Each device not started is `skipped` with the reason `not contacted: the time limit of this call was reached or the request ended` and is not contacted; repeat the call for those devices. The answer is then written before the deadline instead of a 408 with no result. A `stop_on_error` skip is the operator's own choice and keeps its meaning. On `bulk-drift-detect-enhanced` the two differ on purpose: a `stop_on_error` skip answers the report, which leaves the skipped devices out, and a skip for the time limit answers the `bulk-drift-detect` result, which names every device and why it was not read, with no report stored for the devices that did run. Do not unify them: the operator chose the stop and did not choose the time limit. One device that outlasts the margin can still pass the deadline (a device that accepts the connection and never answers took 28 seconds on import and 54 seconds on drift in the fake-device measurement of ginsys/shelly-manager#730), and the route then answers 408: the outcome of that device, and of the devices after it, is unknown. The 10 seconds are a proposal from one measurement, not a bound. A scheduled drift run is not under the request deadline: it stops starting devices after 10 minutes (a device already being read is not interrupted), and a run still marked running after that is marked `interrupted` by a later poll.

**Download.** `bulk-download` takes the request above (admin key; `options.stop_on_error: true` is refused with 400, since a read has nothing to stop, and `false` is the same as leaving it out) and names at most 200 devices (it reads the database only, so the cap bounds the answer held in memory, not device contact; `bulk-apply` keeps 10): more is 400, and nothing is read. It answers the file itself, not the `{success, data}` envelope, with `Content-Disposition: attachment; filename="shelly-configs-<UTC date and time>.json"`: `{"generated_at", "devices": [{"device_id", "device_name", "mac", "status", "config", "error"}]}`, one entry per named device in request order. `config` is the desired configuration (templates and overrides merged) with every secret that is set replaced by the mask `********`; a secret that is not set stays absent. `status` is `ok` or `error`. A device that cannot be read is an `error` entry with one fixed text (`device not found`, `no desired configuration is set for this device`, `the stored configuration has settings this version does not know, so it cannot be downloaded completely` for a stored field the typed configuration would drop, such as a legacy row restored by an import, or `the configuration could not be read`) and the call still answers 200. A device with an empty desired configuration is reported as having none, never as having no settings. The route reads the database and contacts no device.

**Apply.** `bulk-apply` takes `{"device_ids": [1, 2], "config": {...}, "options": {"stop_on_error": false}}` (admin key; the request contract above, plus `config`). `config` is a partial device configuration, the body of `PATCH /devices/{id}/overrides/new`: omitted fields stay, a field sent as `null` removes its override, a secret sent as the mask `********` keeps the stored one. It must be an object with at least one member, and a field the typed configuration does not know is refused with 400 (the PATCH route ignores it). It is stored as each named device's overrides and never as a template, so no device that is not named changes. At most 10 devices: more is 400, and nothing is read or written. For each device the route stores the overrides and then calls the per-device apply (`POST /devices/{id}/config/new/apply`), the only call that reaches a device, so the lock, the identity check and the read-back are the ones of that route.

Before the first write, `config` is checked against the model of every selected device. If any device cannot take it, the call writes to none: that device is `refused` with the setting paths, the other found devices are `skipped`, and nothing is stored. A device that is not found is `refused` with `device not found` and blocks no other.

The answer is `{total, applied, pending, unconfirmed, refused, failed, skipped, results}` with one entry per named device in request order: `{device_id, device_name, device_ip, status, stored, reason, failures, requires_reboot}`. `status` is `applied` (written and read back equal), `pending` (written and read back equal, but the desired configuration changed while it ran, so the device stays pending), `unconfirmed` (written, then the device stopped answering after a network change, so it is not verified), `refused` (nothing was written to the device), `failed` (the call did not end in a recorded, verified write) or `skipped` (not contacted). `stored` says whether the new overrides are stored; it is `null` where the storage call failed part way. A refusal after the overrides were stored (a wrong device at the address, an apply already running, settings the device model does not support, or a generation the apply has no path for) says in its reason that the new settings are stored and the device stays pending. A device whose stored generation the manager has no client for fails before it is contacted and is `failed`, with the generic reason, its overrides stored. `reason` is one fixed text. `failures` lists settings by path only. No value, no text of a device error and no warning text is in the answer.

The call is not atomic: device 3 can be applied while device 4 is refused, and nothing is rolled back. The overrides stay stored when the apply does not succeed, and the device stays pending until a per-device apply or a repeat of the call finishes it. `stop_on_error: true` stops at the first device that does not end `applied`; the rest are `skipped` and not contacted, and the devices already written stay written. Sending the same call twice repeats the writes. The call lasts as long as its devices take; there is no asynchronous job yet. It has a request deadline of its own, 5 minutes, where every other route has 30 seconds (the deadline is given to the route named `bulk-apply` and to no other), and a run budget of 4 minutes inside it. When the budget is spent, or the client has left, no later device is started: it is `skipped` with the reason `not contacted: the time limit of this call was reached or the request ended`, nothing is stored for it, and the device being written when the budget ends is cut by it and gets its honest status (usually `failed`, and pending). A client that leaves while a device is being written does not cut that device; the rest of the run is skipped and the server logs the counts. The budget and the deadline are proposals, not measurements. A 408 on this route can still happen, for example when a device ignores its context and the run passes the deadline, or when a step before the first write stalls. It then means the outcome is unknown: devices may have been written, so check them before sending the call again. A 408 on this route does not tell you that nothing was written.

**Bulk export is gone.** `POST /api/v1/config/bulk-export` and `POST /api/v1/devices/{id}/config/export` are gone and answer 404 (ginsys/shelly-manager#592). They wrote the stored import/export record (`device_configs`) to a device, beside the apply and without its lock, flags or desired configuration. The one device write is `POST /devices/{id}/config/new/apply`, which `bulk-apply` calls once for each named device.

**Changed behaviour.** These routes used to ignore the body and act on every device, and an empty or missing `device_ids` meant "all devices". It is now an error: a client that wants the whole fleet sends every id. The import answer used to carry `errors` (now `failed`), `ip` (now `device_ip`) and the status `error` (now `failed`).

---

### 9. Drift Detection Schedules (8 endpoints)

> **Scheduler off by default (#279):** a stored schedule runs on its five-field
> `cron_spec` only when the scheduler is on (`drift.scheduler_enabled`,
> `SHELLY_DRIFT_SCHEDULER_ENABLED=true`). With it off, create, update, toggle and
> delete still work and nothing runs. A run only reads devices, stops starting
> devices after 10 minutes, and its result is kept in the run history (the newest
> 100 runs per schedule). `GET /api/v1/config/drift-scheduler` answers
> `{"enabled": true|false}` for the server that answers it, so with several
> replicas configured differently the answers differ too. The UI page offers no
> create form yet: until it does, create a schedule through this API.

A body is `{"name", "description", "cron_spec", "enabled", "device_ids", "device_filter"}`
with `name` and `cron_spec` required. `cron_spec` is exactly five fields, in the
server's time zone: a sixth field, a descriptor such as `@hourly`, a `TZ=` prefix
and a spec that never fires are `400`. A schedule checks `device_ids` (each must
exist), or the devices `device_filter` (`{"type", "status"}`) selects, or every
device when it has neither; both at once is `400`, and an unknown key in the body
is `400`. `enabled` defaults to true on create and stays as it is on update. A
schedule saved by an older release with a six-field spec fails its first run once
and is disabled; it cannot be enabled again until an update gives it a valid
spec. `run_count` counts the ticks the scheduler claimed, runs and skipped ticks
alike. Deleting a schedule also deletes its device list and its runs.

Every schedule answer carries a `revision`. It moves when a user updates or toggles
the schedule, and when the scheduler disables it after a failure; a scheduler run
does not move it. An update (`PUT`) takes the `revision` the caller read in its body
and writes only if it is still the stored one. Another revision is `409` with the
current schedule in `error.details`, and nothing is written, so an old copy of a
schedule can never put its fields back. A body without `revision` is `400`. Create
has no `revision`, and sending one there is `400` as an unknown key. A toggle needs
no revision: it flips the stored flag.

| Method | Endpoint | Description | Status |
|--------|----------|-------------|--------|
| GET | `/api/v1/config/drift-schedules` | List drift schedules | ✅ Functional |
| POST | `/api/v1/config/drift-schedules` | Create schedule | ✅ Functional |
| GET | `/api/v1/config/drift-schedules/{id}` | Get schedule | ✅ Functional |
| PUT | `/api/v1/config/drift-schedules/{id}` | Update schedule | ✅ Functional |
| DELETE | `/api/v1/config/drift-schedules/{id}` | Delete schedule | ✅ Functional |
| POST | `/api/v1/config/drift-schedules/{id}/toggle` | Enable/disable schedule | ✅ Functional |
| GET | `/api/v1/config/drift-schedules/{id}/runs?limit=N` | Run history, newest first, without each run's `results` (`limit` 1 to 100, default 20; 400 on any other value, 404 for an unknown schedule, `[]` before the first run; `duration` is integer nanoseconds) | ✅ Functional |
| GET | `/api/v1/config/drift-scheduler` | Whether this server runs the scheduler (`{"enabled": bool}`) | ✅ Functional |

---

### 10. Drift Reporting (4 endpoints)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/config/drift-reports` | Get all drift reports |
| GET | `/api/v1/config/drift-trends` | Get drift trends over time |
| POST | `/api/v1/config/drift-trends/{id}/resolve` | Mark trend as resolved |
| POST | `/api/v1/devices/{id}/drift-report` | Generate device drift report |

**Drift Difference Model:**
```json
{
  "path": "relay.0.name",
  "expected": "Kitchen Light",
  "actual": "Light 1",
  "type": "modified",
  "severity": "warning",
  "category": "device",
  "description": "Relay name changed",
  "impact": "Device identification affected",
  "suggestion": "Sync config to restore expected value"
}
```

---

### 11. Export/Backup Operations (21 endpoints)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/export/plugins` | List available export plugins |
| GET | `/api/v1/export/plugins/{name}` | Get plugin details |
| GET | `/api/v1/export/plugins/{name}/schema` | Get plugin configuration schema |
| POST | `/api/v1/export/backup` | Create backup |
| GET | `/api/v1/export/backup/{id}` | Get backup details |
| GET | `/api/v1/export/backup/{id}/download` | Download backup file |
| DELETE | `/api/v1/export/backup/{id}` | Delete backup |
| GET | `/api/v1/export/backups` | List backups (compat) |
| GET | `/api/v1/export/backup-statistics` | Get backup statistics |
| POST | `/api/v1/export/json` | Create JSON export |
| POST | `/api/v1/export/sma` | Create SMA format export |
| POST | `/api/v1/export/yaml` | Create YAML export |
| POST | `/api/v1/export/gitops` | Create GitOps export |
| GET | `/api/v1/export/gitops/{id}/download` | Download GitOps export (a zip of the export directory) |
| DELETE | `/api/v1/export/gitops/{id}` | Delete the history record of a GitOps export (the files stay) |
| POST | `/api/v1/export` | Generic export |
| POST | `/api/v1/export/preview` | Preview export |
| GET | `/api/v1/export/{id}` | Get export result (from the history record after a restart) |
| GET | `/api/v1/export/{id}/download` | Download export |
| GET | `/api/v1/export/history` | List export history |
| GET | `/api/v1/export/history/{id}` | Get history item |
| GET | `/api/v1/export/statistics` | Get export statistics |

**Plugin status, and what "test" means.** Each plugin in `GET /api/v1/export/plugins` has a `status` with two keys,
`available` and `enabled`. Both are `true`: plugins are compiled into the server and registered at start-up, so a
listed plugin is always both. There is no `configured` key and no stored plugin configuration; the configuration is
part of each export or import request. There is no route that tests a plugin. To test one, send the same request to
`POST /api/v1/export/preview`: it validates the request and builds the preview from your data. For the OPNsense
plugin the preview does not contact the OPNsense host, so it does not check the API key and secret. Only an export
connects to the host first, and it fails with the connection error when they are wrong.

**Export Request Model:**
```json
{
  "plugin_name": "sma",
  "format": "json",
  "config": {},
  "filters": {
    "device_ids": [1, 2, 3],
    "device_types": ["SHSW-1"],
    "time_range": {"start": "...", "end": "..."}
  },
  "output": {"type": "file", "path": "/exports/"},
  "options": {
    "compression": "gzip",
    "include_metadata": true
  }
}
```

---

### 12. Import Operations (12 endpoints)

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/import/restore-preview` | Check a stored backup by `backup_id` (nothing is restored) |
| POST | `/api/v1/import/restore` | Stage a stored SQLite backup by `backup_id`; the server applies it at its next start (`dry_run` checks only) |
| GET | `/api/v1/import/restore` | The staged restore, and what the last start did with one |
| DELETE | `/api/v1/import/restore` | Cancel a staged restore |
| POST | `/api/v1/import/gitops` | Import GitOps config |
| POST | `/api/v1/import/gitops/preview` | Preview GitOps import |
| GET | `/api/v1/import/history` | List import history |
| GET | `/api/v1/import/history/{id}` | Get history item |
| GET | `/api/v1/import/statistics` | Get import statistics |
| POST | `/api/v1/import` | Generic import |
| POST | `/api/v1/import/preview` | Preview import |
| GET | `/api/v1/import/{id}` | Get import result (from the history record after a restart) |

**Import Options:**
```json
{
  "dry_run": true,
  "validate_only": false,
  "skip_errors": false
}
```

---

### 13. Notification System (10 endpoints)

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/notifications/channels` | Create notification channel |
| GET | `/api/v1/notifications/channels` | List channels |
| GET | `/api/v1/notifications/channels/{id}` | Get one channel |
| PUT | `/api/v1/notifications/channels/{id}` | Update channel (replaces name, type, enabled, config, description) |
| DELETE | `/api/v1/notifications/channels/{id}` | Delete channel |
| POST | `/api/v1/notifications/channels/{id}/test` | Test channel |
| POST | `/api/v1/notifications/rules` | Create notification rule |
| GET | `/api/v1/notifications/rules` | List rules |
| DELETE | `/api/v1/notifications/rules/{id}` | Delete rule and its delivery history |
| GET | `/api/v1/notifications/history` | Get notification history |

**Channel Types:** `email`, `webhook`, `slack`, `discord`

**Channel Config Example (Webhook):**
```json
{
  "name": "Ops Webhook",
  "type": "webhook",
  "enabled": true,
  "config": {
    "url": "https://hooks.example.com/alert",
    "method": "POST",
    "headers": {"Authorization": "Bearer ..."},
    "timeout": 30,
    "retries": 3
  }
}
```

---

### 14. Metrics & Monitoring (15 endpoints)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/metrics/prometheus` | Prometheus metrics export |
| GET | `/metrics/status` | Metrics collection status |
| POST | `/metrics/enable` | Enable metrics collection |
| POST | `/metrics/disable` | Disable metrics collection |
| POST | `/metrics/collect` | Trigger metrics collection |
| GET | `/metrics/dashboard` | Dashboard metrics summary |
| GET | `/metrics/ws` | WebSocket real-time stream |
| POST | `/metrics/test-alert` | Send test alert |
| GET | `/metrics/health` | Metrics system health |
| GET | `/metrics/system` | System metrics |
| GET | `/metrics/devices` | Device metrics |
| GET | `/metrics/drift` | Drift summary metrics |
| GET | `/metrics/notifications` | Notification metrics |
| GET | `/metrics/resolution` | Resolution metrics |
| GET | `/metrics/security` | Security metrics |

---

### 15. Discovery & Provisioning (3 endpoints)

| Method | Endpoint | Description | Input |
|--------|----------|-------------|-------|
| POST | `/api/v1/discover` | Discover devices on network | `{network, import_config}` |
| GET | `/api/v1/provisioning/status` | Get provisioning status | - |
| POST | `/api/v1/provisioning/provision` | Provision discovered devices | Device list |

---

### 16. Provisioner Agent Management (10 endpoints)

Every endpoint in this section, and `POST /api/v1/discover` and `POST /api/v1/provisioning/provision` above, requires a key (`Authorization: Bearer <key>` or `X-API-Key`) when the manager has an admin key configured, and answers `401` without it. The admin key opens every route. The agent key (`security.agent_api_key`, `SHELLY_SECURITY_AGENT_API_KEY`) is accepted on the five routes the agent itself calls: register, poll tasks, update task status, report discovered devices and health. Creating a task and listing agents, tasks or discovered devices need the admin key, and so does everything else; the agent key opens nothing else in the API. The provisioner agent sends the agent key as its `api.key` (`SHELLY_API_KEY`). The `Bearer ` prefix is case-sensitive.

**Each agent has its own identity.** The agent key alone opens register and health. Registering returns an `agent_token` in the response data, and the agent sends it as `X-Agent-Token` on the other three routes: poll tasks (the path id must be the token's agent), update task status (the task must be assigned to the token's agent) and report discovered devices (the body `agent_id` must be the token's agent; the report completes only that agent's own task). The answer is `401` when no known token is sent (the agent registers again), and `403` when the token belongs to another agent. The admin key needs no token and may act for any agent. Registering an id that is online (seen within 5 minutes) is refused with `409` unless the request carries that agent's current token or the admin key; a successful register replaces the token. A restarted agent has lost its token, so it waits until its old record is 5 minutes silent. With no admin key configured the routes stay open and no token is checked. With no admin key configured these routes stay open, and the agent key then has no effect. Rotating the admin key (`POST /api/v1/admin/rotate-admin-key`) does not affect the agent key.

**Ids that a key will own.** Every registry row has an enrolment state, and a row from before enrolment, or one a keyless agent registered, is `legacy` (the list does not show it yet). A keyless register of an id whose row is in any other state is refused with `409`, for the agent key, the row's own token and the admin key alike, however long the row was silent. A keyless register of an id that starts with `key-` in any case (`KEY-`, `Key-`) is refused with `400`: that prefix is reserved for the ids a key will derive. A keyless register that would create a new row is refused with `400` when its id holds a space or any byte outside `0x21` to `0x7E` (printable ASCII), unless the id starts with the exact bytes `provisioner-`, the prefix of the agents that exist today, after which any bytes are allowed. This keeps the folded lookup of a register from ever meeting another id: on MySQL 8.0, MySQL 5.7 and MariaDB 10.11 and 11.4, with the server default and `utf8mb4_general_ci`, `utf8mb4_unicode_ci`, `utf8mb4_unicode_520_ci` and `utf8mb4_0900_ai_ci`, two different printable ASCII bytes compare equal only as one letter in its two cases; a collation not listed there is not proven. A row that already exists keeps registering whatever its id holds. Nothing else changes for an agent that registers without a key.

**Where the registry is kept.** Agents, their tokens and the provisioning tasks are rows in the manager's database (`provisioner_agents` and `provisioning_tasks`), so a manager restart keeps them and every replica that shares the database sees the same registry. A token is stored only as its SHA-256 hash. The register answer returns the new token once, in `agent_token`, and no route returns a stored hash. Tasks stay in the table after they finish. A poll that finds no task answers `"tasks": []` (it was `null`). Two answers now follow the stored rows and no longer a shared in-memory object: an agent that was silent for more than 5 minutes and then reports devices is listed `online` again, and cancelling a task no longer changes a sibling task that a bulk create had given the same `config`. The registry is as durable as the database: with SQLite on a path that does not survive a restart (the development overlay puts the database in `/tmp`), the registry does not either. An agent id, a task type, a task status and a task `agent_id` may each be at most 191 characters; a longer value is refused with `400`. On MySQL the two tables use the binary collation `utf8mb4_0900_bin`, so ids compare byte for byte (`alpha` and `Alpha` are two agents), as they already do on SQLite and PostgreSQL. That collation needs MySQL 8.0.17 or later: on MariaDB or an older MySQL the manager logs a warning at start-up and keeps the default collation, where ids that differ only in case (on some older servers, also in trailing spaces) compare as equal.

**How the registry forgets.** An admin deletes an agent with `DELETE /api/v1/provisioner/agents/{id}`. That frees the id at once, whatever its last-seen time, and revokes its token: the agent's next call answers `401` and it registers again. The agent's `assigned` and `in_progress` tasks become `failed` with `config._error` giving the reason. Its `pending` tasks stay and go to whoever holds the id next. An unknown id answers `404`. A register also starts a purge in the background, at most once an hour per manager process, and the register's own answer never depends on it: a purge that fails is logged. The purge removes agents last seen more than 30 days ago that have no `pending` task (their `assigned` and `in_progress` tasks fail as above), and deletes `completed` and `failed` tasks last updated more than 30 days ago. Before the registry was kept in the database, a restart cleared every task. Finished tasks now stay until a purge removes them: they become eligible after 30 days, and the purge runs only when an agent registers.

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/provisioner/agents/register` | Register provisioner agent, returns `agent_token` (agent or admin key; `409` for an online id or an enrolled one; `400` for an id starting `key-`, and for a new id with a space or a non-ASCII byte unless it starts `provisioner-`) |
| GET | `/api/v1/provisioner/agents` | List registered agents (admin key) |
| DELETE | `/api/v1/provisioner/agents/{id}` | Delete an agent: frees its id, revokes its token, fails its assigned tasks (admin key; `404` for an unknown id) |
| GET | `/api/v1/provisioner/agents/{id}/tasks` | Poll tasks for agent (agent key and that agent's token, or admin key) |
| POST | `/api/v1/provisioner/tasks` | Create provisioning task (admin key) |
| GET | `/api/v1/provisioner/tasks` | List tasks (admin key) |
| PUT | `/api/v1/provisioner/tasks/{id}/status` | Update task status (agent key and the token of the task's agent, or admin key) |
| POST | `/api/v1/provisioner/discovered-devices` | Report discovered devices (agent key and the token of `agent_id`, or admin key) |
| GET | `/api/v1/provisioner/discovered-devices` | Get discovered devices (admin key) |
| GET | `/api/v1/provisioner/health` | Provisioner health check (agent or admin key) |

---

### 17. DHCP (1 endpoint)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/dhcp/reservations` | Get DHCP reservations |

---

### 18. Admin Operations (1 endpoint)

| Method | Endpoint | Description | Input |
|--------|----------|-------------|-------|
| POST | `/api/v1/admin/rotate-admin-key` | Rotate API key (`400` for a blank key or one that starts or ends with whitespace) | `{new_key}` |

---

## Standardized Response Format

All API responses follow this envelope:

```json
{
  "success": true,
  "data": { ... },
  "error": {
    "code": "ERROR_CODE",
    "message": "Human readable message",
    "details": { ... }
  },
  "meta": {
    "pagination": {
      "page": 1,
      "page_size": 20,
      "total_pages": 5,
      "has_next": true,
      "has_previous": false
    },
    "count": 20,
    "total_count": 100,
    "version": "v1"
  },
  "timestamp": "2025-11-30T12:00:00Z",
  "request_id": "abc123"
}
```

---

## Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `BAD_REQUEST` | 400 | Invalid request format |
| `UNAUTHORIZED` | 401 | Authentication required |
| `FORBIDDEN` | 403 | Insufficient permissions |
| `NOT_FOUND` | 404 | Resource not found |
| `METHOD_NOT_ALLOWED` | 405 | HTTP method not supported |
| `CONFLICT` | 409 | Resource conflict |
| `VALIDATION_FAILED` | 400 | Input validation error |
| `RATE_LIMIT_EXCEEDED` | 429 | Too many requests |
| `REQUEST_TOO_LARGE` | 413 | Payload too large |
| `INTERNAL_SERVER_ERROR` | 500 | Server error |
| `DEVICE_NOT_FOUND` | 404 | Device does not exist |
| `DEVICE_OFFLINE` | 503 | The stored status of the device is offline (control, status, energy and config drift). Control, energy and drift send nothing to it; control accepts `"force": true` to try anyway. Status probes a device last seen within 5 minutes once first and answers 503 if that probe fails or cannot be set up; for any other offline device it sends nothing. Not `DEVICE_UNREACHABLE` (504), which is a read that got no answer in time |
| `DEVICE_UNREACHABLE` | 504 | Device did not answer in time (status, energy and current-config reads) |
| `CONFIGURATION_ERROR` | 400 | Config operation failed |
| `TEMPLATE_ERROR` | 400 | Template processing error |
| `EXTERNAL_SERVICE_ERROR` | 502 | A device rejected its stored login on a route that talks to the device: the four live reads (`/devices/{id}/status`, `/energy`, `/config/current` and `/current/normalized`), apply and verify (`/devices/{id}/config/new/apply` and `/config/new/verify`). Store the right login with `PUT /devices/{id}/config/auth`. |

`PUT /devices/{id}/config/auth` takes `{"username", "password"}`. A username with an empty password returns 400 `VALIDATION_FAILED`. An empty username turns device auth off.

**Changed answers.** Five answers did not carry the envelope and now do. `GET /devices/{id}/config/current/normalized` with an id that is not a number was HTTP 200 with a string `error`, and is now 400 `BAD_REQUEST`. A request whose `Content-Length` is over the size limit was 413 with no body, and is now 413 `REQUEST_TOO_LARGE`. `GET /provisioning/status`, `POST /provisioning/provision` and `GET /dhcp/reservations` answered a bare object or array, and a client that read it now reads it from `data`.

---

## Security & Middleware

### Middleware Stack (15 layers)
1. Recovery (panic handling)
2. IP Blocking
3. Security Monitoring
4. Security Logging
5. Security Headers (CSP, HSTS, etc.)
6. Request Timeout (30s default)
7. Rate Limiting (1000 req/hour default)
8. Request Size Limiting (10MB)
9. Header Validation
10. Content-Type Validation
11. Query Parameter Validation
12. JSON Validation (depth: 10, array: 1000)
13. CORS
14. HTTP Logging
15. Prometheus Metrics

### Rate Limits by Path
| Path Pattern | Limit |
|--------------|-------|
| Default | 1,000/hour |
| `/devices/{id}/control` | 100/hour |
| `/provisioning/*` | 50/hour |
| `/config/bulk*` | 20/hour |

The default limit counts every request of a client IP. A path limit counts only the requests to its
own paths, per client IP, with its own one-hour window. `/devices/{id}/control` counts per device: 100
calls an hour for each device id. Going over a path limit refuses the calls to that path until its
window resets and blocks no other route. Going over the default limit refuses the client on every
route for 15 minutes, and after that until its window resets. Every refusal is
`429 RATE_LIMIT_EXCEEDED` with `X-RateLimit-Remaining: 0`. A refusal by a path limit shows that limit
in `X-RateLimit-Limit`; a refusal by the default limit shows the default limit. In both,
`X-RateLimit-Reset` (Unix seconds) and `meta.retry_after` (seconds) name the first whole second at
which the client is let in again: after the path limit's window, or after the later of the default
limit's window and the 15-minute block. A request that passes carries no `X-RateLimit-*` header. When
several path patterns match, the longest wins.

### Client Requirements
- Every request needs a `User-Agent` header. An empty one is treated as an
  attack signal: the request fails with `400 VALIDATION_FAILED` and the security
  monitor counts it as suspicious activity from that client IP, which it can
  block. Browsers, `curl`, `wget`, `python-requests`, `axios`, `node-fetch` and
  Go's `net/http` send one by default. Node's core `http` module does not, so a
  client built on it must set one. Known scanner names such as `sqlmap` or
  `nmap` are refused too.
- `POST`, `PUT` and `PATCH` requests need a `Content-Type` header. A missing one
  fails with `400 VALIDATION_FAILED`.
- A client that sends no `User-Agent` (for example `curl -H 'User-Agent:'`) must
  set one.

### Authentication
- Header: `Authorization: Bearer {api_key}`
- Header: `X-API-Key: {api_key}`

**Admin key.** If the server has an admin key configured, these configuration routes need it and return 401 `UNAUTHORIZED` without it: every route under `/api/v1/devices/{id}/config` (the stored, live, typed, import, drift, history, apply-template, capability and apply routes), `/api/v1/devices/{id}/overrides/new`, `/api/v1/devices/{id}/desired-config` and `/api/v1/devices/{id}/drift-report`. The typed conversion and validation routes (`/api/v1/config/validate-typed`, `convert-to-typed`, `convert-to-raw`, `bulk-validate`, `common-settings`) and the drift schedule and drift report routes under `/api/v1/config` need it too, as do the bulk import, drift and download routes, which check it in their handlers. The template routes under `/api/v1/config/templates`, the template assignment routes and `GET /api/v1/config/schema` need it too. If no admin key is configured, the routes are open. The web UI sends the key it holds on every call. `GET /api/v1/devices/{id}/capabilities` needs it too.

---

## Key Files Reference

| File | Purpose |
|------|---------|
| `internal/api/router.go` | Route registration |
| `internal/api/handlers.go` | Core device/config handlers |
| `internal/api/typed_config_handlers.go` | Typed config handlers |
| `internal/api/sync_handlers.go` | Export/backup handlers |
| `internal/api/import_handlers.go` | Import handlers |
| `internal/api/provisioner_handlers.go` | Provisioner handlers |
| `internal/api/response/response.go` | Response formatting |
| `internal/api/middleware/security.go` | Security middleware |
| `internal/api/middleware/validation.go` | Validation middleware |
| `internal/notification/handlers.go` | Notification handlers |
| `internal/metrics/handlers.go` | Metrics handlers |
| `internal/database/models.go` | Database models |
| `internal/configuration/models.go` | Config models |
| `internal/sync/data.go` | Export/import models |

---

## Summary Statistics

- **Total Endpoints**: 112+
- **Handler Modules**: 6 main files
- **HTTP Methods**: GET, POST, PUT, DELETE, OPTIONS
- **API Version**: v1 (single version)
- **WebSocket**: `/metrics/ws` for real-time data
- **Export Formats**: JSON, YAML, SMA, GitOps
- **Notification Channels**: Email, Webhook, Slack, Discord
