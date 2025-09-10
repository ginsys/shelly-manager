import { test, expect } from '@playwright/test'
import { TestHelpers } from './fixtures/test-helpers'

test.describe('Performance Testing', () => {
  let helpers: TestHelpers

  test.beforeEach(async ({ page }) => {
    helpers = new TestHelpers(page)
    
    // Skip if backend is not available
    const isHealthy = await helpers.checkApiHealth()
    test.skip(!isHealthy, 'Backend API is not available')
  })

  test.describe('Page Load Performance', () => {
    test('should load main dashboard within performance budget', async ({ page }) => {
      const startTime = Date.now()
      
      await helpers.navigateToPage('/')
      
      const loadTime = Date.now() - startTime
      
      // Performance target: Main page should load within 3 seconds
      expect(loadTime).toBeLessThan(3000)
      
      // Check for performance metrics
      const metrics = await helpers.capturePerformanceMetrics()
      
      if (metrics.dom) {
        const navigation = metrics.dom as PerformanceNavigationTiming
        expect(navigation.loadEventEnd - navigation.navigationStart).toBeLessThan(3000)
        expect(navigation.domContentLoadedEventEnd - navigation.navigationStart).toBeLessThan(2000)
      }
      
      // Memory usage check (if available)
      if (metrics.memory) {
        const memoryMB = metrics.memory.usedJSHeapSize / (1024 * 1024)
        expect(memoryMB).toBeLessThan(50) // Should use less than 50MB
      }
      
      console.log(`Page load metrics:`, {
        totalLoadTime: loadTime,
        resourceCount: metrics.resources,
        memoryUsage: metrics.memory ? `${(metrics.memory.usedJSHeapSize / (1024 * 1024)).toFixed(2)}MB` : 'N/A'
      })
    })

    test('should load devices page quickly', async ({ page }) => {
      const loadTime = await helpers.measurePageLoadTime()
      await helpers.navigateToPage('/devices')
      
      // Devices page should load within 2 seconds
      expect(loadTime).toBeLessThan(2000)
      
      // Wait for device table to render
      await page.waitForSelector('[data-testid="devices-table"]', { timeout: 5000 })
      
      // Check if all devices are rendered
      const deviceCount = await page.locator('[data-testid="device-row"]').count()
      expect(deviceCount).toBeGreaterThan(0)
    })

    test('should handle large device lists efficiently', async ({ page }) => {
      await helpers.navigateToPage('/devices')
      
      // Measure rendering time for device table
      const startTime = Date.now()
      await page.waitForSelector('[data-testid="devices-table"]')
      const renderTime = Date.now() - startTime
      
      // Should render device list within 1 second regardless of count
      expect(renderTime).toBeLessThan(1000)
      
      // Check for virtual scrolling or pagination if many devices
      const deviceRows = await page.locator('[data-testid="device-row"]').count()
      if (deviceRows > 50) {
        // Should have pagination or virtual scrolling for large lists
        const hasPagination = await page.locator('.q-pagination').isVisible()
        const hasVirtualScroll = await page.locator('[data-testid="virtual-scroll"]').isVisible()
        expect(hasPagination || hasVirtualScroll).toBe(true)
      }
    })
  })

  test.describe('API Performance', () => {
    test('should have fast API response times', async ({ page }) => {
      // Test critical API endpoints
      const endpoints = [
        '/api/v1/devices',
        '/api/v1/status',
        '/api/v1/export/devices',
      ]
      
      for (const endpoint of endpoints) {
        const startTime = Date.now()
        
        const response = await page.request.get(`http://localhost:8080${endpoint}`, {
          headers: {
            'User-Agent': 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36'
          }
        })
        
        const responseTime = Date.now() - startTime
        
        expect(response.status()).toBe(200)
        expect(responseTime).toBeLessThan(200) // API should respond within 200ms
        
        console.log(`${endpoint}: ${responseTime}ms`)
      }
    })

    test('should handle concurrent API requests', async ({ page }) => {
      // Make multiple concurrent requests
      const concurrentRequests = 10
      const promises = Array(concurrentRequests).fill(0).map(() =>
        helpers.measureApiResponseTime(async () => {
          const response = await page.request.get('http://localhost:8080/api/v1/devices', {
            headers: {
              'User-Agent': 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36'
            }
          })
          expect(response.status()).toBe(200)
        })
      )
      
      const responseTimes = await Promise.all(promises)
      const avgResponseTime = responseTimes.reduce((a, b) => a + b, 0) / responseTimes.length
      const maxResponseTime = Math.max(...responseTimes)
      
      // Average response time should remain reasonable under load
      expect(avgResponseTime).toBeLessThan(500)
      expect(maxResponseTime).toBeLessThan(1000)
      
      console.log(`Concurrent API performance:`, {
        averageTime: `${avgResponseTime.toFixed(2)}ms`,
        maxTime: `${maxResponseTime}ms`,
        requests: concurrentRequests
      })
    })

    test('should export large device lists efficiently', async ({ page }) => {
      await helpers.navigateToPage('/')
      
      // Measure export time
      const startTime = Date.now()
      
      const downloadPromise = page.waitForEvent('download')
      await page.click('[data-testid="export-devices-btn"]')
      const download = await downloadPromise
      
      const exportTime = Date.now() - startTime
      
      // Export should complete within 5 seconds even for large lists
      expect(exportTime).toBeLessThan(5000)
      
      // Verify file was created
      const downloadPath = await download.path()
      expect(downloadPath).toBeTruthy()
      
      if (downloadPath) {
        const fs = require('fs')
        const stats = fs.statSync(downloadPath)
        const fileSizeMB = stats.size / (1024 * 1024)
        
        // File size should be reasonable (less than 10MB for typical device counts)
        expect(fileSizeMB).toBeLessThan(10)
        
        console.log(`Export performance:`, {
          exportTime: `${exportTime}ms`,
          fileSize: `${fileSizeMB.toFixed(2)}MB`
        })
      }
    })
  })

  test.describe('Memory and Resource Usage', () => {
    test('should not have memory leaks during navigation', async ({ page }) => {
      // Navigate between pages multiple times
      const pages = ['/', '/devices', '/settings', '/']
      
      for (let i = 0; i < 3; i++) {
        for (const pagePath of pages) {
          await helpers.navigateToPage(pagePath)
          await page.waitForTimeout(1000) // Allow time for cleanup
        }
      }
      
      // Check final memory usage
      const metrics = await helpers.capturePerformanceMetrics()
      
      if (metrics.memory) {
        const memoryMB = metrics.memory.usedJSHeapSize / (1024 * 1024)
        expect(memoryMB).toBeLessThan(100) // Should not exceed 100MB after navigation
      }
    })

    test('should handle export/import operations without memory issues', async ({ page }) => {
      await helpers.navigateToPage('/')
      
      // Perform multiple export operations
      for (let i = 0; i < 5; i++) {
        const downloadPromise = page.waitForEvent('download')
        await page.click('[data-testid="export-devices-btn"]')
        await downloadPromise
        
        await page.waitForTimeout(500) // Allow cleanup
      }
      
      // Check memory usage
      const metrics = await helpers.capturePerformanceMetrics()
      
      if (metrics.memory) {
        const memoryMB = metrics.memory.usedJSHeapSize / (1024 * 1024)
        expect(memoryMB).toBeLessThan(75) // Should not accumulate too much memory
      }
    })
  })

  test.describe('Network Performance', () => {
    test('should optimize network requests', async ({ page }) => {
      // Monitor network requests
      const networkRequests: any[] = []
      
      page.on('request', request => {
        networkRequests.push({
          url: request.url(),
          method: request.method(),
          size: request.postData()?.length || 0
        })
      })
      
      await helpers.navigateToPage('/')
      await page.waitForLoadState('networkidle')
      
      // Analyze network requests
      const apiRequests = networkRequests.filter(req => req.url.includes('/api/'))
      const duplicateRequests = apiRequests.filter((req, index, arr) => 
        arr.findIndex(r => r.url === req.url && r.method === req.method) !== index
      )
      
      // Should not have unnecessary duplicate API calls
      expect(duplicateRequests.length).toBeLessThan(3)
      
      console.log(`Network analysis:`, {
        totalRequests: networkRequests.length,
        apiRequests: apiRequests.length,
        duplicateRequests: duplicateRequests.length
      })
    })

    test('should handle slow network conditions', async ({ page }) => {
      // Simulate slow network
      await page.route('**/api/v1/**', async (route) => {
        await page.waitForTimeout(1000) // Simulate 1s delay
        route.continue()
      })
      
      await helpers.navigateToPage('/')
      
      // Should show loading indicators
      const hasLoading = await page.locator('.q-loading, .loading, .spinner').isVisible()
      expect(hasLoading).toBe(true)
      
      // Should eventually load even with slow network
      await page.waitForSelector('[data-testid="devices-table"]', { timeout: 15000 })
    })
  })
})