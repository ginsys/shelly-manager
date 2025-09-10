import { describe, it, expect, vi, beforeEach } from 'vitest'
import {
  validatePluginConfig,
  generateDefaultConfig,
  getPluginCategoryInfo,
  getPluginStatusInfo,
  formatConfigValue,
  type PluginSchema
} from '../plugin'

describe('Plugin API Utilities', () => {
  const mockSchema: PluginSchema = {
    type: 'object',
    properties: {
      endpoint: {
        name: 'endpoint',
        type: 'string',
        description: 'API endpoint URL',
        format: 'url'
      },
      timeout: {
        name: 'timeout',
        type: 'number',
        description: 'Request timeout in seconds',
        default: 30,
        minimum: 5,
        maximum: 300
      },
      enabled: {
        name: 'enabled',
        type: 'boolean',
        description: 'Enable the plugin',
        default: true
      },
      tags: {
        name: 'tags',
        type: 'array',
        description: 'List of tags',
        items: { name: 'tag', type: 'string' }
      },
      level: {
        name: 'level',
        type: 'string',
        enum: ['debug', 'info', 'warn', 'error'],
        default: 'info'
      }
    },
    required: ['endpoint']
  }

  describe('validatePluginConfig', () => {
    it('should validate configuration successfully', () => {
      const validConfig = {
        endpoint: 'https://valid.example.com',
        timeout: 60,
        enabled: true
      }

      const errors = validatePluginConfig(validConfig, mockSchema)
      expect(errors).toHaveLength(0)
    })

    it('should detect missing required fields', () => {
      const invalidConfig = {
        timeout: 60
      }

      const errors = validatePluginConfig(invalidConfig, mockSchema)
      expect(errors).toContain("Field 'endpoint' is required")
    })

    it('should validate field types', () => {
      const invalidConfig = {
        endpoint: 123, // Should be string
        timeout: 'invalid', // Should be number
        enabled: 'yes' // Should be boolean
      }

      const errors = validatePluginConfig(invalidConfig, mockSchema)
      expect(errors).toContain("Field 'endpoint' must be of type string, got number")
      expect(errors).toContain("Field 'timeout' must be of type number, got string")
      expect(errors).toContain("Field 'enabled' must be of type boolean, got string")
    })

    it('should validate number constraints', () => {
      const invalidConfig = {
        endpoint: 'https://example.com',
        timeout: 500, // Exceeds maximum
        enabled: true
      }

      const errors = validatePluginConfig(invalidConfig, mockSchema)
      expect(errors).toContain("Field 'timeout' must be at most 300")
    })

    it('should validate URL format', () => {
      const invalidConfig = {
        endpoint: 'not-a-url',
        enabled: true
      }

      const errors = validatePluginConfig(invalidConfig, mockSchema)
      expect(errors).toContain("Field 'endpoint' must be a valid URL")
    })

    it('should validate enum values', () => {
      const invalidConfig = {
        endpoint: 'https://example.com',
        level: 'invalid'
      }

      const errors = validatePluginConfig(invalidConfig, mockSchema)
      expect(errors).toContain("Field 'level' must be one of: debug, info, warn, error")
    })
  })

  describe('generateDefaultConfig', () => {
    it('should generate default configuration from schema', () => {
      const defaultConfig = generateDefaultConfig(mockSchema)
      
      expect(defaultConfig.timeout).toBe(30) // From schema default
      expect(defaultConfig.enabled).toBe(true) // From schema default
      expect(defaultConfig.level).toBe('info') // From schema default
      expect(defaultConfig.endpoint).toBe('') // Required field with no default
    })

    it('should handle different field types', () => {
      const complexSchema: PluginSchema = {
        type: 'object',
        properties: {
          stringField: { name: 'stringField', type: 'string', default: 'default value' },
          numberField: { name: 'numberField', type: 'number', default: 42 },
          booleanField: { name: 'booleanField', type: 'boolean', default: false },
          arrayField: { name: 'arrayField', type: 'array', default: ['item1', 'item2'] },
          objectField: { name: 'objectField', type: 'object', default: { key: 'value' } },
          requiredString: { name: 'requiredString', type: 'string' },
          requiredNumber: { name: 'requiredNumber', type: 'number', minimum: 0 }
        },
        required: ['requiredString', 'requiredNumber']
      }

      const config = generateDefaultConfig(complexSchema)
      expect(config.stringField).toBe('default value')
      expect(config.numberField).toBe(42)
      expect(config.booleanField).toBe(false)
      expect(config.arrayField).toEqual(['item1', 'item2'])
      expect(config.objectField).toEqual({ key: 'value' })
      expect(config.requiredString).toBe('')
      expect(config.requiredNumber).toBe(0)
    })
  })

  describe('getPluginCategoryInfo', () => {
    it('should return correct category information', () => {
      const backupInfo = getPluginCategoryInfo('backup')
      expect(backupInfo.label).toBe('Backup')
      expect(backupInfo.icon).toBe('💾')
      expect(backupInfo.color).toBe('#3b82f6')

      const gitopsInfo = getPluginCategoryInfo('gitops')
      expect(gitopsInfo.label).toBe('GitOps')
      expect(gitopsInfo.icon).toBe('🚀')
      expect(gitopsInfo.color).toBe('#10b981')

      const syncInfo = getPluginCategoryInfo('sync')
      expect(syncInfo.label).toBe('Synchronization')
      expect(syncInfo.icon).toBe('🔄')
      expect(syncInfo.color).toBe('#f59e0b')

      const customInfo = getPluginCategoryInfo('custom')
      expect(customInfo.label).toBe('Custom')
      expect(customInfo.icon).toBe('⚙️')
      expect(customInfo.color).toBe('#8b5cf6')
    })

    it('should handle unknown categories', () => {
      const unknownInfo = getPluginCategoryInfo('unknown')
      expect(unknownInfo.label).toBe('unknown')
      expect(unknownInfo.icon).toBe('📦')
      expect(unknownInfo.color).toBe('#6b7280')
    })
  })

  describe('getPluginStatusInfo', () => {
    it('should return correct status information', () => {
      const availableInfo = getPluginStatusInfo('available')
      expect(availableInfo.label).toBe('Available')
      expect(availableInfo.icon).toBe('🔵')
      expect(availableInfo.color).toBe('#6b7280')

      const configuredInfo = getPluginStatusInfo('configured')
      expect(configuredInfo.label).toBe('Configured')
      expect(configuredInfo.icon).toBe('✅')
      expect(configuredInfo.color).toBe('#10b981')

      const errorInfo = getPluginStatusInfo('error')
      expect(errorInfo.label).toBe('Error')
      expect(errorInfo.icon).toBe('❌')
      expect(errorInfo.color).toBe('#ef4444')

      const disabledInfo = getPluginStatusInfo('disabled')
      expect(disabledInfo.label).toBe('Disabled')
      expect(disabledInfo.icon).toBe('⏸️')
      expect(disabledInfo.color).toBe('#9ca3af')
    })

    it('should handle unknown statuses', () => {
      const unknownInfo = getPluginStatusInfo('unknown')
      expect(unknownInfo.label).toBe('unknown')
      expect(unknownInfo.icon).toBe('❓')
      expect(unknownInfo.color).toBe('#6b7280')
    })
  })

  describe('formatConfigValue', () => {
    it('should format different value types correctly', () => {
      expect(formatConfigValue(undefined)).toBe('—')
      expect(formatConfigValue(null)).toBe('—')
      expect(formatConfigValue('simple string')).toBe('simple string')
      expect(formatConfigValue(true)).toBe('Yes')
      expect(formatConfigValue(false)).toBe('No')
      expect(formatConfigValue([1, 2, 3])).toBe('[3 items]')
      expect(formatConfigValue({ a: 1, b: 2 })).toBe('{2 properties}')
      expect(formatConfigValue(123)).toBe('123')
    })

    it('should truncate long strings', () => {
      const longString = 'a'.repeat(100)
      const formatted = formatConfigValue(longString)
      expect(formatted).toHaveLength(50) // 47 chars + '...'
      expect(formatted.endsWith('...')).toBe(true)
    })

    it('should handle edge cases', () => {
      expect(formatConfigValue(0)).toBe('0')
      expect(formatConfigValue('')).toBe('')
      expect(formatConfigValue([])).toBe('[0 items]')
      expect(formatConfigValue({})).toBe('{0 properties}')
    })
  })

  describe('utility functions', () => {
    it('should validate email addresses correctly', () => {
      const schemaWithEmail: PluginSchema = {
        type: 'object',
        properties: {
          email: { name: 'email', type: 'string', format: 'email' }
        },
        required: []
      }

      const validConfig = { email: 'user@example.com' }
      const invalidConfig = { email: 'invalid-email' }

      const validErrors = validatePluginConfig(validConfig, schemaWithEmail)
      const invalidErrors = validatePluginConfig(invalidConfig, schemaWithEmail)

      expect(validErrors).toHaveLength(0)
      expect(invalidErrors).toContain("Field 'email' must be a valid email address")
    })

    it('should validate pattern constraints', () => {
      const schemaWithPattern: PluginSchema = {
        type: 'object',
        properties: {
          code: { 
            name: 'code', 
            type: 'string', 
            pattern: '^[A-Z]{3}[0-9]{3}$'
          }
        },
        required: []
      }

      const validConfig = { code: 'ABC123' }
      const invalidConfig = { code: 'invalid' }

      const validErrors = validatePluginConfig(validConfig, schemaWithPattern)
      const invalidErrors = validatePluginConfig(invalidConfig, schemaWithPattern)

      expect(validErrors).toHaveLength(0)
      expect(invalidErrors).toContain("Field 'code' does not match required pattern")
    })

    it('should handle numeric string conversion', () => {
      const config = {
        endpoint: 'https://example.com',
        timeout: '60' // String that can be converted to number
      }

      const errors = validatePluginConfig(config, mockSchema)
      expect(errors).toHaveLength(0) // Should accept string numbers
    })
  })
})