# Advanced Metrics Integration

**Priority**: MEDIUM - Important Feature
**Status**: not-started
**Effort**: 6 hours (with 1.3x buffer: ~8 hours)

## Context

The backend provides 9 advanced metrics endpoints that are not exposed in the frontend. These endpoints enable Prometheus export, metrics collection control, dashboard summaries, and security/resolution metrics.

**Source**: [Frontend Review](../docs/frontend/frontend-review.md) - Section 4.4 (Unused Endpoints)
**Architect Review**: Recommended to close API coverage gap

## Success Criteria

- [ ] API client extended with 9 metrics endpoints
- [ ] Prometheus metrics export link/display
- [ ] Metrics collection enable/disable controls
- [ ] Dashboard summary integration
- [ ] Test alert functionality
- [ ] Security metrics display
- [ ] Resolution metrics display
- [ ] Notification metrics display
- [ ] Unit tests for API functions
- [ ] Documentation updated in `docs/frontend/frontend-review.md`

## Implementation

### Step 1: Extend Metrics API Client

**File**: `ui/src/api/metrics.ts`

Add endpoints:
- `GET /metrics/prometheus` - Prometheus metrics export
- `POST /metrics/enable` - Enable metrics collection
- `POST /metrics/disable` - Disable metrics collection
- `POST /metrics/collect` - Trigger metrics collection
- `GET /metrics/dashboard` - Dashboard summary
- `POST /metrics/test-alert` - Send test alert
- `GET /metrics/notifications` - Notification metrics
- `GET /metrics/resolution` - Resolution metrics
- `GET /metrics/security` - Security metrics

### Step 2: Update MetricsDashboardPage

**File**: `ui/src/pages/MetricsDashboardPage.vue`

Add sections for:
- Collection controls (enable/disable/trigger)
- Security metrics panel
- Resolution metrics panel
- Notification metrics panel
- Test alert button

### Step 3: Add Prometheus Export

Add link or display for Prometheus metrics:
- Link to `/metrics/prometheus` endpoint
- Or inline display of key metrics

### Step 4: Add Admin Controls

Add metrics administration controls:
- Enable/disable collection toggle
- Manual collection trigger
- Test alert button with result display

### Step 5: Add Tests

**File**: `ui/src/api/__tests__/metrics.test.ts`

Test cases for all 9 new endpoints.

## Backend Endpoints (9 total)

| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/metrics/prometheus` | Prometheus export |
| POST | `/metrics/enable` | Enable collection |
| POST | `/metrics/disable` | Disable collection |
| POST | `/metrics/collect` | Trigger collection |
| GET | `/metrics/dashboard` | Dashboard summary |
| POST | `/metrics/test-alert` | Test alert |
| GET | `/metrics/notifications` | Notification metrics |
| GET | `/metrics/resolution` | Resolution metrics |
| GET | `/metrics/security` | Security metrics |

## Related Tasks

- **251**: Reusable WebSocket Client - may use for real-time metrics updates

## Dependencies

- **Optional**: Task 251 (Reusable WebSocket Client) - enables real-time metrics updates

## Validation

```bash
# Run frontend tests
npm run test

# Run type checking
npm run type-check

# Manual testing
# - Test enable/disable collection
# - Verify Prometheus export
# - Test alert functionality
```

## Documentation

After completing this task, update `docs/frontend/frontend-review.md`:
- Update API coverage statistics in Section 4.2
- Move endpoints from "Unused" to "Used" in Section 4.3/4.4
