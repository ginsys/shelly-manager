import api from './client'
import type { APIResponse } from './types'

export interface MetricsStatus { enabled: boolean; last_collection_time?: string; uptime_seconds?: number }

export async function getMetricsStatus(): Promise<MetricsStatus> {
  const res = await api.get<APIResponse<MetricsStatus>>('/metrics/status')
  if (!res.data.success || !res.data.data) throw new Error(res.data.error?.message || 'Failed to load metrics status')
  return res.data.data
}

export async function getMetricsHealth(): Promise<any> {
  const res = await api.get<APIResponse<any>>('/metrics/health')
  if (!res.data.success || !res.data.data) throw new Error(res.data.error?.message || 'Failed to load health')
  return res.data.data
}

export function openMetricsWebSocket(onMessage: (data:any)=>void): WebSocket {
  const base = (window as any).__API_BASE__ || '/api/v1'
  const loc = window.location
  const proto = loc.protocol === 'https:' ? 'wss' : 'ws'
  const token = (window as any).__ADMIN_KEY__
  const url = `${proto}://${loc.host}${base.replace('/api/v1','')}/metrics/ws${token?`?token=${encodeURIComponent(token)}`:''}`
  const ws = new WebSocket(url)
  ws.onmessage = (ev) => {
    try { onMessage(JSON.parse(ev.data)) } catch { /* ignore */ }
  }
  return ws
}

