import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import DeviceConfigPage from '../DeviceConfigPage.vue'
import * as configNewApi from '@/api/configNew'

vi.mock('@/api/configNew', () => ({
  getDeviceTemplatesNew: vi.fn(),
  setDeviceTemplatesNew: vi.fn(),
  addDeviceTemplateNew: vi.fn(),
  removeDeviceTemplateNew: vi.fn(),
  getDeviceOverridesNew: vi.fn(),
  setDeviceOverridesNew: vi.fn(),
  patchDeviceOverridesNew: vi.fn(),
  deleteDeviceOverridesNew: vi.fn(),
  getDesiredConfigNew: vi.fn(),
  getConfigStatusNew: vi.fn(),
  applyDeviceConfigNew: vi.fn(),
  verifyDeviceConfigNew: vi.fn(),
}))

const api = vi.mocked(configNewApi)

// ginsys/shelly-manager#614: the device configuration page shows a failed action as an error panel
// and an outcome or an input check inline. It never opens a browser dialog.
describe('DeviceConfigPage shows no browser dialog', () => {
  let alertSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    setActivePinia(createPinia())
    vi.resetAllMocks()
    alertSpy = vi.spyOn(window, 'alert').mockImplementation(() => {})
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    api.getDeviceTemplatesNew.mockImplementation(
      async () => ({ templates: [{ id: 3, name: 'Base', scope: 'global' }], template_ids: [3] }) as any,
    )
    api.getDeviceOverridesNew.mockImplementation(async () => ({ overrides: { owner: 'x' } }) as any)
    api.getDesiredConfigNew.mockImplementation(async () => ({ config: { owner: 'x' }, sources: {} }) as any)
    api.getConfigStatusNew.mockImplementation(async () => ({ config_applied: false, pending_changes: true }) as any)
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  async function mountPage() {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/devices/:id/config', component: DeviceConfigPage }],
    })
    router.push('/devices/7/config')
    await router.isReady()
    const wrapper = mount(DeviceConfigPage, {
      global: { plugins: [router], stubs: { ConfigEditor: true, ConfigView: true, RouterLink: true } },
    })
    await flushPromises()
    return { wrapper, router }
  }

  function button(wrapper: ReturnType<typeof mount>, text: string) {
    const found = wrapper.findAll('button').find((b) => b.text().trim() === text)
    if (!found) throw new Error(`no button with text "${text}"`)
    return found
  }

  async function expectActionError(wrapper: ReturnType<typeof mount>, message: string, action: string) {
    const panel = wrapper.find('[data-testid="config-action-error"]')
    expect(panel.exists()).toBe(true)
    expect(panel.text()).toContain(message)
    expect(panel.text()).toContain('UNKNOWN')
    expect(panel.text()).toContain(action)
    expect(alertSpy).not.toHaveBeenCalled()
  }

  it('shows a failed apply as an error panel', async () => {
    api.applyDeviceConfigNew.mockRejectedValue(new Error('device unreachable'))
    const { wrapper } = await mountPage()

    await button(wrapper, 'Apply Config').trigger('click')
    await flushPromises()

    await expectActionError(wrapper, 'device unreachable', 'Apply configuration')
  })

  it('shows a failed verify as an error panel', async () => {
    api.verifyDeviceConfigNew.mockRejectedValue(new Error('verify refused'))
    const { wrapper } = await mountPage()

    await button(wrapper, 'Verify Device').trigger('click')
    await flushPromises()

    await expectActionError(wrapper, 'verify refused', 'Verify device')
  })

  it('shows a failed template removal as an error panel', async () => {
    api.removeDeviceTemplateNew.mockRejectedValue(new Error('template is in use'))
    const { wrapper } = await mountPage()

    await wrapper.get('button[title="Remove"]').trigger('click')
    await flushPromises()

    await expectActionError(wrapper, 'template is in use', 'Remove template')
  })

  it('shows a failed Clear All as an error panel', async () => {
    api.deleteDeviceOverridesNew.mockRejectedValue(new Error('overrides locked'))
    const { wrapper } = await mountPage()

    await button(wrapper, 'Clear All').trigger('click')
    await flushPromises()

    await expectActionError(wrapper, 'overrides locked', 'Clear overrides')
  })

  it('removes the error panel with its Dismiss button', async () => {
    api.verifyDeviceConfigNew.mockRejectedValue(new Error('verify refused'))
    const { wrapper } = await mountPage()

    await button(wrapper, 'Verify Device').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="config-action-error"] .dismiss-button').trigger('click')

    expect(wrapper.find('[data-testid="config-action-error"]').exists()).toBe(false)
  })

  it('clears the error panel as soon as the next action starts, before it finishes', async () => {
    api.verifyDeviceConfigNew.mockRejectedValueOnce(new Error('verify refused'))
    let finishSecond: (value: any) => void = () => {}
    api.verifyDeviceConfigNew.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          finishSecond = resolve
        }) as any,
    )
    const { wrapper } = await mountPage()

    await button(wrapper, 'Verify Device').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="config-action-error"]').exists()).toBe(true)

    await button(wrapper, 'Verify Device').trigger('click')
    await flushPromises() // the second verify is still open
    expect(api.verifyDeviceConfigNew).toHaveBeenCalledTimes(2)
    expect(wrapper.find('[data-testid="config-action-error"]').exists()).toBe(false)

    finishSecond({ match: true })
    await flushPromises()
  })

  it('shows a failed refresh as the page error card after an action failed and its panel was dismissed', async () => {
    api.verifyDeviceConfigNew.mockRejectedValue(new Error('verify refused'))
    const { wrapper } = await mountPage()
    await button(wrapper, 'Verify Device').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="config-action-error"] .dismiss-button').trigger('click')

    api.getConfigStatusNew.mockRejectedValue(new Error('status unavailable'))
    await button(wrapper, 'Refresh').trigger('click')
    await flushPromises()

    expect(wrapper.get('.card.state.error').text()).toContain('status unavailable')
  })

  it('does not let a late failure of the old device hide the load error of the device on screen', async () => {
    let rejectApply: (reason: any) => void = () => {}
    api.applyDeviceConfigNew.mockImplementation(
      () =>
        new Promise((_resolve, reject) => {
          rejectApply = reject
        }) as any,
    )
    const { wrapper, router } = await mountPage()
    await button(wrapper, 'Apply Config').trigger('click')
    await flushPromises() // the apply of device 7 is still open

    api.getConfigStatusNew.mockRejectedValue(new Error('status unavailable'))
    await router.push('/devices/8/config')
    await flushPromises()
    rejectApply(new Error('device 7 went away'))
    await flushPromises()

    expect(wrapper.get('.card.state.error').text()).toContain('status unavailable')
    expect(wrapper.get('[data-testid="config-action-error"]').text()).toContain('device 7 went away')
  })

  it('shows an apply that succeeded inline, in its own result card', async () => {
    api.applyDeviceConfigNew.mockResolvedValue({ success: true, applied_count: 4, failed_count: 0 } as any)
    const { wrapper } = await mountPage()

    await button(wrapper, 'Apply Config').trigger('click')
    await flushPromises()

    const result = wrapper.get('[data-testid="apply-result"]')
    expect(result.text()).toContain('Applied successfully. 4 field(s) updated.')
    expect(result.attributes('data-kind')).toBe('success')
    expect(wrapper.find('[data-testid="config-action-error"]').exists()).toBe(false)
    expect(alertSpy).not.toHaveBeenCalled()
  })

  it('shows an apply with failed fields inline, as neither an error panel nor a success', async () => {
    api.applyDeviceConfigNew.mockResolvedValue({ success: false, applied_count: 1, failed_count: 2 } as any)
    const { wrapper } = await mountPage()

    await button(wrapper, 'Apply Config').trigger('click')
    await flushPromises()

    const result = wrapper.get('[data-testid="apply-result"]')
    expect(result.text()).toContain('Apply completed with errors. 2 field(s) failed.')
    expect(result.attributes('data-kind')).toBe('failed')
    expect(wrapper.find('[data-testid="config-action-error"]').exists()).toBe(false)
    expect(alertSpy).not.toHaveBeenCalled()
  })

  it('shows an unconfirmed apply as its own state', async () => {
    api.applyDeviceConfigNew.mockResolvedValue({
      success: false,
      unconfirmed: true,
      applied_count: 0,
      failed_count: 0,
    } as any)
    const { wrapper } = await mountPage()

    await button(wrapper, 'Apply Config').trigger('click')
    await flushPromises()

    const result = wrapper.get('[data-testid="apply-result"]')
    expect(result.attributes('data-kind')).toBe('unconfirmed')
    expect(result.text()).toContain('unconfirmed')
    expect(result.text()).toContain('may have changed network')
    expect(result.text()).not.toContain('Applied successfully')
    expect(alertSpy).not.toHaveBeenCalled()
  })

  it('removes the result card with its Dismiss button', async () => {
    api.applyDeviceConfigNew.mockResolvedValue({ success: true, applied_count: 1, failed_count: 0 } as any)
    const { wrapper } = await mountPage()

    await button(wrapper, 'Apply Config').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="apply-result-dismiss"]').trigger('click')

    expect(wrapper.find('[data-testid="apply-result"]').exists()).toBe(false)
  })

  it('removes the result card as soon as the next apply starts, before it finishes', async () => {
    api.applyDeviceConfigNew.mockResolvedValueOnce({ success: true, applied_count: 1, failed_count: 0 } as any)
    let finishSecond: (value: any) => void = () => {}
    api.applyDeviceConfigNew.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          finishSecond = resolve
        }) as any,
    )
    const { wrapper } = await mountPage()

    await button(wrapper, 'Apply Config').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="apply-result"]').attributes('data-kind')).toBe('success')

    await button(wrapper, 'Apply Config').trigger('click')
    await flushPromises() // the second apply is still open
    expect(api.applyDeviceConfigNew).toHaveBeenCalledTimes(2)
    expect(wrapper.find('[data-testid="apply-result"]').exists()).toBe(false)

    finishSecond({ success: false, applied_count: 0, failed_count: 1 })
    await flushPromises()
    expect(wrapper.get('[data-testid="apply-result"]').attributes('data-kind')).toBe('failed')
  })

  it('does not carry a result over to another device when the route changes', async () => {
    api.applyDeviceConfigNew.mockResolvedValue({ success: true, applied_count: 1, failed_count: 0 } as any)
    const { wrapper, router } = await mountPage()

    await button(wrapper, 'Apply Config').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="apply-result"]').exists()).toBe(true)

    await router.push('/devices/8/config')
    await flushPromises()

    expect(wrapper.find('[data-testid="apply-result"]').exists()).toBe(false)
  })

  it('shows a failure that arrives after the page moved to another device, naming the device it belongs to', async () => {
    let rejectApply: (reason: any) => void = () => {}
    api.applyDeviceConfigNew.mockImplementation(
      () =>
        new Promise((_resolve, reject) => {
          rejectApply = reject
        }) as any,
    )
    const { wrapper, router } = await mountPage()

    await button(wrapper, 'Apply Config').trigger('click')
    await flushPromises() // the apply of device 7 is still open

    await router.push('/devices/8/config')
    await flushPromises()
    rejectApply(new Error('device 7 went away'))
    await flushPromises()

    const panel = wrapper.get('[data-testid="config-action-error"]')
    expect(panel.text()).toContain('device 7 went away')
    expect(panel.text()).toContain('#7')
    expect(alertSpy).not.toHaveBeenCalled()
  })

  it('shows a failed action once: the error panel, and not the store card as well', async () => {
    api.applyDeviceConfigNew.mockRejectedValue(new Error('device unreachable'))
    const { wrapper } = await mountPage()

    await button(wrapper, 'Apply Config').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="config-action-error"]').exists()).toBe(true)
    expect(wrapper.find('.card.state.error').exists()).toBe(false)
  })

  it('keeps the page body when the error panel is dismissed, and does not bring the store card back', async () => {
    api.applyDeviceConfigNew.mockRejectedValue(new Error('device unreachable'))
    const { wrapper } = await mountPage()
    await button(wrapper, 'Apply Config').trigger('click')
    await flushPromises()

    await wrapper.get('[data-testid="config-action-error"] .dismiss-button').trigger('click')

    expect(wrapper.find('[data-testid="config-action-error"]').exists()).toBe(false)
    expect(wrapper.find('.card.state.error').exists()).toBe(false)
    expect(wrapper.text()).toContain('Verify Device')
  })

  it('still shows a failed load as the page error card', async () => {
    api.getConfigStatusNew.mockRejectedValue(new Error('status unavailable'))
    const { wrapper } = await mountPage()

    expect(wrapper.get('.card.state.error').text()).toContain('status unavailable')
    expect(wrapper.find('[data-testid="config-action-error"]').exists()).toBe(false)
  })

  it('shows an empty template id inline and sends no request', async () => {
    const { wrapper } = await mountPage()

    await button(wrapper, '+ Add Template').trigger('click')
    await button(wrapper, 'Add').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="add-template-id-error"]').text()).toContain('Template ID is required')
    expect(api.addDeviceTemplateNew).not.toHaveBeenCalled()
    expect(alertSpy).not.toHaveBeenCalled()
  })

  it('clears the template id message when the dialog is cancelled and opened again', async () => {
    const { wrapper } = await mountPage()

    await button(wrapper, '+ Add Template').trigger('click')
    await button(wrapper, 'Add').trigger('click')
    expect(wrapper.find('[data-testid="add-template-id-error"]').exists()).toBe(true)

    await button(wrapper, 'Cancel').trigger('click')
    await button(wrapper, '+ Add Template').trigger('click')

    expect(wrapper.find('[data-testid="add-template-id-error"]').exists()).toBe(false)
  })

  it('clears the template id message when the id is typed', async () => {
    const { wrapper } = await mountPage()

    await button(wrapper, '+ Add Template').trigger('click')
    await button(wrapper, 'Add').trigger('click')
    expect(wrapper.find('[data-testid="add-template-id-error"]').exists()).toBe(true)

    await wrapper.get('input[placeholder="Enter template ID"]').setValue('5')
    expect(wrapper.find('[data-testid="add-template-id-error"]').exists()).toBe(false)
  })

  it('keeps the page body when an empty id is submitted after an action failed and was dismissed', async () => {
    api.verifyDeviceConfigNew.mockRejectedValue(new Error('verify refused'))
    const { wrapper } = await mountPage()
    await button(wrapper, 'Verify Device').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="config-action-error"] .dismiss-button').trigger('click')

    await button(wrapper, '+ Add Template').trigger('click')
    await button(wrapper, 'Add').trigger('click')
    await flushPromises()

    expect(wrapper.find('.card.state.error').exists()).toBe(false)
    expect(wrapper.get('[data-testid="add-template-id-error"]').text()).toContain('Template ID is required')
  })

  it('clears the failure of the last add when another id is typed', async () => {
    api.addDeviceTemplateNew.mockRejectedValue(new Error('no such template'))
    const { wrapper } = await mountPage()
    await button(wrapper, '+ Add Template').trigger('click')
    const input = wrapper.get('input[placeholder="Enter template ID"]')
    await input.setValue('5')
    await button(wrapper, 'Add').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="add-template-error"]').exists()).toBe(true)

    await input.setValue('6')

    expect(wrapper.find('[data-testid="add-template-error"]').exists()).toBe(false)
  })

  it('shows an add that fails after the page moved to another device in the page panel, naming its device', async () => {
    let rejectAdd: (reason: any) => void = () => {}
    api.addDeviceTemplateNew.mockImplementation(
      () =>
        new Promise((_resolve, reject) => {
          rejectAdd = reject
        }) as any,
    )
    const { wrapper, router } = await mountPage()
    await button(wrapper, '+ Add Template').trigger('click')
    await wrapper.get('input[placeholder="Enter template ID"]').setValue('5')
    await button(wrapper, 'Add').trigger('click')
    await flushPromises() // the add for device 7 is still open

    await router.push('/devices/8/config')
    await flushPromises()
    rejectAdd(new Error('no such template'))
    await flushPromises()

    const panel = wrapper.get('[data-testid="config-action-error"]')
    expect(panel.text()).toContain('no such template')
    expect(panel.text()).toContain('Add template')
    expect(panel.text()).toContain('#7')
    expect(wrapper.find('[data-testid="add-template-error"]').exists()).toBe(false)
  })

  it('shows an add that fails after the dialog was cancelled in the page panel, and leaves nothing in the dialog', async () => {
    let rejectAdd: (reason: any) => void = () => {}
    api.addDeviceTemplateNew.mockImplementation(
      () =>
        new Promise((_resolve, reject) => {
          rejectAdd = reject
        }) as any,
    )
    const { wrapper } = await mountPage()
    await button(wrapper, '+ Add Template').trigger('click')
    await wrapper.get('input[placeholder="Enter template ID"]').setValue('5')
    await button(wrapper, 'Add').trigger('click')
    await flushPromises() // the add is open
    await button(wrapper, 'Cancel').trigger('click')
    expect(wrapper.find('.modal').exists()).toBe(false)

    rejectAdd(new Error('no such template'))
    await flushPromises()

    const panel = wrapper.get('[data-testid="config-action-error"]')
    expect(panel.text()).toContain('no such template')
    expect(panel.text()).toContain('Add template')
    expect(wrapper.find('.card.state.error').exists()).toBe(false)

    await button(wrapper, '+ Add Template').trigger('click')
    expect(wrapper.find('[data-testid="add-template-error"]').exists()).toBe(false)
  })

  it('shows no Retry button on a retryable failure, since nothing handles it', async () => {
    api.verifyDeviceConfigNew.mockRejectedValue(
      Object.assign(new Error('Network Error'), { isAxiosError: true }),
    )
    const { wrapper } = await mountPage()

    await button(wrapper, 'Verify Device').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="config-action-error"]').text()).toContain('NETWORK_ERROR')
    expect(wrapper.find('[data-testid="config-action-error"] .retry-button').exists()).toBe(false)
  })

  it('shows a failed template add inside the open dialog', async () => {
    api.addDeviceTemplateNew.mockRejectedValue(new Error('no such template'))
    const { wrapper } = await mountPage()

    await button(wrapper, '+ Add Template').trigger('click')
    await wrapper.get('input[placeholder="Enter template ID"]').setValue('5')
    await button(wrapper, 'Add').trigger('click')
    await flushPromises()

    const panel = wrapper.get('.modal [data-testid="add-template-error"]')
    expect(panel.text()).toContain('no such template')
    expect(panel.text()).toContain('Add template')
    expect(wrapper.find('.modal').exists()).toBe(true)
    expect(alertSpy).not.toHaveBeenCalled()
  })
})
