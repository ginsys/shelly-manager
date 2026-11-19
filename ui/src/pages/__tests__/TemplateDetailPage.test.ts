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
    getTemplate: vi.fn(),
    updateTemplate: vi.fn(),
    deleteTemplate: vi.fn(),
    assignTemplateToDevice: vi.fn(),
  }
})

import { getConfigSchema } from '@/api/typedConfig'
import {
  assignTemplateToDevice,
  deleteTemplate,
  getTemplate,
  updateTemplate,
  TemplateSaveError,
} from '@/api/templates'
import { dismissNotice, notice } from '@/composables/useNotice'
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
    vi.mocked(deleteTemplate).mockReset()
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

  // Copy copies the config on screen: in edit mode that is the body being edited.
  it('Copy copies the edited body in edit mode and the saved one otherwise', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
    vi.mocked(getTemplate).mockResolvedValue({ ...stored, config: { power_metering: { max_power: 100 } } } as any)
    const wrapper = mountPage()
    await flushPromises()
    const copy = () => wrapper.findAll('.header-row .btn').find(b => b.text() === 'Copy')!.trigger('click')
    await copy()
    expect(JSON.parse(writeText.mock.calls[0][0])).toEqual({ power_metering: { max_power: 100 } })
    await toolbarButton(wrapper, 'Edit').trigger('click')
    await flushPromises()
    if (!wrapper.find('[data-section="power_metering"] .section-content').exists()) {
      await wrapper.find('[data-section="power_metering"] .section-header').trigger('click')
    }
    await wrapper.find('[data-path="power_metering.max_power"] input').setValue('1400')
    await copy()
    expect(JSON.parse(writeText.mock.calls[1][0])).toEqual({ power_metering: { max_power: 1400 } })
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
  it('a save refusal that arrives after the edit was cancelled leaves no field list behind', async () => {
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

  // The refresh failed, so the page holds the template as it was before the save and shows an
  // error instead of it. An edit would start from that old copy and its Save would undo the saved
  // change, so Edit stays off while the error shows.
  it('Edit stays off when the refresh after a save fails', async () => {
    vi.mocked(getTemplate).mockResolvedValueOnce(stored as any).mockRejectedValueOnce(new Error('refresh down'))
    const save = deferred<any>()
    vi.mocked(updateTemplate).mockReturnValueOnce(save.promise)
    const wrapper = mountPage()
    await startSaveThenCancel(wrapper)
    expect(editDisabled(wrapper)).toBe(true)
    save.resolve({ template: stored })
    await flushPromises()
    expect(wrapper.text()).toContain('refresh down')
    expect(editDisabled(wrapper)).toBe(true)
    await toolbarButton(wrapper, 'Edit').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="save-template"]').exists()).toBe(false)
  })

  // Two Save clicks: the first failing must not release Edit while the second still runs.
  it('a second Save click while a save is pending sends nothing', async () => {
    const first = deferred<any>()
    vi.mocked(updateTemplate).mockReturnValueOnce(first.promise)
    const wrapper = mountPage()
    await flushPromises()
    await toolbarButton(wrapper, 'Edit').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="save-template"]').attributes('disabled')).toBeUndefined()
    await wrapper.find('[data-testid="save-template"]').trigger('click')
    expect(wrapper.find('[data-testid="save-template"]').attributes('disabled')).toBeDefined()
    await wrapper.find('[data-testid="save-template"]').trigger('click')
    expect(vi.mocked(updateTemplate)).toHaveBeenCalledTimes(1)
    first.reject(refusal())
    await flushPromises()
    expect(wrapper.find('[data-testid="save-template"]').attributes('disabled')).toBeUndefined()
  })

  it('a save that succeeds in the same edit leaves edit mode and shows the refreshed template', async () => {
    const renamed = { ...stored, name: 'renamed' }
    vi.mocked(getTemplate).mockResolvedValueOnce(stored as any).mockResolvedValueOnce(renamed as any)
    vi.mocked(updateTemplate).mockResolvedValueOnce({ template: renamed, affected_devices: 3 } as any)
    const wrapper = mountPage()
    await flushPromises()
    await toolbarButton(wrapper, 'Edit').trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid="save-template"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="save-template"]').exists()).toBe(false)
    expect(wrapper.find('.title').text()).toBe('renamed')
    expect(editDisabled(wrapper)).toBe(false)
  })

  it('Delete is off while a save is pending and does nothing', async () => {
    const save = deferred<any>()
    vi.mocked(updateTemplate).mockReturnValueOnce(save.promise)
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
    const wrapper = mountPage()
    await flushPromises()
    await toolbarButton(wrapper, 'Edit').trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid="save-template"]').trigger('click')
    const del = wrapper.find('.btn.danger')
    expect(del.attributes('disabled')).toBeDefined()
    await del.trigger('click')
    expect(confirmSpy).not.toHaveBeenCalled()
    expect(vi.mocked(deleteTemplate)).not.toHaveBeenCalled()
    save.reject(refusal())
    await flushPromises()
    expect(wrapper.find('.btn.danger').attributes('disabled')).toBeUndefined()
    confirmSpy.mockRestore()
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

// ginsys/shelly-manager#614: every outcome shows on the page, never in a browser alert.
describe('TemplateDetailPage outcomes of its actions', () => {
  let alertSpy: ReturnType<typeof vi.spyOn>
  const panel = (wrapper: ReturnType<typeof mountPage>) => wrapper.find('[data-testid="template-action-error"]')
  const statusLine = (wrapper: ReturnType<typeof mountPage>) => wrapper.find('[data-testid="template-status"]')
  const openAssign = async (wrapper: ReturnType<typeof mountPage>) => {
    await wrapper.findAll('.header-row .btn').find(b => b.text() === 'Assign')!.trigger('click')
  }
  const submitAssign = async (wrapper: ReturnType<typeof mountPage>) => {
    await wrapper.find('.modal-actions .btn.primary').trigger('click')
    await flushPromises()
  }

  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(getConfigSchema).mockReset()
    vi.mocked(getConfigSchema).mockResolvedValue(schema)
    vi.mocked(getTemplate).mockReset()
    vi.mocked(getTemplate).mockResolvedValue(stored as any)
    vi.mocked(updateTemplate).mockReset()
    vi.mocked(deleteTemplate).mockReset()
    vi.mocked(assignTemplateToDevice).mockReset()
    alertSpy = vi.spyOn(window, 'alert').mockImplementation(() => {})
    dismissNotice()
  })

  afterEach(() => {
    alertSpy.mockRestore()
    vi.restoreAllMocks()
  })

  it('shows a save that fails for another reason than a refusal in a panel', async () => {
    vi.mocked(updateTemplate).mockRejectedValue(new Error('server down'))
    const wrapper = mountPage()
    await flushPromises()
    await toolbarButton(wrapper, 'Edit').trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid="save-template"]').trigger('click')
    await flushPromises()

    expect(panel(wrapper).text()).toContain('server down')
    expect(panel(wrapper).text()).toContain('Save template')
    expect(alertSpy).not.toHaveBeenCalled()
  })

  it('removes the failure of a save once the retry succeeds', async () => {
    vi.mocked(updateTemplate).mockRejectedValueOnce(new Error('network down'))
    vi.mocked(updateTemplate).mockResolvedValueOnce({ affected_devices: 0 } as any)
    const wrapper = mountPage()
    await flushPromises()
    await toolbarButton(wrapper, 'Edit').trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid="save-template"]').trigger('click')
    await flushPromises()
    expect(panel(wrapper).exists()).toBe(true)

    await wrapper.find('[data-testid="save-template"]').trigger('click')
    await flushPromises()

    expect(panel(wrapper).exists()).toBe(false)
  })

  it('removes the failure of a copy once the retry succeeds', async () => {
    const writeText = vi.fn().mockRejectedValueOnce(new Error('NotAllowedError')).mockResolvedValueOnce(undefined)
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
    const wrapper = mountPage()
    await flushPromises()
    const copy = () => wrapper.findAll('.header-row .btn').find(b => b.text() === 'Copy')!.trigger('click')
    await copy()
    await flushPromises()
    expect(panel(wrapper).exists()).toBe(true)

    await copy()
    await flushPromises()

    expect(panel(wrapper).exists()).toBe(false)
  })

  it('shows a delete that fails in a panel', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    vi.mocked(deleteTemplate).mockRejectedValue(new Error('template is in use'))
    const wrapper = mountPage()
    await flushPromises()
    await wrapper.find('.btn.danger').trigger('click')
    await flushPromises()

    expect(panel(wrapper).text()).toContain('template is in use')
    expect(panel(wrapper).text()).toContain('Delete template')
    expect(alertSpy).not.toHaveBeenCalled()
  })

  it('shows a copy that fails in a panel', async () => {
    const writeText = vi.fn().mockRejectedValue(new Error('NotAllowedError'))
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
    const wrapper = mountPage()
    await flushPromises()
    await wrapper.findAll('.header-row .btn').find(b => b.text() === 'Copy')!.trigger('click')
    await flushPromises()

    expect(panel(wrapper).text()).toContain('Copy config')
    expect(panel(wrapper).text()).toContain('clipboard')
    expect(alertSpy).not.toHaveBeenCalled()
  })

  it('shows a refused assign in the app notice, over the open dialog', async () => {
    vi.mocked(assignTemplateToDevice).mockRejectedValue(new Error('device not found'))
    const wrapper = mountPage()
    await flushPromises()
    await openAssign(wrapper)
    await wrapper.find('.modal input[type="number"]').setValue('7')
    await submitAssign(wrapper)

    expect(notice.value?.message).toContain('device not found')
    expect(notice.value?.context?.action).toBe('Assign template')
    expect(panel(wrapper).exists()).toBe(false)
    expect(wrapper.find('.modal-overlay').exists()).toBe(true)
    expect(alertSpy).not.toHaveBeenCalled()
  })

  it('asks for the device id in the app notice and sends nothing', async () => {
    const wrapper = mountPage()
    await flushPromises()
    await openAssign(wrapper)
    await submitAssign(wrapper)

    expect(notice.value?.message).toContain('Device ID is required')
    expect(assignTemplateToDevice).not.toHaveBeenCalled()
    expect(wrapper.find('.modal-overlay').exists()).toBe(true)
    expect(alertSpy).not.toHaveBeenCalled()
  })

  // Review round 4: the refusal lists fields of the edit form, which the Assign dialog covers and a
  // left page no longer shows. The failure then goes to the app notice.
  it('shows a refused save in the app notice when the Assign dialog opened meanwhile', async () => {
    const save = deferred<any>()
    vi.mocked(updateTemplate).mockReturnValue(save.promise)
    const wrapper = mountPage()
    await flushPromises()
    await toolbarButton(wrapper, 'Edit').trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid="save-template"]').trigger('click')
    await openAssign(wrapper)
    save.reject(refusal())
    await flushPromises()

    expect(notice.value?.message).toContain('Validation failed')
    expect(notice.value?.context?.action).toBe('Save template')
    wrapper.unmount()
  })

  // ginsys/shelly-manager#701: the save worked and the refresh after it failed behind the Assign dialog.
  it('shows a failed refresh after a save in the app notice when the Assign dialog opened meanwhile', async () => {
    const save = deferred<any>()
    vi.mocked(updateTemplate).mockReturnValue(save.promise)
    vi.mocked(getTemplate).mockResolvedValueOnce(stored as any).mockRejectedValueOnce(new Error('refresh down'))
    const wrapper = mountPage()
    await flushPromises()
    await toolbarButton(wrapper, 'Edit').trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid="save-template"]').trigger('click')
    await openAssign(wrapper)
    save.resolve({ template: stored, affected_devices: 0 })
    await flushPromises()

    expect(notice.value?.message).toContain('refresh down')
    expect(notice.value?.context?.action).toBe('Load template')
    wrapper.unmount()
  })

  it('shows a refused save in the app notice when the page was left meanwhile', async () => {
    const save = deferred<any>()
    vi.mocked(updateTemplate).mockReturnValue(save.promise)
    const wrapper = mountPage()
    await flushPromises()
    await toolbarButton(wrapper, 'Edit').trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid="save-template"]').trigger('click')
    wrapper.unmount()
    save.reject(refusal())
    await flushPromises()

    expect(notice.value?.message).toContain('Validation failed')
  })

  // Review round 5: the form is gone after Cancel, and the page is still open, so the page panel says it.
  it('shows a refused save in the page panel when the edit was cancelled meanwhile', async () => {
    const save = deferred<any>()
    vi.mocked(updateTemplate).mockReturnValue(save.promise)
    const wrapper = mountPage()
    await flushPromises()
    await toolbarButton(wrapper, 'Edit').trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid="save-template"]').trigger('click')
    await toolbarButton(wrapper, 'Cancel').trigger('click')
    save.reject(refusal())
    await flushPromises()

    expect(panel(wrapper).text()).toContain('Validation failed')
    expect(panel(wrapper).text()).toContain('Save template')
    expect(wrapper.find('[data-testid="template-save-errors"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('keeps a refused save out of the app notice while its form is open', async () => {
    vi.mocked(updateTemplate).mockRejectedValue(refusal())
    const wrapper = mountPage()
    await flushPromises()
    await toolbarButton(wrapper, 'Edit').trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid="save-template"]').trigger('click')
    await flushPromises()

    expect(notice.value).toBeNull()
    wrapper.unmount()
  })

  it('says on the page that the template was assigned, after the dialog closed', async () => {
    vi.mocked(assignTemplateToDevice).mockResolvedValue(undefined as any)
    const wrapper = mountPage()
    await flushPromises()
    await openAssign(wrapper)
    await wrapper.find('.modal input[type="number"]').setValue('7')
    await submitAssign(wrapper)

    expect(wrapper.find('.modal-overlay').exists()).toBe(false)
    expect(statusLine(wrapper).text()).toBe('Template assigned to the device')
    expect(alertSpy).not.toHaveBeenCalled()
  })
})
