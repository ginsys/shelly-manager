# Changelog

All notable changes to this project are documented here. The project follows Conventional Commits.

## [Unreleased]

## [0.7.0-beta.0] - 2026-11-24

### Added
- Apply and verify on a Gen2 or Gen3 device now also write the login: `auth.enable`, `auth.user`
  and `auth.pass` go out last with `Shelly.SetAuth`, which takes the user (always `admin`), the
  `realm` (the device id) and `ha1`, the SHA-256 of `user:realm:password`, never the password.
  The password is write-only, and the new login is stored once the device accepted it. The
  read-back is `auth_en` from the device info. Refused with 422 before anything is written: a user
  other than `admin`, a login on with no password that can be sent, a login off with a user or
  password, `auth.realm`, and a login change in the same apply as a Wi-Fi network change. `SetAuth`
  is never sent twice, and a failure never carries the device's text (it can echo `ha1`). An
  answer to `SetAuth` that never arrives is a failure that stores nothing, and the response
  warns that the stored login may now be stale. Not confirmed on hardware (read from the Shelly Gen2 documentation):
  whether `SetAuth` restarts the device, and what an absent `ha1` does. Part of
  ginsys/shelly-manager#381.
- Apply and verify on a Gen2 or Gen3 device now also write the Wi-Fi station: enable, ssid,
  password, IPv4 mode and the static address (`ip`, `netmask`, `gw`, `nameserver`) go out in one
  `WiFi.SetConfig` call, after the system, MQTT, cloud, switch and input sections, and are read
  back from `wifi.sta` in `Shelly.GetConfig`. The password is write-only: the device never
  returns it, and while an apply holds one the text of every error of a failed write is replaced
  by a fixed message. A password may be sent alone. A `ssid` is written only with a password;
  where the configuration names the network the device is already on and sets no password, the
  `ssid` is left out of the write. Refused with
  422 before anything is written: a new `ssid` with no password that can be sent, and switching
  the station off (`wifi.enable` false, because the manager reaches the device through the
  station). A device that cannot be read before a Wi-Fi `ssid` write is
  refused too, with the error of a failed device read. A wrong static address strands the device.
  After a change of network the device may stop answering: see the `unconfirmed` entry below. Not confirmed
  on hardware (read from the Shelly Gen2 WiFi reference): every key, the read-back and what
  `restart_required` means for these keys. Part of ginsys/shelly-manager#381.
- Apply and verify on a Gen2 or Gen3 device now also write the Wi-Fi access point and the roam
  threshold: `wifi.ap.enable`, `wifi.ap.ssid` and `wifi.ap.pass` in a `WiFi.SetConfig` call
  with `ap`, `wifi.roam_threshold` in one with `roam` (`rssi_thr`), both before the station
  call, read back from `wifi.ap` and `wifi.roam`. The access point password is write-only like
  the station password and may be sent alone. `wifi.ap.key` is refused (422): the Gen2 access
  point has one secret, `pass`. Two rules refuse before anything is written (422): switching the
  access point off needs a steady station (the device reports it enabled with a `ssid`, and the
  apply changes neither its network nor its password), so the device keeps a way back; and switching an access point on
  needs a `wifi.ap.pass` in the same configuration, or a device that reports it closed, because a
  Shelly access point is open until it has a password. An access point that already runs open
  needs no password to stay enabled, and is left open (a password in the configuration is still sent). A station network change on a device whose access point is off is not checked
  (the documented risk of a wrong static address). `roam.interval` is not written. Not confirmed
  on hardware (read from the Shelly Gen2 WiFi reference): every key, what `ap.enable` false
  does, and whether the access point shuts itself when the station connects. Fixes
  ginsys/shelly-manager#544.
- A Gen2 or Gen3 apply that changes the Wi-Fi station network (a different `ssid`, `enable`,
  `ipv4mode`, `ip`, `netmask` or `gw`; the password and name server never count) now reports
  `unconfirmed: true`, as Gen1 does, when the device then gives no answer to the reboot or to
  the read-back. It is neither a success nor a failure, and the device stays pending. A station
  write that gets no answer is not a failure either: it may have landed, so the apply goes on to
  the read-back (silence there is `unconfirmed`, a match is a success, a difference is reported)
  and requests no reboot for it. A device that answers with an error, or a reply that is not a
  configuration, keeps its error. Any station write of those
  keys now reads the device first and is refused when it cannot be read (before, only an
  `ssid` did). The stored address is not changed: after a static address change, re-discover the
  device or correct its address, then apply again. Fixes ginsys/shelly-manager#545 and
  ginsys/shelly-manager#550.
- Apply to a Shelly 2.5 in roller mode now writes the roller keys beyond the travel times,
  direction and inputs, with `POST /settings/roller/0`, and reads them back: `default_state`,
  the button type (`button_type`, sent as `btn_type`, read back as `button_type`),
  `button_reverse` (`btn_reverse`), `schedule`, `schedule_rules`, `obstacle_mode`,
  `obstacle_action`, `obstacle_power`, `obstacle_delay`, `safety_mode`, `safety_action`,
  `safety_allowed_on_trigger`, `off_power` and `ends_delay`. A value outside the reference's list,
  a negative power or delay, an `ends_delay` outside 0 to 10000 or off the step of 100, and an
  empty or comma-holding schedule rule or more than 20 rules are refused with 422 before anything
  is written. An omitted `schedule_rules` leaves the rules alone, an empty list clears them, and
  verify compares the whole list. Positioning and its calibration are not applied. Not confirmed
  on hardware: the key names and values, read from the Gen1 reference for the Shelly 2.5, and the
  differing types the reference gives for `btn_reverse` and `positioning`. Part of
  ginsys/shelly-manager#381.
