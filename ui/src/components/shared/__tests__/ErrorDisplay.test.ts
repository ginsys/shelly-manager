import { mount } from '@vue/test-utils'
import ErrorDisplay from '@/components/shared/ErrorDisplay.vue'

describe('ErrorDisplay', () => {
  it('renders error code and message', () => {
    const wrapper = mount(ErrorDisplay, {
      props: {
        title: 'Failed to load',
        error: { code: 'UNAUTHORIZED', message: 'Unauthorized', retryable: false },
      },
    })
    expect(wrapper.text()).toContain('Failed to load')
    expect(wrapper.text()).toContain('[UNAUTHORIZED]')
    expect(wrapper.text()).toContain('Unauthorized')
  })

  it('shows Retry when retryable', async () => {
    const wrapper = mount(ErrorDisplay, {
      props: {
        title: 'Oops',
        error: { code: 'NETWORK_ERROR', message: 'Network error', retryable: true },
      },
    })
    const retryBtn = wrapper.find('button.btn.primary')
    expect(retryBtn.exists()).toBe(true)
    await retryBtn.trigger('click')
    expect(wrapper.emitted('retry')).toBeTruthy()
  })

  it('emits dismiss', async () => {
    const wrapper = mount(ErrorDisplay, {
      props: {
        title: 'Oops',
        error: { code: 'UNKNOWN', message: 'Unknown' },
      },
    })
    await wrapper.findAll('button.btn')[0].trigger('click')
    // First button is Dismiss when retryable is false
    expect(wrapper.emitted('dismiss')).toBeTruthy()
  })
})

