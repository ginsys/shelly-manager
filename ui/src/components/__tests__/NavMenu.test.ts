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

  // jsdom has no PointerEvent, so a plain Event carries the pointerType the handlers read.
  const pointer = (type: string, pointerType: string) =>
    Object.assign(new Event(type), { pointerType })
  const root = () => document.body.querySelector('.nav-dropdown') as HTMLElement

  it('opens on mouse hover through the same state as a click, so aria-expanded agrees', async () => {
    mountMenu()
    root().dispatchEvent(pointer('pointerenter', 'mouse'))
    await nextTick()
    expect(isOpen()).toBe(true)
    expect(trigger().getAttribute('aria-expanded')).toBe('true')
    expect(document.body.querySelector('.dropdown-menu')!.getAttribute('aria-hidden')).toBe('false')
  })

  it('keeps a menu open when the click follows the hover that opened it, and the next click closes it', async () => {
    mountMenu()
    root().dispatchEvent(pointer('pointerenter', 'mouse'))
    await nextTick()
    trigger().click()
    await nextTick()
    expect(isOpen()).toBe(true)
    trigger().click()
    await nextTick()
    expect(isOpen()).toBe(false)
  })

  it('closes on Escape pressed with the focus outside the menu, and leaves the focus alone', async () => {
    mountMenu()
    root().dispatchEvent(pointer('pointerenter', 'mouse'))
    await nextTick()
    expect(document.activeElement).not.toBe(trigger())
    document.body.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await nextTick()
    expect(isOpen()).toBe(false)
    expect(document.activeElement).not.toBe(trigger())
  })

  it('does not open on a touch pointer, whose tap then toggles it with the click', async () => {
    mountMenu()
    root().dispatchEvent(pointer('pointerenter', 'touch'))
    trigger().click()
    await nextTick()
    expect(isOpen()).toBe(true)
  })

  it('stays closed after Escape even while the pointer is over it', async () => {
    mountMenu()
    root().dispatchEvent(pointer('pointerenter', 'mouse'))
    await nextTick()
    items()[0].dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await nextTick()
    expect(isOpen()).toBe(false)
    expect(trigger().getAttribute('aria-expanded')).toBe('false')
  })

  it('closes when the mouse leaves, unless keyboard focus is still inside', async () => {
    mountMenu()
    root().dispatchEvent(pointer('pointerenter', 'mouse'))
    await nextTick()
    root().dispatchEvent(pointer('pointerleave', 'mouse'))
    await nextTick()
    expect(isOpen()).toBe(false)

    trigger().click()
    await nextTick()
    // the stubbed links have no href, so they cannot take focus; the button stands in for an item
    trigger().focus()
    expect(document.activeElement).toBe(trigger())
    root().dispatchEvent(pointer('pointerleave', 'mouse'))
    await nextTick()
    expect(isOpen()).toBe(true)
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
