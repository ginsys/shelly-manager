import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import SchemaForm from '../SchemaForm.vue'
import type { FieldSchema } from '@/types/schema'

describe('SchemaForm', () => {
  it('renders text input fields', () => {
    const schema: FieldSchema = {
      username: {
        type: 'string',
        label: 'Username',
        placeholder: 'Enter username',
        required: true
      }
    }

    const wrapper = mount(SchemaForm, {
      props: {
        schema,
        modelValue: { username: '' }
      }
    })

    expect(wrapper.find('.field-label').text()).toContain('Username')
    expect(wrapper.find('.required-indicator').exists()).toBe(true)
    expect(wrapper.find('input[type="text"]').exists()).toBe(true)
    expect(wrapper.find('input').attributes('placeholder')).toBe('Enter username')
  })

  it('renders number input fields', () => {
    const schema: FieldSchema = {
      port: {
        type: 'number',
        label: 'Port',
        min: 1,
        max: 65535
      }
    }

    const wrapper = mount(SchemaForm, {
      props: {
        schema,
        modelValue: { port: 8080 }
      }
    })

    const input = wrapper.find('input[type="number"]')
    expect(input.exists()).toBe(true)
    expect(input.attributes('min')).toBe('1')
    expect(input.attributes('max')).toBe('65535')
  })

  it('renders checkbox fields', () => {
    const schema: FieldSchema = {
      enabled: {
        type: 'boolean',
        label: 'Enabled',
        placeholder: 'Enable feature'
      }
    }

    const wrapper = mount(SchemaForm, {
      props: {
        schema,
        modelValue: { enabled: false }
      }
    })

    expect(wrapper.find('input[type="checkbox"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('Enable feature')
  })

  it('renders select dropdown fields', () => {
    const schema: FieldSchema = {
      format: {
        type: 'select',
        label: 'Format',
        options: [
          { value: 'json', label: 'JSON' },
          { value: 'yaml', label: 'YAML' },
          { value: 'xml', label: 'XML' }
        ]
      }
    }

    const wrapper = mount(SchemaForm, {
      props: {
        schema,
        modelValue: { format: 'json' }
      }
    })

    const select = wrapper.find('select')
    expect(select.exists()).toBe(true)
    expect(select.findAll('option')).toHaveLength(4) // 3 options + placeholder
    expect(select.findAll('option')[1].text()).toBe('JSON')
  })

  it('renders textarea fields', () => {
    const schema: FieldSchema = {
      description: {
        type: 'textarea',
        label: 'Description',
        placeholder: 'Enter description'
      }
    }

    const wrapper = mount(SchemaForm, {
      props: {
        schema,
        modelValue: { description: '' }
      }
    })

    expect(wrapper.find('textarea').exists()).toBe(true)
    expect(wrapper.find('textarea').attributes('placeholder')).toBe('Enter description')
  })

  it('shows field descriptions', () => {
    const schema: FieldSchema = {
      apiKey: {
        type: 'string',
        label: 'API Key',
        description: 'Your API key for authentication'
      }
    }

    const wrapper = mount(SchemaForm, {
      props: {
        schema,
        modelValue: { apiKey: '' }
      }
    })

    expect(wrapper.find('.field-description').text()).toBe('Your API key for authentication')
  })

  it('emits update:modelValue when field changes', async () => {
    const schema: FieldSchema = {
      name: { type: 'string', label: 'Name' }
    }

    const wrapper = mount(SchemaForm, {
      props: {
        schema,
        modelValue: { name: '' }
      }
    })

    const input = wrapper.find('input')
    await input.setValue('test-value')

    expect(wrapper.emitted('update:modelValue')).toBeTruthy()
    expect(wrapper.emitted('update:modelValue')![0]).toEqual([{ name: 'test-value' }])
  })

  it('renders multiple fields', () => {
    const schema: FieldSchema = {
      host: { type: 'string', label: 'Host' },
      port: { type: 'number', label: 'Port' },
      ssl: { type: 'boolean', label: 'Use SSL' }
    }

    const wrapper = mount(SchemaForm, {
      props: {
        schema,
        modelValue: { host: '', port: 443, ssl: true }
      }
    })

    expect(wrapper.findAll('.form-field')).toHaveLength(3)
    expect(wrapper.findAll('.field-label')[0].text()).toContain('Host')
    expect(wrapper.findAll('.field-label')[1].text()).toContain('Port')
    expect(wrapper.findAll('.field-label')[2].text()).toContain('Use SSL')
  })

  it('uses field key as label when label is not provided', () => {
    const schema: FieldSchema = {
      hostname: { type: 'string' }
    }

    const wrapper = mount(SchemaForm, {
      props: {
        schema,
        modelValue: { hostname: '' }
      }
    })

    expect(wrapper.find('.field-label').text()).toContain('hostname')
  })
})
