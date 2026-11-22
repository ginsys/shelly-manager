Fixed

- The manager image's health check never worked: `CMD [...] || exit 1` is read as a shell command that tries to run `[./shelly-manager,`, so the image could not report healthy. The compose file's own check ran `--version`, which only proves the binary starts, so a hung server still read as healthy. Both now ask the server (`/healthz`, on the port in `SHELLY_SERVER_PORT`, 8080 when unset) and read the answer, because that route replies 200 even when the database check fails. Refs ginsys/shelly-manager#846.
