import api from './client'
import { errorEnvelopeDetails, errorEnvelopeMessage } from './errorBody'
import type { APIResponse, PaginationMeta } from './types'

// The server sends and takes snake_case objects with numeric ids (the Go models in
// internal/notification). The pages work with the camelCase models below, whose ids are strings.
// The conversion happens here and nowhere else: ginsys/shelly-manager#250 was the pages and the
// server each assuming the other's shape.

export type ChannelType = 'email' | 'webhook' | 'slack'

interface ChannelWire {
  id: number
  name: string
  type: ChannelType
  enabled: boolean
  config: Record<string, unknown> | null
  description?: string
  created_at: string
  updated_at: string
}

interface RuleWire {
  id: number
  name: string
  description?: string
  enabled: boolean
  channel_id: number
  alert_level: string
  min_severity: string
  categories: string[] | null
  created_at: string
  updated_at: string
}

interface HistoryWire {
  id: number
  rule_id: number
  channel_id: number
  trigger_type: string
  status: 'pending' | 'sent' | 'failed' | 'retry'
  sent_at?: string
  error?: string
  created_at: string
}

export interface NotificationChannel {
  id: string
  name: string
  type: ChannelType
  config: Record<string, unknown>
  enabled: boolean
  description: string
  createdAt: string
  updatedAt: string
}

export type NotificationChannelInput = Pick<NotificationChannel, 'name' | 'type' | 'config' | 'enabled'> & {
  description?: string
}

export interface NotificationRule {
  id: string
  name: string
  description: string
  channelId: string
  enabled: boolean
  alertLevel: string
  minSeverity: string
  categories: string[]
  createdAt: string
  updatedAt: string
}

export type NotificationRuleInput = Pick<
  NotificationRule,
  'name' | 'channelId' | 'enabled' | 'alertLevel' | 'minSeverity' | 'categories'
> & { description?: string }

export interface NotificationHistory {
  id: string
  channelId: string
  ruleId: string
  triggerType: string
  status: 'pending' | 'sent' | 'failed' | 'retry'
  sentAt?: string
  createdAt: string
  error?: string
}

function channelFromWire(w: ChannelWire): NotificationChannel {
  return {
    id: String(w.id),
    name: w.name,
    type: w.type,
    config: w.config ?? {},
    enabled: w.enabled,
    description: w.description ?? '',
    createdAt: w.created_at,
    updatedAt: w.updated_at
  }
}

function channelToWire(c: NotificationChannelInput) {
  return { name: c.name, type: c.type, enabled: c.enabled, config: c.config, description: c.description ?? '' }
}

function ruleFromWire(w: RuleWire): NotificationRule {
  return {
    id: String(w.id),
    name: w.name,
    description: w.description ?? '',
    channelId: String(w.channel_id),
    enabled: w.enabled,
    alertLevel: w.alert_level,
    minSeverity: w.min_severity,
    categories: w.categories ?? [],
    createdAt: w.created_at,
    updatedAt: w.updated_at
  }
}

// The server's ids are numbers; a page's are strings. A value that is not a whole number is a bug in the
// caller, and sending it would reach the server as channel 0.
function wireId(id: string): number {
  const n = Number(id)
  if (!Number.isInteger(n) || n <= 0) throw new Error(`Invalid notification id: ${id}`)
  return n
}

function ruleToWire(r: NotificationRuleInput) {
  return {
    name: r.name,
    description: r.description ?? '',
    enabled: r.enabled,
    channel_id: wireId(r.channelId),
    alert_level: r.alertLevel,
    min_severity: r.minSeverity,
    categories: r.categories
  }
}

function historyFromWire(w: HistoryWire): NotificationHistory {
  return {
    id: String(w.id),
    channelId: String(w.channel_id),
    ruleId: String(w.rule_id),
    triggerType: w.trigger_type,
    status: w.status,
    sentAt: w.sent_at,
    createdAt: w.created_at,
    error: w.error
  }
}

// Channels
export async function getChannels(): Promise<NotificationChannel[]> {
  const res = await api.get<APIResponse<{ channels: ChannelWire[] }>>('/notifications/channels')
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to load notification channels'
    throw new Error(msg)
  }
  return (res.data.data?.channels || []).map(channelFromWire)
}

