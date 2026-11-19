import { describe, it, expect, beforeEach } from 'vitest'
import { defineComponent, h, ref } from 'vue'
import { mount } from '@vue/test-utils'
import { usePageError, notice, dismissNotice } from '../useNotice'

// The module holds one notice for the whole app, so each test starts from an empty one.
beforeEach(() => dismissNotice())

const ctx = { action: 'Apply configuration', resource: 'Device', resourceId: 7 }

function mountPage(covered?: () => boolean) {
  let page!: ReturnType<typeof usePageError>
  const wrapper = mount(
    defineComponent({
      setup() {
        page = usePageError(covered)
        return () => h('div')
      },
    })
  )
  return { wrapper, page }
}

describe('usePageError (ginsys/shelly-manager#648)', () => {
  it('keeps a failure on the page while the page is mounted and uncovered', () => {
    const { page } = mountPage()
    page.setError(new Error('boom'), ctx)
    expect(page.error.value?.message).toBe('boom')
    expect(notice.value).toBeNull()
  })

  it('sends a failure to the notice while a modal covers the page panel', () => {
    const open = ref(true)
    const { page } = mountPage(() => open.value)
    page.setError(new Error('behind the dialog'), ctx)
    expect(notice.value?.message).toBe('behind the dialog')
    expect(notice.value?.context?.action).toBe('Apply configuration')
    expect(page.error.value).toBeNull()

    // The test reads the flag at the time of the failure, not at mount.
    dismissNotice()
    open.value = false
    page.setError(new Error('dialog closed'), ctx)
    expect(page.error.value?.message).toBe('dialog closed')
    expect(notice.value).toBeNull()
  })

  it('sends a failure to the notice after the page is unmounted', () => {
    const { wrapper, page } = mountPage()
    wrapper.unmount()
    page.setError(new Error('late'), ctx)
    expect(notice.value?.message).toBe('late')
    expect(page.error.value).toBeNull()
  })

  it('shows one notice: the next failure replaces the first', () => {
    const { wrapper, page } = mountPage(() => true)
    page.setError(new Error('first'), ctx)
    page.setError(new Error('second'), ctx)
    expect(notice.value?.message).toBe('second')
    wrapper.unmount()
  })

  it('clears the notice only by dismissNotice, never by the page clearError', () => {
    const { page } = mountPage(() => true)
    page.setError(new Error('x'), ctx)
    page.clearError()
    expect(notice.value?.message).toBe('x')
    dismissNotice()
    expect(notice.value).toBeNull()
  })

  it('clearError empties the page panel', () => {
    const { page } = mountPage()
    page.setError(new Error('x'), ctx)
    page.clearError()
    expect(page.error.value).toBeNull()
  })
})