- Apply to a Gen1 device now writes `appliance_type`, `schedule`, `schedule_rules` and `max_power`
  of each relay channel, next to `name`, `default_state`, `auto_on` and `auto_off`, and reads
  them back. `schedule_rules` goes out as one comma-separated parameter. A negative `max_power`,
  an empty schedule rule, a rule with a comma and more than 20 rules are refused with 422
  before anything is written. Fixed: on a Shelly 2.5 in relay mode, `power_metering.max_power`
  was written to `POST /settings`, where the Shelly reference says it applies "for roller mode
  only". It is now written to each channel that states no `max_power` of its own, and `POST
  /settings` is used for it in roller mode and on every other model. Not confirmed on hardware
  (read from the Shelly reference): the comma-separated rule form, the rule bound and that no
  reboot is needed. Part of ginsys/shelly-manager#381.
- Apply to a Gen1 device now writes its Wi-Fi settings. The station goes out with
  `POST /settings/sta` (`enabled`, `ssid`, `key`, `ipv4_method`, and for a static address `ip`,
  `netmask`, `gateway`, `dns`) and the access point with `POST /settings/ap` (`enabled` and
  `key` only), each only for the keys the desired configuration sets, then both are read
  back from `GET /settings`. A new `ssid` with an empty or redacted password, an access point
  name or key, `roam_threshold`, and a station and an access point both enabled are refused
  with 422 before anything is written. A failed `GET /settings` before the write also writes
  nothing and returns the read's own error. A refused apply leaves `config_applied` as it was.
  The password is never sent empty or redacted, never logged and never echoed. A Wi-Fi change
  no longer reports `requires_reboot`. If a write changes the station network (`ssid`,
  `enabled`, `ipv4_method`, `ip`, `netmask` or `gateway`) and the device then gives no answer
  to the read-back, the apply response has `unconfirmed: true`: neither success nor failure,
  the device stays pending, the sections after Wi-Fi are not written (a warning says so), and
  the device page shows it as its own state. A station write whose own response is lost
  counts the same: it is read back, and silence there is unconfirmed. A device that answers
  with an error is a failure, even if the retries after that answer got none. The Gen1 client now keeps an HTTP
  error from an earlier attempt, on a read or a write, when a later retry fails at the
  transport. The converter now reads
  the `mask` key a real device reports, and still accepts `netmask` in old data. Not confirmed
  on hardware (read from a summary of the Shelly reference): the route behaviour, the key
  names, `enabled` as 1 or 0, that no reboot is needed and that `GET /settings` never returns
  the key. Part of ginsys/shelly-manager#381.
- Apply and verify on a Gen2 or Gen3 device now also write the switch and input settings.
  Each input is written with `Input.SetConfig` (`name`, `type` as `switch` or `button`, and
  `inverted` as `invert`), then each switch with `Switch.SetConfig` (`name`, `default_state` as
  `initial_state`, `auto_on` and `auto_off`, and the `mode` of input N as `in_mode`), one call
  per channel, after the system, MQTT and cloud sections, and read back from
  `Shelly.GetConfig`. A Gen1 word of the same meaning is accepted and read as the Gen2 word:
  `last` is `restore_last`, `switch` is `match_input`, `toggle` is `follow` and `edge` is
  `flip`; verify treats each pair as one value. `auto_on` and `auto_off` are seconds: above 0
  is the timer on with that delay, 0 is the timer off. A timer that the device keeps on with a
  delay that is not a whole number of seconds above 0 is read as unset, so verify reports a
  difference. Verify pairs a channel with the device channel of the same id; a list with a
  repeated id is compared by position, as before. The mode of input N is written as the
  `in_mode` of switch N. Where a model lets a switch take another input (`input_id`), apply
  reads the device first and refuses the mode with 422, because the write would change the
  input that switch runs and not input N; the path is `input.inputs.I.mode`, with I the position
  in the desired list. A channel list that holds one id twice is refused too, at the second
  entry (for example `relay.relays.1.id`), whether or not the entries set anything, because it
  cannot say which channel the settings belong to. Refused with 422 before anything is
  written, with one path for each setting: any other value (the Gen1 mode `action`, an input
  type `analog` or `count`, a negative timer), an input push action or timing, a relay
  schedule and the top-level relay and input settings. `cycle` and `activate` are left to the device, which answers with its
  own error on a model that lacks them. Saving a configuration for a Gen2 model, or a template
  with no model, now accepts the words `restore_last` and `match_input` (initial state) and
  `cycle`, `activate`, `toggle` and `edge` (input mode); a supported Gen1 model keeps its own words, and a Gen1 apply refuses `restore_last` and
  `match_input` with 422. Not confirmed on hardware (read from the Shelly
  Gen2 Switch and Input reference): every key, the read-back and whether a write needs a
  restart. Part of ginsys/shelly-manager#381.
- Apply and verify on a Gen2 or Gen3 device now also write the MQTT and cloud settings. MQTT:
  enable, server and port (one `host:port` string on the device), user, password, client ID,
  topic prefix and the RPC and status notification flags go out with `MQTT.SetConfig`. Cloud:
  enable and server go out with `Cloud.SetConfig`. The password is write-only: it is sent only
  with a user, never when it is empty or the mask `********`, never compared, and never shown in
  a failure or a log line. While an apply holds a password, the text of every device error is
  replaced by a fixed message, because a device may echo the request in any encoding. Other
  MQTT settings (keep alive, clean session, update period, retained-message limits,
  certificate and SNI flags) are refused with 422 before anything is written, and so is a
  password with no user, a server that already carries a port beside a separate port, and an
  IPv6 endpoint in one string (give the host and the port apart). Not confirmed on hardware: the
  read-back of each key, and whether a section whose write the device refused leaves the
  earlier sections written. Part of ginsys/shelly-manager#381.
- Apply and verify now work on a Gen2 or Gen3 device, for the system section only: device
  name, eco mode, discoverable, time zone, latitude, longitude and time server go out with
  `Sys.SetConfig` and are read back from `Shelly.GetConfig`. Only the keys the desired
  configuration sets are sent, because the Shelly documentation does not say that
  `SetConfig` merges. The device decides about a reboot: the apply reboots only when the
  answer says `restart_required`, and keeps that reboot owed if it fails. Any other section
  (MQTT, cloud, Wi-Fi, login, switches, inputs) and a time server written as `host:port` are
  refused with 422 before anything is written; later changes add the sections. Refs
  ginsys/shelly-manager#381.
- Apply to a Gen1 device now writes the button type and the reverse flag of its
  inputs. For each entry of `input.inputs`, `mode` goes out as `btn_type` (`momentary`,
  `follow` as `toggle`, `flip` as `edge`, `detached`) and `inverted` as `btn_reverse`,
  and each input is read back. A Shelly 1, 1PM or 2.5 keeps both in its relay channel; a
  Shelly i3 has its own `/settings/input/<n>` route, and `GET` on it is new in the Gen1
  client. A Shelly i3 accepts `momentary` and `follow` only; any other mode, an input
  `name`, `type` or push action, and an `id` that differs from its position are refused
  with 422 before anything is written. The converter now reads and writes the keys a real
  device has (`btn_type`, `btn_reverse`), and a Shelly 1, 1PM or 2.5 reports its inputs
  from its relays. The parameter names, the toggle and edge mapping,
  the i3 mode set and whether a button type change needs a reboot were read from the Shelly
  reference and are not confirmed on hardware. Part of ginsys/shelly-manager#381.
- The device page has a Device Login card for the username and password the manager
  uses to reach a device. Until now only the API could store them, and a device that
  rejected the stored login gave the user no way to fix it in the UI. A saved password
  shows as a mask and is kept when the mask is sent back, an empty username turns
  device authentication off, and a username without a password is refused before
  anything is sent. (ginsys/shelly-manager#137)
- Applying a configuration to a Gen1 device with roller mode (Shelly 2.5) now writes five
  roller settings: the open and close travel times, the motor direction, the input swap
  and the input mode. Each one is written with POST /settings/roller/0 and read back, and
  a value the device keeps unchanged is reported as a failed path. A travel time below one
  second, a value with no Gen1 spelling and every other roller field are refused before
  the device is touched. The key names are unverified until a hardware readback, and a
  device in relay mode refuses the endpoint. (ginsys/shelly-manager#504)

### Changed
- **Behaviour change:** `POST /devices/{id}/config/export` and `POST /config/bulk-export` are
  removed and answer 404. They wrote the stored import/export record (`device_configs`) to a
  device beside the apply: no apply lock, no `config_applied` or `pending_reboot` update, and not
  the desired configuration. A client that wrote a device through them uses the apply,
  `POST /devices/{id}/config/new/apply`, which sends the desired configuration (templates and
  overrides). A bulk apply does not exist yet (ginsys/shelly-manager#594). The unused
  `ResolutionService` in `internal/configuration` went with them. ginsys/shelly-manager#592.
- **Behaviour change:** `PUT /devices/{id}` no longer writes `template_ids`, `overrides`,
  `desired_config` or `config_applied`. A body that carries them changes none of them, and the
  answer shows the stored values. A client that set them through the device `PUT` uses the owning
  route instead: `PUT /devices/{id}/templates/new`, `PUT` or `PATCH /devices/{id}/overrides/new`,
  and the apply for `config_applied`. `desired_config` is derived from the templates and
  overrides. A body without `updated_at` is now saved against the row the server reads when the save
  starts, so a writer that saves between that read and the save makes it answer 409, as a
  versioned body already did.
  (ginsys/shelly-manager#572)
- An apply waits for the device lock under one deadline: the database lock no longer sets a
  second, inner one, and the caller's wait (15 seconds) is the only budget. The answers do not
  change (409 after the wait, the request's own error when it ends first). The OpenAPI entry of
  `POST /api/v1/devices/{id}/config/new/apply` now lists the 500 that answers an apply whose database
  lock was lost while it ran (the device may hold a mix of two applies, and the apply is repeated),
  and the 400 and 408 it already returned. ginsys/shelly-manager#582.
- The typed-configuration routes answer with the standard `{success, data | error}` envelope on
  success and on failure. `PUT /devices/{id}/config/typed`, `POST /config/validate-typed`,
  `POST /config/bulk-validate` and `GET /devices/{id}/config/typed/normalized` used to answer
  a bare object, so a client that read the bare object now reads it from `data`, and the
  normalized read puts its configuration at `data.configuration`. The failures of `PUT`,
  `validate-typed` and `GET /devices/{id}/capabilities`, and a malformed `convert-to-typed`
  body, used to be plain text and are now JSON: 400 `BAD_REQUEST`, 404 `NOT_FOUND` or 500 `INTERNAL_SERVER_ERROR`,
  with no internal detail on a 500. The failures of the normalized read, and a failed
  conversion on `GET /devices/{id}/config/typed`, were JSON with a string `error` and now
  carry the structured `error`. A refused `PUT /devices/{id}/config/typed` used to answer
  `{error, validation}` and now answers 400 `VALIDATION_FAILED` with the validation result
  in `error.details`. The request failures of `convert-to-raw` and `bulk-validate` are 400
  `BAD_REQUEST` with the specific message, where they were 400 `VALIDATION_FAILED`.
  `convert-to-typed` without a `configuration` is 400, where it was a 500 with the same envelope. The success body of
  `GET /devices/{id}/capabilities` stays bare. The request keys are `configuration`,
  `device_model` and `configurations`, and the API docs now give the routes under their real
  `/config/` paths. (ginsys/shelly-manager#251)
- The four legacy capability routes are removed: `PUT /devices/{id}/config/relay`,
  `/dimming`, `/roller` and `/power-metering`. Each one wrote its section into the
  stored configuration of the device and nothing applied it to the device, and no page
  called them. A caller that used them must use the typed configuration routes. `PUT /devices/{id}/config/auth` stays. The device page now lists
  the capabilities that `GET /devices/{id}/capabilities` returns, where it used to guess
  them from the model name, and that route now lists `roller` for the model whose schema
  has a roller section. The configuration editor says "not applied" on a section that
  apply never writes, such as the network section. (ginsys/shelly-manager#490)
- The time server (`system.sntp.server`), the MQTT server (`mqtt.server`) and the CoIoT peer
  (`coiot.peer`) are checked when the configuration is saved, not only at apply. The two servers
  take an IPv4 address or a host name, the one rule apply uses; the MQTT server may also carry
  `:port`, as a stored broker does, or be a bare IPv6 address with the port in its own field (the
  bracketed one-string form `[addr]:port` is refused, as the Gen2 apply refuses it). The peer takes `host:port` with a
  port from 1 to 65535, or nothing for multicast. The form shows the error under the field, and the
  override and template routes answer 400 `VALIDATION_FAILED` with the field in `error.details.fields`.
  An empty value passes. A Gen1 apply now refuses an IPv6 MQTT server (422 `VALIDATION_FAILED`
  naming `mqtt.server`); it used to write it joined to the port as `fd00::1:1883`, which the device
  cannot parse. A name whose last label is all digits must be a valid IPv4 address, so
  `192.168.1.300` is refused here and at apply, where it used to pass as a host name.
  (ginsys/shelly-manager#523)
- The configuration form shows a default only on a field that apply writes, and only one that a
  source states: `auth.enable` off, `mqtt.update_period` 30 seconds and `system.sntp.server`
  `time.google.com` from the Shelly Gen1 API reference, and `mqtt.port` 1883 from OASIS MQTT 5.0
  section 4.2, which registers TCP port 1883 with IANA for MQTT (paraphrase). A field apply never writes (the flat
  `relay.default_state`, the network section, `led.enabled` and others) shows none, and so do the
  applied fields no source gives a default for (`relay.relays[].default_state`, `cloud.enable`,
  `coiot.enable`, `coiot.update_period`, `input.inputs[].mode`, `mqtt.clean_session`,
  `mqtt.keep_alive`, `system.device.discoverable` and the rest). Three of those used to show an
  unsourced default: true, 60 and true. A default is a hint only: a port left empty is still
  written as the bare host. The defaults live in one table, `gen1FieldDefaults`, and a Go test
  fails when an applied path has neither a default nor a place on the list of those without one.
  (ginsys/shelly-manager#524)

### Removed
- Dead frontend and backend code that nothing called: the `typed_config_handlers.go.backup`
  file, the `TypedConfigForm`, `ValidationResults` and `CapabilitiesViewer` components, the
  `openMetricsWebSocket` helper, seven functions of `api/deviceConfig.ts`, and the six
  typed-configuration client functions with the store actions that called them. No page
  reached any of it, so nothing changes for a user or an API client. (ginsys/shelly-manager#256)

### Fixed
- The four bulk configuration routes (`bulk-import`, `bulk-export`, `bulk-drift-detect` and
  `bulk-drift-detect-enhanced`) now act on the devices named in `device_ids` and on no others.
  They ignored the body and acted on every device, so a request for two devices wrote to or read
  the whole fleet. **Behaviour change:** a request with no body, invalid JSON, a missing or empty
  `device_ids`, or an id that is not a positive integer is now 400 `VALIDATION_FAILED` and
  contacts nothing; an empty or missing `device_ids` used to mean "all devices". A field or option
  the server does not read (`deviceIds`, `configurations`, `validate_only`) is refused with 400
  naming it. The one option is `options.stop_on_error`, which reports the devices after the first
  failure as `skipped` without contacting them. Import and export now answer
  `{total, success, failed, skipped, results[{device_id, device_name, device_ip, status, error}]}`
  where they answered `errors`, `ip` and the status `error`; drift keeps its shape and gains
  `skipped`. Bulk export writes the stored import/export record, takes no apply lock and never
  changes `config_applied` or `pending_reboot`, as the per-device export does
  (ginsys/shelly-manager#592 tracks moving it onto apply). The web UI types for these routes
  follow in a separate change. Fixes ginsys/shelly-manager#248.
- A malformed PostgreSQL or MySQL DSN no longer puts a password into an error or a log line. The
  drivers print what they parsed (the database name, host and user of pgx, the value a MySQL parse
  refused, the address of a dial), and the old sanitizer could not know where a credential began.
  The DSN is now parsed once with the driver's own parser, a DSN it refuses reports a fixed
  message, and the pool opens on a connector whose every attempt (first connect, pool growth,
  reconnect) fails with a classification built from the error's type, such as `database connection
  failed: connection refused`, plus the server's SQLSTATE or MySQL error number where there is one.
  The GORM log writer gets the same text, the MySQL driver's own stderr logger is silenced for
  the provider's connector, and the connect log line names only the target (`postgresql (DSN
  redacted)`), no longer a host and a database. The sanitizers and the host and database helpers
  are removed. A SQLite DSN is printed without its query (`DatabaseConfig.String()`, the connect log
  and the config's database target). Diagnostics lost: the driver's own wording of a connect
  failure (host, user and database in it) and a server's message. Behaviour notes: the GORM
  PostgreSQL dialector no longer reads a `TimeZone=` DSN key (pgx still sends it as the session
  `timezone`, and no repository DSN sets it); a SQLite DSN with a query now takes its directory
  from the path part only, so a stray directory is no longer made from a query value. Not covered
  here: the text of a query error on a connection that is already open after `Connect` returned
  (the ping and the version probe of `Connect` are sealed, and GORM logs only once they are
  done), and the value a SQLite driver quotes when it refuses an invalid `_` parameter.
  (ginsys/shelly-manager#587)
- The bulk operations types and dialog in the web UI now match the bulk routes. The request is
  `{device_ids, options: {stop_on_error}}` and the answers are typed as the backend sends them
  (`failed` and `skipped` counts, `device_name` and `device_ip`; no operation id or overall status,
  and no timestamps on an import answer). The dialog no longer offers what the routes never did: the
  JSON to apply, "validate only", the export format, secrets and metadata, and the comparison,
  threshold and history options. It no longer offers bulk export either: that route writes a second,
  unlocked copy of the configuration and is being retired in favour of the apply
  (ginsys/shelly-manager#592), so the dialog points to the per-device apply for writing a
  configuration to a device. Import reads the devices into the manager and the dialog says so. The
  dialog is still not mounted (ginsys/shelly-manager#136); applying
  operator JSON to selected devices and downloading their configuration are
  ginsys/shelly-manager#594 and ginsys/shelly-manager#595. Fixes ginsys/shelly-manager#248.
- A device writer saves only the columns it owns, and a whole-row save exists only where it is
  versioned. The discovery upsert, the GitOps import and the CLI discover command saved the whole
  row they had read, so a flag, a setting or a desired configuration that another writer committed
  in between was written back as it was read. The first two now write the discovery columns
  (address, firmware, status, last seen, and the type only when the discovery found one) or the
  name and the type, and move `updated_at`. The CLI discover command writes nothing of its own
  after the discovery has stored the device. The unversioned whole-row `UpdateDevice` and the unused
  `UpsertDevice` are removed. Not covered here: a discovery pass moves `updated_at` even when
  nothing changed, so a person's next save can meet a 409 (ginsys/shelly-manager#583).
  (ginsys/shelly-manager#572)
- The Kubernetes kustomize manifests now configure the app. They set unprefixed names (`DATABASE_PROVIDER`,
  `LOG_LEVEL`, `API_URL` and so on) that the app never reads, because it reads only `SHELLY_` plus the
  config key, so every deploy from them ran the SQLite config baked into the image whatever the overlay
  said. Every name in the base ConfigMap, the provisioner DaemonSet and the three overlays is now the name
  the app reads (`SHELLY_LOGGING_LEVEL`, `SHELLY_PROVISIONING_AUTO_PROVISION`, ...); the dead `GIN_MODE`
  entries are gone. **A deploy from the staging or production overlay now runs on PostgreSQL**, with a
  pool of 8 open and 2 idle connections per replica (80 at the production autoscaler's maximum of 10
  replicas), and the pod logs in JSON as the base ConfigMap says. The database credentials are new config
  keys, `database.username` and `database.password` (`SHELLY_DATABASE_USERNAME` and
  `SHELLY_DATABASE_PASSWORD`, which the Secret already carried and nothing read): the app puts them into
  a PostgreSQL DSN that has none, escaped, so a password may hold `@`, `/`, `:`, `%` or `;`. A DSN that
  carries credentials and these keys together is refused, and so is a DSN query other than `sslmode`,
  `connect_timeout`, `application_name`, `search_path`, `sslcert`, `sslkey` and `sslrootcert` (the driver
  reads `user`, `password`, `passfile` and `service` from the query and lets them win). MySQL refuses the
  two keys, and SQLite ignores them. A password never appears in an error, a health status or a printed
  database config (which prints no DSN for a network provider, and a DSN whose path is not one plain
  database name is refused with a fixed message); the keyed patterns of the PostgreSQL error
  sanitizer also missed a value that starts with `s` or holds `=`, now fixed. A test reads the manifests
  and fails on any name the app does not read, a credential outside a Secret, or a URL with a query.
  The provisioner's `SHELLY_API_URL` is no longer set (it named a Service that does not exist under the
  overlays' name prefix, and on port 8080 instead of 80); it stays unset until a per-overlay value is
  added. Not changed: the Dockerfiles and compose files still set the unused `GIN_MODE`.
  (ginsys/shelly-manager#571)
- The database pool now follows the provider. The built-in default of `database.max_open_conns` and
  `max_idle_conns` was 1 for every provider, and the example config set 1 too, so a PostgreSQL or
  MySQL deployment that set only the provider and the DSN ran a pool of one. An unset or 0 value now
  means the provider's pool (SQLite 1; PostgreSQL and MySQL 25 open, 5 idle; the `postgres` and
  `sqlite3` aliases included, where `postgres` used to get 10 and 2), and a value you set is used as
  is on PostgreSQL and MySQL (SQLite always runs with one connection, whatever is set). With only `max_open_conns` set, the idle count never exceeds it. The example config no longer
  sets either key. Visible cost: each PostgreSQL or MySQL replica now opens up to 25 server
  connections, so replicas times `max_open_conns` must stay below the server's `max_connections`; set
  the pool explicitly where that does not hold. The Kubernetes manifests still do not reach the app
  (their `DATABASE_*` names lack the `SHELLY_` prefix): ginsys/shelly-manager#571. Fixes
  ginsys/shelly-manager#570.
- A configured `power_metering.energy_reset` now survives the configuration merge. The merge walked
  into the time field by field, found no field it can set, and left the zero time in the merged
  configuration with no source, so the desired configuration of a device (and what the API returns
  for it) showed `0001-01-01T00:00:00Z` where a template or override had set a time. A struct with
  no exported field is now taken whole, with the later layer winning and its source recorded.
  The desired configuration now carries the real time. Verify is unchanged: no device reports
  `energy_reset`, so verifying a configuration that sets it still reports a difference there, as it
  did with the zero time (ginsys/shelly-manager#567 covers that). Fixes ginsys/shelly-manager#562.
- Apply to one device is now serialized across manager replicas, not only inside one process. On
  PostgreSQL and MySQL the apply holds a lock in the database for its whole span: snapshot,
  device writes, read-back, the pending-reboot record and clear, and the `config_applied` update.
  A second replica's apply waits up to 15 seconds, then is refused with `409` `CONFLICT` and
  writes nothing. A lock lost during an apply fails the apply and leaves the device pending with
  a reboot owed (`config_applied` false, `pending_reboot` true), since the device may hold a mix
  of two applies. SQLite keeps the in-process lock. A connection pooler in transaction mode
  breaks the lock, and an abrupt loss of a replica frees the lock only when the server notices the
  dead client. The lock needs `database.max_open_conns` of at least 2: with one connection
  there is no cross-replica lock, only the in-process one, and the server logs a warning once. The
  PostgreSQL and MySQL default pool is 25 (ginsys/shelly-manager#570), so only a pool set to 1 on
  purpose, or kept from an older example config, loses the lock. Refs ginsys/shelly-manager#425.
- A second apply to a device now waits a bounded time even when the apply before it runs on the
  same server. The in-process lock used to be waited for without a bound, so the documented 15
  second wait and `409` `CONFLICT` held only for a lock held by another replica; on the same server,
  or with a pool of one connection, the second apply queued until the request timed out (`408`) or,
  below the HTTP layer, forever. One 15 second wait now covers the in-process lock and the database
  lock together, and an apply that cannot get either is refused with `409` and writes nothing. A
  request that ends while it waits stops waiting at once. A second apply that arrives while an
  apply runs longer than 15 seconds (one that waits for a reboot, for example) now gets `409`
  where it used to queue. Fixes ginsys/shelly-manager#573.
- A credential save no longer writes old values over another writer's columns. The login
  fallback, the clear of a rejected login, a credential update, the discovery settings backfill
  and a discovery refresh each saved the whole device row they had read earlier, so a save that
  landed after an apply cleared `config_applied`, or after an edit replaced `desired_config`,
  put the old values back. Each now writes only its own columns (the settings text, or the
  address, status, last seen time and firmware) with one UPDATE, and still moves `updated_at`, so a
  device `PUT` that read the older row gets a 409. The login fallback also built its text from
  the model, the generation and the login alone, which dropped every other settings key; it now
  keeps the stored document, built from a fresh read, and leaves a login that was stored
  meanwhile. `pending_reboot` was never affected: a whole-row save cannot write it.
  (ginsys/shelly-manager#565)
- An apply whose database lock was lost while it stalled now writes nothing. Right before its first
  write (to the device or to the stored reboot flag) the apply checks that it still holds the lock;
  if the session is gone it fails with the lost-lock error, writes nothing to the device, forces
  no `pending_reboot` and makes no later flag change, and leaves the result of the replica that
  took over in place. The failed apply does not restore the `config_applied` flag it cleared at
  its start: the device stays pending unless another apply has set the flag since. Cases that
  stay open and are accepted: an apply that stalls after the check, an apply that stalls before it
  and clears `config_applied` after the replica that took over set it, and an unreachable database
  (the forcing flag writes fail too). The cost of each is at most one extra reboot, or one stale
  flag until the next apply or verify. A per-device epoch on the flag writes was weighed and not
  built: a Shelly device checks no fencing token, so the epoch would only reorder the flag writes
  with the same outcome. Fixes ginsys/shelly-manager#564.
- Device control, verify and the handling of stored logins no longer act on whatever device
  answers at the stored address. `POST /devices/{id}/control` (on, off, toggle, reboot) and
  `POST /devices/{id}/config/new/verify` read the MAC the device reports and answer `409`
  `CONFLICT` when it is not the stored MAC, when the device reports none or when the record has
  none; nothing is sent, and verify no longer reports another device as in sync or as drifted.
  The service also reads the identity, without a login, before it clears a stored login that a
  device rejected and before it saves the global login after a device accepted it: another
  device's answer changes no stored login. A stored login that such a device rejects is kept and
  the caller gets the identity error. Status reads send the extra request only on those two
  paths. Fixes ginsys/shelly-manager#556.
- The merged configuration no longer shares memory with the layers it was built from. The merge
  copied a pointer to a value (`*bool`, `*int`, `*string`, `*float64`), a pointer to a list
  (`schedule_rules`) and the elements of a position-merged list by assignment, so a change to the
  merged result reached the stored layer, and a later change to a layer reached a configuration
  that was already merged. Every such value is now a deep copy. The merged values and their
  sources do not change. Fixes ginsys/shelly-manager#547.
- A Gen2 or Gen3 device with a login on could never be reached through the manager: the digest
  client answered with MD5 where the device asks for SHA-256 (`algorithm=SHA-256`), and its
  authenticated retry carried no request body. Both are fixed for every authenticated Gen2 call
  (reads and writes), and `Shelly.SetAuth` now sends `user`, `realm` and `ha1` instead of `user`
  and `pass`, which the device does not take.
- Apply, export and bulk export no longer write to whatever device answers at the stored address.
  Before the first write they read the MAC the device reports and compare it with the stored MAC
  (case and the separators colon, dash and dot are ignored). A different MAC, a device that reports
  none, or a device record with no MAC is refused with 409 `CONFLICT` (in bulk export, as that
  device's own error), nothing is written, and apply changes no flag. The error says what to do: run
  discovery when the address now leads to another device, or set the missing MAC with
  `PUT /api/v1/devices/{id}`. A record with no MAC is refused because the device at its address
  cannot be identified. A failed client build in apply now also leaves `config_applied` as it was.
  Before, a stale address (for example a moved DHCP lease) sent one device's name, relays, MQTT,
  Wi-Fi and login to another device. Other device writes (control actions, credential writes, Wi-Fi
  provisioning) are unchanged. Fixes ginsys/shelly-manager#551.
- The Gen2 and Gen3 apply now names a negative channel id on its own, at the entry's place in
  the desired list, such as `relay.relays.1.id` or `input.inputs.1.id`, with the other 422
  refusals. Before, the refusal named the whole section (`relay`, `input`) and a stray key
  such as `switch:-1`, or, for an entry that sets nothing, did not refuse at all. A negative id
  is now refused on every entry, also on one that sets nothing, because no channel with that id
  can be written. Fixes ginsys/shelly-manager#535.
- The configuration merge now pairs the channels of a template and an override by id, not by
  list position. Before, a sparse override such as `[{"id": 1, "name": "B"}]` over a template
  with channels 0 and 1 was merged into the first list entry and copied its id over it, so
  channel 0's settings could end up under channel 1 (and the Gen2 apply, which writes by id,
  wrote them there). It covers `relay.relays`, `input.inputs`, the effective configuration,
  the inherited view and `PATCH` of the device overrides, where a `null` inside a channel
  removes that field of the channel with the same `id` and no longer of the element at the same
  position. A channel that the lower layer lacks is added in id order, in front of the first
  channel with a higher id, so a list in id order stays in id order, as the Gen1 apply requires.
  A merged channel is a copy that shares nothing with the override. A list written without ids
  (every id decodes as 0) takes the list index as the id, as the Gen1 apply already reads it. A
  list that repeats an id still merges by position, and the Gen2 apply refuses it at the second
  entry. Fixes ginsys/shelly-manager#537.
- A fractional `auto_on`, `auto_off` or `max_power` is now read as no value instead of being cut to
  its whole part, on every read of a device document: the Gen1 and Gen2 device clients, the Gen1
  converter, the typed and raw configuration conversions, the typed import and the normalized
  configuration. Before, a live 7.5 was stored as 7 and verify reported no drift. A number no int
  holds exactly (above 2^53-1, or an infinity) is read as no value too, and a whole number or a
  missing key behaves as before. A fractional `max_power` stays in the raw remainder of the
  normalized configuration, and so does a fractional relay `auto_on` or `auto_off` (as
  `relay_auto_on` and `relay_auto_off`), so a comparison still sees it. One shared helper,
  `shelly.WholeNumber`, now holds the rule that the Gen1 roller read-back already used. Fixes
  ginsys/shelly-manager#532.
- The same rule now holds for the other numeric configuration fields. A fraction, an infinity, a
  number above 2^53-1 or a non-number is read as no value (a field that has a default keeps its
  default) instead of being cut to its whole part, for the MQTT port, keep-alive and update
  period, the CoIoT port and periods, the power-metering maximum voltage and reporting period, the
  dimming, roller, input, LED and colour numbers, the timezone offset, and a device's `gen`
  setting, in the typed import, the Gen1 converter, the raw conversion, the template context and
  the normalized configuration. A fractional timezone offset stays in the raw remainder of the
  typed normalization. Device status values and the clock keep their whole-part reading. The
  `channel` of a device control action (`on`, `off`, `toggle`) must now be a whole number of 0 or
  more: a fraction, a negative number or a non-number is refused with 400 before any device is
  called, where a fraction switched a neighbouring relay and a negative channel crashed the
  toggle. Fixes ginsys/shelly-manager#543.
- Importing a device's typed configuration now keeps, for each Gen1 relay channel,
  `schedule_rules` and `max_power`, and reads `appliance_type` as the appliance type. Before, the
  import dropped the first two and stored `appliance_type` as the relay name when the entry had
  none, so an import followed by an apply lost the schedule rules and the per-channel max power
  and could rename the relay. It covers both the `relays` array and the `relay_0`, `relay_1`
  form. Not confirmed on hardware: the device document is read as the Gen1 reference describes
  it. Fixes ginsys/shelly-manager#528.
- `PUT` and `PATCH /devices/{id}/overrides/new` now check the body against the schema of the
  device model, as the template routes do. A value outside it (an out-of-range number, a text
  that is not an option, a malformed address, a section the model lacks) answers 400
  `VALIDATION_FAILED` with one entry per field in `error.details.fields`, and nothing is stored.
  Before, the server stored any value and only the form checked it. A field of format `ipv4`
  is now checked on the override and template routes and in the form: four decimal parts, an empty value passes. The
  device configuration page lists the refused fields beside the editor. An override that is
  already stored with a bad value still loads, and a `PUT` of it is refused until the value is
  fixed or removed. (ginsys/shelly-manager#522)
- `PUT /devices/{id}` with a body that leaves out `last_seen` or `created_at` no longer
  answers 500 on MySQL. The missing values decoded to a zero time, which MySQL rejects as an
  invalid datetime. A device save now keeps the stored `created_at` and `last_seen` where the
  device carries a zero value. A value the request does carry is still saved.
  (ginsys/shelly-manager#515)
- Importing an archive with a zero time (`0001-01-01T00:00:00Z`) no longer fails on
  MySQL, which rejects a zero datetime. A device with a zero `last_seen` is dated by the
  import time when it is new, and keeps its stored `last_seen` with force overwrite. A
  discovered device with a zero `discovered` time is dated by the import time on both
  paths. A time the archive carries is saved as before (ginsys/shelly-manager#518).
- `GET /devices/{id}/capabilities` now lists `mqtt`, `cloud` and `auth` for a Gen1
  device. It listed them for generation 2 and later only, so a Gen1 device looked as if
  it had no MQTT, cloud or login settings, although the manager already writes them. The
  configuration validator reads the same list, so a Gen1 device with MQTT enabled no longer
  draws the "device may not support MQTT" warning. Bluetooth and Ethernet stay listed for
  generation 2 and later. The API overview describes the list. (ginsys/shelly-manager#507)
- A plain device `PUT` can no longer overwrite changes saved after the client read the
  device. The client sends back the `updated_at` that it read. If the stored `updated_at`
  differs, the server saves nothing and answers `409`. A body with no `updated_at` saves
  as before. The device page and the devices list send the version. Two limits remain.
  MySQL keeps `updated_at` in milliseconds, and whole
  seconds on a legacy column, where two saves inside one second look equal. The status and
  the last seen time are not part of the version: a status read and a control command now
  record them with a narrow write, so a device page that polls the status keeps a version
  that the next `PUT` accepts. A change of the stored login still moves `updated_at`, also
  when a status read causes it once, because it changes the settings that a client edits.
  The only other write on the device row that does not move
  `updated_at` is the temporary address hold of a forced archive restore.
  (ginsys/shelly-manager#502)
- The save of a device `PUT` that carries `updated_at` is now one conditional SQL
  `UPDATE` that matches the `updated_at` that the server read. Before, the server compared
  and saved in two steps, and a writer that does not take the lock of the `PUT` (the
  discovery upsert, the GitOps importer, the archive import, the configuration
  repository) could save between them, so the `PUT` overwrote that change and answered
  `200`. Now the `PUT` saves nothing and answers `409`. Tests cover SQLite, and
  PostgreSQL and MySQL in the provider job of the CI. (ginsys/shelly-manager#509)
- A device save no longer restores an older login. A plain device `PUT`, the refresh
  after a discovery import and the settings backfill of a discovery each saved a whole
  device row that they had read earlier, so a login saved in between was lost. Each
  one now takes `clientMu`, reloads the row and changes only its own fields. A `PUT`
  that leaves the login as the client read it keeps the login that is stored now, and
  a `PUT` that changes the login saves the new one. This closes the gap that the next
  entry names. A client that saves old data it read long before is not covered, and
  ginsys/shelly-manager#502 tracks that. (ginsys/shelly-manager#496)
- A rejected login is now read from the client that the device rejected. The auth
  retry used to take it from the caller's copy of the device record, and a cache hit
  does not refresh that copy. A login saved as A, changed to B and restored to A
  during one request made the retry clear A, although the device had rejected B.
  Each cached client now keeps the saved login it was built with, and the clear
  compares the stored login with that one. Writers that take no `clientMu` can still
  overwrite a saved login, which ginsys/shelly-manager#496 tracks.
  (ginsys/shelly-manager#487)
- A retried Gen1 apply no longer loses a reboot. If an apply wrote a setting that needs a
  reboot (Wi-Fi, login, any MQTT key) and a later write failed, the apply returned before
  the reboot, and the retry read the new value as already stored, reported no reboot and
  could mark the configuration applied while the device still ran the old value. The reboot
  is now recorded on the device (a `pending_reboot` column that only the apply writes)
  before the first write, and stays owed until a reboot succeeds, also across a failed
  reboot. If it cannot be recorded, or the device settings cannot be read first, nothing is
  written. A write the device refused or never
  answered counts too, because it may have landed, so a refused write can cost one reboot
  too many. A device restarted by hand keeps the flag, so the next apply reboots it once
  more. (ginsys/shelly-manager#495)
- The SQL log no longer prints device logins. With `database.log_level` set to
  `info` and `logging.level` set to `debug`, every statement was logged with its
  values filled in, and `devices.settings` holds the clear-text login, so any
  device write printed it. The sqlite, postgresql and mysql loggers now log
  placeholders instead of values. (ginsys/shelly-manager#489)
- One model now owns the `device_tags` table, and one migration creates it.
  Two Go structs with identical columns mapped onto it, and both the database
  migration and the configuration repository created it. The database migration is
  now the only one. The unused template, tag and device-config methods of the
  database `Manager` are removed with their tests; the configuration service
  keeps the live ones. An existing database needs no step: the table keeps its
  columns, indexes and rows, and startup issues no schema statement for it.
  (ginsys/shelly-manager#485)
- One model now owns the `config_templates` table, and one migration creates it.
  Three Go structs used to map onto it and three startup steps took turns
  migrating it, so the table was the union of their columns and a change to one
  struct could rebuild it. The database migration, which runs after the legacy
  repair and stops startup when it fails, is now the only one. The three columns
  that only the wider struct had (`generation`, `variables`, `is_default`) are
  kept, because exports and the template routes read them. An existing database
  needs no step: a table that already has them is left alone, an older table gains
  them as nullable columns, and no row is changed. (ginsys/shelly-manager#280)
- A device status read could put an old login back. It loaded the device, asked it for its
  status, and then wrote the whole record, so a login saved while the device answered was
  lost. This held for the normal read and for the recovery of a device that was marked
  offline. Both writes now go through a reloaded record under the client cache lock.
  (ginsys/shelly-manager#482)
- A drift check no longer writes to the stored configuration or its history. It used
  to run a full import first, which replaced the stored config with the device's,
  added a history row and moved `updated_at`, and then wrote the old config back.
  A check left a history row whenever the imported JSON differed from the stored
  bytes, which its own import timestamp usually caused, and between the two writes
  the stored row held the device's config. It now only reads the device and compares. The one thing it stores
  is the sync status (`synced` or `drift`), because the import status and the stored
  configuration in the API and in exports carry it. It writes that column only when the value changes, and leaves
  the config, the sync time and `updated_at` as they were. (ginsys/shelly-manager#287)
- A device request that the device rejected with 401 could erase a login that was saved
  while the request ran. The three retry paths (reading a device, controlling it, and the
  configuration client) now clear the saved login only while it is still the rejected one. Saving a login now holds the
  client cache lock for its read, write and cache drop. (ginsys/shelly-manager#477)
- A device read that overlapped a credential update could cache a client that still
  used the old login, and later reads reused it. The read now reloads the device
  under the client cache lock before it builds a client. (ginsys/shelly-manager#399)
- The device overrides `PUT`, the template `PUT` and `PUT /devices/{id}/config/typed`
  now refuse a configuration body that repeats a key, in any casing, with 400 and
  save nothing. Before, a secret sent as `null` beside the same key in another
  casing, or inside a repeated section, could be missed, and the old password
  came back. No client of ours sends such a body. (ginsys/shelly-manager#458)
- `PATCH /api/v1/devices/{id}/overrides/new` now removes a field that the body
  sets to JSON `null`, as `PUT` already did for a secret. It answered 200 and
  kept the stored value before, so a field could not be removed. This holds for
  every field and for a whole section. The device then inherits the value of
  its templates again. An omitted key, `""` and the mask work as before.
  (ginsys/shelly-manager#462)
- A forced SMA import no longer fails on MySQL when a stored device address spells the
  temporary address the import parks a device on, under an accent-insensitive collation.
  The temporary address now carries 128 random bits, so a stored or archived address matches
  it under no collation in practice. (ginsys/shelly-manager#413)
- A save that sends a secret of an array entry as the mask, or leaves it out,
  now gets the stored secret back in three cases that dropped it or refused it.
  A `raw` part that is an array at the top is restored like one inside an
  object. Stored entries that share a password and differ in another secret are
  ambiguous only for the secrets the save gets back, so replacing both tokens
  and sending the password as the mask works; entries that disagree at a
  restored secret are still refused with 400. Numbers match by value, so a
  stored `1.0` matches the `1` of a client that writes its JSON again.
  (ginsys/shelly-manager#464)
- The device configuration routes now need the admin key when one is configured:
  every route under `/api/v1/devices/{id}/config`, the device overrides, the
  desired configuration and the device drift report. So do the typed conversion
  and validation routes, the drift schedules and the drift reports under
  `/api/v1/config`. They answered any request before. Without a configured key
  they stay open. (ginsys/shelly-manager#465)
- A forced SMA import that replaces a template now recomputes the devices that use it.
  Before, a device outside the archive kept its old desired configuration and stayed
  marked applied, so the restored template had no effect on it. The recompute runs in the
  import transaction and marks each device pending. A device the archive restored with its
  own desired configuration keeps it, and a recompute failure fails the whole import. (#410)
- The configuration template routes, the device template assignment routes and
  `GET /api/v1/config/schema` now need the admin key when one is configured.
  They answered any request before. Without a configured key they stay open.
  (ginsys/shelly-manager#459)
- A typed save no longer deletes the Wi-Fi key and the login password of a
  configuration stored in the device's own shape. The conversion read
  `wifi_sta.pass` and `login.pass` only, so a device's `wifi_sta.key` and
  `login.password` were missing from the typed view, and the typed save replaced
  the stored configuration without them. It now reads those two spellings as the
  typed Wi-Fi and login passwords, and the save keeps them like any other saved
  secret. (ginsys/shelly-manager#460)
- The typed configuration routes no longer return saved secrets in clear.
  `GET /devices/{id}/config/typed` and `.../typed/normalized` show a secret as
  the mask `********`, in the typed sections and in the `raw` part by key name.
  `PUT /devices/{id}/config/typed` follows the rules of the overrides routes: a
  secret sent as the mask or left out keeps its stored value, an empty string
  or `null` removes it, and the mask is never stored. The answer is masked too,
  and so are the validation messages, which can quote a secret. The same holds
  for a secret inside `raw`. In an array, a masked or omitted secret returns
  only to the entry that matches it at the same position; where more than one
  stored entry matches and their secrets differ, the request is refused with
  400 and asks for the secret value. (#450)
- Saving a template or the device overrides no longer deletes a stored secret
  that the body leaves out. The API never returns a secret, so a client that
  edits the body it read sends it back without the password. A secret that is
  left out, or sent as the mask, now keeps its stored value, and only an empty
  string clears it. JSON `null` removes a secret, so a device override that is
  reset inherits the template secret again; the device editor's Reset sends it.
  Template responses carry `stored_secrets`, the path of each
  stored secret, so the form tells the access point password from the station
  password, and the template form no longer asks for a saved password again
  after an edit. It has a "Clear stored password" action. The rule covers the
  template routes and `PUT` and `PATCH` on the overrides. (ginsys/shelly-manager#395)
- A layer merge no longer drops the plain `network.ethernet` and
  `network.wifi` fields (`enable`, `ssid`, `is_open`, `ipv4mode`, `max_sta` and
  the others). Before, a template value for one of them never reached the
  desired configuration, and a device override of `false` or `""` was lost. The
  fields are now optional (a missing one inherits), so an empty value is a real
  value. This also means an empty string clears the `network.wifi.*.pass`
  passwords, the template form offers a Clear action for them, and `PATCH` on
  the overrides keeps them. A field that is not set is left out of the JSON, so
  `enable: false` is no longer always present; the form reads a missing switch as
  off. (ginsys/shelly-manager#435)
- The device routes no longer return saved secrets in clear. The list, get, add
  and update routes and the configuration reads (`/config`, `/config/import`,
  `/config/current`) show a secret held in `settings`, `overrides`,
  `desired_config` or the device's own settings as the mask `********`. A save
  that sends the mask back, including `PUT /devices/{id}/config`, keeps the
  stored secret and never stores the mask.
  The JSON, YAML and GitOps exports are not covered. (#436)
- The JSON, YAML and GitOps exports, and the preview of an export, no longer
  write saved device secrets in clear. A secret in the device settings, the
  device configurations, the templates or the template variables is replaced by
  `[REDACTED]`, the marker the SMA export uses, also when the secret is a list
  or an object. The SMA import keeps the stored value for it; the JSON, YAML and
  GitOps plugins have no import. Set `exclude_sensitive` to `false` in the
  export `config` to ask for a full export. The `backup` plugin still writes a
  full database snapshot. (#443)

### Added
- Apply to a Gen1 device now writes the time server and the MQTT update period.
  It sends `sntp_server` and `mqtt_update_period` as flat parameters on
  `POST /settings` and checks both on the read-back. A time server that is not an
  IPv4 address or a valid host name, and a negative update period (`0` is kept as
  "off"), are refused with 422 before anything is written. Wi-Fi and inputs stay
  refused. A change to any MQTT setting apply writes now reports that a reboot is
  required (it did not before for any of them). The
  parameter names, the accepted period range and that reboot were read from a
  summary of the Shelly reference and are not confirmed on hardware.
  (Refs ginsys/shelly-manager#381)
- Apply to a Gen1 device now writes the login (`POST /settings/login`: `enabled`,
  `username`, `password`, 1 to 50 bytes each, both together). It is written
  last, after every other setting. Once the device accepts it, the manager stores
  the new credentials (the same way as `PUT /devices/{id}/config/auth`), builds a
  new client from them, and reboots and reads back with that client, because the
  device answers the old credentials with 401. A rejected write stores nothing.
  The password is never logged, returned or compared (the device does not return
  it). A credential with the login off or unset, and a username or password of the
  wrong length, are refused with 422 before anything is written. Two limits: a
  response lost after the device accepted the write leaves the stored login stale,
  and entering the login again recovers it; and the Gen1 login reference was read
  through a summary only, so the field names and whether a reboot is needed are
  not confirmed on hardware. (Refs ginsys/shelly-manager#381)
- Apply to a Gen1 device now writes the CoIoT settings. It sends `coiot_enable`,
  `coiot_update_period` and `coiot_peer` as flat parameters on `POST /settings`
  (Gen1 has no `/settings/coiot` endpoint, and the nested document that
  the old code sent was ignored) and checks them on the read-back. An empty peer is
  written as `mcast`. An update period outside 15 to 65535 seconds, and the literal
  peer `mcast`, are refused with 422 before anything is written. Wi-Fi, login and
  inputs stay refused. Whether a CoIoT change needs a reboot is not documented
  by Shelly and is not confirmed on hardware. (Refs ginsys/shelly-manager#381)
- Apply to a Gen1 device now writes the cloud setting. It sends the `enabled` flag with
  `POST /settings/cloud` and checks it on the read-back, so a device that answers 200 and
  keeps the old flag stays pending. A cloud `server` in the desired configuration is
  refused with 422, because Gen1 reports no cloud server and accepts none. Wi-Fi, login
  and inputs stay refused. (Refs ginsys/shelly-manager#381)
- An SMA archive now carries each template's scope. A template row has an optional
  `scope` key (`global`, `group` or `device_type`), the export always writes it, and an
  import creates the template with it, so a `group` template no longer comes back as
  `global` or `device_type`. An archive without `scope` keeps the old derivation from the
  device type, and an invalid scope is refused by validation and by apply. A replacement
  with `force_overwrite` still keeps the stored scope. The format version stays `2026.1`;
  a reader with the older closed key list refuses an archive that has `scope`. (#398)
- A device-type template now takes its type from a list. `GET /api/v1/config/device-types`
  returns the supported models (code, name, generation), and the create dialog offers
  them in a select instead of a free-text field. The template create and update routes
  refuse a device type outside the list with a `device_type` field error. A template
  stored with another type stays editable, and the startup migration and import still
  accept it. (#439)
- SMA import now restores devices (with their desired configuration),
  templates and discovered devices. The import runs in one transaction, so a
  failure writes nothing. Existing records are skipped, or replaced with
  `force_overwrite`, and `validate_only` writes nothing. Network settings,
  plugin configurations and system settings in the archive are not restored,
  and `backup_before` is ignored; both are reported as warnings. An export
  followed by an import now restores the data. (#284)
- `GET /api/v1/config/schema?device_type=<model>` (or `?device_id=<id>`) returns
  the schema of the configuration stored for that model. It holds only the
  model's sections and only fields the stored configuration keeps, and marks
  each field `x-applicable` when apply can write it today. A test fails when
  the marks and apply disagree. The MQTT password is applicable but write-only:
  a Gen1 device never returns it, so verify cannot read it back, and a test
  fails if a second applicable field is skipped by verify without being named.
  The CoIoT section uses the stored keys
  (`enable`, `peer`, `update_period`). The stored network section is always
  present, and SHSW-25 has the roller section, both not applicable. An empty
  or malformed parameter is a 400. Without a parameter the generic schema is
  unchanged. (#384)
- The device override editor is now a real form. It loads the schema of the
  device's model, shows nested objects as nested fields, and shows each field's
  inherited value and its source. Editing a field makes it an override, Reset
  removes it, and "Use default" sets the schema default. A field that apply
  cannot write yet is read-only. Schema validation (type, range, length,
  pattern, enum, required, including the keys auth and Wi-Fi require while
  enabled) runs on every change, and Save stays disabled while a field is
  invalid or the schema is still loading. Only changed fields go into the
  override body: a value set back to the inherited one removes the override.
  The inherited value is the one the templates give, also where an override
  already sits on the field: `GET /devices/{id}/desired-config` and the
  set-templates response now carry `inherited` and `inherited_sources` (the
  template layers alone), and a template secret that is set shows as a mask,
  never as its value. The device overrides routes (`GET`, `PUT` and `PATCH`
  `/devices/{id}/overrides/new`) show a saved secret as the same mask, where
  they showed nothing. A write that sends the mask back keeps the stored
  secret, and the mask is never stored, so an unrelated edit no longer drops a
  saved password and the form does not ask for it again. One list of secret
  fields serves removal, masking and restoring. It covers the Wi-Fi access
  point key and the station and access point passwords of the network part,
  and the set-templates response no longer returns a template secret in its
  desired configuration. A section whose last saved override is reset stays
  checked for required fields, so Save cannot remove a saved password while a
  template keeps authentication on. Lists still use a JSON box. (#385)
- The template editors (the create dialog and the template page) use the same
  form. A device_type template gets the schema of its model. A global or group
  template gets the new `GET /api/v1/config/schema?scope=global|group`, which
  holds every stored section. A new template starts empty, and each field
  offers its default. The server now checks a template body against that
  schema on create and update, and refuses a wrong type, an out-of-range value,
  a value outside its enum, or a section the model does not have. It also
  refuses an unknown key, a key in the wrong case (`Relay` for `relay`) and a
  repeated key, and names the key, because the stored body is decoded without
  regard to case and merges repeats, so a schema check could read a different
  body than the one stored. The refusal is a `VALIDATION_FAILED` error with
  each field in `error.details.fields`. The JSON view stays. Scope and device
  type are read-only after create, because the update API keeps them. A
  template with a saved password can be renamed: the unchanged config is left
  out of the update, and the saved password counts as given while the config is
  untouched. Once the config changes, the password is required again, because
  the update replaces the config. A late schema response no longer replaces the
  schema of the current scope or model. (#386)

### Changed
- Go toolchain is 1.27.4. It fixes GO-2026-6859 in `net/http` and keeps the
  crypto/x509 intermediate-certificate mitigation (CVE-2026-111943) from 1.27.3.
  golangci-lint is 2.15.0. Module updates include
  prometheus/client_golang v1.25.0 and current golang.org/x releases.
  Dependabot again proposes golang.org/x minor updates. (#429)
- Export and import previews now use the registered plugin list and each
  plugin's backend schema. Export preview supports every registered format;
  browser import preview is deliberately limited to SMA data and enforces a
  7 MiB pre-base64 upload limit. Schema forms preserve typed falsy values and
  validate recursive array/object fields. (#268)
- Test mode (`SHELLY_SECURITY_VALIDATION_TEST_MODE`) registers the production
  `/api/v1` route table. The security middleware stack stays off. `GET
  /api/v1/metrics/security` remains production-only because it closes over the
  security monitor, which test mode does not construct. (#252)
- SMA is now a closed, strict `2026.1` format. Generated archives are
  deterministic gzip files with RFC 8785 SHA-256 integrity, a 100 MiB
  normalized-data limit, strict JSON/schema/number/depth validation, joined
  persisted device settings/configurations and templates, and atomic
  publication. Raw JSON is accepted only as an import representation.
  A non-preview import restores devices, templates and discovered devices in
  one transaction (#284). (#268)
- Export/import audit identity uses trimmed `X-User-ID`, then `X-User`, and
  otherwise `api`; credentials and network addresses are never recorded as the
  requester. (#268)
- Frontend type checking now has a zero-error baseline and raw `vue-tsc`
  succeeds. (#268)

### Removed
- Removed the inert sync export scheduling API, UI, capability flag, and
  documentation. Drift, notification, and device/relay scheduling are
  unchanged. Former export-schedule paths now return ordinary HTTP 404
  responses. (#268)
- Plugin configuration, connection testing and enable/disable are removed from
  the UI and API client (#264). The frontend called `GET/PUT
  /export/plugins/{name}/config` and `POST /export/plugins/{name}/test` plus an
  enable/disable toggle built on `/config` — none of those routes exist on the
  backend, and there is no server-side model for stored plugin configuration
  (config is supplied per export/import request). The dead API functions, store
  actions, the card's Configure/Test/Enable-Disable controls and
  `PluginConfigForm.vue` were deleted (no HTTP 501 stubs — the routes never
  existed). Plugins are now read-only: list, per-category grouping, a read-only
  **schema viewer**, and read-only details. A product model for stored plugin
  configuration/testing/enablement is tracked as a design vertical (#290).

### Fixed
- The template pages no longer start an action while a save or a refresh is
  pending. A list request that ended brought Edit back while the save was still
  running, so Edit could open from the old row and its Save could undo the
  saved change. Edit, Delete, Create, Save, the scope filter and (on the detail
  page) Copy now stay off until every request has finished, and a failed
  refresh on the detail page has a Retry button. (#441)
- The device configuration editor no longer lets Save delete a stored password.
  Typing `null`, a number, `false` or `""` as the whole body in the JSON view
  cleared the editor's memory of what was saved, so a following `{}` passed the
  required-field check. The check now reads the overrides as the page loaded
  them, and no edit changes that. (#437)
- The device detail page always showed "No Drift", and the configuration page
  had no way to see what the device holds. The UI read `hasDrift` and
  `driftFields`, which the drift endpoint never sends; it now maps the
  endpoint's `differences` list (and the `drift: false` no-configuration
  answer) to the badge. `GET /devices/{id}/config/current` now answers in the
  standard `{success, data}` envelope the UI client reads, instead of a bare
  `configuration` member, and the configuration page has a read-only "Live on
  device" panel that reads the device on demand. The live reads
  (`/config/current` and `/config/current/normalized`) no longer store what
  they read: before, one read replaced the stored configuration and marked it
  synced, so the next drift check showed no drift. `POST
  /devices/{id}/config/import` still accepts the device's state as the stored
  configuration. (#388)
- The device client cache is keyed by IP address and kept the entry at a
  device's old address after the device moved. The cache now records which
  device each client was built for, drops the entry at a device's previous
  address on its next request, and never hands a client to a different device
  that takes the address. (#396)
- Gen1 apply refused every relay setting, so power-on state and auto on/off
  could not be managed. Apply now writes `default_state`, `auto_on`, `auto_off`
  and the per-channel `name` with `POST /settings/relay/<n>` as flat form
  parameters, then reads each channel back with `GET /settings/relay/<n>`. A
  value the device did not keep is reported as a failure of that path, for
  example `relays[0].auto_off`. Other relay keys, such as `schedule`, are
  still refused with HTTP 422, and the device schema marks exactly the four
  written fields applicable. Each relay `id` must equal its position in the
  list, because the position is the channel written; any other `id` is refused
  with HTTP 422 before the device is touched. Wi-Fi is still never written. (#387)
- Device credentials saved with `PUT /devices/{id}/config/auth` were stored
  where no client builder reads them, and a cached client kept the old login
  until restart. They are now stored as `auth_enabled`/`auth_user`/`auth_pass`
  (an empty user disables auth), and the cached client is evicted on change.
  Credentials saved earlier in the old `auth.username`/`auth.password` shape
  are still used until they are saved again. A save also keeps that old pair
  in step, so a forced SMA restore of an archive exported in the old shape
  keeps the login. A device that rejects the saved
  login no longer hangs the request while the client is rebuilt. A device that
  rejects the stored login now returns 502 `EXTERNAL_SERVICE_ERROR` from
  `GET /devices/{id}/config/current` (was a generic 500) and from
  `/current/normalized` (was 200 with `success: false`). A username with an
  empty password is now refused with 400 `VALIDATION_FAILED`. (#376)
- A device that holds its login only in the old `auth.username`/`auth.password`
  pair, and rejects it, is now reached with the provisioning login on live
  configuration reads (was 502 on every read), and the rejected pair is
  cleared. (#415)
- An SMA import now also writes a restored device's configuration to the
  `device_configs` table, with sync status `pending`. Before, it wrote only
  `devices.desired_config`, so after a restore the configuration endpoint
  answered 500, the import status read `not_imported` and drift detection was
  disabled. A forced import updates the existing row in place and keeps its
  template. (#411)
- `POST /devices/{id}/config/new/apply` and `/verify` were stubs that always
  reported success. Apply now sends the desired configuration to the device,
  reads it back and compares. It reports real `applied_count`, `failed_count`
  and `requires_reboot`, lists failures and read-back differences, and sets
  `config_applied` only when the read-back matches. Verify reports the real
  differences and changes nothing. Secret values (passwords, keys, tokens)
  are masked in the differences. Only Gen1 devices are supported; Gen2 and
  Gen3 return HTTP 501 rather than a false success. On Gen1, apply writes the
  name, timezone, location, eco mode, discoverable, max power, LED, MQTT,
  relay and cloud settings. A desired configuration that holds any other setting
  (Wi-Fi, CoIoT, login, inputs, a cloud server, or a relay key other than the four
  written ones) returns HTTP 422 naming the settings, and nothing is written. Apply clears `config_applied` before writing, and a
  desired configuration edited during the apply leaves the device pending.
  Settings the converter has no Gen1 mapping for (for example SNTP) are also
  refused, and so is a stored desired configuration with fields the
  configuration model does not know, such as a legacy row from an SMA import.
  Applies to one device run one at a time, apply needs the admin key when
  one is configured and is open without one,
  the write-only MQTT password is not compared on read-back, an MQTT server
  given as `host:port` matches the device's split read-back, and a custom
  timezone or location also switches off automatic detection. The device model
  is read from the stored device settings. A device that rejects the login
  during the write returns HTTP 502, and a refused write names the setting but
  not the device's error text, which can echo the value sent. Applies are
  serialized inside one server process only, so run one manager replica when
  you use apply (#425). The routes are documented in
  `docs/api/api-overview.md`. (#375)
- Drift detection no longer reports a Gen1 plug's live `meters` reading (power,
  counters, timestamp) as configuration drift, so a freshly imported device is
  no longer drifted. Only the top-level `meters` key is ignored; `emeters` is
  configuration on energy-meter models and is still compared. The device
  clock (top-level `time` and `unixtime`) is ignored the same way. (#377)
- The web UI showed "The API has changed since this UI was built" after every
  server start, because it compared the UI build time with the server start
  time. It now compares the git revision each side was built from: `GET
  /api/v1/version` returns `git_sha`, and the banner shows only when both
  revisions are known and differ. A server built without version control
  information (`go run`, `-buildvcs=false`) or a dev UI shows no banner. (#438)

### Changed
- Plugin list/detail responses are now modelled as exact DTOs (#266): `Plugin`
  (list item) and `PluginDetail = { info, capabilities }`, replacing a permissive
  superset. Plugin status is backend-hardcoded, so a listed plugin is presented
  as "Registered" only; the fictional status filter/sort, configured/disabled/
  error statistics and `health` rendering were removed.
- Drift detection schedules now fail closed. The scheduler
  (`configuration.NewScheduler`) has no callers — schedules were stored but never
  executed, so `next_run` was never set, `run_count` never advanced and the run
  table was empty by construction. The API and UI previously presented these
  inert rows as operational jobs. `POST /config/drift-schedules`,
  `PUT /config/drift-schedules/{id}`, `POST /config/drift-schedules/{id}/toggle`
  and `GET /config/drift-schedules/{id}/runs` now return **HTTP 501
  Not Implemented** in the standard error envelope, short-circuiting before any
  input parsing or database access (no side effects). List, detail and delete
  keep working so stored schedules stay inspectable and removable. The four
  bypass service methods that wrote GORM directly were removed so no inert
  write/history path remains. The UI shows a "not executed in this release"
  notice, renders schedules as stored/inactive with the `device_filter` as
  read-only JSON, and offers deletion only. Scheduled execution is tracked
  separately as #279. (#270)

### Fixed
- A forced SMA import (`force_overwrite`) now restores devices that swapped IP
  addresses since the export. Before, the first update broke the unique IP index
  and the whole import failed. The import frees the address of every device it
  overwrites inside the transaction, then writes the archived addresses; an
  address that a device outside the archive holds still fails the import. (#397)
- SMA `validate_only` and `dry_run` imports now refuse an archive with a device
  or discovered device without a MAC address, or a template without a name.
  Before, they reported the archive valid and the apply refused it. One shared
  check serves validation, preview and the apply. (#406)
- The auto-rebase workflow skips draft pull requests. It used to force-push every
  open draft onto `main` after each merge, which moved a draft to a new head while it
  was being reviewed and verified and cancelled the test run in progress. A pull
  request is rebased again once it is marked ready for review. (#367)
- The server now also answers `GET /api/v1/version`. The UI calls that path, so
  the "UI is older than the server" banner and the database labels on the
  backup page never got their data. The root `GET /version` stays. (#363)
- The device configuration page follows the device in the address bar. After a
  route change from one device to another (for example with the browser back
  button) it kept the first device id, so apply, verify, save and template
  changes went to the wrong device. The page now reloads, closes the editor, and
  ignores a late answer for the device it left. (#400)
- `GET /provisioner/discovered-devices` started a cleanup goroutine that
  nothing waited for, so `TestDiscoveredDevicesDatabaseIntegration` failed at
  random under `go test -race` when the test closed the database while the
  cleanup still read it. The handler now tracks that work, and
  `Handler.WaitBackground` waits for it before the database is closed. (#366)
- The device configuration page drops work in flight when it is left. A verify,
  apply, save or template answer for one device could still reach the page of
  the next device, for example after the device page in between. A new page
  starts from an empty store, and a page that was left stops its own handlers.
  (#402)
- mDNS discovery read each service entry while the mDNS client was still
  writing its fields, a data race under `go test -race`. `DiscoverDevices` now
  collects the entries and reads them after the query returns. `POST
  /api/v1/discover` also tracks its scan goroutine, so `Handler.WaitBackground`
  waits for it. The API tests no longer scan a real network. (#412)
- `database.path` (and `SHELLY_DATABASE_PATH`) is honoured again: a built-in
  `database.dsn` default always won, so unless `dsn` was set explicitly every
  SQLite run used `data/shelly.db` whatever `path` said. `dsn` still takes
  precedence when set; the shipped `configs/shelly-manager.yaml` no longer sets
  it. Startup logs now name the database actually opened, with
  network-provider DSNs redacted, instead of the configured `path`.
  **Upgrade note:** an explicitly set `dsn` (in the config file or
  `SHELLY_DATABASE_DSN`) keeps working unchanged, including a config file
  copied from the old shipped one, which sets `dsn: "data/shelly.db"`.
  Otherwise, if `path` names any other file, the data written so far is in
  `data/shelly.db` (relative to the working directory); move it to `path`, or
  point `path` at it, before upgrading. This includes `configs/development.yaml`
  and the docker-compose `SHELLY_DATABASE_PATH` setting. Before removing an
  explicit `dsn` to rely on `path`, move the file it names to `path`. (#276)
- `GET /config/drift-schedules/{id}` returned HTTP 500 instead of 404 for a
  missing schedule: the service wraps `gorm.ErrRecordNotFound` with `%w`, but the
  handler compared with `==`; it now uses `errors.Is`. (#270)

- Configuration drift detection no longer reports in-sync devices as drifted.
  `ImportFromDevice` re-stamps a volatile `_metadata.imported_at` (plus a
  `device_info` subtree) on every import, and `DetectDrift` re-imports the device
  before diffing — so any check whose timestamp differed from the stored baseline
  registered a spurious difference and set `RequiresAction`, marking a
  perfectly-synced device as drifted (and intermittently failing the drift tests
  in CI). Drift comparison now ignores the `_metadata`/`device_info` bookkeeping
  subtrees via an exact raw-key match applied during recursion, so genuine config
  drift is still detected while our own import metadata is not. Audit-history
  change tracking (`createHistory`) is unchanged and still records metadata
  changes.
- Real-time metrics over WebSocket never actually worked and reported a false
  "LIVE": the backend emitted `initial_metrics`/`metrics_update`/`alert`/
  `device_status_change`/`drift_detected` while the frontend only accepted a
  disjoint set (`status`/`health`/`system`/…), so every live frame was discarded
  yet the badge turned green on connect. The message types are now named Go
  constants mirrored by a frontend manifest and enforced across the boundary by a
  contract test; the store validates every frame at runtime, hydrates from both
  snapshot types, and drives a truthful "LIVE" from an explicit freshness state
  machine (idle→polling→live→stale) with a reactive watchdog — connection state
  alone never marks the feed live. REST polling stays active until the first
  snapshot is applied and can't be clobbered by a late response; the stale socket
  is recycled on heartbeat timeout and late frames from a superseded socket are
  ignored; `alert`/`device_status_change`/`drift_detected` feed a live-events
  panel. Charts drop the fictional CPU/memory/disk series (no backend telemetry)
  for a real device-count trend. (#247)

### Security
- Require the configured admin key on all four bulk configuration endpoints
  (`POST /api/v1/config/bulk-import`, `/bulk-export`, `/bulk-drift-detect`,
  `/bulk-drift-detect-enhanced`). These operate across every device and
  `bulk-export` pushes stored config to physical hardware, so they were an
  unauthenticated fleet-wide mutation surface. Also documents the previously
  missing `bulk-drift-detect-enhanced` route in the OpenAPI spec. (#245)
- Require the configured admin key on the metrics control endpoints
  (`POST /metrics/enable`, `/disable`, `/collect`, `/test-alert`) and the data
  reads (`GET /metrics/status`, `/dashboard`, `/security`). Previously the
  mutating endpoints were unauthenticated while the read summaries required the
  key — an inverted policy. `/metrics/prometheus` stays public by convention.
  (#246)

### Fixed
- SMA non-dry-run import no longer fakes success. `performImport` and its
  per-entity helpers were placeholders that returned `success: true` with a
  positive `records_imported` count while writing nothing, so a restore reported
  success but silently lost data. Persistence was not built yet (a larger
  follow-up), so a real import failed closed: the generic import endpoint
  returned `501 Not Implemented` (`NOT_IMPLEMENTED`) instead of a fabricated
  success or an opaque `500`. Dry-run preview was unchanged. The actual
  persistence replaced that 501 in #284. (#272)
- Existing databases could not start the server after upgrading: AutoMigrate
  emitted `ALTER TABLE config_templates ADD scope text NOT NULL`, which SQLite
  refuses on a populated table and PostgreSQL rejects for the NULLs it would
  create. Startup now repairs a legacy schema before AutoMigrate runs — a
  read-only preflight that classifies every row, then a transactional backfill,
  then per-column `NOT NULL` tightening (only where a column is still nullable).
  A missing scope is derived from the legacy `device_type`, preserving or
  narrowing applicability but never widening it: exactly `all` becomes `global`,
  any other concrete type becomes `device_type`, and an empty or NULL device type
  aborts startup with **every** offending template listed and a remedy. A NULL
  `config` aborts the same way rather than being invented as `{}`. The repair is
  idempotent and resumes safely from a partially completed run. A permanent
  `default:'global'` column default was rejected as the fix: it would have
  silently accepted scope-less inserts forever. See
  `docs/guides/database-upgrade.md`. (#275)
- Closed the write path that recreates those rows: `POST`/`PUT
  /api/v1/config/templates` validated nothing and wrote straight to the
  database, so a client could store an empty or nonsense scope that the next
  startup would refuse to migrate. Both paths now share one validator with the
  newer repository path, and a violation returns a standard-envelope **400**
  instead of the blanket 500 the handlers previously produced. (#275)
- The PostgreSQL and MySQL providers could never connect: `Connect()` called
  `Ping()` before setting `connected`, and `Ping()` refuses while `connected` is
  false, so every attempt failed with `failed to ping database: not connected to
  database`. Only SQLite was reachable in practice. (#275)
- MySQL could not migrate the schema at all, which the new provider CI job
  surfaced. `TEXT` columns cannot carry a `DEFAULT` (`Device.TemplateIDs`,
  `Overrides` and `DesiredConfig` now seed their empty documents in a
  `BeforeSave` hook instead), and indexed string columns need a bounded length,
  so all 24 of them are now `size:191` — the largest utf8mb4 prefix that fits
  MySQL's 767-byte index limit. (#275)
- Repaired the SMA import round-trip, which was broken end to end. Decompression
  called the non-existent `pako.gunzip` with the obsolete `{ to: 'string' }`
  option; and checksum validation always failed because the generator hashed the
  pre-checksum JSON while the parser hashed the post-checksum JSON. Switched
  `sma-parser`/`sma-generator` to pako 3 named imports with `{ toText: true }`,
  made both sides hash the same canonical form, fixed the uncompressed-size
  measurement to count UTF-8 bytes, removed the stale `@types/pako`, and added an
  unmocked generate→parse round-trip test (real pako + real sha256). (#260)
- Disabled the backup Restore action in the UI until its backend endpoints
  exist (#249). The previous handlers recursed into themselves and targeted
  missing `/import/restore*` routes; `BackupList` now fails closed via a
  `restoreEnabled` prop. (#260)

### Added
- CI job `database-providers`, running the legacy-schema upgrade matrix against
  health-checked `postgres:16` and `mysql:8` service containers. The startup
  repair runs on every provider, so it is no longer verified on SQLite alone.
  The job pins the one test by name rather than exporting the provider env vars
  package-wide, which would also enable unrelated env-gated suites. (#275)

### Changed
- Enforce the frontend `vue-tsc` baseline in CI as a strict per-file ratchet
  (#254). The 117 → 30 burn-down was previously unprotected — `typecheck` ran in
  neither `make test-ci` nor any workflow, so the count could creep back
  silently. `ui/typecheck-baseline.json` now pins per-file counts; the gate fails
  if any count rises, if a new file reports errors, **and** if counts drop until
  the baseline is regenerated, so every improvement is locked in by the same PR
  that makes it. The updater is monotonic (it refuses to raise the baseline, so a
  regression cannot be blessed by re-running it); renames go through a narrow
  `--move old=new` that requires the old file to be gone and the new one present.
  Parsing is fail-closed — unrecognised compiler output, a crashed or
  signal-killed compiler, unexpected stderr, or a mismatched exit status all fail
  rather than being read as "no errors", and never rewrite the baseline. Covered
  by 74 tests. Note raw `npm run typecheck` remains red (30 known errors, tracked
  by #260); `npm run typecheck:baseline` is the gate. (#254)
- Cleaned up the frontend type-check backlog (#260): removed dead code
  (unused imports, the never-mounted `SMAImportForm` component, and orphaned
  helpers in `BackupManagementPage`) and applied minimal, contract-preserving
  type fixes (DOM event casts, null-guards, value coercion). Behavior is
  unchanged; `vue-tsc` errors dropped from 67 to 38. (#260)
- Charts now always load the installed `echarts` package. `BarChart`/`LineChart`
  had a development-only branch importing a pinned echarts 5.5.0 from
  `cdn.jsdelivr.net` while the repo ships echarts 6.1.0 — a dev/prod version skew
  and an external network dependency. The paired `rollupOptions.external` dev
  branch was removed too. Chart instance refs are now typed
  (`shallowRef<ECharts | null>`), and the previously missing lifecycle was added:
  resize on window resize, dispose plus listener removal on unmount. `vue-tsc`
  errors dropped from 38 to 30. (#260)

### Added
- The devices list now has a Refresh button in the toolbar and an `R` shortcut that reload the list. The button shows `Refreshing...` and stays off while a load runs, and the shortcut does nothing while you type in a field, hold a modifier key or have a dialog open. A device selection is kept across a refresh. An older list answer that arrives after a newer one is now dropped, so a page change during a refresh no longer shows stale rows. Fixes ginsys/shelly-manager#80.
- The devices page now has a select column and a Bulk Operations button that opens the bulk
  dialog (import, drift detection, drift report) for the ticked devices. The button stays off
  while nothing is ticked, the select-all box covers the rows on the page, and the choice is kept
  when you move to another page. The dialog lists the picked devices, lets you drop one, sends
  exactly those and shows a failed request in the dialog instead of a browser alert. Fixes
  ginsys/shelly-manager#136.
- Stored drift schedules can now run: set `drift.scheduler_enabled` (`SHELLY_DRIFT_SCHEDULER_ENABLED=true`; off by default) and the server polls the schedule rows every 30 seconds, runs each due schedule on its five-field `cron_spec` once even with several replicas, only reads devices, stops a run from starting more devices after 10 minutes, and keeps the newest 100 runs per schedule. With the scheduler on, a stop on SIGINT or SIGTERM takes at most 25 seconds (a 15 second scheduler wait and a 10 second server shutdown), inside Kubernetes' default 30 second grace period. The create, update, toggle and run-history routes still answer 501. Refs ginsys/shelly-manager#279.
- The Drift schedules page can now create and edit a schedule and enable or disable it. The form takes a name, a description, a cron preset (hourly, daily at 03:00, weekly on Monday at 03:00) or a custom spec, and one of three device choices: all devices, chosen devices, or a type and status filter. It sends at most one of the device list and the filter (neither for all devices), and keeps Save off until a chosen list or a filter has something in it, so an empty choice never means all devices by accident. Only a new schedule has an Enabled box; an edit sends no `enabled`, so a save never undoes the card's Enable or Disable. The server stays the validator and its refusal is shown as written. Refs ginsys/shelly-manager#279.
- `GET /api/v1/config/drift-schedules/{id}/runs` now returns the runs of a schedule instead of 501: newest first, `limit` 1 to 100 (default 20, 400 on any other value), 404 for an unknown schedule, `[]` before the first run, and no `results` document in the list. `GET /api/v1/config/drift-scheduler` returns `{"enabled": bool}`, the `drift.scheduler_enabled` of the server that answers (admin key). Refs ginsys/shelly-manager#279.
- Each schedule on the Drift schedules page has a "Recent runs" list, read when the card is first opened: started time, status (running, completed, failed, interrupted or skipped), duration and the error of a failed run, newest first, with an empty state and a failed-load state that can be retried. Refs ginsys/shelly-manager#279.
- Drift schedules can now be created, updated and enabled or disabled through the API (`POST`, `PUT` and `POST .../toggle` on `/api/v1/config/drift-schedules`). A body that names a bad five-field `cron_spec`, an unknown device, an unknown filter key, or both a device list and a filter is a 400 and writes nothing. Deleting a schedule now answers 404 for a missing one and removes its device list and runs, and list and get return each schedule's `device_ids`. The run-history route still answers 501. Refs ginsys/shelly-manager#279.
- The server now logs a `WARN` at startup when test mode is on. With
  `SHELLY_SECURITY_VALIDATION_TEST_MODE=true` it names the protections the test-mode router leaves out
  (rate limiting, request validation and the rest); with `security.validation_test_mode`
  alone it names the two request checks that are skipped. Nothing it turns off changes. Fixes
  ginsys/shelly-manager#368.
- A desired configuration can now state the Gen1 actions of a device (the HTTP calls a button or an output makes), as `actions`: a list of entries with `name`, `index`, `enabled` and `secret_urls`. Apply writes each entry with `POST /settings/actions`, reads the actions back and compares them, and verify and drift report a difference without showing the URLs. A value the device cannot keep (more than 5 URLs, a URL that is not `http://` or is longer than 256 bytes, an action name the model does not have, or any action on a Shelly i3 or a model the manager does not know) is refused with 422 before the device is touched. The URLs can hold a password or a token, so every route that returns a configuration masks them, and a masked value sent back restores the stored URLs only where the entry still matches. Not yet confirmed on hardware: the Plug and Plug S action names, what a device answers for a name it does not know, and which URL schemes it accepts. Importing a configuration from a device does not read its actions. Refs ginsys/shelly-manager#381.
- The Shelly Bulb (`SHBLB-1`) is now a supported Gen1 model. Applying a desired `light` section writes its one channel to `POST /settings/light/0` with only the keys set (`default_state`, `auto_on`, `auto_off`, `effect`, `schedule`, `schedule_rules`) and reads it back, and a desired mode (`system.device.mode`, `white` or `color`) is written first and read back. An `effect` outside 0 to 6, a second light, a key the Bulb's page does not list (`name`, `transition`, `btn_type`, `btn_reverse`, `btn_debounce`, `swap_inputs`), an unknown mode, a `relay` section and a `dimming` section are refused before anything is written. The Duo and the Vintage stay unsupported, and the reset parameter is never sent. The key names, words and ranges, whether `effect` also applies in white mode, and the form verb are not confirmed on hardware. Refs ginsys/shelly-manager#381.
- The Shelly Dimmer (`SHDM-1`, `SHDM-2`) is a supported Gen1 model, with its channel settings in a new `light` section: `default_state`, `auto_on`, `auto_off`, `schedule`, `schedule_rules`, `btn_type`, `btn_debounce` (60 to 200 ms) and `swap_inputs` are written to `POST /settings/light/<n>` and read back, and a value outside those lists or ranges is refused before anything is written. A Dimmer has no relay, so a desired `relay` section on it is now refused before any write instead of being sent to a path the device does not have; the relay and roller results of the other six models do not change. The device-level Dimmer settings (brightness limits, night mode, warm-up) and the typed read of the light channels are not covered yet, and the key names and ranges are not confirmed on hardware. Refs ginsys/shelly-manager#381.
- A Shelly Dimmer (`SHDM-1`, `SHDM-2`) now takes its night mode (`enabled`, start and end as `hh:mm`, brightness 1 to 100) and its warm-up (`enabled`, brightness 10 to 100, time 20 to 200 ms) from a template: each is written to its own endpoint and read back, and a value the device did not keep fails at its path. A time that is not `hh:mm` or a value outside those ranges is refused before anything is written; with warm-up off only its `enabled` flag is written, so a config imported from a Dimmer that was never set still applies. The key names, the `enabled` type, whether a reboot is owed and the warm-up maximum (200 or 1000 ms) are not confirmed on hardware, and calibration, `max_brightness`, `default_brightness` and `min_dim_level` are still not applied. Refs ginsys/shelly-manager#381.
- The device-level settings of a Shelly Dimmer (`SHDM-1`, `SHDM-2`) are written and read back: `dimming.min_brightness` (0 to 50), `fade_rate` (1 to 5), `transition` (0 to 5000 ms) and the new `zcross_debounce` (50 to 150 microseconds) as flat parameters of `POST /settings`, and the status LED as `led_status_disable`. A value outside those ranges is refused before anything is written, and so is a desired power LED, which the Dimmer does not have. `dimming.leading_edge` is written as `pulse_mode` only when the device reports `pulse_mode_detected`, which the reference gives to the Dimmer 2 only, so it is refused on every Dimmer 1 and nothing is written. Calibration, night mode and warm-up are not covered yet, and whether a `pulse_mode` change needs a reboot, the key names and the ranges are not confirmed on hardware. Refs ginsys/shelly-manager#381.
- The Shelly Duo (`SHBDUO-1`) and the Shelly Vintage (`SHVIN-1`) are now supported Gen1 models. Applying a desired `light` section writes their one channel to `POST /settings/light/0` with only the keys set (`name`, `default_state`, `auto_on`, `auto_off`, `schedule`, `schedule_rules`) and reads it back, and a desired night mode (`dimming.night_mode`, `night_mode_start`, `night_mode_end`, `night_mode_brightness` of 0 to 100) is written to `POST /settings/night_mode` and read back. An `effect`, a `mode`, a second light, `transition` and the button keys, any other `dimming` field, a `relay` section, and a night mode on a model that has none are refused before anything is written, and the reset parameter is never sent. The settings `transition` and the Duo's `device_submodel` are not applied. The key names, ranges, the type ids a device reports, the form verb and whether the night-mode brightness takes 0 are not confirmed on hardware. Refs ginsys/shelly-manager#381.
- Applying a desired `light` section to a Shelly RGBW2 (`SHRGBW2`) now writes its channels: one color channel to `POST /settings/color/0` or four white channels to `POST /settings/white/<n>`, by the device mode (`system.device.mode`, `color` or `white`, written first), with only the keys set (`name`, `effect`, `transition`, `default_state`, `auto_on`, `auto_off`, `schedule`, `schedule_rules`, `btn_type`, `btn_reverse`), and reads each back. A light section for the mode the device is not in, an `effect` in white mode, `btn_type` or `btn_reverse` on white channels 1 to 3, an unknown mode, and a word or number outside the page's lists and ranges are refused before anything is written. The unused client method `SetColorMode` is removed: the page has no `/settings/mode` path and the mode is a parameter of `POST /settings`. The key names, words and ranges are not confirmed on hardware. Refs ginsys/shelly-manager#381.
- Applying a desired roller to a Gen2 or Gen3 device in cover profile (Plus 2PM, Pro 2PM, Pro Dual Cover) now writes
  the travel times, motor direction, input mode and swap, and the obstacle and safety settings with `Cover.SetConfig`
  on `cover:0`, sending only the keys set, and reads them back. A roller on a device in switch profile, a travel time
  outside 1 to 300 seconds, and a roller setting with no Cover key are refused before any write, and the apply never
  changes the profile. Changing the motor direction marks a restart owed. Refs ginsys/shelly-manager#381.
