import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import ErrorDisplay from '../ErrorDisplay.vue'
import type { AppError } from '@/types/errors'

const retryable: AppError = {
  code: 'NETWORK_ERROR',
  message: 'Network error',
  retryable: true,
}

describe('ErrorDisplay retry button', () => {
  it('shows Retry for a retryable error by default, and emits retry', async () => {
    const wrapper = mount(ErrorDisplay, { props: { error: retryable } })

    await wrapper.get('.retry-button').trigger('click')

    expect(wrapper.emitted('retry')).toHaveLength(1)
  })

  it('hides Retry when showRetry is false, and keeps Dismiss', async () => {
    const wrapper = mount(ErrorDisplay, { props: { error: retryable, showRetry: false } })

    expect(wrapper.find('.retry-button').exists()).toBe(false)
    await wrapper.get('.dismiss-button').trigger('click')
    expect(wrapper.emitted('dismiss')).toHaveLength(1)
  })

  it('shows no Retry for an error that is not retryable', () => {
    const wrapper = mount(ErrorDisplay, { props: { error: { ...retryable, retryable: false } } })

    expect(wrapper.find('.retry-button').exists()).toBe(false)
  })
})
