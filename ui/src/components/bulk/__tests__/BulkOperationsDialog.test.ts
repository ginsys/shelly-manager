import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import BulkOperationsDialog from '../BulkOperationsDialog.vue'
import { bulkImport, bulkDriftDetect } from '@/api/bulk'

vi.mock('@/api/bulk', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/bulk')>()
  return {
    ...actual,
    bulkImport: vi.fn(),
    bulkDriftDetect: vi.fn(),
    bulkDriftDetectEnhanced: vi.fn()
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

    await byId(wrapper, 'bulk-op-import').trigger('click')
    expect(bulkImport).not.toHaveBeenCalled()
  })

  it('emits close from the header button', async () => {
    const wrapper = open()

    await byId(wrapper, 'bulk-close').trigger('click')

    expect(wrapper.emitted('close')).toHaveLength(1)
  })
})