- `POST /api/v1/config/bulk-apply` stores one partial configuration as the overrides of up to 10 named devices and applies it through the per-device apply. It checks the configuration against every selected device's model before the first write, answers 200 with one entry per device (`applied`, `pending`, `unconfirmed`, `refused`, `failed` or `skipped`, whether the overrides are stored, and the setting paths that failed), and is not atomic: nothing is rolled back and the overrides stay stored when an apply does not succeed. Refs ginsys/shelly-manager#594.
- The bulk operations dialog has an Apply Configuration card, the first bulk operation that writes devices. You type one partial configuration as JSON (`null` removes an override, a secret sent as `********` keeps the stored one), review the exact devices and the exact JSON in a confirmation, and apply it to up to 10 devices. The result says per device what was written and whether the new settings are stored and the device stays pending. Only a refusal before any device was touched is reported as "nothing was written"; any other outcome, such as a timeout, is reported as unknown, with a reminder to check each device before sending again. Not in this card: a tag, group or template as the target, and a dry run. Fixes ginsys/shelly-manager#594.
- `POST /api/v1/config/bulk-download` downloads the desired configuration of up to 10 named devices as one JSON file, with every secret masked and an `error` entry for a device that cannot be read. It reads the database and contacts no device; the web UI does not offer it yet. Refs ginsys/shelly-manager#595.
- The bulk operations dialog has a Download Configuration card. It saves the desired configuration of the picked devices as one JSON file, with every secret masked as `********`, so the file cannot restore a device. No device is contacted, and a device that cannot be read is listed with the reason. The dialog shows the route's own text when it refuses a request, such as more than 10 devices, and the export and backup downloads now show a refusal's reason too. Fixes ginsys/shelly-manager#595.
- The web UI now asks for the admin key, so a production build works against a server that has an admin key set. The first request the server refuses with `401 UNAUTHORIZED` opens an "Admin key" dialog, and an "Admin key" button in the header opens it at any time. The key is kept in `sessionStorage` for the browser tab only (never `localStorage`, never in the bundle), the refused request is sent again once after you enter it, a refused key shows "That key was not accepted." and asks again, and "Forget key" clears it. The metrics WebSocket and the admin key rotation use the same key. `VITE_ADMIN_KEY` and `/app-config.js` still work for development. Fixes ginsys/shelly-manager#619.
- The Provisioning Agents page shows each agent's enrolment state and fingerprint, lists the agents that wait for approval to compare with the fingerprint in the agent's log, and approves, rejects (after a confirmation) and pre-approves agents, so an operator no longer calls the admin routes by hand. Refs ginsys/shelly-manager#638.
- The provisioner agent can enrol by key. Give it a seed (`shelly-provisioner seed` makes one; set `SHELLY_PROVISIONING_AGENT_SEED` or its `_FILE` form) and it registers with a key of its own: the manager lists its fingerprint as `pending`, the operator approves it, and the agent signs the manager's challenge to get its token. `shelly-provisioner fingerprint` prints the fingerprint from the seed alone. The agent's id is `keyed-` and the 32 characters of that fingerprint (not `key-`, which would read as a Mailgun key to secret scanners). The new `provisioning.enrolment` setting is `open` by default, so an agent with no seed registers as before; `strict` makes the agent refuse to start without a seed, which lets it work with a manager in `strict` enrolment. A provisioner config file that does not load now shows one fixed sentence instead of the decoder's text, because the decoder quotes the value it refused and a seed in the file would be printed; check the syntax with a YAML linter and each setting's type against `configs/shelly-provisioner.yaml`. Refs ginsys/shelly-manager#638.
- The admin key can now decide an agent's enrolment by key: `POST /api/v1/provisioner/agents/{id}/approve` and `/reject` move a row between the `pending`, `preapproved`, `approved` and `rejected` states, and `POST /api/v1/provisioner/enrolments` pre-approves a fingerprint before its agent starts. Rejecting ends the agent's session and fails its assigned tasks. The agent list shows each agent's `enrolment_state` and a readable `fingerprint`, and the health figures count only `legacy` and `approved` agents as active. No agent can enrol yet, so nothing changes for agents that register without a key. Refs ginsys/shelly-manager#638.
- The manager can now enrol an agent by key: a register that carries an Ed25519 `public_key` gets a `pending` row (about 20 may wait: the cap is soft), an approved or pre-approved key is asked to sign a 60-second `challenge`, and a good `signature` earns the token with no five-minute wait. The manager derives the fingerprint and the id (`keyed-` and the fingerprint) itself, and answers `pending`, `challenge` or `rejected` as a `200`, a token as a `201`, and `401`, `400`, `409` and `429` as the API reference lists. Agents that register without a key are unchanged. Refs ginsys/shelly-manager#638.
- The manager has a strict enrolment mode, `security.agent_enrolment: strict` (or `SHELLY_SECURITY_AGENT_ENROLMENT=strict`). In it a register without a public key is refused with `403` for every caller, and the token of an agent that is not approved by its key stops working on poll, task status and report, so the agent key alone creates no keyless agent, issues no token and opens no claim (a key enrolment still leaves a `pending` row until the operator approves it). It needs an admin key and the manager refuses to start without one; any value other than `open` (the default, unchanged behaviour) or `strict` also stops the start. Refs ginsys/shelly-manager#638.
- `DELETE /api/v1/provisioner/agents/{id}` (admin key) frees an agent's id and revokes its token, and fails
  the agent's assigned tasks. The manager also forgets by itself: a register starts a purge, at most once
  an hour, that removes agents silent for 30 days with no pending task and deletes completed and failed
  tasks last updated more than 30 days ago. Finished tasks now stay until a purge removes them, where a
  restart used to clear every task. Refs ginsys/shelly-manager#638.
