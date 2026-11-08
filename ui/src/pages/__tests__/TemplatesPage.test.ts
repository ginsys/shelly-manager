import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import type { ConfigSchema } from '@/api/typedConfig'

vi.mock('@/api/typedConfig', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/typedConfig')>()
  return { ...actual, getConfigSchema: vi.fn() }
})

vi.mock('@/api/templates', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/templates')>()
  return {
    ...actual,
    listTemplates: vi.fn(),
    createTemplate: vi.fn(),
    updateTemplate: vi.fn(),
  }
})

import { getConfigSchema } from '@/api/typedConfig'
import { createTemplate, listTemplates, updateTemplate, TemplateSaveError } from '@/api/templates'
import TemplatesPage from '../TemplatesPage.vue'

// A cut-down template schema, in the shape GET /config/schema returns.
const schema: ConfigSchema = {
  type: 'object',
  'x-apply-supported': true,
  properties: {
    power_metering: {
      type: 'object',
      title: 'Power Metering',
      properties: {
        max_power: { type: 'integer', minimum: 0, default: 2500, 'x-applicable': true },
      },
    },
  },
}

function mountPage() {
  return mount(TemplatesPage, { global: { stubs: { 'router-link': true } } })
}

async function openCreate(wrapper: ReturnType<typeof mountPage>) {
  await wrapper.find('.toolbar .btn.primary').trigger('click')
  await flushPromises()
}

async function openSection(wrapper: ReturnType<typeof mountPage>, section: string) {
  if (!wrapper.find(`[data-section="${section}"] .section-content`).exists()) {
    await wrapper.find(`[data-section="${section}"] .section-header`).trigger('click')
  }
}

describe('TemplatesPage template form', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(getConfigSchema).mockReset()
    vi.mocked(getConfigSchema).mockResolvedValue(schema)
    vi.mocked(listTemplates).mockReset()
    vi.mocked(listTemplates).mockResolvedValue([])
    vi.mocked(createTemplate).mockReset()
    vi.mocked(updateTemplate).mockReset()
  })

  it('a new global template gets the template schema and starts empty', async () => {
    const wrapper = mountPage()
    await flushPromises()
    await openCreate(wrapper)
    expect(getConfigSchema).toHaveBeenCalledWith(undefined, undefined, 'global')
    await openSection(wrapper, 'power_metering')
    // Empty start: the field holds no value, and the default is offered.
    expect((wrapper.find('[data-path="power_metering.max_power"] input').element as HTMLInputElement).value).toBe('')
    expect(wrapper.find('[data-path="power_metering.max_power"] [data-action="use-default"]').exists()).toBe(true)
  })

  it('saves only the fields set in the form', async () => {
    vi.mocked(createTemplate).mockResolvedValue({
      id: 1, name: 'plugs', scope: 'global', config: {}, created_at: '', updated_at: '',
    })
    const wrapper = mountPage()
    await flushPromises()
    await openCreate(wrapper)
    await wrapper.find('.modal .form-input[type="text"]').setValue('plugs')
    await openSection(wrapper, 'power_metering')
    await wrapper.find('[data-path="power_metering.max_power"] [data-action="use-default"]').trigger('click')
    await wrapper.find('[data-testid="save-template"]').trigger('click')
    await flushPromises()
    expect(createTemplate).toHaveBeenCalledWith({
      name: 'plugs',
      description: '',
      scope: 'global',
      device_type: undefined,
      config: { power_metering: { max_power: 2500 } },
    })
  })

  it('a device_type template loads the schema of its model once the type is entered', async () => {
    const wrapper = mountPage()
    await flushPromises()
    await openCreate(wrapper)
    await wrapper.find('[data-testid="template-scope"]').setValue('device_type')
    expect(wrapper.find('[data-testid="template-needs-type"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="save-template"]').attributes('disabled')).toBeDefined()

    vi.mocked(getConfigSchema).mockClear()
    const input = wrapper.find('[data-testid="template-device-type"]')
    await input.setValue('SHPLG-S') // setValue fires input, then change
    await flushPromises()
    expect(getConfigSchema).toHaveBeenCalledTimes(1)
    expect(getConfigSchema).toHaveBeenCalledWith('SHPLG-S', undefined, undefined)
    expect(wrapper.find('[data-testid="template-needs-type"]').exists()).toBe(false)
  })

  it('an invalid field blocks save', async () => {
    const wrapper = mountPage()
    await flushPromises()
    await openCreate(wrapper)
    await openSection(wrapper, 'power_metering')
    await wrapper.find('[data-path="power_metering.max_power"] input').setValue('-1')
    expect(wrapper.find('[data-testid="save-template"]').attributes('disabled')).toBeDefined()
  })

  it('shows each field the server refused', async () => {
    vi.mocked(createTemplate).mockRejectedValue(
      new TemplateSaveError('Validation failed', [{ path: 'roller', message: 'SHPLG-S has no roller settings' }]),
    )
    const wrapper = mountPage()
    await flushPromises()
    await openCreate(wrapper)
    await wrapper.find('[data-testid="save-template"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="template-save-errors"]').text()).toContain('roller: SHPLG-S has no roller settings')
    expect(wrapper.find('.modal').exists()).toBe(true)
  })
})
