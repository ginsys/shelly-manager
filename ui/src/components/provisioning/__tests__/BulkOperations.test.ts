import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import BulkOperations from '../BulkOperations.vue'
import { useProvisioningStore } from '@/stores/provisioning'
import { dismissNotice, notice } from '@/composables/useNotice'

// ginsys/shelly-manager#701: the store keeps no action failure, so a form closed while the request runs
// must not lose it.

function mountForm() {
  const wrapper = mount(BulkOperations, { global: { plugins: [createTestingPinia({ createSpy: vi.fn })] } })
  return { wrapper, store: useProvisioningStore() }
}

async function submit(wrapper: ReturnType<typeof mountForm>['wrapper']) {
  await wrapper.find('.primary-button').trigger('click')
  await wrapper.find('#deviceIds').setValue('device-1')
  await wrapper.find('form').trigger('submit')
}

describe('BulkOperations failure', () => {
  beforeEach(() => dismissNotice())

  it('shows a failure in the open form, and not in the app notice', async () => {
    const { wrapper, store } = mountForm()
    vi.mocked(store.createBulkTasks).mockRejectedValue(new Error('Failed to create tasks'))
    await submit(wrapper)
    await flushPromises()
    expect(wrapper.find('.error-message').text()).toBe('Failed to create tasks')
    expect(notice.value).toBeNull()
  })

  it('shows a failure in the app notice when the form was cancelled meanwhile', async () => {
    const { wrapper, store } = mountForm()
    let fail: (e: Error) => void = () => {}
    vi.mocked(store.createBulkTasks).mockImplementation(() => new Promise((_r, reject) => { fail = reject }) as any)
    await submit(wrapper)
    await wrapper.find('.secondary-button').trigger('click')
    expect(wrapper.find('form').exists()).toBe(false)
    fail(new Error('Failed to create tasks'))
    await flushPromises()
    expect(notice.value?.message).toContain('Failed to create tasks')
    expect(notice.value?.context?.action).toBe('Create bulk tasks')
  })
})
