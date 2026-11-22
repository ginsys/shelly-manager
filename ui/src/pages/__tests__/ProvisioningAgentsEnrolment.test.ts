import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'

import ProvisioningAgentsPage from '../ProvisioningAgentsPage.vue'
import { useProvisioningStore } from '@/stores/provisioning'

// The agents page shows how each agent enrolled, lists the agents that wait for approval, and lets the
// operator approve, reject and pre-approve (ginsys/shelly-manager#638, slice 4). The store's actions are
// spies here; the manager's answers are read by the API tests.

const FP_KEYED = 'aaaa-aaaa-aaaa-aaaa-aaaa-aaaa-aaaa-aaaa'
const FP_PENDING = 'bbbb-bbbb-bbbb-bbbb-bbbb-bbbb-bbbb-bbbb'
const FP_PREAPPROVED = 'cccc-cccc-cccc-cccc-cccc-cccc-cccc-cccc'
const FP_REJECTED = 'dddd-dddd-dddd-dddd-dddd-dddd-dddd-dddd'

const fresh = new Date().toISOString()
const stale = '2026-01-01T00:00:00Z'
const base = { version: '1.0.0', capabilities: ['wifi'], lastSeen: fresh }

const approved = { ...base, id: 'keyed-aaaa', name: 'keyed-host', status: 'online', enrolmentState: 'approved', fingerprint: FP_KEYED }
const legacy = { ...base, id: 'old-one', name: 'legacy-host', status: 'online', enrolmentState: 'legacy' }
// The manager stores the state name in `status` for a row that is not enrolled, and `offline` once it is stale.
const pending = { ...base, id: 'keyed-bbbb', name: 'waiting-host', status: 'pending', enrolmentState: 'pending', fingerprint: FP_PENDING }
const preapproved = { ...base, id: 'keyed-cccc', name: '', version: '', capabilities: [], status: 'preapproved', enrolmentState: 'preapproved', fingerprint: FP_PREAPPROVED }
const rejected = { ...base, id: 'keyed-dddd', name: 'refused-host', lastSeen: stale, status: 'offline', enrolmentState: 'rejected', fingerprint: FP_REJECTED }

const realCreateElement = Document.prototype.createElement.bind(document)

beforeEach(() => {
  vi.mocked(document.createElement).mockImplementation(((tag: string, opts?: any) => realCreateElement(tag, opts)) as any)
})
afterEach(() => {
  vi.restoreAllMocks()
})

function mountWith(agents: unknown[]) {
  const wrapper = mount(ProvisioningAgentsPage, {
    global: {
      plugins: [createTestingPinia({ createSpy: vi.fn, initialState: { provisioning: { agents, agentsLoading: false, agentsError: '' } } })],
    },
  })
  return { wrapper, store: useProvisioningStore() }
}

const cells = (tr: { findAll: (s: string) => { text: () => string }[] }) => tr.findAll('td').map((td) => td.text())
const all = [approved, legacy, pending, preapproved, rejected]

