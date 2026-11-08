import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import DeviceConfigPage from '../DeviceConfigPage.vue'
import { useDeviceConfigNewStore } from '@/stores/deviceConfigNew'

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: '7' } }),
}))

vi.mock('@/api/deviceConfig', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/deviceConfig')>()
  return { ...actual, getCurrentDeviceConfig: vi.fn() }
})

import { getCurrentDeviceConfig } from '@/api/deviceConfig'

const stubs = {
  'router-link': true,
  ConfigEditor: true,
  ConfigView: {
    props: ['config'],
    template: '<pre class="stub-view">{{ JSON.stringify(config) }}</pre>',
  },
}

async function mountPage() {
  const store = useDeviceConfigNewStore()
  // The page refreshes the desired-config store on mount; keep that offline.
  vi.spyOn(store, 'refresh').mockResolvedValue(undefined as never)
  const wrapper = mount(DeviceConfigPage, { global: { stubs } })
  await flushPromises()
  return wrapper
}

describe('DeviceConfigPage live panel (#388)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('does not contact the device on page load', async () => {
    await mountPage()

    expect(getCurrentDeviceConfig).not.toHaveBeenCalled()
  })

  it('shows the device settings after the button is clicked', async () => {
    vi.mocked(getCurrentDeviceConfig).mockResolvedValue({
      id: 1,
      device_id: 7,
      config: { name: 'Live Name' },
    })
    const wrapper = await mountPage()

    await wrapper.get('[data-testid="live-load"]').trigger('click')
    await flushPromises()

    expect(getCurrentDeviceConfig).toHaveBeenCalledWith('7')
    expect(wrapper.get('[data-testid="live-config"]').text()).toContain('Live Name')
    expect(wrapper.find('[data-testid="live-error"]').exists()).toBe(false)
  })

  it('shows the error text when the device cannot be reached, without failing the page', async () => {
    vi.mocked(getCurrentDeviceConfig).mockRejectedValue(new Error('Device not responding'))
    const wrapper = await mountPage()

    await wrapper.get('[data-testid="live-load"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="live-error"]').text()).toBe('Device not responding')
    expect(wrapper.find('[data-testid="live-config"]').exists()).toBe(false)
    // The rest of the page is still there.
    expect(wrapper.text()).toContain('Desired Configuration')
  })

  it('clears an earlier error when a later read succeeds', async () => {
    vi.mocked(getCurrentDeviceConfig)
      .mockRejectedValueOnce(new Error('Device not responding'))
      .mockResolvedValueOnce({ config: { name: 'Back Online' } })
    const wrapper = await mountPage()

    await wrapper.get('[data-testid="live-load"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="live-load"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="live-error"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="live-config"]').text()).toContain('Back Online')
  })
})
