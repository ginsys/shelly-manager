import { describe, it, expect } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useDevicesStore } from './devices'

describe('devices store pagination parsing', () => {
  setActivePinia(createPinia())
  it('defaults non-integer page to 1', () => {
    const s = useDevicesStore()
    s.page = 5
    s.setPageFromQuery('abc')
    expect(s.page).toBe(1)
  })
  it('sets valid integer page', () => {
    const s = useDevicesStore()
    s.setPageFromQuery('3')
    expect(s.page).toBe(3)
  })
})
