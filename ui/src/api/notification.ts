import api from './client'
import type { APIResponse } from './types'

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
export async function getChannels(): Promise<NotificationChannel[]> {
  const res = await api.get<APIResponse<{ channels: NotificationChannel[] }>>('/notifications/channels')
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to load notification channels'
    throw new Error(msg)
  }
  return res.data.data?.channels || []
}

export async function getChannel(id: string): Promise<NotificationChannel> {
  const res = await api.get<APIResponse<NotificationChannel>>(`/notifications/channels/${id}`)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Notification channel not found'
    throw new Error(msg)
  }
  return res.data.data
}

export async function createChannel(data: Partial<NotificationChannel>): Promise<NotificationChannel> {
  const res = await api.post<APIResponse<NotificationChannel>>('/notifications/channels', data)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to create notification channel'
    throw new Error(msg)
  }
  return res.data.data
}

export async function updateChannel(id: string, data: Partial<NotificationChannel>): Promise<NotificationChannel> {
  const res = await api.put<APIResponse<NotificationChannel>>(`/notifications/channels/${id}`, data)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to update notification channel'
    throw new Error(msg)
  }
  return res.data.data
}

export async function deleteChannel(id: string): Promise<void> {
  const res = await api.delete<APIResponse<void>>(`/notifications/channels/${id}`)
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to delete notification channel'
    throw new Error(msg)
  }
}

// Rules
export async function getRules(): Promise<NotificationRule[]> {
  const res = await api.get<APIResponse<{ rules: NotificationRule[] }>>('/notifications/rules')
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to load notification rules'
    throw new Error(msg)
  }
  return res.data.data?.rules || []
}

export async function createRule(data: Partial<NotificationRule>): Promise<NotificationRule> {
  const res = await api.post<APIResponse<NotificationRule>>('/notifications/rules', data)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to create notification rule'
    throw new Error(msg)
  }
  return res.data.data
}

export async function deleteRule(id: string): Promise<void> {
  const res = await api.delete<APIResponse<void>>(`/notifications/rules/${id}`)
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to delete notification rule'
    throw new Error(msg)
  }
}

// History
export interface GetHistoryParams {
  page?: number
  limit?: number
}

export async function getHistory(params: GetHistoryParams = {}): Promise<NotificationHistory[]> {
  const res = await api.get<APIResponse<{ history: NotificationHistory[] }>>('/notifications/history', { params })
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to load notification history'
    throw new Error(msg)
  }
  return res.data.data?.history || []
}