- The web UI top bar now reaches every page. New entries: Templates, and Drift, Notifications and Provisioning menus next to Export & Import, so 15 pages that could be opened only by typing the URL are one click away. The Export ID and Import ID in the history tables open the export and import detail pages. The menus work from the keyboard: a menu stays open while focus moves into its items, and Escape closes it and returns focus to its button (Tab used to close the Export & Import menu before any item could be reached). Fixes ginsys/shelly-manager#641.
- A failure that arrives after you left the page, or while a dialog of the page is open over its
  error panel, now shows in one notice at the top right of the app, with the action and the device
  it belongs to, until you dismiss it (ginsys/shelly-manager#648). A second failure replaces the
  first. It covers the devices list, the device page, the device configuration page and the configuration
  history page. A
  failure on the page you are on, with no dialog over it, still shows in the page panel. The result
  of an apply that worked is still shown only on the configuration page.
- A "Common settings" page (Templates, `/templates/common-settings`) compares the last imported configuration of several devices (all of one model, or a hand pick) and saves what they share as a template. It is served by `POST /api/v1/config/common-settings` (admin key), which reads the database only. Secrets are masked and then stripped before any two imports are compared, and per-device values (name, address, channel names) are left out, so a password that is the same on every device is never returned. A device with no import is named and skipped, never counted as agreeing. This is the last slice of the work to create templates from device imports. Fixes ginsys/shelly-manager#747.
- The device configuration page has a **Create template from this import** button on the last imported configuration. A dialog lists the sections a template for the device's model can hold (`wifi`, `network` and `auth` start unticked), takes a name, a description and a scope, and saves a template. Secrets are never copied (the server's `********` is dropped and the secrets left out are named), and what is one device's own (name, hostname, MAC, MQTT client id and topic prefix, access-point name, channel names and every static address) is always left out. A refusal from the server shows its reason. Refs ginsys/shelly-manager#747.
- A stored SQLite backup can be restored again, in two steps that never replace the database under a running server. `POST /api/v1/import/restore` takes only a `backup_id`, unpacks a `.sqlite.gz` or `.sqlite.zip` backup into a directory beside the database, checks the SQLite header, `PRAGMA integrity_check` and the `devices` table, and answers 202 "applies at the next start" (`dry_run` checks and stages nothing). Restart the server, in Kubernetes restart the pod: before the database is opened the start keeps one `.pre-restore` copy of the live file, removes the stale `-wal` and `-shm` files and moves the staged file into place, and if the restored file cannot be opened or migrated it puts the copy back. `GET /api/v1/import/restore` shows the staged restore and the result of the last start, `DELETE` cancels it, and PostgreSQL and MySQL answer 501. The in-process restore (`RestoreBackup` on the SQLite provider, the backup plugin and the import engine) is removed; the Backup page still says restoring is not offered until its change lands. A backup made by a newer version is not detected. Only the `server` command applies a staged restore, and the base Kubernetes Deployment now uses the `Recreate` strategy so the old pod is gone before the new one starts. Refs ginsys/shelly-manager#748.
- The Backup page can restore a SQLite backup. A backup's Restore button opens a dialog: Check backup, then Stage restore, which runs the server's full checks and keeps a copy without touching the live database. The dialog says the restore is applied when the server next starts, so the operator restarts the server. The page shows the staged restore with a Cancel restore button, and after the restart the result of the last restore, with the server's reason when it failed. On PostgreSQL and MySQL the check still works and staging is not offered: restore is only available for SQLite. The notices that restoring was not offered are gone. Fixes ginsys/shelly-manager#748.
- Every real change to a device's desired configuration, by a template or override recompute or by an import, now leaves one history row with its old and new text, secrets masked; a device keeps about its newest 50. Nothing shows or uses the rows yet, so the desired configuration of a device changes exactly as before. A recompute now writes only to the state it read, and reads again (up to three times) if the device changed meanwhile. Deleting a device deletes its rows. Refs ginsys/shelly-manager#775.
- The device configuration page has a "Desired configuration versions" card: it lists the versions of the desired configuration and rolls the device back to one. The confirm shows the difference with the current configuration (secrets are not compared), the templates that will be detached and the way back; a version that needs secrets asks for them in the same dialog. The device is not contacted and stays pending until you apply. Refs ginsys/shelly-manager#775.
- `POST /api/v1/devices/{id}/config/new/rollback` sets a device's desired configuration back to a stored version, and `GET /api/v1/devices/{id}/desired-config/history` lists the versions. The device is not contacted and stays pending until an apply. A secret is never restored from history: the current one is kept where the place it is sent to is unchanged, and for every other the route answers 409 with the paths and takes the secrets you enter on the retry. Refs ginsys/shelly-manager#775.
- The service can now set a device's desired configuration back to a stored version: the whole version becomes the device's overrides, its templates are detached, and the device stays pending until an apply. A version that a recompute would change, or that the device changed under (an edit in between), is refused and nothing is written; a rollback to what the device already holds writes nothing. Refs ginsys/shelly-manager#775.
- The image build can now be called with a release version (the `version` input of the `docker-build.yml` workflow when another workflow calls it): both images get that version as a tag beside the date name, and both binaries report it. A release build does not move `latest`, which stays the newest main image. With no version the build is the one it has always been, and a binary built that way reports `dev`. Refs ginsys/shelly-manager#846.
- The application now has one version, set at build time with `-ldflags -X` on `internal/version.Version` and `dev` when a build sets none. `GET /api/v1/version` answers it as `version` beside `git_sha`, both binaries print it with `--version`, the manager logs it at start, and the provisioner sends it when it registers. The repository root `VERSION` file holds the next release, `v0.6.0-alpha.1`. A plain `make build` stamps it with `-dev` added (`v0.6.0-alpha.1-dev`), so an untagged build never claims a released version; a release build passes the bare tag as `make build APP_VERSION=<tag>`, and `APP_VERSION=` empty stamps nothing. The template variable `System.Version` changes from the fixed `"1.0.0"` to this version, the provisioner registers with it instead of the fixed `0.5.0-alpha`, and the version an export records (SMA `system_info.version`, GitOps `system_version`) is this version instead of the fixed `v0.5.3-alpha`. No tag or release is cut by this change. Refs ginsys/shelly-manager#846.
- A new `Release` workflow, started by hand from `main`, makes a release through the shared release workflow of ginsys/.github: it assembles the changelog fragments into a release pull request, the pull request merges after the required checks, and the workflow makes the tag on the merged release commit and moves `VERSION` to the next number. Seven actions (`release`, `release-as-beta`, `release-as-rc`, `release-as-stable`, `start-next-minor`, `start-next-major`, `skip-prerelease-number`); the last three only move `VERSION` and make no tag. The dry run is on by default and reads no secret, and a pull request that changes a release file runs it. A real run needs the GitHub App `ginsys-release-bot` (no ruleset bypass) and the `release` environment that the owner creates; until then only dry runs work, and nothing has been released. See `docs/development/RELEASING.md`. Refs ginsys/shelly-manager#846.
- The `Release` workflow now publishes in the same run that makes the tag: the tag it makes starts no other workflow. After the tag, it builds both images with the tag through `docker-build.yml`, builds the provisioner for linux amd64 and arm64 with GoReleaser (`CGO_ENABLED=0`, the tag stamped in, one `.tar.gz` each with the `LICENSE`), runs the amd64 binary and refuses it unless it reports the tag, and then the shared publish workflow creates a GitHub pre-release, never Latest, with one `SHA256SUMS` and the notes: that tag's `CHANGELOG.md` section and one fixed paragraph (back up the database before upgrading; to roll back, restore it and pin the previous tag). A re-run cannot push the images of a released tag again: the release guard refuses a tag that exists. A dry run builds the archives and previews the notes and the sums, and pushes no image. The old `Release Publish` workflow, which ran when a `v*` tag was pushed, is removed, and `Test` no longer runs a second time at the tag, because the release pull request ran it. Nothing has been released yet. Refs ginsys/shelly-manager#846.

