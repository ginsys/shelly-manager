Added

- The Drift schedules page can now create and edit a schedule and enable or disable it. The form takes a name, a description, a cron preset (hourly, daily at 03:00, weekly on Monday at 03:00) or a custom spec, and one of three device choices: all devices, chosen devices, or a type and status filter. It sends exactly one of the device list and the filter, and keeps Save off until a chosen list or a filter has something in it, so an empty choice never means all devices by accident. The server stays the validator and its refusal is shown as written. Refs ginsys/shelly-manager#279.
