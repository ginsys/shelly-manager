import { describe, it, expect, beforeAll, afterAll, afterEach } from 'vitest'
import { mount, RouterLinkStub, type VueWrapper } from '@vue/test-utils'
import { nextTick } from 'vue'
import NavMenu from '../NavMenu.vue'

// The global test setup replaces document.createElement('a') with a mock object and stubs
// document.body.appendChild. A real anchor and a real attached DOM are needed here to test focus.
const stubbedCreateElement = document.createElement
beforeAll(() => {
  document.createElement = Document.prototype.createElement.bind(document) as typeof document.createElement
})
afterAll(() => {
  document.createElement = stubbedCreateElement
})

const sections = [
  {
    title: 'First',
    items: [
      { label: 'One', to: '/one', icon: '1' },
      { label: 'Two', to: '/two' },
    ],
  },
  { items: [{ label: 'Three', to: '/three' }] },
]

let wrapper: VueWrapper | null = null
let host: HTMLElement | null = null

function mountMenu(props: { active?: boolean } = {}) {
  host = document.createElement('div')
  Element.prototype.appendChild.call(document.body, host)
  wrapper = mount(NavMenu, {
    attachTo: host,
    props: { label: 'Area', active: false, sections, ...props },
    global: { stubs: { RouterLink: RouterLinkStub } },
  })
  return wrapper
}

const trigger = () => document.body.querySelector('.dropdown-trigger') as HTMLButtonElement
const items = () => Array.from(document.body.querySelectorAll('.dropdown-item')) as HTMLElement[]
const isOpen = () => document.body.querySelector('.nav-dropdown')!.classList.contains('open')

describe('NavMenu', () => {
  afterEach(() => {
    wrapper?.unmount()
    wrapper = null
    host?.remove()
    host = null
  })

  it('renders its label, every item as a link to its route, and the section titles', () => {
    mountMenu()
    expect(trigger().textContent).toContain('Area')
    expect(wrapper!.findAllComponents(RouterLinkStub).map((l) => l.props('to'))).toEqual([
      '/one',
      '/two',
      '/three',
    ])
    expect(items().map((i) => i.textContent!.replace(/\s+/g, ' ').trim())).toEqual(['1 One', 'Two', 'Three'])
    expect(document.body.querySelector('.dropdown-section-title')!.textContent).toBe('First')
  })

  it('is closed at first and toggles on a click of the trigger', async () => {
    mountMenu()
    expect(isOpen()).toBe(false)
    expect(trigger().getAttribute('aria-expanded')).toBe('false')
    trigger().click()
    await nextTick()
    expect(isOpen()).toBe(true)
    expect(trigger().getAttribute('aria-expanded')).toBe('true')
    trigger().click()
    await nextTick()
    expect(isOpen()).toBe(false)
  })

  it('marks the trigger active from the active prop', () => {
    mountMenu({ active: true })
    expect(trigger().classList.contains('active')).toBe(true)
  })

  it('stays open while focus moves from the trigger to an item inside it', async () => {
    mountMenu()
    trigger().click()
    await nextTick()
    trigger().focus()
    items()[0].focus()
    trigger().dispatchEvent(new FocusEvent('focusout', { bubbles: true, relatedTarget: items()[0] }))
    await nextTick()
    expect(isOpen()).toBe(true)
  })

  it('closes when focus leaves the menu for something outside it', async () => {
    mountMenu()
    trigger().click()
    await nextTick()
    const outside = document.createElement('button')
    Element.prototype.appendChild.call(document.body, outside)
    items()[2].dispatchEvent(new FocusEvent('focusout', { bubbles: true, relatedTarget: outside }))
    await nextTick()
    outside.remove()
    expect(isOpen()).toBe(false)
  })

  it('stays open when a pointer press inside the panel blurs the trigger with no related target', async () => {
    mountMenu()
    trigger().click()
    await nextTick()
    const panel = document.body.querySelector('.dropdown-menu') as HTMLElement
    panel.dispatchEvent(new Event('pointerdown', { bubbles: true }))
    trigger().dispatchEvent(new FocusEvent('focusout', { bubbles: true, relatedTarget: null }))
    await nextTick()
    expect(isOpen()).toBe(true)
    // once the press ends, a blur with no related target does close it
    panel.dispatchEvent(new Event('pointerup', { bubbles: true }))
    trigger().dispatchEvent(new FocusEvent('focusout', { bubbles: true, relatedTarget: null }))
    await nextTick()
    expect(isOpen()).toBe(false)
  })

  it('closes on Escape and returns focus to the trigger', async () => {
    mountMenu()
    trigger().click()
    await nextTick()
    // the stubbed links have no href, so give one a tabindex to hold the focus
    items()[1].setAttribute('tabindex', '0')
    items()[1].focus()
    expect(document.activeElement).toBe(items()[1])
    items()[1].dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await nextTick()
    expect(isOpen()).toBe(false)
    expect(document.activeElement).toBe(trigger())
  })

  it('closes on a pointer press outside the menu', async () => {
    mountMenu()
    trigger().click()
    await nextTick()
    document.body.dispatchEvent(new Event('pointerdown', { bubbles: true }))
    await nextTick()
    expect(isOpen()).toBe(false)
  })

  it('closes on Escape pressed with the focus outside the menu, and leaves the focus alone', async () => {
    mountMenu()
    trigger().click()
    await nextTick()
    trigger().blur()
    expect(document.activeElement).not.toBe(trigger())
    document.body.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await nextTick()
    expect(isOpen()).toBe(false)
    expect(trigger().getAttribute('aria-expanded')).toBe('false')
    expect(document.activeElement).not.toBe(trigger())
  })

  it('closes on a Tab out after a press that began inside the menu and ended outside it', async () => {
    mountMenu()
    trigger().click()
    await nextTick()
    const panel = document.body.querySelector('.dropdown-menu') as HTMLElement
    panel.dispatchEvent(new Event('pointerdown', { bubbles: true }))
    // a drag out of the panel: the release happens outside it
    document.body.dispatchEvent(new Event('pointerup', { bubbles: true }))
    const outside = document.createElement('button')
    Element.prototype.appendChild.call(document.body, outside)
    items()[0].dispatchEvent(new FocusEvent('focusout', { bubbles: true, relatedTarget: outside }))
    await nextTick()
    outside.remove()
    expect(isOpen()).toBe(false)
  })

  it('also ends a press on pointercancel', async () => {
    mountMenu()
    trigger().click()
    await nextTick()
    const panel = document.body.querySelector('.dropdown-menu') as HTMLElement
    panel.dispatchEvent(new Event('pointerdown', { bubbles: true }))
    document.body.dispatchEvent(new Event('pointercancel', { bubbles: true }))
    trigger().dispatchEvent(new FocusEvent('focusout', { bubbles: true, relatedTarget: null }))
    await nextTick()
    expect(isOpen()).toBe(false)
  })

  it('closes when an item is chosen', async () => {
    mountMenu()
    trigger().click()
    await nextTick()
    items()[0].click()
    await nextTick()
    expect(isOpen()).toBe(false)
  })
})
