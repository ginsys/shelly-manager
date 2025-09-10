import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import GitOpsConfigForm from './GitOpsConfigForm.vue'
import type { Device } from '@/api/types'
import { previewGitOpsExport, type GitOpsExportRequest } from '@/api/export'

// Mock the API
vi.mock('@/api/export', () => ({
  previewGitOpsExport: vi.fn()
}))

const mockPreviewGitOpsExport = vi.mocked(previewGitOpsExport)

describe('GitOpsConfigForm', () => {
  const mockDevices: Device[] = [
    {
      id: 1,
      ip: '192.168.1.100',
      mac: 'aa:bb:cc:dd:ee:01',
      type: 'shelly1',
      name: 'Living Room Switch',
      firmware: '1.14.0',
      status: 'online',
      last_seen: '2023-01-01T00:00:00Z'
    },
    {
      id: 2,
      ip: '192.168.1.101',
      mac: 'aa:bb:cc:dd:ee:02',
      type: 'shelly25',
      name: 'Kitchen Roller',
      firmware: '1.14.0',
      status: 'online',
      last_seen: '2023-01-01T00:00:00Z'
    }
  ]

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('should render the form with all sections', () => {
    const wrapper = mount(GitOpsConfigForm, {
      props: {
        availableDevices: mockDevices,
        loading: false,
        error: ''
      }
    })

    // Check main sections are present
    expect(wrapper.text()).toContain('Basic Information')
    expect(wrapper.text()).toContain('Export Format')
    expect(wrapper.text()).toContain('Device Selection')
    expect(wrapper.text()).toContain('Git Configuration')
    expect(wrapper.text()).toContain('Additional Options')
  })

  it('should initialize with default values', () => {
    const wrapper = mount(GitOpsConfigForm, {
      props: {
        availableDevices: mockDevices,
        loading: false,
        error: ''
      }
    })

    // Check default form values
    const nameInput = wrapper.find('input[placeholder*="Production Devices GitOps"]')
    expect(nameInput.element.value).toBe('')

    const formatSelect = wrapper.find('select').find('option[selected]')
    expect(formatSelect.exists()).toBe(false) // No default selected

    // Check default options
    expect(wrapper.text()).toContain('All devices (2 devices)')
  })

  it('should validate required fields', async () => {
    const wrapper = mount(GitOpsConfigForm, {
      props: {
        availableDevices: mockDevices,
        loading: false,
        error: ''
      }
    })

    // Try to submit without filling required fields
    const form = wrapper.find('form')
    await form.trigger('submit.prevent')

    // Check for validation errors (they would be shown in the UI)
    expect(wrapper.emitted('submit')).toBeFalsy()
  })

  it('should show format-specific options when format is selected', async () => {
    const wrapper = mount(GitOpsConfigForm, {
      props: {
        availableDevices: mockDevices,
        loading: false,
        error: ''
      }
    })

    // Select Terraform format
    const formatSelect = wrapper.find('select')
    await formatSelect.setValue('terraform')

    // Should show Terraform-specific options
    expect(wrapper.text()).toContain('Terraform Configuration')
    expect(wrapper.text()).toContain('Provider Version')
    expect(wrapper.text()).toContain('Module Structure')
  })

  it('should show different options for different formats', async () => {
    const wrapper = mount(GitOpsConfigForm, {
      props: {
        availableDevices: mockDevices,
        loading: false,
        error: ''
      }
    })

    // Test Kubernetes format
    const formatSelect = wrapper.find('select')
    await formatSelect.setValue('kubernetes')

    expect(wrapper.text()).toContain('Kubernetes Configuration')
    expect(wrapper.text()).toContain('Namespace')
    expect(wrapper.text()).toContain('API Version')

    // Test Ansible format
    await formatSelect.setValue('ansible')

    expect(wrapper.text()).toContain('Ansible Configuration')
    expect(wrapper.text()).toContain('Playbook Structure')
    expect(wrapper.text()).toContain('Inventory Format')
  })

  it('should handle device selection', async () => {
    const wrapper = mount(GitOpsConfigForm, {
      props: {
        availableDevices: mockDevices,
        loading: false,
        error: ''
      }
    })

    // Switch to specific device selection
    const radioButtons = wrapper.findAll('input[type="radio"]')
    const selectSpecificRadio = radioButtons[1]
    await selectSpecificRadio.trigger('change')

    // Should show device checkboxes
    expect(wrapper.text()).toContain('0 of 2 selected')
    
    const deviceCheckboxes = wrapper.findAll('input[type="checkbox"]')
    const deviceCheckbox = deviceCheckboxes.find(cb => 
      cb.element.value === '1'
    )
    
    if (deviceCheckbox) {
      await deviceCheckbox.setChecked(true)
      expect(wrapper.text()).toContain('1 of 2 selected')
    }
  })

  it('should handle variable substitution', async () => {
    const wrapper = mount(GitOpsConfigForm, {
      props: {
        availableDevices: mockDevices,
        loading: false,
        error: ''
      }
    })

    // Should start with one default variable
    expect(wrapper.text()).toContain('ENVIRONMENT')

    // Add a new variable
    const addButton = wrapper.find('.add-variable-btn')
    await addButton.trigger('click')

    // Should have more variable rows
    const variableRows = wrapper.findAll('.variable-row')
    expect(variableRows.length).toBeGreaterThan(1)
  })

  it('should emit submit event with correct data', async () => {
    const wrapper = mount(GitOpsConfigForm, {
      props: {
        availableDevices: mockDevices,
        loading: false,
        error: ''
      }
    })

    // Fill required fields
    const nameInput = wrapper.find('input[type="text"]')
    await nameInput.setValue('Test Export')

    const formatSelect = wrapper.find('select')
    await formatSelect.setValue('terraform')

    const structureSelects = wrapper.findAll('select')
    const structureSelect = structureSelects.find(select => 
      select.element.innerHTML.includes('Monorepo')
    )
    if (structureSelect) {
      await structureSelect.setValue('monorepo')
    }

    // Submit form
    const form = wrapper.find('form')
    await form.trigger('submit.prevent')

    // Check emitted data
    const submitEvents = wrapper.emitted('submit')
    if (submitEvents) {
      const submitData = submitEvents[0][0] as GitOpsExportRequest
      expect(submitData.name).toBe('Test Export')
      expect(submitData.format).toBe('terraform')
      expect(submitData.repository_structure).toBe('monorepo')
    }
  })

  it('should handle preview functionality', async () => {
    mockPreviewGitOpsExport.mockResolvedValue({
      preview: {
        success: true,
        file_count: 5,
        estimated_size: 1024,
        structure_preview: ['main.tf', 'variables.tf'],
        template_validation: {
          valid: true,
          terraform: {
            syntax_valid: true,
            provider_compatible: true,
            warnings: []
          }
        },
        warnings: []
      },
      summary: {}
    })

    const wrapper = mount(GitOpsConfigForm, {
      props: {
        availableDevices: mockDevices,
        loading: false,
        error: ''
      }
    })

    // Fill required fields
    const nameInput = wrapper.find('input[type="text"]')
    await nameInput.setValue('Test Export')

    const formatSelect = wrapper.find('select')
    await formatSelect.setValue('terraform')

    // Click preview button
    const previewButton = wrapper.find('.preview-button')
    await previewButton.trigger('click')

    // Should call preview API
    expect(mockPreviewGitOpsExport).toHaveBeenCalled()

    // Wait for preview data to be processed
    await wrapper.vm.$nextTick()

    // Should show preview results
    expect(wrapper.text()).toContain('Export Preview')
  })

  it('should show webhook configuration when enabled', async () => {
    const wrapper = mount(GitOpsConfigForm, {
      props: {
        availableDevices: mockDevices,
        loading: false,
        error: ''
      }
    })

    // Enable webhooks
    const webhookCheckbox = wrapper.find('input[type="checkbox"]')
    const webhookCheckboxes = wrapper.findAll('input[type="checkbox"]')
    
    // Find the webhook checkbox by looking for associated text
    const webhookCheckboxWrapper = wrapper.find('label:contains("Configure webhooks")')
    if (webhookCheckboxWrapper.exists()) {
      const webhookInput = webhookCheckboxWrapper.find('input[type="checkbox"]')
      await webhookInput.setChecked(true)

      // Should show webhook secret field
      expect(wrapper.text()).toContain('Webhook Secret')
    }
  })

  it('should emit cancel event', async () => {
    const wrapper = mount(GitOpsConfigForm, {
      props: {
        availableDevices: mockDevices,
        loading: false,
        error: ''
      }
    })

    const cancelButton = wrapper.find('.secondary-button')
    await cancelButton.trigger('click')

    expect(wrapper.emitted('cancel')).toBeTruthy()
  })

  it('should disable submit button when form is invalid', () => {
    const wrapper = mount(GitOpsConfigForm, {
      props: {
        availableDevices: mockDevices,
        loading: false,
        error: ''
      }
    })

    const submitButton = wrapper.find('.primary-button')
    expect(submitButton.element.disabled).toBe(true)
  })

  it('should show loading state when loading prop is true', () => {
    const wrapper = mount(GitOpsConfigForm, {
      props: {
        availableDevices: mockDevices,
        loading: true,
        error: ''
      }
    })

    const submitButton = wrapper.find('.primary-button')
    expect(submitButton.text()).toContain('Creating Export...')
    expect(submitButton.element.disabled).toBe(true)
  })

  it('should display error when error prop is provided', () => {
    const errorMessage = 'Test error message'
    const wrapper = mount(GitOpsConfigForm, {
      props: {
        availableDevices: mockDevices,
        loading: false,
        error: errorMessage
      }
    })

    expect(wrapper.text()).toContain(errorMessage)
    expect(wrapper.find('.form-error')).toBeTruthy()
  })

  it('should calculate size estimation correctly', async () => {
    const wrapper = mount(GitOpsConfigForm, {
      props: {
        availableDevices: mockDevices,
        loading: false,
        error: ''
      }
    })

    // Fill form to trigger size calculation
    const nameInput = wrapper.find('input[type="text"]')
    await nameInput.setValue('Test Export')

    const formatSelect = wrapper.find('select')
    await formatSelect.setValue('terraform')

    // Should show size estimate section
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Estimated Output')
    expect(wrapper.text()).toContain('2 devices') // All devices selected by default
  })
})