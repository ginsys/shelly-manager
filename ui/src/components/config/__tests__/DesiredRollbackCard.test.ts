import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, enableAutoUnmount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import DesiredRollbackCard from '../DesiredRollbackCard.vue'
import { notice as appNotice, dismissNotice } from '@/composables/useNotice'

vi.mock('@/api/desiredRollback', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/desiredRollback')>()
  return { ...actual, getDesiredHistory: vi.fn(), rollbackDesired: vi.fn() }
})

import { getDesiredHistory, rollbackDesired, RollbackRefused } from '@/api/desiredRollback'

// ginsys/shelly-manager#775 slice 3: the card that lists the versions of the desired configuration and rolls
// the device's desired configuration back to one. No device is contacted: the card only calls the two routes.
const SENTINEL = 'sentinel-secret-9f3a'
const MASK = '********'

const version = (id: number, over: Record<string, unknown> = {}) => ({
  id,
  created_at: '2026-11-20T10:00:00Z',
  action: 'recompute',
  changed_by: 'user',
  restored_from: null,
  new_desired: JSON.stringify({ mqtt: { server: 'old.example', pass: MASK }, sleep: { enable: true } }),
  ...over,
})

const current = { mqtt: { server: 'new.example' }, wifi: { ssid: 'office' } }
const templates = [{ id: 1, name: 'Base' }, { id: 2, name: 'Kitchen' }]
const result = (over: Record<string, unknown> = {}) => ({
  version_id: 9,
  no_change: false,
  detached_template_ids: [1, 2],
  message: 'Desired configuration set to version 9. The device is unchanged until you apply it.',
  ...over,
})
const refused = (status: number, message: string, paths: string[] = []) => new RollbackRefused(status, 'X', message, paths)

let pinia: ReturnType<typeof createPinia>
enableAutoUnmount(afterEach)

function mountCard(over: Record<string, unknown> = {}) {
  return mount(DesiredRollbackCard, {
    props: { deviceId: '7', current, templates, editing: false, ready: true, ...over },
    global: { plugins: [pinia] },
    attachTo: document.body,
  })
}

async function open(wrapper: VueWrapper, id = 5) {
  await wrapper.get(`[data-testid="rollback-${id}"]`).trigger('click')
}
const confirm = async (wrapper: VueWrapper) => {
  await wrapper.get('[data-testid="rollback-confirm"]').trigger('click')
  await flushPromises()
}
const dialogText = (wrapper: VueWrapper) => wrapper.get('[data-testid="rollback-dialog"]').text()

