# Provisioner status and Wi-Fi scanning

`shelly-provisioner status` checks that the host can find Shelly devices in access-point mode. This page says
what each line means and what the provisioner does when the Wi-Fi device cannot scan. It covers Linux with
NetworkManager (`nmcli`). On other platforms the provisioner uses a mock interface and reports it as available.

## The interface line

The provisioner reads the first Wi-Fi device from `nmcli device status` and prints its name and NetworkManager
state. Every `nmcli` call of the provisioner runs in the `C` locale, so a translated `nmcli` does not change a verdict, the disconnect, or the current-connection read.

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

## Log level

The shipped `configs/shelly-provisioner.yaml` logs at `info`. Set `logging.level: debug` to see the raw
`nmcli` network list and how each of its lines is read.
