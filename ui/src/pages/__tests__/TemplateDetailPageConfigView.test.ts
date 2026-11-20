import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: '1' } }),
  useRouter: () => ({ push: vi.fn() }),
}))

vi.mock('@/api/typedConfig', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/typedConfig')>()
  return { ...actual, getConfigSchema: vi.fn() }
})

vi.mock('@/api/templates', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/templates')>()
  return { ...actual, getTemplate: vi.fn() }
})

import { getConfigSchema } from '@/api/typedConfig'
import { getTemplate } from '@/api/templates'
import TemplateDetailPage from '../TemplateDetailPage.vue'

// ginsys/shelly-manager#747 slice 1: the read view of a template shows its settings as fields, and
// keeps the JSON behind a toggle, as the device configuration page does.
const stubs = {
  'router-link': true,
  ConfigEditor: true,
  ConfigView: {
    props: ['config'],
    template: '<div class="stub-view" data-testid="template-config-view">{{ Object.keys(config || {}).join(",") }}</div>',
  },
}

const template = {
  id: 1,
  name: 'plugs',
  description: '',
  scope: 'global',
  created_at: '',
  updated_at: '',
  config: { mqtt: { user: 'iot' }, relay: { auto_off: 30 } },
}

async function mountPage() {
  const wrapper = mount(TemplateDetailPage, { global: { stubs } })
  await flushPromises()
  return wrapper
}

describe('TemplateDetailPage read view of the config', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(getConfigSchema).mockReset()
    vi.mocked(getConfigSchema).mockResolvedValue({ type: 'object', properties: {} } as any)
    vi.mocked(getTemplate).mockReset()
    vi.mocked(getTemplate).mockResolvedValue(template as any)
  })

  it('shows the config as fields, not as a JSON string', async () => {
    const wrapper = await mountPage()

    expect(wrapper.get('[data-testid="template-config-view"]').text()).toBe('mqtt,relay')
    expect(wrapper.find('textarea').exists()).toBe(false)
  })

  it('no longer calls the heading JSON', async () => {
    const wrapper = await mountPage()

    expect(wrapper.text()).not.toContain('Config (JSON)')
    expect(wrapper.findAll('h2').map((h) => h.text())).toContain('Config')
  })

  it('keeps the JSON behind a Raw JSON toggle, and the toggle goes back', async () => {
    const wrapper = await mountPage()
    const toggle = () => wrapper.findAll('button').find((b) => b.text() === 'Raw JSON' || b.text() === 'Structured View')!

    await toggle().trigger('click')

    const text = wrapper.get('textarea').element as HTMLTextAreaElement
    expect(JSON.parse(text.value)).toEqual(template.config)
    expect(wrapper.find('[data-testid="template-config-view"]').exists()).toBe(false)
    expect(toggle().text()).toBe('Structured View')

    await toggle().trigger('click')
    expect(wrapper.find('textarea').exists()).toBe(false)
    expect(wrapper.find('[data-testid="template-config-view"]').exists()).toBe(true)
  })

  it('still copies the config as JSON', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
    const wrapper = await mountPage()

    await wrapper.findAll('button').find((b) => b.text() === 'Copy')!.trigger('click')
    await flushPromises()

    expect(JSON.parse(writeText.mock.calls[0][0])).toEqual(template.config)
  })

  it('shows the form, and no Raw JSON toggle, in edit mode', async () => {
    const wrapper = await mountPage()

    await wrapper.findAll('.toolbar .btn').find((b) => b.text() === 'Edit')!.trigger('click')

    expect(wrapper.find('[data-testid="template-config-view"]').exists()).toBe(false)
    expect(wrapper.findAll('button').some((b) => b.text() === 'Raw JSON')).toBe(false)
  })
})
