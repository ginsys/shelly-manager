Changed
- A change now records its release note as one file, `changelog.d/<issue>-<slug>.md`, instead of an
  entry in `CHANGELOG.md`, so two open PRs no longer conflict on the `[Unreleased]` section when one
  merges. The `changelog-fragment` CI check fails a PR that adds no fragment or adds a line to
  `CHANGELOG.md`; the PR label `no-changelog` (CI, tests, docs-only changes, the release PR) and
  Dependabot or Renovate branches are skipped. The Release workflow cuts a release: it keeps every
  line under `## [Unreleased]`, adds the fragment sections after them, and removes the fragments. The
  entries already under `[Unreleased]` stay there until a release. The `merge=union` rule for
  `CHANGELOG.md` and its section in `CONTRIBUTING.md` are gone. Fixes ginsys/shelly-manager#600.
