import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import { h } from 'vue'
import DriftReportsPage from '../DriftReportsPage.vue'
import DriftSchedulesPage from '../DriftSchedulesPage.vue'
import { useDriftStore } from '@/stores/drift'

// ginsys/shelly-manager#614: a failed drift action shows the store's own error text, once, and opens no
// alert(). The store sets `error` and rethrows; the page shows that text where the user acted.

const RouterLinkStub = { props: ['to'], render(this: any) { return h('a', this.$slots.default?.()) } }

const trend = {
  id: 7,
  device_id: 3,
  path: 'wifi.ssid',
  category: 'config',
  severity: 'high',
  occurrences: 2,
  first_seen: '2026-11-01T00:00:00Z',
  last_seen: '2026-11-02T00:00:00Z',
  resolved: false,
}

const schedule = {
  id: 1, name: 'Nightly drift', description: '', enabled: true, cron_spec: '0 0 * * *',
  device_ids: [1], device_filter: {}, last_run: null, next_run: null, run_count: 0,
  created_at: '2026-07-01T00:00:00Z', updated_at: '2026-07-01T00:00:00Z',
}

// The shared test setup answers document.createElement('a') with a plain object, which Vue cannot render.
const realCreateElement = Document.prototype.createElement.bind(document)

let alertSpy: ReturnType<typeof vi.spyOn>

beforeEach(() => {
  vi.mocked(document.createElement).mockImplementation(((tag: string, opts?: any) => realCreateElement(tag, opts)) as any)
  alertSpy = vi.spyOn(window, 'alert').mockImplementation(() => {})
})
afterEach(() => {
  alertSpy.mockRestore()
})

function mountWith(page: any, drift: Record<string, unknown>) {
  const wrapper = mount(page, {
    global: {
      plugins: [createTestingPinia({ createSpy: vi.fn, initialState: { drift } })],
      stubs: { 'router-link': RouterLinkStub },
    },
  })
  return { wrapper, store: useDriftStore() }
}

describe('DriftReportsPage resolve failure', () => {
  it('shows the store error inside the resolve dialog and raises no alert', async () => {
    const { wrapper, store } = mountWith(DriftReportsPage, { trends: [trend], loading: false, error: null })
    vi.mocked(store.resolveTrend).mockImplementation(async () => {
      store.error = 'Failed to resolve drift item'
      throw new Error('Failed to resolve drift item')
    })
    await flushPromises()
    await wrapper.find('.btn-small').trigger('click')
    await wrapper.find('.form-actions .btn.primary').trigger('click')
    await flushPromises()
    expect(alertSpy).not.toHaveBeenCalled()
    // the dialog stays open, because the item is not resolved, and carries the text
    const inDialog = wrapper.find('.modal [data-testid="drift-resolve-error"]')
    expect(inDialog.exists()).toBe(true)
    expect(inDialog.text()).toBe('Failed to resolve drift item')
  })

  it('shows no error text in the dialog when nothing failed', async () => {
    const { wrapper } = mountWith(DriftReportsPage, { trends: [trend], loading: false, error: null })
    await flushPromises()
    await wrapper.find('.btn-small').trigger('click')
    expect(wrapper.find('[data-testid="drift-resolve-error"]').exists()).toBe(false)
  })
})

describe('DriftSchedulesPage delete failure', () => {
  it('shows the store error once on the page and raises no alert', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
    const { wrapper, store } = mountWith(DriftSchedulesPage, { schedules: [schedule], loading: false, error: null })
    vi.mocked(store.remove).mockImplementation(async () => {
      store.error = 'Failed to delete drift schedule'
      throw new Error('Failed to delete drift schedule')
    })
    await flushPromises()
    await wrapper.find('button.btn-icon.danger').trigger('click')
    await flushPromises()
    expect(alertSpy).not.toHaveBeenCalled()
    expect(wrapper.text().split('Failed to delete drift schedule').length - 1).toBe(1)
    confirmSpy.mockRestore()
  })
})
