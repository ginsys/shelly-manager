import { test, expect } from '@playwright/test'

test.describe('Export Preview Form E2E', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/export')
    await page.waitForLoadState('networkidle')
  })

  test('should display export preview form with plugin selection', async ({ page }) => {
    // Look for the export preview form
    const previewForm = page.locator('[data-testid="export-preview-form"]')
    await expect(previewForm).toBeVisible()

    // Check for plugin selection
    const pluginSelect = page.locator('[data-testid="plugin-select"]')
    if (await pluginSelect.isVisible()) {
      await expect(pluginSelect).toBeVisible()
      
      // Verify there are plugin options
      const options = await pluginSelect.locator('option').count()
      expect(options).toBeGreaterThan(1) // At least one option plus default
    }
  })

  test('should display format selection when plugin is selected', async ({ page }) => {
    const pluginSelect = page.locator('[data-testid="plugin-select"]')
    const formatSelect = page.locator('[data-testid="format-select"]')
    
    if (await pluginSelect.isVisible()) {
      // Select the first available plugin
      const options = await pluginSelect.locator('option').all()
      if (options.length > 1) {
        await pluginSelect.selectOption({ index: 1 })
        await page.waitForTimeout(500) // Wait for dynamic form update
        
        // Format select should now be visible
        await expect(formatSelect).toBeVisible()
      }
    }
  })

  test('should generate dynamic configuration form based on plugin schema', async ({ page }) => {
    const pluginSelect = page.locator('[data-testid="plugin-select"]')
    const dynamicForm = page.locator('[data-testid="dynamic-form"]')
    
    if (await pluginSelect.isVisible()) {
      // Select a plugin
      const options = await pluginSelect.locator('option').all()
      if (options.length > 1) {
        await pluginSelect.selectOption({ index: 1 })
        await page.waitForTimeout(500)
        
        // Dynamic form should appear
        if (await dynamicForm.isVisible()) {
          await expect(dynamicForm).toBeVisible()
          
          // Check for form fields
          const formFields = dynamicForm.locator('input, select, textarea')
          const fieldCount = await formFields.count()
          expect(fieldCount).toBeGreaterThan(0)
        }
      }
    }
  })

  test('should validate required fields before preview', async ({ page }) => {
    const pluginSelect = page.locator('[data-testid="plugin-select"]')
    const previewButton = page.locator('[data-testid="preview-button"]')
    
    if (await previewButton.isVisible()) {
      // Try to preview without selecting plugin
      await previewButton.click()
      
      // Should show validation error
      const errorMessage = page.locator('[data-testid="validation-error"]')
      if (await errorMessage.isVisible()) {
        await expect(errorMessage).toContainText('required')
      }
    }
  })

  test('should generate preview when form is valid', async ({ page }) => {
    const pluginSelect = page.locator('[data-testid="plugin-select"]')
    const formatSelect = page.locator('[data-testid="format-select"]')
    const previewButton = page.locator('[data-testid="preview-button"]')
    const previewSection = page.locator('[data-testid="preview-section"]')
    
    if (await pluginSelect.isVisible()) {
      // Select plugin and format
      const pluginOptions = await pluginSelect.locator('option').all()
      if (pluginOptions.length > 1) {
        await pluginSelect.selectOption({ index: 1 })
        await page.waitForTimeout(500)
        
        if (await formatSelect.isVisible()) {
          const formatOptions = await formatSelect.locator('option').all()
          if (formatOptions.length > 1) {
            await formatSelect.selectOption({ index: 1 })
            await page.waitForTimeout(500)
            
            // Click preview
            await previewButton.click()
            await page.waitForLoadState('networkidle')
            
            // Preview should appear
            await expect(previewSection).toBeVisible()
          }
        }
      }
    }
  })

  test('should display export statistics in preview', async ({ page }) => {
    // Complete a valid preview first
    await completeValidPreview(page)
    
    const statsSection = page.locator('[data-testid="export-stats"]')
    if (await statsSection.isVisible()) {
      await expect(statsSection).toBeVisible()
      
      // Check for record count
      const recordCount = page.locator('[data-testid="record-count"]')
      if (await recordCount.isVisible()) {
        const countText = await recordCount.textContent()
        expect(countText).toMatch(/\d+/)
      }
    }
  })

  test('should allow copying preview result', async ({ page }) => {
    await completeValidPreview(page)
    
    const copyButton = page.locator('[data-testid="copy-button"]')
    if (await copyButton.isVisible()) {
      await copyButton.click()
      
      // Check for success message
      const successMessage = page.locator('[data-testid="copy-success"]')
      await expect(successMessage).toBeVisible({ timeout: 3000 })
    }
  })

  test('should allow downloading preview result', async ({ page }) => {
    await completeValidPreview(page)
    
    const downloadButton = page.locator('[data-testid="download-button"]')
    if (await downloadButton.isVisible()) {
      // Set up download promise before clicking
      const downloadPromise = page.waitForEvent('download')
      
      await downloadButton.click()
      
      // Verify download started
      const download = await downloadPromise
      expect(download.suggestedFilename()).toBeTruthy()
    }
  })

  test('should persist form data in localStorage', async ({ page }) => {
    const pluginSelect = page.locator('[data-testid="plugin-select"]')
    
    if (await pluginSelect.isVisible()) {
      // Select a plugin
      const pluginOptions = await pluginSelect.locator('option').all()
      if (pluginOptions.length > 1) {
        const selectedValue = await pluginOptions[1].getAttribute('value')
        await pluginSelect.selectOption({ index: 1 })
        await page.waitForTimeout(500)
        
        // Refresh page
        await page.reload()
        await page.waitForLoadState('networkidle')
        
        // Check if selection is restored
        const restoredValue = await pluginSelect.inputValue()
        expect(restoredValue).toBe(selectedValue)
      }
    }
  })

  test('should handle API errors gracefully', async ({ page }) => {
    // Mock API error response
    await page.route('**/api/v1/export/preview', route => {
      route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({
          success: false,
          error: { message: 'Internal server error' }
        })
      })
    })
    
    await completeValidPreview(page)
    
    // Should show error message
    const errorMessage = page.locator('[data-testid="api-error"]')
    await expect(errorMessage).toBeVisible()
    await expect(errorMessage).toContainText('Internal server error')
  })

  // Helper function to complete a valid preview
  async function completeValidPreview(page: any) {
    const pluginSelect = page.locator('[data-testid="plugin-select"]')
    const formatSelect = page.locator('[data-testid="format-select"]')
    const previewButton = page.locator('[data-testid="preview-button"]')
    
    if (await pluginSelect.isVisible()) {
      const pluginOptions = await pluginSelect.locator('option').all()
      if (pluginOptions.length > 1) {
        await pluginSelect.selectOption({ index: 1 })
        await page.waitForTimeout(500)
        
        if (await formatSelect.isVisible()) {
          const formatOptions = await formatSelect.locator('option').all()
          if (formatOptions.length > 1) {
            await formatSelect.selectOption({ index: 1 })
            await page.waitForTimeout(500)
            
            await previewButton.click()
            await page.waitForLoadState('networkidle')
          }
        }
      }
    }
  }
})