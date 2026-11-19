# Changelog fragments

Every change adds one file here instead of editing `CHANGELOG.md`, so two PRs never touch the same
lines. A release turns the fragments into a `CHANGELOG.md` section and deletes them.

- **Name:** `<issue>-<slug>.md`, for example `587-dsn-error-text.md`. `<issue>` is the issue number
  (the PR number if there is no issue); `<slug>` is the branch slug (lowercase letters, digits, dashes).
- **Line 1:** the section: `Added`, `Changed`, `Deprecated`, `Removed`, `Fixed` or `Security`.
- **The rest:** exactly one bullet, as in `CHANGELOG.md`: it starts with `- `, continuation lines are
  indented two spaces, and it ends with `Fixes owner/repo#N.` or `Refs owner/repo#N.`.

```
Fixed
- A malformed database DSN no longer reaches the error text. Fixes ginsys/shelly-manager#587.
```

- **A change that needs no entry** (CI, tests, docs only): label the PR `no-changelog`. The release PR
  (see below) takes the same label. Dependabot and Renovate branches are skipped by themselves.
  The label lifts the need for a fragment, not its format: a fragment the PR adds or edits is
  validated either way.
- **Check a branch:** `make check-changelog-fragment` (against `origin/main`). **Preview a release:**
  `make changelog-assemble`. **Cut one:** `make changelog-assemble VERSION=X.Y.Z`; it keeps every line
  under `## [Unreleased]`, adds the fragment sections after them, heads the block `## [X.Y.Z] - <date>`,
  and removes the fragments.
- Entries written before fragments began stay in `CHANGELOG.md` under `## [Unreleased]` until a release.
