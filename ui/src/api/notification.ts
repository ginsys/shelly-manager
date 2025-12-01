import api from './client'
import type { APIResponse, Metadata } from './types'

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

export interface NotificationHistoryItem {
  id: string
  channelId: string
  ruleId: string
  eventType: string
  status: 'sent' | 'failed' | 'pending'
  sentAt: string
  error?: string
}

export interface ListResult<T> {
  items: T[]
  meta?: Metadata
}

// Channels
export async function listChannels(): Promise<ListResult<NotificationChannel>> {
  const res = await api.get<APIResponse<{ channels: NotificationChannel[] }>>('/notifications/channels')
  if (!res.data.success) {
    throw new Error(res.data.error?.message || 'Failed to load channels')
  }
  return { items: res.data.data?.channels || [], meta: res.data.meta }
}

export async function getChannel(id: string): Promise<NotificationChannel> {
  const res = await api.get<APIResponse<NotificationChannel>>(`/notifications/channels/${id}`)
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to load channel')
  }
  return res.data.data
}

export async function createChannel(data: Partial<NotificationChannel>): Promise<NotificationChannel> {
  const res = await api.post<APIResponse<NotificationChannel>>('/notifications/channels', data)
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to create channel')
  }
  return res.data.data
}

export async function updateChannel(id: string, data: Partial<NotificationChannel>): Promise<NotificationChannel> {
  const res = await api.put<APIResponse<NotificationChannel>>(`/notifications/channels/${id}`, data)
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to update channel')
  }
  return res.data.data
}

export async function deleteChannel(id: string): Promise<void> {
  const res = await api.delete<APIResponse<unknown>>(`/notifications/channels/${id}`)
  if (!res.data.success) {
    throw new Error(res.data.error?.message || 'Failed to delete channel')
  }
}

// Rules
export async function listRules(): Promise<ListResult<NotificationRule>> {
  const res = await api.get<APIResponse<{ rules: NotificationRule[] }>>('/notifications/rules')
  if (!res.data.success) {
    throw new Error(res.data.error?.message || 'Failed to load rules')
  }
  return { items: res.data.data?.rules || [], meta: res.data.meta }
}

export async function createRule(data: Partial<NotificationRule>): Promise<NotificationRule> {
  const res = await api.post<APIResponse<NotificationRule>>('/notifications/rules', data)
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to create rule')
  }
  return res.data.data
}

export async function deleteRule(id: string): Promise<void> {
  const res = await api.delete<APIResponse<unknown>>(`/notifications/rules/${id}`)
  if (!res.data.success) {
    throw new Error(res.data.error?.message || 'Failed to delete rule')
  }
}

// History
export interface ListHistoryParams {
  page?: number
  pageSize?: number
  status?: 'sent' | 'failed' | 'pending'
  eventType?: string
}

export async function listHistory(params: ListHistoryParams = {}): Promise<ListResult<NotificationHistoryItem>> {
  const { page = 1, pageSize = 20, status, eventType } = params
  const res = await api.get<APIResponse<{ history: NotificationHistoryItem[] }>>('/notifications/history', {
    params: { page, page_size: pageSize, status, event_type: eventType },
  })
  if (!res.data.success) {
    throw new Error(res.data.error?.message || 'Failed to load notification history')
  }
  return { items: res.data.data?.history || [], meta: res.data.meta }
}

