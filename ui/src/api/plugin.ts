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

export interface PluginConfig {
  plugin_name: string
  config: Record<string, any>
  enabled: boolean
  metadata?: Record<string, any>
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

export interface PluginTestResult {
  success: boolean
  duration_ms?: number
  message?: string
  details?: Record<string, any>
  errors?: string[]
  warnings?: string[]
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
 * Test a plugin configuration
 */
export async function testPlugin(name: string, config?: Record<string, any>): Promise<PluginTestResult> {
  const res = await api.post<APIResponse<PluginTestResult>>(`/export/plugins/${name}/test`, {
    config: config || {}
  })
  
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to test plugin')
  }
  
  return res.data.data
}

/**
 * Get plugin configuration
 */
export async function getPluginConfig(name: string): Promise<PluginConfig> {
  const res = await api.get<APIResponse<PluginConfig>>(`/export/plugins/${name}/config`)
  
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to load plugin configuration')
  }
  
  return res.data.data
}

/**
 * Update plugin configuration
 */
export async function updatePluginConfig(name: string, config: Record<string, any>, enabled: boolean = true): Promise<PluginConfig> {
  const res = await api.put<APIResponse<PluginConfig>>(`/export/plugins/${name}/config`, {
    config,
    enabled
  })
  
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.error?.message || 'Failed to update plugin configuration')
  }
  
  return res.data.data
}

/**
 * Validate plugin configuration against schema
 */
export function validatePluginConfig(config: Record<string, any>, schema: PluginSchema): string[] {
  const errors: string[] = []
  
  if (!schema.properties) {
    return errors
  }
  
  // Check required fields
  if (schema.required) {
    for (const field of schema.required) {
      if (!(field in config) || config[field] === null || config[field] === undefined || config[field] === '') {
        errors.push(`Field '${field}' is required`)
      }
    }
  }
  
  // Validate field types and constraints
  for (const [field, property] of Object.entries(schema.properties)) {
    if (!(field in config)) continue
    
    const value = config[field]
    const fieldErrors = validateSchemaProperty(value, property, field)
    errors.push(...fieldErrors)
  }
  
  return errors
}

/**
 * Validate a single schema property
 */
function validateSchemaProperty(value: any, property: PluginSchemaProperty, fieldName: string): string[] {
  const errors: string[] = []
  
  if (value === null || value === undefined) {
    return errors // Skip validation for null/undefined (handled by required check)
  }
  
  // Type validation
  switch (property.type) {
    case 'string':
      if (typeof value !== 'string') {
        errors.push(`Field '${fieldName}' must be a string`)
        return errors
      }
      
      if (property.minLength && value.length < property.minLength) {
        errors.push(`Field '${fieldName}' must be at least ${property.minLength} characters`)
      }
      
      if (property.maxLength && value.length > property.maxLength) {
        errors.push(`Field '${fieldName}' must be at most ${property.maxLength} characters`)
      }
      
      if (property.pattern && !new RegExp(property.pattern).test(value)) {
        errors.push(`Field '${fieldName}' format is invalid`)
      }
      
      if (property.enum && !property.enum.includes(value)) {
        errors.push(`Field '${fieldName}' must be one of: ${property.enum.join(', ')}`)
      }
      break
      
    case 'number':
    case 'integer':
      if (typeof value !== 'number') {
        errors.push(`Field '${fieldName}' must be a number`)
        return errors
      }
      
      if (property.type === 'integer' && !Number.isInteger(value)) {
        errors.push(`Field '${fieldName}' must be an integer`)
      }
      
      if (property.minimum !== undefined && value < property.minimum) {
        errors.push(`Field '${fieldName}' must be at least ${property.minimum}`)
      }
      
      if (property.maximum !== undefined && value > property.maximum) {
        errors.push(`Field '${fieldName}' must be at most ${property.maximum}`)
      }
      break
      
    case 'boolean':
      if (typeof value !== 'boolean') {
        errors.push(`Field '${fieldName}' must be a boolean`)
      }
      break
      
    case 'array':
      if (!Array.isArray(value)) {
        errors.push(`Field '${fieldName}' must be an array`)
        return errors
      }
      
      if (property.items) {
        for (let i = 0; i < value.length; i++) {
          const itemErrors = validateSchemaProperty(value[i], property.items, `${fieldName}[${i}]`)
          errors.push(...itemErrors)
        }
      }
      break
      
    case 'object':
      if (typeof value !== 'object' || Array.isArray(value)) {
        errors.push(`Field '${fieldName}' must be an object`)
        return errors
      }
      
      if (property.properties) {
        // Validate nested properties
        for (const [nestedField, nestedProperty] of Object.entries(property.properties)) {
          if (nestedField in value) {
            const nestedErrors = validateSchemaProperty(value[nestedField], nestedProperty, `${fieldName}.${nestedField}`)
            errors.push(...nestedErrors)
          }
        }
        
        // Check required nested fields
        if (property.required) {
          for (const requiredField of property.required) {
            if (!(requiredField in value) || value[requiredField] === null || value[requiredField] === undefined) {
              errors.push(`Field '${fieldName}.${requiredField}' is required`)
            }
          }
        }
      }
      break
  }
  
  return errors
}

/**
 * Generate default configuration from schema
 */
export function generateDefaultConfig(schema: PluginSchema): Record<string, any> {
  const config: Record<string, any> = {}
  
  if (!schema.properties) {
    return config
  }
  
  for (const [field, property] of Object.entries(schema.properties)) {
    if (property.default !== undefined) {
      config[field] = property.default
    } else {
      // Generate sensible defaults based on type
      switch (property.type) {
        case 'string':
          config[field] = ''
          break
        case 'number':
        case 'integer':
          config[field] = property.minimum || 0
          break
        case 'boolean':
          config[field] = false
          break
        case 'array':
          config[field] = []
          break
        case 'object':
          if (property.properties) {
            config[field] = generateDefaultConfig({ 
              type: 'object', 
              properties: property.properties,
              required: property.required 
            } as PluginSchema)
          } else {
            config[field] = {}
          }
          break
      }
    }
  }
  
  return config
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