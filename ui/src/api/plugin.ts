import api from './client'
import type { APIResponse, Metadata } from './types'

// Exact list DTO returned by GET /export/plugins. Mirrors the backend
// `pluginDTO` (internal/api/sync_handlers.go). This is a precise contract, NOT
// a permissive superset: fields the list endpoint does not return (health,
// config_schema, example_config, metadata) are intentionally absent so consumers
// cannot read list-absent data without a type error. The per-plugin detail
// endpoint has a different shape — see PluginDetail.
export interface Plugin {
  name: string
  display_name: string
  description: string
  version: string
  category: string
  capabilities: string[]
  status: PluginStatus
}

// Backend hardcodes available/configured/enabled to true today
// (internal/api/sync_handlers.go); the UI treats a listed plugin as "Registered"
// and does not present configured/enabled as meaningful state. Fields the
// backend never emits (error, last_used) are intentionally not modelled.
export interface PluginStatus {
  available: boolean
  configured: boolean
  enabled: boolean
}

// Detail DTO returned by GET /export/plugins/{name}: `{ info, capabilities }`,
// a different shape from the list item above. Mirrors the Go PluginInfo /
// PluginCapabilities structs (internal/sync/plugin.go).
export interface PluginInfo {
  name: string
  version: string
  description: string
  author: string
  website?: string
  license: string
  supported_formats: string[]
  tags: string[]
  category: string
}

export interface PluginCapabilities {
  supports_incremental: boolean
  supports_scheduling: boolean
  requires_authentication: boolean
  supported_outputs: string[]
  max_data_size: number
  concurrency_level: number
}

export interface PluginDetail {
  info: PluginInfo
  capabilities: PluginCapabilities
}

export interface PluginSchema {
  type: string
  properties: Record<string, PluginSchemaProperty>
  required?: string[]
  title?: string
  description?: string
  examples?: Record<string, any>[]
}

export interface PluginSchemaProperty {
  type: string
  title?: string
  description?: string
  default?: any
  enum?: any[]
  format?: string
  minimum?: number
  maximum?: number
  minLength?: number
  maxLength?: number
  pattern?: string
  items?: PluginSchemaProperty
  properties?: Record<string, PluginSchemaProperty>
  required?: string[]
  examples?: any[]
}

export interface PluginCategory {
  name: string
  display_name: string
  description: string
  plugin_count: number
  plugins: Plugin[]
}

export interface ListPluginsResult {
  plugins: Plugin[]
  categories: PluginCategory[]
  meta?: Metadata
}

/**
 * List all available plugins with optional filtering
 */
export async function listPlugins(category?: string): Promise<ListPluginsResult> {
  const res = await api.get<APIResponse<{ plugins: Plugin[]; categories: PluginCategory[] }>>('/export/plugins', {
    params: { 
      category: category || undefined
    },
  })
  
  if (!res.data.success) {
    throw new Error(res.data.error?.message || 'Failed to load plugins')
  }
  
  return { 
    plugins: res.data.data?.plugins || [], 
    categories: res.data.data?.categories || [],
    meta: res.data.meta 
  }
}

/**
 * Get details for a specific plugin. Returns the detail shape
 * `{ info, capabilities }` (GET /export/plugins/{name}), NOT the list item.
 */
export async function getPlugin(name: string): Promise<PluginDetail> {
  const res = await api.get<APIResponse<PluginDetail>>(`/export/plugins/${name}`)

  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to load plugin details')
  }

  return res.data.data
}

/**
 * Get configuration schema for a plugin
 */
export async function getPluginSchema(name: string): Promise<PluginSchema> {
  const res = await api.get<APIResponse<PluginSchema>>(`/export/plugins/${name}/schema`)
  
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to load plugin schema')
  }
  
  return res.data.data
}

/**
 * Get plugin category display information
 */
export function getPluginCategoryInfo(category: string): { display_name: string; description: string; icon: string } {
  const categories: Record<string, { display_name: string; description: string; icon: string }> = {
    backup: {
      display_name: 'Backup Plugins',
      description: 'Full system backup, selective backup, and incremental backup solutions',
      icon: '💾'
    },
    gitops: {
      display_name: 'GitOps Plugins',
      description: 'Infrastructure as Code exports for Terraform, Ansible, Kubernetes, and more',
      icon: '🚀'
    },
    sync: {
      display_name: 'Sync Plugins',
      description: 'Integration with Home Assistant, OpenHAB, Node-RED, MQTT, and other platforms',
      icon: '🔄'
    },
    custom: {
      display_name: 'Custom Plugins',
      description: 'User-developed plugins with custom schemas and functionality',
      icon: '🔧'
    }
  }
  
  return categories[category] || {
    display_name: category.charAt(0).toUpperCase() + category.slice(1),
    description: `${category} plugins`,
    icon: '📦'
  }
}