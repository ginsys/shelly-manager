Fixed
- The typed configuration of a stored second-generation device now holds its device name, time and location,
  Wi-Fi station and access point, MQTT, cloud, and switch and input channels, where it held little more than
  `cloud`, so "Create template from this import" no longer makes an empty template for such a device. The
  settings with no typed home are named in `conversion_info.warnings` and kept in `raw`, secrets masked, and the
  access-point name is never copied into a template. Fixes ginsys/shelly-manager#789.
