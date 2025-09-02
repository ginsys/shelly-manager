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
- `docs(tasks): update Phase 7 progress and next steps`

Common scopes

- `ui/export`, `ui/import`, `ui/metrics`, `ui/admin`, `ui/stats`
- `api`, `service`, `database`
- `docs/tasks`, `docs/api`, `ci`, `chore`

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

