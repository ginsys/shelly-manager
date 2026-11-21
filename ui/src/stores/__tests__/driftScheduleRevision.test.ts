import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('@/api/drift', async importOriginal => {
  const actual = await importOriginal<typeof import('@/api/drift')>()
  return { ...actual, updateDriftSchedule: vi.fn(), toggleDriftSchedule: vi.fn() }
})

import * as driftApi from '@/api/drift'
import { useDriftStore } from '../drift'

const row = (revision: number, name: string, enabled = true): driftApi.DriftSchedule => ({
  id: 1, name, enabled, cron_spec: '0 * * * *', device_ids: [], device_filter: null,
  run_count: 0, revision, created_at: 'x', updated_at: 'x'
})
const input = (name: string): driftApi.DriftScheduleInput => ({ name, cron_spec: '0 * * * *' })

// ginsys/shelly-manager#790: an update is made from the revision its form was opened on, and a refusal as
// stale leaves the stored change in place and the list on the current schedule.
describe('drift store update by revision', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(driftApi.updateDriftSchedule).mockReset()
    vi.mocked(driftApi.toggleDriftSchedule).mockReset()
  })

  it('sends the revision it is given and puts the answer in the list', async () => {
    const store = useDriftStore()
    store.schedules = [row(2, 'old')]
    vi.mocked(driftApi.updateDriftSchedule).mockResolvedValueOnce(row(3, 'new'))

    await store.update(1, input('new'), 2)

    expect(driftApi.updateDriftSchedule).toHaveBeenCalledWith(1, input('new'), 2)
    expect(store.schedules).toEqual([row(3, 'new')])
  })

  it('puts the current schedule in the list and rethrows when the update is stale', async () => {
    const store = useDriftStore()
    store.schedules = [row(2, 'old')]
    const stale = new driftApi.StaleScheduleError('changed', row(4, 'saved elsewhere'))
    vi.mocked(driftApi.updateDriftSchedule).mockRejectedValueOnce(stale)

    await expect(store.update(1, input('mine'), 2)).rejects.toBe(stale)

    expect(store.schedules).toEqual([row(4, 'saved elsewhere')])
  })

  it('leaves the list alone when the update fails for another reason', async () => {
    const store = useDriftStore()
    store.schedules = [row(2, 'old')]
    vi.mocked(driftApi.updateDriftSchedule).mockRejectedValueOnce(new Error('cron spec is invalid'))

    await expect(store.update(1, input('mine'), 2)).rejects.toThrow('cron spec is invalid')

    expect(store.schedules).toEqual([row(2, 'old')])
  })

  // Sequence 1 of the issue: an edit saves and returns first, a slower toggle answer (made before the edit)
  // then puts its old row in the store, and the next edit is made from that old row. The server refuses it,
  // and the store ends on the schedule as stored, not on the old fields.
  it('ends a save made from a late toggle answer refused, not silently reverted', async () => {
    const store = useDriftStore()
    store.schedules = [row(1, 'nightly')]
    let answerToggle: (s: driftApi.DriftSchedule) => void = () => {}
    vi.mocked(driftApi.toggleDriftSchedule).mockImplementationOnce(
      () => new Promise(resolve => (answerToggle = resolve))
    )
    const toggling = store.toggle(1) // sent first; the server applies it as revision 2
    vi.mocked(driftApi.updateDriftSchedule).mockResolvedValueOnce(row(3, 'renamed', false))

    await store.update(1, input('renamed'), 1) // the edit saved and returned first
    answerToggle(row(2, 'nightly', false)) // the late answer holds the fields from before the edit
    await toggling
    expect(store.schedules[0].name).toBe('nightly') // the old row is shown until the next write

    const current = row(3, 'renamed', false)
    vi.mocked(driftApi.updateDriftSchedule).mockRejectedValueOnce(new driftApi.StaleScheduleError('changed', current))
    await expect(store.update(1, input('nightly again'), store.schedules[0].revision)).rejects.toThrow('changed')

    expect(driftApi.updateDriftSchedule).toHaveBeenLastCalledWith(1, input('nightly again'), 2)
    expect(store.schedules[0].name).toBe('renamed')
    expect(store.schedules[0].revision).toBe(3)
  })
})
