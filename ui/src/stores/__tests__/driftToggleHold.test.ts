import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('@/api/drift', async importOriginal => {
  const actual = await importOriginal<typeof import('@/api/drift')>()
  return { ...actual, listDriftSchedules: vi.fn(), toggleDriftSchedule: vi.fn() }
})

import * as driftApi from '@/api/drift'
import { useDriftStore } from '../drift'

const schedule = (id: number, enabled: boolean) =>
  ({ id, name: `s${id}`, enabled, cron_spec: '0 * * * *', device_ids: [], device_filter: null }) as never

describe('drift store toggle hold', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(driftApi.toggleDriftSchedule).mockReset()
  })

  it('marks the schedule while its toggle runs and sends no second request for it', async () => {
    const store = useDriftStore()
    store.schedules = [schedule(1, true), schedule(2, true)]
    let answer: (s: never) => void = () => {}
    vi.mocked(driftApi.toggleDriftSchedule).mockImplementationOnce(() => new Promise(resolve => (answer = resolve as never)))

    const first = store.toggle(1)
    expect(store.togglingIds).toEqual([1])
    await store.toggle(1)
    expect(driftApi.toggleDriftSchedule).toHaveBeenCalledTimes(1)

    answer(schedule(1, false))
    await first
    expect(store.togglingIds).toEqual([])
    expect(store.schedules.find(s => s.id === 1)?.enabled).toBe(false)
  })

  it('releases the mark after a failed request', async () => {
    const store = useDriftStore()
    vi.mocked(driftApi.toggleDriftSchedule).mockRejectedValueOnce(new Error('refused'))
    await expect(store.toggle(1)).rejects.toThrow('refused')
    expect(store.togglingIds).toEqual([])
  })

  it('reads a string id as the same schedule', async () => {
    const store = useDriftStore()
    vi.mocked(driftApi.toggleDriftSchedule).mockImplementationOnce(() => new Promise(() => {}))
    void store.toggle('3')
    expect(store.togglingIds).toEqual([3])
  })
})
