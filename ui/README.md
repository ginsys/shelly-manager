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

Dev runtime config (served by `make run`, not loaded by the UI)
- `make run` now sets `SHELLY_DEV_EXPOSE_ADMIN_KEY=1` and the Go server serves `/app-config.js` that injects:
  - `window.__API_BASE__ = '/api/v1'`
  - `window.__ADMIN_KEY__ = '<security.admin_api_key from config>'` when set
- `ui/index.html` does not load `/app-config.js` today, so the Axios client does not pick it up: set `VITE_ADMIN_KEY` in `ui/.env.local` (see "Admin key in a production build" below).
- Safety: the admin key is only exposed when `SHELLY_DEV_EXPOSE_ADMIN_KEY` is set (dev only). Do not enable this in production.

How to run the SPA (development)
- Backend: run `make run` in project root (serves API on :8080 and `/app-config.js`).
- Frontend: run `make ui-dev` (Vite on :5173). The included `vite.config.ts` proxies `/api`, `/metrics`, and `/app-config.js` to :8080, so API calls work without extra env when the server has no admin key; with one, set `VITE_ADMIN_KEY` in `ui/.env.local`. The Metrics dashboard route is available at `/dashboard` (not `/metrics`), to avoid conflict with backend metrics APIs.
- Optional: You can also run directly from `ui/` with `npm run dev`.

Manual fallback (if not using the provided proxy)
- If you choose not to use the Vite proxy, set at runtime in console:
  - `window.__ADMIN_KEY__ = 'your_key'`
  - `window.__API_BASE__ = 'http://localhost:8080/api/v1'`

Admin key in a production build
- When the server has `security.admin_api_key` set, every `/api/v1` route except `GET /api/v1/version` and
  `GET /api/v1/metrics/prometheus` answers 401 without the key (ginsys/shelly-manager#611). The UI sends the key
  only when it has one: `VITE_ADMIN_KEY` at build time (in `ui/.env.local` for `make ui-dev`; in a production build
  the key then sits in the public bundle) or a `window.__ADMIN_KEY__` set by hand in the browser. The Go server's
  dev-only `/app-config.js` exposes the key, but `ui/index.html` does not load that script, so it does not reach the
  UI by itself. A build with no key gets 401 on the device list and on every configuration page. There is no
  supported production way to give the UI its key yet (ginsys/shelly-manager#619). With no admin key on the
  server, nothing changes.

Production-like run
- Build the SPA: `make ui-build` (writes to `ui/dist`).
- The Go server will auto-serve `ui/dist` at `/` if it exists (no separate web server needed). Use `make run` and open `http://localhost:8080`.

## Configuration

### Development

For local development with custom backend configuration, create `.env.development.local`:

```env
VITE_API_URL=http://localhost:8080
VITE_WS_URL=ws://localhost:8080
VITE_ENABLE_DEVTOOLS=true
```

### Production

For production deployments, create `.env.production.local`:

```env
VITE_API_URL=/api
VITE_WS_URL=wss://shelly.example.com
VITE_ENABLE_DEVTOOLS=false
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `VITE_API_URL` | Backend API base URL | `/api` |
| `VITE_WS_URL` | WebSocket server URL | Auto-detected from API URL |
| `VITE_ENABLE_DEVTOOLS` | Enable Vue devtools | `false` |

**Note**: With `make run` and `make ui-dev` the API base needs no `.env` file. A server with an admin key needs `VITE_ADMIN_KEY` set in `ui/.env.local` (the dev `/app-config.js` is not loaded by the UI).

## End-to-End Testing

The project includes comprehensive E2E testing with Playwright covering 810+ test scenarios.

### Prerequisites
- Backend server running on port 8080
- Frontend dev server running on port 5173

### Running E2E Tests

```bash
# Install Playwright browsers (first time only)
npm run test:install

# Run all E2E tests
npm run test:e2e

# Run tests with UI mode (interactive)
npm run test:e2e:ui

# Run specific test file
npm run test:e2e -- tests/e2e/smoke.spec.ts

# Run tests in headed mode (see browser)
npm run test:e2e:headed

# Generate test report
npm run test:e2e:report
```

### Test Configuration

Tests are configured in `playwright.config.ts`:
- **Local development**: Uses `http://localhost:5173` for frontend
- **CI environment**: Uses `http://localhost:5173` with static build
- **Backend API**: Always uses `http://localhost:8080/api/v1`

### CI/CD Integration

E2E tests run automatically in GitHub Actions:
- Builds both backend and frontend
- Starts services with test configuration
- Runs tests across multiple browsers (Chrome, Firefox, Safari)
- Uploads test results and screenshots on failure

### Test Organization

- `tests/e2e/smoke.spec.ts` - Basic application health checks
- `tests/e2e/api-tests.spec.ts` - API integration testing
- `tests/e2e/export-import.spec.ts` - Export/Import workflow testing
- `tests/e2e/performance.spec.ts` - Performance and load testing
- `tests/e2e/cross-browser.spec.ts` - Cross-browser compatibility

## Phase 7.3 UI Work (Chunks)

1) Export History slice (this commit)
- API clients for history/statistics
- Pinia store with pagination/filters
- Shared table/pagination/filter components
- Export History page with plugin/success filters and pagination

Next slices:
 - Import History page and store (this commit)
 - Admin key rotation UI affordance (this commit)
 - Combined statistics page/cards (this commit)
 - Metrics charts via REST polling (this commit) and WS wiring (next)