### Changed
- The export and import detail pages now show the main fields of a result (not the `metadata`, the export `output_path` or `webhook_sent`): status, created time, duration, errors and warnings, and for an import the changes by type with the resource, ID and field of each (never the old or new values). Both link back to the history and use the page and card styling of the device page. The export page's Download button now fetches the file with the admin key and saves it, and a failed download shows the server's message; the old plain link sent no key and got 401 as soon as an admin key was set. The import history page now shows its statistics, and both history pages show the error when the statistics request fails, where they showed zeros before. An import has no download because the API has no such route, and results the server no longer holds in memory still answer "not found" (ginsys/shelly-manager#647). Fixes ginsys/shelly-manager#139.
- The admin key rotation page takes the new key in two password fields and rotates only when both match, refuses a key that starts or ends with a space (the server could not match it again), says that a rotation lasts only until the server restarts, and carries test ids for end-to-end tests. Fixes ginsys/shelly-manager#255.
- The Drift schedules page no longer says schedules are not executed. It reads the scheduler state and shows a banner when the scheduler is off on the server (`drift.scheduler_enabled`), a banner when that state could not be read, and the badge "Disabled", "Scheduler off" or "Enabled" on each schedule. Refs ginsys/shelly-manager#279.
- The plugin list (`GET /api/v1/export/plugins`) no longer returns `status.configured`, which was always `true`: plugin configuration is not stored, so there is nothing to report. `status` keeps `available` and `enabled`. A client that reads `configured` finds it absent. The docs now say that testing a plugin means `POST /api/v1/export/preview`, a dry run that does not check OPNsense credentials. Fixes ginsys/shelly-manager#290.
- Gen1 leftovers of the Shelly 2.5, the Dimmer and the Duo and Vintage: a template that carries input settings, applied to a 2.5 in roller mode (desired, or else the mode the device reports), is now refused as a whole with 422 naming `input.inputs[I]` and nothing is written, where it used to write the relay endpoint; keep the inputs out of that device's template and set `roller.button_type`, `roller.button_reverse`, `roller.swap_inputs` and `roller.input_mode` instead, and a configuration read from a roller-mode 2.5 now holds no `input` section. A Dimmer `fade_rate` outside 1 to 5 (the page's example reports 0) is read as no value, so an imported configuration applies again; a desired 0 stays refused. The Duo and Vintage now apply and read back `dimming.transition` (0 to 5000 ms), unconfirmed on hardware; `device_submodel` stays out. Refs ginsys/shelly-manager#381.
- A Shelly 2.5 can be switched between relay and roller mode through the desired configuration: the new `system.device.mode` field (`relay` or `roller`, the Gen1 counterpart of the Gen2 Sys profile) is written first, read back, and only then are the channel sections written, so the roller keys no longer fail on a device that still runs as a relay. A mode that is already on the device sends nothing, a mode that does not hold stops the apply before any other section, and a mode on another model, or the channel sections of the other mode, are refused before anything is written. Not confirmed on hardware. Refs ginsys/shelly-manager#381.
- Applying a configuration to a Gen1 device now writes Wi-Fi roaming (`wifi.roam_enable` and `wifi.roam_threshold`) and the Wi-Fi recovery reboot (`wifi.recovery_reboot`, on the Shelly 1, 1PM, 2.5 and Plug/PlugS), and reads them back. A threshold outside -100 to -50 dBm, the recovery reboot on a Shelly i3, and a key that the device does not report (firmware before v1.10.0) are refused before anything is written. Refs ginsys/shelly-manager#381.
- A Gen2 or Gen3 switch now takes its protection settings from a template: `max_power` (sent as `power_limit`), `voltage_limit`, `undervoltage_limit`, `current_limit`, `in_locked`, `autorecover_voltage_errors` and `power_restore_delay` (seconds, 0 for off), and an input takes `enable` and `factory_reset`. Each is written with `Switch.SetConfig` or `Input.SetConfig` and read back. A key the device does not report, a negative limit, a restore delay outside 0 to 600 and an `undervoltage_limit` above the `voltage_limit` are refused before anything is written; a limit the device itself refuses is a failed row. A cover `input_mode` of `dual` is refused too while the device's safety switch is enabled and the configuration does not set it. A Gen1 model keeps `max_power` as it was. The bounds are relayed from the Shelly pages and unconfirmed on hardware. Refs ginsys/shelly-manager#381.
- A Gen2 or Gen3 Pro device now takes its Ethernet and Bluetooth RPC settings from a template: `network.eth` (`enable`, `ipv4mode`, static `ip`, `netmask`, `gw` and `nameserver`) is written with `Eth.SetConfig` and `system.device.ble.rpc.enable` with `BLE.SetConfig` (`keep_running` is not written and is refused: its place on the Shelly page is unconfirmed), and both are read back. An Ethernet change is refused before anything is written when the manager reaches the device on its Ethernet address, or cannot prove it does not (it must have no Ethernet address, or use the Wi-Fi station address), when it comes with a Wi-Fi network change or a station password, when the device runs Ethernet in server mode, and when a static address has no address or no gateway. A device that does not report the `eth` or `ble` component is refused. Every Ethernet change owes a restart. The strand behaviour, the restart time and Bluetooth pairing are unconfirmed on hardware. Refs ginsys/shelly-manager#381.
- A Gen2 or Gen3 switch now takes `reverse` and `input_id` from a template. Each is written with `Switch.SetConfig` and read back, only where the device's own switch section reports the key; a device that does not report it, an `input_id` other than 0 or 1, and an `input_id` that changes in the same apply as an input mode are refused before anything is written. An `input_id` equal to the one the device runs is no change, so a template imported from the device can be applied again. An input mode for an input whose switch the device does not report (a light device) is refused before any write too, where it used to come back as a failed row. A changed `reverse` owes a restart whether or not the answer says so, because the Switch page says setting it requires one. The page names no `model` or `app` value for the devices that have `input_id` (Pro 1, Pro 1 PM, 1L Gen3), so there is no model table and which devices report the keys is unconfirmed on hardware, as are the keys' firmware and the `restart_required` answer. Refs ginsys/shelly-manager#381.
- A Gen2 or Gen3 light now also takes its limits and input mode from a template: `max_power` (watts, written as `power_limit`), `voltage_limit`, `undervoltage_limit` and `current_limit`, and `in_mode` (`follow`, `flip`, `activate`, `detached`, `dim` or `dual_dim`) on `light.lights[]`. Each is written with `Light.SetConfig` and read back, and a key the device does not report, a negative limit and an `in_mode` outside the six words are refused before anything is written. The Light page gives the limits no range and no order between them, so an `undervoltage_limit` above the `voltage_limit` is written and the device decides. The page lists `dim` and `dual_dim` as "if applicable", so a device may refuse them as a failed row. The key names and words are relayed from the Shelly Light page and unconfirmed on hardware, as is whether the device answers `restart_required`. Refs ginsys/shelly-manager#381.
- A Gen2 or Gen3 light (`light:N`) now takes its settings from the stored `light.lights` list, matched by `id`: `name`, `default_state` (`on`, `off`, `restore_last`, `toggle`; `last` is read as `restore_last`), `auto_on` and `auto_off` (seconds, 0 for off), and the new `transition_duration` (seconds), `min_brightness_on_toggle`, `button_fade_rate` (1 to 5), `night_mode`, `night_mode_brightness`, `night_mode_start` and `night_mode_end` (`HH:MM`, set together), `warmup_enabled`, `warmup_brightness` (10 to 100) and `warmup_time` (20 to 1000 ms). Each is written with `Light.SetConfig` and read back; only the keys that are set are sent, and inside `night_mode` and `warmup` only the sub-keys that are set. A light the device does not report, a key the device does not report, an out-of-range value, a word outside the list and a repeated or negative id are refused before anything is written. If the device answers `restart_required`, the restart is owed; the page does not say that it does. The light limits, `in_mode`, `op_mode`, `gamma`, `range_map` and `button_presets` are not written. The bounds are relayed from the Shelly Light page and unconfirmed on hardware. Refs ginsys/shelly-manager#381.
- The database pool defaults now live in one place. The PostgreSQL and MySQL providers carried fallbacks of their own that no running path could reach and that disagreed with each other and with the configuration (lifetime 1h and 30m against 5m, MySQL open connections 20 against 25), and a default-config function only tests called. Those are deleted: the config helper holds the defaults, and the providers apply the pool they are given. A configuration the helper already fills behaves exactly as before. Fixes ginsys/shelly-manager#581.
- A change now records its release note as one file, `changelog.d/<issue>-<slug>.md`, instead of an
  entry in `CHANGELOG.md`, so two open PRs no longer conflict on the `[Unreleased]` section when one
  merges. The `changelog-fragment` CI check fails a PR that adds no fragment or adds a line to
  `CHANGELOG.md`; the PR label `no-changelog` (CI, tests, docs-only changes, the release PR) and
  Dependabot or Renovate branches are skipped. The Release workflow cuts a release: it keeps every
  line under `## [Unreleased]`, adds the fragment sections after them, and removes the fragments. The
  entries already under `[Unreleased]` stay there until a release. The `merge=union` rule for
  `CHANGELOG.md` and its section in `CONTRIBUTING.md` are gone. Fixes ginsys/shelly-manager#600.
- The devices list, the device page, the device configuration page and the configuration history
  page no longer open a browser dialog when an action fails or finishes. A failed action shows an
  error panel with its error code and suggested actions, where it happened: under the toolbar, or
  inside the dialog that is open. The result of an apply shows in a card on the configuration page
  (it names the device when you have moved to another one), and an empty template id is
  flagged next to the field. The other pages still use dialogs, tracked in
  ginsys/shelly-manager#614.
- The devices list keeps its rows on screen while it refreshes, and a refresh updates the name and address of ticked devices that are on the page. Opening the bulk operations window now checks the ticked devices on other pages against the server with one list request, drops those that no longer exist and says so in one line. Fixes ginsys/shelly-manager#618.
- A provisioner agent that registers without a key can no longer create a new row with an id that holds a space or a byte outside printable ASCII (`0x21` to `0x7E`); the register answers `400` and leaves no row. Ids that start with `provisioner-`, the ids of the agents that exist today, may hold any bytes, and a row that already exists keeps registering. Refs ginsys/shelly-manager#638.
- The provisioner registry gains the columns for agent enrolment by key, all nullable, and every existing row becomes `legacy`. A keyless register of an id whose row is not `legacy` is refused with `409` (the admin key included), and a keyless register of an id that starts with `keyed-` in any case is refused with `400`; ids of agents registered today, which start with `provisioner-`, are unaffected. Refs ginsys/shelly-manager#638.
- The hourly registry purge now goes by an agent's enrolment state. Agents that register without a key (state `legacy`) are removed after 30 days of silence as before; a `pending` agent after 7 days; an `approved` agent keeps its row, key and approval and only loses its session after 30 days; a `preapproved` or `rejected` row is never purged. No row is in a state other than `legacy` until the admin routes exist, so today's behaviour does not change. Refs ginsys/shelly-manager#638.
- The provisioner registry (agents, their tokens and the tasks) now lives in the database instead of the
  manager's memory, so a manager restart keeps it and every replica sees the same registry. A token is
  stored only as a hash. An agent id, a task type, a task status or a task agent id longer than 191
  characters is now refused with 400. A poll that finds no task answers an empty list instead of null. An agent
  that was silent for more than 5 minutes and then reports devices is listed online again, and cancelling
  a task no longer changes a sibling task that a bulk create had given the same configuration.
  On MySQL the two new tables use a binary collation, which needs MySQL 8.0.17 or later; on MariaDB or an
  older MySQL the manager logs a warning at start-up and keeps the default collation. Refs
  ginsys/shelly-manager#638.
- The agents page summary card that counted only keyless and approved agents reads "Enrolled" instead of "Total", and the "Waiting for approval" hint no longer names a version that table does not show. Refs ginsys/shelly-manager#638.
- The device configuration page no longer keeps a second copy of an action failure. The
  configuration store sets its error only when a load fails, and an action failure shows once, in the
  page's error panel (ginsys/shelly-manager#650). An apply that changed the device no longer reports
  a failed apply when the status read after it fails: the result card shows and the failed reload
  shows as a load error.
- The server now closes a new connection that has not sent its complete request header within 10
  seconds, and a keep-alive connection that sends no new request for 120 seconds (on such a
  connection the 10 seconds start once the first bytes of the next request arrive). Before, both
  were held open for ever. Nothing needs setting, and request deadlines are unchanged (30 seconds,
  and 5 minutes on `bulk-apply`). A client that sends its body or reads its answer slowly is not
  covered by these two limits. Fixes ginsys/shelly-manager#726.
