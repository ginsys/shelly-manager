import { describe, it, expect, beforeAll, afterAll, afterEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import MainLayout from '../MainLayout.vue'

// The global test setup replaces document.createElement('a') with a mock object, so router links
// would render nothing real. Restore the real one for this file.
const stubbedCreateElement = document.createElement
beforeAll(() => {
  document.createElement = Document.prototype.createElement.bind(document) as typeof document.createElement
})
afterAll(() => {
  document.createElement = stubbedCreateElement
})

const Page = { template: '<div />' }

async function mountAt(path: string, category?: string) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/:rest(.*)*', component: Page, meta: category ? { category } : {} }],
  })
  await router.push(path)
  await router.isReady()
  const wrapper = mount(MainLayout, { global: { plugins: [router] } })
  await flushPromises()
  return wrapper
}

let wrapper: VueWrapper | null = null

// Every top-bar entry: the link or menu item, with the route it must lead to.
const topLevelLinks = ['/', '/templates', '/plugins', '/dashboard', '/admin']
const menus: Record<string, string[]> = {
  'Export & Import': ['/export/backup', '/export/gitops', '/export/history', '/import/history'],
  Drift: ['/drift/schedules', '/drift/reports', '/drift/trends'],
  Notifications: ['/notifications/channels', '/notifications/rules', '/notifications/history'],
  Provisioning: ['/provisioning', '/provisioning/tasks', '/provisioning/agents'],
}

describe('MainLayout navigation', () => {
  afterEach(() => {
    wrapper?.unmount()
    wrapper = null
  })

  it('links every top-level page directly from the top bar', async () => {
    wrapper = await mountAt('/')
    const hrefs = wrapper.findAll('nav[data-testid="navigation"] > a.nav-link').map((a) => a.attributes('href'))
    expect(hrefs).toEqual(topLevelLinks)
  })

  it('has one menu per area, each leading to every page of that area', async () => {
    wrapper = await mountAt('/')
    const found: Record<string, string[]> = {}
    for (const menu of wrapper.findAll('.nav-dropdown')) {
      const label = menu.find('.dropdown-trigger').text()
      found[label] = menu.findAll('a.dropdown-item').map((a) => a.attributes('href')!)
    }
    expect(found).toEqual(menus)
  })

  it.each([
    ['Drift', '/drift/reports', 'drift'],
    ['Notifications', '/notifications/channels/42', 'notifications'],
    ['Provisioning', '/provisioning/tasks/7', 'provisioning'],
    ['Export & Import', '/export/9', 'export'],
    ['Export & Import', '/import/9', 'import'],
  ])('highlights the %s menu on %s', async (label, path, category) => {
    wrapper = await mountAt(path, category)
    const active = wrapper.findAll('.dropdown-trigger.active').map((t) => t.text())
    expect(active).toEqual([label])
  })

  it('highlights Templates on a template detail page and no menu', async () => {
    wrapper = await mountAt('/templates/3', 'templates')
    const links = wrapper.findAll('nav[data-testid="navigation"] > a.nav-link')
    expect(links.filter((a) => a.classes('active')).map((a) => a.attributes('href'))).toEqual(['/templates'])
    expect(wrapper.findAll('.dropdown-trigger.active')).toHaveLength(0)
  })
})
