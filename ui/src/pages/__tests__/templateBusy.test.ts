import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
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
    listTemplates: vi.fn(),
    createTemplate: vi.fn(),
    updateTemplate: vi.fn(),
    deleteTemplate: vi.fn(),
    getTemplate: vi.fn(),
    assignTemplateToDevice: vi.fn(),
  }
})

import { getConfigSchema } from '@/api/typedConfig'
import { deleteTemplate, getTemplate, listTemplates, updateTemplate } from '@/api/templates'
import TemplatesPage from '../TemplatesPage.vue'
import TemplateDetailPage from '../TemplateDetailPage.vue'

// One rule for both template pages: while a save or a refresh is pending, no action opens, copies
// or sends a template. Cancel is the way out and stays.

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

const tpl100 = {
  id: 1, name: 'plugs', description: '', scope: 'global', created_at: '', updated_at: '',
  config: { power_metering: { max_power: 100 } },
}
const tpl200 = { ...tpl100, config: { power_metering: { max_power: 200 } } }

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason: unknown) => void
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}

let confirmSpy: ReturnType<typeof vi.spyOn>
let writeText: ReturnType<typeof vi.fn>

beforeEach(() => {
  setActivePinia(createPinia())
  vi.mocked(getConfigSchema).mockReset()
  vi.mocked(getConfigSchema).mockResolvedValue(schema)
  for (const fn of [listTemplates, updateTemplate, deleteTemplate, getTemplate]) vi.mocked(fn).mockReset()
  confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
  writeText = vi.fn().mockResolvedValue(undefined)
  Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
})

afterEach(() => {
  confirmSpy.mockRestore()
})

describe('TemplatesPage is busy while a save or a refresh is pending', () => {
  const mountPage = () => mount(TemplatesPage, { global: { stubs: { 'router-link': true } } })
  const rowButton = (wrapper: ReturnType<typeof mountPage>, label: string) =>
    wrapper.findAll('.action-btn').find(b => b.text() === label)

  // The sequence of the issue: Save (the PUT is pending), Cancel, change the scope filter, and let
  // the filter's GET finish. The save is still pending.
  async function pendingSave() {
    vi.mocked(listTemplates).mockResolvedValue([tpl100 as any])
    const save = deferred<any>()
    vi.mocked(updateTemplate).mockReturnValueOnce(save.promise)
    const wrapper = mountPage()
    await flushPromises()
    await rowButton(wrapper, 'Edit')!.trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid="save-template"]').trigger('click')
    await wrapper.findAll('.modal-actions .btn').find(b => b.text() === 'Cancel')!.trigger('click')
    const filterGet = deferred<any[]>()
    vi.mocked(listTemplates).mockReturnValueOnce(filterGet.promise)
    await wrapper.find('.filters select').setValue('global')
    filterGet.resolve([tpl100])
    await flushPromises()
    return { wrapper, save }
  }

  const rows: Array<[string, (w: ReturnType<typeof mountPage>) => Promise<void>, () => void]> = [
    ['row Edit', async (w) => { await rowButton(w, 'Edit')?.trigger('click'); await flushPromises() },
      () => {}],
    ['row Delete', async (w) => { await rowButton(w, 'Delete')?.trigger('click'); await flushPromises() },
      () => expect(deleteTemplate).not.toHaveBeenCalled()],
    ['Create', async (w) => { await w.find('.toolbar .btn.primary').trigger('click'); await flushPromises() },
      () => {}],
  ]

  it.each(rows)('%s starts nothing while the save is pending', async (_name, act, check) => {
    const { wrapper } = await pendingSave()
    await act(wrapper)
    expect(wrapper.find('.modal').exists()).toBe(false)
    check()
  })

  it('the scope filter starts no request while the save is pending', async () => {
    const { wrapper } = await pendingSave()
    // The initial load, and nothing from the filter change.
    expect(vi.mocked(listTemplates)).toHaveBeenCalledTimes(1)
    expect(wrapper.find('.filters select').attributes('disabled')).toBeDefined()
  })

  it('every action is back when the save and its refresh have finished', async () => {
    const { wrapper, save } = await pendingSave()
    save.resolve({ template: tpl200 })
    await flushPromises()
    expect(rowButton(wrapper, 'Edit')!.attributes('disabled')).toBeUndefined()
    expect(rowButton(wrapper, 'Delete')!.attributes('disabled')).toBeUndefined()
    expect(wrapper.find('.toolbar .btn.primary').attributes('disabled')).toBeUndefined()
    expect(wrapper.find('.filters select').attributes('disabled')).toBeUndefined()
  })
})

