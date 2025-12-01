import api from './client'
import type { APIResponse, Metadata } from './types'

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

export interface ListResult<T> { items: T[]; meta?: Metadata }

// Tasks
export async function listTasks(params: { status?: string; page?: number; pageSize?: number } = {}): Promise<ListResult<ProvisioningTask>> {
  const { status, page = 1, pageSize = 20 } = params
  const res = await api.get<APIResponse<{ tasks: ProvisioningTask[] }>>('/provisioning/tasks', {
    params: { status, page, page_size: pageSize },
  })
  if (!res.data.success) throw new Error(res.data.error?.message || 'Failed to load tasks')
  return { items: res.data.data?.tasks || [], meta: res.data.meta }
}

export async function getTask(id: string): Promise<ProvisioningTask> {
  const res = await api.get<APIResponse<ProvisioningTask>>(`/provisioning/tasks/${id}`)
  if (!res.data.success || !res.data.data) throw new Error(res.data.error?.message || 'Failed to load task')
  return res.data.data
}

export async function createTask(data: Partial<ProvisioningTask>): Promise<ProvisioningTask> {
  const res = await api.post<APIResponse<ProvisioningTask>>('/provisioning/tasks', data)
  if (!res.data.success || !res.data.data) throw new Error(res.data.error?.message || 'Failed to create task')
  return res.data.data
}

export async function cancelTask(id: string): Promise<void> {
  const res = await api.post<APIResponse<unknown>>(`/provisioning/tasks/${id}/cancel`, {})
  if (!res.data.success) throw new Error(res.data.error?.message || 'Failed to cancel task')
}

// Bulk operations
export async function bulkProvision(deviceIds: string[], config: Record<string, unknown>): Promise<{ accepted: number } | undefined> {
  const res = await api.post<APIResponse<{ accepted: number }>>('/provisioning/bulk', { deviceIds, config })
  if (!res.data.success) throw new Error(res.data.error?.message || 'Bulk provision failed')
  return res.data.data
}

// Agents
export async function listAgents(): Promise<ListResult<ProvisioningAgent>> {
  const res = await api.get<APIResponse<{ agents: ProvisioningAgent[] }>>('/provisioning/agents')
  if (!res.data.success) throw new Error(res.data.error?.message || 'Failed to load agents')
  return { items: res.data.data?.agents || [], meta: res.data.meta }
}

export async function getAgentStatus(id: string): Promise<{ status: ProvisioningAgent['status']; lastSeen: string }> {
  const res = await api.get<APIResponse<{ status: ProvisioningAgent['status']; lastSeen: string }>>(`/provisioning/agents/${id}/status`)
  if (!res.data.success || !res.data.data) throw new Error(res.data.error?.message || 'Failed to load agent status')
  return res.data.data
}

