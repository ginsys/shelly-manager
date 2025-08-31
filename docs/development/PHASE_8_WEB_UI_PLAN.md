# Phase 8 — New Web UI (SPA) Plan

Status: Planning (backend prerequisites complete in Phase 7)
Scope: Replace the legacy HTML UI entirely with a new SPA. No parallel maintenance.

## Overview
- Replace legacy UI (web/static) with a modern, secure, testable SPA.
- Backend remains as-is; we will consume Phase 7 APIs (standardized responses, metrics endpoints, export/import, notifications).
- Strict security posture: CSP-friendly (no inline scripts/styles), sanitized rendering, route guards.

## Goals & Non‑Goals
- Goals: faster iteration, eliminate duplication, real-time visibility (metrics WS), better DX, secure-by-default.
- Non‑Goals (Phase 8): full RBAC (comes later), deep branding/theming (can be postponed), compatibility shim for legacy UI.

## Decisions & Questions (to be resolved incrementally)
1) UI Library
   - Decision: Quasar (agreed)
   - Rationale: fastest path to a complete admin console; rich components; good a11y; CSP-friendly builds.
   - Actions: scaffold with Quasar; adopt Quasar tables/forms/dialogs/nav; define tokens + dark mode.

2) Charting
   - Decision: ECharts (agreed)
   - Rationale: richer dashboards, stronger interactions, good for real‑time.

3) Auth transport (interim)
   - Decision: Bearer admin key (agreed)
   - Rationale: matches backend; simple for Phase 8; auth adapter swappable for RBAC later.

4) API typing
   - Decision: Hand-typed TypeScript models (agreed)
   - Rationale: fastest to start; migrate to OpenAPI codegen later if we formalize a spec.

5) E2E Framework
   - Decision: Playwright (agreed)
   - Rationale: parallelism, modern APIs, robust auto-waiting, good cross-browser support.

6) Theming/Design Tokens
   - Decision: Postpone deep visual design (agreed)
   - Rationale: set basic tokens + dark mode now; tune branding after first screens are testable.

7) Repository Layout
   - Decision: Monorepo folder `ui/` (agreed)
   - Rationale: faster integration with backend, shared CI, simpler local dev.

8) Legacy UI Deletion Timing
   - Decision: Delete legacy UI at the start (agreed)
   - Rationale: avoid parallel maintenance and compatibility shims; reduce complexity.

Note: We can postpone 6) to reduce risk and decide after first screens.

## Plan & Steps
1) Remove legacy UI completely
   - Delete `web/static/` and any server static routes referencing it.
   - Confirm no residual references in router or docs.

2) Scaffold SPA foundation (ui/)
   - Vue 3 + TypeScript + Vite + Quasar; ESLint/Prettier; strict CSP index.html.
   - Project structure: `ui/src/{pages,components,stores,api,styles}`; Quasar layout + navigation shell.
   - Routing (Vue Router), State (Pinia), HTTP (Axios); set up Quasar config for CSP compatibility (no inline scripts/styles).

3) API integration layer
   - Centralized client with interceptors (request_id propagation), standardized error surfaces.
   - Types for the key endpoints (devices/config/export/import/notifications/metrics).

4) Auth interim (Phase 8)
   - Bearer admin key (env-configured), route guards for admin-only views.
   - Instrumentation logs; prepare for future RBAC switch.

5) Feature verticals (MVP)
   - Devices: list/detail (pagination/sort/filter), bulk selection (read-only initially).
   - Configuration: typed forms, diff/normalize, import/export preview flows.
   - Notifications: channels/rules CRUD, history with filters/pagination, test send.
   - Metrics: real-time WS dashboard + summaries (`/metrics/health/system/devices/drift/notifications/resolution`).

6) Testing & CI
   - Unit: Vitest + Testing Library; E2E: Playwright/Cypress (decision #5).
   - CI job to build/lint/test SPA; keep `make` parity.

7) Cutover
   - Serve SPA at root; validate flows; document release; legacy UI already removed.

## Milestones & Status
- 8.1 Foundation: SPA scaffold, routing, state, API client (Quasar) — Planned
- 8.2 Auth + API typing: interim admin key, typed endpoints — Planned
- 8.3 Devices + Config: core flows — Planned
- 8.4 Notifications + Metrics: WS + summaries — Planned
- 8.5 QA + a11y + perf + cutover — Planned

## Risks & Mitigations
- Scope creep: time-boxed milestones; focus on MVP flows first.
- CSP regressions: keep zero-inline policy; lint for unsafe patterns.
- Auth change later: isolate auth adapter; keep HTTP client auth swappable.

## Acceptance Criteria (Phase 8)
- Legacy UI removed from repo and server routing.
- SPA builds, lint/tests pass in CI, serves at root.
- Devices/Config/Notifications/Metrics basic flows operational (read APIs, act where safe).
- Security posture: CSP-friendly, admin paths guarded by interim auth.

## Decisions Log
- 2025‑08‑31: UI Library = Quasar (agreed)
- 2025‑08‑31: Charting = ECharts (agreed)
- 2025‑08‑31: Interim Auth = Bearer admin key (agreed)
- 2025‑08‑31: API Typing = Hand-typed TS (agreed)
- 2025‑08‑31: E2E Framework = Playwright (agreed)
- 2025‑08‑31: Visual design = Postpone deep theming (agreed)
- 2025‑08‑31: Repository layout = Monorepo `ui/` (agreed)
- 2025‑08‑31: Legacy UI deletion = Delete at start (agreed)
