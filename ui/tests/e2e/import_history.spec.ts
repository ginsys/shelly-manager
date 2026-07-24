import { expect, test } from '@playwright/test'

const envelope = (data: unknown, meta?: unknown) => ({
  success: true,
  data,
  ...(meta ? { meta } : {}),
  timestamp: '2026-01-02T03:04:05Z',
})

test.describe('mocked import history and SMA preview', () => {
  test.beforeEach(async ({ page }) => {
    await page.route('**/api/v1/export/plugins*', route => route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify(envelope({
        plugins: [{
          name: 'sma',
          display_name: 'SMA',
          description: 'SMA import',
          version: '2026.1',
          category: 'backup',
          capabilities: ['sma'],
          status: { available: true, configured: true, enabled: true },
        }],
        categories: [],
      })),
    }))
    await page.route('**/api/v1/import/statistics*', route => route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify(envelope({ total: 1, success: 1, failure: 0, by_plugin: { sma: 1 } })),
    }))
    await page.route('**/api/v1/import/history*', route => route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify(envelope({
        history: [{
          id: 1,
          import_id: 'sma-import-1',
          plugin_name: 'sma',
          format: 'sma',
          requested_by: 'api',
          success: true,
          records_imported: 0,
          records_skipped: 0,
          created_at: '2026-01-02T03:04:05Z',
        }],
      }, {
        pagination: { page: 1, page_size: 20, total_pages: 1, has_next: false, has_previous: false },
      })),
    }))
  })

  test('renders history and sends the exact browser SMA preview request', async ({ page }) => {
    let previewBody: unknown
    await page.route('**/api/v1/import/preview', async route => {
      previewBody = route.request().postDataJSON()
      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify(envelope({
          preview: {
            success: true,
            import_id: 'preview',
            plugin_name: 'sma',
            format: 'sma',
            records_imported: 0,
            records_skipped: 0,
            changes: [],
            warnings: [],
          },
          changes_count: 0,
          summary: { will_create: 0, will_update: 0, will_delete: 0 },
        })),
      })
    })

    await page.goto('/import/history')
    await expect(page.getByRole('heading', { name: 'Import History' })).toBeVisible()
    await expect(page.getByRole('cell', { name: 'sma-import-1' })).toBeVisible()
    await page.locator('#import-plugin').selectOption('sma')
    await page.locator('#import-text').fill('{}')
    await page.getByRole('button', { name: 'Preview Import' }).click()
    await expect(page.getByText('Will create').locator('..')).toContainText('0')
    expect(previewBody).toEqual({
      plugin_name: 'sma',
      format: 'sma',
      source: { type: 'data', data: 'e30=' },
      config: {},
      options: { dry_run: true, validate_only: true },
    })
    await page.screenshot({
      path: '../docs/frontend/images/sma-import-preview.png',
      fullPage: true,
    })
  })
})
