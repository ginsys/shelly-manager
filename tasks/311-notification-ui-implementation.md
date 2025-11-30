# Notification UI Implementation

**Priority**: MEDIUM - Important Feature
**Status**: not-started
**Effort**: 8-12 hours

## Context

The backend has 7 notification endpoints ready, but there's no frontend integration. Operators need to manage notification channels, rules, and inspect history from the SPA.

## Success Criteria

- [ ] API client created (`ui/src/api/notification.ts`)
- [ ] Pinia stores created for channels, rules, history with pagination/filters
- [ ] Pages created for list/detail/edit views
- [ ] Client unit tests with mocked Axios
- [ ] Operators can manage channels/rules from the SPA
- [ ] Operators can inspect notification history

## Implementation

### Step 1: Create API Client

**File**: `ui/src/api/notification.ts`

```typescript
import axios from './axios'

export interface NotificationChannel {
  id: string
  name: string
  type: 'email' | 'webhook' | 'slack'
  config: Record<string, unknown>
  enabled: boolean
  createdAt: string
  updatedAt: string
}

export interface NotificationRule {
  id: string
  name: string
  channelId: string
  eventTypes: string[]
  filters: Record<string, unknown>
  enabled: boolean
  createdAt: string
  updatedAt: string
}

export interface NotificationHistory {
  id: string
  channelId: string
  ruleId: string
  eventType: string
  status: 'sent' | 'failed' | 'pending'
  sentAt: string
  error?: string
}

// Channels
export const getChannels = () => axios.get<NotificationChannel[]>('/api/v1/notifications/channels')
export const getChannel = (id: string) => axios.get<NotificationChannel>(`/api/v1/notifications/channels/${id}`)
export const createChannel = (data: Partial<NotificationChannel>) => axios.post('/api/v1/notifications/channels', data)
export const updateChannel = (id: string, data: Partial<NotificationChannel>) => axios.put(`/api/v1/notifications/channels/${id}`, data)
export const deleteChannel = (id: string) => axios.delete(`/api/v1/notifications/channels/${id}`)

// Rules
export const getRules = () => axios.get<NotificationRule[]>('/api/v1/notifications/rules')
export const createRule = (data: Partial<NotificationRule>) => axios.post('/api/v1/notifications/rules', data)
export const deleteRule = (id: string) => axios.delete(`/api/v1/notifications/rules/${id}`)

// History
export const getHistory = (params?: { page?: number; limit?: number }) =>
  axios.get<NotificationHistory[]>('/api/v1/notifications/history', { params })
```

### Step 2: Create Pinia Stores

**File**: `ui/src/stores/notifications.ts`

Create stores for channels, rules, and history with pagination support.

### Step 3: Create Pages

- `ui/src/pages/NotificationChannelsPage.vue` - List channels
- `ui/src/pages/NotificationChannelDetailPage.vue` - Channel details/edit
- `ui/src/pages/NotificationRulesPage.vue` - List rules
- `ui/src/pages/NotificationHistoryPage.vue` - History with filters

### Step 4: Add Routes

**File**: `ui/src/router/index.ts`

Add routes for notification pages.

### Step 5: Add Tests

**File**: `ui/src/api/__tests__/notification.test.ts`

Create unit tests with mocked Axios.

## Validation

```bash
# Run frontend tests
npm run test

# Run type checking
npm run type-check

# Manual testing in browser
```

## Backend Endpoints (Already Implemented)

1. `GET /api/v1/notifications/channels`
2. `POST /api/v1/notifications/channels`
3. `GET /api/v1/notifications/channels/:id`
4. `PUT /api/v1/notifications/channels/:id`
5. `DELETE /api/v1/notifications/channels/:id`
6. `GET /api/v1/notifications/rules`
7. `GET /api/v1/notifications/history`

## Dependencies

None - Backend is ready

## Risk

Medium - Frontend-heavy task, requires Vue/Pinia expertise
