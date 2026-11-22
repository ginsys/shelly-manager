import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('@/api/provisioning', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/provisioning')>()
  return {
    ...actual,
    getAgents: vi.fn(),
    getAgentStatus: vi.fn(),
    approveAgent: vi.fn(),
    rejectAgent: vi.fn(),
    preapproveFingerprint: vi.fn(),
  }
})

import { getAgents, getAgentStatus, approveAgent, rejectAgent, preapproveFingerprint } from '@/api/provisioning'
import { useProvisioningStore } from '../provisioning'

const FP = '0123-4567-89ab-cdef-0123-4567-89ab-cdef'
const agent = (over: Record<string, unknown>) => ({
  id: 'a', name: 'a', status: 'online', version: '1', capabilities: [], lastSeen: '2026-11-22T00:00:00Z',
  enrolmentState: 'legacy', ...over,
}) as any

describe('provisioning store: enrolment', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('counts only working agents, and lists pending and not enrolled rows separately', () => {
    const store = useProvisioningStore()
    store.agents = [
      agent({ id: '1', enrolmentState: 'approved', fingerprint: FP }),
      agent({ id: '2', enrolmentState: 'legacy', status: 'busy' }),
      agent({ id: '3', enrolmentState: 'pending', status: 'pending', fingerprint: FP }),
      agent({ id: '4', enrolmentState: 'preapproved', status: 'preapproved' }),
      agent({ id: '5', enrolmentState: 'rejected', status: 'offline' }),
    ]
    expect(store.workingAgents.map((a) => a.id)).toEqual(['1', '2'])
    expect(store.onlineAgents.map((a) => a.id)).toEqual(['1'])
    expect(store.busyAgents.map((a) => a.id)).toEqual(['2'])
    expect(store.offlineAgents).toEqual([]) // the stale rejected row is not an offline agent
    expect(store.pendingAgents.map((a) => a.id)).toEqual(['3'])
    expect(store.notEnrolledAgents.map((a) => a.id)).toEqual(['3', '4', '5'])
  })

  it('Refresh Status keeps the row\'s state and fingerprint', async () => {
    const store = useProvisioningStore()
    store.agents = [agent({ id: 'k', enrolmentState: 'approved', fingerprint: FP, status: 'busy' })]
    vi.mocked(getAgentStatus).mockResolvedValue(agent({ id: 'k', enrolmentState: 'approved', fingerprint: FP, status: 'online' }))
    await store.refreshAgentStatus('k')
    expect(store.agents[0]).toMatchObject({ status: 'online', enrolmentState: 'approved', fingerprint: FP })
  })

  it('each act calls the API, then loads the list again', async () => {
    const store = useProvisioningStore()
    vi.mocked(getAgents).mockResolvedValue([agent({ id: 'k', enrolmentState: 'approved', fingerprint: FP })])
    await store.approveAgent('k')
    await store.rejectAgent('k')
    await store.preapproveFingerprint(FP)
    expect(approveAgent).toHaveBeenCalledWith('k')
    expect(rejectAgent).toHaveBeenCalledWith('k')
    expect(preapproveFingerprint).toHaveBeenCalledWith(FP)
    expect(getAgents).toHaveBeenCalledTimes(3)
    expect(store.agents[0].enrolmentState).toBe('approved')
  })

  it('a refused act rethrows, loads nothing and leaves agentsError and the list alone', async () => {
    const store = useProvisioningStore()
    store.agents = [agent({ id: 'k' })]
    vi.mocked(approveAgent).mockRejectedValue(new Error("The agent's enrolment state does not allow this"))
    await expect(store.approveAgent('k')).rejects.toThrow('does not allow this')
    expect(getAgents).not.toHaveBeenCalled()
    expect(store.agentsError).toBe('')
    expect(store.agents).toHaveLength(1)
  })
})
