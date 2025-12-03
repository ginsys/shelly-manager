/**
 * Schema-driven form field definitions
 * Supports dynamic form generation from configuration objects
 */

export interface SelectOption {
  value: string | number | boolean
  label: string
}

export interface FieldDefinition {
  type?: 'string' | 'number' | 'boolean' | 'select' | 'textarea'
  label?: string
  description?: string
  placeholder?: string
  required?: boolean
  // Number-specific
  min?: number
  max?: number
  // Select-specific
  options?: SelectOption[]
  // Default value
  default?: unknown
}

export interface FieldSchema {
  [key: string]: FieldDefinition
}

export interface FormSchema {
  config?: FieldSchema
  filters?: FieldSchema
  [section: string]: FieldSchema | undefined
}
