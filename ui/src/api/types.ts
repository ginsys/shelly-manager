// Generic API response types matching backend standard format

export interface PaginationMeta {
  page: number
  page_size: number
  total_pages: number
  has_next: boolean
  has_previous: boolean
}

export interface CacheInfo {
  cached: boolean
  cached_at?: string
  expires_at?: string
  ttl_seconds?: number
}

export interface Metadata {
  pagination?: PaginationMeta
  count?: number
  total_count?: number
  version?: string
  cache?: CacheInfo
}

export interface APIError {
  code: string
  message: string
  details?: unknown
}

export interface APIResponse<T> {
  success: boolean
  data?: T
  error?: APIError
  meta?: Metadata
  timestamp: string
  request_id?: string
}

// Domain types (subset)

export interface Device {
  id: number
  ip: string
  mac: string
  type: string
  name: string
  firmware: string
  status: string
  last_seen: string
  settings?: string
  created_at?: string
  updated_at?: string
}

// Device Status Types
export interface WiFiStatus {
  connected: boolean
  ssid?: string
  ip?: string
  rssi?: number
}

export interface SwitchStatus {
  id: number
  output: boolean
  apower?: number
  voltage?: number
  current?: number
  temperature?: number
  source?: string
}

export interface MeterStatus {
  id: number
  power: number
  is_valid: boolean
  total?: number
  total_returned?: number
}

export interface DeviceStatus {
  device_id: number
  ip: string
  temperature?: number
  uptime?: number
  wifi?: WiFiStatus
  switches?: SwitchStatus[]
  meters?: MeterStatus[]
}

// Device Energy Types
export interface DeviceEnergy {
  timestamp: string
  power: number
  total: number
  total_returned: number
  voltage: number
  current: number
  pf?: number
}

// Device Control Types
export interface ControlDeviceRequest {
  action: 'on' | 'off' | 'toggle' | 'reboot'
  params?: {
    channel?: number
  }
}

export interface ControlDeviceResponse {
  status: string
  device_id: number
  action: string
}

// Device Create/Update Types
export interface CreateDeviceRequest {
  ip: string
  mac: string
  type?: string
  name?: string
  firmware?: string
  status?: string
  settings?: string
}

export interface UpdateDeviceRequest {
  ip?: string
  mac?: string
  type?: string
  name?: string
  firmware?: string
  status?: string
  settings?: string
}

