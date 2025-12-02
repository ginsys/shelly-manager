# Documentation Polish

**Priority**: LOW - Enhancement
**Status**: not-started
**Effort**: 2-3 hours

## Context

The documentation could use some polish, particularly the WebSocket section in Observability docs and UI README with dev/prod configuration.

## Success Criteria

- [ ] WebSocket section extended in Observability docs
- [ ] UI README updated with dev/prod config
- [ ] All documentation is accurate and up-to-date
- [ ] Examples are tested and working

## Implementation

### Step 1: Extend WebSocket Documentation

**File**: `docs/operations/observability.md`

Add section:

```markdown
## Real-Time Metrics via WebSocket

Shelly Manager provides real-time metrics updates via WebSocket connection.

### Connection

```javascript
const ws = new WebSocket('ws://localhost:8080/api/v1/metrics/ws')

ws.onmessage = (event) => {
  const metrics = JSON.parse(event.data)
  console.log('Received metrics:', metrics)
}

ws.onerror = (error) => {
  console.error('WebSocket error:', error)
  // Fallback to REST polling
}
```

### Message Format

```json
{
  "type": "metrics_update",
  "timestamp": "2025-01-15T10:30:00Z",
  "data": {
    "devices": {
      "total": 25,
      "online": 23,
      "offline": 2
    },
    "system": {
      "cpu_percent": 15.2,
      "memory_mb": 128
    }
  }
}
```

### Automatic Failover

The UI automatically falls back to REST polling if WebSocket fails:
- Initial connection timeout: 5 seconds
- Reconnection attempts: 3
- Fallback polling interval: 30 seconds
```

### Step 2: Update UI README

**File**: `ui/README.md`

Add development vs production configuration section:

```markdown
## Configuration

### Development

Create `.env.development.local`:

```env
VITE_API_URL=http://localhost:8080
VITE_WS_URL=ws://localhost:8080
VITE_ENABLE_DEVTOOLS=true
```

### Production

Create `.env.production.local`:

```env
VITE_API_URL=/api
VITE_WS_URL=wss://shelly.example.com
VITE_ENABLE_DEVTOOLS=false
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `VITE_API_URL` | Backend API base URL | `/api` |
| `VITE_WS_URL` | WebSocket server URL | Auto-detected |
| `VITE_ENABLE_DEVTOOLS` | Enable Vue devtools | `false` |
```

## Validation

- Documentation renders correctly on GitHub
- All code examples are accurate
- Links work correctly

## Dependencies

None

## Risk

None - Documentation only
