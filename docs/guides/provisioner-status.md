# Provisioner status and Wi-Fi scanning

`shelly-provisioner status` checks that the host can find Shelly devices in access-point mode. This page says
what each line means and what the provisioner does when the Wi-Fi device cannot scan. It covers Linux with
NetworkManager (`nmcli`). On other platforms the provisioner uses a mock interface and reports it as available.

## The interface line

The provisioner reads the first Wi-Fi device from `nmcli device status` and prints its name and NetworkManager
state. Every `nmcli` call of the provisioner runs in the `C.UTF-8` locale, so a translated `nmcli` does not change a verdict, the disconnect, or the current-connection read, and network names with non-ASCII characters stay intact.

| State | Line | Meaning |
|---|---|---|
| `connected`, `disconnected` | `✅ Network Interface: wlo1 (disconnected)` | The radio is on. `disconnected` means joined to no network, the normal state of a host that scans. |
| `unavailable` | `❌ Network Interface: ... is unavailable ...` | The radio may be off or blocked (NetworkManager also reports this state for other causes, such as missing firmware). Check `nmcli radio wifi` and `rfkill list`. |
| `unmanaged` | `❌ Network Interface: ... is unmanaged ...` | NetworkManager ignores the device. Run `nmcli device set <device> managed yes`. |
| no Wi-Fi device | `❌ Network Interface: no Wi-Fi device found ...` | The host has no Wi-Fi device in NetworkManager. |
| `nmcli` fails or takes longer than 10 s | `❌ Network Interface: ... cannot be read ...` | Check that NetworkManager runs. |

The provisioner never turns the radio on and never changes a connection. Any other state (for example
`connecting`) is printed as it is and counts as usable.

## The scanning line

`status` scans only when the interface line is a tick. Then:

- `✅ WiFi Scanning: Working (N networks found)`: the list holds networks.
- `⚠️  WiFi Scanning: No networks listed ...`: the scan returned an empty list. Nothing may be in range, or the
  rescan was refused (see the log).
- `❌ WiFi Scanning: Failed (...)`: `nmcli` could not list networks.

## Access-point discovery

Discovery (`scan-ap`, and the scan inside the provision flow) uses the same state read. For a device that is
unavailable, unmanaged, unreadable or missing, the scan fails with the reason above and does not report
"found 0 networks". A rescan that fails on a usable device is logged as a warning, and the list is still read.

## Enrolling by key

An agent with a seed is identified by its key, not by the shared agent key alone. Without a seed it registers
as before, with an id made from the host name and process id.

1. Make a seed: `shelly-provisioner seed`. It prints 64 hexadecimal characters and nothing else. The seed is a
   secret: put it in your secret store.
2. Give it to the agent in `SHELLY_PROVISIONING_AGENT_SEED`, or in a file named by
   `SHELLY_PROVISIONING_AGENT_SEED_FILE`. There is no flag, because a flag shows in the process list.
3. Read the fingerprint: `shelly-provisioner fingerprint` prints it from the seed alone (it finds the seed where the agent does, and
   needs no manager), as eight groups of four
   (for example `5647-5aa7-5463-474c-0285-df5d-bf2b-cab7`). The agent prints and logs the same line when it
   starts. Its id is `keyed-` and the 32 characters of the fingerprint without hyphens.
4. Approve it in the manager: the agent list shows the fingerprint as `pending`. Compare it with the one from
   step 3, then approve it (`POST /api/v1/provisioner/agents/{id}/approve`), or pre-approve the fingerprint
   before the agent starts (`POST /api/v1/provisioner/enrolments`).

A `pending` agent asks again every 30 seconds, whatever `provisioning.provision_interval` says. A `rejected`
agent logs an error with its fingerprint and asks again at the poll interval, so an operator who changes their
mind needs no restart. After a restart the same seed signs its way in again with no operator step. A new seed is
a new agent: approve its fingerprint again.

Agent settings:

| Setting | Default | Meaning |
|---|---|---|
| `provisioning.agent_seed` (`SHELLY_PROVISIONING_AGENT_SEED`, `_FILE`) | none | The seed. Set it from the environment, not from the config file. |
| `provisioning.enrolment` (`SHELLY_PROVISIONING_ENROLMENT`) | `open` | `open`: an agent with no seed registers as before, and a manager that gives a token without a challenge is accepted. `strict`: the agent refuses to start without a seed, and refuses a manager that gives a token without a signed challenge. |

Use `strict` on the agent when the manager is `strict` too (`security.agent_enrolment`). An older manager with no
enrolment code answers a keyed register with a token at once; in `open` the agent carries on under its new id,
and in `strict` it refuses.

When the manager, the agent or `fingerprint` cannot load a config that exists, it prints the file's path and one
fixed sentence ("the configuration is not valid ...") and nothing from the file, the environment or the decoder,
not even a line number or a setting name. The decoders quote the value they refused, and a secret written in the
config file, or in a `SHELLY_` variable, would be on the screen and in the logs. The sentence covers both sources:
check the file's syntax with a YAML linter, compare each setting's type with `configs/shelly-manager.yaml` or
`configs/shelly-provisioner.yaml` (a linter finds a syntax error, never a value of the wrong type), and check that
each `SHELLY_` variable holds a value of the setting's type. A file that is missing or unreadable, or whose
extension is not a config type, is named, with the system's own reason, as it always was.

A manager that holds too many agents waiting for approval answers `429`: approve or reject some, and the agent
tries again on its next poll.

## Log level

The shipped `configs/shelly-provisioner.yaml` logs at `info`. Set `logging.level: debug` to see the raw
`nmcli` network list and how each of its lines is read.
