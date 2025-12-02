# Reusable WebSocket Client

**Priority**: HIGH
**Status**: completed
**Effort**: 8 hours
**Completed**: 2025-12-02

## Context

Metrics store was tightly coupled to WebSocket logic. A reusable WebSocket composable was introduced to standardize connection management across features (metrics, notifications, provisioning status) and support reconnection and heartbeat.

## Success Criteria

- [x] Generic composable (`useWebSocket`) with typed messages
- [x] Auto-reconnect with backoff and jitter
- [x] Heartbeat/keepalive support
- [x] Connection status and error exposure
- [x] Clean lifecycle (mount/unmount) management
- [x] Adopted by metrics feature

## Implementation

- `ui/src/composables/useWebSocket.ts`
  - `status`, `data`, `error` refs; `connect`, `disconnect`, and `send`
  - Auto-reconnect with exponential backoff and jitter (cap 30s)
  - Heartbeat interval and message hook
  - Type-friendly parsing (JSON first; fallback to raw)
  - Clean teardown on component unmount

## Validation

- Manual verification on metrics dashboard (connectivity, reconnect, heartbeat)
- Unit usage verified by integrating in store/components

```bash
make test-ci
```

## Notes

This composable is now the foundation for future real-time features (notifications UI, provisioning progress, drift detectors).
