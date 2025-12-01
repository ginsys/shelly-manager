import api from './client'
import type { APIResponse } from './types'

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

export interface GetTasksParams {
  status?: string
  page?: number
  limit?: number
}

export interface BulkProvisionRequest {
  deviceIds: string[]
  config: Record<string, unknown>
}

// Tasks
export async function getTasks(params?: GetTasksParams): Promise<ProvisioningTask[]> {
  const res = await api.get<APIResponse<{ tasks: ProvisioningTask[] }>>('/provisioning/tasks', { params })
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to load provisioning tasks'
    throw new Error(msg)
  }
  return res.data.data?.tasks || []
}

export async function getTask(id: string): Promise<ProvisioningTask> {
  const res = await api.get<APIResponse<ProvisioningTask>>(`/provisioning/tasks/${id}`)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Provisioning task not found'
    throw new Error(msg)
  }
  return res.data.data
}

export async function createTask(data: Partial<ProvisioningTask>): Promise<ProvisioningTask> {
  const res = await api.post<APIResponse<ProvisioningTask>>('/provisioning/tasks', data)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to create provisioning task'
    throw new Error(msg)
  }
  return res.data.data
}

export async function cancelTask(id: string): Promise<void> {
  const res = await api.post<APIResponse<void>>(`/provisioning/tasks/${id}/cancel`)
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to cancel provisioning task'
    throw new Error(msg)
  }
}

// Bulk operations
export async function bulkProvision(request: BulkProvisionRequest): Promise<ProvisioningTask[]> {
  const res = await api.post<APIResponse<{ tasks: ProvisioningTask[] }>>('/provisioning/bulk', request)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to create bulk provisioning tasks'
    throw new Error(msg)
  }
  return res.data.data.tasks || []
}

// Agents
export async function getAgents(): Promise<ProvisioningAgent[]> {
  const res = await api.get<APIResponse<{ agents: ProvisioningAgent[] }>>('/provisioning/agents')
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to load provisioning agents'
    throw new Error(msg)
  }
  return res.data.data?.agents || []
}

export async function getAgentStatus(id: string): Promise<ProvisioningAgent> {
  const res = await api.get<APIResponse<ProvisioningAgent>>(`/provisioning/agents/${id}/status`)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to load agent status'
    throw new Error(msg)
  }
  return res.data.data
}