describe('TemplateDetailPage is busy while a save or a refresh is pending', () => {
  const mountPage = () => mount(TemplateDetailPage, { global: { stubs: { 'router-link': true } } })
  const toolbar = (wrapper: ReturnType<typeof mountPage>, label: string) =>
    wrapper.findAll('.toolbar .btn').find(b => b.text() === label)
  const copy = (wrapper: ReturnType<typeof mountPage>) =>
    wrapper.findAll('.header-row .btn').find(b => b.text() === 'Copy')

  // Edit, Save, Cancel before the answer: the page shows the saved body (100) while the PUT runs.
  async function pendingSave() {
    vi.mocked(getTemplate).mockResolvedValue(tpl100 as any)
    const save = deferred<any>()
    vi.mocked(updateTemplate).mockReturnValueOnce(save.promise)
    const wrapper = mountPage()
    await flushPromises()
    await toolbar(wrapper, 'Edit')!.trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid="save-template"]').trigger('click')
    await toolbar(wrapper, 'Cancel')!.trigger('click')
    return { wrapper, save }
  }

  it('Edit is off while the save is pending', async () => {
    const { wrapper } = await pendingSave()
    expect(toolbar(wrapper, 'Edit')!.attributes('disabled')).toBeDefined()
  })

  it('Delete is off while the save is pending', async () => {
    const { wrapper } = await pendingSave()
    await wrapper.find('.btn.danger').trigger('click')
    expect(confirmSpy).not.toHaveBeenCalled()
    expect(deleteTemplate).not.toHaveBeenCalled()
  })

  // The page still shows 100 while the PUT carries 200. A copy taken now is the old body.
  it('Copy copies nothing while the save is pending', async () => {
    const { wrapper } = await pendingSave()
    expect(copy(wrapper)!.attributes('disabled')).toBeDefined()
    await copy(wrapper)!.trigger('click')
    expect(writeText).not.toHaveBeenCalled()
  })

  it('Copy is back when the save and its refresh have finished', async () => {
    const { wrapper, save } = await pendingSave()
    vi.mocked(getTemplate).mockResolvedValue(tpl200 as any)
    save.resolve({ template: tpl200 })
    await flushPromises()
    await copy(wrapper)!.trigger('click')
    expect(JSON.parse(writeText.mock.calls[0][0])).toEqual(tpl200.config)
  })

  it('Save is off while the save is pending', async () => {
    vi.mocked(getTemplate).mockResolvedValue(tpl100 as any)
    vi.mocked(updateTemplate).mockReturnValueOnce(deferred<any>().promise)
    const wrapper = mountPage()
    await flushPromises()
    await toolbar(wrapper, 'Edit')!.trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid="save-template"]').trigger('click')
    expect(wrapper.find('[data-testid="save-template"]').attributes('disabled')).toBeDefined()
  })

  it('a failed refresh shows Retry, and Retry brings the template and Edit back', async () => {
    vi.mocked(getTemplate)
      .mockResolvedValueOnce(tpl100 as any)
      .mockRejectedValueOnce(new Error('refresh down'))
      .mockResolvedValueOnce(tpl200 as any)
    vi.mocked(updateTemplate).mockResolvedValueOnce({ template: tpl200 } as any)
    const wrapper = mountPage()
    await flushPromises()
    await toolbar(wrapper, 'Edit')!.trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid="save-template"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('refresh down')
    expect(toolbar(wrapper, 'Edit')!.attributes('disabled')).toBeDefined()
    await wrapper.find('[data-testid="retry-load"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).not.toContain('refresh down')
    expect(wrapper.find('[data-testid="retry-load"]').exists()).toBe(false)
    expect(toolbar(wrapper, 'Edit')!.attributes('disabled')).toBeUndefined()
    // The retry showed the server's copy, not the one from before the save.
    await copy(wrapper)!.trigger('click')
    expect(JSON.parse(writeText.mock.calls[0][0])).toEqual(tpl200.config)
  })

  it('Retry shows only for a failed load, not for a healthy page', async () => {
    vi.mocked(getTemplate).mockResolvedValue(tpl100 as any)
    const wrapper = mountPage()
    await flushPromises()
    expect(wrapper.find('[data-testid="retry-load"]').exists()).toBe(false)
  })
})
