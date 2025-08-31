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

