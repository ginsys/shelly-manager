import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import type { ConfigSchema } from '@/api/typedConfig'

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: '1' } }),
  useRouter: () => ({ push: vi.fn() }),
}))

vi.mock('@/api/typedConfig', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/typedConfig')>()
  return { ...actual, getConfigSchema: vi.fn() }
})

vi.mock('@/api/templates', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/templates')>()
  return {
    ...actual,
    getTemplate: vi.fn(),
    updateTemplate: vi.fn(),
    deleteTemplate: vi.fn(),
    assignTemplateToDevice: vi.fn(),
  }
})

import { getConfigSchema } from '@/api/typedConfig'
import { getTemplate, updateTemplate, TemplateSaveError } from '@/api/templates'
import TemplateDetailPage from '../TemplateDetailPage.vue'

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

const stored = {
  id: 1, name: 'plugs', description: '', scope: 'global', created_at: '', updated_at: '', config: {},
}

function mountPage() {
  return mount(TemplateDetailPage, { global: { stubs: { 'router-link': true } } })
}

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason: unknown) => void
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}

const toolbarButton = (wrapper: ReturnType<typeof mountPage>, label: string) =>
  wrapper.findAll('.toolbar .btn').find(b => b.text() === label)!

const refusal = () => new TemplateSaveError('Validation failed',
  [{ path: 'relay.auto_of', message: 'Unknown key "auto_of"' }])

describe('TemplateDetailPage template form', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(getConfigSchema).mockReset()
    vi.mocked(getConfigSchema).mockResolvedValue(schema)
    vi.mocked(getTemplate).mockReset()
    vi.mocked(getTemplate).mockResolvedValue(stored as any)
    vi.mocked(updateTemplate).mockReset()
  })

  it('shows the error of a refused save while that edit stays open', async () => {
    vi.mocked(updateTemplate).mockRejectedValue(refusal())
    const wrapper = mountPage()
    await flushPromises()
    await toolbarButton(wrapper, 'Edit').trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid="save-template"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="template-save-errors"]').text()).toContain('Unknown key "auto_of"')
  })

  // Save, Cancel and Edit again before the answer: the answer belongs to the first edit.
  it('a save error that arrives after the edit was cancelled does not show in the next edit', async () => {
    const pending = deferred<any>()
    vi.mocked(updateTemplate).mockReturnValueOnce(pending.promise)
    const wrapper = mountPage()
    await flushPromises()
    await toolbarButton(wrapper, 'Edit').trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid="save-template"]').trigger('click')
    await toolbarButton(wrapper, 'Cancel').trigger('click')
    await toolbarButton(wrapper, 'Edit').trigger('click')
    await flushPromises()
    pending.reject(refusal())
    await flushPromises()
    expect(wrapper.find('[data-testid="template-save-errors"]').exists()).toBe(false)
  })

  it('a save that finishes after the edit was cancelled does not close the next edit', async () => {
    const pending = deferred<any>()
    vi.mocked(updateTemplate).mockReturnValueOnce(pending.promise)
    const wrapper = mountPage()
    await flushPromises()
    await toolbarButton(wrapper, 'Edit').trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid="save-template"]').trigger('click')
    await toolbarButton(wrapper, 'Cancel').trigger('click')
    await toolbarButton(wrapper, 'Edit').trigger('click')
    await flushPromises()
    pending.resolve({ template: stored })
    await flushPromises()
    expect(wrapper.find('[data-testid="save-template"]').exists()).toBe(true)
  })
})
