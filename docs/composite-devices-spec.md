
# Composite Devices Specification (Shelly Manager → Home Assistant via MQTT)

Author: Shelly Manager Team • Version: 1.0 • Status: Draft

---

## 1) Goals & Non-Goals

**Goals**
- Model relations between **multiple Shelly devices** to form **one logical “virtual device”** (e.g., a gate composed of a relay Shelly + a contact sensor Shelly).
- Generate **ready-to-drop** Home Assistant (**static**) MQTT YAML that:
  - Creates the needed **entities** (cover/switch/light/binary_sensor/sensor/etc.).
  - **Groups** them under a single **device card** (via consistent `device.identifiers` + `unique_id`).
  - Works in a **Kubernetes + VLAN** topology (no mDNS/CoAP needs).
- Be **future-proof** across Shelly families (Gen1 topics, Gen2/Plus/Pro JSON-RPC, BLU via bridge, future RPCs).

**Non-Goals**
- Not an HA custom integration. Output is **pure MQTT YAML** (optionally MQTT Discovery in a future phase).
- No GUI/UX spec; this is an engine/data-model + export spec.
- No cloud dependence; assumes **local MQTT broker**.

---

## 2) High-Level Architecture

```
+----------------+           +----------------------+           +-------------------+
| Shelly Manager |  reads    |   Inventory & Graph  |  renders  |  HA MQTT YAML     |
| (CLI/Service)  +----------->  (Physical + Virtual) +----------->  (configuration)  |
+--------+-------+           +---+--------------+----+           +----+--------------+
         |                       |              |                      |
         | autodiscovery/import  |              | mapping/rules        |
         v                       v              v                      v
   Shelly Devices         PhysicalDevice   VirtualDevice           Files per device
   (Gen1/Gen2/BLU/…)      + Capabilities   + Entities              or one fragment
```

- **PhysicalDevice**: a real Shelly (id, family, firmware, capabilities, topics).
- **VirtualDevice**: a logical device composed from multiple PhysicalDevice **capabilities**.
- **Entity**: a Home Assistant entity (cover/switch/sensor/…) that binds **commands** and **state** to one or more PhysicalDevices’ topics/RPCs.
- **Exporter**: outputs **static MQTT YAML**, ensuring all entities share the same `device.identifiers` so HA presents a single device card.

---

## 3) Device Taxonomy & Capability Model

### 3.1 Physical Families
- **Gen1** (classic MQTT topic tree `shellies/<id>/…`): `relay`, `roller`, `input`, `door`, `motion`, `temperature`, `humidity`, `power`, `energy`, LWT (`online`), etc.
- **Gen2/Plus/Pro** (MQTT **JSON-RPC** on `<device-id>/rpc` + telemetry):  
  `Switch.Set/Get`, `Light.Set/Get`, `Cover.*`, `Input.*`, `Device.GetStatus`, `OTA.*`, etc.
- **BLU** (BLE sensors) typically appear via a **gateway/bridge** (e.g., Shelly Plus acting as BLE gateway → publishes to MQTT). Treat the **bridge** as a PhysicalDevice whose capabilities expose bridged sensors.

### 3.2 Canonical Capabilities (normalized)
Each PhysicalDevice is parsed into a set of **Capabilities**, e.g.:
- **Actuation**: `Switch(ch)`, `Cover(ch)`, `Light(ch)`
- **Inputs/Sensors**: `Contact(ch)`, `Motion(ch)`, `Analog(ch)`, `Input(ch)`
- **Telemetry**: `Power(ch)`, `Energy(ch)`, `Temp`, `Humidity`, `Battery`, `RSSI`
- **Meta**: `Availability`, `FirmwareVersion`, `DeviceInfo`

> The capability layer abstracts **how** to talk (Gen1 topics vs Gen2 RPC) behind a stable interface.

```go
type Capability interface {
  Family() Family     // Gen1/Gen2/BLU/Unknown
  Kind()   Kind       // Switch, Contact, Power, ...
  Channels() []int    // per-channel devices
  Command(c Command) error     // e.g. Pulse, Open, Close
  Subscribe(s Subscription)    // bind to topic(s) for state
  Topics() []TopicSpec         // assist YAML rendering
}
```

---

## 4) Virtual Device Composition

