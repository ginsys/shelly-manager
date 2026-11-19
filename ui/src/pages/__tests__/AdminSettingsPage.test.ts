import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import AdminSettingsPage from '../AdminSettingsPage.vue'
import { rotateAdminKey } from '@/api/admin'

vi.mock('@/api/admin', () => ({ rotateAdminKey: vi.fn() }))

const rotate = vi.mocked(rotateAdminKey)

const byId = (w: ReturnType<typeof mount>, id: string) => w.find(`[data-testid="${id}"]`)

async function fill(w: ReturnType<typeof mount>, key: string, again: string) {
  await byId(w, 'admin-new-key').setValue(key)
  await byId(w, 'admin-new-key-confirm').setValue(again)
}

const submitDisabled = (w: ReturnType<typeof mount>) =>
  (byId(w, 'admin-rotate-submit').element as HTMLButtonElement).disabled

describe('AdminSettingsPage', () => {
  beforeEach(() => {
    rotate.mockReset()
  })

  it('takes the key in two password fields', () => {
    const w = mount(AdminSettingsPage)
    expect(byId(w, 'admin-new-key').attributes('type')).toBe('password')
    expect(byId(w, 'admin-new-key-confirm').attributes('type')).toBe('password')
  })

  it('says the rotation is in memory and ends at a server restart', () => {
    const text = mount(AdminSettingsPage).get('[data-testid="admin-rotate-note"]').text()
    expect(text).toMatch(/in memory/i)
    expect(text).toMatch(/restart/i)
  })

  it('keeps the button disabled until both fields hold the same non-blank key', async () => {
    const w = mount(AdminSettingsPage)
    expect(submitDisabled(w)).toBe(true)
    await fill(w, 'abc', '')
    expect(submitDisabled(w)).toBe(true)
    await fill(w, 'abc', 'abd')
    expect(submitDisabled(w)).toBe(true)
    await fill(w, 'abc', 'abc')
    expect(submitDisabled(w)).toBe(false)
  })

  it('blocks a key made only of spaces', async () => {
    const w = mount(AdminSettingsPage)
    await fill(w, '   ', '   ')
    expect(submitDisabled(w)).toBe(true)
  })

  it('shows the mismatch line only while the second field differs', async () => {
    const w = mount(AdminSettingsPage)
    expect(byId(w, 'admin-rotate-mismatch').exists()).toBe(false)
    await fill(w, 'abc', 'ab')
    expect(byId(w, 'admin-rotate-mismatch').text()).toBe('The two keys do not match.')
    await fill(w, 'abc', 'abc')
    expect(byId(w, 'admin-rotate-mismatch').exists()).toBe(false)
  })

  it.each([
    [' k3y', ' k3y'],
    ['k3y ', 'k3y '],
    ['\tk3y', '\tk3y'],
  ])('refuses a key with whitespace at an edge: %j', async (key, again) => {
    const w = mount(AdminSettingsPage)
    await fill(w, key, again)
    expect(submitDisabled(w)).toBe(true)
    expect(byId(w, 'admin-rotate-edge-space').text()).toBe('A key cannot start or end with a space.')
  })

  it('allows spaces inside the key and sends the key exactly as typed', async () => {
    rotate.mockResolvedValue(true)
    const w = mount(AdminSettingsPage)
    await fill(w, 'pass  phrase', 'pass  phrase')
    expect(byId(w, 'admin-rotate-edge-space').exists()).toBe(false)
    expect(submitDisabled(w)).toBe(false)
    await w.get('form').trigger('submit')
    await flushPromises()
    expect(rotate).toHaveBeenCalledTimes(1)
    expect(rotate).toHaveBeenCalledWith('pass  phrase')
  })

  it('sends nothing when a submit skips the disabled button with a mismatch', async () => {
    const w = mount(AdminSettingsPage)
    await fill(w, 'abc', 'abd')
    await w.get('form').trigger('submit')
    await flushPromises()
    expect(rotate).not.toHaveBeenCalled()
  })

  it('sends nothing when a submit skips the disabled button with a key that has an edge space', async () => {
    const w = mount(AdminSettingsPage)
    await fill(w, 'abc ', 'abc ')
    await w.get('form').trigger('submit')
    await flushPromises()
    expect(rotate).not.toHaveBeenCalled()
  })

  it('clears both fields and says what changed after a rotation', async () => {
    rotate.mockResolvedValue(true)
    const w = mount(AdminSettingsPage)
    await fill(w, 'abc', 'abc')
    await w.get('form').trigger('submit')
    await flushPromises()
    expect((byId(w, 'admin-new-key').element as HTMLInputElement).value).toBe('')
    expect((byId(w, 'admin-new-key-confirm').element as HTMLInputElement).value).toBe('')
    expect(byId(w, 'admin-rotate-message').text()).toMatch(/rotated/i)
    expect(byId(w, 'admin-rotate-message').text()).toMatch(/this tab/i)
  })

  it('keeps the fields and shows the error when the rotation fails', async () => {
    rotate.mockRejectedValue(new Error('Admin authorization required'))
    const w = mount(AdminSettingsPage)
    await fill(w, 'abc', 'abc')
    await w.get('form').trigger('submit')
    await flushPromises()
    expect((byId(w, 'admin-new-key').element as HTMLInputElement).value).toBe('abc')
    expect((byId(w, 'admin-new-key-confirm').element as HTMLInputElement).value).toBe('abc')
    expect(byId(w, 'admin-rotate-message').text()).toBe('Admin authorization required')
  })

  it('disables the button while the call runs', async () => {
    let finish!: (v: boolean) => void
    rotate.mockReturnValue(new Promise<boolean>((resolve) => (finish = resolve)))
    const w = mount(AdminSettingsPage)
    await fill(w, 'abc', 'abc')
    await w.get('form').trigger('submit')
    expect(submitDisabled(w)).toBe(true)
    finish(true)
    await flushPromises()
  })
})
