Fixed
- The MySQL provider now refuses at startup the TLS settings the driver cannot use. `options.tls`
  takes only `false`, `true`, `skip-verify` or `preferred`; `required`, `verify-ca`, `verify-identity`
  and `custom`, and the options `ca`, `cert`, `key` and `serverName`, are refused with an error that
  names the supported modes, and the provider no longer writes those four options into the DSN. The
  MySQL docs and the Kubernetes example now match: the DSN comes from a Secret through
  `SHELLY_DATABASE_DSN`. `configs/shelly-manager.yaml` no longer carries a default `api.url`, which only
  the provisioner reads. Refs ginsys/shelly-manager#705.
