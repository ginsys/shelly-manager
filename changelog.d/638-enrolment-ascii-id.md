Changed
- A provisioner agent that registers without a key can no longer create a new row with an id that holds a space or a byte outside printable ASCII (`0x21` to `0x7E`); the register answers `400` and leaves no row. Ids that start with `provisioner-`, the ids of the agents that exist today, may hold any bytes, and a row that already exists keeps registering. Refs ginsys/shelly-manager#638.
