# Shelly Manager

Shelly Manager keeps track of Shelly smart-home devices (Gen1 and Gen2) and sets their configuration from one place. It is a Go API server with a Vue web UI, plus a provisioning agent that runs next to new devices.

- Code: read [ARCHITECTURE.md](../ARCHITECTURE.md) before you change code. It holds the code map, the main flows, the rules the code keeps and its boundaries.
- Working rules, build and test commands, commit and pull request rules: [AGENTS.md](../AGENTS.md).
- Branching and the pull request flow: [CONTRIBUTING.md](../CONTRIBUTING.md).
- Work is tracked in [GitHub issues](https://github.com/ginsys/shelly-manager/issues) and milestones.

## Go version

`.go-version` is the one source of the Go version. Run `make check-go-version` to check every file that names it. Run `make upgrade-go-version VERSION=X.Y.Z` to change it everywhere. CI runs the same check.

## Frontend work

[Frontend Review](../docs/frontend/frontend-review.md) holds the frontend architecture, the API coverage definition and the open frontend work. Update it when you finish frontend work. It holds no counts: run `node ui/scripts/api-coverage/cli.mjs` for API coverage.
