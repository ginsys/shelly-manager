import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import BulkApplyForm from '../BulkApplyForm.vue'

const picked = [
  { id: 4, name: 'Kitchen', ip: '10.0.0.4' },
  { id: 9, name: 'Garage', ip: '10.0.0.9' }
]

function form(devices = picked, modelValue = false) {
  return mount(BulkApplyForm, { props: { devices, modelValue } })
}

const by = (w: ReturnType<typeof mount>, id: string) => w.find(`[data-testid="${id}"]`)

async function type(w: ReturnType<typeof mount>, text: string) {
  await by(w, 'bulk-apply-json').setValue(text)
}

async function review(w: ReturnType<typeof mount>, text: string) {
  await type(w, text)
  await w.find('form').trigger('submit')
}

describe('BulkApplyForm', () => {
  it('explains what is written, that null removes, that the mask keeps a secret and the limits', () => {
    const w = form()
    const text = w.text()

    expect(text).toContain('overrides, never as a template')
    expect(text).toContain('PATCH /devices/{id}/overrides')
    expect(by(w, 'bulk-apply-notes').text()).toContain('null removes that override')
    expect(by(w, 'bulk-apply-notes').text()).toContain('A secret sent as ******** keeps the stored secret')
    expect(by(w, 'bulk-apply-notes').text()).toContain('At most 10 devices per call')
    expect(by(w, 'bulk-apply-notes').text()).toContain('up to 5 minutes')
    expect(by(w, 'bulk-apply-notes').text()).toContain('check each device before sending again')
  })

  // Nothing reaches the confirmation, and nothing is emitted, from input that is not an object to write.
  it.each([
    ['nothing', '', 'Type the configuration'],
    ['only spaces', '   \n ', 'Type the configuration'],
    ['text that is not JSON', '{ name: Kitchen', 'not valid JSON'],
    ['an array', '[1, 2]', 'must be a JSON object'],
    ['null', 'null', 'must be a JSON object'],
    ['a string', '"name"', 'must be a JSON object'],
    ['a number', '7', 'must be a JSON object'],
    ['an empty object', '{}', 'is empty']
  ])('%s shows an error and goes no further', async (_name, input, message) => {
    const w = form()

    await review(w, input)

    expect(by(w, 'bulk-apply-problem').text()).toContain(message)
    expect(by(w, 'bulk-apply-confirm').exists()).toBe(false)
    expect(w.emitted('submit')).toBeUndefined()
  })

  it('does not echo what was typed when it is not valid JSON', async () => {
    const w = form()

    await review(w, '{ "wifi": { "pass": "hunter2" ')

    expect(by(w, 'bulk-apply-problem').text()).not.toContain('hunter2')
  })

  it('shows the exact devices and the exact JSON before anything is sent', async () => {
    const w = form()

    await review(w, '{"name":"New","wifi":{"pass":"s3cret"},"x":null}')

    const confirm = by(w, 'bulk-apply-confirm')
    expect(confirm.exists()).toBe(true)
    expect(confirm.text()).toContain('This writes to 2 devices. Nothing is undone if a later device fails.')
    const rows = w.findAll('[data-testid="bulk-apply-confirm-device"]').map((r) => r.text())
    expect(rows).toEqual([expect.stringContaining('Kitchen'), expect.stringContaining('Garage')])
    expect(rows[0]).toContain('10.0.0.4')
    expect(rows[1]).toContain('10.0.0.9')
    // What the request carries: the parsed value, written out. The typed secret is shown as typed.
    expect(by(w, 'bulk-apply-confirm-json').text()).toBe(
      JSON.stringify({ name: 'New', wifi: { pass: 's3cret' }, x: null }, null, 2)
    )
    expect(w.emitted('submit')).toBeUndefined()
  })

  it('shows, of a key typed twice, the one that wins', async () => {
    const w = form()

    await review(w, '{"name":"first","name":"second"}')

    expect(by(w, 'bulk-apply-confirm-json').text()).toBe(JSON.stringify({ name: 'second' }, null, 2))
  })

  it('names the stop-on-error consequence in the confirmation', async () => {
    const off = form(picked, false)
    await review(off, '{"a":1}')
    expect(by(off, 'bulk-apply-stop-note').text()).toContain('every device is tried')

    const on = form(picked, true)
    await review(on, '{"a":1}')
    expect(by(on, 'bulk-apply-stop-note').text()).toContain('the rest are not contacted')
  })

  it('emits the ids and the configuration when Apply is pressed', async () => {
    const w = form()
    await review(w, '{"name":"New"}')

    await by(w, 'bulk-apply-submit').trigger('click')

    expect(w.emitted('submit')).toEqual([[{ deviceIds: [4, 9], config: { name: 'New' } }]])
  })

  // The operator confirmed a list: a selection that moves afterwards changes nothing that was written.
  it('sends the devices that were reviewed, even after the selection changed', async () => {
    const w = form()
    await review(w, '{"name":"New"}')

    await w.setProps({ devices: [{ id: 1, name: 'Other', ip: '10.0.0.1' }] })
    await by(w, 'bulk-apply-submit').trigger('click')

    expect(w.emitted('submit')).toEqual([[{ deviceIds: [4, 9], config: { name: 'New' } }]])
    expect(by(w, 'bulk-apply-confirm-devices').text()).toContain('Kitchen')
  })

  it('returns to the typed text with Edit and reviews again from there', async () => {
    const w = form()
    await review(w, '{"name":"New"}')

    await by(w, 'bulk-apply-edit').trigger('click')

    expect(by(w, 'bulk-apply-confirm').exists()).toBe(false)
    expect((by(w, 'bulk-apply-json').element as HTMLTextAreaElement).value).toBe('{"name":"New"}')
  })

  it('takes at most 10 devices: over it Review is disabled and the cap is named', async () => {
    const many = Array.from({ length: 11 }, (_, i) => ({ id: i + 1, name: `D${i + 1}`, ip: `10.0.0.${i + 1}` }))
    const w = form(many)
    await type(w, '{"a":1}')

    expect(by(w, 'bulk-apply-review').attributes('disabled')).toBeDefined()
    expect(by(w, 'bulk-apply-cap').text()).toContain('11 devices are selected')
    expect(by(w, 'bulk-apply-cap').text()).toContain('at most 10')
    await w.find('form').trigger('submit')
    expect(by(w, 'bulk-apply-confirm').exists()).toBe(false)

    const ten = form(many.slice(0, 10))
    expect(by(ten, 'bulk-apply-review').attributes('disabled')).toBeUndefined()
    expect(by(ten, 'bulk-apply-cap').exists()).toBe(false)
  })

  it('cannot be reviewed with no device selected', async () => {
    const w = form([])
    await type(w, '{"a":1}')

    expect(by(w, 'bulk-apply-review').attributes('disabled')).toBeDefined()
    await w.find('form').trigger('submit')
    expect(by(w, 'bulk-apply-confirm').exists()).toBe(false)
  })

  it('passes the stop-on-error choice up and shows the shared value', async () => {
    const w = form(picked, true)
    const box = by(w, 'bulk-apply-stop-on-error')

    expect((box.element as HTMLInputElement).checked).toBe(true)
    await box.setValue(false)

    expect(w.emitted('update:modelValue')).toEqual([[false]])
  })

  it('says Back to the dialog', async () => {
    const w = form()
    await by(w, 'bulk-apply-back').trigger('click')
    expect(w.emitted('back')).toHaveLength(1)
  })

  // A typed secret must not stay in component state after an answer arrived or the dialog closed.
  it('forgets the text, the stage and the snapshot on reset()', async () => {
    const w = form()
    await review(w, '{"wifi":{"pass":"s3cret"}}')
    expect(by(w, 'bulk-apply-confirm').exists()).toBe(true)

    ;(w.vm as unknown as { reset: () => void }).reset()
    await w.vm.$nextTick()

    expect(by(w, 'bulk-apply-confirm').exists()).toBe(false)
    expect((by(w, 'bulk-apply-json').element as HTMLTextAreaElement).value).toBe('')
    expect(w.html()).not.toContain('s3cret')
    await by(w, 'bulk-apply-review').trigger('submit')
    expect(w.emitted('submit')).toBeUndefined()
  })

  it('forgets an error shown before the reset', async () => {
    const w = form()
    await review(w, '[]')
    expect(by(w, 'bulk-apply-problem').exists()).toBe(true)

    ;(w.vm as unknown as { reset: () => void }).reset()
    await w.vm.$nextTick()

    expect(by(w, 'bulk-apply-problem').exists()).toBe(false)
  })
})
