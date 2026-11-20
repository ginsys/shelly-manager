import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import ConfigView from '../ConfigView.vue'

const twoRelays = {
  relay: {
    default_state: 'off',
    relays: [
      { id: 0, name: 'Lamp', auto_on: 30, schedule_rules: ['0800-0123456-on', '2200-0123456-off'] },
      { id: 1, schedule: false },
    ],
  },
}

function mountView(props: Record<string, unknown> = {}) {
  return mount(ConfigView, { props: { config: twoRelays, expandAll: true, ...props } })
}

describe('ConfigView nested values', () => {
  it('shows a list of relays as fields, never as a JSON string', () => {
    const text = mountView().text()
    // The control: the section and its top-level field still render.
    expect(text).toContain('Relay')
    expect(text).toContain('Default state:')
    // The defect: one string of JSON for the whole list.
    expect(text).not.toContain('[{')
    expect(text).not.toContain('"id"')
    expect(text).not.toContain('"name"')
    expect(text).toContain('Name:')
    expect(text).toContain('Lamp')
    expect(text).toContain('Auto on:')
  })

  it('makes one group per item, named by its name, else by its id', () => {
    const titles = mountView().findAll('[data-nested="item"] .nested-title').map(t => t.text())
    expect(titles).toEqual(['Lamp', 'Item 1'])
  })

  it('shows a list of plain values as a comma list', () => {
    expect(mountView().text()).toContain('0800-0123456-on, 2200-0123456-off')
  })

  // A list that mixes objects and other values cannot be one group per item. Its objects must not
  // read as "[object Object]": a stored configuration is shown as it is, whatever its shape.
  it('keeps the fields of an object inside a list of mixed values readable', () => {
    const text = mountView({ config: { custom: [{ name: 'visible-name', timeout: 30 }, null, 'plain'] } }).text()
    expect(text).not.toContain('[object Object]')
    expect(text).toContain('visible-name')
    expect(text).toContain('30')
    expect(text).toContain('plain')
  })

  it('shows an empty list as (empty) and a boolean as Yes or No', () => {
    const wrapper = mountView({ config: { relay: { relays: [], has_timer: true } } })
    expect(wrapper.text()).toContain('(empty)')
    expect(wrapper.text()).toContain('Yes')
  })

  it('shows a nested object as indented fields', () => {
    const wrapper = mountView({ config: { wifi: { ap: { ssid: 'office', enable: false } } } })
    expect(wrapper.text()).not.toContain('{"')
    expect(wrapper.text()).toContain('Ssid:')
    expect(wrapper.text()).toContain('office')
    expect(wrapper.text()).toContain('No')
  })

  it('puts the source badge at the nested path, an item by its position', () => {
    const wrapper = mountView({
      showSourceBadges: true,
      sources: { 'relay.default_state': 'global', 'relay.relays.0.name': 'device overrides', 'relay.relays.1.schedule': 'group' },
    })
    const badges = wrapper.findAll('.source-badge').map(b => b.text())
    expect(badges).toEqual(expect.arrayContaining(['global', 'device overrides', 'group']))
  })

  it('shows no badge for a nested field the server gave no source', () => {
    const wrapper = mountView({ showSourceBadges: true, sources: { 'relay.relays.0.name': 'device overrides' } })
    expect(wrapper.findAll('.source-badge').map(b => b.text())).toEqual(['device overrides'])
  })

  it('still says so for an empty configuration', () => {
    expect(mountView({ config: {} }).text()).toContain('No configuration data')
  })
})
