import { expect, test } from '@playwright/test'

// The device list answers come from `page.route`, registered before the first navigation, so no
// device is contacted and nothing is written. The mock plays a server with an admin key set: it
// answers 401 UNAUTHORIZED until the request carries `Bearer right-key`.

const goodKey = 'right-key'

const envelope = (data: unknown, meta?: unknown) => ({
  success: true,
  data,
  ...(meta ? { meta } : {}),
  timestamp: '2026-11-19T00:00:00Z',
})

const devices = [
  { id: 31, name: 'Hall', ip: '10.0.0.31', mac: 'AA:BB:CC:00:00:31', type: 'SHSW-1', firmware: '1.0.0', status: 'online', last_seen: '2026-11-19T00:00:00Z' },
]

test('the UI asks for the admin key, keeps it for the tab, and forgets it on request', async ({ page }) => {
  const seen: (string | undefined)[] = []

  await page.route('**/api/v1/version', route =>
    route.fulfill({ contentType: 'application/json', body: JSON.stringify(envelope({})) }),
  )
  await page.route('**/api/v1/devices?*', async route => {
    const auth = route.request().headers()['authorization']
    seen.push(auth)
    if (auth === `Bearer ${goodKey}`) {
      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify(envelope(
          { devices },
          { pagination: { page: 1, page_size: 25, total_pages: 1, has_next: false, has_previous: false } },
        )),
      })
      return
    }
    await route.fulfill({
      status: 401,
      contentType: 'application/json',
      body: JSON.stringify({
        success: false,
        error: { code: 'UNAUTHORIZED', message: 'Admin authorization required' },
        timestamp: '2026-11-19T00:00:00Z',
      }),
    })
  })

  const dialog = page.getByTestId('admin-key-dialog')
  const input = page.getByTestId('admin-key-input')

  // The first 401 opens the dialog by itself.
  await page.goto('/')
  await expect(dialog).toBeVisible()

  // A wrong key is tried once, then the dialog shows one message and asks again.
  await input.fill('wrong-key')
  await page.getByTestId('admin-key-submit').click()
  await expect(page.getByTestId('admin-key-message')).toContainText('That key was not accepted.')
  await expect(dialog).toBeVisible()
  expect(seen).toEqual([undefined, 'Bearer wrong-key'])

  // The right key closes it and is kept for this tab only.
  await input.fill(goodKey)
  await page.getByTestId('admin-key-submit').click()
  await expect(dialog).toBeHidden()
  const stored = await page.evaluate(() => ({
    session: Object.values(sessionStorage),
    local: Object.values(localStorage),
  }))
  expect(stored.session).toContain(goodKey)
  expect(stored.local).not.toContain(goodKey)

  // A reload keeps the session: the list loads with no dialog.
  await page.reload()
  await expect(page.getByTestId('devices-table')).toBeVisible()
  await expect(dialog).toBeHidden()
  expect(seen[seen.length - 1]).toBe(`Bearer ${goodKey}`)

  // "Forget key" clears it, and the next load asks again.
  await page.getByTestId('admin-key-forget-btn').click()
  await expect(page.getByTestId('admin-key-forget-btn')).toBeHidden()
  await page.reload()
  await expect(dialog).toBeVisible()
})
