# Shelly Manager UI (Phase 8)

This is the new SPA built with Vue 3 + TypeScript + Vite + Quasar. It replaces the legacy HTML UI entirely.

- UI library: Quasar (agreed)
- Charts: ECharts (agreed)
- E2E tests: Playwright (agreed)
- Auth (interim): Bearer admin key (agreed), swapped later for RBAC

Folder structure (planned):
- src/
  - api/ (hand-typed API clients)
    - export.ts, import.ts (history/statistics)
  - pages/ (route views)
    - ExportHistoryPage.vue
  - components/ (shared components)
    - DataTable.vue, PaginationBar.vue, FilterBar.vue
  - stores/ (Pinia stores)
    - export.ts (history + statistics)
  - router/ (Vue Router)
  - styles/ (tokens/theme)

Note: Dependencies are not installed here to avoid network usage in this environment.

Dev runtime config (automatic with make run)
- `make run` now sets `SHELLY_DEV_EXPOSE_ADMIN_KEY=1` and the Go server serves `/app-config.js` that injects:
  - `window.__API_BASE__ = '/api/v1'`
  - `window.__ADMIN_KEY__ = '<security.admin_api_key from config>'` when set
- `ui/index.html` loads `/app-config.js` before the app, so the Axios client picks it up automatically.
- Safety: the admin key is only exposed when `SHELLY_DEV_EXPOSE_ADMIN_KEY` is set (dev only). Do not enable this in production.

Manual fallback (if using Vite dev server on a different port)
- If the SPA runs at another origin (e.g., `:3000`) without proxying `/app-config.js` to `:8080`, set in console:
  - `window.__ADMIN_KEY__ = 'your_key'`
  - `window.__API_BASE__ = 'http://localhost:8080/api/v1'`

## Phase 7.3 UI Work (Chunks)

1) Export History slice (this commit)
- API clients for history/statistics
- Pinia store with pagination/filters
- Shared table/pagination/filter components
- Export History page with plugin/success filters and pagination

Next slices (planned):
 - Import History page and store (this commit)
 - Admin key rotation UI affordance (this commit)
 - Combined statistics page/cards (this commit)
