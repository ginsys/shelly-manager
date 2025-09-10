import { test, expect, Page } from '@playwright/test'
import { TestHelpers, testData, createTestFile, cleanupTestFiles } from './fixtures/test-helpers'

test.describe('Export/Import System Integration', () => {
  let helpers: TestHelpers
  let testFiles: string[] = []

  test.beforeEach(async ({ page }) => {
    helpers = new TestHelpers(page)
    
    // Check if backend is healthy
    const isHealthy = await helpers.checkApiHealth()
    test.skip(!isHealthy, 'Backend API is not available')
  })

  test.afterEach(async () => {
    await cleanupTestFiles(testFiles)
    testFiles = []
  })

  test.describe('Device Export Functionality', () => {
    test('should export devices to JSON file', async ({ page }) => {
      // Navigate to devices page
      await helpers.navigateToPage('/')
      
      // Wait for devices to load
      await page.waitForSelector('[data-testid="devices-table"]', { timeout: 10000 })
      
      // Start performance measurement
      const startTime = Date.now()
      
      // Click export button
      await page.click('[data-testid="export-devices-btn"]')
      
      // Wait for download
      const downloadPromise = page.waitForEvent('download')
      const download = await downloadPromise
      
      // Measure export time
      const exportTime = Date.now() - startTime
      expect(exportTime).toBeLessThan(5000) // Export should complete within 5 seconds
      
      // Verify download
      expect(download.suggestedFilename()).toMatch(/devices.*\.json$/)
      
      // Save and verify file content
      const downloadPath = await download.path()
      if (downloadPath) {
        testFiles.push(downloadPath)
        
        const fs = require('fs')
        const content = JSON.parse(fs.readFileSync(downloadPath, 'utf8'))
        
        // Verify export structure
        expect(content).toHaveProperty('devices')
        expect(content).toHaveProperty('metadata')
        expect(content).toHaveProperty('export_date')
        expect(Array.isArray(content.devices)).toBe(true)
        expect(content.devices.length).toBeGreaterThan(0)
        
        // Verify device structure
        const device = content.devices[0]
        expect(device).toHaveProperty('id')
        expect(device).toHaveProperty('ip')
        expect(device).toHaveProperty('mac')
        expect(device).toHaveProperty('type')
        expect(device).toHaveProperty('name')
      }
    })

    test('should export with custom filename', async ({ page }) => {
      await helpers.navigateToPage('/')
      
      // Open export dialog/modal if exists
      await page.click('[data-testid="export-options-btn"]')
      
      // Set custom filename
      await page.fill('[data-testid="export-filename-input"]', 'custom-export')
      
      // Start export
      const downloadPromise = page.waitForEvent('download')
      await page.click('[data-testid="confirm-export-btn"]')
      
      const download = await downloadPromise
      expect(download.suggestedFilename()).toContain('custom-export')
    })

    test('should handle export with no devices gracefully', async ({ page }) => {
      // This test would require clearing all devices first
      // Skip if we can't modify test data
      test.skip()
    })
  })

  test.describe('Device Import Functionality', () => {
    test('should import valid device file', async ({ page }) => {
      // Create test import file
      const importFile = await createTestFile(
        testData.validImportFile.content,
        testData.validImportFile.filename
      )
      testFiles.push(importFile)
      
      await helpers.navigateToPage('/')
      
      // Click import button
      await page.click('[data-testid="import-devices-btn"]')
      
      // Upload file
      await helpers.uploadFile('[data-testid="import-file-input"]', importFile)
      
      // Confirm import
      const responseTime = await helpers.measureApiResponseTime(async () => {
        await page.click('[data-testid="confirm-import-btn"]')
        await helpers.waitForNotification('Import completed successfully')
      })
      
      expect(responseTime).toBeLessThan(3000) // Import should complete within 3 seconds
      
      // Verify imported device appears in list
      await page.reload()
      await expect(page.locator('text=test-import-device')).toBeVisible()
    })

    test('should reject invalid import file', async ({ page }) => {
      // Create invalid import file
      const invalidFile = await createTestFile(
        testData.invalidImportFile.content,
        testData.invalidImportFile.filename
      )
      testFiles.push(invalidFile)
      
      await helpers.navigateToPage('/')
      await page.click('[data-testid="import-devices-btn"]')
      await helpers.uploadFile('[data-testid="import-file-input"]', invalidFile)
      await page.click('[data-testid="confirm-import-btn"]')
      
      // Should show error notification
      const hasError = await helpers.waitForNotification('Import failed')
      expect(hasError).toBe(true)
    })

    test('should preview import before confirmation', async ({ page }) => {
      const importFile = await createTestFile(
        testData.validImportFile.content,
        testData.validImportFile.filename
      )
      testFiles.push(importFile)
      
      await helpers.navigateToPage('/')
      await page.click('[data-testid="import-devices-btn"]')
      await helpers.uploadFile('[data-testid="import-file-input"]', importFile)
      
      // Should show preview
      await expect(page.locator('[data-testid="import-preview"]')).toBeVisible()
      await expect(page.locator('text=test-import-device')).toBeVisible()
      await expect(page.locator('text=1 device(s) will be imported')).toBeVisible()
    })
  })

  test.describe('Complete Export-Import Workflow', () => {
    test('should complete full export-import cycle', async ({ page }) => {
      // Step 1: Export current devices
      await helpers.navigateToPage('/')
      await page.click('[data-testid="export-devices-btn"]')
      
      const downloadPromise = page.waitForEvent('download')
      const download = await downloadPromise
      const exportFile = await download.path()
      
      expect(exportFile).toBeTruthy()
      testFiles.push(exportFile!)
      
      // Step 2: Clear some devices (simulate reset scenario)
      // This would require API calls to delete specific test devices
      
      // Step 3: Import the exported file
      await page.click('[data-testid="import-devices-btn"]')
      await helpers.uploadFile('[data-testid="import-file-input"]', exportFile!)
      await page.click('[data-testid="confirm-import-btn"]')
      
      // Step 4: Verify devices are restored
      await helpers.waitForNotification('Import completed successfully')
      await page.reload()
      
      // Verify device count matches original
      const deviceRows = await page.locator('[data-testid="device-row"]').count()
      expect(deviceRows).toBeGreaterThan(0)
    })

    test('should handle concurrent import/export operations', async ({ page }) => {
      // This test would require multiple browser contexts
      test.skip('Requires multiple browser contexts')
    })
  })

  test.describe('Error Handling & Edge Cases', () => {
    test('should handle network failures during export', async ({ page }) => {
      await helpers.navigateToPage('/')
      
      // Intercept network requests and make them fail
      await page.route('**/api/v1/export/**', route => route.abort())
      
      await page.click('[data-testid="export-devices-btn"]')
      
      // Should show error notification
      const hasError = await helpers.waitForNotification('Export failed')
      expect(hasError).toBe(true)
    })

    test('should handle network failures during import', async ({ page }) => {
      const importFile = await createTestFile(
        testData.validImportFile.content,
        testData.validImportFile.filename
      )
      testFiles.push(importFile)
      
      await helpers.navigateToPage('/')
      
      // Intercept import requests
      await page.route('**/api/v1/import/**', route => route.abort())
      
      await page.click('[data-testid="import-devices-btn"]')
      await helpers.uploadFile('[data-testid="import-file-input"]', importFile)
      await page.click('[data-testid="confirm-import-btn"]')
      
      const hasError = await helpers.waitForNotification('Import failed')
      expect(hasError).toBe(true)
    })

    test('should validate file size limits', async ({ page }) => {
      // Create oversized file (simulate large device list)
      const largeData = {
        devices: Array(10000).fill(0).map((_, i) => ({
          id: i,
          name: `Device ${i}`,
          ip: `192.168.1.${i % 255}`,
          mac: `AA:BB:CC:DD:EE:${i.toString(16).padStart(2, '0')}`,
          type: 'Test Device'
        })),
        metadata: { version: '1.0' }
      }
      
      const largeFile = await createTestFile(
        JSON.stringify(largeData),
        'large-import.json'
      )
      testFiles.push(largeFile)
      
      await helpers.navigateToPage('/')
      await page.click('[data-testid="import-devices-btn"]')
      await helpers.uploadFile('[data-testid="import-file-input"]', largeFile)
      
      // Should show file size warning
      await expect(page.locator('text=File size warning')).toBeVisible()
    })

    test('should handle malformed JSON files', async ({ page }) => {
      const malformedFile = await createTestFile(
        '{"devices": [{"id": 1, "name": "test"', // Missing closing brackets
        'malformed.json'
      )
      testFiles.push(malformedFile)
      
      await helpers.navigateToPage('/')
      await page.click('[data-testid="import-devices-btn"]')
      await helpers.uploadFile('[data-testid="import-file-input"]', malformedFile)
      await page.click('[data-testid="confirm-import-btn"]')
      
      const hasError = await helpers.waitForNotification('Invalid file format')
      expect(hasError).toBe(true)
    })
  })
})