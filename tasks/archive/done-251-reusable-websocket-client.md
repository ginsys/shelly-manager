# Extract Reusable WebSocket Client

**Priority**: HIGH - Foundation Component
**Status**: done
**Effort**: 8 hours (with 1.3x buffer)

## Context

The metrics store contains a comprehensive WebSocket implementation with reconnection, heartbeat, and fallback logic. This implementation is tightly coupled to the metrics feature and cannot be reused for other real-time features (notifications, provisioning status).

This is a **foundation task** that should be completed early as it enables real-time features in Tasks 311 (Notifications), 321 (Provisioning), and 345 (Drift Detection).

**Source**: [Frontend Review](../docs/frontend/frontend-review.md) - Section 7.2 (Code Quality Tasks)
**Architect Review**: Promoted to HIGH priority as foundational component

## Success Criteria

- [x] Generic `useWebSocket` composable created
- [x] Supports typed messages with generics
- [x] Automatic reconnection with exponential backoff
- [x] Heartbeat/ping-pong support
- [x] Connection status tracking
- [x] Event-based message handling
- [x] Metrics store refactored to use composable
- [x] Unit tests for composable (>80% coverage)
- [x] Documentation updated in `docs/frontend/frontend-review.md`

## Implementation

### Step 1: Analyze Existing Implementation

**File**: `ui/src/stores/metrics.ts`

Extract WebSocket logic:
- Connection management
- Reconnection with exponential backoff
- Heartbeat detection
- Message parsing
- Error handling
- Fallback to REST polling

### Step 2: Design Composable Interface

```typescript
interface UseWebSocketOptions<T> {
  url: string | (() => string)
  protocols?: string[]
  autoConnect?: boolean
  reconnect?: boolean
  reconnectAttempts?: number
  reconnectInterval?: number
  heartbeatInterval?: number
  heartbeatMessage?: string | (() => string)
  onMessage?: (data: T) => void
  onOpen?: (event: Event) => void
  onClose?: (event: CloseEvent) => void
  onError?: (event: Event) => void
}

interface UseWebSocketReturn<T> {
  status: Ref<'connecting' | 'open' | 'closing' | 'closed'>
  data: Ref<T | null>
  error: Ref<Event | null>
  connect: () => void
  disconnect: () => void
  send: (data: string | object) => void
  isConnected: ComputedRef<boolean>
}
```

### Step 3: Create Composable

**File**: `ui/src/composables/useWebSocket.ts`

```typescript
export function useWebSocket<T = unknown>(
  options: UseWebSocketOptions<T>
): UseWebSocketReturn<T> {
  const status = ref<WebSocketStatus>('closed')
  const data = ref<T | null>(null)
  const error = ref<Event | null>(null)

  let socket: WebSocket | null = null
  let reconnectCount = 0
  let heartbeatTimer: number | null = null

  function connect() { ... }
  function disconnect() { ... }
  function send(message: string | object) { ... }
  function setupHeartbeat() { ... }
  function handleReconnect() { ... }

  // Auto-connect if enabled
  if (options.autoConnect !== false) {
    onMounted(connect)
  }

  // Cleanup on unmount
  onUnmounted(disconnect)

  return {
    status: readonly(status),
    data: readonly(data),
    error: readonly(error),
    connect,
    disconnect,
    send,
    isConnected: computed(() => status.value === 'open')
  }
}
```

### Step 4: Add Message Type Support

Create typed message handling:

```typescript
interface WebSocketMessage<T> {
  type: string
  data: T
  timestamp: string
}

// Usage
const { data } = useWebSocket<MetricsMessage>({
  url: '/metrics/ws',
  onMessage: (msg) => {
    if (msg.type === 'system') handleSystemMetrics(msg.data)
  }
})
```

### Step 5: Refactor Metrics Store

**File**: `ui/src/stores/metrics.ts`

Replace WebSocket implementation with composable:

```typescript
const { data, status, connect, disconnect } = useWebSocket<MetricsWSMessage>({
  url: () => getMetricsWsUrl(),
  reconnect: true,
  heartbeatInterval: 30000,
  onMessage: handleMetricsMessage
})
```

### Step 6: Add Tests

**File**: `ui/src/composables/__tests__/useWebSocket.test.ts`

Test cases:
- Connection lifecycle
- Reconnection logic with exponential backoff
- Heartbeat functionality
- Message handling with types
- Error handling
- Cleanup on unmount
- Multiple instance support

### Step 7: Document Usage

Create usage examples for:
- Metrics (existing)
- Notifications (future - Task 311)
- Provisioning status (future - Task 321)
- Drift updates (future - Task 345)

## Enables Future Tasks

- **311**: Notification UI Implementation - real-time notification updates
- **321**: Provisioning UI Integration - provisioning status updates
- **345**: Drift Detection UI - real-time drift updates

## Validation

```bash
# Run unit tests
npm run test -- --grep "useWebSocket"

# Run type checking
npm run type-check

# Run metrics E2E tests (verify no regression)
npm run test:e2e -- --grep "metrics"
```

## Documentation

After completing this task, update `docs/frontend/frontend-review.md`:
- Update Section 1.4 to mark WebSocket coupling as resolved
- Document new composable in Appendix