export async function getChannel(id: string): Promise<NotificationChannel> {
  const n = wireId(id)
  const res = await api.get<APIResponse<ChannelWire>>(`/notifications/channels/${n}`)
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Notification channel not found'
    throw new Error(msg)
  }
  return channelFromWire(res.data.data)
}

// A refused config is a 400 that axios throws. The server's reason is in `details` ("invalid channel
// configuration: email config must have at least one recipient") and `message` is a fixed phrase, so the
// reason comes first, as for the backup check.
async function refusal(e: any, fallback: string): Promise<Error> {
  const body = e?.response?.data
  return new Error((await errorEnvelopeDetails(body)) || (await errorEnvelopeMessage(body)) || e?.message || fallback)
}

export async function createChannel(data: NotificationChannelInput): Promise<NotificationChannel> {
  let res
  try {
    res = await api.post<APIResponse<ChannelWire>>('/notifications/channels', channelToWire(data))
  } catch (e: any) {
    throw await refusal(e, 'Failed to create notification channel')
  }
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to create notification channel'
    throw new Error(msg)
  }
  return channelFromWire(res.data.data)
}

// A PUT replaces the channel's name, type, enabled, config and description, and the server answers
// {status: "updated"}, not the channel: callers read the channel again. The config goes as the edit form
// built it: a credential kept is the placeholder the read carried, which the server takes back as the stored
// value (ginsys/shelly-manager#783).
export async function updateChannel(id: string, data: NotificationChannelInput): Promise<void> {
  const n = wireId(id)
  let res
  try {
    res = await api.put<APIResponse<{ status: string }>>(`/notifications/channels/${n}`, channelToWire(data))
  } catch (e: any) {
    throw await refusal(e, 'Failed to update notification channel')
  }
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to update notification channel'
    throw new Error(msg)
  }
}

// Enable or disable a channel: read it, then send it back complete with only `enabled` changed, because
// the PUT replaces every field it carries. The config comes back masked and the server restores the
// stored credentials from the placeholder.
export async function setChannelEnabled(id: string, enabled: boolean): Promise<void> {
  const current = await getChannel(id)
  await updateChannel(id, { ...current, enabled })
}

export async function deleteChannel(id: string): Promise<void> {
  const n = wireId(id)
  const res = await api.delete<APIResponse<void>>(`/notifications/channels/${n}`)
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to delete notification channel'
    throw new Error(msg)
  }
}

// Sends one synthetic notification through the channel. Resolves with the server's message.
export async function testChannel(id: string): Promise<string> {
  const n = wireId(id)
  const res = await api.post<APIResponse<{ status: string; message: string }>>(`/notifications/channels/${n}/test`)
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to send the test notification'
    throw new Error(msg)
  }
  return res.data.data?.message || 'Test notification sent'
}

// Rules
export async function getRules(): Promise<NotificationRule[]> {
  const res = await api.get<APIResponse<{ rules: RuleWire[] }>>('/notifications/rules')
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to load notification rules'
    throw new Error(msg)
  }
  return (res.data.data?.rules || []).map(ruleFromWire)
}

export async function createRule(data: NotificationRuleInput): Promise<NotificationRule> {
  const res = await api.post<APIResponse<RuleWire>>('/notifications/rules', ruleToWire(data))
  if (!res.data.success || !res.data.data) {
    const msg = res.data.error?.message || 'Failed to create notification rule'
    throw new Error(msg)
  }
  return ruleFromWire(res.data.data)
}

export async function deleteRule(id: string): Promise<void> {
  const n = wireId(id)
  const res = await api.delete<APIResponse<void>>(`/notifications/rules/${n}`)
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

export interface HistoryPage {
  items: NotificationHistory[]
  pagination?: PaginationMeta
  total: number
}

// The server pages by limit and offset (page 1 is offset 0) and describes the page in `meta`.
export async function getHistory(params: GetHistoryParams = {}): Promise<HistoryPage> {
  const limit = params.limit ?? 50
  const page = params.page ?? 1
  const res = await api.get<APIResponse<{ history: HistoryWire[] }>>('/notifications/history', {
    params: { limit, offset: (page - 1) * limit }
  })
  if (!res.data.success) {
    const msg = res.data.error?.message || 'Failed to load notification history'
    throw new Error(msg)
  }
  const items = (res.data.data?.history || []).map(historyFromWire)
  return {
    items,
    pagination: res.data.meta?.pagination,
    total: res.data.meta?.total_count ?? items.length
  }
}
