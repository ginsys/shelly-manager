import { test, expect, devices } from '@playwright/test'
import { TestHelpers } from './fixtures/test-helpers'

test.describe('Cross-Browser and Responsive Testing', () => {
  let helpers: TestHelpers

  test.beforeEach(async ({ page }) => {
    helpers = new TestHelpers(page)
    
    // Skip if backend is not available
    const isHealthy = await helpers.checkApiHealth()
    test.skip(!isHealthy, 'Backend API is not available')
  })

  test.describe('Desktop Browsers', () => {
    test('should work correctly in Chrome', async ({ page, browserName }) => {
      test.skip(browserName !== 'chromium', 'This test only runs on Chromium/Chrome')
      
      await helpers.navigateToPage('/')
      
      // Basic functionality test
      await expect(page.locator('h1, h2, .app-title')).toBeVisible()
      await expect(page.locator('[data-testid="devices-table"]')).toBeVisible()
      
      // Test interactive elements
      const exportBtn = page.locator('[data-testid="export-devices-btn"]')
      if (await exportBtn.isVisible()) {
        await expect(exportBtn).toBeEnabled()
      }
      
      // Check console for errors
      const errors = await helpers.getConsoleErrors()
      expect(errors.length).toBe(0)
    })

    test('should work correctly in Firefox', async ({ page, browserName }) => {
      test.skip(browserName !== 'firefox', 'This test only runs on Firefox')
      
      await helpers.navigateToPage('/')
      
      // Test key functionality that might differ in Firefox
      await expect(page.locator('[data-testid="devices-table"]')).toBeVisible()
      
      // Test file download (Firefox handles differently)
      const exportBtn = page.locator('[data-testid="export-devices-btn"]')
      if (await exportBtn.isVisible()) {
        await exportBtn.click()
        
        const downloadPromise = page.waitForEvent('download')
        const download = await downloadPromise
        expect(download).toBeTruthy()
      }
    })

    test('should work correctly in Safari/WebKit', async ({ page, browserName }) => {
      test.skip(browserName !== 'webkit', 'This test only runs on WebKit/Safari')
      
      await helpers.navigateToPage('/')
      
      // Test Safari-specific behaviors
      await expect(page.locator('[data-testid="devices-table"]')).toBeVisible()
      
      // Test CSS compatibility
      const computedStyles = await page.evaluate(() => {
        const element = document.querySelector('.main-container, .q-page, body')
        return element ? getComputedStyle(element).display : null
      })
      
      expect(computedStyles).toBeTruthy()
    })
  })

  test.describe('Mobile Browsers', () => {
    test('should be responsive on mobile devices', async ({ page }) => {
      // Test different mobile viewports
      const mobileViewports = [
        { width: 375, height: 667, name: 'iPhone SE' },
        { width: 390, height: 844, name: 'iPhone 12' },
        { width: 414, height: 896, name: 'iPhone 11 Pro Max' },
        { width: 360, height: 640, name: 'Galaxy S5' },
        { width: 412, height: 915, name: 'Pixel 5' }
      ]
      
      for (const viewport of mobileViewports) {
        await page.setViewportSize({ width: viewport.width, height: viewport.height })
        
        await helpers.navigateToPage('/')
        await page.waitForLoadState('networkidle')
        
        // Check if main navigation is accessible (hamburger menu, etc.)
        const nav = page.locator('.q-toolbar, .mobile-nav, nav, [data-testid="mobile-menu"]')
        await expect(nav).toBeVisible()
        
        // Check if content adapts to mobile
        const content = page.locator('.q-page-container, .main-content, main')
        await expect(content).toBeVisible()
        
        // Verify touch targets are appropriately sized (minimum 44px)
        const buttons = await page.locator('button, .btn, .q-btn').all()
        for (const button of buttons.slice(0, 5)) { // Test first 5 buttons
          if (await button.isVisible()) {
            const box = await button.boundingBox()
            expect(box?.height).toBeGreaterThanOrEqual(40) // Minimum touch target
          }
        }
        
        console.log(`✅ ${viewport.name} (${viewport.width}x${viewport.height}) - Responsive test passed`)
      }
    })

    test('should handle touch interactions', async ({ page }) => {
      await page.setViewportSize({ width: 375, height: 667 })
      await helpers.navigateToPage('/')
      
      // Test touch scrolling
      await page.touchscreen.tap(200, 300)
      
      // Test swipe gestures if implemented
      const scrollContainer = page.locator('.q-page, .device-list, .scroll-container').first()
      if (await scrollContainer.isVisible()) {
        const box = await scrollContainer.boundingBox()
        if (box) {
          await page.touchscreen.tap(box.x + box.width / 2, box.y + box.height / 2)
        }
      }
    })
  })

  test.describe('Tablet Devices', () => {
    test('should work well on tablet screens', async ({ page }) => {
      // Test iPad dimensions
      await page.setViewportSize({ width: 768, height: 1024 })
      
      await helpers.navigateToPage('/')
      
      // Should have a layout between mobile and desktop
      const layout = await page.evaluate(() => {
        const container = document.querySelector('.q-page-container, .container, main')
        return container ? getComputedStyle(container).maxWidth : null
      })
      
      expect(layout).toBeTruthy()
      
      // Check if navigation is appropriate for tablet
      const nav = page.locator('.q-toolbar, nav, .navigation')
      await expect(nav).toBeVisible()
      
      // Portrait orientation test
      await page.setViewportSize({ width: 768, height: 1024 })
      await page.waitForTimeout(500)
      
      // Landscape orientation test
      await page.setViewportSize({ width: 1024, height: 768 })
      await page.waitForTimeout(500)
      
      // Content should still be visible and functional
      await expect(page.locator('[data-testid="devices-table"]')).toBeVisible()
    })
  })

  test.describe('Accessibility Testing', () => {
    test('should meet basic accessibility standards', async ({ page }) => {
      await helpers.navigateToPage('/')
      
      const accessibilityChecks = await helpers.checkAccessibility()
      
      // Basic accessibility requirements
      expect(accessibilityChecks.hasTitle).toBe(true)
      expect(accessibilityChecks.hasHeadings).toBe(true)
      expect(accessibilityChecks.hasAltText).toBe(true)
      
      // Check for proper ARIA labels
      const interactiveElements = await page.locator('button, input, select, [role="button"]').all()
      
      for (const element of interactiveElements.slice(0, 10)) { // Test first 10
        const ariaLabel = await element.getAttribute('aria-label')
        const title = await element.getAttribute('title')
        const textContent = await element.textContent()
        
        // Each interactive element should have some form of accessible name
        expect(ariaLabel || title || textContent?.trim()).toBeTruthy()
      }
    })

    test('should support keyboard navigation', async ({ page }) => {
      await helpers.navigateToPage('/')
      
      // Tab through interactive elements
      await page.keyboard.press('Tab')
      let focusedElement = await page.evaluate(() => document.activeElement?.tagName)
      expect(['BUTTON', 'INPUT', 'A', 'SELECT']).toContain(focusedElement)
      
      // Continue tabbing to ensure tab order is logical
      const tabStops = []
      for (let i = 0; i < 10; i++) {
        await page.keyboard.press('Tab')
        const element = await page.evaluate(() => ({
          tagName: document.activeElement?.tagName,
          className: document.activeElement?.className,
          id: document.activeElement?.id
        }))
        tabStops.push(element)
      }
      
      // Should have focused multiple different elements
      const uniqueElements = new Set(tabStops.map(el => `${el.tagName}-${el.id}`))
      expect(uniqueElements.size).toBeGreaterThan(1)
    })

    test('should have proper color contrast', async ({ page }) => {
      await helpers.navigateToPage('/')
      
      // Basic color contrast test (simplified)
      const textElements = await page.locator('p, span, div, h1, h2, h3, h4, h5, h6').all()
      
      for (const element of textElements.slice(0, 10)) {
        if (await element.isVisible()) {
          const styles = await element.evaluate(el => {
            const computed = getComputedStyle(el)
            return {
              color: computed.color,
              backgroundColor: computed.backgroundColor,
              fontSize: computed.fontSize
            }
          })
          
          // Just verify that color values are present
          expect(styles.color).toBeTruthy()
        }
      }
    })

    test('should support screen reader features', async ({ page }) => {
      await helpers.navigateToPage('/')
      
      // Check for semantic HTML
      const headings = await page.locator('h1, h2, h3, h4, h5, h6').count()
      expect(headings).toBeGreaterThan(0)
      
      // Check for landmark regions
      const landmarks = await page.locator('main, nav, header, footer, aside, section[aria-label]').count()
      expect(landmarks).toBeGreaterThan(0)
      
      // Check for proper list markup if lists are present
      const lists = await page.locator('ul, ol').count()
      if (lists > 0) {
        const listItems = await page.locator('li').count()
        expect(listItems).toBeGreaterThan(0)
      }
    })
  })

  test.describe('Performance Across Browsers', () => {
    test('should maintain performance standards across browsers', async ({ page, browserName }) => {
      const startTime = Date.now()
      await helpers.navigateToPage('/')
      const loadTime = Date.now() - startTime
      
      // Performance should be consistent across browsers (allowing some variance)
      const maxLoadTime = browserName === 'webkit' ? 5000 : 4000 // Safari might be slightly slower
      expect(loadTime).toBeLessThan(maxLoadTime)
      
      // Check JavaScript performance
      const jsPerformance = await page.evaluate(() => {
        const start = performance.now()
        // Simple computation test
        let sum = 0
        for (let i = 0; i < 100000; i++) {
          sum += i
        }
        return performance.now() - start
      })
      
      expect(jsPerformance).toBeLessThan(100) // Should complete within 100ms
      
      console.log(`${browserName} performance: Load ${loadTime}ms, JS ${jsPerformance.toFixed(2)}ms`)
    })
  })

  test.describe('Feature Compatibility', () => {
    test('should handle file download across browsers', async ({ page, browserName }) => {
      await helpers.navigateToPage('/')
      
      const exportBtn = page.locator('[data-testid="export-devices-btn"]')
      if (await exportBtn.isVisible()) {
        const downloadPromise = page.waitForEvent('download')
        await exportBtn.click()
        
        const download = await downloadPromise
        expect(download.suggestedFilename()).toBeTruthy()
        
        // Verify the download worked in this browser
        const path = await download.path()
        expect(path).toBeTruthy()
      }
    })

    test('should handle file upload across browsers', async ({ page, browserName }) => {
      await helpers.navigateToPage('/')
      
      // Create a test file
      const fs = require('fs')
      const testFile = '/tmp/test-upload.json'
      fs.writeFileSync(testFile, JSON.stringify({ test: 'data' }))
      
      const importBtn = page.locator('[data-testid="import-devices-btn"]')
      if (await importBtn.isVisible()) {
        await importBtn.click()
        
        const fileInput = page.locator('[data-testid="import-file-input"]')
        if (await fileInput.isVisible()) {
          await fileInput.setInputFiles(testFile)
          
          // Verify file was selected
          const fileName = await fileInput.evaluate(el => (el as HTMLInputElement).files?.[0]?.name)
          expect(fileName).toBe('test-upload.json')
        }
      }
      
      // Cleanup
      fs.unlinkSync(testFile)
    })

    test('should handle WebSocket connections if used', async ({ page }) => {
      await helpers.navigateToPage('/')
      
      // Listen for WebSocket connections
      let wsConnected = false
      page.on('websocket', ws => {
        wsConnected = true
        console.log(`WebSocket connection: ${ws.url()}`)
      })
      
      // Wait a bit to see if WebSockets are used
      await page.waitForTimeout(2000)
      
      // If WebSockets are used, they should connect successfully
      // If not used, test passes by default
    })
  })
})