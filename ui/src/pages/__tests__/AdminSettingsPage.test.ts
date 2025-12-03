import { mount, flushPromises } from '@vue/test-utils'
import AdminSettingsPage from '@/pages/AdminSettingsPage.vue'

const rotateAdminKey = vi.fn()

vi.mock('@/api/admin', () => ({
  rotateAdminKey: (...args: any[]) => rotateAdminKey(...args),
}))

describe('AdminSettingsPage', () => {
  beforeEach(() => {
    rotateAdminKey.mockReset()
  })

  it('shows ErrorDisplay on rotation failure and success banner on success', async () => {
    // First call fails
    rotateAdminKey.mockRejectedValueOnce(new Error('Rotation failed'))

    const wrapper = mount(AdminSettingsPage, { attachTo: document.body })

    // Enter key and submit
    const input = wrapper.find('input')
    await input.setValue('new-key-123')
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(wrapper.find('[data-testid="error-display"]').exists()).toBe(true)

    // Next call succeeds
    rotateAdminKey.mockResolvedValueOnce({})

    // Click Retry
    const retry = wrapper.find('button.btn.primary')
    // Retry button may be absent if error not marked retryable; ensure no throw
    if (retry.exists()) {
      await retry.trigger('click')
      await flushPromises()
    } else {
      // Fallback: resubmit form to retry
      await wrapper.find('form').trigger('submit.prevent')
      await flushPromises()
    }

    // On success, show MessageBanner (not error)
    expect(wrapper.find('[data-testid="error-display"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('Admin key rotated')
  })
})
