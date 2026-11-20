Changed

- `mise.toml` now pins Node 24 beside Go and golangci-lint, and `mise.lock` carries it, so `mise install` gives a contributor, or a container that installs only the mise tools, what the frontend checks (`make typecheck-check test-vitest`) need. CI keeps its own Node 24 setup and does not install through mise. Refs ginsys/shelly-manager#777.
