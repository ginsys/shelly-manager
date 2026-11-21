import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent, h } from 'vue'

vi.mock('@/api/devices', () => ({ listDevices: vi.fn() }))
vi.mock('@/api/commonSettings', () => ({ getCommonSettings: vi.fn() }))
vi.mock('@/api/deviceConfig', () => ({ getTypedDeviceConfig: vi.fn() }))
vi.mock('@/api/typedConfig', () => ({ getConfigSchema: vi.fn() }))
vi.mock('@/api/templates', async (importActual) => {
  const actual = await importActual<typeof import('@/api/templates')>()
  return { ...actual, createTemplate: vi.fn(), listDeviceTypes: vi.fn() }
})

import { listDevices } from '@/api/devices'
import { getCommonSettings } from '@/api/commonSettings'
import { getConfigSchema } from '@/api/typedConfig'
import { createTemplate, listDeviceTypes } from '@/api/templates'
import CommonSettingsPage from '../CommonSettingsPage.vue'

const device = (id: number, name: string, model: string) => ({
  id,
  name,
  ip: `10.0.0.${id}`,
  mac: `AA:BB:CC:00:00:0${id}`,
  type: 'Smart Plug',
  firmware: '1',
  status: 'online',
  last_seen: '',
  settings: JSON.stringify({ model, gen: 1 }),
})

const devices = [device(1, 'Hall', 'SHPLG-S'), device(2, 'Kitchen', 'SHPLG-S'), device(3, 'Garden', 'SHSW-25')]

const schema = {
  type: 'object',
  'x-device-type': 'SHPLG-S',
  properties: {
    mqtt: { type: 'object', properties: { enable: { type: 'boolean' }, server: { type: 'string' } } },
    cloud: { type: 'object', properties: { enable: { type: 'boolean' } } },
  },
}

const answer = {
  compared: [1, 2],
  skipped: [],
  deviceType: 'SHPLG-S',
  common: { mqtt: { enable: true, server: 'broker:1883' }, cloud: { enable: false } },
  differing: { mqtt: 1 },
}

// The global test setup replaces every <a> with a plain object, so the stub for the router link renders a span.
const LinkStub = defineComponent({
  props: ['to'],
  setup: (p, { slots }) => () => h('span', { 'data-to': JSON.stringify(p.to) }, slots.default?.()),
})

async function open() {
  const wrapper = mount(CommonSettingsPage, {
    global: { stubs: { RouterLink: LinkStub, ConfigView: true } },
  })
  await flushPromises()
  return wrapper
}

