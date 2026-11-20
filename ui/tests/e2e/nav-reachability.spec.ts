import { expect, test, type Page } from '@playwright/test'

// Every route is reachable by clicking: from the top bar, or from a page the top bar reaches.
// All API answers come from `page.route`, registered before the first navigation, so no device is
// contacted and nothing is written. The fake data is the smallest that gives each list page one
// row. A page that logs an error with this data is recorded as an annotation, not failed: some
// pages sit on parked work (notifications, drift schedules) and are linked all the same.

const now = '2026-11-19T00:00:00Z'
const exportId = '123e4567-e89b-42d3-a456-426614174000'
const importId = '223e4567-e89b-42d3-a456-426614174001'

const envelope = (data: unknown, meta?: unknown) => ({
  success: true,
  data,
  ...(meta ? { meta } : {}),
  timestamp: now,
})
const pagination = { pagination: { page: 1, page_size: 20, total_pages: 1, has_next: false, has_previous: false } }

const template = { id: 1, name: 'Standard plug', scope: 'global', config: {}, created_at: now, updated_at: now }
// The server's own shape (snake_case, numeric id), as the notification pages receive it.
const channel = { id: 1, name: 'Ops mail', type: 'email', config: {}, enabled: true, description: '', created_at: now, updated_at: now }
const task = {
  id: 't1', deviceId: '31', deviceName: 'Hall', status: 'pending', taskType: 'configure',
  config: {}, createdAt: now, updatedAt: now,
}

// Answers keyed by API path (after /api/v1). Anything else answers an empty object.
const answers: Record<string, unknown> = {
  '/config/templates/new': envelope({ templates: [template] }),
  '/config/templates/new/1': envelope({ template }),
  '/config/device-types': envelope({ device_types: [] }),
  '/notifications/channels': envelope({ channels: [channel] }),
  '/notifications/channels/1': envelope(channel),
  '/notifications/rules': envelope({ rules: [] }),
  '/notifications/history': envelope({ history: [] }),
  '/provisioning/tasks': envelope({ tasks: [task] }),
  '/provisioning/tasks/t1': envelope(task),
  '/provisioning/agents': envelope({ agents: [] }),
  '/config/drift-schedules': envelope([]),
  '/config/drift-scheduler': envelope({ enabled: true }),
  '/config/drift-reports': envelope([]),
  '/config/drift-trends': envelope([]),
  '/devices': envelope({ devices: [] }, pagination),
  '/export/history': envelope(
    { history: [{ id: 1, export_id: exportId, plugin_name: 'json', format: 'json', success: true, record_count: 0, created_at: now }] },
    pagination,
  ),
  '/export/statistics': envelope({ total: 1, success: 1, failure: 0, by_plugin: { json: 1 } }),
  [`/export/${exportId}`]: envelope({ export_id: exportId, plugin_name: 'json', format: 'json', record_count: 0 }),
  '/import/history': envelope(
    { history: [{ id: 1, import_id: importId, plugin_name: 'json', format: 'json', success: true, records_imported: 0, created_at: now }] },
    pagination,
  ),
  '/import/statistics': envelope({ total: 1, success: 1, failure: 0, by_plugin: { json: 1 } }),
  [`/import/${importId}`]: envelope({ import_id: importId, plugin_name: 'json', format: 'json', success: true }),
}

// Page errors per visited path, filled while the test runs.
async function mockAndWatch(page: Page, errors: string[]) {
  await page.route('**/api/v1/**', route => {
    const path = new URL(route.request().url()).pathname.replace(/^\/api\/v1/, '')
    route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify(answers[path] ?? envelope({})),
    })
  })
  page.on('pageerror', err => errors.push(`${new URL(page.url()).pathname}: ${err.message}`))
  page.on('console', msg => {
    // Chrome logs this for every request the API client sends (it sets User-Agent); it says nothing
    // about a page.
    if (msg.type() === 'error' && !msg.text().includes('unsafe header "User-Agent"')) errors.push(`${new URL(page.url()).pathname}: console: ${msg.text()}`)
  })
}

function report(errors: string[]) {
  for (const e of errors) test.info().annotations.push({ type: 'page-error', description: e })
}

// Menu entries: [menu label, item text, route, page heading]
const menuEntries: [string, string, string, string][] = [
  ['Drift', 'Schedules', '/drift/schedules', 'Drift Detection Schedules'],
  ['Drift', 'Reports', '/drift/reports', 'Drift Reports'],
  ['Drift', 'Trends', '/drift/trends', 'Drift Trends'],
  ['Notifications', 'Channels', '/notifications/channels', 'Notification Channels'],
  ['Notifications', 'Rules', '/notifications/rules', 'Notification Rules'],
  ['Notifications', 'History', '/notifications/history', 'Notification History'],
  ['Provisioning', 'Dashboard', '/provisioning', 'Provisioning Dashboard'],
  ['Provisioning', 'Tasks', '/provisioning/tasks', 'Provisioning Tasks'],
  ['Provisioning', 'Agents', '/provisioning/agents', 'Provisioning Agents'],
  ['Export & Import', 'Export History', '/export/history', 'Export History'],
  ['Export & Import', 'Import History', '/import/history', 'Import History'],
]

