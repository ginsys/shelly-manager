import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import BulkOperationsDialog from '../BulkOperationsDialog.vue'
import { bulkImport, bulkDriftDetect, bulkDownload } from '@/api/bulk'

vi.mock('@/api/bulk', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/bulk')>()
  return {
    ...actual,
    bulkImport: vi.fn(),
    bulkDriftDetect: vi.fn(),
    bulkDriftDetectEnhanced: vi.fn(),
    bulkDownload: vi.fn()
  }
})

const picked = [
  { id: 4, name: 'Kitchen', ip: '10.0.0.4' },
  { id: 9, name: 'Garage', ip: '10.0.0.9' }
]

const importAnswer = {
  total: 2,
  success: 2,
  failed: 0,
  skipped: 0,
  results: [
    { device_id: 4, device_name: 'Kitchen', device_ip: '10.0.0.4', status: 'success' },
    { device_id: 9, device_name: 'Garage', device_ip: '10.0.0.9', status: 'success' }
  ]
}

function open(selectedDevices = picked) {
  return mount(BulkOperationsDialog, { props: { isOpen: true, selectedDevices } })
}

const byId = (w: ReturnType<typeof mount>, id: string) => w.find(`[data-testid="${id}"]`)

describe('BulkOperationsDialog', () => {
  let alertSpy: ReturnType<typeof vi.fn>

  beforeEach(() => {
    vi.clearAllMocks()
    alertSpy = vi.fn()
    vi.stubGlobal('alert', alertSpy)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('renders nothing while closed', () => {
    const wrapper = mount(BulkOperationsDialog, { props: { isOpen: false, selectedDevices: picked } })

    expect(wrapper.find('.modal').exists()).toBe(false)
  })

  it('lists the picked devices by name and address, above the operation choice', () => {
    const wrapper = open()

    const rows = wrapper.findAll('[data-testid="bulk-picked-device"]')
    expect(rows.map((r) => r.text())).toEqual([
      expect.stringContaining('Kitchen'),
      expect.stringContaining('Garage')
    ])
    expect(rows[0].text()).toContain('10.0.0.4')
    expect(wrapper.text()).toContain('2 device(s) selected')
  })

  it('asks the page to drop a device when its remove control is pressed', async () => {
    const wrapper = open()

    await wrapper.findAll('[data-testid="bulk-remove-device"]')[1].trigger('click')

    expect(wrapper.emitted('remove')).toEqual([[9]])
  })

  it('sends exactly the picked ids and the stop option', async () => {
    vi.mocked(bulkImport).mockResolvedValue(importAnswer as any)
    const wrapper = open()

    await byId(wrapper, 'bulk-op-import').trigger('click')
    await byId(wrapper, 'bulk-stop-on-error').setValue(true)
    await byId(wrapper, 'bulk-submit').trigger('submit')
    await flushPromises()

    expect(bulkImport).toHaveBeenCalledTimes(1)
    expect(bulkImport).toHaveBeenCalledWith({
      device_ids: [4, 9],
      options: { stop_on_error: true }
    })
  })

  it('shows the results and tells the page it is complete', async () => {
    vi.mocked(bulkImport).mockResolvedValue(importAnswer as any)
    const wrapper = open()

    await byId(wrapper, 'bulk-op-import').trigger('click')
    await byId(wrapper, 'bulk-submit').trigger('submit')
    await flushPromises()

    expect(byId(wrapper, 'bulk-results').exists()).toBe(true)
    expect(byId(wrapper, 'bulk-results').text()).toContain('Kitchen')
    expect(wrapper.emitted('complete')).toHaveLength(1)
  })

  it('shows a refused request as an error in the dialog and raises no browser alert', async () => {
    vi.mocked(bulkDriftDetect).mockRejectedValue(new Error('device_ids is required'))
    const wrapper = open()

    await byId(wrapper, 'bulk-op-drift').trigger('click')
    await byId(wrapper, 'bulk-submit').trigger('submit')
    await flushPromises()

    expect(byId(wrapper, 'bulk-error').text()).toContain('device_ids is required')
    expect(alertSpy).not.toHaveBeenCalled()
    // Not stuck on "Processing...": the form is back and can be sent again.
    expect(wrapper.text()).not.toContain('Processing')
    expect(byId(wrapper, 'bulk-submit').exists()).toBe(true)
    expect(wrapper.emitted('complete')).toBeUndefined()
  })

  it('clears an earlier error when the request is sent again', async () => {
    vi.mocked(bulkImport)
      .mockRejectedValueOnce(new Error('upstream failed'))
      .mockResolvedValueOnce(importAnswer as any)
    const wrapper = open()
    await byId(wrapper, 'bulk-op-import').trigger('click')
    await byId(wrapper, 'bulk-submit').trigger('submit')
    await flushPromises()
    expect(byId(wrapper, 'bulk-error').exists()).toBe(true)

    await byId(wrapper, 'bulk-submit').trigger('submit')
    await flushPromises()

    expect(byId(wrapper, 'bulk-error').exists()).toBe(false)
    expect(byId(wrapper, 'bulk-results').exists()).toBe(true)
  })

  it('starts nothing when no device is picked', async () => {
    const wrapper = open([])

    expect(byId(wrapper, 'bulk-empty').exists()).toBe(true)
    expect(byId(wrapper, 'bulk-op-import').attributes('disabled')).toBeDefined()
    expect(byId(wrapper, 'bulk-op-drift').attributes('disabled')).toBeDefined()
    expect(byId(wrapper, 'bulk-op-drift-enhanced').attributes('disabled')).toBeDefined()
    expect(byId(wrapper, 'bulk-op-download').attributes('disabled')).toBeDefined()

    await byId(wrapper, 'bulk-op-import').trigger('click')
    expect(bulkImport).not.toHaveBeenCalled()
  })

  it('sends nothing when the selection empties after the form was opened', async () => {
    const wrapper = open()
    await byId(wrapper, 'bulk-op-import').trigger('click')

    await wrapper.setProps({ selectedDevices: [] })
    await byId(wrapper, 'bulk-submit').trigger('submit')
    await flushPromises()

    expect(bulkImport).not.toHaveBeenCalled()
  })

  it('ignores the answer of a request sent before the dialog was closed and reopened', async () => {
    let answer!: (value: unknown) => void
    vi.mocked(bulkImport).mockReturnValue(new Promise((resolve) => (answer = resolve)) as any)
    const wrapper = open([picked[0]])
    await byId(wrapper, 'bulk-op-import').trigger('click')
    await byId(wrapper, 'bulk-submit').trigger('submit')

    await byId(wrapper, 'bulk-close').trigger('click')
    await wrapper.setProps({ selectedDevices: [picked[1]] })
    await byId(wrapper, 'bulk-op-drift').trigger('click')
    answer(importAnswer)
    await flushPromises()

    expect(byId(wrapper, 'bulk-results').exists()).toBe(false)
    expect(byId(wrapper, 'bulk-submit').exists()).toBe(true)
    // The devices were contacted, so the page is still told to refresh its list.
    expect(wrapper.emitted('complete')).toHaveLength(1)
  })

  describe('while the page checks the selection', () => {
    const cards = ['bulk-op-import', 'bulk-op-drift', 'bulk-op-drift-enhanced', 'bulk-op-download']

    it('says how many devices it is checking and offers no operation yet', () => {
      const wrapper = mount(BulkOperationsDialog, {
        props: { isOpen: true, selectedDevices: picked, checking: true }
      })

      expect(byId(wrapper, 'bulk-checking').text()).toBe('Checking 2 devices...')
      for (const id of cards) {
        expect(byId(wrapper, id).attributes('disabled')).toBeDefined()
      }
    })

    it('shows no checking line and enables the operations once the check is over', () => {
      const wrapper = mount(BulkOperationsDialog, {
        props: { isOpen: true, selectedDevices: picked, checking: false }
      })

      expect(byId(wrapper, 'bulk-checking').exists()).toBe(false)
      for (const id of cards) {
        expect(byId(wrapper, id).attributes('disabled')).toBeUndefined()
      }
    })

    it('shows neither the checking line nor the removed line by default', () => {
      const wrapper = open()

      expect(byId(wrapper, 'bulk-checking').exists()).toBe(false)
      expect(byId(wrapper, 'bulk-removed').exists()).toBe(false)
    })

    it('says in one line how many selected devices were removed because they no longer exist', () => {
      const plural = mount(BulkOperationsDialog, {
        props: { isOpen: true, selectedDevices: picked, removedCount: 2 }
      })
      const singular = mount(BulkOperationsDialog, {
        props: { isOpen: true, selectedDevices: picked, removedCount: 1 }
      })

      expect(byId(plural, 'bulk-removed').text()).toBe('2 selected devices no longer exist and were removed')
      expect(byId(plural, 'bulk-removed').attributes('role')).toBe('status')
      expect(byId(singular, 'bulk-removed').text()).toBe('1 selected device no longer exists and was removed')
    })
  })

  describe('download configuration', () => {
    const readable = {
      device_id: 4,
      device_name: 'Kitchen',
      status: 'ok',
      config: { system: { device: { name: 'config-marker-xyz' } }, mqtt: { pass: '********' } }
    }
    const unreadable = {
      device_id: 9,
      device_name: 'Garage',
      status: 'error',
      error: 'no desired configuration is set for this device'
    }
    const answerOf = (devices: unknown[], filename: string | null = 'shelly-configs-20261120-002000.json') => ({
      blob: new Blob(['{}']),
      // null stands for a server that sent no Content-Disposition name
      filename: filename ?? undefined,
      file: { generated_at: '2026-11-20T00:20:00Z', devices }
    })

    let clicks: { download: string; href: string }[]
    const createUrl = vi.fn()
    const revokeUrl = vi.fn()

    // The shared test setup answers document.createElement('a') with a plain object whose click is its
    // own mock: use real elements and watch the click of the download anchor instead.
    const realCreateElement = Document.prototype.createElement.bind(document)

    beforeEach(() => {
      clicks = []
      vi.mocked(document.createElement).mockImplementation(((tag: string, opts?: any) =>
        realCreateElement(tag, opts)) as any)
      createUrl.mockReturnValue('blob:fake')
      ;(URL as any).createObjectURL = createUrl
      ;(URL as any).revokeObjectURL = revokeUrl
      vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (this: HTMLAnchorElement) {
        clicks.push({ download: this.download, href: this.href })
      })
    })

    afterEach(() => {
      vi.restoreAllMocks()
    })

    async function openForm(devices = picked) {
      const wrapper = open(devices)
      await byId(wrapper, 'bulk-op-download').trigger('click')
      return wrapper
    }

    const send = async (wrapper: ReturnType<typeof mount>) => {
      await byId(wrapper, 'bulk-download-submit').trigger('submit')
      await flushPromises()
    }

    it('sends exactly the picked ids and no options, whatever the stop box says in another form', async () => {
      vi.mocked(bulkDownload).mockResolvedValue(answerOf([readable]) as any)
      const wrapper = open()
      await byId(wrapper, 'bulk-op-import').trigger('click')
      await byId(wrapper, 'bulk-stop-on-error').setValue(true)
      await wrapper.findAll('button').find((b) => b.text() === 'Back')!.trigger('click')
      await byId(wrapper, 'bulk-op-download').trigger('click')

      await send(wrapper)

      expect(bulkDownload).toHaveBeenCalledTimes(1)
      const request = vi.mocked(bulkDownload).mock.calls[0][0]
      expect(request.device_ids).toEqual([4, 9])
      expect(Object.keys(request)).toEqual(['device_ids'])
    })

    it('saves the one file under the name the server sent, and does not tell the page to refresh', async () => {
      const answer = answerOf([readable])
      vi.mocked(bulkDownload).mockResolvedValue(answer as any)
      const wrapper = await openForm()

      await send(wrapper)

      expect(createUrl).toHaveBeenCalledTimes(1)
      expect(createUrl).toHaveBeenCalledWith(answer.blob)
      expect(clicks).toEqual([{ download: 'shelly-configs-20261120-002000.json', href: 'blob:fake' }])
      expect(revokeUrl).toHaveBeenCalledWith('blob:fake')
      // No device was contacted, so the devices page has nothing to refresh.
      expect(wrapper.emitted('complete')).toBeUndefined()
    })

    it('names the file itself when the server sent no name', async () => {
      vi.mocked(bulkDownload).mockResolvedValue(answerOf([readable], null) as any)
      const wrapper = await openForm()

      await send(wrapper)

      expect(clicks.map((c) => c.download)).toEqual(['shelly-configs.json'])
    })

    it('says which device could not be read and why, and counts the others as saved', async () => {
      vi.mocked(bulkDownload).mockResolvedValue(answerOf([readable, unreadable]) as any)
      const wrapper = await openForm()

      await send(wrapper)

      const saved = byId(wrapper, 'bulk-download-saved')
      expect(saved.text()).toContain('shelly-configs-20261120-002000.json')
      expect(saved.text()).toContain('1 of 2')
      const failed = byId(wrapper, 'bulk-download-failed')
      expect(failed.text()).toContain('Garage')
      expect(failed.text()).toContain('no desired configuration is set for this device')
      expect(failed.text()).not.toContain('Kitchen')
    })

    it('shows no failed list when every device was read', async () => {
      vi.mocked(bulkDownload).mockResolvedValue(answerOf([readable]) as any)
      const wrapper = await openForm()

      await send(wrapper)

      expect(byId(wrapper, 'bulk-download-saved').text()).toContain('1 of 1')
      expect(byId(wrapper, 'bulk-download-failed').exists()).toBe(false)
    })

    it('shows the cap the route named when it refuses the selection, saves no file and stays on the form', async () => {
      vi.mocked(bulkDownload).mockRejectedValue(new Error('bulk-download takes at most 10 devices, got 12'))
      const wrapper = await openForm()

      await send(wrapper)

      expect(byId(wrapper, 'bulk-error').text()).toContain('bulk-download takes at most 10 devices, got 12')
      expect(createUrl).not.toHaveBeenCalled()
      expect(clicks).toEqual([])
      expect(byId(wrapper, 'bulk-download-saved').exists()).toBe(false)
      expect(byId(wrapper, 'bulk-download-submit').exists()).toBe(true)
      expect(wrapper.emitted('complete')).toBeUndefined()
    })

    it('says on the form and in the summary that every secret is masked, and renders no configuration', async () => {
      vi.mocked(bulkDownload).mockResolvedValue(answerOf([readable, unreadable]) as any)
      const wrapper = await openForm()

      const notice = byId(wrapper, 'bulk-download-notice')
      expect(notice.text()).toContain('********')
      expect(notice.text()).toMatch(/masked/i)
      expect(notice.text()).toMatch(/cannot restore/i)

      await send(wrapper)

      expect(byId(wrapper, 'bulk-download-saved').text()).toContain('********')
      expect(byId(wrapper, 'bulk-download-saved').text()).toMatch(/masked/i)
      // The configuration goes to the file and never to the screen.
      expect(wrapper.text()).not.toContain('config-marker-xyz')
      expect(wrapper.text()).not.toContain('mqtt')
    })

    it('saves nothing when the dialog was closed before the file arrived', async () => {
      let answer!: (value: unknown) => void
      vi.mocked(bulkDownload).mockReturnValue(new Promise((resolve) => (answer = resolve)) as any)
      const wrapper = await openForm()
      await byId(wrapper, 'bulk-download-submit').trigger('submit')

      await byId(wrapper, 'bulk-close').trigger('click')
      answer(answerOf([readable]))
      await flushPromises()

      expect(createUrl).not.toHaveBeenCalled()
      expect(clicks).toEqual([])
      expect(byId(wrapper, 'bulk-download-saved').exists()).toBe(false)
    })

    it('sends nothing when the selection empties after the form was opened', async () => {
      const wrapper = await openForm()

      await wrapper.setProps({ selectedDevices: [] })
      await send(wrapper)

      expect(bulkDownload).not.toHaveBeenCalled()
    })

    it('keeps Back off while the request is pending, then opens the card on the form and not on a summary', async () => {
      let answer!: (value: unknown) => void
      vi.mocked(bulkDownload).mockReturnValue(new Promise((resolve) => (answer = resolve)) as any)
      const wrapper = await openForm()
      await byId(wrapper, 'bulk-download-submit').trigger('submit')

      const back = () => wrapper.findAll('button').find((b) => b.text() === 'Back')!
      expect(back().attributes('disabled')).toBeDefined()

      answer(answerOf([readable]))
      await flushPromises()
      // The file was saved once, for the form the operator stayed on.
      expect(clicks).toHaveLength(1)
      await byId(wrapper, 'bulk-download-close').trigger('click')

      // Reopened for the same devices: the card opens on the form, not on the old summary.
      await wrapper.setProps({ isOpen: true })
      await byId(wrapper, 'bulk-op-download').trigger('click')
      expect(byId(wrapper, 'bulk-download-submit').exists()).toBe(true)
      expect(byId(wrapper, 'bulk-download-saved').exists()).toBe(false)
    })
  })

  it('emits close from the header button', async () => {
    const wrapper = open()

    await byId(wrapper, 'bulk-close').trigger('click')

    expect(wrapper.emitted('close')).toHaveLength(1)
  })
})
