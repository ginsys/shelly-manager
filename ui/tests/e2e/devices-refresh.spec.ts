import { expect, test } from '@playwright/test'

// The list answers come from `page.route`, registered before the first navigation, so no device
// is contacted and nothing is written.

const envelope = (data: unknown, meta?: unknown) => ({
  success: true,
  data,
  ...(meta ? { meta } : {}),
  timestamp: '2026-11-18T20:00:00Z',
})

const devices = [
  { id: 21, name: 'Kitchen', ip: '10.0.0.21', mac: 'AA:BB:CC:00:00:21', type: 'SHSW-1', firmware: '1.0.0', status: 'online', last_seen: '2026-11-18T20:00:00Z' },
  { id: 22, name: 'Garage', ip: '10.0.0.22', mac: 'AA:BB:CC:00:00:22', type: 'SHSW-1', firmware: '1.0.0', status: 'online', last_seen: '2026-11-18T20:00:00Z' },
]

test('the refresh button and the R key reload the device list', async ({ page }) => {
  let listRequests = 0
  let release: () => void = () => {}
  let hold: Promise<void> | null = null

  await page.route('**/api/v1/version', route =>
    route.fulfill({ contentType: 'application/json', body: JSON.stringify(envelope({})) }),
  )
  await page.route('**/api/v1/devices?*', async route => {
    listRequests += 1
    // A held answer shows the loading state; the first load and the third are answered at once.
    if (hold) await hold
    await route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify(envelope(
        { devices },
        { pagination: { page: 1, page_size: 25, total_pages: 1, has_next: false, has_previous: false } },
      )),
    })
  })

  await page.goto('/')
  await expect(page.getByTestId('devices-table')).toBeVisible()
  expect(listRequests).toBe(1)

  const refresh = page.getByTestId('refresh-btn')
  await expect(refresh).toBeEnabled()

  // The button reloads, shows the loading label and is off while the answer is held.
  hold = new Promise<void>(resolve => { release = resolve })
  await refresh.click()
  await expect(refresh).toBeDisabled()
  await expect(refresh).toHaveText('Refreshing...')
  expect(listRequests).toBe(2)
  hold = null
  release()
  await expect(refresh).toBeEnabled()
  await expect(refresh).toHaveText('Refresh')
  await expect(page.getByTestId('devices-table')).toBeVisible()

  // A plain R reloads too, but not while the focus is in the search box.
  await page.getByTestId('device-search').focus()
  await page.keyboard.press('r')
  await expect(page.getByTestId('device-search')).toHaveValue('r')
  expect(listRequests).toBe(2)

  await page.getByTestId('device-search').fill('')
  await page.getByTestId('page-title').click()
  await page.keyboard.press('r')
  await expect.poll(() => listRequests).toBe(3)
})
