Security
- The `sslmode` and `connect_timeout` values in `database.options` are now escaped before they join the PostgreSQL DSN, so a value such as `require&password=x` can no longer add a `password` or `user` key next to `database.username` and `database.password`, which the DSN check refuses. Refs ginsys/shelly-manager#590.
