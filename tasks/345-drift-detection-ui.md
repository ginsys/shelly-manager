# Drift Detection UI

**Priority**: MEDIUM - Important Feature
**Status**: not-started
**Effort**: 13 hours (with 1.3x buffer)

## Context

The backend provides 11 drift detection endpoints (7 for schedules, 4 for reporting) that are not exposed in the frontend. Drift detection allows users to monitor configuration changes and detect when device configurations deviate from expected states.

**Source**: [Frontend Review](../docs/frontend/frontend-review.md) - Section 4.4 (Unused Endpoints)

## Success Criteria

- [ ] API client created for drift detection endpoints
- [ ] Drift schedule management (CRUD)
- [ ] Drift reports dashboard
- [ ] Drift trends visualization
- [ ] Resolution workflow
- [ ] Per-device drift report generation
- [ ] Integration with metrics dashboard
- [ ] Unit tests for API functions
- [ ] Documentation updated in `docs/frontend/frontend-review.md`

## Implementation

### Step 1: Create API Client

**File**: `ui/src/api/drift.ts`

Add endpoints:
- `GET /api/v1/config/drift-schedules` - List schedules
- `POST /api/v1/config/drift-schedules` - Create schedule
- `GET /api/v1/config/drift-schedules/{id}` - Get schedule
- `PUT /api/v1/config/drift-schedules/{id}` - Update schedule
- `DELETE /api/v1/config/drift-schedules/{id}` - Delete schedule
- `POST /api/v1/config/drift-schedules/{id}/toggle` - Enable/disable
- `GET /api/v1/config/drift-schedules/{id}/runs` - Get run history
- `GET /api/v1/config/drift-reports` - Get all reports
- `GET /api/v1/config/drift-trends` - Get trends
- `POST /api/v1/config/drift-trends/{id}/resolve` - Mark resolved
- `POST /api/v1/devices/{id}/drift-report` - Generate device report

### Step 2: Create Pinia Store

**File**: `ui/src/stores/drift.ts`

State management for:
- Drift schedules list
- Drift reports
- Trend data for charts
- Resolution status

### Step 3: Create Pages

**Files**:
- `ui/src/pages/DriftSchedulesPage.vue` - Schedule management
- `ui/src/pages/DriftReportsPage.vue` - Reports dashboard
- `ui/src/pages/DriftTrendsPage.vue` - Trend visualization

### Step 4: Create Components

**Files**:
- `ui/src/components/drift/ScheduleForm.vue` - Create/edit schedule
- `ui/src/components/drift/ScheduleList.vue` - Schedule table
- `ui/src/components/drift/DriftReport.vue` - Display single report
- `ui/src/components/drift/TrendChart.vue` - Trend visualization (ECharts)
- `ui/src/components/drift/ResolutionDialog.vue` - Mark as resolved

### Step 5: Add Routes and Navigation

**File**: `ui/src/main.ts`

Add routes:
- `/drift/schedules` - Schedule management
- `/drift/reports` - Reports dashboard
- `/drift/trends` - Trend visualization

Add to navigation menu under Metrics or as new "Drift Detection" item.

### Step 6: Integration

- Add drift status to DeviceDetailPage
- Add drift widget to MetricsDashboardPage
- Link from device config pages

## Backend Endpoints (11 total)

### Schedule Endpoints (7)
| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/api/v1/config/drift-schedules` | List schedules |
| POST | `/api/v1/config/drift-schedules` | Create schedule |
| GET | `/api/v1/config/drift-schedules/{id}` | Get schedule |
| PUT | `/api/v1/config/drift-schedules/{id}` | Update schedule |
| DELETE | `/api/v1/config/drift-schedules/{id}` | Delete schedule |
| POST | `/api/v1/config/drift-schedules/{id}/toggle` | Toggle enabled |
| GET | `/api/v1/config/drift-schedules/{id}/runs` | Run history |

### Reporting Endpoints (4)
| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/api/v1/config/drift-reports` | Get all reports |
| GET | `/api/v1/config/drift-trends` | Get trends |
| POST | `/api/v1/config/drift-trends/{id}/resolve` | Mark resolved |
| POST | `/api/v1/devices/{id}/drift-report` | Generate report |

## Related Tasks

- **342**: Device Configuration UI - shows drift per device
- **346**: Bulk Operations UI - bulk drift detection
- **251**: Reusable WebSocket Client - real-time drift updates

## Dependencies

- **Optional**: Task 251 (Reusable WebSocket Client) - enables real-time drift updates

## Validation

```bash
# Run frontend tests
npm run test

# Run E2E tests
npm run test:e2e -- --grep "drift"

# Type checking
npm run type-check
```

## Documentation

After completing this task, update `docs/frontend/frontend-review.md`:
- Update API coverage statistics in Section 4.2
- Move endpoints from "Unused" to "Used" in Section 4.3/4.4