test.describe('navigation reachability', () => {
  const errors: string[] = []
  test.beforeEach(async ({ page }) => {
    errors.length = 0
    await mockAndWatch(page, errors)
  })
  test.afterEach(() => report(errors))

  for (const [menu, item, path, heading] of menuEntries) {
    test(`${menu} > ${item} leads to ${path}`, async ({ page }) => {
      await page.goto('/')
      await page.getByRole('button', { name: menu }).click()
      await page.locator('.nav-dropdown.open .dropdown-item', { hasText: item }).click()
      await expect(page).toHaveURL(new RegExp(`${path}$`))
      await expect(page.getByRole('heading', { level: 1, name: heading })).toBeVisible()
    })
  }

  test('Export & Import menu still leads to backup and GitOps export', async ({ page }) => {
    await page.goto('/')
    await page.getByRole('button', { name: 'Export & Import' }).click()
    await page.locator('.nav-dropdown.open .dropdown-item', { hasText: 'Backup Management' }).click()
    await expect(page).toHaveURL(/\/export\/backup$/)
    await page.getByRole('button', { name: 'Export & Import' }).click()
    await page.locator('.nav-dropdown.open .dropdown-item', { hasText: 'GitOps Export' }).click()
    await expect(page).toHaveURL(/\/export\/gitops$/)
  })

  test('Templates is a top-bar link, and its detail page opens from the list', async ({ page }) => {
    await page.goto('/')
    await page.getByTestId('navigation').getByRole('link', { name: 'Templates' }).click()
    await expect(page).toHaveURL(/\/templates$/)
    await expect(page.getByRole('heading', { level: 1, name: 'Configuration Templates' })).toBeVisible()
    await page.getByRole('link', { name: 'Standard plug' }).click()
    await expect(page).toHaveURL(/\/templates\/1$/)
    await expect(page.getByRole('heading', { level: 1, name: 'Standard plug' })).toBeVisible()
    await expect(page.getByTestId('navigation').getByRole('link', { name: 'Templates' })).toHaveClass(/active/)
  })

  test('a notification channel and a provisioning task open from their lists', async ({ page }) => {
    await page.goto('/notifications/channels')
    await page.getByRole('link', { name: 'Edit' }).click()
    await expect(page).toHaveURL(/\/notifications\/channels\/1$/)
    await expect(page.getByRole('heading', { level: 1, name: 'Channel Details' })).toBeVisible()

    await page.goto('/provisioning/tasks')
    await page.getByRole('link', { name: 'View' }).click()
    await expect(page).toHaveURL(/\/provisioning\/tasks\/t1$/)
    await expect(page.getByRole('heading', { level: 1, name: 'Task Details' })).toBeVisible()
  })

  test('the history rows open the export and import detail pages', async ({ page }) => {
    await page.goto('/export/history')
    await page.getByRole('link', { name: exportId }).click()
    await expect(page).toHaveURL(new RegExp(`/export/${exportId}$`))
    await expect(page.getByRole('heading', { level: 1, name: 'Export Result' })).toBeVisible()

    await page.goto('/import/history')
    await page.getByRole('link', { name: importId }).click()
    await expect(page).toHaveURL(new RegExp(`/import/${importId}$`))
    await expect(page.getByRole('heading', { level: 1, name: 'Import Result' })).toBeVisible()
  })

  test('a menu opens from the keyboard, its items take focus, and Escape returns to the trigger', async ({ page }) => {
    await page.goto('/')
    const trigger = page.getByRole('button', { name: 'Drift' })
    await trigger.focus()
    await page.keyboard.press('Enter')
    await expect(page.locator('.nav-dropdown.open')).toHaveCount(1)
    await page.keyboard.press('Tab')
    await expect(page.locator('.nav-dropdown.open .dropdown-item').first()).toBeFocused()
    await page.keyboard.press('Escape')
    await expect(page.locator('.nav-dropdown.open')).toHaveCount(0)
    await expect(trigger).toBeFocused()
  })

  test('a menu opens only on a click, hides on Escape with the pointer still over it, and stays while it holds focus', async ({ page }) => {
    await page.goto('/')
    const trigger = page.getByRole('button', { name: 'Notifications' })
    const panel = page.locator('.nav-dropdown', { has: trigger }).locator('.dropdown-menu')

    // Hover alone opens nothing.
    await trigger.hover()
    await expect(panel).toBeHidden()
    await expect(trigger).toHaveAttribute('aria-expanded', 'false')

    // A click opens it, and the visible panel agrees with aria-expanded.
    await trigger.click()
    await expect(panel).toBeVisible()
    await expect(trigger).toHaveAttribute('aria-expanded', 'true')
    await page.keyboard.press('Escape')
    await expect(panel).toBeHidden()
    await expect(trigger).toHaveAttribute('aria-expanded', 'false')

    // Focus moved into an item: moving the mouse away does not hide it.
    await trigger.click()
    await page.keyboard.press('Tab')
    const first = panel.locator('.dropdown-item').first()
    await expect(first).toBeFocused()
    await page.mouse.move(5, 400)
    await expect(panel).toBeVisible()
    await expect(first).toBeFocused()
  })

  test('a menu closes on a click outside it', async ({ page }) => {
    await page.goto('/')
    const trigger = page.getByRole('button', { name: 'Drift' })
    const panel = page.locator('.nav-dropdown', { has: trigger }).locator('.dropdown-menu')
    await trigger.click()
    await expect(panel).toBeVisible()
    await page.mouse.click(5, 400)
    await expect(panel).toBeHidden()
  })

  test('the top bar fits on one line at 1280 px', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 })
    await page.goto('/')
    const nav = page.getByTestId('navigation')
    const header = page.getByTestId('header')
    const [navBox, headerBox] = [await nav.boundingBox(), await header.boundingBox()]
    expect(navBox!.height).toBeLessThanOrEqual(56)
    expect(navBox!.x + navBox!.width).toBeLessThanOrEqual(headerBox!.x + headerBox!.width)
    // the key buttons stay on screen
    await expect(page.getByTestId('admin-key-btn')).toBeInViewport()
  })
})