### 4.1 VirtualDevice schema
```yaml
id: "front-gate"
name: "Front Gate"
class: "cover.gate"          # semantic profile (see §4.3)
metadata:
  room: "Driveway"
  manufacturer: "Composite"
  model: "Virtual Gate v1"
bindings:
  actuator:      { ref: "shellyplus1-ABC123", channel: 0, mode: "momentary" }
  closed_sensor: { ref: "shellydw2-XYZ",     channel: 0 }
logic:
  state:
    open_when:   "closed_sensor == open"
    closed_when: "closed_sensor == close"
  rules:
    - name: "obstruction"
      when: "actuator.pulse and closed_sensor unchanged for 8s"
      publish: { topic: "site/front-gate/obstruction", payload_on: "ON", payload_off: "OFF" }
export:
  device_identifiers: ["vd-front-gate"]
  unique_id_prefix:  "vd-front-gate"
```

### 4.2 Entities produced
- `cover.front_gate` (commands → actuator; state ← sensor)
- `binary_sensor.gate_closed`
- optional: `binary_sensor.gate_obstruction`, `sensor.rssi`, `sensor.battery`, `sensor.power`, …

### 4.3 Profiles (reusable templates)
- `cover.gate.edge_trigger`: pulse to toggle; state from contact(s).
- `cover.roller.dual_relay`: open/close mapped to two relays; position via timing or endstops.
- `light.multichannel`: combine multiple Switch/Light channels into one virtual device.
- `hvac.radiator.trv`: bind TRV RPC + external sensors.

Profiles define required **bindings** and default **mappings**.

---

## 5) Mapping to MQTT (Gen1 vs Gen2)

### 5.1 Commands
- **Gen1 Switch pulse**  
  Topic: `shellies/<id>/relay/<ch>/command` • Payload: `on` (use device auto-off)
- **Gen2 Switch pulse (JSON-RPC)**  
  Topic: `<device-id>/rpc` • Payload:  
  `{"id":1,"src":"mgr","method":"Switch.Set","params":{"id":<ch>,"on":true}}`

### 5.2 State
- **Gen1 Contact** → `shellies/<id>/door` (or device-specific topic); payload `open/close`.
- **Gen2 Input/Status** → subscribe to telemetry/events or JSON status topic; use `value_template` to select fields when needed.

### 5.3 Availability (LWT)
- **Gen1**: `shellies/<id>/online` → `true/false`  
- **Gen2**: use firmware-specific LWT topic or manager heartbeat as fallback.

The manager normalizes availability into a single binding per entity.

---

## 6) Home Assistant MQTT YAML Export (Static)

### 6.1 Grouping into one **device card**
Every entity **must** have:
- a stable **`unique_id`** (e.g., `<prefix>_<entity_kind>`), and
- the same **`device.identifiers`** array.

### 6.2 Example: Gate (edge trigger relay + contact)
```yaml
mqtt:

  cover:
    - name: "Front Gate"
      unique_id: vd-front-gate_cover
      command_topic: "shellyplus1-ABC123/rpc"
      command_template: >-
        {{ {"id":1,"src":"ha","method":"Switch.Set","params":{"id":0,"on":true}} | tojson }}
      state_topic: "shellies/shellydw2-XYZ/door"
      state_open: "open"
      state_closed: "close"
      optimistic: false
      availability_topic: "shellies/shellyplus1-ABC123/online"
      payload_available: "true"
      payload_not_available: "false"
      device:
        name: "Front Gate"
        identifiers: ["vd-front-gate"]
        manufacturer: "Composite"
        model: "Virtual Gate v1"

  binary_sensor:
    - name: "Gate Closed"
      unique_id: vd-front-gate_closed
      state_topic: "shellies/shellydw2-XYZ/door"
      payload_on: "close"
      payload_off: "open"
      device_class: door
      availability_topic: "shellies/shellydw2-XYZ/online"
      payload_available: "true"
      payload_not_available: "false"
      device:
        name: "Front Gate"
        identifiers: ["vd-front-gate"]

    - name: "Gate Obstruction"
      unique_id: vd-front-gate_obstruction
      state_topic: "site/front-gate/obstruction"
      payload_on: "ON"
      payload_off: "OFF"
      device_class: problem
      device:
        name: "Front Gate"
        identifiers: ["vd-front-gate"]
```

---

## 7) Data Model & Config Files

### 7.1 Inventory (physical devices)
```yaml
devices:
  - id: "shellyplus1-ABC123"
    family: "gen2"
    model: "Shelly Plus 1"
    capabilities:
      - kind: "Switch"
        channel: 0
      - kind: "Availability"
  - id: "shellydw2-XYZ"
    family: "gen1"
    model: "Shelly DW2"
    capabilities:
      - kind: "Contact"
        channel: 0
      - kind: "Battery"
      - kind: "Availability"
```

