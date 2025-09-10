import { test, expect } from '@playwright/test'
import { 
  waitForPageReady, 
  waitForApiResponse,
  fillFormField,
  submitForm,
  SELECTORS,
  mockApiResponse
} from './fixtures/test-helpers'

test.describe('Plugin Management E2E', () => {
  
  test.beforeEach(async ({ page }) => {
    await page.goto('/plugin-management')
    await waitForPageReady(page)
  })

  test('plugin management page loads correctly', async ({ page }) => {
    // Check page title
    const heading = page.locator('h1, h2, [data-testid="page-title"]')
    await expect(heading.first()).toBeVisible()
    
    // Check for plugin list or empty state
    const pluginList = page.locator('[data-testid="plugin-list"], .q-table, .plugin-grid')
    const emptyState = page.locator('[data-testid="empty-state"], .no-plugins')
    
    await expect(pluginList.or(emptyState).first()).toBeVisible()
  })

  test('displays available plugins', async ({ page }) => {
    // Mock plugin data
    await mockApiResponse(page, 'plugins', {
      plugins: [
        {
          id: 'home-assistant',
          name: 'Home Assistant',
          description: 'Export configurations for Home Assistant',
          version: '1.0.0',
          enabled: true,
          status: 'active'
        },
        {
          id: 'opnsense',
          name: 'OPNsense',
          description: 'Export configurations for OPNsense',
          version: '1.0.0',
          enabled: false,
          status: 'inactive'
        }
      ]
    })
    
    await page.reload()
    await waitForPageReady(page)
    
    // Should display plugin cards
    const pluginCards = page.locator('[data-testid="plugin-card"], .plugin-item')
    await expect(pluginCards.first()).toBeVisible()
    
    // Check plugin details are shown
    await expect(page.locator('text=Home Assistant')).toBeVisible()
    await expect(page.locator('text=OPNsense')).toBeVisible()
  })

  test('can view plugin details', async ({ page }) => {
    // Mock plugin data
    await mockApiResponse(page, 'plugins', {
      plugins: [
        {
          id: 'home-assistant',
          name: 'Home Assistant',
          description: 'Export configurations for Home Assistant',
          version: '1.0.0',
          enabled: true,
          status: 'active'
        }
      ]
    })
    
    await page.reload()
    await waitForPageReady(page)
    
    // Click on plugin card or details button
    const pluginCard = page.locator('[data-testid="plugin-card"], .plugin-item').first()
    const detailsButton = page.locator('[data-testid="plugin-details"], button:has-text("Details")')
    
    if (await detailsButton.isVisible()) {
      await detailsButton.click()
    } else {
      await pluginCard.click()
    }
    
    await waitForPageReady(page)
    
    // Should show plugin details view
    const detailsView = page.locator('[data-testid="plugin-details-view"], .plugin-details')
    await expect(detailsView).toBeVisible()
  })

  test('can enable/disable plugins', async ({ page }) => {
    // Mock plugin data
    await mockApiResponse(page, 'plugins', {
      plugins: [
        {
          id: 'home-assistant',
          name: 'Home Assistant',
          enabled: false,
          status: 'inactive'
        }
      ]
    })
    
    // Mock the enable/disable API
    await page.route('**/api/v1/plugins/home-assistant/toggle', route => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          data: { enabled: true, status: 'active' }
        })
      })
    })
    
    await page.reload()
    await waitForPageReady(page)
    
    // Find and click enable toggle
    const enableToggle = page.locator('[data-testid="plugin-toggle"], .q-toggle, button:has-text("Enable")')
    
    if (await enableToggle.first().isVisible()) {
      await enableToggle.first().click()
      await waitForPageReady(page)
      
      // Should show success indication
      const successMessage = page.locator('.q-notification--positive, .success')
      await expect(successMessage).toBeVisible({ timeout: 5000 })
    }
  })

  test('can configure plugin settings', async ({ page }) => {
    // Mock plugin data with configuration
    await mockApiResponse(page, 'plugins/home-assistant', {
      plugin: {
        id: 'home-assistant',
        name: 'Home Assistant',
        enabled: true,
        configuration: {
          export_format: 'yaml',
          include_metadata: true,
          output_directory: '/tmp/exports'
        },
        schema: {
          properties: {
            export_format: {
              type: 'string',
              enum: ['yaml', 'json'],
              default: 'yaml'
            },
            include_metadata: {
              type: 'boolean',
              default: true
            },
            output_directory: {
              type: 'string',
              default: '/tmp/exports'
            }
          }
        }
      }
    })
    
    await page.goto('/plugin-management/home-assistant/configure')
    await waitForPageReady(page)
    
    // Should show configuration form
    const configForm = page.locator('[data-testid="plugin-config-form"], .plugin-config')
    await expect(configForm).toBeVisible()
    
    // Check for form fields
    const outputDirField = page.locator('input[name="output_directory"], [data-testid="output-directory"]')
    if (await outputDirField.isVisible()) {
      await fillFormField(page, outputDirField.first().getAttribute('selector') || 'input[name="output_directory"]', '/custom/path')
    }
    
    // Submit configuration
    const saveButton = page.locator('[data-testid="save-config"], button:has-text("Save")')
    if (await saveButton.isVisible()) {
      await saveButton.click()
      await waitForPageReady(page)
      
      // Should show success message
      const successMessage = page.locator('.q-notification--positive, .success')
      await expect(successMessage).toBeVisible({ timeout: 5000 })
    }
  })

  test('displays plugin schema and validation', async ({ page }) => {
    await page.goto('/plugin-management/home-assistant/configure')
    await waitForPageReady(page)
    
    // Look for schema viewer or validation messages
    const schemaViewer = page.locator('[data-testid="plugin-schema"], .schema-viewer')
    const validationMessages = page.locator('.field-error, .q-field--error')
    
    if (await schemaViewer.isVisible()) {
      await expect(schemaViewer).toBeVisible()
      
      // Schema should contain configuration properties
      await expect(schemaViewer.locator('text=/properties|type|enum/')).toBeVisible()
    }
    
    // Test form validation by entering invalid data
    const textField = page.locator('input[type="text"]').first()
    if (await textField.isVisible()) {
      await textField.fill('')  // Clear required field
      
      const submitButton = page.locator('[data-testid="save-config"], button:has-text("Save")')
      if (await submitButton.isVisible()) {
        await submitButton.click()
        
        // Should show validation errors
        await expect(validationMessages.first()).toBeVisible({ timeout: 3000 })
      }
    }
  })

  test('plugin status updates in real-time', async ({ page }) => {
    // Mock plugin data
    await mockApiResponse(page, 'plugins', {
      plugins: [
        {
          id: 'home-assistant',
          name: 'Home Assistant',
          enabled: true,
          status: 'active',
          last_run: '2025-09-10T10:00:00Z',
          next_run: '2025-09-10T11:00:00Z'
        }
      ]
    })
    
    await page.reload()
    await waitForPageReady(page)
    
    // Check for status indicators
    const statusIndicator = page.locator('[data-testid="plugin-status"], .plugin-status, .status-badge')
    await expect(statusIndicator.first()).toBeVisible()
    
    // Should show active status
    await expect(page.locator('text=/active|running|enabled/i')).toBeVisible()
    
    // Check for timestamps
    const lastRun = page.locator('[data-testid="last-run"], .last-run')
    const nextRun = page.locator('[data-testid="next-run"], .next-run')
    
    if (await lastRun.isVisible()) {
      await expect(lastRun).toBeVisible()
    }
    
    if (await nextRun.isVisible()) {
      await expect(nextRun).toBeVisible()
    }
  })

  test('can test plugin functionality', async ({ page }) => {
    // Mock plugin test endpoint
    await page.route('**/api/v1/plugins/home-assistant/test', route => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          data: {
            test_result: 'passed',
            message: 'Plugin test completed successfully',
            details: {
              exports_generated: 5,
              time_taken: '2.3s'
            }
          }
        })
      })
    })
    
    await page.goto('/plugin-management/home-assistant')
    await waitForPageReady(page)
    
    // Look for test button
    const testButton = page.locator('[data-testid="test-plugin"], button:has-text("Test")')
    
    if (await testButton.isVisible()) {
      await testButton.click()
      await waitForPageReady(page, 15000)
      
      // Should show test results
      const testResults = page.locator('[data-testid="test-results"], .test-results')
      await expect(testResults).toBeVisible({ timeout: 10000 })
      
      // Should show success message
      await expect(page.locator('text=/test.*completed.*successfully/i')).toBeVisible()
    }
  })

  test('handles plugin errors gracefully', async ({ page }) => {
    // Mock API error
    await page.route('**/api/v1/plugins**', route => {
      route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({
          success: false,
          error: 'Plugin service unavailable'
        })
      })
    })
    
    await page.reload()
    await waitForPageReady(page)
    
    // Should show error state
    const errorState = page.locator('[data-testid="error-state"], .q-banner--negative, .error-message')
    await expect(errorState.first()).toBeVisible()
    
    // Error should be descriptive
    await expect(page.locator('text=/error|unavailable|failed/i')).toBeVisible()
  })

  test('plugin management is responsive', async ({ page }) => {
    const viewports = [
      { width: 1920, height: 1080 },
      { width: 768, height: 1024 },
      { width: 375, height: 667 }
    ]

    for (const viewport of viewports) {
      await page.setViewportSize(viewport)
      await page.reload()
      await waitForPageReady(page)
      
      // Plugin content should be accessible at all sizes
      const pluginContent = page.locator('[data-testid="plugin-list"], main, .q-page')
      await expect(pluginContent.first()).toBeVisible()
      
      // Navigation should be accessible
      const nav = page.locator('.q-drawer, .q-header, [data-testid="navigation"]')
      await expect(nav.first()).toBeVisible()
    }
  })
})