describe('ProvisioningAgentsPage: enrolment', () => {
  it('shows each row\'s state and fingerprint, and no fingerprint for a keyless row', async () => {
    const { wrapper } = mountWith(all)
    await flushPromises()
    const rows = wrapper.findAll('.data-table tbody tr')
    expect(rows).toHaveLength(5)
    const byName = (name: string) => rows.find((tr) => tr.text().includes(name))!
    expect(cells(byName('keyed-host'))).toContain('approved')
    expect(cells(byName('keyed-host'))).toContain(FP_KEYED)
    expect(cells(byName('legacy-host'))).toContain('legacy')
    expect(byName('legacy-host').find('.fingerprint').exists()).toBe(false)
    expect(cells(byName('refused-host'))).toContain('rejected')
    expect(cells(byName('refused-host'))).toContain(FP_REJECTED)
  })

  it('says a pre-approved row is not registered yet, and shows no last-seen time for it', async () => {
    const { wrapper } = mountWith(all)
    await flushPromises()
    const row = wrapper.findAll('.data-table tbody tr').find((tr) => tr.text().includes(FP_PREAPPROVED))!
    expect(row.text()).toContain('Not registered yet')
    expect(row.text()).not.toContain('Just now')
  })

  it('does not repeat the state in the Activity column and counts only working agents', async () => {
    const { wrapper } = mountWith(all)
    await flushPromises()
    const rows = wrapper.findAll('.data-table tbody tr')
    // Columns: Name, State, Fingerprint, Activity, ...
    const activity = (name: string) => rows.find((tr) => tr.text().includes(name))!.findAll('td')[3].text()
    expect(activity('keyed-host')).toBe('online')
    expect(activity('legacy-host')).toBe('online')
    expect(activity('waiting-host')).toBe('—')
    expect(activity('refused-host')).toBe('—')
    const summary = wrapper.find('.stats').text()
    expect(summary).toContain('Total: 2')
    expect(summary).toContain('Online: 2')
    expect(summary).toContain('Offline: 0')
    expect(summary).toContain('Not enrolled: 3')
  })

  it('does not repeat `approved` in Activity for a row just approved, before it registers', async () => {
    // Approving stores the state name in `status` too, and a fresh row keeps it until the agent registers.
    const justApproved = { ...pending, status: 'approved', enrolmentState: 'approved' }
    const { wrapper } = mountWith([justApproved])
    await flushPromises()
    const row = wrapper.find('.data-table tbody tr')
    expect(row.findAll('td')[1].text()).toBe('approved')
    expect(row.findAll('td')[3].text()).toBe('—')
  })

  it('lists the pending rows with their fingerprints and says to compare with the agent\'s own log', async () => {
    const { wrapper } = mountWith(all)
    await flushPromises()
    const section = wrapper.find('[data-testid="pending-agents"]')
    expect(section.exists()).toBe(true)
    expect(section.text()).toContain('Waiting for approval (1)')
    expect(section.text()).toContain('waiting-host')
    expect(section.text()).toContain(FP_PENDING)
    expect(section.text()).not.toContain('keyed-host')
    expect(section.text()).toContain("Compare each fingerprint with the one in the agent's own log")
    expect(section.text()).toContain('prove nothing')
  })

  it('has no pending section when nothing waits', async () => {
    const { wrapper } = mountWith([approved, legacy])
    await flushPromises()
    expect(wrapper.find('[data-testid="pending-agents"]').exists()).toBe(false)
  })

  it('offers Approve and Reject only where the manager allows them', async () => {
    const { wrapper } = mountWith(all)
    await flushPromises()
    const has = (kind: string, id: string) => wrapper.find(`[data-testid="row-${kind}-${id}"]`).exists()
    expect([has('approve', 'keyed-aaaa'), has('reject', 'keyed-aaaa')]).toEqual([false, true]) // approved
    expect([has('approve', 'old-one'), has('reject', 'old-one')]).toEqual([false, false]) // legacy
    expect([has('approve', 'keyed-bbbb'), has('reject', 'keyed-bbbb')]).toEqual([true, true]) // pending
    expect([has('approve', 'keyed-cccc'), has('reject', 'keyed-cccc')]).toEqual([false, true]) // preapproved
    expect([has('approve', 'keyed-dddd'), has('reject', 'keyed-dddd')]).toEqual([true, false]) // rejected
  })

  it('approves a pending row at once, with no dialog', async () => {
    const { wrapper, store } = mountWith(all)
    await flushPromises()
    await wrapper.find('[data-testid="approve-keyed-bbbb"]').trigger('click')
    await flushPromises()
    expect(store.approveAgent).toHaveBeenCalledWith('keyed-bbbb')
    expect(wrapper.find('[data-testid="confirm-dialog"]').exists()).toBe(false)
  })

  it('asks before it rejects, and says what rejecting costs', async () => {
    const { wrapper, store } = mountWith(all)
    await flushPromises()
    await wrapper.find('[data-testid="row-reject-keyed-aaaa"]').trigger('click')
    const dialog = wrapper.find('[data-testid="confirm-dialog"]')
    expect(dialog.text()).toContain('keyed-host')
    expect(dialog.text()).toContain('token stops working')
    expect(dialog.text()).toContain('assigned and in-progress tasks fail')
    expect(dialog.text()).toContain('pending tasks stay')
    await wrapper.find('[data-testid="cancel-act"]').trigger('click')
    expect(store.rejectAgent).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="confirm-dialog"]').exists()).toBe(false)

    await wrapper.find('[data-testid="row-reject-keyed-aaaa"]').trigger('click')
    await wrapper.find('[data-testid="confirm-act"]').trigger('click')
    await flushPromises()
    expect(store.rejectAgent).toHaveBeenCalledWith('keyed-aaaa')
    expect(wrapper.find('[data-testid="confirm-dialog"]').exists()).toBe(false)
  })

  it('asks before it approves a rejected row, which lets a refused key back in', async () => {
    const { wrapper, store } = mountWith(all)
    await flushPromises()
    await wrapper.find('[data-testid="row-approve-keyed-dddd"]').trigger('click')
    const dialog = wrapper.find('[data-testid="confirm-dialog"]')
    expect(dialog.text()).toContain('was rejected')
    expect(dialog.text()).toContain('back in')
    expect(store.approveAgent).not.toHaveBeenCalled()
    await wrapper.find('[data-testid="confirm-act"]').trigger('click')
    await flushPromises()
    expect(store.approveAgent).toHaveBeenCalledWith('keyed-dddd')
  })

  it('pre-approves the typed fingerprint, trimmed, and clears the field', async () => {
    const { wrapper, store } = mountWith([])
    await flushPromises()
    const button = wrapper.find('[data-testid="preapprove-form"] button')
    expect(button.attributes('disabled')).toBeDefined()
    await wrapper.find('[data-testid="preapprove-input"]').setValue('  0123-4567-89AB-CDEF-0123-4567-89ab-cdef ')
    expect(button.attributes('disabled')).toBeUndefined()
    await wrapper.find('[data-testid="preapprove-form"]').trigger('submit')
    await flushPromises()
    expect(store.preapproveFingerprint).toHaveBeenCalledWith('0123-4567-89AB-CDEF-0123-4567-89ab-cdef')
    expect((wrapper.find('[data-testid="preapprove-input"]').element as HTMLInputElement).value).toBe('')
  })

  it.each([
    ['approve', 'The agent\'s enrolment state does not allow this', async (w: VueWrapper) => w.find('[data-testid="approve-keyed-bbbb"]').trigger('click')],
    ['reject', 'Agent not found', async (w: VueWrapper) => {
      await w.find('[data-testid="row-reject-keyed-aaaa"]').trigger('click')
      await w.find('[data-testid="confirm-act"]').trigger('click')
    }],
  ])('a refused %s shows the server\'s message once and keeps the rows', async (kind, message, act) => {
    const { wrapper, store } = mountWith(all)
    vi.mocked(kind === 'approve' ? store.approveAgent : store.rejectAgent).mockRejectedValue(new Error(message))
    await flushPromises()
    await act(wrapper)
    await flushPromises()
    expect(wrapper.find('[data-testid="agent-action-error"]').text().split(message).length - 1).toBe(1)
    expect(wrapper.findAll('.data-table tbody tr')).toHaveLength(5)
  })

  it('a refused pre-approve shows the manager\'s message and keeps what was typed', async () => {
    const { wrapper, store } = mountWith([])
    const message = 'A fingerprint is 32 hexadecimal characters, or eight groups of four joined by hyphens'
    vi.mocked(store.preapproveFingerprint).mockRejectedValue(new Error(message))
    await flushPromises()
    await wrapper.find('[data-testid="preapprove-input"]').setValue('not a fingerprint')
    await wrapper.find('[data-testid="preapprove-form"]').trigger('submit')
    await flushPromises()
    expect(wrapper.find('[data-testid="agent-action-error"]').text()).toContain(message)
    expect((wrapper.find('[data-testid="preapprove-input"]').element as HTMLInputElement).value).toBe('not a fingerprint')
  })

  // A pin: the page reads only what the UI list carries, and that holds no key, challenge or token hash.
  it('shows no key, challenge or token hash even from an answer that carries them', async () => {
    const leaky = { ...approved, publicKey: 'SECRET-KEY-VALUE', public_key: 'SECRET-KEY-VALUE', challenge: 'SECRET-CHALLENGE', tokenHash: 'SECRET-HASH', token_hash: 'SECRET-HASH' }
    const { wrapper } = mountWith([leaky])
    await flushPromises()
    expect(wrapper.html()).not.toContain('SECRET')
  })

  it('keeps Refresh Status the first action of a row', async () => {
    const { wrapper } = mountWith([approved])
    await flushPromises()
    expect(wrapper.find('button.link-button').text()).toBe('Refresh Status')
  })
})