describe('DesiredRollbackCard', () => {
  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    vi.resetAllMocks()
    dismissNotice()
    vi.mocked(getDesiredHistory).mockResolvedValue([version(5), version(4, { action: 'rollback', restored_from: 2 })] as any)
  })

  describe('the versions', () => {
    it('lists them as the server gives them, newest first, with time, action, who and restored-from', async () => {
      const wrapper = mountCard()
      await flushPromises()
      expect(getDesiredHistory).toHaveBeenCalledWith('7')
      const rows = wrapper.findAll('[data-testid="version-entry"]')
      expect(rows.map((r) => r.text())).toEqual([
        expect.stringContaining('Version 5'),
        expect.stringContaining('Version 4'),
      ])
      expect(rows[0].text()).toContain(new Date('2026-11-20T10:00:00Z').toLocaleString())
      expect(rows[0].text()).toContain('recompute')
      expect(rows[0].text()).toContain('user')
      expect(rows[1].text()).toContain('rollback')
      expect(rows[1].text()).toContain('restored from version 2')
    })

    it('names itself apart from the History card, which lists imports', async () => {
      const wrapper = mountCard()
      await flushPromises()
      expect(wrapper.get('h2').text()).toBe('Desired configuration versions')
      expect(wrapper.text()).toContain('What Apply sends to the device')
      expect(wrapper.text()).toContain('History card')
    })

    it('says that a version with no stored text cannot be rolled back to, and gives it no button', async () => {
      vi.mocked(getDesiredHistory).mockResolvedValue([version(5, { new_desired: null }), version(4)] as any)
      const wrapper = mountCard()
      await flushPromises()
      expect(wrapper.get('[data-testid="version-no-text"]').text()).toContain('cannot be rolled back to')
      expect(wrapper.find('[data-testid="rollback-5"]').exists()).toBe(false)
      expect(wrapper.find('[data-testid="rollback-4"]').exists()).toBe(true)
    })

    it('shows a failed read in the card and an empty list as such', async () => {
      vi.mocked(getDesiredHistory).mockRejectedValue(new Error('boom'))
      const failed = mountCard()
      await flushPromises()
      expect(failed.get('[data-testid="versions-error"]').text()).toBe('boom')
      vi.mocked(getDesiredHistory).mockResolvedValue([])
      const empty = mountCard()
      await flushPromises()
      expect(empty.get('[data-testid="versions-empty"]').exists()).toBe(true)
    })

    it('keeps Roll back off, with its reason, while the overrides editor is open, and opens nothing', async () => {
      const wrapper = mountCard({ editing: true })
      await flushPromises()
      const button = wrapper.get('[data-testid="rollback-5"]')
      expect(button.attributes('disabled')).toBeDefined()
      expect(button.attributes('title')).toBe('Save or cancel the overrides edit first')
      await button.trigger('click')
      expect(wrapper.find('[data-testid="rollback-dialog"]').exists()).toBe(false)
      expect(rollbackDesired).not.toHaveBeenCalled()
    })
  })

  describe('the confirm', () => {
    it('shows the difference, the templates to detach, the way back and that the device is unchanged, and posts nothing yet', async () => {
      const wrapper = mountCard()
      await flushPromises()
      await open(wrapper)
      const lines = wrapper.findAll('[data-testid="rollback-diff-line"]').map((l) => l.text().replace(/\s+/g, ' '))
      expect(lines).toEqual([
        'mqtt.server: changed from "new.example" to "old.example"',
        'sleep.enable: added to true',
        'wifi.ssid: removed from "office"',
      ])
      expect(wrapper.get('[data-testid="rollback-detach"]').text()).toBe('These templates are detached: Base, Kitchen.')
      expect(wrapper.get('[data-testid="rollback-way-back"]').text()).toContain('re-attach the templates listed above, then clear the device overrides')
      expect(wrapper.get('[data-testid="rollback-way-back"]').text()).toContain('neither contacts the device')
      expect(dialogText(wrapper)).toContain('The device is unchanged until you apply it.')
      expect(wrapper.get('[data-testid="rollback-secrets-note"]').text()).toContain('Secrets are not compared')
      expect(rollbackDesired).not.toHaveBeenCalled()
    })

    it('does not show a secret as a change when the version masks it and the current holds none', async () => {
      const wrapper = mountCard()
      await flushPromises()
      await open(wrapper)
      expect(dialogText(wrapper)).not.toContain('mqtt.pass')
    })

    it('says so when no template is attached, and when the text cannot be read as settings', async () => {
      vi.mocked(getDesiredHistory).mockResolvedValue([version(5, { new_desired: MASK })] as any)
      const wrapper = mountCard({ templates: [] })
      await flushPromises()
      await open(wrapper)
      expect(wrapper.get('[data-testid="rollback-detach"]').text()).toBe('No template is attached.')
      expect(wrapper.get('[data-testid="rollback-no-diff"]').text()).toContain('could not be read as settings')
    })

    // The dialog does not trap the keyboard: the overrides editor can be opened behind it after it opened.
    it('refuses to start while the overrides editor is open behind the dialog, and says why', async () => {
      const wrapper = mountCard()
      await flushPromises()
      await open(wrapper)
      await wrapper.setProps({ editing: true })
      expect(wrapper.get('[data-testid="rollback-confirm"]').attributes('disabled')).toBeDefined()
      await (wrapper.vm as any).confirm()
      expect(rollbackDesired).not.toHaveBeenCalled()
      expect(wrapper.get('[data-testid="rollback-refusal"]').text()).toContain('Save or cancel the overrides edit first')
    })

    // The card is always on the page, so it can be used before the page has read the current configuration: the
    // difference and the detach list would then compare with nothing.
    it('keeps Roll back off, with its reason, until the page has read the configuration, and confirm refuses when it is being read again', async () => {
      const wrapper = mountCard({ ready: false })
      await flushPromises()
      const button = wrapper.get('[data-testid="rollback-5"]')
      expect(button.attributes('disabled')).toBeDefined()
      expect(button.attributes('title')).toBe('Wait until the configuration has been read')
      await wrapper.setProps({ ready: true })
      await open(wrapper)
      await wrapper.setProps({ ready: false })
      expect(wrapper.get('[data-testid="rollback-confirm"]').attributes('disabled')).toBeDefined()
      await (wrapper.vm as any).confirm()
      expect(rollbackDesired).not.toHaveBeenCalled()
      expect(wrapper.get('[data-testid="rollback-refusal"]').text()).toContain('being read again')
    })

    it('reads the versions again each time the page finishes reading, and not when it starts to', async () => {
      const wrapper = mountCard()
      await flushPromises()
      const reads = () => vi.mocked(getDesiredHistory).mock.calls.length
      const first = reads()
      await wrapper.setProps({ ready: false })
      expect(reads()).toBe(first)
      await wrapper.setProps({ ready: true })
      await flushPromises()
      expect(reads()).toBe(first + 1)
    })

    it('cancel closes the dialog and posts nothing', async () => {
      const wrapper = mountCard()
      await flushPromises()
      await open(wrapper)
      await wrapper.get('[data-testid="rollback-cancel"]').trigger('click')
      expect(wrapper.find('[data-testid="rollback-dialog"]').exists()).toBe(false)
      expect(rollbackDesired).not.toHaveBeenCalled()
    })
  })

  describe('the answer', () => {
    it('shows the server message word for word and asks the page to read again; the page finishing is the cue to read the versions', async () => {
      vi.mocked(rollbackDesired).mockResolvedValue(result() as any)
      const wrapper = mountCard()
      await flushPromises()
      await open(wrapper)
      await confirm(wrapper)
      expect(rollbackDesired).toHaveBeenCalledWith('7', 5, undefined)
      expect(wrapper.get('[data-testid="rollback-notice"]').text()).toContain(
        'Desired configuration set to version 9. The device is unchanged until you apply it.',
      )
      expect(wrapper.emitted('refresh')).toHaveLength(1)
      expect(getDesiredHistory).toHaveBeenCalledTimes(1)
      expect(wrapper.find('[data-testid="rollback-dialog"]').exists()).toBe(false)
    })

    it('shows the server message of an answer that wrote nothing', async () => {
      const message = 'Desired configuration already is version 5. Nothing was written.'
      vi.mocked(rollbackDesired).mockResolvedValue(result({ no_change: true, message }) as any)
      const wrapper = mountCard()
      await flushPromises()
      await open(wrapper)
      await confirm(wrapper)
      expect(wrapper.get('[data-testid="rollback-notice"]').text()).toContain(message)
    })

    it.each([
      ['a 422 with paths names them and offers no input', refused(422, 'Would change on recompute.', ['system.sntp.server', 'a.b']), true],
      ['a 422 with no paths shows the server text', refused(422, 'This version has no stored text.'), false],
      ['a 400 shows the server text', refused(400, 'Invalid JSON request body'), false],
      ['a 404 shows the server text', refused(404, 'Desired configuration version not found'), false],
      ['no answer shows its fixed text', refused(0, 'The rollback request got no answer from the server.'), false],
    ])('%s, and reports nothing as rolled back', async (_name, err, withPaths) => {
      vi.mocked(rollbackDesired).mockRejectedValue(err)
      const wrapper = mountCard()
      await flushPromises()
      await open(wrapper)
      await confirm(wrapper)
      const refusal = wrapper.get('[data-testid="rollback-refusal"]').text()
      expect(refusal).toContain(err.message)
      if (withPaths) for (const p of err.paths) expect(refusal).toContain(p)
      expect(wrapper.find('[data-testid="rollback-secrets"]').exists()).toBe(false)
      expect(wrapper.find('[data-testid="rollback-notice"]').exists()).toBe(false)
      expect(wrapper.emitted('refresh')).toBeUndefined()
      expect(wrapper.find('[data-testid="rollback-dialog"]').exists()).toBe(true)
    })
  })

  describe('the secrets a version needs', () => {
    const needSecrets = (...paths: string[]) => refused(409, 'This version holds secrets that history does not keep.', paths)

    it('keeps the dialog open with one input per listed path, and retries with exactly those keys', async () => {
      vi.mocked(rollbackDesired).mockRejectedValueOnce(needSecrets('mqtt.pass', 'wifi.password')).mockResolvedValueOnce(result() as any)
      const wrapper = mountCard()
      await flushPromises()
      await open(wrapper)
      await confirm(wrapper)

      expect(wrapper.find('[data-testid="rollback-dialog"]').exists()).toBe(true)
      expect(wrapper.get('[data-testid="rollback-refusal"]').text()).toContain('holds secrets that history does not keep')
      expect(wrapper.findAll('[data-testid="rollback-secrets"] input').map((i) => i.attributes('data-testid'))).toEqual([
        'secret-mqtt.pass',
        'secret-wifi.password',
      ])
      await wrapper.get('[data-testid="secret-mqtt.pass"]').setValue('p1')
      await wrapper.get('[data-testid="secret-wifi.password"]').setValue('p2')
      await confirm(wrapper)

      expect(rollbackDesired).toHaveBeenNthCalledWith(2, '7', 5, { 'mqtt.pass': 'p1', 'wifi.password': 'p2' })
      expect(wrapper.emitted('refresh')).toHaveLength(1)
    })

    it('takes a password input that no browser stores or submits', async () => {
      vi.mocked(rollbackDesired).mockRejectedValue(needSecrets('mqtt.pass', 'actions."a".0.secret_urls'))
      const wrapper = mountCard()
      await flushPromises()
      await open(wrapper)
      await confirm(wrapper)
      const input = wrapper.get('input[data-testid="secret-mqtt.pass"]')
      expect(input.attributes('type')).toBe('password')
      expect(input.attributes('autocomplete')).toBe('new-password')
      expect(wrapper.get('textarea').attributes('autocomplete')).toBe('off')
      expect(wrapper.find('form').exists()).toBe(false)
    })

    it.each([
      ['actions."door".0.secret_urls', true],
      ['actions."door".0.secret_urls#1', true],
      ['mqtt.pass', false],
      ['x.secret_urls', false],
      ['actions."door".0.secret_urls_extra', false],
    ])('reads the path %s as a list input: %s', async (path, isList) => {
      vi.mocked(rollbackDesired).mockRejectedValue(needSecrets(path))
      const wrapper = mountCard()
      await flushPromises()
      await open(wrapper)
      await confirm(wrapper)
      expect(wrapper.find('textarea').exists()).toBe(isList)
      expect(wrapper.find('input[type="password"]').exists()).toBe(!isList)
    })

    it('sends a list slot as a list, one URL per line, with the blank lines dropped', async () => {
      const key = 'actions."door".0.secret_urls'
      vi.mocked(rollbackDesired).mockRejectedValueOnce(needSecrets(key)).mockResolvedValueOnce(result() as any)
      const wrapper = mountCard()
      await flushPromises()
      await open(wrapper)
      await confirm(wrapper)
      await wrapper.get('textarea').setValue('http://a.example/x\n\n  \n http://b.example/y \n')
      await confirm(wrapper)
      expect(rollbackDesired).toHaveBeenNthCalledWith(2, '7', 5, { [key]: ['http://a.example/x', 'http://b.example/y'] })
    })

    it.each([[''], ['\n'], ['  \n \n']])('sends no request for an empty input (%j)', async (text) => {
      const key = 'actions."door".0.secret_urls'
      vi.mocked(rollbackDesired).mockRejectedValueOnce(needSecrets('mqtt.pass', key))
      const wrapper = mountCard()
      await flushPromises()
      await open(wrapper)
      await confirm(wrapper)
      await wrapper.get('[data-testid="secret-mqtt.pass"]').setValue('p1')
      await wrapper.get('textarea').setValue(text)
      await confirm(wrapper)
      expect(rollbackDesired).toHaveBeenCalledTimes(1)
      expect(wrapper.get('[data-testid="rollback-refusal"]').text()).toContain('Enter a value for every listed path: ' + key + '.')
      expect(wrapper.get('[data-testid="rollback-refusal"]').text()).toContain(key)
      expect(wrapper.findAll('[data-testid="rollback-secrets"] input, [data-testid="rollback-secrets"] textarea')).toHaveLength(2)
    })

    it('after a refused retry keeps the listed paths with empty inputs and the server message', async () => {
      vi.mocked(rollbackDesired)
        .mockRejectedValueOnce(needSecrets('mqtt.pass'))
        .mockRejectedValueOnce(refused(400, 'The secrets entered are not an answer to the list.'))
      const wrapper = mountCard()
      await flushPromises()
      await open(wrapper)
      await confirm(wrapper)
      await wrapper.get('[data-testid="secret-mqtt.pass"]').setValue(SENTINEL)
      await confirm(wrapper)

      expect(wrapper.get('[data-testid="rollback-refusal"]').text()).toContain('not an answer to the list')
      const input = wrapper.get('[data-testid="secret-mqtt.pass"]')
      expect((input.element as HTMLInputElement).value).toBe('')
      expect(wrapper.get('[data-testid="rollback-confirm"]').text()).toBe('Retry with these secrets')
    })

    it('replaces the list when a second 409 names other paths', async () => {
      vi.mocked(rollbackDesired).mockRejectedValueOnce(needSecrets('mqtt.pass')).mockRejectedValueOnce(needSecrets('auth.pass', 'wifi.password'))
      const wrapper = mountCard()
      await flushPromises()
      await open(wrapper)
      await confirm(wrapper)
      await wrapper.get('[data-testid="secret-mqtt.pass"]').setValue('p1')
      await confirm(wrapper)
      expect(wrapper.findAll('[data-testid="rollback-secrets"] input').map((i) => i.attributes('data-testid'))).toEqual([
        'secret-auth.pass',
        'secret-wifi.password',
      ])
    })

    it('reads a 409 with no paths as "changed while prepared": shows it, asks the page to read again and shows no form', async () => {
      vi.mocked(rollbackDesired).mockRejectedValue(refused(409, 'The desired configuration changed while the rollback was prepared. Read it again and retry.'))
      const wrapper = mountCard()
      await flushPromises()
      await open(wrapper)
      await confirm(wrapper)
      expect(wrapper.find('[data-testid="rollback-secrets"]').exists()).toBe(false)
      expect(wrapper.find('[data-testid="rollback-dialog"]').exists()).toBe(false)
      expect(wrapper.get('[data-testid="rollback-notice"]').text()).toContain('changed while the rollback was prepared')
      expect(wrapper.emitted('refresh')).toHaveLength(1)
      expect(getDesiredHistory).toHaveBeenCalledTimes(1)
    })
  })

  describe('where a typed secret may be', () => {
    let spies: ReturnType<typeof vi.spyOn>[]
    beforeEach(() => {
      spies = (['log', 'info', 'warn', 'error', 'debug'] as const).map((m) => vi.spyOn(console, m).mockImplementation(() => {}))
    })
    afterEach(() => spies.forEach((s) => s.mockRestore()))

    // The error an axios request leaves behind holds the request body. The api function never lets it out, but
    // the card must stay clean even when a refusal carries the text: nothing it shows or stores may hold it.
    function expectNoSentinel() {
      const holders: Record<string, string> = {
        dom: document.body.innerHTML,
        inputs: JSON.stringify([...document.querySelectorAll('input, textarea')].map((i) => (i as HTMLInputElement).value)),
        pinia: JSON.stringify(pinia.state.value),
        localStorage: JSON.stringify({ ...localStorage }),
        sessionStorage: JSON.stringify({ ...sessionStorage }),
        location: window.location.href,
        appNotice: JSON.stringify(appNotice.value),
        console: JSON.stringify(spies.flatMap((s) => s.mock.calls.map((c) => c.map(String)))),
      }
      for (const [where, text] of Object.entries(holders)) expect(text, where).not.toContain(SENTINEL)
    }

    const needSecrets = () => refused(409, 'This version holds secrets that history does not keep.', ['mqtt.pass'])

    it.each([
      ['success', () => vi.mocked(rollbackDesired).mockResolvedValueOnce(result() as any)],
      ['a refused retry', () => vi.mocked(rollbackDesired).mockRejectedValueOnce(refused(400, 'Invalid JSON request body'))],
      ['a second 409', () => vi.mocked(rollbackDesired).mockRejectedValueOnce(needSecrets())],
      ['no answer', () => vi.mocked(rollbackDesired).mockRejectedValueOnce(refused(0, 'The rollback request got no answer from the server.'))],
    ])('is nowhere after %s', async (_name, answer) => {
      vi.mocked(rollbackDesired).mockRejectedValueOnce(needSecrets())
      answer()
      const wrapper = mountCard()
      await flushPromises()
      await open(wrapper)
      await confirm(wrapper)
      await wrapper.get('[data-testid="secret-mqtt.pass"]').setValue(SENTINEL)
      expect(wrapper.html()).not.toContain(SENTINEL) // a typed value is a property, never markup
      await confirm(wrapper)
      expect(vi.mocked(rollbackDesired).mock.calls[1][2]).toEqual({ 'mqtt.pass': SENTINEL })
      expectNoSentinel()
    })

    it('is nowhere after cancel', async () => {
      vi.mocked(rollbackDesired).mockRejectedValueOnce(needSecrets())
      const wrapper = mountCard()
      await flushPromises()
      await open(wrapper)
      await confirm(wrapper)
      await wrapper.get('[data-testid="secret-mqtt.pass"]').setValue(SENTINEL)
      await wrapper.get('[data-testid="rollback-cancel"]').trigger('click')
      expect((wrapper.vm as any).entered).toEqual({}) // not even held in memory
      expectNoSentinel()
      await open(wrapper) // a reopened dialog starts without it
      expectNoSentinel()
    })

    it('is nowhere after the card goes, and a late answer fills nothing', async () => {
      let release: (v: unknown) => void = () => {}
      vi.mocked(rollbackDesired)
        .mockRejectedValueOnce(needSecrets())
        .mockReturnValueOnce(new Promise((resolve) => (release = resolve)) as any)
      const wrapper = mountCard()
      await flushPromises()
      await open(wrapper)
      await confirm(wrapper)
      await wrapper.get('[data-testid="secret-mqtt.pass"]').setValue(SENTINEL)
      await wrapper.get('[data-testid="rollback-confirm"]').trigger('click')
      const calls = vi.mocked(getDesiredHistory).mock.calls.length

      const vm = wrapper.vm as any
      wrapper.unmount()
      release(result())
      await flushPromises()

      expect(vm.entered).toEqual({}) // the typed text does not outlive the card
      expect(wrapper.emitted('refresh')).toBeUndefined()
      expect(getDesiredHistory).toHaveBeenCalledTimes(calls)
      expectNoSentinel()
    })
  })

  describe('a late answer', () => {
    it('reaches the app notice as a result when it is a success (a write happened), and never as a failure', async () => {
      let release: (v: unknown) => void = () => {}
      vi.mocked(rollbackDesired).mockReturnValueOnce(new Promise((resolve) => (release = resolve)) as any)
      const wrapper = mountCard()
      await flushPromises()
      await open(wrapper)
      await wrapper.get('[data-testid="rollback-confirm"]').trigger('click')
      wrapper.unmount()
      release(result())
      await flushPromises()
      expect(appNotice.value?.message).toBe(result().message)
    })

    it('drops a refusal that arrives after the card went', async () => {
      let rejectPost: (e: unknown) => void = () => {}
      vi.mocked(rollbackDesired).mockReturnValueOnce(new Promise((_r, reject) => (rejectPost = reject)) as any)
      const wrapper = mountCard()
      await flushPromises()
      await open(wrapper)
      await wrapper.get('[data-testid="rollback-confirm"]').trigger('click')
      wrapper.unmount()
      rejectPost(refused(422, 'late'))
      await flushPromises()
      expect(appNotice.value).toBeNull()
      expect(wrapper.emitted('refresh')).toBeUndefined()
    })

    it('drops a list that arrives after the card went', async () => {
      let releaseList: (v: unknown) => void = () => {}
      vi.mocked(getDesiredHistory).mockReturnValueOnce(new Promise((resolve) => (releaseList = resolve)) as any)
      const wrapper = mountCard()
      wrapper.unmount()
      releaseList([version(5)])
      await flushPromises()
      expect(document.body.innerHTML).not.toContain('Version 5')
      expect(appNotice.value).toBeNull()
    })
  })
})
