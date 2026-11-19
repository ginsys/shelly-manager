import { describe, it, expect, beforeEach } from 'vitest'
import { defineComponent, h, ref } from 'vue'
import { mount } from '@vue/test-utils'
import { usePageError, useLateFailure, useLateResult, notice, noticeSeverity, dismissNotice } from '../useNotice'

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

// ginsys/shelly-manager#660: a result of an action (a success, an unconfirmed apply) reaches the user
// through the same notice once the page is gone, with its own severity.
function mountLate() {
  let late!: { failure: ReturnType<typeof useLateFailure>; result: ReturnType<typeof useLateResult> }
  const wrapper = mount(
    defineComponent({
      setup() {
        late = { failure: useLateFailure(), result: useLateResult() }
        return () => h('div')
      },
    })
  )
  return { wrapper, late }
}

describe('a late result in the app notice (ginsys/shelly-manager#660)', () => {
  it('shows nothing while the page is open: the page card shows the result', () => {
    const { wrapper, late } = mountLate()
    late.result('info', 'Applied successfully. 4 field(s) updated.', ctx)
    expect(notice.value).toBeNull()
    wrapper.unmount()
  })

  it.each([
    ['info', 'Applied successfully. 4 field(s) updated.'],
    ['warning', 'Applied, unconfirmed: find the device on its new network.'],
  ] as const)('shows a %s result once the page is gone, with that severity and no error code', (severity, message) => {
    const { wrapper, late } = mountLate()
    wrapper.unmount()
    late.result(severity, message, ctx)
    expect(notice.value?.message).toBe(message)
    expect(notice.value?.context?.action).toBe('Apply configuration')
    expect(notice.value?.code).toBe('')
    expect(notice.value?.suggestions ?? []).toEqual([])
    expect(noticeSeverity.value).toBe(severity)
  })

  it('reads "error" again when a late failure follows a late result', () => {
    const { wrapper, late } = mountLate()
    wrapper.unmount()
    late.result('info', 'Applied successfully.', ctx)
    expect(noticeSeverity.value).toBe('info')
    late.failure(new Error('device did not answer'), ctx)
    expect(noticeSeverity.value).toBe('error')
    expect(notice.value?.message).toBe('device did not answer')
  })

  it('reads the result severity when a late result follows a late failure', () => {
    const { wrapper, late } = mountLate()
    wrapper.unmount()
    late.failure(new Error('device did not answer'), ctx)
    expect(noticeSeverity.value).toBe('error')
    late.result('warning', 'Applied, unconfirmed.', ctx)
    expect(noticeSeverity.value).toBe('warning')
    expect(notice.value?.message).toBe('Applied, unconfirmed.')
  })

  it('reads "error" after a page failure replaced a result, and after Dismiss', () => {
    const { wrapper, late } = mountLate()
    wrapper.unmount()
    late.result('info', 'Applied successfully.', ctx)
    dismissNotice()
    expect(notice.value).toBeNull()
    expect(noticeSeverity.value).toBe('error')
    const { page } = mountPage(() => true)
    late.result('warning', 'Applied, unconfirmed.', ctx)
    page.setError(new Error('behind the dialog'), ctx)
    expect(noticeSeverity.value).toBe('error')
  })
})
