import { describe, it, expect } from 'vitest'
import { diffDesired, parseVersion } from '../desiredDiff'

const MASK = '********'

// ginsys/shelly-manager#775 slice 3. A stored version is a JSON text with every secret masked; the
// current desired configuration has every secret removed. A secret is therefore never a difference.
describe('diffDesired', () => {
  it('shows an added, a removed and a changed value, each with its path', () => {
    const lines = diffDesired(
      { mqtt: { server: 'b', user: 'u' }, sleep: { enable: true } },
      { mqtt: { server: 'a' }, wifi: { ssid: 'x' } },
    )
    expect(lines).toEqual([
      { path: 'mqtt.server', kind: 'changed', from: 'a', to: 'b' },
      { path: 'mqtt.user', kind: 'added', to: 'u' },
      { path: 'sleep.enable', kind: 'added', to: true },
      { path: 'wifi.ssid', kind: 'removed', from: 'x' },
    ])
  })

  it('shows a secret that is the mask in the version and absent from the current as no difference', () => {
    expect(diffDesired({ mqtt: { pass: MASK, user: 'u' } }, { mqtt: { user: 'u' } })).toEqual([])
  })

  it('still shows a value that is not the mask where the current has nothing', () => {
    expect(diffDesired({ mqtt: { user: 'u', pass: 'plain' } }, { mqtt: { user: 'u' } })).toEqual([
      { path: 'mqtt.pass', kind: 'added', to: 'plain' },
    ])
  })

  // The history masks the whole secret_urls list of an action (or each URL in it), and the desired read
  // removes it: lists are walked, so one rule covers both forms and the rest of the action still compares.
  it.each([
    ['the whole list masked', { actions: [{ name: 'out_on_url', index: 0, enabled: true, secret_urls: MASK }] }],
    ['each URL masked', { actions: [{ name: 'out_on_url', index: 0, enabled: true, secret_urls: [MASK] }] }],
  ])('treats a masked action URL list as no difference (%s)', (_label, target) => {
    const current = { actions: [{ name: 'out_on_url', index: 0, enabled: true }] }
    expect(diffDesired(target, current)).toEqual([])
    const changed = { actions: [{ ...target.actions[0], enabled: false }] }
    expect(diffDesired(changed, current)).toEqual([
      { path: 'actions.0.enabled', kind: 'changed', from: true, to: false },
    ])
  })

  it('does not read two objects that differ only in key order as a difference', () => {
    const target = { actions: [{ name: 'a', index: 0, enabled: true }] }
    const current = { actions: [{ enabled: true, index: 0, name: 'a' }] }
    expect(diffDesired(target, current)).toEqual([])
  })

  it('compares a list by position and an empty list as a value', () => {
    expect(diffDesired({ l: [1, 2], e: [] }, { l: [1, 3] })).toEqual([
      { path: 'l.1', kind: 'changed', from: 3, to: 2 },
      { path: 'e', kind: 'added', to: [] },
    ])
  })
})

describe('parseVersion', () => {
  it('reads settings, and nothing else', () => {
    expect(parseVersion('{"a":1}')).toEqual({ a: 1 })
    expect(parseVersion(null)).toBeNull()
    expect(parseVersion('********')).toBeNull()
    expect(parseVersion('[1]')).toBeNull()
    expect(parseVersion('not json')).toBeNull()
  })
})
