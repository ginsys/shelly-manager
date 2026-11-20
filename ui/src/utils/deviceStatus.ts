// A stored status of "online" is a statement about the last time something read the device, and
// nothing on the server writes "offline" (ginsys/shelly-manager#755). So a device the manager has not
// seen for a day still reads "online" in the list. The list and the detail page show it as not seen
// recently instead, from `last_seen`, which every successful read does update.

export const LAST_SEEN_FRESH_MS = 24 * 3600 * 1000

export const NOT_SEEN_RECENTLY = 'not seen recently'

interface StatusFields {
  status?: string
  last_seen?: string
}

export interface ShownStatus {
  label: string
  // The CSS class of the chip: the stored status, or `stale` for a stale "online".
  cssClass: string
  title?: string
}

export function shownDeviceStatus(device: StatusFields, now: number = Date.now()): ShownStatus {
  const status = device.status || ''
  if (status !== 'online') return { label: status || 'unknown', cssClass: status }
  const seen = Date.parse(device.last_seen ?? '')
  // An unreadable or zero time (the server's zero time is year 1) is older than any limit.
  if (Number.isNaN(seen) || now - seen > LAST_SEEN_FRESH_MS) {
    const when = Number.isNaN(seen) || seen < 0 ? 'never' : new Date(seen).toLocaleString()
    return { label: NOT_SEEN_RECENTLY, cssClass: 'stale', title: `Last seen: ${when}` }
  }
  return { label: 'online', cssClass: 'online' }
}
