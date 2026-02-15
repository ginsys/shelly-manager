# Contributing Guide

Thanks for your interest in contributing! This document summarizes the local workflow, quality gates, and commit hygiene used in this repository.

## Local Dev & Tests

- Install tool versions pinned by `mise.toml` (optional but recommended):
  - `mise install`
- Useful commands (same as CI):
  - `make test-ci` — deps → race+coverage → coverage gate → lint (run before every commit)
  - `make test` / `make test-full` — fast tests / full tests (network included)
  - `make lint-ci` — golangci-lint
  - `make ui-dev` / `make ui-build` — SPA dev/build (see `ui/README.md`)

## Branching & Pull Requests

All changes go through feature branches and pull requests against `develop`.

### Branch Naming

Use the format `<type>/<issue>-<short-description>`:

- `fix/74-partial-updates`
- `feat/80-refresh-button`
- `docs/84-frontend-audit`
- `refactor/91-handler-cleanup`
- `test/95-database-coverage`
- `chore/99-dependency-update`

Types: `fix/`, `feat/`, `docs/`, `refactor/`, `test/`, `chore/`

### Workflow

1. Create a branch from `develop`:
   ```bash
   git checkout develop
   git pull origin develop
   git checkout -b feat/80-refresh-button
   ```
2. Make changes, commit with conventional commits
3. Run `make test-ci` before pushing
4. Push and open a PR against `develop`:
   ```bash
   git push -u origin feat/80-refresh-button
   gh pr create --base develop
   ```
5. PR must pass CI checks before merge
6. After merge, delete the feature branch:
   ```bash
   git checkout develop
   git pull origin develop
   git branch -d feat/80-refresh-button
   ```

Periodically, `develop` is merged to `main` for releases.

### PR Guidelines

- Link the GitHub Issue in the PR body (e.g., `Closes #80`)
- Keep PRs focused — one issue per PR
- Unrelated refactors go into separate PRs

### Handling Dependabot PRs

Use `@dependabot` commands in PR comments:

| Command | Effect |
|---------|--------|
| `@dependabot close` | Close PR, prevent recreation |
| `@dependabot ignore this dependency` | Ignore permanently |
| `@dependabot ignore this major version` | Ignore major updates |
| `@dependabot rebase` | Rebase the PR |
| `@dependabot recreate` | Recreate from scratch |

When deferring an update: comment `@dependabot close` with an explanation of why.

## Commit & PR Guidelines

We use Conventional Commits with concise subjects and precise scopes.

- Subject:
  - Keep ≤72 characters
  - Use a precise scope (examples below)
  - Don’t pack multiple features into a single subject — move detail to the body
- Body:
  - Short bullets for what/why
  - Reference issues if applicable

Examples

- `feat(ui/export): add history slice`
  - API clients, store, page
  - Tests for client
- `test(api): add pagination tests for devices`
- `docs(frontend): update Phase 7 progress and next steps`

Common scopes

- `ui/export`, `ui/import`, `ui/metrics`, `ui/admin`, `ui/stats`
- `api`, `service`, `database`
- `docs/frontend`, `docs/api`, `ci`, `chore`

## PR Requirements

- Describe the change and the rationale (what/why)
- Include tests for new behavior
- Update docs when touching APIs/architecture (docs/ and CHANGELOG)
- Pass `make test-ci` locally
- Keep changes focused; unrelated refactors go into separate PRs

## Security & Secrets

- Never commit secrets. Use `.env` and `configs/*.yaml`; see `.env.example`.
- Admin-only endpoints are protected with the admin key; see `docs/SECURITY_SECRETS.md`.

## UI Notes

- SPA uses Vue 3 + Pinia + Quasar + ECharts; tests use Vitest and Playwright.
- Dev runtime config (`/app-config.js`) is served by the Go server in dev; see `ui/README.md`.

## Questions

Open a draft PR or discussion if you need feedback on direction before investing heavily.

