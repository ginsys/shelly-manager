import { describe, it, expect, beforeAll, afterAll, beforeEach, afterEach, vi } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import MainLayout from '../MainLayout.vue'
import DeviceConfigPage from '@/pages/DeviceConfigPage.vue'
import { dismissNotice } from '@/composables/useNotice'
import * as configNewApi from '@/api/configNew'

// The global test setup replaces document.createElement('a') with a mock object, so router links
// would render nothing real. Restore the real one for this file.
const stubbedCreateElement = document.createElement
beforeAll(() => {
  document.createElement = Document.prototype.createElement.bind(document) as typeof document.createElement
})
afterAll(() => {
  document.createElement = stubbedCreateElement
})

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

function answerReads() {
  api.getDeviceTemplatesNew.mockImplementation(async () => ({ templates: [], template_ids: [] }) as any)
  api.getDeviceOverridesNew.mockImplementation(async (id: any) => ({ overrides: { owner: `device-${id}` } }) as any)
  api.getDesiredConfigNew.mockImplementation(async (id: any) => ({ config: { owner: `device-${id}` }, sources: {} }) as any)
  // Pending changes, so the page offers Apply Config.
  api.getConfigStatusNew.mockImplementation(async () => ({ config_applied: false, pending_changes: true }) as any)
}

const DevicePage = { template: '<div data-testid="device-page">device</div>' }

let wrapper: VueWrapper | null = null

async function mountConfigPage() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/devices/:id/config', component: DeviceConfigPage },
      { path: '/devices/:id', component: DevicePage },
    ],
  })
  await router.push('/devices/7/config')
  await router.isReady()
  wrapper = mount(MainLayout, {
    global: { plugins: [router], stubs: { ConfigEditor: true, ConfigView: true } },
  })
  await flushPromises()
  return { wrapper, router }
}

function buttonWithText(w: VueWrapper, text: string) {
  const found = w.findAll('button').find((b) => b.text().trim() === text)
  if (!found) throw new Error(`no button with text "${text}"`)
  return found
}

// A request that stays open until the test settles it, so a test decides when the failure arrives.
function holdApply() {
  let fail: (e: Error) => void = () => {}
  api.applyDeviceConfigNew.mockImplementation(
    () =>
      new Promise((_resolve, reject) => {
        fail = reject
      }) as any,
  )
  return (e: Error) => fail(e)
}

describe('app notice for failures the page cannot show (ginsys/shelly-manager#648)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.resetAllMocks()
    answerReads()
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    // One notice is shared by the whole app: a case that leaves it set must not reach the next one.
    dismissNotice()
  })

  afterEach(() => {
    wrapper?.unmount()
    wrapper = null
    vi.restoreAllMocks()
  })

  it('shows a failure that arrives after the user left the page', async () => {
    const failApply = holdApply()
    const { wrapper: w, router } = await mountConfigPage()

    await buttonWithText(w, 'Apply Config').trigger('click')
    await flushPromises() // the apply request is open
    await router.push('/devices/7')
    await flushPromises()
    expect(w.find('[data-testid="device-page"]').exists()).toBe(true)
    expect(w.find('[data-testid="app-notice"] .error-display').exists()).toBe(false)

    failApply(new Error('device did not answer'))
    await flushPromises()

    const notice = w.get('[data-testid="app-notice"]')
    expect(notice.text()).toContain('device did not answer')
    expect(notice.text()).toContain('Apply configuration')
    expect(notice.text()).toContain('Device')
    expect(notice.text()).toContain('7')
    // The notice is a live region and has no Retry that would do nothing.
    expect(notice.attributes('role')).toBe('alert')
    expect(notice.find('.retry-button').exists()).toBe(false)
  })

  it('shows a failure on the page, not in the notice, while the page is open and no dialog covers it', async () => {
    const failApply = holdApply()
    const { wrapper: w } = await mountConfigPage()

    await buttonWithText(w, 'Apply Config').trigger('click')
    await flushPromises()
    failApply(new Error('inline failure'))
    await flushPromises()

    expect(w.get('[data-testid="config-action-error"]').text()).toContain('inline failure')
    expect(w.find('[data-testid="app-notice"] .error-display').exists()).toBe(false)
  })

  it('shows a failure in the notice when the page moved to another device and a dialog covers the panel', async () => {
    const failApply = holdApply()
    const { wrapper: w, router } = await mountConfigPage()

    await buttonWithText(w, 'Apply Config').trigger('click')
    await flushPromises() // the apply of device 7 is open
    // The same page instance now shows device 8, with its body loaded, so its dialog can open.
    await router.push('/devices/8/config')
    await flushPromises()
    await buttonWithText(w, '+ Add Template').trigger('click')
    await flushPromises()
    expect(w.find('.modal-overlay').exists()).toBe(true)

    failApply(new Error('failure behind the dialog'))
    await flushPromises()

    const notice = w.get('[data-testid="app-notice"]')
    expect(notice.text()).toContain('failure behind the dialog')
    expect(notice.text()).toContain('7') // it names the device the apply was for
    expect(w.find('[data-testid="config-action-error"]').exists()).toBe(false)
  })

  it('removes the notice when the user dismisses it', async () => {
    const failApply = holdApply()
    const { wrapper: w, router } = await mountConfigPage()

    await buttonWithText(w, 'Apply Config').trigger('click')
    await flushPromises()
    await router.push('/devices/7')
    await flushPromises()
    failApply(new Error('late failure'))
    await flushPromises()
    expect(w.find('[data-testid="app-notice"] .error-display').exists()).toBe(true)

    await w.get('[data-testid="app-notice"] .dismiss-button').trigger('click')
    expect(w.find('[data-testid="app-notice"] .error-display').exists()).toBe(false)
  })
})
