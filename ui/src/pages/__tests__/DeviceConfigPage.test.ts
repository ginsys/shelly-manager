import { mount, flushPromises } from '@vue/test-utils'
import DeviceConfigPage from '@/pages/DeviceConfigPage.vue'

const getStoredConfig = vi.fn()

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: '1' } }),
}))

vi.mock('@/api/deviceConfig', () => ({
  getStoredConfig: (...args: any[]) => getStoredConfig(...args),
  getLiveConfig: vi.fn(),
  getLiveConfigNormalized: vi.fn(),
  getTypedNormalizedConfig: vi.fn(),
  updateStoredConfig: vi.fn(),
  importConfig: vi.fn(),
  getImportStatus: vi.fn(),
  exportConfig: vi.fn(),
  detectDrift: vi.fn(),
  getConfigHistory: vi.fn(),
  applyTemplate: vi.fn(),
}))

describe('DeviceConfigPage', () => {
  beforeEach(() => {
    getStoredConfig.mockReset()
  })

  it('renders ErrorDisplay on config load failure and retries', async () => {
    // First call fails, second succeeds
    getStoredConfig.mockRejectedValueOnce(new Error('Failed to load configuration'))
    getStoredConfig.mockResolvedValueOnce({ example: true })

    const wrapper = mount(DeviceConfigPage)

    // Click the Stored button to trigger load
    const btn = wrapper.findAll('button.btn').find(b => b.text().includes('Stored'))
    expect(btn).toBeTruthy()
    await (btn as any)!.trigger('click')
    await flushPromises()

    // ErrorDisplay appears
    expect(wrapper.find('[data-testid="error-display"]').exists()).toBe(true)

    // Click Retry
    const retry = wrapper.find('button.btn.primary')
    if (retry.exists()) {
      await retry.trigger('click')
      await flushPromises()
    }

    // After success, error display should disappear
    expect(wrapper.find('[data-testid="error-display"]').exists()).toBe(false)
  })
})