### 7.2 Virtual devices graph
```yaml
virtual_devices:
  - id: "front-gate"
    name: "Front Gate"
    class: "cover.gate.edge_trigger"
    export:
      device_identifiers: ["vd-front-gate"]
      unique_id_prefix: "vd-front-gate"
    bindings:
      actuator:      { ref: "shellyplus1-ABC123", channel: 0, mode: "momentary" }
      closed_sensor: { ref: "shellydw2-XYZ",     channel: 0 }
    logic:
      state:
        open_when:   "closed_sensor == open"
        closed_when: "closed_sensor == close"
      rules:
        - name: "obstruction"
          when: "actuator.pulse and closed_sensor unchanged for 8s"
          publish: { topic: "site/front-gate/obstruction", payload_on: "ON", payload_off: "OFF" }
```

---

## 8) Exporter: File Layout & GitOps

### 8.1 Output layout options
- **Monolith**: one `configuration.yaml` fragment containing all MQTT sections.
- **Split**: `packages/virtual-devices/<id>.yaml` (recommended). HA “packages” keeps each VD isolated.
- **Kubernetes**: render into a ConfigMap/Secret (values redacted) mounted into HA container.

### 8.2 Idempotency & Determinism
- **Stable sorting** of entities and keys.
- **Stable `unique_id`**: `<prefix>_<entity-kind>[_<channel>]`.
- **Stable `device.identifiers`** (don’t change unless VD id changes).

### 8.3 Secrets & TLS
- MQTT broker URL/creds live outside the generated files (in HA’s main `mqtt:` config or separate secret).
- If using TLS client certs, mount certs/keys separately.

---

## 9) Validation & Error Handling
- Validate required **bindings** for the selected profile/class.
- Validate PhysicalDevice **capabilities** (e.g., if class requires `Contact`, ensure it exists).
- Cross-family checks (topic vs RPC mismatches).
- Lint for **`unique_id` collisions** and **`device.identifiers` reuse** across VDs.

---

## 10) Testing Strategy
1. **Unit tests**: capability mappers; profile compilers; YAML emitter (golden files).
2. **Integration tests**: simulated MQTT broker; publish sample Gen1/Gen2 telemetry; assert HA-compatible states.
3. **End-to-end**: render `packages/virtual-devices/*` and boot HA in a container with those packages mounted.

---

## 11) Migration Plan
- Introduce **Virtual Devices** gradually; keep raw Shelly entities but mark them **hidden** in UI.
- Update automations to reference VDs; deprecate raw entities over time.

---

## 12) Future-Proofing Hooks
- **RPC registry**: externalize RPC specs per family/firmware → update without code changes.
- **Transform mini-DSL**: allow `value_template`-like transforms for new payloads.
- **BLU bridging**: treat BLE as capabilities of gateway device.
- **Position estimation**: time-based covers with recalibration by endstops or power signatures.

---

## 13) Minimal Implementation Plan (phased)

**Phase 1 — Core graph + exporter**
- Build PhysicalDevice mappers (Gen1 topics; Gen2 RPC & telemetry index).
- Implement `cover.gate.edge_trigger` profile.
- Exporter to HA MQTT YAML (packages layout), grouping via `device.identifiers`.

**Phase 2 — Profiles & sensors**
- Add `cover.roller.dual_relay`, `light.multichannel`, `garage.door` (with dual endstops), power/energy sensors.
- Availability normalization & per-entity LWT.

**Phase 3 — Tooling & QA**
- CLI: `shellymgr vd compile --out ./ha/packages`
- Validator/linter and diff tool.
- Golden tests + simulated MQTT ITs.

---

## 14) Worked Example (roller with dual relays, Gen1)

```yaml
mqtt:
  cover:
    - name: "Living Room Shutter"
      unique_id: vd-lr-shutter_cover
      command_topic: "shellies/shelly2.5-AAAA/roller/0/command"
      # OPEN/CLOSE/STOP payloads map directly on Shelly 2.5 roller mode
      state_topic:   "shellies/shelly2.5-AAAA/roller/0"
      value_template: "{{ value_json.state | lower }}"  # 'open'|'close'|'stop'
      availability_topic: "shellies/shelly2.5-AAAA/online"
      payload_available: "true"
      payload_not_available: "false"
      device:
        name: "Living Room Shutter"
        identifiers: ["vd-lr-shutter"]
  sensor:
    - name: "Shutter Power"
      unique_id: vd-lr-shutter_power
      state_topic: "shellies/shelly2.5-AAAA/roller/0/power"
      unit_of_measurement: "W"
      device_class: power
      device:
        name: "Living Room Shutter"
        identifiers: ["vd-lr-shutter"]
```

---

### Deliverables
- **Library**: capability mappers + profiles + compiler + YAML emitter.
- **CLI**: import inventory; compile VDs; validate; render.
- **Docs**: profile reference; binding cookbook; export how-to; troubleshooting.

