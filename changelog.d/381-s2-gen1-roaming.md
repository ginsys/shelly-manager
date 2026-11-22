Changed

- Applying a configuration to a Gen1 device now writes Wi-Fi roaming (`wifi.roam_enable` and `wifi.roam_threshold`) and the Wi-Fi recovery reboot (`wifi.recovery_reboot`, on the Shelly 1, 1PM, 2.5 and Plug/PlugS), and reads them back. A threshold outside -100 to -50 dBm, the recovery reboot on a Shelly i3, and a key that the device does not report (firmware before v1.10.0) are refused before anything is written. Refs ginsys/shelly-manager#381.
