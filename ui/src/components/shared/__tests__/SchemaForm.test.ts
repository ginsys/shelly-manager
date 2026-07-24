import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import SchemaForm from '../SchemaForm.vue'
import { initialPluginConfig } from '@/utils/plugin-schema'
import type { PluginSchema } from '@/api/plugin'

const schema: PluginSchema = {
  version: '1',
  required: ['endpoint', 'enabled'],
  properties: {
    endpoint: { type: 'string', description: 'API endpoint', sensitive: true },
    enabled: { type: 'boolean', description: 'Enable integration' },
    retries: { type: 'number', description: 'Retry count', minimum: 0, maximum: 5 },
    mode: { type: 'string', description: 'Mode', enum: ['safe', 1] },
    payload: { type: 'object', description: 'JSON payload', default: {} },
  },
}

describe('schema form', () => {
  it('renders backend-shaped fields and accessible errors', async () => {
    const wrapper = mount(SchemaForm, {
      props: { schema, values: initialPluginConfig(schema), touched: {}, showAllErrors: true },
    })
    expect(wrapper.find('input[type="password"]').exists()).toBe(true)
    expect(wrapper.find('input[type="checkbox"]').exists()).toBe(true)
    expect(wrapper.find('select').exists()).toBe(true)
    expect(wrapper.find('textarea').element.value).toBe('{}')
    expect(wrapper.find('[aria-invalid="true"]').exists()).toBe(true)
  })

  it('preserves zero when a number is entered', async () => {
    const wrapper = mount(SchemaForm, {
      props: { schema, values: initialPluginConfig(schema), touched: {} },
    })
    await wrapper.find('input[type="number"]').setValue('0')
    expect(wrapper.emitted('update:values')?.[0]?.[0]).toMatchObject({ retries: 0 })
  })
})
