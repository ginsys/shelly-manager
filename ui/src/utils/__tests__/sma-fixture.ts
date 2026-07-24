import type { SMADevice } from '../sma-parser'

export const validDevice: SMADevice = {
  id: 1,
  mac: 'aabbccddeeff',
  ip: '192.0.2.1',
  type: 'switch',
  name: 'Kitchen',
  model: 'Shelly',
  firmware: '1',
  status: 'online',
  last_seen: '2026-01-02T03:04:05Z',
  settings: {},
  created_at: '2026-01-02T03:04:05Z',
  updated_at: '2026-01-02T03:04:05Z',
}
