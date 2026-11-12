import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import DeviceLoginCard from '../DeviceLoginCard.vue'

vi.mock('@/api/devices', () => ({ updateDeviceAuth: vi.fn() }))
import { updateDeviceAuth } from '@/api/devices'

const update = vi.mocked(updateDeviceAuth)

// The device routes return the settings text with a saved password as the mask.
const maskedSettings = JSON.stringify({ auth_enabled: true, auth_user: 'admin', auth_pass: '********' })

function mountCard(settings?: string) {
  return mount(DeviceLoginCard, { props: { deviceId: 7, settings } })
}

function username(w: ReturnType<typeof mountCard>) {
  return w.get('[data-testid="device-login-username"]')
}
function password(w: ReturnType<typeof mountCard>) {
  return w.get('[data-testid="device-login-password"]')
}

describe('DeviceLoginCard', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    update.mockResolvedValue(undefined)
  })

  it('shows the stored user and the mask, and sends the mask back unchanged', async () => {
    const w = mountCard(maskedSettings)
    expect((username(w).element as HTMLInputElement).value).toBe('admin')
    expect((password(w).element as HTMLInputElement).value).toBe('********')

    await username(w).setValue('operator')
    await w.get('[data-testid="device-login-save"]').trigger('click')
    await flushPromises()

    // The server keeps the stored password when it receives the mask.
    expect(update).toHaveBeenCalledWith(7, { username: 'operator', password: '********' })
    expect(w.emitted('saved')).toHaveLength(1)
    expect(w.get('[data-testid="device-login-message"]').text()).toBe('Device login saved.')
  })

  it('sends a new password as typed and shows the mask afterwards', async () => {
    const w = mountCard(maskedSettings)
    await password(w).setValue('s3cret')
    await w.get('[data-testid="device-login-save"]').trigger('click')
    await flushPromises()

    expect(update).toHaveBeenCalledWith(7, { username: 'admin', password: 's3cret' })
    expect((password(w).element as HTMLInputElement).value).toBe('********')
  })

  it('turns device auth off when the username is empty, even with the mask in the password', async () => {
    const w = mountCard(maskedSettings)
    await username(w).setValue('')
    await w.get('[data-testid="device-login-save"]').trigger('click')
    await flushPromises()

    expect(update).toHaveBeenCalledWith(7, { username: '', password: '' })
    expect((password(w).element as HTMLInputElement).value).toBe('')
    expect(w.get('[data-testid="device-login-message"]').text()).toBe('Device authentication is off.')
  })

  it('blocks a username with no password and sends nothing', async () => {
    const w = mountCard(undefined)
    await username(w).setValue('admin')
    await w.get('[data-testid="device-login-save"]').trigger('click')
    await flushPromises()

    expect(update).not.toHaveBeenCalled()
    expect(w.get('[data-testid="device-login-message"]').text()).toContain('needs a password')
    expect(w.emitted('saved')).toBeUndefined()
  })

  it('shows the server reason and keeps what was typed when the save fails', async () => {
    update.mockRejectedValue(new Error('Device not found'))
    const w = mountCard(maskedSettings)
    await password(w).setValue('s3cret')
    await w.get('[data-testid="device-login-save"]').trigger('click')
    await flushPromises()

    expect(w.get('[data-testid="device-login-message"]').text()).toBe('Device not found')
    expect((password(w).element as HTMLInputElement).value).toBe('s3cret')
    expect(w.emitted('saved')).toBeUndefined()
  })

  it('reads the nested login that older versions wrote', () => {
    const w = mountCard(JSON.stringify({ auth: { username: 'legacy', password: '********' } }))
    expect((username(w).element as HTMLInputElement).value).toBe('legacy')
    expect((password(w).element as HTMLInputElement).value).toBe('********')
  })

  it('starts empty for settings that hold no login or are not JSON', () => {
    for (const settings of [undefined, '', '{}', '********', 'not json']) {
      const w = mountCard(settings)
      expect((username(w).element as HTMLInputElement).value).toBe('')
      expect((password(w).element as HTMLInputElement).value).toBe('')
    }
  })

  it('uses the flat pair or the legacy pair, never a mix of the two', () => {
    // A flat password with no flat user wins; the legacy user is not borrowed for it.
    const mixed = mountCard(
      JSON.stringify({ auth_user: '', auth_pass: '********', auth: { username: 'legacy', password: '********' } }),
    )
    expect((username(mixed).element as HTMLInputElement).value).toBe('')
    expect((password(mixed).element as HTMLInputElement).value).toBe('********')

    // An incomplete legacy pair is no login, and discovery's boolean "auth" is none either.
    for (const auth of [{ username: 'legacy', password: '' }, { username: '', password: '********' }, true]) {
      const w = mountCard(JSON.stringify({ auth_user: '', auth_pass: '', auth }))
      expect((username(w).element as HTMLInputElement).value).toBe('')
      expect((password(w).element as HTMLInputElement).value).toBe('')
    }
  })

  it('reloads the fields when the device settings change and nothing was typed', async () => {
    const w = mountCard(maskedSettings)
    await w.setProps({ settings: JSON.stringify({ auth_user: 'other', auth_pass: '********' }) })
    expect((username(w).element as HTMLInputElement).value).toBe('other')
  })

  it('keeps what was typed when the device settings reload', async () => {
    const w = mountCard(maskedSettings)
    await username(w).setValue('typing')
    await password(w).setValue('half-typed')
    await w.setProps({ settings: JSON.stringify({ auth_user: 'other', auth_pass: '********', note: 'x' }) })
    expect((username(w).element as HTMLInputElement).value).toBe('typing')
    expect((password(w).element as HTMLInputElement).value).toBe('half-typed')
  })

  it('locks the inputs during the save and reports what was sent', async () => {
    let finish: () => void = () => {}
    update.mockImplementation(() => new Promise<void>((resolve) => { finish = resolve }))
    const w = mountCard(maskedSettings)
    await w.get('[data-testid="device-login-save"]').trigger('click')

    // The request carries the stored user; the user cannot clear the field before it answers.
    expect((username(w).element as HTMLInputElement).disabled).toBe(true)
    expect((password(w).element as HTMLInputElement).disabled).toBe(true)
    finish()
    await flushPromises()

    expect(w.get('[data-testid="device-login-message"]').text()).toBe('Device login saved.')
    expect((username(w).element as HTMLInputElement).disabled).toBe(false)
  })

  it('reports and shows the sent login even when the fields change while the request is pending', async () => {
    let finish: () => void = () => {}
    update.mockImplementation(() => new Promise<void>((resolve) => { finish = resolve }))
    const w = mountCard(maskedSettings)
    await w.get('[data-testid="device-login-save"]').trigger('click')
    expect(update).toHaveBeenCalledWith(7, { username: 'admin', password: '********' })

    // A reload that arrives mid-request clears the fields: the sent login, not the fields, decides.
    await w.setProps({ settings: '{}' })
    expect((username(w).element as HTMLInputElement).value).toBe('')
    finish()
    await flushPromises()

    expect(w.get('[data-testid="device-login-message"]').text()).toBe('Device login saved.')
    expect((username(w).element as HTMLInputElement).value).toBe('admin')
    expect((password(w).element as HTMLInputElement).value).toBe('********')
  })
})