- The device configuration page now shows the last imported configuration, with its import time and status, and the history of that configuration, where each entry opens to its settings as fields. A device that was never imported says so, and `GET /devices/{id}/config` answers 404 for it where it answered 500. The template page shows its config as fields, with the JSON behind a Raw JSON toggle. The history page listed nothing before, because the client read another shape than the server sends; it now lists the entries, and its Restore button is gone: it merged the entry into the stored row and could not bring back a secret (a real restore is ginsys/shelly-manager#775). Refs ginsys/shelly-manager#747.
- Downloading the configuration of several devices now takes up to 200 devices instead of 10. The download reads the database and contacts no device, so the old cap, copied from bulk apply, had no reason; a download of 200 fully populated configurations is about 1 MiB. More than 200 is still refused with a message that names the limit, and bulk apply keeps its limit of 10. Fixes ginsys/shelly-manager#751.
- The old configuration history page is gone: "View History" on the device page opens the History card of the configuration page, and a saved link to the old address lands there too. The card shows the same rows (time, source, user and the recorded configuration). A test now shows that no secret reaches a log line or an answer of the rollback route. Fixes ginsys/shelly-manager#775.
- `mise.toml` now pins Node 24 beside Go and golangci-lint, and `mise.lock` carries it, so `mise install` gives a contributor, or a container that installs only the mise tools, what the frontend checks (`make typecheck-check test-vitest`) need. CI keeps its own Node 24 setup and does not install through mise. Refs ginsys/shelly-manager#777.
- A backup is now written only inside one directory: `sync.export_base_dir` when it is set, `./data/backups` when it is not. A backup export whose `output_path` is outside it answers `400` with that directory in the reason and creates nothing; before, it wrote to any directory the server could write. An empty `output_path` still means the default, and the result's `output_path` is now an absolute path. A deployment that kept backups elsewhere sets `sync.export_base_dir` (and `export.output_directory`, for downloads) to that directory. Fixes ginsys/shelly-manager#785.
- A drift schedule update now carries the `revision` the caller read. `PUT /api/v1/config/drift-schedules/{id}` writes only if it is still the stored revision; another one is `409` with the current schedule in `error.details` and nothing is written, and a body without `revision` is `400`. Every schedule answer now carries `revision`. A save from an old copy of a schedule can no longer put its old fields back after an edit or a toggle. The Drift schedules page sends the revision it opened and, on a `409`, shows "This schedule changed since you opened it. Close this form and open the schedule again.", turns Save off in that form and keeps the current schedule in the list. Fixes ginsys/shelly-manager#790.

### Removed
- The API no longer restores a backup, and `POST /api/v1/import/backup` and `POST /api/v1/import/backup/validate` are gone: they took a file path from the client, and a restore replaced the live database with the file's raw bytes. `POST /api/v1/import` and `POST /api/v1/import/preview` now answer 501 for the backup plugin, whatever the source or the dry-run flag, so an operator who restored a raw SQLite file through the API loses that until a safe restore ships (ginsys/shelly-manager#748). In its place `POST /api/v1/import/restore-preview` takes only a `backup_id`, finds the stored file in the export history, refuses anything that is not a regular backup file inside the export directory, and reports whether the file is present and can be opened, with its size and record count; it reads no contents and restores nothing. The Backup page's Restore button became Check: its dialog shows that result and says restoring is not offered yet, and the client functions for the restore steps that never had a server route were removed. Refs ginsys/shelly-manager#249.
- The Drift schedules page no longer has a pager. The schedule list route answers every schedule in one plain array with no paging data, so the pager could never show. Refs ginsys/shelly-manager#279.
- The security monitor and its IP blocking are gone. It counted suspicious requests per client IP, could refuse an IP for a while, and served `GET /api/v1/metrics/security`. That route now answers `404` (so the earlier note that it stays production-only in test mode no longer applies), and the Metrics page no longer shows a Security panel. The `security.monitoring.*` and `security.ip_blocking.*` keys (and their `SHELLY_SECURITY_MONITORING_*` and `SHELLY_SECURITY_IP_BLOCKING_*` variables) are no longer read: a config file that still sets them starts as before, with the keys ignored, and the start-up check that refused some combinations of them is gone. The rate limiter, the security headers, CORS and request validation are unchanged. The `component` field of the request log lines changes from `security_monitor` to `security_logging`; the suspicious-request warning is gone. Fixes ginsys/shelly-manager#876.

### Fixed
- The metrics dashboard now reads what the server sends. The summary, notification and resolution
  panels show the real fields, a successful test alert is reported as a success instead of
  a failure, and one failing panel no longer blanks the others: the error banner names the panel
  that failed. The panels for
  exports, imports, recent notifications and resolutions by user are gone, because the server has
  no such numbers. Fixes ginsys/shelly-manager#134.
- The GitOps Export page now talks to routes the server has, and the export history shows what a GitOps
  record holds. The page lists the GitOps history, reads one record, previews through the generic
  preview route and creates with only what the server reads: a name, a grouping (location or device
  type), the common and template options and a device filter. The form options that no part of the
  server read (Terraform, Ansible, Kubernetes and Docker Compose formats, repository structure, git
  and webhook settings, secrets and README switches, fixed sample devices, the fixed integration
  panel, per-format statistics and the Download button) are gone; a GitOps export is a directory and
  the server does not zip it yet. New `DELETE /api/v1/export/gitops/{id}` deletes the history record
  of a GitOps export and never touches the exported files. The preview now follows the common and
  template options and lists the devices no group claims under `ungrouped/`, as the export writes
  them. The history size of a GitOps export is now the total size of the exported files, where it
  was the size of the directory entry. Fixes ginsys/shelly-manager#249.
- The notification pages now match the API. The API gains `GET /api/v1/notifications/channels/{id}` and `DELETE /api/v1/notifications/rules/{id}` (a rule is deleted with its delivery history). Disabling a channel, and creating a channel or rule with `enabled: false`, now take effect. A channel `PUT` replaces `name`, `type`, `enabled`, `config` and `description`, so a body that omits `enabled` disables the channel and one that omits `description` blanks it. The pages read and write the API's own field names, the history page pages through the server's `limit` and `offset` and shows the trigger and sent time, the rules page lists and creates rules with `alert_level`, `min_severity` and `categories`, and the channel page can send a test. Fixes ginsys/shelly-manager#250.
- A drift schedule now carries a revision that every edit, toggle and disable bumps, and the scheduler claims a tick only at the revision it read. A tick read before a change of the device selection, or before a disable and enable, is no longer run from the old read. An edit costs the tick in flight one claim; whether the schedule then runs on the next poll or at the next instant of its spec depends on whether the edit recomputed its next run. Refs ginsys/shelly-manager#279.
- The configuration form showed a list of items that have no `id` (the Gen1 `actions` and the `tariffs`) as a note with no way to add an item, and an item edited there reached the server without its other fields. It now shows such a list as a JSON box. The relay list, whose items carry an `id`, keeps its fields. Refs ginsys/shelly-manager#381.
- The configuration form of a Gen2 or Gen3 device no longer reads "nothing can be applied". `GET /api/v1/config/schema?device_id=<id>` now answers for a Gen2 device with `x-apply-supported` true and the sections a Gen2 apply writes, and marks `x-applicable` exactly the fields it writes: the system name, eco mode and discoverable flag, time server, location, MQTT, cloud, login, Wi-Fi station, Ethernet, the switch, input, light and cover fields. The marks of the switch, input, light and cover fields come from the same lists the apply check reads, so a key a later change adds is marked with no second edit, and a test sets every field and every enum word to prove each mark. Enums list only words the Gen2 apply accepts (an input type of `analog` is gone, and the login user is `admin` only), and the cover times are 1 to 300 seconds. The schema is one for every Gen2 model: a device without Ethernet, a cover or a light still shows those fields as applicable, and apply refuses them from the device's own answer before anything is written. `wifi.enable` stays not applicable because apply refuses `false`; `system.device.ble` and `wifi.ap` have no field in the schema yet. A global or group template marks a field applicable when either generation writes it, so a Gen2-only field such as `relay.relays[].voltage_limit` is editable there and is refused when it reaches a Gen1 device. Refs ginsys/shelly-manager#381.
- Verify now compares a struct that has no exported field as one value. Before, it compared nothing
  inside it, so two different `power_metering.energy_reset` times were reported as a match; the same
  instant written in two time zones is still a match. A desired `energy_reset` against a read-back
  without it stays a `removed` difference, which is the true answer: no device reports a reset time and
  apply refuses one. Fixes ginsys/shelly-manager#567.
- A discovery pass no longer changes the version of a device it found unchanged, so a device form that was open during a pass can still be saved. The version now moves only when the address, the type, the firmware or the stored settings changed, and one helper sets the next version for every writer. Fixes ginsys/shelly-manager#583 and ginsys/shelly-manager#580.
- `shelly-manager discover` says "Added to database" for a device that the run added and "Already in database" for a known one; it printed "Already in database" for every device. `shelly-manager add <ip> [name]` no longer fails with a duplicate key on every call: it reports the device at that address as added, applies the name, or says the device is already known. Fixes ginsys/shelly-manager#585.
- The Dockerfiles, compose files, test scripts and docs now name only the environment variables the
  app reads. `GIN_MODE` and `SHELLY_TEST_MODE_FAST_HEALTH` are gone, and the bare `DATABASE_PROVIDER`,
  `DATABASE_DSN`, `POSTGRES_*`, `MYSQL_*`, `SHELLY_PROVISIONING_AUTO` and `SHELLY_HTTP_PORT` became
  `SHELLY_` plus the config key. A test fails on an unread name in those files. The smoke test now
  starts its backend on the port it checks. The provisioner's `api.url` has no default any more, so
  `shelly-provisioner agent` with no URL stops with "API URL required" instead of dialling
  `http://0.0.0.0:8080`. Fixes ginsys/shelly-manager#589 and ginsys/shelly-manager#609.
- The PostgreSQL provider now accepts the pgx client options `target_session_attrs`, `sslcertmode`, `sslsni`, `sslnegotiation`, `channel_binding`, `require_auth`, `statement_cache_capacity` and `description_cache_capacity` in the DSN next to `database.username` and `database.password`. Every key that could carry a credential, a host or a user is still refused. Fixes ginsys/shelly-manager#590.
- The provisioner DaemonSet in the kustomize tree now reaches the manager. It set no
  `SHELLY_API_URL`, so it fell back to `http://0.0.0.0:8080`, and being on the host network it also
  could not resolve a Service name. Each overlay now sets `SHELLY_API_URL` to its own Service
  (`http://dev-shelly-manager:80`, `staging-` and `prod-` likewise), the DaemonSet reads that one
  key from `shelly-manager-config`, and `dnsPolicy` is `ClusterFirstWithHostNet`. An overlay that
  sets none leaves the pod unable to start instead of dialling a default.
  Fixes ginsys/shelly-manager#591.
- A SQLite DSN parameter the driver refuses, such as `_txlock` or `_time_format`, is now named in the connect error without its value. The driver quoted the value, so a mistyped secret in the DSN query could reach the error and the log. Fixes ginsys/shelly-manager#596.
- Errors from an open database connection no longer carry driver text into the logs or the API.
  A rejected statement, a transaction that could not begin, a failed migration or `Close`, and the
  apply lock's queries used to quote the server address or socket path, identifiers and values;
  they now report `database operation failed: <kind>` with the SQLSTATE or MySQL error number
  where there is one. `gorm.ErrDuplicatedKey`, `gorm.ErrRecordNotFound` and context errors still
  answer `errors.Is`. Fixes ginsys/shelly-manager#601.
- Drift detection now compares the live device with its desired configuration, the one apply
  writes and verify reads, instead of the stored import record. A device that was just applied no
  longer shows drift against an old import, and a device that left its desired configuration is
  no longer reported "in sync" because it still equals that import. A device with no desired
  configuration cannot be checked: the single check answers 404 and a bulk run marks it skipped
  with the reason, where both used to compare it with whatever the import record held. Fixes
  ginsys/shelly-manager#606.
- No page shows an outcome in a browser alert any more. A page that gets its data through a store shows the store's error where you acted: in the list, or inside the dialog that stays open (new channel, new rule, resolve drift, save template). The metrics dashboard and the template detail page show a failure in an error panel (in the app notice while the assign dialog covers the page) and a success in one status line. A test now fails if a page calls `alert(`. A failed delete or create still replaces the whole list with the error text (ginsys/shelly-manager#701). Fixes ginsys/shelly-manager#614.
- The example environment files told the operator to set a provisioner key that is not the agent key: the Compose example had `API_KEY`, which Compose does not read (the provisioner sends `AGENT_API_KEY`, or `ADMIN_API_KEY` when that is unset), and the root example had no agent key line. Both examples now say the provisioner gets the agent key, never the admin key, and have a line for it; the Compose example also loses two retry settings that no Compose file reads. The Compose `.env` is no longer tracked and is now ignored by git, so only the examples are tracked. If you edited deploy/docker-compose/.env, run `mv deploy/docker-compose/.env deploy/docker-compose/.env.mine` and `git checkout -- deploy/docker-compose/.env` before you update, then `mv deploy/docker-compose/.env.mine deploy/docker-compose/.env` and add an AGENT_API_KEY line. If you never edited it, make it again from deploy/docker-compose/.env.example. Compose reads that file from its own directory: without it every setting falls back to its default and the stack starts with no error. Fixes ginsys/shelly-manager#617.
- The provisioner agent can now register with the manager and read its answers. `POST
  /api/v1/provisioner/agents/register` answered without the `{success, data}` envelope, and the agent's
  client read every answer from the top level, so registration failed with "agent registration failed" and
  a poll showed no tasks. The register route now uses the envelope, and the client reads `data`. So do
  `GET /api/v1/provisioner/tasks` (its list is now under `data.tasks`; no web UI code reads it) and the
  provisioner routes that carried a second `success` inside `data`, which is gone. The agent also registers
  again before a poll when its start-up registration failed or the manager lost it in a restart, instead of
  failing until it was restarted. A test now runs the real client against the real router. Fixes
  ginsys/shelly-manager#627.
- The admin key dialog no longer reopens after Cancel when a late or retried request is refused: the refused key is still forgotten, but the operator is not asked again until they open the dialog from the header. Fixes ginsys/shelly-manager#632.
- Verify of a device with no desired configuration now answers 404 `NOT_FOUND`, as drift does, instead of reporting a match, and the configuration status route no longer words an old stored `drift` value as drift detected: `sync_status` is import and restore state only. Fixes ginsys/shelly-manager#635 and ginsys/shelly-manager#636.
- The export and import result routes (`GET /export/{id}`, `GET /export/backup/{id}` and `GET /import/{id}`) answer from the history record when the server no longer holds the result in memory, so a row the history lists no longer leads to "result not found" after a restart. The answer has the fields the record holds and leaves out the rest. Fixes ginsys/shelly-manager#647.
- A path rate limit now counts only the requests to its own paths, and the device control limit is
  enforced for the first time (ginsys/shelly-manager#658). Before, the bulk limit of 20 was
  compared with all the requests of a client IP: after 20 ordinary requests in the hour, the first
  bulk call was refused with 429 and the client was blocked on every route for 15 minutes. A path
  limit now refuses only its own paths until its one-hour window resets and blocks no other route.
  The control limit of 100 an hour never matched a real request before; it now counts per client IP
  and per device, and the longest matching path pattern wins. The default limit of 1000 requests an
  hour and its 15-minute block are unchanged.
- The result of an Apply (applied, applied but unconfirmed, or completed with errors) now shows in the app notice when you left the device configuration page before the request ended, as a notice or a warning and no longer as an error. Fixes ginsys/shelly-manager#660.
- A request that times out now has exactly one writer. The timeout middleware used to answer on
  the same response writer that its handler was still writing to, which is a data race, and
  `go test -race` reported it. The handler now writes through a guarded writer, so after the
  deadline its writes are dropped. Two cases change for the client: a handler that had already
  started its response keeps it and no longer gets the 408 JSON appended to its body, and headers
  the handler had set but not yet sent no longer appear on the 408 answer. A panic in the handler
  no longer ends the server process: it is logged and answered with a 500 when nothing was sent
  yet. Fixes ginsys/shelly-manager#664.
- A request refused by a path rate limit now reports that limit (ginsys/shelly-manager#665). The `429` showed the default limit in `X-RateLimit-Limit` and a reset one window from now whichever limit refused the request. It now shows the limit of the rule that refused, and `X-RateLimit-Reset` and `meta.retry_after` name the end of that rule's window for the client, as the same whole second. A refusal by the default limit and a request that passes are unchanged.
- Rotating the admin key is now one atomic switch. The key was kept in four separate fields (API, export, import and metrics handlers), written with no lock while requests read it, and changed one handler after another. A request that ran during a rotation could read a half-written key (a data race), and for a moment some routes accepted the old key while others took the new one. All four handlers now share one key holder, so the old key stops working on every route at the same instant. The development `/app-config.js` page now serves the live key, so a rotation shows there too, including one made from a manager that started with no key. Fixes ginsys/shelly-manager#668.
- A request refused by the default rate limit now reports when the client is let in again (ginsys/shelly-manager#677). The `429` always showed a reset one window from now, whatever the client's real return moment. `X-RateLimit-Reset` and `meta.retry_after` now name the first whole second after the later of the client's window end and its 15-minute block end, the same way a path limit's refusal does. When a request is refused, the limit and the block are unchanged.
- Rotating the admin key now refuses a new key that starts or ends with whitespace, with the same
  400 validation error as a blank key, and keeps the stored key. A header value loses trailing
  whitespace, and leading whitespace too in `X-API-Key`, so most such keys could not be presented
  again and every client, the one that rotated included, was locked out until a restart. Whitespace
  inside a key is still allowed. Fixes ginsys/shelly-manager#680.
- With `security.cors.allowed_origins` set, a request from an origin outside the list no longer gets
  `Access-Control-Allow-Origin: *`. It gets no `Access-Control-*` header, so a browser refuses its
  preflight and will not let the page read the response. A listed origin, a request with no `Origin`
  header, and a server with no allowlist (still `*`) behave as before. Fixes
  ginsys/shelly-manager#687.
- The database DSN check read the text before the driver decoded it, so it stopped nothing real and refused valid input: PostgreSQL could not set `default_query_exec_mode`, and a host, a database name or a password holding a word the check listed was refused. Both providers now read the DSN as net/url and the driver do. The MySQL provider reads the `timeout`, `readTimeout` and `writeTimeout` options as durations and refuses any other value, escapes the `charset` option (a list keeps its commas), and both providers add a default when the DSN does not give its key, no longer when a word of the connection text matches it. MySQL session settings are not filtered: a setting the server does not know stops the first connection with the server's error 1193 (`unknown system variable`). Fixes ginsys/shelly-manager#692.
- A refused backup download now shows the server's message (for example "Admin authorization required") and not "Request failed with status code 401". Fixes ginsys/shelly-manager#697.
- A failed delete, cancel, create, update or refresh on the notification, provisioning, drift schedule and template pages no longer replaces the list with the error: the list stays, and the failure shows in a panel above it or inside the open dialog. The stores keep their error field for list-load failures only. After Enable or Disable on the metrics dashboard, a failed status refresh is now said on the page, and a failed refresh of the advanced metrics or of a saved template shows in the app notice when the page was left or a dialog covers it. Fixes ginsys/shelly-manager#701.
- Five API answers now carry the standard envelope. `GET /api/v1/devices/{id}/config/current/normalized` with an id that is not a number answers 400 `BAD_REQUEST` instead of HTTP 200 with a string error. A request body over the size limit answers 413 `REQUEST_TOO_LARGE` instead of an empty 413. `GET /api/v1/provisioning/status`, `POST /api/v1/provisioning/provision` and `GET /api/v1/dhcp/reservations` send their object or array under `data`, so a client that read the bare body reads it from `data` now. The discover log now counts the devices a pass adds (`new_devices` read 0 every time), and `discovery.enable_mdns: false` now stops the mDNS multicast query, which every discovery sent whatever the setting said. Fixes ginsys/shelly-manager#707.
- The MySQL provider now refuses at startup the TLS settings the driver cannot use. `options.tls`
  takes only `false`, `true`, `skip-verify` or `preferred`; `required`, `verify-ca`, `verify-identity`
  and `custom`, and the options `ca`, `cert`, `key` and `serverName`, are refused with an error that
  names the supported modes, and the provider no longer writes those four options into the DSN. The
  MySQL docs and the Kubernetes example now match: the DSN comes from a Secret through
  `SHELLY_DATABASE_DSN`. `configs/shelly-manager.yaml` no longer carries a default `api.url`, which only
  the provisioner reads. Refs ginsys/shelly-manager#705.
- The device scanner, the generation detection, the Gen1 and Gen2 clients and the provisioner no
  longer follow an HTTP redirect from a device: the redirect is returned as the answer, so a device
  or a host that answers with one cannot send the manager to another address. The provisioner
  reports any answer outside 2xx as a failed request. Redirect behaviour of real devices was not
  checked on hardware. The unit tests also stopped sending packets off the host: they use a closed
  loopback port, a loopback range or a local test server, and the `scan-ap` CLI test runs with no
  WiFi tool on its PATH. Refs ginsys/shelly-manager#712.
- Saving a template or a device override now stores the new desired configuration and marks the device pending in one database write. Before, a failure of the second write could leave a device showing as applied against a configuration it never received. Fixes ginsys/shelly-manager#716.
- With an admin key set, a write request (POST, PUT, PATCH or DELETE) that carries no key at all now
  answers 401 before the server reads its JSON body. Before, a malformed body drew a 400, and a valid
  one cost a full parse before the 401. A request that carries a key, right or wrong, is checked as
  before. With no admin key configured nothing changes (ginsys/shelly-manager#720).
- `POST /api/v1/config/bulk-apply` no longer ends in a 408 with no result after writing to some devices. The call has a request deadline of 5 minutes (every other route keeps 30 seconds) and a run budget of 4 minutes: when the budget is spent or the client has left, no later device is started, it is reported `skipped` with a reason and has nothing stored, and the device being written is not cut by the client leaving. A 408 on this route can still happen (for example when a device ignores its context), and then the outcome is unknown. Fixes ginsys/shelly-manager#725.
- `POST /api/v1/config/bulk-import`, `bulk-drift-detect` and `bulk-drift-detect-enhanced` no longer end in a 408 with no result when their devices are slow or dead. A run stops starting devices 10 seconds before the 30 second request deadline, or when the client has left, and answers with each device it did not start as `skipped` and the reason; the enhanced route then answers the basic drift result (no report is stored for the devices that did run). A device that accepts the connection and never answers can still pass the deadline, and the outcome is then unknown. Fixes ginsys/shelly-manager#730.
- The OPNsense plugin's schema example no longer shows `${OPNSENSE_API_KEY}` and `${OPNSENSE_API_SECRET}`, which nothing expands: it shows plain placeholders, and the `api_key` and `api_secret` descriptions say the value is sent as written. Fixes ginsys/shelly-manager#736.
- The GitOps export no longer copies the request configuration into `export-summary.yaml`: a secret in the request, such as a token, reached a file that normally goes into a git repository. The file keeps its export metadata, structure counts and generation time. Fixes ginsys/shelly-manager#737.
- A notification channel can now be created from the Channels page. The form always sent an empty config, which the server refuses for every type, so no channel could be created. It now has the fields of each type: email recipients (one or more) and subject; webhook URL, method, headers and secret; Slack webhook URL, channel and username. Changing the type clears what was typed for the other one. A config the server refuses now shows the server's reason, for example "email config must have at least one recipient", where it showed "Request failed with status code 400". A message template, webhook timeout and retries and the Slack icon are not in the form, and a channel's config still cannot be edited after creation (ginsys/shelly-manager#783). Fixes ginsys/shelly-manager#744.
- A GitOps export no longer deletes a directory it did not write, and no longer writes outside its output directory. It used to remove `config.output_path` whatever it held, so one `POST /api/v1/export/gitops` could wipe any directory the server user can write; and a group name (a `group_mapping` value) or device type such as `../../x` was joined into the output path unchecked. Now an existing non-empty `output_path` is cleaned only while it holds the `.shelly-manager-gitops` marker file or the `export-summary.yaml` file earlier versions wrote, either as a regular file (a symlink or a directory of that name does not count); any other directory answers 400 and is left as it is. An empty or new directory works as before, and the next export replaces the files of the previous one. Group and device type directory names are now lower-cased with `/`, `\`, spaces and a few other characters replaced (as device file names already were), so two names can share one directory and their device files then sit together; a name that is empty, `.` or `..` becomes `unnamed`. An export marks the directory it writes, so files a user adds to such a directory are removed by the next export, including after deleting `export-summary.yaml` from it. Fixes ginsys/shelly-manager#745.
- Downloading a GitOps export now sends a zip of its directory. `GET /api/v1/export/gitops/{id}/download` used to run a file server on the directory, which answered with a redirect or a listing of names, so nothing usable came back. It now streams `application/zip` (named `<directory>.zip`, files under their relative paths, built as it is sent, no temporary file). It zips only a directory that still holds the `.shelly-manager-gitops` marker file as a regular file and answers 403 for any other; a symlink inside the tree is skipped, never followed; a path outside the export base directory is still refused. Files added to the directory after the export are in the archive too. The GitOps page has its Download button back on every successful row. Fixes ginsys/shelly-manager#749.
- The "Bulk Operations" button on the Devices page now says why it is greyed out: tick one or more devices first, or, when the Select column is hidden, that the column is hidden and can be shown again from Columns. Before, the button stayed greyed with no reason, and with the Select column hidden (the browser remembers that) nothing on the page said how to enable it. Fixes ginsys/shelly-manager#750.
- Bulk import and both drift checks no longer fail after 10 seconds. They wait as long as the server does for the devices, so a result that takes 12 seconds is shown as a result. A request that gets no answer at all now shows a timeout in the dialog, naming the operation and the number of devices, instead of "Contact support". Fixes ginsys/shelly-manager#752.
- A call to a device that retries no longer sleeps through the pause between attempts after its request has ended. The Gen1 and Gen2 clients now stop at once when the request's time is up, so a device that does not answer holds a request for the time the request allows and no longer for several seconds more. Part of ginsys/shelly-manager#755.
- A device that does not answer no longer blocks its detail page or comes back as "Internal server error". The status, energy and current-configuration reads now stop before the server's 30 s request deadline and answer 504 `DEVICE_UNREACHABLE`, with a message that names the device and its address. The page shows the stored device at once and fills in each live read as it arrives, and a device last seen more than a day ago no longer shows "online" in the list. Fixes ginsys/shelly-manager#755.
- The device detail page has one control that opens the configuration page instead of two ("View Full Config" is gone, "Edit Configuration" stays). The drift status now says why it is empty: "No desired configuration yet: add a template or overrides" for a device with no desired configuration, and "Could not check drift" with the reason when the check fails. The drift check waits as long as the server does for the device. Fixes ginsys/shelly-manager#756.
- A device that refuses the saved login is now reported as a refused login even when the identity
  check that follows it times out. Status, energy and the current configuration reads answer 502
  with the message that names the credentials, where they said "did not answer" or a bare 500
  before. An identity mismatch keeps its own answer. Fixes ginsys/shelly-manager#759.
- A GitOps export or preview that leaves out `include_common` or `include_templates` now writes the common file and the templates, as the plugin's schema says (both default to `true`). It used to read a missing key as `false`, so a client that followed the schema got neither. The visible effect: a stored or scripted request that omits the keys now exports more files than before; send `false` to leave them out. The page always sends both keys, so exports made from it do not change. A value that is not a boolean is refused with a validation error, as a wrong `group_by` already was. Fixes ginsys/shelly-manager#763.
- A device's relays and other nested configuration now show as labelled fields instead of one JSON string, and the override editor edits each relay in its own fieldset and saves only the relays and fields you change. A list of text values (schedule rules) is one entry per line. A refused save shows its error under the relay field it names. Fixes ginsys/shelly-manager#765.
- `shelly-provisioner status` no longer reports a Wi-Fi device that cannot scan as available and working. It prints the device name and its NetworkManager state, and a device that is unavailable or missing is a failure line that names `nmcli radio wifi` and `rfkill list`, an unmanaged one names `nmcli device set <device> managed yes`, and a state `nmcli` cannot read is a failure line too; every `nmcli` call runs in the `C.UTF-8` locale, so a translated `nmcli` hides neither a device that cannot scan nor an active connection (disconnect and the current-connection read compared "activated" in English) and non-ASCII network names stay intact; "disconnected" is not a failure. A scan on such a device now fails with that reason instead of returning an empty list, so access-point discovery fails with the reason where it printed "found 0 networks". A good scan that sees nothing is a warning, not a tick. The shipped provisioner config logs at `info`. Fixes ginsys/shelly-manager#766.
- A POST, PUT or PATCH with no body is no longer refused for lacking a `Content-Type` header. The
  Metrics page's collection controls (enable, disable, collect) and the notification channel test
  sent no body, so axios sent no content type and the server answered 400 "Content-Type header is
  required"; curl without `-H` failed the same way. A request with a body and no type, or with an
  unlisted type, is still refused. Fixes ginsys/shelly-manager#767.
- A SQLite backup is now a consistent snapshot of every committed write. It was a copy of the database file, which misses writes that still sit in the `-wal` file, so a restore could bring back an older database than the one that was backed up. Backups keep their file names and `.gz` and `.zip` handling. While a backup runs it needs free space for one full copy of the database in the directory of the database's DSN path (beside the live database file for a plain path), and it holds the database's one connection, so other queries wait for it. Fixes ginsys/shelly-manager#772.
- A notification channel's config can now be edited from its page. The channel page showed the config read-only, so the recipients, URL, headers and secret of a channel could never be changed. An Edit button on that page now opens the same fields as the create form, filled from the channel. The server never shows a stored credential, so the URL, the headers and the secret each have a "Keep the stored ..." box, ticked: a credential you leave alone is kept, and one you replace is sent as you typed it. Unticking the headers with no row, or the secret with no value, removes it. A config the server refuses shows the server's reason and keeps what you typed, and Cancel sends nothing. A save keeps the config keys the form does not show (a template, a timeout, retries, a Slack icon). A refused enable or disable of a channel now shows the server's reason too. The per-type fields are one component shared by the create and edit forms. Fixes ginsys/shelly-manager#783.
- Creating or updating a notification channel now refuses a webhook method other than POST, PUT or PATCH and a webhook or Slack URL that is not an absolute http or https URL with a host, with the rule as the 400 reason. Before, such a channel was stored and failed at every delivery. The reason never carries the URL, and a save that leaves the stored config unchanged is not checked again, so a channel stored earlier can still be disabled; it keeps failing at delivery until its method or URL is fixed. Refs ginsys/shelly-manager#784.
- The typed configuration of a stored second-generation device now holds its device name, time and location,
  Wi-Fi station and access point, MQTT, cloud, and switch and input channels, where it held little more than
  `cloud`, so "Create template from this import" no longer makes an empty template for such a device. The
  `raw` part holds the whole imported document, secrets masked, so a typed save loses no setting the form has
  no home for. `conversion_info.warnings` names each top-level key that nothing is read from, and the
  access-point name is never copied into a template. Fixes ginsys/shelly-manager#789.
- Export output paths and downloads are now confined to one export directory, which is always set: `sync.export_base_dir` if set, else `export.output_directory`, else `./data/exports` (created at start-up). The JSON, YAML, SMA and GitOps exports and the generic export route join a relative `output_path` under it and refuse any path that leaves it (absolute, `..` or through a symbolic link) with a `400` that names the directory; the Backup export page now leaves the path empty by default. Three changes to watch for. The server now gives the backup plugin `export.output_directory` when only that key is set, so backups move from `./data/backups` to under it, matching where the restore checks already looked; with no key set, backups stay in `./data/backups` and are downloaded from there. With no key set, any other export recorded earlier outside `./data/exports` can no longer be downloaded (`403`). An absolute `output_path` outside the export directory is refused. Not covered: import paths, and a deployment that sets both keys to different directories. Fixes ginsys/shelly-manager#792.
- A streaming download that fails after its first byte now reaches the client as a failed transfer. The recovery and request-timeout layers used to swallow `http.ErrAbortHandler` and, once the response had started, ended it `200`, so a GitOps export download that lost a file midway (or ran past its request deadline) was saved as a truncated zip with no error. Both layers now let the abort through, the GitOps download aborts on a failure after the first byte, and a failure before the first byte is still the normal `500`. Fixes ginsys/shelly-manager#796.
- A device's generation is now read from its `gen` setting, then from the shape of its firmware, and is `0`
  when neither says, where it was guessed from the firmware alone: Gen2 firmware `1.x` was read as Gen1 and an
  empty firmware string as Gen2. The device page lists `ble` and `ethernet` for generation 2 and later only,
  so a device with no `gen` and Gen2 firmware `1.x` now lists both, and one with no `gen` and no recognisable
  firmware lists neither. The typed validator reads `gen` too. A `gen` of 0 or less is no value, so the firmware decides. Fixes ginsys/shelly-manager#798.
- A template made from a device import, and the common settings of several devices, no longer carry the device's latitude and longitude (`location` and `system.location`), because those belong to one device and a template is shared. The time zone is kept. Fixes ginsys/shelly-manager#800.
- The normalized configuration of a stored Gen2 device now shows its time zone and position, which it
  left empty, and `raw.additional` lists only the top-level keys that nothing is read from (`ble`,
  `modbus`, scripts, `ws`) instead of a second copy of every section. A setting under a section the typed
  form reads in part, such as `sys.cfg_rev`, is no longer listed there and stays in the `raw` part of
  `GET /devices/{id}/config/typed`. The time zone and position now come from the top-level `location`
  of any typed configuration that holds one, Gen1 included, and it wins for each field it sets over
  `system.location`. Fixes ginsys/shelly-manager#801.
- A Gen2 document key such as `switch:007`, `switch:+1` or `switch:01` is no longer taken as a channel,
  a `switch:1` next to `switch:01` no longer gives two relays with the same id, and a channel or
  section key whose value is not an object (`null`, text) is listed as unread instead of read.
  Apply is unchanged for the documents it writes. Fixes ginsys/shelly-manager#802.
- The GitOps export now writes every directory and file through the opened export directory (`os.Root`), as the SMA export does, so a symbolic link swapped in between the path check and the write can no longer send a write outside the export directory. The up-front check stays and still answers a `400` that names the directory. One behaviour change: an output directory, or a directory below it, that is a symbolic link with an absolute target is now refused even when the target lies inside the export directory; a relative link inside it still works. The JSON, YAML and backup exports are covered under ginsys/shelly-manager#815. Fixes ginsys/shelly-manager#804.
- A backup's `output_path` is now checked with the same code as the other exports, which follows symbolic links: a path that leads out of the backup directory through a link is refused with a `400` that names the directory, where before only the path text was checked. The backup directory is created when a backup or a validation first needs it. A relative `output_path` still means the working directory when no export directory is set, and is joined under the export directory when one is. A link swapped in between the check and the backup file's write is refused too (ginsys/shelly-manager#815). Fixes ginsys/shelly-manager#805.
- The common-settings answer no longer offers a list whose elements share nothing but their `id`, or a `null` that every device holds, as common settings. A list is kept only when every element keeps something beyond its `id`, otherwise the whole list is left out, and what is left out is counted in the per-section number of differing values. Known limit: a zero-filled field (`color: 0`) still compares equal to an explicit 0, because a stored document does not tell the two apart. Fixes ginsys/shelly-manager#810.
- A device added by hand keeps the generation it really has. Adding a device with no `gen` in its
  settings used to store Gen 1, so a Gen2 or Gen3 device added from the UI read as Gen1. The server
  now asks the device once at the address in the request and stores the generation it reports;
  a device that does not answer is stored as Gen 1, as before. Updating a device keeps its stored
  `gen` and never adds one. A Gen2 device added by hand before this fix keeps `gen: 1` until a
  settings edit changes `gen` to another value or it is discovered again. Fixes ginsys/shelly-manager#811.
- The live device reads (`GET /devices/{id}/status`, `/energy`, `/config/current` and `/config/current/normalized`) now answer a device that is not the stored one with `409` `CONFLICT` and the identity text, as apply, verify and control already did, instead of a bare `500`. It happens when the device refuses the stored login and the identity read that follows finds another MAC, none stored or none reported. A refused login whose identity read fails in any other way keeps its `502`. Status and energy now share the error answer of the current-config reads, so a refused or unanswered read is logged too. Fixes ginsys/shelly-manager#814.
- The backup file is now created and written through the opened backup directory (`os.Root`), as the other exports are, so a symbolic link swapped in between the path check and the write can no longer send it outside that directory. A failed backup removes the file it had created. A symbolic link with an absolute target in the backup directory is now refused. Not covered: the export history still reads the finished file's size by its path. One change to watch for: the temporary snapshot of the database (`.backup-*`, one full uncompressed copy while the backup runs) is now made in a private directory in the directory of the database's DSN path (beside the live database file for a plain path), on that disk and not beside the backup file; a crash can leave it there, so delete a stray `.backup-*` next to the database by hand. Fixes ginsys/shelly-manager#815.
- The JSON and YAML exports now write their file (plain, gzip or zip) through the opened export directory (`os.Root`), as the SMA and GitOps exports do, so a symbolic link swapped in between the path check and the write can no longer send the write outside the export directory. A link whose target is an absolute path is now refused there too, even one that points inside the directory. A gzip or zip export now closes the archive before it syncs the file and reports a failure of that close (the trailer is written there), so a full disk no longer leaves a truncated archive reported as a success. Refs ginsys/shelly-manager#815.
- `docs/api/openapi.yaml` parses again: the description of the common-settings `400` answer held an unquoted `: ` that made the whole file invalid YAML for a viewer, a client generator or a lint. A test now parses the file, so the next break fails CI. Fixes ginsys/shelly-manager#816.
- Adding a device by hand stores the generation the device reports even when the client hangs up while
  the server asks it: the generation probe now has its own 2 s deadline instead of the request's. A
  device that reports a generation below 1 is stored as Gen 1, as one that does not answer is, and the
  log now says whether the device did not answer, is not a Shelly, or was given an address that is not
  a valid URL host. Fixes ginsys/shelly-manager#820.
- The `go test` lines that the cluster verify and the pull request CI run (`make test-coverage-ci`, `make test-security`, `make test-all` and the per-package and JUnit steps of the test workflow) now pass `-timeout=20m` instead of go's 10 minute default, so a slow runner no longer fails a run inside a test the change did not touch. A real hang still fails, after 20 minutes. Fixes ginsys/shelly-manager#823.
- A SQLite database opened with a `file:` DSN can be backed up again: the snapshot directory and the directory the provider creates no longer start with `file:`, and the database size in the stats follows the file the DSN names, for a `file:` DSN and for any DSN with a `?` query. Fixes ginsys/shelly-manager#837.
- A template or override that sets a plain setting, such as the SNTP time server, now reaches the device's desired configuration; the merge used to drop it with no error. A device shows as pending after its next recompute, which saving its template or override again starts, and apply then writes the time server. An empty value in a later layer does not clear an earlier value. The debug, RPC, sleep, UI-data and BLE settings still cannot be applied and are still refused, as before. Fixes ginsys/shelly-manager#839.
- An override or template that holds only empty, `false` or `0` plain values, such as `{"system":{"sntp":{"server":""}}}`, no longer leaves an empty section in the desired configuration that apply refuses. The route still answers 200 and stores the override as sent; only the desired configuration has no such section. A device that already holds such a section in its desired configuration applies, verifies and is drift-checked as if the section were not there (ginsys/shelly-manager#863). Fixes ginsys/shelly-manager#843.
- The main image build now compares against the commit the images were last built from, not against the last commit of the push. A push of several commits is one Test run on its tip, so a README tip hid an earlier commit that changed `deploy/docker/**` or Go code, and no image was built. The build reads the commit from the date-sha tag of the newest `latest` image of `shelly-provisioner`, which the build makes from the commit it checked out; if that read fails or names no commit in this history, the run builds. A release call with a `version` is unchanged and always builds. Refs ginsys/shelly-manager#846.
- The main image build has pushed no image since 2025-09-11: its change filter was written as a `paths:` and `paths-ignore:` mapping, which the filter action reads as change types, so it matched no file and every build job was skipped. The filter is now one list of the paths an image is built from (`deploy/docker/`, `go.mod`, `go.sum`, `cmd/`, `internal/`, `configs/`, and `ui/` since the manager image builds the web UI) minus Go test files, so a commit that changes none of them still does not build. The old ignore entries outside those paths are dropped, as they could not match. Refs ginsys/shelly-manager#846.
- The manager image's health check never worked: `CMD [...] || exit 1` is read as a shell command that tries to run `[./shelly-manager,`, so the image could not report healthy. The compose file's own check ran `--version`, which only proves the binary starts, so a hung server still read as healthy. Both now ask the server (`/healthz`, on the port in `SHELLY_SERVER_PORT`, 8080 when unset) and read the answer, because that route replies 200 even when the database check fails. Refs ginsys/shelly-manager#846.
- The published manager image now serves the web UI at `/`: the image build compiles `ui/` in a Node stage and copies `ui/dist` into the final image, where it used to show only the minimal landing page. Refs ginsys/shelly-manager#846.
- A provisioner register without a public key that holds a NUL byte in its id, hostname, version, capabilities or metadata is now refused with `400` on every database, before the store is read. On PostgreSQL such an id answered `500`, and on SQLite one that started with `provisioner-` was stored, because the id rule exempts that prefix. An agent row that already holds a NUL in its id (only a hand-made request could have written one on SQLite or MySQL) can no longer register again: the admin removes it. Fixes ginsys/shelly-manager#852.
- An unsaved edit of the device overrides is no longer lost when the page refreshes (after a rollback, Apply, Verify or Refresh). If the saved overrides change under an open edit, the edit is kept, a notice says so, and Save is off until Cancel loads the saved overrides. Fixes ginsys/shelly-manager#855.
- Text typed in the JSON view of the device overrides editor that does not parse yet is no longer lost when the page refreshes: the editor opens in the JSON view with the same text and Save stays off. Cancel, Clear All and moving to another device drop it. Fixes ginsys/shelly-manager#858.
- A device whose stored desired configuration holds a section with nothing set in it, such as `{"system":{"sntp":{}}}`, is no longer refused by apply, and verify and the drift check no longer report that section as a difference. The section is skipped at any depth, and so is one that holds only empty, `false` or `0` plain values; a `false`, `0` or `[]` set on its own field, such as `mqtt.enable`, is still a value. The stored configuration is not rewritten. Fixes ginsys/shelly-manager#863.
- A config that does not load no longer prints the value the decoder refused, on the manager as well as the agent. A secret written in the wrong type in the config file, or in a `SHELLY_` variable, used to reach the start-up error whole; the manager and the agent now print the file's path and one fixed sentence, with no line number and no setting name, and the loader owns it so every start-up path shares it. Check the file's syntax with a YAML linter, compare each setting's type with the sample config, and check each `SHELLY_` variable's type. A file that is missing or unreadable is still named with its cause. Fixes ginsys/shelly-manager#872.
- A provisioner agent whose id holds a `/` can now be approved, rejected, deleted, polled and read on the agents page: the five per-agent routes take the id as everything between their fixed parts, so the slash may be sent as it is or as `%2F`. A new agent id with an empty segment (`//`, or a `/` at either end) or a `.` segment, which the routes cannot reach cleanly, or with `..` anywhere (the request validator refuses `../` in the `agent_id` query value of the discovered-devices route), is refused with `400`; an id with a single inner `/` still registers. Fixes ginsys/shelly-manager#873.

### Security
- The `sslmode` and `connect_timeout` values in `database.options` are now escaped before they join the PostgreSQL DSN, so a value such as `require&password=x` can no longer add a `password` or `user` key next to `database.username` and `database.password`, which the DSN check refuses. Refs ginsys/shelly-manager#590.
- The provisioner agent routes (`/api/v1/provisioner/*`), `POST /api/v1/discover` and
  `POST /api/v1/provisioning/provision` now refuse a request without the admin key when one is set;
  the task poll returned device credentials and Wi-Fi settings to anyone. The kustomize DaemonSet
  gives the agent the key from the `shelly-manager-secrets` Secret through `secretKeyRef`. A route
  table test now fails on any route that answers without the key and is not listed; 29 such routes
  are listed (ginsys/shelly-manager#611). With no admin key configured the routes stay open, as
  before. Rotating the admin key through the API breaks every agent until the Secret is updated
  (ginsys/shelly-manager#610). Fixes ginsys/shelly-manager#607.
- The provisioner agent now has its own key, `security.agent_api_key`
  (`SHELLY_SECURITY_AGENT_API_KEY`). The manager accepts it next to the admin key on the five agent
  routes only: register, poll tasks, update task status, report discovered devices and health. Creating
  tasks and listing agents, tasks or discovered devices still need the admin key, and the agent key
  opens no other route. The kustomize DaemonSet reads it from the `agent-api-key` key of its Secret, so
  the agent no longer holds the admin key, and rotating the admin key through the API no
  longer breaks the agents. The admin key is now compared in constant time on every guard that checks
  it, and the `Bearer ` prefix stays case-sensitive. Migration: set `agent-api-key` in
  `shelly-manager-secrets` to a value different from the admin key, then restart the manager and the
  DaemonSet; until then the agents get 401 when an admin key is set. Fixes ginsys/shelly-manager#610.
- With an admin key set, the 27 `/api/v1` routes that still answered without it now return 401: the device and tag routes, the notification routes (their reads return stored webhook URLs, headers and secrets), the export plugin reads, `GET /api/v1/provisioning/status` and `GET /api/v1/dhcp/reservations`. Only `GET /api/v1/version` and `GET /api/v1/metrics/prometheus` stay open. This is an API behaviour change: scripts and integrations must send `Authorization: Bearer <key>` or `X-API-Key: <key>`, and a deployment with an admin key needs the bundled UI to have that key (a build-time `VITE_ADMIN_KEY`; the dev-only `/app-config.js` does not reach the UI), or its device list and notification pages answer 401. With no admin key configured nothing changes. Fixes ginsys/shelly-manager#611.
- Notification channel credentials are no longer returned by the API. The channel list, the rules list, the
  history list and the create response send `********` in place of the value (of any type) under a webhook
  `url`, `webhook_url`, `headers` or any secret-named key. A channel update that sends `********` for such a key
  keeps the stored value, and a config that repeats a member name is refused with 400. Refs ginsys/shelly-manager#620.
- Each provisioner agent now has its own identity. The manager issues an `agent_token` when an agent
  registers, and the agent sends it as `X-Agent-Token`. Poll tasks, update task status and report
  discovered devices act only for the token's agent: another agent's token is refused with 403, a missing
  or unknown token with 401 (the agent then registers again), and a report completes only its own agent's
  task. The shared agent key alone now opens register and health only; the admin key is unchanged and
  needs no token. Registering an id that is online (seen within 5 minutes) is refused with 409 unless the
  request carries that agent's token or the admin key. Migration: upgrade the manager and the agents
  together. Tokens are kept in memory, so after a manager restart the agents register again by
  themselves, and a restarted agent waits up to 5 minutes until its old record is silent. An agent from
  before this change cannot poll while an admin key is set. Fixes ginsys/shelly-manager#622.
- The manager's agent key now reads its `_FILE` form, `SHELLY_SECURITY_AGENT_API_KEY_FILE`, like the admin key, so Docker secrets users can mount it as a file. Fixes ginsys/shelly-manager#623.
- The start-up schema check no longer quotes the database server's text when it fails. Reading a
  table's columns before the migration could return a driver error (server address, identifiers)
  that the manager printed as it was; it now reports `database operation failed: <kind>` like the
  other connection errors. Fixes ginsys/shelly-manager#624.
- A failed webhook or Slack delivery no longer puts the request URL, which carries the webhook token, into its
  error text. The history `error` field, the failed test-send response and the log lines name the operation and
  the failure class only (for example `webhook request failed: Post timed out`), and the webhook success log
  line no longer logs the URL. History rows written by an earlier version are rewritten at start: the whole old
  text is replaced by its first words and a note (`failed to send webhook: details removed`). Fixes
  ginsys/shelly-manager#652.
- Device configuration text and notification channel configs are now masked and restored by one walker, so the device routes gain what the notification routes learned. Visible changes on the device routes: a key the JSON decoder reads as a secret name (such as `ſecret`) is masked; the stored key is found without regard to case, and two stored keys that differ only in case restore neither; a non-empty list or object under a secret key is masked whole and the mask restores the whole value (a read change: it showed its inner values before); a whole text that is the mask as a JSON string takes the stored text; list entries pair through a secret container; a container that comes back with its members masked one by one (a read from before this change) still restores them on the device routes; the notification routes take such a container as sent. New refusal: a device text that repeats a member name is answered with 400 `device configuration text repeats a member name: send each name once` on create and update. On the notification routes, a list entry with credentials now pairs only with the stored entry that equals it except for them, and an ambiguous pair is refused with 400. Fixes ginsys/shelly-manager#653.
- An empty secret variable or an empty key file no longer wins over the real key. Before, `SHELLY_SECURITY_ADMIN_API_KEY=` (the shipped Compose file always sets it, empty by default) beat its `_FILE` form, so a mounted key file was never read, and it also blanked an admin key set in the config file, which opens every guarded route. An empty variable now falls through to its `_FILE` form, an empty or newline-only key file counts as absent, and with no value the config file key stays. This covers every secret read from the environment: the admin and agent keys, the SMTP password, the OPNsense key and secret, and the provisioner key. Fixes ginsys/shelly-manager#672.
- A secret `_FILE` variable that names a file the process cannot read now stops the manager and the provisioner at start-up, with an error that names the variable and the path. Before, a typo in a mount path was skipped without a message and left the key empty, and an empty admin key opens every guarded route. A deployment with such a broken path, silently ignored until now, will stop starting until the path is fixed. A plain variable with a value still wins before the file is read, and an unset or empty `_FILE` variable is unchanged. Fixes ginsys/shelly-manager#682.
- `GET /devices/{id}/config/history` no longer returns saved device secrets. It showed the configuration before and after each change, and the changed values, in clear, so a stored device password was in an admin-route answer. The three fields now follow the rules of `GET /devices/{id}/config`: a value under a secret key is the mask, also inside a changed or removed section. The stored history rows keep the real values. Refs ginsys/shelly-manager#747.
- Logs no longer carry secrets: the provisioner does not log the default access-point password, the template engine logs the size of a rendered configuration that is not valid JSON instead of its text (which could hold the device password), and the provisioner agent client logs the size of a registration answer it cannot decode instead of the answer (which carries the agent token). Refs ginsys/shelly-manager#768.

## [0.5.4] (2024-01-15)

### 🚀 Major Features Added

#### Export/Import System Integration (Phase 7.2)
- **SMA Format**: New Shelly Management Archive format for complete system backup
  - Compressed JSON with Gzip compression (35% average size reduction)
  - SHA-256 integrity verification and version compatibility
  - Complete system state: devices, templates, configurations, network settings
  - Metadata tracking: export context, system info, audit trails
- **Comprehensive Export APIs**: Complete export system with 21+ endpoints
  - Export preview with record counts and size estimation
  - Multi-format support: SMA, Terraform, Ansible, Kubernetes, Docker Compose, JSON, CSV
  - Scheduled exports with retention policies (daily/weekly/monthly)
  - Export history with pagination, filtering, and statistics
  - Safe download restrictions to prevent path traversal
- **Intelligent Import System**: Advanced import capabilities with validation
  - Import preview with change detection and conflict resolution
  - Dry-run mode for safe operation testing
  - Backup-before-import with automatic rollback capabilities
  - Smart conflict resolution (update, rename, merge, skip strategies)
  - Comprehensive validation: schema, integrity, dependencies
- **Plugin Architecture**: Extensible plugin system for custom formats
  - Built-in plugins: SMA, Terraform, Ansible, Kubernetes, JSON, CSV
  - Dynamic plugin discovery and configuration
  - Plugin health monitoring and testing capabilities
  - Custom plugin development framework

#### Vue.js Frontend Modernization (Phase 8)
- **Modern SPA Architecture**: Vue 3 + TypeScript + Vite foundation
  - Component-based architecture with Composition API
  - Type-safe API layer with automatic OpenAPI generation
  - State management with Pinia stores
  - Progressive Web App (PWA) capabilities
- **Advanced UI Components**: Schema-driven forms and interfaces
  - Dynamic form generation from backend schemas
  - Real-time validation with error highlighting
  - Export/import wizards with step-by-step guidance
  - Drag-and-drop file upload with progress tracking
- **Responsive Design**: Mobile-first responsive interface
  - Touch-friendly interfaces optimized for mobile devices
  - Adaptive layouts for tablet and desktop
  - Progressive enhancement for advanced features
  - WCAG 2.1 AA accessibility compliance
- **Management Interfaces**: Comprehensive management dashboards
  - Export/import operation management with real-time status
  - Plugin configuration with testing capabilities
  - Metrics dashboard with customizable widgets
  - Notification management with channel configuration

#### Real-time Metrics & Monitoring (Phase 7.5)
- **WebSocket Integration**: Real-time metrics streaming
  - Admin-authenticated WebSocket connections with token support
  - Per-IP connection limits and origin restrictions
  - Automatic reconnection with exponential backoff
  - Live dashboard updates with minimal latency
- **Comprehensive Metrics Collection**: System and application metrics
  - System status: CPU, memory, disk usage, uptime
  - Device metrics: online/offline status, performance indicators
  - Export/import statistics: success rates, performance trends
  - Drift detection: configuration drift monitoring and alerts
- **Prometheus Integration**: Production-ready monitoring
  - Prometheus metrics endpoint with standardized metrics
  - HTTP metrics middleware: request counts, durations, response sizes
  - Custom metrics for business logic monitoring
  - Alert integration with notification system

#### Notification System (Phase 7.2.b)
- **Multi-Channel Support**: Flexible notification delivery
  - Email notifications with SMTP configuration and templates
  - Webhook notifications with custom headers and payloads
  - Slack integration with channel selection and formatting
  - Extensible channel architecture for custom integrations
- **Rule Engine**: Intelligent notification triggers
  - Event-based triggers: drift detection, export/import events, system alerts
  - Severity-based filtering (critical, warning, info levels)
  - Rate limiting: per-rule min intervals and hourly limits
  - Schedule support: time-based notification windows
- **Notification Management**: Complete notification lifecycle
  - Channel CRUD operations with testing capabilities
  - Rule configuration with condition builder
  - Delivery history with status tracking and failure analysis
  - Performance monitoring and delivery metrics

### 🔒 Security Enhancements (Phase 7.3)

#### Admin RBAC System
- **Admin API Key Protection**: Secure access control for sensitive operations
  - All export/import operations protected by admin authentication
  - Flexible key configuration: environment variables or secure files
  - Header-based authentication: Bearer tokens or X-API-Key headers
  - Admin key rotation endpoint with audit logging
- **Safe Download System**: Secure file access controls
  - Configurable output directory restrictions
  - Path traversal protection with 403 responses for violations
  - File extension validation and MIME type checking
  - Temporary file cleanup and secure deletion

#### Enhanced Security Features
- **Secrets Management**: Centralized secret resolution system
  - Environment variable support with `*_FILE` suffix patterns
  - Secure file-based secret loading for Kubernetes integration
  - SMTP, OPNSense, and admin key secret management
  - Docker Compose and Kubernetes examples with best practices
- **TLS/Proxy Hardening**: Production deployment security
  - NGINX/Traefik configuration examples with HTTPS enforcement
  - HSTS headers and security header configuration
  - WebSocket timeout configuration and probe snippets
  - Kubernetes liveness and readiness probe integration

### 📊 Operational Excellence (Phase 7.3.c)

#### Observability Improvements
- **Health Check System**: Comprehensive health monitoring
  - Liveness endpoint (`GET /healthz`) for container orchestration
  - Readiness endpoint (`GET /readyz`) with dependency checking
  - Service uptime tracking and metrics integration
  - Database connectivity and plugin health verification
- **Audit and Logging**: Enhanced operational visibility
  - Admin key rotation audit events with detailed logging
  - Export/import operation tracking with complete metadata
  - Performance metrics and system resource monitoring
  - Structured logging with correlation IDs and context

### 🛠️ Developer Experience

#### Testing Infrastructure Enhancements
- **Comprehensive Test Coverage**: Production-ready testing
  - Database manager: 82.8% coverage (29/31 methods tested)
  - Plugin registry: 63.3% coverage improvement
  - Export/import API: Complete endpoint testing with pagination
  - Security vulnerability resolution: Rate limiting, context propagation
- **Advanced Testing Features**: Quality assurance automation
  - Test isolation framework with automatic cleanup
  - Pagination and filtering test harness for API endpoints
  - Export/import preview and summary validation
  - Notification system testing with rate limiting verification

#### Documentation Expansion
- **Comprehensive Documentation**: Complete system documentation
  - Export/Import System guide with usage examples and troubleshooting
  - SMA format specification with technical details
  - UI guide with component documentation and workflows
  - Migration guide with step-by-step upgrade instructions
  - API documentation for all new endpoints with request/response examples

### 📈 Performance & Scalability

#### Export/Import Performance
- **Optimized Operations**: High-performance data processing
  - Streaming export for large datasets with memory efficiency
  - Parallel import processing with configurable worker pools
  - Compression optimization: 35% average file size reduction
  - Progress tracking with cancel capabilities during long operations

#### WebSocket Performance
- **Efficient Real-time Updates**: Scalable WebSocket implementation
  - Connection pooling with per-IP limits
  - Message broadcasting with selective client filtering
  - Automatic cleanup of stale connections
  - Performance monitoring and connection health tracking

### 🔄 Integration & Compatibility

#### Infrastructure as Code
- **Multi-Platform Export**: DevOps integration capabilities
  - Terraform provider generation with resource definitions
  - Ansible playbook creation with task organization
  - Kubernetes ConfigMap and Secret generation
  - Docker Compose file generation for containerized deployments

#### Backward Compatibility
- **Migration Support**: Seamless upgrade path
  - Automatic database migration with rollback capability
  - Configuration migration with validation
  - Legacy API endpoint compatibility during transition
  - Data format migration with integrity verification

### 🔄 Changed

#### API Response Format Standardization
- **Unified Response Wrapper**: All API endpoints now use standardized response format
  ```json
  {
    "success": true|false,
    "data": { ... },
    "error": { "code": "...", "message": "...", "details": ... },
    "meta": { ... },
    "timestamp": "RFC3339",
    "request_id": "..."
  }
  ```
- **Enhanced Error Handling**: Structured error responses with actionable details
- **Pagination Metadata**: Comprehensive pagination information for list endpoints
- **Request Tracing**: Unique request IDs for debugging and audit trails

#### Configuration Schema Updates
- **Security Section**: New security configuration with admin API key management
- **Export/Import Sections**: Dedicated configuration sections for export/import operations
- **Plugin Configuration**: Structured plugin configuration with validation
- **Environment Variable Support**: Enhanced environment variable mapping with `*_FILE` patterns

#### Database Schema Evolution
- **New Tables**: Export/import history, scheduled operations, notification management
- **Performance Indexes**: Optimized indexes for filtering and pagination
- **Migration System**: Automatic migration with rollback capability
- **Data Integrity**: Enhanced constraints and foreign key relationships

### 🚨 Breaking Changes

#### Authentication Requirements
- **Admin API Key Mandatory**: Export/import operations require admin authentication when security is enabled
  ```bash
  # Previously (v0.5.3)
  curl http://localhost:8080/api/v1/export?format=json
  
  # Now (v0.5.4) - requires authentication
  curl -H "Authorization: Bearer <ADMIN_KEY>" \
    http://localhost:8080/api/v1/export?format=json
  ```

#### API Endpoint Changes
- **Export Endpoints**: `/api/v1/export?format=X` → `/api/v1/export` with POST body
- **Response Format**: All endpoints now return standardized wrapper format
- **Error Codes**: Updated error codes with structured error details
- **Pagination**: New pagination format with comprehensive metadata

#### Configuration File Changes
- **New Required Sections**: Security, export, import configuration sections
- **Environment Variables**: Updated variable naming with `SHELLY_` prefix consistency
- **Plugin Configuration**: Plugin settings moved to dedicated sections

### 📋 Migration Guide

#### Immediate Actions Required
1. **Backup Current System**: Create complete backup before upgrading
   ```bash
   tar -czf shelly-manager-backup-$(date +%Y%m%d).tar.gz \
     /var/lib/shelly/ /etc/shelly/ /var/log/shelly/
   ```

2. **Generate Admin API Key**: Create secure admin API key for new features
   ```bash
   openssl rand -hex 32 > /etc/shelly/admin-key.txt
   chmod 600 /etc/shelly/admin-key.txt
   ```

3. **Update Configuration**: Add new configuration sections
   ```yaml
   security:
     admin_api_key_file: "/etc/shelly/admin-key.txt"
   export:
     output_directory: "/var/exports/shelly-manager"
   import:
     temp_directory: "/var/imports/shelly-manager"
   ```

4. **Create Required Directories**: Set up export/import directories
   ```bash
   sudo mkdir -p /var/exports/shelly-manager /var/imports/shelly-manager
   sudo chown shelly-manager:shelly-manager /var/exports/shelly-manager
   sudo chown shelly-manager:shelly-manager /var/imports/shelly-manager
   ```

#### Compatibility Notes
- **Legacy Endpoints**: Old export endpoints remain functional but deprecated
- **Configuration**: Old configuration format supported with warnings
- **Database**: Automatic migration preserves all existing data
- **API Clients**: Update clients to handle new response format and authentication

#### Rollback Procedure
If issues occur during migration:
1. Stop new service: `sudo systemctl stop shelly-manager`
2. Restore binary: `sudo cp /usr/local/bin/shelly-manager.backup /usr/local/bin/shelly-manager`
3. Restore database: `cp /var/lib/shelly/shelly.db.backup /var/lib/shelly/shelly.db`
4. Restore config: `cp -r /etc/shelly.backup/* /etc/shelly/`
5. Start service: `sudo systemctl start shelly-manager`

### 🐛 Fixed

#### Security Vulnerabilities
- **Rate Limiting Bypass**: Fixed rate limiting bypass vulnerability in API endpoints
- **Context Propagation**: Resolved context propagation issues in middleware
- **Hostname Sanitization**: Enhanced hostname validation and sanitization
- **Path Traversal**: Implemented safe download restrictions with directory validation

#### Performance Issues
- **Memory Leaks**: Fixed memory leaks in WebSocket connection handling
- **Database Queries**: Optimized database queries with proper indexing
- **Concurrent Operations**: Improved handling of concurrent export/import operations
- **Resource Cleanup**: Enhanced cleanup of temporary files and connections

#### API Issues
- **Pagination Bugs**: Fixed pagination edge cases with proper bounds checking
- **Error Handling**: Improved error handling with consistent error responses
- **Validation Issues**: Enhanced input validation with comprehensive error messages
- **WebSocket Stability**: Improved WebSocket connection stability and recovery

### 🚀 Performance Improvements

#### Export/Import Optimization
- **Streaming Processing**: Implemented streaming for large dataset operations
- **Compression Efficiency**: Optimized compression algorithms for better performance
- **Parallel Processing**: Added parallel processing for import operations
- **Memory Management**: Reduced memory footprint for large operations

#### WebSocket Performance
- **Connection Pooling**: Optimized WebSocket connection management
- **Message Batching**: Implemented message batching for reduced overhead
- **Automatic Reconnection**: Enhanced reconnection logic with exponential backoff
- **Resource Usage**: Reduced CPU and memory usage for real-time features

#### Database Performance
- **Query Optimization**: Optimized database queries with proper indexing
- **Connection Pooling**: Enhanced database connection pooling
- **Batch Operations**: Implemented batch operations for bulk data processing
- **Migration Performance**: Optimized database migration performance

### 📖 Documentation

#### New Documentation
- **Export/Import System Guide**: Comprehensive guide with usage examples (`docs/export-import-system.md`)
- **SMA Format Specification**: Technical specification for SMA format (`docs/sma-format.md`)
- **UI Component Guide**: Complete UI documentation with responsive design (`docs/ui-guide.md`)
- **Migration Guide**: Step-by-step migration instructions (`docs/migration-guide.md`)
- **API Reference**: Updated API documentation with new endpoints

#### Updated Documentation
- **README**: Enhanced with Export/Import System section and updated capabilities
- **Configuration Guide**: Updated with new security and export/import sections
- **Deployment Guide**: Enhanced with new security considerations and requirements
- **Troubleshooting Guide**: Expanded with export/import and WebSocket troubleshooting

### 🧪 Testing

#### Test Coverage Improvements
- **Database Manager**: Increased coverage to 82.8% (29/31 methods tested)
- **Plugin Registry**: Improved coverage from 0% to 63.3%
- **Export/Import APIs**: Comprehensive endpoint testing with edge cases
- **Security Testing**: Complete security vulnerability testing and resolution

#### New Testing Features
- **Test Isolation Framework**: Comprehensive test isolation with automatic cleanup
- **API Testing Harness**: Advanced API testing with pagination and filtering validation
- **WebSocket Testing**: Real-time feature testing with connection management
- **End-to-End Testing**: Complete workflow testing across all major features

### Added
- Tests: API pagination and filters hardening
  - Devices: pagination meta, beyond-total pages, zero/omitted page_size, non-integer defaults, meta.version asserted.
  - Export/Import history: pagination meta, `plugin` and `success` filters (case-sensitive plugin), bounds/defaults (page=0 → 1, page_size>100 → 20, non-integer values → defaults), unknown plugin returns empty.
  - Statistics endpoints: asserted totals/success/failure and `by_plugin` counts.
- Secrets management (Phase 7.3.a):
  - Centralized secret resolution with `*_FILE` support (`internal/security/secrets`).
  - Env/file overrides for SMTP password, OPNSense API key/secret, admin key, and provisioner API key.
  - Docs: `docs/SECURITY_SECRETS.md` expanded with Compose/K8s and `*_FILE` examples; `.env.example` updated; Compose examples annotated.
- Admin key rotation endpoint:
  - `POST /api/v1/admin/rotate-admin-key` (guarded by current admin key) rotates in-memory key across API/WS/export/import handlers; logs audit event.
- TLS/Proxy hardening (Phase 7.3.b):
  - Expanded `docs/SECURITY_TLS_PROXY.md` with NGINX/Traefik examples, WS timeout annotation, and Kubernetes probe snippets.
- Operational observability (Phase 7.3.c):
  - Liveness `GET /healthz` and readiness `GET /readyz` endpoints.
  - Prometheus HTTP metrics middleware: request totals, durations, and response sizes.
- Notification API enablement (Phase 7.2.b):
  - Standardized responses for channels/rules/test/history.
  - History endpoint with filters (`channel_id`, `status`) and pagination meta.
  - Per‑rule rate limits (`min_interval_minutes`, `max_per_hour`) and `min_severity` matching.
  - API tests for channel CRUD, test endpoint, and history; service tests for rate limiting and severity.
- Export/Import readiness (Phase 7.2.a):
  - Export preview returns `data.preview` + `data.summary` (record_count, estimated_size).
  - Import preview enforces dry‑run + validate‑only and returns `data.preview` + `data.summary` (will_create/update/delete).
  - Tests for export/import preview summaries.
- Metrics documentation (Phase 7.2.c):
  - Documented HTTP endpoints and WebSocket usage with example message types.
  - Added `/metrics/test-alert` to metrics routes.
- Notification emitters integration (Phase 7.2.d):
  - Drift detection notifier hook in configuration service, wired to Notification Service in server startup.
  - Metrics test‑alert notifier hook wired to Notification Service.
- Documentation: added API docs
  - `docs/API_EXPORT_IMPORT.md`
  - `docs/API_NOTIFICATION.md`
  - `docs/METRICS_API.md`

### UI (Phase 8)
- New SPA scaffolding under `ui/` (Vue 3 + TS + Vite) with layout, router, and typed API layer.
- Devices list (pagination/search/sort) and Device detail pages.
- Dev convenience: server serves `/app-config.js` and `make run` auto-injects admin key (when configured) for development.
- Tests: Vitest unit tests for API layer (mocked) and a Playwright smoke test for Devices page.

### Metrics (Phase 7.5 backend)
- WebSocket security for `/metrics/ws` (admin‑key auth via Bearer token or `?token=`) with per‑IP connection limits and origin checks.
- New admin‑protected summary endpoints:
  - `GET /metrics/health`, `/metrics/system`, `/metrics/devices`, `/metrics/drift`, `/metrics/notifications`, `/metrics/resolution`.
- Service uptime seconds added to status/health.
- Docs: updated `docs/METRICS_API.md` with WS security notes and token client example.
- Tests: WS auth negative/positive paths; health endpoint auth.

### Security (Phase 7.3)
- Admin RBAC guard on export/import/schedules/history/statistics (config: `security.admin_api_key`).
- Safe download restriction to `export.output_directory` (403 when outside).
- Persisted export/import history; added history list/detail and statistics endpoints.
- Tests for RBAC, path restriction, and history endpoints.
- TLS/Proxy hardening docs: Added `docs/SECURITY_TLS_PROXY.md` with NGINX/Traefik examples (HTTPS redirect, HSTS, headers).

### Changed
- README: linked to detailed API docs and changelog.

### CI
- Ensured `make test-ci` passes (coverage 43.0%, lint green).

## [0.5.4-alpha] (existing baseline)
- Refer to repository history for prior changes.
