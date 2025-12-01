# Observability (Operations)

## Real-Time Metrics via WebSocket

Shelly Manager provides real-time metrics updates via WebSocket.

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
    "devices": { "total": 25, "online": 23, "offline": 2 },
    "system": { "cpu_percent": 15.2, "memory_mb": 128 }
  }
}
```

### Automatic Failover

The UI falls back to REST polling if WebSocket fails:

- Initial connection timeout: 5 seconds
- Reconnection attempts: 3
- Fallback polling interval: 30 seconds

See also: docs/security/OBSERVABILITY.md for response metadata and log field standards.

