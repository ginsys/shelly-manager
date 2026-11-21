Fixed

- The configuration form showed a list of items that have no `id` (the Gen1 `actions` and the `tariffs`) as a note with no way to add an item, and an item edited there reached the server without its other fields. It now shows such a list as a JSON box. The relay list, whose items carry an `id`, keeps its fields. Refs ginsys/shelly-manager#381.
