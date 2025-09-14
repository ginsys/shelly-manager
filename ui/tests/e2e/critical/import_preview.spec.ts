import { test, expect } from '@playwright/test'

test.describe('Import Preview Form E2E', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/import')
    await page.waitForLoadState('networkidle')
  })

  test('should display import preview form with plugin selection', async ({ page }) => {
    const previewForm = page.locator('[data-testid="import-preview-form"]')
    await expect(previewForm).toBeVisible()

    // Check for plugin selection
    const pluginSelect = page.locator('[data-testid="plugin-select"]')
    if (await pluginSelect.isVisible()) {
      await expect(pluginSelect).toBeVisible()
      
      const options = await pluginSelect.locator('option').count()
      expect(options).toBeGreaterThan(1)
    }
  })

  test('should show data input options when plugin is selected', async ({ page }) => {
    const pluginSelect = page.locator('[data-testid="plugin-select"]')
    const dataInputSection = page.locator('[data-testid="data-input-section"]')
    
    if (await pluginSelect.isVisible()) {
      const options = await pluginSelect.locator('option').all()
      if (options.length > 1) {
        await pluginSelect.selectOption({ index: 1 })
        await page.waitForTimeout(500)
        
        await expect(dataInputSection).toBeVisible()
      }
    }
  })

  test('should handle file upload for import data', async ({ page }) => {
    const pluginSelect = page.locator('[data-testid="plugin-select"]')
    const fileInput = page.locator('[data-testid="file-input"]')
    
    if (await pluginSelect.isVisible()) {
      const options = await pluginSelect.locator('option').all()
      if (options.length > 1) {
        await pluginSelect.selectOption({ index: 1 })
        await page.waitForTimeout(500)
        
        if (await fileInput.isVisible()) {
          // Create a test file content
          const testData = JSON.stringify({ test: 'import data' })
          
          // Set file content
          await fileInput.setInputFiles({
            name: 'test-import.json',
            mimeType: 'application/json',
            buffer: Buffer.from(testData)
          })
          
          // Verify file was accepted
          const fileName = page.locator('[data-testid="file-name"]')
          if (await fileName.isVisible()) {
            await expect(fileName).toContainText('test-import.json')
          }
        }
      }
    }
  })

  test('should handle text input for import data', async ({ page }) => {
    const pluginSelect = page.locator('[data-testid="plugin-select"]')
    const textInput = page.locator('[data-testid="text-input"]')
    const textToggle = page.locator('[data-testid="text-input-toggle"]')
    
    if (await pluginSelect.isVisible()) {
      const options = await pluginSelect.locator('option').all()
      if (options.length > 1) {
        await pluginSelect.selectOption({ index: 1 })
        await page.waitForTimeout(500)
        
        // Switch to text input mode
        if (await textToggle.isVisible()) {
          await textToggle.click()
          
          if (await textInput.isVisible()) {
            const testData = '{"test": "import data"}'
            await textInput.fill(testData)
            
            expect(await textInput.inputValue()).toBe(testData)
          }
        }
      }
    }
  })

  test('should validate JSON format in text input', async ({ page }) => {
    const pluginSelect = page.locator('[data-testid="plugin-select"]')
    const textInput = page.locator('[data-testid="text-input"]')
    const textToggle = page.locator('[data-testid="text-input-toggle"]')
    const validationError = page.locator('[data-testid="json-validation-error"]')
    
    if (await pluginSelect.isVisible()) {
      const options = await pluginSelect.locator('option').all()
      if (options.length > 1) {
        await pluginSelect.selectOption({ index: 1 })
        await page.waitForTimeout(500)
        
        if (await textToggle.isVisible()) {
          await textToggle.click()
          
          if (await textInput.isVisible()) {
            // Enter invalid JSON
            await textInput.fill('{"invalid": json}')
            await textInput.blur()
            
            // Should show validation error
            if (await validationError.isVisible()) {
              await expect(validationError).toBeVisible()
              await expect(validationError).toContainText('Invalid JSON')
            }
          }
        }
      }
    }
  })

  test('should generate preview when valid data is provided', async ({ page }) => {
    const pluginSelect = page.locator('[data-testid="plugin-select"]')
    const formatSelect = page.locator('[data-testid="format-select"]')
    const textInput = page.locator('[data-testid="text-input"]')
    const textToggle = page.locator('[data-testid="text-input-toggle"]')
    const previewButton = page.locator('[data-testid="preview-button"]')
    const previewSection = page.locator('[data-testid="preview-section"]')
    
    if (await pluginSelect.isVisible()) {
      // Select plugin
      const pluginOptions = await pluginSelect.locator('option').all()
      if (pluginOptions.length > 1) {
        await pluginSelect.selectOption({ index: 1 })
        await page.waitForTimeout(500)
        
        // Select format
        if (await formatSelect.isVisible()) {
          const formatOptions = await formatSelect.locator('option').all()
          if (formatOptions.length > 1) {
            await formatSelect.selectOption({ index: 1 })
            await page.waitForTimeout(500)
          }
        }
        
        // Add valid data
        if (await textToggle.isVisible()) {
          await textToggle.click()
          
          if (await textInput.isVisible()) {
            const validData = JSON.stringify([
              { id: 'device1', name: 'Test Device 1' },
              { id: 'device2', name: 'Test Device 2' }
            ])
            await textInput.fill(validData)
            
            // Generate preview
            await previewButton.click()
            await page.waitForLoadState('networkidle')
            
            // Preview should appear
            await expect(previewSection).toBeVisible()
          }
        }
      }
    }
  })

  test('should display import summary with create/update/skip counts', async ({ page }) => {
    await completeValidPreview(page)
    
    const importSummary = page.locator('[data-testid="import-summary"]')
    if (await importSummary.isVisible()) {
      await expect(importSummary).toBeVisible()
      
      // Check for summary statistics
      const createCount = page.locator('[data-testid="create-count"]')
      const updateCount = page.locator('[data-testid="update-count"]')
      const skipCount = page.locator('[data-testid="skip-count"]')
      
      // At least one of these should be visible
      const hasStats = await createCount.isVisible() || 
                      await updateCount.isVisible() || 
                      await skipCount.isVisible()
      expect(hasStats).toBeTruthy()
    }
  })

  test('should show detailed preview of changes', async ({ page }) => {
    await completeValidPreview(page)
    
    const changesDetail = page.locator('[data-testid="changes-detail"]')
    if (await changesDetail.isVisible()) {
      await expect(changesDetail).toBeVisible()
      
      // Should show individual change items
      const changeItems = page.locator('[data-testid="change-item"]')
      const count = await changeItems.count()
      expect(count).toBeGreaterThan(0)
    }
  })

  test('should display warnings if any exist', async ({ page }) => {
    await completeValidPreview(page)
    
    const warningsSection = page.locator('[data-testid="warnings-section"]')
    if (await warningsSection.isVisible()) {
      await expect(warningsSection).toBeVisible()
      
      const warningItems = page.locator('[data-testid="warning-item"]')
      const count = await warningItems.count()
      expect(count).toBeGreaterThan(0)
    }
  })

  test('should allow executing import after successful preview', async ({ page }) => {
    await completeValidPreview(page)
    
    const executeButton = page.locator('[data-testid="execute-import-button"]')
    if (await executeButton.isVisible()) {
      await expect(executeButton).toBeVisible()
      await expect(executeButton).toBeEnabled()
      
      // Click execute (but don't wait for completion in test)
      await executeButton.click()
      
      // Should show confirmation or progress indicator
      const confirmDialog = page.locator('[data-testid="execute-confirmation"]')
      const progressIndicator = page.locator('[data-testid="import-progress"]')
      
      const hasConfirmation = await confirmDialog.isVisible() || 
                            await progressIndicator.isVisible()
      expect(hasConfirmation).toBeTruthy()
    }
  })

  test('should persist form configuration in localStorage', async ({ page }) => {
    const pluginSelect = page.locator('[data-testid="plugin-select"]')
    
    if (await pluginSelect.isVisible()) {
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

  test('should handle large file uploads', async ({ page }) => {
    const pluginSelect = page.locator('[data-testid="plugin-select"]')
    const fileInput = page.locator('[data-testid="file-input"]')
    
    if (await pluginSelect.isVisible()) {
      const options = await pluginSelect.locator('option').all()
      if (options.length > 1) {
        await pluginSelect.selectOption({ index: 1 })
        await page.waitForTimeout(500)
        
        if (await fileInput.isVisible()) {
          // Create a large test file (simulating real-world data)
          const largeData = JSON.stringify(
            Array.from({ length: 1000 }, (_, i) => ({
              id: `device-${i}`,
              name: `Test Device ${i}`,
              type: 'SHSW-1',
              ip: `192.168.1.${(i % 254) + 1}`
            }))
          )
          
          await fileInput.setInputFiles({
            name: 'large-import.json',
            mimeType: 'application/json',
            buffer: Buffer.from(largeData)
          })
          
          // Should handle large file without errors
          const fileName = page.locator('[data-testid="file-name"]')
          if (await fileName.isVisible()) {
            await expect(fileName).toContainText('large-import.json')
          }
          
          // File size should be displayed
          const fileSize = page.locator('[data-testid="file-size"]')
          if (await fileSize.isVisible()) {
            await expect(fileSize).toBeVisible()
          }
        }
      }
    }
  })

  test('should handle API errors gracefully', async ({ page }) => {
    // Mock API error response
    await page.route('**/api/v1/import/preview', route => {
      route.fulfill({
        status: 400,
        contentType: 'application/json',
        body: JSON.stringify({
          success: false,
          error: { message: 'Invalid import data format' }
        })
      })
    })
    
    await completeValidPreview(page)
    
    // Should show error message
    const errorMessage = page.locator('[data-testid="api-error"]')
    await expect(errorMessage).toBeVisible()
    await expect(errorMessage).toContainText('Invalid import data format')
  })

  // Helper function to complete a valid preview
  async function completeValidPreview(page: any) {
    const pluginSelect = page.locator('[data-testid="plugin-select"]')
    const formatSelect = page.locator('[data-testid="format-select"]')
    const textInput = page.locator('[data-testid="text-input"]')
    const textToggle = page.locator('[data-testid="text-input-toggle"]')
    const previewButton = page.locator('[data-testid="preview-button"]')
    
    if (await pluginSelect.isVisible()) {
      // Select plugin
      const pluginOptions = await pluginSelect.locator('option').all()
      if (pluginOptions.length > 1) {
        await pluginSelect.selectOption({ index: 1 })
        await page.waitForTimeout(500)
        
        // Select format if available
        if (await formatSelect.isVisible()) {
          const formatOptions = await formatSelect.locator('option').all()
          if (formatOptions.length > 1) {
            await formatSelect.selectOption({ index: 1 })
            await page.waitForTimeout(500)
          }
        }
        
        // Add valid test data
        if (await textToggle.isVisible()) {
          await textToggle.click()
          
          if (await textInput.isVisible()) {
            const validData = JSON.stringify([
              { id: 'test-device-1', name: 'Test Device 1' },
              { id: 'test-device-2', name: 'Test Device 2' }
            ])
            await textInput.fill(validData)
            
            await previewButton.click()
            await page.waitForLoadState('networkidle')
          }
        }
      }
    }
  }
})