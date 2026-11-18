import { expect, test, type Page } from '@playwright/test'

// Every API call is answered here: no device is contacted and nothing is written. A bulk route
// the spec does not stub is aborted, so a request the test did not expect fails the test.

const envelope = (data: unknown, meta?: unknown) => ({
  success: true,
  data,
  ...(meta ? { meta } : {}),
  timestamp: '2026-11-18T20:00:00Z',
})

const devices = [
  { id: 11, name: 'Kitchen', ip: '10.0.0.11', mac: 'AA:BB:CC:00:00:11', type: 'SHSW-1', firmware: '1.0.0', status: 'online', last_seen: '2026-11-18T20:00:00Z' },
  { id: 12, name: 'Garage', ip: '10.0.0.12', mac: 'AA:BB:CC:00:00:12', type: 'SHSW-1', firmware: '1.0.0', status: 'online', last_seen: '2026-11-18T20:00:00Z' },
  { id: 13, name: 'Hallway', ip: '10.0.0.13', mac: 'AA:BB:CC:00:00:13', type: 'SHSW-1', firmware: '1.0.0', status: 'online', last_seen: '2026-11-18T20:00:00Z' },
]

async function stubDeviceList(page: Page) {
  await page.route('**/api/v1/devices?*', route =>
    route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify(envelope(
        { devices },
        { pagination: { page: 1, page_size: 25, total_pages: 1, has_next: false, has_previous: false } },
      )),
    }),
  )
}

// Anything under the bulk routes that no test stubbed is refused, before any request leaves.
async function refuseUnstubbedBulkRoutes(page: Page) {
  await page.route('**/api/v1/config/bulk-*', route => route.abort())
}

test.describe('bulk operations on the devices page (mocked API)', () => {
  test.beforeEach(async ({ page }) => {
    // Registered before the first navigation: later routes win, so the bulk stubs of a test
    // override the refusal below.
    await refuseUnstubbedBulkRoutes(page)
    await stubDeviceList(page)
    await page.goto('/')
    await expect(page.getByTestId('devices-table')).toBeVisible()
  })

  test('the button stays off until a device is ticked', async ({ page }) => {
    const button = page.getByTestId('bulk-ops-btn')
    await expect(button).toBeDisabled()

    await page.getByTestId('select-device').first().check()
    await expect(button).toBeEnabled()
    await expect(button).toContainText('(1)')

    await page.getByTestId('select-device').first().uncheck()
    await expect(button).toBeDisabled()
  })

  test('select all ticks the visible rows', async ({ page }) => {
    await page.getByTestId('select-all-devices').check()

    await expect(page.getByTestId('select-device')).toHaveCount(3)
    for (const box of await page.getByTestId('select-device').all()) {
      await expect(box).toBeChecked()
    }
    await expect(page.getByTestId('bulk-ops-btn')).toContainText('(3)')
  })

  test('sends exactly the two ticked devices and shows the answer', async ({ page }) => {
    let body: unknown
    await page.route('**/api/v1/config/bulk-import', async route => {
      body = route.request().postDataJSON()
      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify(envelope({
          total: 2,
          success: 2,
          failed: 0,
          skipped: 0,
          results: [
            { device_id: 11, device_name: 'Kitchen', device_ip: '10.0.0.11', status: 'success' },
            { device_id: 13, device_name: 'Hallway', device_ip: '10.0.0.13', status: 'success' },
          ],
        })),
      })
    })

    const boxes = page.getByTestId('select-device')
    await boxes.nth(0).check()
    await boxes.nth(2).check()
    await page.getByTestId('bulk-ops-btn').click()

    const picked = page.getByTestId('bulk-picked-device')
    await expect(picked).toHaveCount(2)
    await expect(picked.nth(0)).toContainText('Kitchen')
    await expect(picked.nth(0)).toContainText('10.0.0.11')
    await expect(picked.nth(1)).toContainText('Hallway')

    await page.getByTestId('bulk-op-import').click()
    await page.getByTestId('bulk-submit').click()

    await expect(page.getByTestId('bulk-results')).toContainText('Kitchen')
    expect(body).toEqual({ device_ids: [11, 13], options: { stop_on_error: false } })
  })

  test('a device removed in the dialog is not sent', async ({ page }) => {
    let body: unknown
    await page.route('**/api/v1/config/bulk-drift-detect', async route => {
      body = route.request().postDataJSON()
      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify(envelope({
          total: 1,
          in_sync: 1,
          drifted: 0,
          errors: 0,
          skipped: 0,
          results: [{ device_id: 12, device_name: 'Garage', device_ip: '10.0.0.12', status: 'synced' }],
          started_at: '2026-11-18T20:00:00Z',
          completed_at: '2026-11-18T20:00:01Z',
          duration: 1000000000,
        })),
      })
    })

    await page.getByTestId('select-all-devices').check()
    await page.getByTestId('bulk-ops-btn').click()
    await page.getByTestId('bulk-remove-device').nth(0).click() // Kitchen
    await page.getByTestId('bulk-remove-device').nth(1).click() // Hallway, now second

    await expect(page.getByTestId('bulk-picked-device')).toHaveCount(1)
    await page.getByTestId('bulk-op-drift').click()
    await page.getByTestId('bulk-submit').click()

    await expect(page.getByTestId('bulk-results')).toBeVisible()
    expect(body).toEqual({ device_ids: [12], options: { stop_on_error: false } })
  })

  test('a refused request shows an error in the dialog and no browser dialog', async ({ page }) => {
    const dialogs: string[] = []
    page.on('dialog', async dialog => {
      dialogs.push(dialog.message())
      await dialog.dismiss()
    })
    await page.route('**/api/v1/config/bulk-import', route =>
      route.fulfill({
        status: 400,
        contentType: 'application/json',
        body: JSON.stringify({
          success: false,
          error: { code: 'VALIDATION_FAILED', message: 'Invalid request', details: 'device_ids must not be empty' },
          timestamp: '2026-11-18T20:00:00Z',
        }),
      }),
    )

    await page.getByTestId('select-device').first().check()
    await page.getByTestId('bulk-ops-btn').click()
    await page.getByTestId('bulk-op-import').click()
    await page.getByTestId('bulk-submit').click()

    await expect(page.getByTestId('bulk-error')).toContainText('device_ids must not be empty')
    await expect(page.getByTestId('bulk-submit')).toBeVisible()
    expect(dialogs).toEqual([])
  })
})
