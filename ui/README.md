# Shelly Manager UI (Phase 8)

This is the new SPA built with Vue 3 + TypeScript + Vite + Quasar. It replaces the legacy HTML UI entirely.

- UI library: Quasar (agreed)
- Charts: ECharts (agreed)
- E2E tests: Playwright (agreed)
- Auth (interim): Bearer admin key (agreed), swapped later for RBAC

Folder structure (planned):
- src/
  - api/ (hand-typed API clients)
  - pages/ (route views)
  - components/ (shared components)
  - stores/ (Pinia stores)
  - router/ (Vue Router)
  - styles/ (tokens/theme)

Note: Dependencies are not installed here to avoid network usage in this environment.
