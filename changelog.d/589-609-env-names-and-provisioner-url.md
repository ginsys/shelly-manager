Fixed
- The Dockerfiles, compose files, test scripts and docs now name only the environment variables the
  app reads. `GIN_MODE` and `SHELLY_TEST_MODE_FAST_HEALTH` are gone, and the bare `DATABASE_PROVIDER`,
  `DATABASE_DSN`, `POSTGRES_*`, `MYSQL_*`, `SHELLY_PROVISIONING_AUTO` and `SHELLY_HTTP_PORT` became
  `SHELLY_` plus the config key. A test fails on an unread name in those files. The smoke test now
  starts its backend on the port it checks. The provisioner's `api.url` has no default any more, so
  `shelly-provisioner agent` with no URL stops with "API URL required" instead of dialling
  `http://0.0.0.0:8080`. Fixes ginsys/shelly-manager#589 and ginsys/shelly-manager#609.
