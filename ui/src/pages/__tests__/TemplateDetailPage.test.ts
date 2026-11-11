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

  const editDisabled = (wrapper: ReturnType<typeof mountPage>) =>
    toolbarButton(wrapper, 'Edit').attributes('disabled') !== undefined

  async function startSaveThenCancel(wrapper: ReturnType<typeof mountPage>) {
    await flushPromises()
    await toolbarButton(wrapper, 'Edit').trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid="save-template"]').trigger('click')
    await toolbarButton(wrapper, 'Cancel').trigger('click')
  }

  // Save, then Cancel before the answer: the answer belongs to the cancelled edit. Edit stays off
  // until the save has settled, so no new edit can be open when the answer arrives.
  it('a save error that arrives after the edit was cancelled shows nowhere', async () => {
    const pending = deferred<any>()
    vi.mocked(updateTemplate).mockReturnValueOnce(pending.promise)
    const wrapper = mountPage()
    await startSaveThenCancel(wrapper)
    expect(editDisabled(wrapper)).toBe(true)
    pending.reject(refusal())
    await flushPromises()
    expect(editDisabled(wrapper)).toBe(false)
    expect(wrapper.find('[data-testid="template-save-errors"]').exists()).toBe(false)
    await toolbarButton(wrapper, 'Edit').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="template-save-errors"]').exists()).toBe(false)
  })

  it('a save that finishes after the edit was cancelled leaves the page out of edit mode', async () => {
    const pending = deferred<any>()
    vi.mocked(updateTemplate).mockReturnValueOnce(pending.promise)
    const wrapper = mountPage()
    await startSaveThenCancel(wrapper)
    pending.resolve({ template: stored })
    await flushPromises()
    expect(wrapper.find('[data-testid="save-template"]').exists()).toBe(false)
    expect(editDisabled(wrapper)).toBe(false)
  })

  // Saved 200 over a stored 100, then Cancel. While the refresh after the save is pending, Edit
  // must not start: it would copy 100, the refresh would then show 200, and an unchanged Save would
  // send 100 and undo the saved change.
  it('an edit cannot start while the refresh after a cancelled save is pending', async () => {
    const stored100 = { ...stored, config: { power_metering: { max_power: 100 } } }
    const stored200 = { ...stored, config: { power_metering: { max_power: 200 } } }
    const refresh = deferred<any>()
    vi.mocked(getTemplate).mockResolvedValueOnce(stored100 as any).mockReturnValueOnce(refresh.promise)
    const save = deferred<any>()
    vi.mocked(updateTemplate).mockReturnValueOnce(save.promise).mockResolvedValue({ template: stored200 } as any)
    const wrapper = mountPage()
    await startSaveThenCancel(wrapper)
    save.resolve({ template: stored200 })
    await flushPromises()
    // The refresh GET is pending. Try Edit anyway.
    await toolbarButton(wrapper, 'Edit').trigger('click')
    await flushPromises()
    refresh.resolve(stored200)
    await flushPromises()
    const edit = wrapper.findAll('.toolbar .btn').find(b => b.text() === 'Edit')
    if (edit) await edit.trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid="save-template"]').trigger('click')
    await flushPromises()
    const second = vi.mocked(updateTemplate).mock.calls[1]
    expect(second[1]).not.toHaveProperty('config')
  })

  it('Edit comes back when the refresh after a save fails', async () => {
    vi.mocked(getTemplate).mockResolvedValueOnce(stored as any).mockRejectedValueOnce(new Error('refresh down'))
    const save = deferred<any>()
    vi.mocked(updateTemplate).mockReturnValueOnce(save.promise)
    const wrapper = mountPage()
    await startSaveThenCancel(wrapper)
    expect(editDisabled(wrapper)).toBe(true)
    save.resolve({ template: stored })
    await flushPromises()
    expect(editDisabled(wrapper)).toBe(false)
    await toolbarButton(wrapper, 'Edit').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="save-template"]').exists()).toBe(true)
  })

  it('Edit comes back when the save itself fails', async () => {
    const save = deferred<any>()
    vi.mocked(updateTemplate).mockReturnValueOnce(save.promise)
    const alertSpy = vi.spyOn(window, 'alert').mockImplementation(() => {})
    const wrapper = mountPage()
    await startSaveThenCancel(wrapper)
    save.reject(new Error('server down'))
    await flushPromises()
    expect(editDisabled(wrapper)).toBe(false)
    alertSpy.mockRestore()
  })
})