describe('CommonSettingsPage', () => {
  beforeEach(() => {
    ;(listDevices as any).mockReset().mockResolvedValue({ items: devices })
    ;(getCommonSettings as any).mockReset().mockResolvedValue(answer)
    ;(getConfigSchema as any).mockReset().mockResolvedValue(schema)
    ;(listDeviceTypes as any)
      .mockReset()
      .mockResolvedValue([{ device_type: 'SHPLG-S', name: 'Shelly Plug S', generation: 1 }])
    ;(createTemplate as any).mockReset().mockResolvedValue({ id: 42, name: 'Plugs' })
  })

  it('reads every device and offers each model once, with its device count', async () => {
    const wrapper = await open()

    expect(listDevices).toHaveBeenCalledWith({ all: true })
    const options = wrapper.findAll('[data-testid="model-select"] option').map((o) => o.text())
    expect(options).toContain('SHPLG-S (2 devices)')
    expect(options).toContain('SHSW-25 (1 device)')
  })

  it('compares every device of the chosen model', async () => {
    const wrapper = await open()
    await wrapper.get('[data-testid="model-select"]').setValue('SHPLG-S')

    await wrapper.get('[data-testid="compare"]').trigger('click')
    await flushPromises()

    expect(getCommonSettings).toHaveBeenCalledWith([1, 2])
  })

  it('compares a hand pick of devices', async () => {
    const wrapper = await open()
    await wrapper.get('[data-testid="mode-pick"]').setValue(true)
    await wrapper.get('[data-testid="device-1"]').setValue(true)
    await wrapper.get('[data-testid="device-3"]').setValue(true)

    await wrapper.get('[data-testid="compare"]').trigger('click')
    await flushPromises()

    expect(getCommonSettings).toHaveBeenCalledWith([1, 3])
  })

  it('keeps Compare off until a device is chosen', async () => {
    const wrapper = await open()
    const compare = () => (wrapper.get('[data-testid="compare"]').element as HTMLButtonElement).disabled

    expect(compare()).toBe(true)
    await wrapper.get('[data-testid="model-select"]').setValue('SHPLG-S')
    expect(compare()).toBe(false)
  })

  it('says how many devices were compared and names each one it skipped, with the reason', async () => {
    ;(getCommonSettings as any).mockResolvedValue({
      ...answer,
      skipped: [{ deviceId: 3, deviceName: 'Garden', reason: 'no stored import' }],
    })
    const wrapper = await open()
    await wrapper.get('[data-testid="model-select"]').setValue('SHPLG-S')
    await wrapper.get('[data-testid="compare"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="common-compared"]').text()).toContain('2 devices compared')
    expect(wrapper.get('[data-testid="common-skipped"]').text()).toContain('Garden')
    expect(wrapper.get('[data-testid="common-skipped"]').text()).toContain('no stored import')
  })

  it('does not call one device a comparison', async () => {
    ;(getCommonSettings as any).mockResolvedValue({ ...answer, compared: [1] })
    const wrapper = await open()
    await wrapper.get('[data-testid="model-select"]').setValue('SHPLG-S')
    await wrapper.get('[data-testid="compare"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="common-compared"]').text()).toContain('Only one device was compared')
  })

  it('offers no template when the devices share nothing', async () => {
    ;(getCommonSettings as any).mockResolvedValue({ ...answer, common: {}, differing: {} })
    const wrapper = await open()
    await wrapper.get('[data-testid="model-select"]').setValue('SHPLG-S')
    await wrapper.get('[data-testid="compare"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="common-none"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="common-create"]').exists()).toBe(false)
  })

  it('shows the reason a comparison failed', async () => {
    ;(getCommonSettings as any).mockRejectedValue(new Error('common-settings takes at most 200 devices'))
    const wrapper = await open()
    await wrapper.get('[data-testid="model-select"]').setValue('SHPLG-S')
    await wrapper.get('[data-testid="compare"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="common-error"]').text()).toContain('at most 200 devices')
  })

  it('shows the reason the device list could not be read', async () => {
    ;(listDevices as any).mockRejectedValue(new Error('Failed to load devices'))
    const wrapper = await open()

    expect(wrapper.get('[data-testid="devices-error"]').text()).toContain('Failed to load devices')
  })

  it('shows no answer for devices that are no longer chosen when the selection changes during a compare', async () => {
    let finish: (v: typeof answer) => void = () => {}
    ;(getCommonSettings as any).mockReset().mockReturnValue(new Promise((resolve) => (finish = resolve)))
    const wrapper = await open()
    await wrapper.get('[data-testid="model-select"]').setValue('SHPLG-S')
    await wrapper.get('[data-testid="compare"]').trigger('click')

    await wrapper.get('[data-testid="model-select"]').setValue('SHSW-25')
    finish(answer)
    await flushPromises()

    expect(wrapper.find('[data-testid="common-result"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="common-create"]').exists()).toBe(false)
    expect((wrapper.get('[data-testid="compare"]').element as HTMLButtonElement).disabled).toBe(false)
  })

  it('saves the ticked sections of the common settings as a template', async () => {
    const wrapper = await open()
    await wrapper.get('[data-testid="model-select"]').setValue('SHPLG-S')
    await wrapper.get('[data-testid="compare"]').trigger('click')
    await flushPromises()

    await wrapper.get('[data-testid="common-create"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="section-differing-mqtt"]').text()).toContain('1 differ')
    await wrapper.get('[data-testid="section-cloud"]').setValue(false)
    await wrapper.get('[data-testid="from-import-name"]').setValue('Plug defaults')
    await wrapper.get('[data-testid="from-import-save"]').trigger('click')
    await flushPromises()

    expect(createTemplate).toHaveBeenCalledTimes(1)
    const sent = (createTemplate as any).mock.calls[0][0]
    expect(sent).toMatchObject({ name: 'Plug defaults', scope: 'device_type', device_type: 'SHPLG-S' })
    expect(sent.config).toEqual({ mqtt: { enable: true, server: 'broker:1883' } })
    expect(wrapper.get('[data-testid="from-import-saved"]').text()).toContain('Template saved')
  })
})
