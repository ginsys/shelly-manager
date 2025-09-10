import { test, expect } from '@playwright/test'

test.describe('Export History E2E', () => {
  test.beforeEach(async ({ page }) => {
    // Navigate to the export history page
    await page.goto('/export')
    await page.waitForLoadState('networkidle')
  })

  test('should display export history with pagination', async ({ page }) => {
    // Check page title
    await expect(page).toHaveTitle(/Export.*Shelly Manager/)
    
    // Check main heading
    await expect(page.locator('h1')).toContainText('Export History')
    
    // Check if the export history table or list is visible
    const historyContainer = page.locator('[data-testid="export-history"]')
    await expect(historyContainer).toBeVisible()
    
    // Check for pagination controls if there are multiple pages
    const paginationExists = await page.locator('[data-testid="pagination"]').isVisible()
    if (paginationExists) {
      await expect(page.locator('[data-testid="pagination"]')).toBeVisible()
    }
  })

  test('should filter export history by plugin', async ({ page }) => {
    // Wait for the filter controls to be visible
    const pluginFilter = page.locator('[data-testid="plugin-filter"]')
    if (await pluginFilter.isVisible()) {
      // Select a specific plugin filter
      await pluginFilter.selectOption('home-assistant')
      
      // Wait for the filtered results
      await page.waitForLoadState('networkidle')
      
      // Verify that results are filtered (all visible items should be home-assistant)
      const exportItems = page.locator('[data-testid="export-item"]')
      const count = await exportItems.count()
      
      if (count > 0) {
        for (let i = 0; i < count; i++) {
          const item = exportItems.nth(i)
          await expect(item).toContainText('home-assistant')
        }
      }
    }
  })

  test('should filter export history by success status', async ({ page }) => {
    const successFilter = page.locator('[data-testid="success-filter"]')
    if (await successFilter.isVisible()) {
      // Filter for successful exports only
      await successFilter.selectOption('true')
      await page.waitForLoadState('networkidle')
      
      // Verify all visible items show success status
      const exportItems = page.locator('[data-testid="export-item"]')
      const count = await exportItems.count()
      
      if (count > 0) {
        for (let i = 0; i < count; i++) {
          const item = exportItems.nth(i)
          const statusIcon = item.locator('[data-testid="success-status"]')
          await expect(statusIcon).toBeVisible()
        }
      }
    }
  })

  test('should handle empty export history gracefully', async ({ page }) => {
    // Check if there's an empty state message
    const emptyState = page.locator('[data-testid="empty-state"]')
    const hasExports = await page.locator('[data-testid="export-item"]').count() > 0
    
    if (!hasExports) {
      await expect(emptyState).toBeVisible()
      await expect(emptyState).toContainText('No export history')
    }
  })

  test('should display export details when clicking on an item', async ({ page }) => {
    const exportItems = page.locator('[data-testid="export-item"]')
    const count = await exportItems.count()
    
    if (count > 0) {
      // Click on the first export item
      await exportItems.first().click()
      
      // Check if details view opens (modal or navigation)
      const detailsModal = page.locator('[data-testid="export-details"]')
      const detailsPage = page.locator('h1:has-text("Export Details")')
      
      const modalVisible = await detailsModal.isVisible({ timeout: 3000 }).catch(() => false)
      const pageVisible = await detailsPage.isVisible({ timeout: 3000 }).catch(() => false)
      
      expect(modalVisible || pageVisible).toBeTruthy()
    }
  })

  test('should navigate between pages using pagination', async ({ page }) => {
    const pagination = page.locator('[data-testid="pagination"]')
    
    if (await pagination.isVisible()) {
      const nextButton = pagination.locator('[data-testid="next-page"]')
      const prevButton = pagination.locator('[data-testid="prev-page"]')
      
      // If there are multiple pages
      if (await nextButton.isEnabled()) {
        // Get current page content
        const firstPageContent = await page.locator('[data-testid="export-item"]').first().textContent()
        
        // Navigate to next page
        await nextButton.click()
        await page.waitForLoadState('networkidle')
        
        // Verify content changed
        const secondPageContent = await page.locator('[data-testid="export-item"]').first().textContent()
        expect(firstPageContent).not.toBe(secondPageContent)
        
        // Navigate back
        await prevButton.click()
        await page.waitForLoadState('networkidle')
        
        // Verify we're back to original content
        const backToFirstContent = await page.locator('[data-testid="export-item"]').first().textContent()
        expect(backToFirstContent).toBe(firstPageContent)
      }
    }
  })

  test('should respond correctly to page size changes', async ({ page }) => {
    const pageSizeSelect = page.locator('[data-testid="page-size"]')
    
    if (await pageSizeSelect.isVisible()) {
      // Count items with default page size
      const initialCount = await page.locator('[data-testid="export-item"]').count()
      
      // Change page size to a smaller value
      await pageSizeSelect.selectOption('5')
      await page.waitForLoadState('networkidle')
      
      // Verify the number of items changed appropriately
      const newCount = await page.locator('[data-testid="export-item"]').count()
      
      if (initialCount > 5) {
        expect(newCount).toBeLessThanOrEqual(5)
      }
    }
  })

  test('should maintain filters when navigating between pages', async ({ page }) => {
    const pluginFilter = page.locator('[data-testid="plugin-filter"]')
    const pagination = page.locator('[data-testid="pagination"]')
    
    if (await pluginFilter.isVisible() && await pagination.isVisible()) {
      // Apply a filter
      await pluginFilter.selectOption('home-assistant')
      await page.waitForLoadState('networkidle')
      
      const nextButton = pagination.locator('[data-testid="next-page"]')
      
      if (await nextButton.isEnabled()) {
        // Navigate to next page
        await nextButton.click()
        await page.waitForLoadState('networkidle')
        
        // Verify filter is still applied
        expect(await pluginFilter.inputValue()).toBe('home-assistant')
        
        // Verify filtered results on the second page
        const exportItems = page.locator('[data-testid="export-item"]')
        const count = await exportItems.count()
        
        if (count > 0) {
          await expect(exportItems.first()).toContainText('home-assistant')
        }
      }
    }
  })
})
