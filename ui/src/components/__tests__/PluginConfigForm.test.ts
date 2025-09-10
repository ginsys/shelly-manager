import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { Quasar } from 'quasar'
import PluginConfigForm from '../PluginConfigForm.vue'
import type { Plugin, PluginSchema } from '../../api/plugin'

// Mock the plugin store
const mockPluginStore = {
  getPluginSchema: vi.fn()
}

vi.mock('../../stores/plugin', () => ({
  usePluginStore: () => mockPluginStore
}))

// Mock Quasar notify
const mockNotify = vi.fn()
vi.mock('quasar', async () => {
  const actual = await vi.importActual('quasar')
  return {
    ...actual,
    useQuasar: () => ({
      notify: mockNotify
    })
  }
})

const mockPlugin: Plugin = {
  name: 'test-plugin',
  version: '1.0.0',
  description: 'Test plugin for form testing',
  category: 'custom',
  status: 'active',
  author: 'Test Author',
  enabled: true,
  configured: false,
  supported_formats: ['json'],
  tags: ['test'],
  created_at: '2023-01-01T00:00:00Z',
  updated_at: '2023-01-01T00:00:00Z'
}

const mockSchema: PluginSchema = {
  type: 'object',
  properties: {
    apiKey: {
      type: 'string',
      title: 'API Key',
      description: 'Authentication key for the service',
      format: 'password'
    },
    maxRetries: {
      type: 'integer',
      title: 'Max Retries',
      description: 'Maximum number of retry attempts',
      minimum: 1,
      maximum: 10,
      default: 3
    },
    enableLogging: {
      type: 'boolean',
      title: 'Enable Logging',
      description: 'Whether to enable debug logging',
      default: false
    },
    categories: {
      type: 'array',
      title: 'Categories',
      description: 'Select applicable categories',
      items: {
        type: 'string',
        enum: ['sync', 'backup', 'notification']
      }
    },
    format: {
      type: 'string',
      title: 'Output Format',
      description: 'Format for output data',
      enum: ['json', 'yaml', 'xml']
    },
    advancedConfig: {
      type: 'object',
      title: 'Advanced Configuration',
      description: 'Advanced settings in JSON format'
    }
  },
  required: ['apiKey', 'maxRetries']
}

