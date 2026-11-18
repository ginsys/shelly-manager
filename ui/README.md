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

Admin key in production (the key dialog)
- When the server has an admin key set, the UI asks for it. The first request the server refuses with `401 UNAUTHORIZED` opens the "Admin key" dialog by itself, and the header's "Admin key" button opens it at any time.
- The key is kept in `sessionStorage` for this browser tab only. It is gone when the tab closes, it is never written to `localStorage` and it is never part of the built bundle. A reload of the tab keeps it. "Forget key" in the header clears it.
- After you enter a key, the request that was refused is sent again once. If the server refuses that key too, the UI forgets it, shows "That key was not accepted." and opens the dialog again; reload the page or press Refresh after you enter the right key. Cancel closes the dialog and leaves later refused requests quiet until you open it again from the header.
- The metrics WebSocket sends the same key as `?token=`, because a browser cannot set headers on a socket. A socket that the server refuses at the handshake shows no 401 to the page, so open the dialog from the header on the Metrics page if it stays offline.
- Each tab asks once: `sessionStorage` is not shared between tabs. This is an interim route; a login with a session token is tracked in ginsys/shelly-manager#82.
- After you rotate the key on the Admin page, the new key replaces the one kept for the tab.

Dev runtime config (automatic with make run)
- `make run` now sets `SHELLY_DEV_EXPOSE_ADMIN_KEY=1` and the Go server serves `/app-config.js` that injects:
  - `window.__API_BASE__ = '/api/v1'`
  - `window.__ADMIN_KEY__ = '<security.admin_api_key from config>'` when set
- `ui/index.html` loads `/app-config.js` before the app, so the Axios client picks it up automatically.
- Safety: the admin key is only exposed when `SHELLY_DEV_EXPOSE_ADMIN_KEY` is set (dev only). Do not enable this in production; use the key dialog above instead.
- A key you enter in the dialog wins over `VITE_ADMIN_KEY`, and that wins over `window.__ADMIN_KEY__`. `VITE_ADMIN_KEY` is read at build time and ends up in the bundle, so set it for local builds only.

How to run the SPA (development)
- Backend: run `make run` in project root (serves API on :8080 and `/app-config.js`).
- Frontend: run `make ui-dev` (Vite on :5173). The included `vite.config.ts` proxies `/api`, `/metrics`, and `/app-config.js` to :8080, so the app works without extra env. The Metrics dashboard route is available at `/dashboard` (not `/metrics`), to avoid conflict with backend metrics APIs.
- Optional: You can also run directly from `ui/` with `npm run dev`.

Manual fallback (if not using the provided proxy)
- If you choose not to use the Vite proxy, set the API base at runtime in the console (enter the admin key in the key dialog, or set `window.__ADMIN_KEY__` before the app loads):
  - `window.__API_BASE__ = 'http://localhost:8080/api/v1'`

Production-like run
- Build the SPA: `make ui-build` (writes to `ui/dist`).
- The Go server will auto-serve `ui/dist` at `/` if it exists (no separate web server needed). Use `make run` and open `http://localhost:8080`. With an admin key configured, the UI asks for it in the key dialog.

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

**Note**: The default setup with `make run` and `make ui-dev` handles configuration automatically via `/app-config.js` injection. Manual `.env` files are only needed for custom deployments.

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
