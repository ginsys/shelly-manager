import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { nextTick } from 'vue'
import AdminKeyDialog from '../AdminKeyDialog.vue'
import {
  promptOpen,
  promptMessage,
  requestAdminKey,
  openAdminKeyPrompt,
  rejectAdminKey,
  forgetAdminKey,
  cancelAdminKeyPrompt,
  getAdminKey,
} from '@/api/adminKey'

let wrapper: VueWrapper | null = null
let host: HTMLElement | null = null

// The global test setup replaces document.body.appendChild with a stub, so attachTo: document.body
// attaches nothing. Append the mount point through the prototype to get a real, focusable DOM.
function mountDialog() {
  host = document.createElement('div')
  Element.prototype.appendChild.call(document.body, host)
  wrapper = mount(AdminKeyDialog, { attachTo: host })
  return wrapper
}
const dialog = () => document.body.querySelector('[data-testid="admin-key-dialog"]')
const input = () => document.body.querySelector('[data-testid="admin-key-input"]') as HTMLInputElement | null
const click = (id: string) => (document.body.querySelector(`[data-testid="${id}"]`) as HTMLElement).click()

describe('AdminKeyDialog', () => {
  beforeEach(() => {
    sessionStorage.clear()
    forgetAdminKey()
    // lift any cancel left by an earlier test, then close
    openAdminKeyPrompt()
    cancelAdminKeyPrompt()
    openAdminKeyPrompt()
    promptOpen.value = false
    promptMessage.value = ''
  })
  afterEach(() => {
    wrapper?.unmount()
    wrapper = null
    host?.remove()
    host = null
  })

  it('renders nothing while the prompt is closed', () => {
    mountDialog()
    expect(dialog()).toBeNull()
  })

  it('opens by itself when a request asks for the key, and focuses the field', async () => {
    mountDialog()
    const p = requestAdminKey()
    await flushPromises()
    expect(dialog()).not.toBeNull()
    expect(input()?.type).toBe('password')
    expect(document.activeElement).toBe(input())
    cancelAdminKeyPrompt()
    await p
  })

  it('hands the typed key to the waiting request and closes', async () => {
    mountDialog()
    const p = requestAdminKey()
    await nextTick()
    input()!.value = 'typed-key'
    input()!.dispatchEvent(new Event('input'))
    click('admin-key-submit')
    await expect(p).resolves.toBe('typed-key')
    await flushPromises()
    expect(dialog()).toBeNull()
    expect(getAdminKey()).toBe('typed-key')
  })

  it('submits on Enter in the field', async () => {
    mountDialog()
    const p = requestAdminKey()
    await nextTick()
    input()!.value = 'enter-key'
    input()!.dispatchEvent(new Event('input'))
    document.body.querySelector('form')!.dispatchEvent(new Event('submit', { cancelable: true }))
    await expect(p).resolves.toBe('enter-key')
  })

  it('Cancel resolves the waiting request with null', async () => {
    mountDialog()
    const p = requestAdminKey()
    await nextTick()
    click('admin-key-cancel')
    await expect(p).resolves.toBeNull()
    await flushPromises()
    expect(dialog()).toBeNull()
  })

  it('Escape cancels', async () => {
    mountDialog()
    const p = requestAdminKey()
    await nextTick()
    input()!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await expect(p).resolves.toBeNull()
  })

  it('shows the rejection message in a live region and clears the old key from the field', async () => {
    mountDialog()
    const p = requestAdminKey()
    await nextTick()
    input()!.value = 'wrong-key'
    input()!.dispatchEvent(new Event('input'))
    click('admin-key-submit')
    await p
    rejectAdminKey('wrong-key')
    await nextTick()
    const msg = document.body.querySelector('[data-testid="admin-key-message"]') as HTMLElement
    expect(msg.textContent).toContain('That key was not accepted.')
    expect(msg.getAttribute('aria-live')).toBe('polite')
    expect(input()!.value).toBe('')
    cancelAdminKeyPrompt()
  })

  it('unmounting while a request waits settles it with null', async () => {
    const w = mountDialog()
    const p = requestAdminKey()
    await nextTick()
    w.unmount()
    wrapper = null
    await expect(p).resolves.toBeNull()
    expect(promptOpen.value).toBe(false)
  })

  it('opens by hand from the header route and never prints the key as text', async () => {
    mountDialog()
    openAdminKeyPrompt()
    await nextTick()
    input()!.value = 'do-not-echo'
    input()!.dispatchEvent(new Event('input'))
    await nextTick()
    expect(dialog()!.textContent).not.toContain('do-not-echo')
    expect(input()!.getAttribute('autocomplete')).toBe('off')
    cancelAdminKeyPrompt()
  })
})