describe('PluginConfigForm', () => {
  let wrapper: any

  beforeEach(() => {
    vi.clearAllMocks()
    mockPluginStore.getPluginSchema.mockResolvedValue(mockSchema)
  })

  const createWrapper = (props = {}) => {
    return mount(PluginConfigForm, {
      props: {
        plugin: mockPlugin,
        modelValue: true,
        ...props
      },
      global: {
        plugins: [Quasar]
      }
    })
  }

  describe('Component Initialization', () => {
    it('should render the dialog when modelValue is true', async () => {
      wrapper = createWrapper()
      await wrapper.vm.$nextTick()

      expect(wrapper.find('.q-dialog').exists()).toBe(true)
      expect(wrapper.find('.text-h6').text()).toBe('Configure test-plugin')
    })

    it('should not render the dialog when modelValue is false', async () => {
      wrapper = createWrapper({ modelValue: false })
      await wrapper.vm.$nextTick()

      expect(wrapper.find('.q-dialog').exists()).toBe(false)
    })

    it('should display plugin information', async () => {
      wrapper = createWrapper()
      await wrapper.vm.$nextTick()

      expect(wrapper.text()).toContain('test-plugin')
      expect(wrapper.text()).toContain('Test plugin for form testing')
      expect(wrapper.text()).toContain('Version: 1.0.0')
    })

    it('should load plugin schema on mount', async () => {
      wrapper = createWrapper()
      await wrapper.vm.$nextTick()
      await new Promise(resolve => setTimeout(resolve, 0))

      expect(mockPluginStore.getPluginSchema).toHaveBeenCalledWith('test-plugin')
    })
  })

  describe('Form Field Generation', () => {
    beforeEach(async () => {
      wrapper = createWrapper()
      await wrapper.vm.$nextTick()
      await new Promise(resolve => setTimeout(resolve, 0))
    })

    it('should generate string input fields', () => {
      const apiKeyField = wrapper.find('input[aria-label="API Key"]')
      expect(apiKeyField.exists()).toBe(true)
    })

    it('should generate number input fields', () => {
      const retriesField = wrapper.find('input[aria-label="Max Retries"]')
      expect(retriesField.exists()).toBe(true)
      expect(retriesField.attributes('type')).toBe('number')
    })

    it('should generate boolean toggle fields', () => {
      const loggingToggle = wrapper.find('[role="switch"]')
      expect(loggingToggle.exists()).toBe(true)
    })

    it('should generate select fields for enums', () => {
      const formatSelect = wrapper.find('[aria-label="Output Format"]')
      expect(formatSelect.exists()).toBe(true)
    })

    it('should generate multiselect for arrays', () => {
      const categoriesSelect = wrapper.find('[aria-label="Categories"]')
      expect(categoriesSelect.exists()).toBe(true)
    })

    it('should generate textarea for objects', () => {
      const advancedConfigArea = wrapper.find('textarea[aria-label="Advanced Configuration (JSON)"]')
      expect(advancedConfigArea.exists()).toBe(true)
    })
  })

  describe('Form Validation', () => {
    beforeEach(async () => {
      wrapper = createWrapper()
      await wrapper.vm.$nextTick()
      await new Promise(resolve => setTimeout(resolve, 0))
    })

    it('should show validation errors for missing required fields', async () => {
      // Clear required fields
      await wrapper.setData({
        formData: {
          apiKey: '',
          maxRetries: null,
          enableLogging: false
        }
      })

      expect(wrapper.vm.validationErrors).toContain('API Key is required')
      expect(wrapper.vm.validationErrors).toContain('Max Retries is required')
    })

    it('should validate number constraints', async () => {
      await wrapper.setData({
        formData: {
          apiKey: 'test-key',
          maxRetries: -1, // violates minimum
          enableLogging: false
        }
      })

      expect(wrapper.vm.validationErrors).toContain('Max Retries must be at least 1')
    })

    it('should validate JSON format for object fields', async () => {
      await wrapper.setData({
        formData: {
          apiKey: 'test-key',
          maxRetries: 3,
          advancedConfig: 'invalid json'
        }
      })

      expect(wrapper.vm.validationErrors).toContain('Advanced Configuration must be valid JSON')
    })

    it('should disable submit button when validation errors exist', async () => {
      await wrapper.setData({
        formData: {
          apiKey: '',
          maxRetries: null
        }
      })

      const submitButton = wrapper.find('button[type="submit"]')
      expect(submitButton.attributes('disabled')).toBeDefined()
    })
  })

  describe('Form Interaction', () => {
    beforeEach(async () => {
      wrapper = createWrapper()
      await wrapper.vm.$nextTick()
      await new Promise(resolve => setTimeout(resolve, 0))
    })

    it('should update form data when inputs change', async () => {
      const apiKeyInput = wrapper.find('input[aria-label="API Key"]')
      await apiKeyInput.setValue('new-api-key')

      expect(wrapper.vm.formData.apiKey).toBe('new-api-key')
    })

    it('should reset form data when reset button is clicked', async () => {
      // Change form data
      await wrapper.setData({
        formData: {
          apiKey: 'changed-key',
          maxRetries: 5
        }
      })

      const resetButton = wrapper.find('button[type="reset"]')
      await resetButton.trigger('click')

      // Should reset to default values
      expect(wrapper.vm.formData.maxRetries).toBe(3) // default value from schema
      expect(wrapper.vm.formData.enableLogging).toBe(false) // default value from schema
    })

    it('should show configuration preview', async () => {
      const previewButton = wrapper.find('[label="Configuration Preview"]')
      await previewButton.trigger('click')

      expect(wrapper.find('pre').exists()).toBe(true)
      expect(wrapper.text()).toContain('"apiKey"')
    })
  })

  describe('Profile Management', () => {
    beforeEach(async () => {
      wrapper = createWrapper()
      await wrapper.vm.$nextTick()
      await new Promise(resolve => setTimeout(resolve, 0))
    })

    it('should save configuration profiles', async () => {
      const saveProfileSpy = vi.spyOn(Storage.prototype, 'setItem')
      
      await wrapper.setData({
        formData: { apiKey: 'test', maxRetries: 5 },
        newProfileName: 'Test Profile',
        showSaveProfile: true
      })

      await wrapper.vm.saveProfile()

      expect(saveProfileSpy).toHaveBeenCalled()
      expect(mockNotify).toHaveBeenCalledWith({
        type: 'positive',
        message: 'Profile saved successfully'
      })
    })

    it('should load configuration profiles', async () => {
      const getItemSpy = vi.spyOn(Storage.prototype, 'getItem')
      getItemSpy.mockReturnValue(JSON.stringify({
        'Test Profile': { apiKey: 'saved-key', maxRetries: 7 }
      }))

      await wrapper.vm.loadSavedProfiles()
      await wrapper.vm.loadProfile('Test Profile')

      expect(wrapper.vm.formData.apiKey).toBe('saved-key')
      expect(wrapper.vm.formData.maxRetries).toBe(7)
    })
  })

  describe('Configuration Testing', () => {
    beforeEach(async () => {
      wrapper = createWrapper()
      await wrapper.vm.$nextTick()
      await new Promise(resolve => setTimeout(resolve, 0))
    })

    it('should test configuration when test button is clicked', async () => {
      await wrapper.setData({
        formData: {
          apiKey: 'test-key',
          maxRetries: 3,
          enableLogging: true
        }
      })

      const testButton = wrapper.find('[label="Test Config"]')
      await testButton.trigger('click')

      expect(wrapper.vm.testing).toBe(true)
    })

    it('should show loading state during test', async () => {
      await wrapper.setData({ testing: true })

      const testButton = wrapper.find('[label="Test Config"]')
      expect(testButton.attributes('loading')).toBe('true')
    })
  })

  describe('Form Submission', () => {
    beforeEach(async () => {
      wrapper = createWrapper()
      await wrapper.vm.$nextTick()
      await new Promise(resolve => setTimeout(resolve, 0))
    })

    it('should emit configured event on successful submission', async () => {
      await wrapper.setData({
        formData: {
          apiKey: 'test-key',
          maxRetries: 3,
          enableLogging: true
        }
      })

      const form = wrapper.find('form')
      await form.trigger('submit.prevent')

      expect(wrapper.emitted().configured).toBeTruthy()
      expect(wrapper.emitted().configured[0]).toEqual([
        mockPlugin,
        {
          apiKey: 'test-key',
          maxRetries: 3,
          enableLogging: true
        }
      ])
    })

    it('should show loading state during save', async () => {
      await wrapper.setData({ saving: true })

      const submitButton = wrapper.find('button[type="submit"]')
      expect(submitButton.attributes('loading')).toBe('true')
    })

    it('should not submit when validation errors exist', async () => {
      await wrapper.setData({
        formData: {
          apiKey: '', // required field empty
          maxRetries: 3
        }
      })

      const form = wrapper.find('form')
      await form.trigger('submit.prevent')

      expect(wrapper.emitted().configured).toBeFalsy()
    })
  })

  describe('Error Handling', () => {
    it('should handle schema loading errors', async () => {
      mockPluginStore.getPluginSchema.mockRejectedValue(new Error('Failed to load schema'))
      
      wrapper = createWrapper()
      await wrapper.vm.$nextTick()
      await new Promise(resolve => setTimeout(resolve, 0))

      expect(wrapper.vm.error).toBe('Failed to load plugin schema')
      expect(wrapper.text()).toContain('Failed to load plugin schema')
    })

    it('should display loading state while fetching schema', async () => {
      // Create a promise that we can control
      let resolveSchema: any
      const schemaPromise = new Promise(resolve => { resolveSchema = resolve })
      mockPluginStore.getPluginSchema.mockReturnValue(schemaPromise)

      wrapper = createWrapper()
      await wrapper.vm.$nextTick()

      expect(wrapper.vm.loading).toBe(true)
      expect(wrapper.text()).toContain('Loading plugin schema...')

      // Resolve the promise
      resolveSchema(mockSchema)
      await wrapper.vm.$nextTick()

      expect(wrapper.vm.loading).toBe(false)
    })
  })

  describe('Dialog Control', () => {
    it('should emit update:modelValue when dialog is closed', async () => {
      wrapper = createWrapper()
      await wrapper.vm.$nextTick()

      const closeButton = wrapper.find('[icon="close"]')
      await closeButton.trigger('click')

      expect(wrapper.emitted()['update:modelValue']).toBeTruthy()
      expect(wrapper.emitted()['update:modelValue'][0]).toEqual([false])
    })

    it('should close dialog after successful configuration', async () => {
      wrapper = createWrapper()
      await wrapper.vm.$nextTick()
      await new Promise(resolve => setTimeout(resolve, 0))

      await wrapper.setData({
        formData: {
          apiKey: 'test-key',
          maxRetries: 3
        }
      })

      const form = wrapper.find('form')
      await form.trigger('submit.prevent')
      await wrapper.vm.$nextTick()

      expect(wrapper.emitted()['update:modelValue']).toBeTruthy()
      expect(wrapper.emitted()['update:modelValue'].slice(-1)[0]).toEqual([false])
    })
  })

  describe('Accessibility', () => {
    beforeEach(async () => {
      wrapper = createWrapper()
      await wrapper.vm.$nextTick()
      await new Promise(resolve => setTimeout(resolve, 0))
    })

    it('should have proper ARIA labels on form fields', () => {
      const apiKeyField = wrapper.find('input[aria-label="API Key"]')
      const retriesField = wrapper.find('input[aria-label="Max Retries"]')
      
      expect(apiKeyField.exists()).toBe(true)
      expect(retriesField.exists()).toBe(true)
    })

    it('should have proper form structure for screen readers', () => {
      const form = wrapper.find('form')
      expect(form.exists()).toBe(true)
      
      const fieldsets = wrapper.findAll('fieldset')
      expect(fieldsets.length).toBeGreaterThan(0)
    })

    it('should provide validation feedback for screen readers', async () => {
      await wrapper.setData({
        formData: {
          apiKey: '',
          maxRetries: null
        }
      })

      expect(wrapper.text()).toContain('API Key is required')
      expect(wrapper.text()).toContain('Max Retries is required')
    })
  })
})