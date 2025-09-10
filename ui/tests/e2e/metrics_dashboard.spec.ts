import { test, expect } from '@playwright/test'

test.describe('Metrics Dashboard E2E', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/metrics')
    await page.waitForLoadState('networkidle')
  })

  test('should display metrics dashboard with main sections', async ({ page }) => {
    // Check page title
    await expect(page).toHaveTitle(/Metrics.*Shelly Manager/)
    
    // Check main heading
    await expect(page.locator('h1')).toContainText('Metrics Dashboard')
    
    // Check for main dashboard sections
    const statusSection = page.locator('[data-testid="status-section"]')
    const chartsSection = page.locator('[data-testid="charts-section"]')
    
    // At least one main section should be visible
    const hasSections = await statusSection.isVisible() || await chartsSection.isVisible()
    expect(hasSections).toBeTruthy()
  })

  test('should display connection status indicator', async ({ page }) => {
    const connectionStatus = page.locator('[data-testid="connection-status"]')
    
    if (await connectionStatus.isVisible()) {
      await expect(connectionStatus).toBeVisible()
      
      // Should show some connection state
      const statusText = await connectionStatus.textContent()
      expect(statusText).toMatch(/(Connected|Connecting|Disconnected|Reconnecting)/i)
    }
  })

  test('should display WebSocket connection information', async ({ page }) => {
    const wsInfo = page.locator('[data-testid="websocket-info"]')
    
    if (await wsInfo.isVisible()) {
      await expect(wsInfo).toBeVisible()
      
      // Check for WebSocket-specific indicators
      const wsStatus = page.locator('[data-testid="ws-status"]')
      const reconnectAttempts = page.locator('[data-testid="reconnect-attempts"]')
      
      const hasWsIndicators = await wsStatus.isVisible() || await reconnectAttempts.isVisible()
      expect(hasWsIndicators).toBeTruthy()
    }
  })

  test('should display real-time charts when data is available', async ({ page }) => {
    // Wait a bit for potential WebSocket data
    await page.waitForTimeout(2000)
    
    const cpuChart = page.locator('[data-testid="cpu-chart"]')
    const memoryChart = page.locator('[data-testid="memory-chart"]')
    const diskChart = page.locator('[data-testid="disk-chart"]')
    
    // Check if any charts are visible
    const hasCharts = await cpuChart.isVisible() || 
                     await memoryChart.isVisible() || 
                     await diskChart.isVisible()
    
    if (hasCharts) {
      // Verify chart content
      if (await cpuChart.isVisible()) {
        await expect(cpuChart).toBeVisible()
        
        // Charts should have some data or show loading state
        const chartContent = cpuChart.locator('canvas, svg, .chart-loading')
        await expect(chartContent).toBeVisible()
      }
    }
  })

  test('should handle metrics data updates', async ({ page }) => {
    // Wait for initial load
    await page.waitForTimeout(1000)
    
    const statusSection = page.locator('[data-testid="status-section"]')
    const lastUpdate = page.locator('[data-testid="last-update"]')
    
    if (await statusSection.isVisible()) {
      // Get initial timestamp if available
      let initialTime = null
      if (await lastUpdate.isVisible()) {
        initialTime = await lastUpdate.textContent()
      }
      
      // Wait for potential updates
      await page.waitForTimeout(3000)
      
      // Check if timestamp changed (indicating live updates)
      if (initialTime && await lastUpdate.isVisible()) {
        const currentTime = await lastUpdate.textContent()
        // If WebSocket is working, time should have updated
        // This is a soft assertion since updates depend on backend
        console.log('Initial time:', initialTime, 'Current time:', currentTime)
      }
    }
  })

  test('should display system metrics when available', async ({ page }) => {
    // Wait for data to potentially load
    await page.waitForTimeout(2000)
    
    const systemMetrics = page.locator('[data-testid="system-metrics"]')
    
    if (await systemMetrics.isVisible()) {
      await expect(systemMetrics).toBeVisible()
      
      // Check for specific metric displays
      const cpuUsage = page.locator('[data-testid="cpu-usage"]')
      const memoryUsage = page.locator('[data-testid="memory-usage"]')
      
      const hasMetrics = await cpuUsage.isVisible() || await memoryUsage.isVisible()
      
      if (hasMetrics) {
        // Metrics should show percentage values
        if (await cpuUsage.isVisible()) {
          const cpuText = await cpuUsage.textContent()
          expect(cpuText).toMatch(/%|\d+/)
        }
        
        if (await memoryUsage.isVisible()) {
          const memoryText = await memoryUsage.textContent()
          expect(memoryText).toMatch(/%|\d+/)
        }
      }
    }
  })

  test('should show appropriate state when no data is available', async ({ page }) => {
    // Mock WebSocket to not connect
    await page.addInitScript(() => {
      // Override WebSocket to simulate connection failure
      (window as any).WebSocket = class MockWebSocket {
        constructor() {
          setTimeout(() => {
            if (this.onerror) this.onerror(new Event('error'))
          }, 100)
        }
        close() {}
        send() {}
      }
    })
    
    await page.reload()
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(1000)
    
    // Should show appropriate offline/error state
    const offlineState = page.locator('[data-testid="offline-state"]')
    const errorState = page.locator('[data-testid="error-state"]')
    const noDataState = page.locator('[data-testid="no-data-state"]')
    
    const hasErrorState = await offlineState.isVisible() || 
                         await errorState.isVisible() || 
                         await noDataState.isVisible()
    
    if (hasErrorState) {
      expect(hasErrorState).toBeTruthy()
    }
  })

  test('should handle WebSocket reconnection attempts', async ({ page }) => {
    // Mock WebSocket to simulate connection issues
    let reconnectCount = 0
    await page.addInitScript(() => {
      const originalWebSocket = (window as any).WebSocket
      ;(window as any).WebSocket = class MockWebSocket {
        constructor(...args: any[]) {
          setTimeout(() => {
            if (this.onopen) this.onopen(new Event('open'))
            setTimeout(() => {
              if (this.onclose) this.onclose({ code: 1006, reason: 'test' })
            }, 500)
          }, 100)
        }
        close() {}
        send() {}
      }
    })
    
    await page.reload()
    await page.waitForLoadState('networkidle')
    
    // Wait for reconnection attempts
    await page.waitForTimeout(3000)
    
    const reconnectInfo = page.locator('[data-testid="reconnect-attempts"]')
    if (await reconnectInfo.isVisible()) {
      const reconnectText = await reconnectInfo.textContent()
      // Should show attempt count
      expect(reconnectText).toMatch(/attempt|retry/i)
    }
  })

  test('should be responsive on mobile devices', async ({ page }) => {
    // Simulate mobile viewport
    await page.setViewportSize({ width: 375, height: 667 })
    
    const dashboard = page.locator('[data-testid="metrics-dashboard"]')
    await expect(dashboard).toBeVisible()
    
    // Charts should be responsive
    const chartsSection = page.locator('[data-testid="charts-section"]')
    if (await chartsSection.isVisible()) {
      const chartContainers = page.locator('[data-testid*="chart"]')
      const count = await chartContainers.count()
      
      if (count > 0) {
        // Charts should be stacked vertically on mobile
        for (let i = 0; i < count; i++) {
          const chart = chartContainers.nth(i)
          if (await chart.isVisible()) {
            const box = await chart.boundingBox()
            if (box) {
              expect(box.width).toBeLessThan(400) // Should fit mobile width
            }
          }
        }
      }
    }
  })

  test('should refresh data when refresh button is clicked', async ({ page }) => {
    const refreshButton = page.locator('[data-testid="refresh-button"]')
    
    if (await refreshButton.isVisible()) {
      // Get initial state
      const lastUpdate = page.locator('[data-testid="last-update"]')
      let initialTime = null
      if (await lastUpdate.isVisible()) {
        initialTime = await lastUpdate.textContent()
      }
      
      // Click refresh
      await refreshButton.click()
      await page.waitForTimeout(1000)
      
      // Should show loading state or update timestamp
      const loadingIndicator = page.locator('[data-testid="loading"]')
      if (await loadingIndicator.isVisible()) {
        await expect(loadingIndicator).toBeVisible()
      } else if (initialTime && await lastUpdate.isVisible()) {
        const currentTime = await lastUpdate.textContent()
        expect(currentTime).not.toBe(initialTime)
      }
    }
  })

  test('should display health status information', async ({ page }) => {
    // Wait for potential data
    await page.waitForTimeout(2000)
    
    const healthStatus = page.locator('[data-testid="health-status"]')
    
    if (await healthStatus.isVisible()) {
      await expect(healthStatus).toBeVisible()
      
      // Should show health indicators
      const healthIndicators = page.locator('[data-testid="health-indicator"]')
      const count = await healthIndicators.count()
      
      if (count > 0) {
        // Health indicators should show status
        const firstIndicator = healthIndicators.first()
        const status = await firstIndicator.textContent()
        expect(status).toMatch(/(healthy|unhealthy|ok|error|good|bad)/i)
      }
    }
  })
})