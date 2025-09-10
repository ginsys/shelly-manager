import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, VueWrapper } from '@vue/test-utils'
import PluginSchemaViewer from '../PluginSchemaViewer.vue'
import type { Plugin, PluginSchema, PluginTestResult } from '@/api/plugin'

// Mock the plugin API functions
vi.mock('@/api/plugin', () => ({
  getPluginCategoryInfo: vi.fn((category: string) => ({
    label: category.charAt(0).toUpperCase() + category.slice(1),
    icon: '📦',
    color: '#6b7280'
  }))
}))

describe('PluginSchemaViewer', () => {
  let wrapper: VueWrapper<any>

  const mockPlugin: Plugin = {
    name: 'test-plugin',
    display_name: 'Test Plugin',
    description: 'A test plugin for unit testing',
    version: '1.0.0',
    author: 'Test Author',
    category: 'backup',
    capabilities: [
      { name: 'backup', description: 'Full system backup', required: true },
      { name: 'incremental', description: 'Incremental backup', required: false }
    ],
    status: 'configured'
  }

  const mockSchema: PluginSchema = {
    type: 'object',
    title: 'Test Plugin Configuration',
    description: 'Configuration schema for the test plugin',
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
      metadata: {
        name: 'metadata',
        type: 'object',
        description: 'Additional metadata'
      },
      level: {
        name: 'level',
        type: 'string',
        description: 'Log level',
        enum: ['debug', 'info', 'warn', 'error'],
        default: 'info'
      },
      password: {
        name: 'password',
        type: 'string',
        description: 'Password field',
        format: 'password'
      },
      description_field: {
        name: 'description_field',
        type: 'string',
        description: 'Long description field',
        format: 'textarea'
      }
    },
    required: ['endpoint'],
    examples: [
      {
        endpoint: 'https://example.com/api',
        timeout: 60,
        enabled: true,
        tags: ['production', 'backup'],
        metadata: { region: 'us-east-1' },
        level: 'info'
      }
    ]
  }

  const defaultProps = {
    plugin: mockPlugin,
    schema: mockSchema,
    configuration: {
      endpoint: 'https://test.example.com',
      timeout: 30,
      enabled: true,
      tags: ['test'],
      metadata: { env: 'test' }
    },
    validationErrors: [],
    loading: false,
    testing: false
  }

  beforeEach(() => {
    wrapper = mount(PluginSchemaViewer, {
      props: defaultProps
    })
  })

  describe('component rendering', () => {
    it('should render plugin overview', () => {
      expect(wrapper.find('.plugin-basic-info h4').text()).toContain('Test Plugin')
      expect(wrapper.find('.plugin-basic-info h4').text()).toContain('v1.0.0')
      expect(wrapper.find('.plugin-description').text()).toBe('A test plugin for unit testing')
      expect(wrapper.find('.plugin-author').text()).toBe('by Test Author')
    })

    it('should render plugin capabilities', () => {
      const capabilities = wrapper.findAll('.capability-badge')
      expect(capabilities).toHaveLength(2)
      expect(capabilities[0].text()).toContain('backup')
      expect(capabilities[0].classes()).toContain('required')
      expect(capabilities[1].text()).toContain('incremental')
      expect(capabilities[1].classes()).not.toContain('required')
    })

    it('should render form header with action buttons', () => {
      const buttons = wrapper.findAll('.action-button')
      expect(buttons).toHaveLength(4) // Generate Default, Reset, Test, Save
      expect(buttons[0].text()).toContain('Generate Default')
      expect(buttons[1].text()).toContain('Reset')
      expect(buttons[2].text()).toContain('Test')
      expect(buttons[3].text()).toContain('Save')
    })
  })

  describe('form field rendering', () => {
    it('should render string field with correct input type', () => {
      const endpointInput = wrapper.find('input[type="url"]')
      expect(endpointInput.exists()).toBe(true)
      expect((endpointInput.element as HTMLInputElement).value).toBe('https://test.example.com')
    })

    it('should render password field with password input type', () => {
      const passwordInput = wrapper.find('input[type="password"]')
      expect(passwordInput.exists()).toBe(true)
    })

    it('should render textarea for long content', () => {
      const textarea = wrapper.find('.field-textarea')
      expect(textarea.exists()).toBe(true)
    })

    it('should render number field with constraints', () => {
      const numberInput = wrapper.find('input[type="number"]')
      expect(numberInput.exists()).toBe(true)
      expect((numberInput.element as HTMLInputElement).min).toBe('5')
      expect((numberInput.element as HTMLInputElement).max).toBe('300')
      expect((numberInput.element as HTMLInputElement).value).toBe('30')
    })

    it('should render boolean field as checkbox', () => {
      const checkbox = wrapper.find('.field-checkbox')
      expect(checkbox.exists()).toBe(true)
      expect((checkbox.element as HTMLInputElement).checked).toBe(true)
    })

    it('should render select field for enum values', () => {
      const select = wrapper.find('select')
      expect(select.exists()).toBe(true)
      const options = select.findAll('option')
      expect(options).toHaveLength(5) // Empty option + 4 enum values
      expect(options[1].text()).toBe('debug')
      expect(options[2].text()).toBe('info')
    })

    it('should render array field with add/remove buttons', () => {
      const arrayField = wrapper.find('.array-field')
      expect(arrayField.exists()).toBe(true)
      
      const arrayItems = wrapper.findAll('.array-item')
      expect(arrayItems).toHaveLength(1) // One tag in the configuration
      
      const addButton = wrapper.find('.array-add-button')
      expect(addButton.exists()).toBe(true)
      expect(addButton.text()).toContain('Add')
      
      const removeButton = wrapper.find('.array-remove-button')
      expect(removeButton.exists()).toBe(true)
    })

    it('should render object field as JSON textarea', () => {
      const objectTextarea = wrapper.find('.field-object-json')
      expect(objectTextarea.exists()).toBe(true)
      const value = (objectTextarea.element as HTMLTextAreaElement).value
      expect(JSON.parse(value)).toEqual({ env: 'test' })
    })

    it('should show required markers for required fields', () => {
      const requiredMarkers = wrapper.findAll('.required-marker')
      expect(requiredMarkers.length).toBeGreaterThan(0)
    })
  })

  describe('field interactions', () => {
    it('should update string field value', async () => {
      const input = wrapper.find('input[type="url"]')
      await input.setValue('https://updated.example.com')
      
      expect(wrapper.emitted('update:configuration')).toBeTruthy()
      const emittedConfig = wrapper.emitted('update:configuration')![0][0] as Record<string, any>
      expect(emittedConfig.endpoint).toBe('https://updated.example.com')
    })

    it('should update number field value', async () => {
      const input = wrapper.find('input[type="number"]')
      await input.setValue('60')
      
      expect(wrapper.emitted('update:configuration')).toBeTruthy()
      const emittedConfig = wrapper.emitted('update:configuration')![0][0] as Record<string, any>
      expect(emittedConfig.timeout).toBe(60)
    })

    it('should update boolean field value', async () => {
      const checkbox = wrapper.find('.field-checkbox')
      await checkbox.setChecked(false)
      
      expect(wrapper.emitted('update:configuration')).toBeTruthy()
      const emittedConfig = wrapper.emitted('update:configuration')![0][0] as Record<string, any>
      expect(emittedConfig.enabled).toBe(false)
    })

    it('should update select field value', async () => {
      const select = wrapper.find('select')
      await select.setValue('debug')
      
      expect(wrapper.emitted('update:configuration')).toBeTruthy()
      const emittedConfig = wrapper.emitted('update:configuration')![0][0] as Record<string, any>
      expect(emittedConfig.level).toBe('debug')
    })

    it('should add array item', async () => {
      const addButton = wrapper.find('.array-add-button')
      await addButton.trigger('click')
      
      expect(wrapper.emitted('update:configuration')).toBeTruthy()
      const emittedConfig = wrapper.emitted('update:configuration')![0][0] as Record<string, any>
      expect(emittedConfig.tags).toHaveLength(2)
      expect(emittedConfig.tags[1]).toBe('')
    })

    it('should remove array item', async () => {
      const removeButton = wrapper.find('.array-remove-button')
      await removeButton.trigger('click')
      
      expect(wrapper.emitted('update:configuration')).toBeTruthy()
      const emittedConfig = wrapper.emitted('update:configuration')![0][0] as Record<string, any>
      expect(emittedConfig.tags).toHaveLength(0)
    })

    it('should update array item value', async () => {
      const arrayInput = wrapper.find('.array-item-input')
      await arrayInput.setValue('updated-tag')
      
      expect(wrapper.emitted('update:configuration')).toBeTruthy()
      const emittedConfig = wrapper.emitted('update:configuration')![0][0] as Record<string, any>
      expect(emittedConfig.tags[0]).toBe('updated-tag')
    })

    it('should update object field with valid JSON', async () => {
      const objectTextarea = wrapper.find('.field-object-json')
      const newValue = JSON.stringify({ key: 'value', number: 123 }, null, 2)
      await objectTextarea.setValue(newValue)
      
      expect(wrapper.emitted('update:configuration')).toBeTruthy()
      const emittedConfig = wrapper.emitted('update:configuration')![0][0] as Record<string, any>
      expect(emittedConfig.metadata).toEqual({ key: 'value', number: 123 })
    })
  })

  describe('action buttons', () => {
    it('should emit test event when test button is clicked', async () => {
      const testButton = wrapper.find('.action-button.test')
      await testButton.trigger('click')
      
      expect(wrapper.emitted('test')).toBeTruthy()
      expect(wrapper.emitted('test')![0][0]).toEqual(defaultProps.configuration)
    })

    it('should emit save event when save button is clicked', async () => {
      const saveButton = wrapper.find('.action-button.primary')
      await saveButton.trigger('click')
      
      expect(wrapper.emitted('save')).toBeTruthy()
      expect(wrapper.emitted('save')![0][0]).toEqual(defaultProps.configuration)
    })

    it('should emit reset event when reset button is clicked', async () => {
      const resetButton = wrapper.findAll('.action-button.secondary')[1] // Second secondary button is Reset
      await resetButton.trigger('click')
      
      expect(wrapper.emitted('reset')).toBeTruthy()
    })

    it('should emit generate-default event when generate default button is clicked', async () => {
      const generateButton = wrapper.findAll('.action-button.secondary')[0] // First secondary button is Generate Default
      await generateButton.trigger('click')
      
      expect(wrapper.emitted('generate-default')).toBeTruthy()
    })

    it('should disable save button when there are validation errors', async () => {
      await wrapper.setProps({
        ...defaultProps,
        validationErrors: ['Endpoint is required']
      })
      
      const saveButton = wrapper.find('.action-button.primary')
      expect((saveButton.element as HTMLButtonElement).disabled).toBe(true)
    })

    it('should show testing state on test button', async () => {
      await wrapper.setProps({
        ...defaultProps,
        testing: true
      })
      
      const testButton = wrapper.find('.action-button.test')
      expect(testButton.text()).toContain('Testing...')
      expect((testButton.element as HTMLButtonElement).disabled).toBe(true)
    })
  })

  describe('validation errors display', () => {
    it('should display validation errors', async () => {
      await wrapper.setProps({
        ...defaultProps,
        validationErrors: ['Endpoint is required', 'Timeout must be positive']
      })
      
      const errorSection = wrapper.find('.validation-errors')
      expect(errorSection.exists()).toBe(true)
      
      const errorItems = wrapper.findAll('.error-item')
      expect(errorItems).toHaveLength(2)
      expect(errorItems[0].text()).toBe('Endpoint is required')
      expect(errorItems[1].text()).toBe('Timeout must be positive')
    })

    it('should not display validation errors section when no errors', () => {
      const errorSection = wrapper.find('.validation-errors')
      expect(errorSection.exists()).toBe(false)
    })
  })

  describe('test result display', () => {
    const successResult: PluginTestResult = {
      success: true,
      message: 'Test completed successfully',
      response_time_ms: 150,
      details: { status: 'ok' }
    }

    const failureResult: PluginTestResult = {
      success: false,
      message: 'Connection failed',
      error: 'Timeout after 30 seconds'
    }

    it('should display successful test result', async () => {
      await wrapper.setProps({
        ...defaultProps,
        testResult: successResult
      })
      
      const testResult = wrapper.find('.test-result.success')
      expect(testResult.exists()).toBe(true)
      expect(testResult.text()).toContain('Test Successful')
      expect(testResult.text()).toContain('Test completed successfully')
      expect(testResult.text()).toContain('(150ms)')
    })

    it('should display failed test result with error', async () => {
      await wrapper.setProps({
        ...defaultProps,
        testResult: failureResult
      })
      
      const testResult = wrapper.find('.test-result.failure')
      expect(testResult.exists()).toBe(true)
      expect(testResult.text()).toContain('Test Failed')
      expect(testResult.text()).toContain('Connection failed')
      expect(testResult.text()).toContain('Timeout after 30 seconds')
    })

    it('should display test details when available', async () => {
      await wrapper.setProps({
        ...defaultProps,
        testResult: successResult
      })
      
      const detailsSection = wrapper.find('.test-details')
      expect(detailsSection.exists()).toBe(true)
      expect(detailsSection.text()).toContain('Details')
    })

    it('should not display test result when testing', async () => {
      await wrapper.setProps({
        ...defaultProps,
        testing: true,
        testResult: successResult
      })
      
      const testResult = wrapper.find('.test-result')
      expect(testResult.exists()).toBe(false)
    })
  })

  describe('template examples', () => {
    it('should display configuration templates', () => {
      const templatesSection = wrapper.find('.schema-examples')
      expect(templatesSection.exists()).toBe(true)
      
      const templateCards = wrapper.findAll('.template-card')
      expect(templateCards).toHaveLength(1)
      
      const templateJson = wrapper.find('.template-json')
      expect(templateJson.exists()).toBe(true)
    })

    it('should load template when clicked', async () => {
      const templateCard = wrapper.find('.template-card')
      await templateCard.trigger('click')
      
      expect(wrapper.emitted('update:configuration')).toBeTruthy()
      const emittedConfig = wrapper.emitted('update:configuration')![0][0] as Record<string, any>
      expect(emittedConfig.endpoint).toBe('https://example.com/api')
      expect(emittedConfig.timeout).toBe(60)
    })

    it('should show field examples when available', () => {
      // The schema has examples, so field examples should be visible
      const examples = wrapper.findAll('.field-examples')
      expect(examples.length).toBeGreaterThan(0)
    })
  })

  describe('loading states', () => {
    it('should display loading state when loading', async () => {
      await wrapper.setProps({
        plugin: mockPlugin,
        schema: null,
        configuration: {},
        validationErrors: [],
        loading: true,
        testing: false
      })
      
      const loadingState = wrapper.find('.loading-state')
      expect(loadingState.exists()).toBe(true)
      expect(loadingState.text()).toContain('Loading schema...')
    })

    it('should display no schema state when schema is null and not loading', async () => {
      await wrapper.setProps({
        plugin: mockPlugin,
        schema: null,
        configuration: {},
        validationErrors: [],
        loading: false,
        testing: false
      })
      
      const noSchemaState = wrapper.find('.no-schema-state')
      expect(noSchemaState.exists()).toBe(true)
      expect(noSchemaState.text()).toContain('No configuration schema available')
    })
  })

  describe('field utilities', () => {
    it('should generate proper field labels', () => {
      // Test that field names are converted to proper labels
      const labels = wrapper.findAll('.field-label')
      const endpointLabel = labels.find(label => label.text().includes('Endpoint'))
      expect(endpointLabel).toBeTruthy()
    })

    it('should generate appropriate placeholders', () => {
      const urlInput = wrapper.find('input[type="url"]')
      expect(urlInput.attributes('placeholder')).toBeTruthy()
    })

    it('should handle different number step values', () => {
      // The timeout field should have step="1" since minimum is an integer
      const numberInput = wrapper.find('input[type="number"]')
      expect(numberInput.attributes('step')).toBe('1')
    })
  })

  describe('accessibility', () => {
    it('should associate labels with form controls', () => {
      const labels = wrapper.findAll('.field-label')
      expect(labels.length).toBeGreaterThan(0)
      
      // Each form field should have a descriptive label
      const inputs = wrapper.findAll('input, select, textarea')
      inputs.forEach(input => {
        // Should have either a label association or aria-label
        const hasLabel = input.attributes('id') || input.attributes('aria-label')
        expect(hasLabel).toBeTruthy()
      })
    })

    it('should provide field descriptions', () => {
      const descriptions = wrapper.findAll('.field-description')
      expect(descriptions.length).toBeGreaterThan(0)
      
      // Each description should contain helpful text
      descriptions.forEach(desc => {
        expect(desc.text().length).toBeGreaterThan(0)
      })
    })

    it('should mark required fields clearly', () => {
      const requiredMarkers = wrapper.findAll('.required-marker')
      expect(requiredMarkers.length).toBeGreaterThan(0)
      
      // Required markers should be clearly visible
      requiredMarkers.forEach(marker => {
        expect(marker.text()).toContain('*')
      })
    })
  })
})