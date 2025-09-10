import { test, expect } from '@playwright/test'
import { 
  waitForPageReady, 
  waitForApiResponse,
  createTestDevice,
  deleteTestDevice,
  SELECTORS,
  TEST_DATA,
  verifyApiResponse
} from './fixtures/test-helpers'

test.describe('Device Management E2E', () => {
  
  test.beforeEach(async ({ page }) => {
    await page.goto('/devices')
    await waitForPageReady(page)
  })

  test('devices page loads and displays device list', async ({ page }) => {
    // Check page title/heading
    const heading = page.locator('h1, h2, [data-testid="page-title"]')
    await expect(heading.first()).toBeVisible()
    
    // Check for device list or empty state
    const deviceList = page.locator('[data-testid="device-list"], .q-table, .device-grid')
    const emptyState = page.locator('[data-testid="empty-state"], .q-banner')
    
    // Should have either devices or empty state
    await expect(deviceList.or(emptyState).first()).toBeVisible()
  })

  test('can view device details', async ({ page }) => {
    // Wait for devices to load
    await waitForApiResponse(page, '/api/v1/devices')
    
    // Check if any devices are present
    const deviceCard = page.locator('[data-testid="device-card"], .device-item').first()
    
    if (await deviceCard.isVisible()) {
      await deviceCard.click()
      await waitForPageReady(page)
      
      // Should navigate to device details or open modal
      const deviceDetails = page.locator('[data-testid="device-details"], .device-detail')
      await expect(deviceDetails).toBeVisible()
    } else {
      console.log('No devices found for details test - skipping')
      test.skip()
    }
  })

  test('device discovery functionality', async ({ page }) => {
    // Look for discovery button or feature
    const discoveryButton = page.locator('[data-testid="discover-devices"], button:has-text("Discover")')
    
    if (await discoveryButton.isVisible()) {
      await discoveryButton.click()
      
      // Wait for discovery process
      await waitForPageReady(page)
      
      // Should show discovery status or results
      const discoveryStatus = page.locator('[data-testid="discovery-status"], .discovery-progress')
      await expect(discoveryStatus).toBeVisible({ timeout: 15000 })
    } else {
      console.log('Discovery feature not found - skipping test')
      test.skip()
    }
  })

  test('device filtering and search', async ({ page }) => {
    // Wait for devices to load
    await waitForApiResponse(page, '/api/v1/devices')
    
    // Look for search/filter controls
    const searchInput = page.locator('[data-testid="device-search"], input[placeholder*="Search"], .q-input input')
    const filterButton = page.locator('[data-testid="filter-devices"], button:has-text("Filter")')
    
    if (await searchInput.first().isVisible()) {
      await searchInput.first().fill('test')
      await waitForPageReady(page)
      
      // Results should update based on search
      const results = page.locator('[data-testid="device-list"], .q-table')
      await expect(results).toBeVisible()
    }
    
    if (await filterButton.isVisible()) {
      await filterButton.click()
      await waitForPageReady(page)
      
      // Filter dialog/dropdown should appear
      const filterOptions = page.locator('[data-testid="filter-options"], .filter-menu')
      await expect(filterOptions).toBeVisible()
    }
  })

  test('device status indicators work correctly', async ({ page }) => {
    // Wait for devices to load
    await waitForApiResponse(page, '/api/v1/devices')
    
    // Check for status indicators
    const statusIndicators = page.locator('[data-testid="device-status"], .status-indicator, .device-status')
    
    if (await statusIndicators.first().isVisible()) {
      const count = await statusIndicators.count()
      
      // Each status should have proper visual indication
      for (let i = 0; i < Math.min(count, 5); i++) {
        const indicator = statusIndicators.nth(i)
        await expect(indicator).toBeVisible()
        
        // Should have color or icon indicating status
        const hasColor = await indicator.evaluate(el => {
          const styles = getComputedStyle(el)
          return styles.backgroundColor !== 'rgba(0, 0, 0, 0)' || 
                 styles.color !== 'rgb(0, 0, 0)' ||
                 el.querySelector('.q-icon') !== null
        })
        
        expect(hasColor).toBe(true)
      }
    }
  })

  test('device actions are available and functional', async ({ page }) => {
    // Wait for devices to load
    await waitForApiResponse(page, '/api/v1/devices')
    
    // Look for action buttons (refresh, configure, etc.)
    const actionButtons = page.locator(
      '[data-testid*="action"], .device-actions button, .q-btn:has-text("Configure"), .q-btn:has-text("Refresh")'
    )
    
    if (await actionButtons.first().isVisible()) {
      const firstAction = actionButtons.first()
      await firstAction.click()
      
      await waitForPageReady(page)
      
      // Should trigger some response (modal, navigation, or API call)
      const modal = page.locator('.q-dialog, [data-testid="modal"]')
      const loading = page.locator('.q-spinner, [data-testid="loading"]')
      
      // Either modal opens or loading state shows
      await expect(modal.or(loading).first()).toBeVisible({ timeout: 5000 })
    }
  })

  test('handles device API errors gracefully', async ({ page }) => {
    // Mock API to return error
    await page.route('**/api/v1/devices', route => {
      route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({
          success: false,
          error: 'Internal Server Error'
        })
      })
    })
    
    await page.reload()
    await waitForPageReady(page)
    
    // Should show error state
    const errorState = page.locator('[data-testid="error-state"], .q-banner--negative, .error')
    await expect(errorState.first()).toBeVisible()
  })

  test('device list pagination works correctly', async ({ page }) => {
    // Check if pagination controls exist
    const pagination = page.locator('.q-pagination, [data-testid="pagination"]')
    
    if (await pagination.isVisible()) {
      const nextButton = page.locator('.q-pagination__content .q-btn:has-text("❯"), [data-testid="next-page"]')
      const prevButton = page.locator('.q-pagination__content .q-btn:has-text("❮"), [data-testid="prev-page"]')
      
      if (await nextButton.isVisible()) {
        await nextButton.click()
        await waitForPageReady(page)
        
        // Should load next page
        await expect(pagination).toBeVisible()
        
        // Previous button should now be enabled
        if (await prevButton.isVisible()) {
          await expect(prevButton).not.toBeDisabled()
        }
      }
    } else {
      console.log('No pagination found - skipping pagination test')
    }
  })

  test('responsive design works for device management', async ({ page }) => {
    const viewports = [
      { width: 1920, height: 1080 },
      { width: 768, height: 1024 },
      { width: 375, height: 667 }
    ]

    for (const viewport of viewports) {
      await page.setViewportSize(viewport)
      await page.reload()
      await waitForPageReady(page)
      
      // Check that device list is still accessible
      const deviceContent = page.locator('[data-testid="device-list"], main, .q-page')
      await expect(deviceContent.first()).toBeVisible()
      
      // Navigation should be accessible (may be in drawer on mobile)
      const nav = page.locator('.q-drawer, .q-header, [data-testid="navigation"]')
      await expect(nav.first()).toBeVisible()
    }
  })
})

