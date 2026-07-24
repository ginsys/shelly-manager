import { expect, test } from '@playwright/test'

const envelope = (data: unknown, meta?: unknown) => ({
  success: true,
  data,
  ...(meta ? { meta } : {}),
  timestamp: '2026-01-02T03:04:05Z',
})

test.describe('mocked export history and preview', () => {
  test.beforeEach(async ({ page }) => {
    await page.route('**/api/v1/export/plugins*', route => route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify(envelope({
        plugins: [{
          name: 'json',
          display_name: 'JSON',
          description: 'JSON export',
          version: '1',
          category: 'custom',
          capabilities: ['json'],
          status: { available: true, configured: true, enabled: true },
        }],
        categories: [],
      })),
    }))
    await page.route('**/api/v1/export/statistics*', route => route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify(envelope({ total: 1, success: 1, failure: 0, by_plugin: { json: 1 } })),
    }))
    await page.route('**/api/v1/export/history*', route => route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify(envelope({
        history: [{
          id: 1,
          export_id: '123e4567-e89b-42d3-a456-426614174000',
          plugin_name: 'json',
          format: 'json',
          requested_by: 'api',
          success: true,
          record_count: 0,
          file_size: 0,
          created_at: '2026-01-02T03:04:05Z',
        }],
      }, {
        pagination: { page: 1, page_size: 20, total_pages: 1, has_next: false, has_previous: false },
      })),
    }))
    await page.route('**/api/v1/export/plugins/json/schema', route => route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify(envelope({ version: '1', properties: {}, required: [] })),
    }))
    await page.route('**/api/v1/export/preview', route => route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify(envelope({
        preview: { success: true, sample_data: '', record_count: 0, estimated_size: 0 },
        summary: { record_count: 0, estimated_size: 0 },
      })),
    }))
  })

  test('renders backend-shaped zero values and filters through the page API', async ({ page }) => {
    await page.goto('/export/history')
    await expect(page.getByRole('heading', { name: 'Export History' })).toBeVisible()
    await expect(page.getByRole('cell', { name: '123e4567-e89b-42d3-a456-426614174000' })).toBeVisible()
    await expect(page.getByRole('cell', { name: '0', exact: true })).toBeVisible()
    await expect(page.getByText('Failure: 0')).toBeVisible()

    const request = page.waitForRequest(request =>
      request.url().includes('/api/v1/export/history') && request.url().includes('plugin=json'),
    )
    await page.getByLabel('Plugin:').fill('json')
    await request

    await page.locator('#export-plugin').selectOption('json')
    await page.locator('#export-format').selectOption('json')
    await page.getByRole('button', { name: 'Preview Export' }).click()
    await expect(page.getByTestId('preview-section')).toContainText('0 B')
    await page.screenshot({
      path: '../docs/frontend/images/registry-export-preview.png',
      fullPage: true,
    })
  })
})
