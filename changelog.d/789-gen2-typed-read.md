Fixed
- The typed configuration of a stored second-generation device now holds its device name, time and location,
  Wi-Fi station and access point, MQTT, cloud, and switch and input channels, where it held little more than
  `cloud`, so "Create template from this import" no longer makes an empty template for such a device. The
  `raw` part holds the whole imported document, secrets masked, so a typed save loses no setting the form has
  no home for. `conversion_info.warnings` names each top-level key that nothing is read from, and the
  access-point name is never copied into a template. Fixes ginsys/shelly-manager#789.
