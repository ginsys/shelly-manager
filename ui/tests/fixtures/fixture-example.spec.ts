import { test, expect } from '@playwright/test'
import { setupTestEnvironment, waitForPageReady } from '../e2e/fixtures/test-helpers.js'
import { fixtures } from './fixture-helper.js'

/**
 * Example test demonstrating fixture usage
 * This test runs much faster than equivalent tests that make real API calls
 */
test.describe('Fixture Example Tests', () => {
  test.beforeEach(async ({ page }) => {
    // Set up fixtures before each test
    await setupTestEnvironment(page, true)
  })

  test('should load device list with fixture data', async ({ page }) => {
    // Navigate to devices page
    await page.goto('/')
    await waitForPageReady(page)

    // Navigate to devices if not already there
    try {
      await page.click('[data-testid="nav-devices"]')
      await waitForPageReady(page)
    } catch {
      // Already on devices page or nav not available
    }

    // Verify that fixture data is loaded
    // This test runs without making real API calls
    const deviceElements = page.locator('[data-testid="device-card"], .device-item, tr')

    // Should have some devices from fixtures
    await expect(deviceElements.first()).toBeVisible({ timeout: 5000 })

    // Verify we can access fixture data directly for assertions
    expect(fixtures.devices.devices).toHaveLength(5)
    expect(fixtures.devices.devices[0].name).toBe('Test Device 1')
    expect(fixtures.devices.devices[0].type).toBe('SHSW-1')
  })

  test('should load metrics dashboard with fixture data', async ({ page }) => {
    // Navigate to metrics page
    await page.goto('/metrics')
    await waitForPageReady(page)

    // Check for metrics cards - these should load from fixtures
    const metricsElements = page.locator('.card, [data-testid="metric-card"], .stats-section')

    // Should have metrics displayed
    await expect(metricsElements.first()).toBeVisible({ timeout: 5000 })

    // Verify fixture metrics data
    expect(fixtures.metrics.status.enabled).toBe(true)
    expect(fixtures.metrics.devices.total).toBe(5)
    expect(fixtures.metrics.devices.online).toBe(4)
  })

  test('should load plugins page with fixture data', async ({ page }) => {
    // Navigate to plugins page
    await page.goto('/plugins')
    await waitForPageReady(page)

    // Check for plugin elements
    const pluginElements = page.locator('[data-testid="plugin-card"], .plugin-item, .plugin-grid > *')

    // Should have plugins displayed from fixtures
    await expect(pluginElements.first()).toBeVisible({ timeout: 5000 })

    // Verify fixture plugin data
    expect(fixtures.plugins.plugins).toHaveLength(5)
    expect(fixtures.plugins.enabled).toBe(3)
    expect(fixtures.plugins.disabled).toBe(2)
  })

  test('should handle fixture error simulation', async ({ page }) => {
    // This test demonstrates error handling with fixtures
    // The comprehensive fixtures include error simulation
    await page.goto('/export')
    await waitForPageReady(page)

    // Even with potential errors, the page should still load
    const pageContent = page.locator('main, .page-content, #app')
    await expect(pageContent).toBeVisible()

    // Verify export history fixture data
    expect(fixtures.exportHistory.exports).toHaveLength(3)
    expect(fixtures.exportHistory.stats.completed).toBe(2)
    expect(fixtures.exportHistory.stats.failed).toBe(1)
  })
})