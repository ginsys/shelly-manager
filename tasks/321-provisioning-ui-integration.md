# Provisioning UI Integration

**Priority**: MEDIUM - Important Feature
**Status**: not-started
**Effort**: 10-15 hours

## Context

The backend has 8 provisioning agent management endpoints, but there's no frontend integration. Users need to manage provisioning tasks, monitor status, and perform bulk operations from the SPA.

## Success Criteria

- [ ] Expose provisioning agent management with admin permissions
- [ ] Create provisioning UI pages
- [ ] Add task monitoring UI
- [ ] Add bulk operations UI
- [ ] Implement provisioning status dashboard

## Implementation

### Step 1: Create API Client

**File**: `ui/src/api/provisioning.ts`

```typescript
import axios from './axios'

export interface ProvisioningTask {
  id: string
  deviceId: string
  deviceName: string
  status: 'pending' | 'running' | 'completed' | 'failed'
  taskType: 'configure' | 'update' | 'restart'
  config: Record<string, unknown>
  result?: Record<string, unknown>
  error?: string
  createdAt: string
  updatedAt: string
}

export interface ProvisioningAgent {
  id: string
  name: string
  status: 'online' | 'offline' | 'busy'
  version: string
  capabilities: string[]
  lastSeen: string
}

// Tasks
export const getTasks = (params?: { status?: string; page?: number }) =>
  axios.get<ProvisioningTask[]>('/api/v1/provisioning/tasks', { params })
export const getTask = (id: string) => axios.get<ProvisioningTask>(`/api/v1/provisioning/tasks/${id}`)
export const createTask = (data: Partial<ProvisioningTask>) => axios.post('/api/v1/provisioning/tasks', data)
export const cancelTask = (id: string) => axios.post(`/api/v1/provisioning/tasks/${id}/cancel`)

// Bulk operations
export const bulkProvision = (deviceIds: string[], config: Record<string, unknown>) =>
  axios.post('/api/v1/provisioning/bulk', { deviceIds, config })

// Agents
export const getAgents = () => axios.get<ProvisioningAgent[]>('/api/v1/provisioning/agents')
export const getAgentStatus = (id: string) => axios.get(`/api/v1/provisioning/agents/${id}/status`)
```

### Step 2: Create Pinia Store

**File**: `ui/src/stores/provisioning.ts`

Create store for tasks and agents with real-time updates.

### Step 3: Create Pages

- `ui/src/pages/ProvisioningDashboardPage.vue` - Overview with status
- `ui/src/pages/ProvisioningTasksPage.vue` - Task list with filters
- `ui/src/pages/ProvisioningTaskDetailPage.vue` - Task details
- `ui/src/pages/ProvisioningAgentsPage.vue` - Agent management

### Step 4: Add Status Dashboard Component

**File**: `ui/src/components/provisioning/StatusDashboard.vue`

Real-time status of provisioning operations with WebSocket updates.

### Step 5: Add Bulk Operations Component

**File**: `ui/src/components/provisioning/BulkOperations.vue`

UI for selecting devices and applying bulk configurations.

## Validation

```bash
# Run frontend tests
npm run test

# Run E2E tests
npm run test:e2e

# Manual testing with real devices
```

## Backend Status

- 60% Backend implemented
- 0% Frontend implemented

## Dependencies

None - Backend handlers exist

## Risk

Medium - Complex feature with real-time updates
