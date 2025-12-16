# Fix Dashboard "Resource Not Found" Error

**Priority**: HIGH
**Status**: not-started
**Effort**: 3 hours

## Context
User feedback: "on http://localhost:8080/dashboard I dont get any data, but a big red box with The requested resource was not found"

WebSocket connection establishes successfully (logs show client connect/disconnect), but frontend doesn't properly handle the incoming data.

## Evidence
```
timestamp=... level=INFO msg="New WebSocket client connected" component=websocket clients=1
timestamp=... level=INFO msg="WebSocket client disconnected" component=websocket clients=0
```

Backend WebSocket works, but frontend displays error instead of metrics.

## Success Criteria
- [ ] Debug WebSocket data binding on dashboard page
- [ ] Fix "resource not found" error display
- [ ] Verify real-time metrics display correctly
- [ ] Test WebSocket reconnection on disconnect
- [ ] Add unit tests for WebSocket data handling
- [ ] Run `npm run test` to ensure no regressions

## Files to Investigate
- `web/src/views/DashboardPage.vue`
- `web/src/composables/useWebSocket.ts` (or similar)
- `web/src/stores/` (metrics store)
- `web/src/router/` (routing configuration)

## Validation
```bash
# Navigate to http://localhost:8080/dashboard
# Verify metrics display without errors
# Test WebSocket reconnection after disconnect

npm run test
npm run build
```
