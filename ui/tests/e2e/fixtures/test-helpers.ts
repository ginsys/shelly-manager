import { Page, expect } from '@playwright/test'
import { setupFixtures, setupMinimalFixtures, setupComprehensiveFixtures } from '../../fixtures/fixture-helper.js'

/**
 * Test helper utilities for Shelly Manager E2E tests
 * Enhanced with fixture support for reduced API calls
 */

// Common selectors
export const SELECTORS = {
  // Navigation
  navDrawer: '[data-testid="nav-drawer"]',
  navItem: (name: string) => `[data-testid="nav-${name}"]`,
  
  // Common UI elements
  loadingSpinner: '.q-spinner',
  errorDialog: '[data-testid="error-dialog"]',
  confirmDialog: '[data-testid="confirm-dialog"]',
  
  // Forms
  submitButton: '[data-testid="submit-button"]',
  cancelButton: '[data-testid="cancel-button"]',
  
  // Tables
  table: '.q-table',
  tableRow: '.q-tr',
  tableCell: '.q-td',
  
  // Device related
  deviceCard: '[data-testid="device-card"]',
  deviceStatus: '[data-testid="device-status"]',
  
  // Plugin related
  pluginCard: '[data-testid="plugin-card"]',
  pluginConfigForm: '[data-testid="plugin-config-form"]',
  
  // Schedule related
  scheduleForm: '[data-testid="schedule-form"]',
  scheduleItem: '[data-testid="schedule-item"]',
  
  // Backup related
  backupForm: '[data-testid="backup-form"]',
  backupItem: '[data-testid="backup-item"]',
  
  // GitOps related
  gitopsForm: '[data-testid="gitops-form"]',
  gitopsConfig: '[data-testid="gitops-config"]',
} as const

// Test data
export const TEST_DATA = {
  devices: [
    {
      id: 'test-device-1',
      name: 'Test Shelly 1',
      type: 'SHSW-1',
      ip: '192.168.1.100',
      generation: 1,
      firmware_version: '20230109-114426/v1.12.1-ga9117d3',
      mac: 'AA:BB:CC:DD:EE:01'
    },
    {
      id: 'test-device-2', 
      name: 'Test Shelly Plus 1',
      type: 'SNSW-001X16EU',
      ip: '192.168.1.101',
      generation: 2,
      firmware_version: '0.12.0-beta1',
      mac: 'AA:BB:CC:DD:EE:02'
    }
  ],
  schedules: [
    {
      name: 'Daily Export',
      plugin: 'home-assistant',
      frequency: 'daily',
      time: '02:00'
    },
    {
      name: 'Weekly Backup',
      plugin: 'backup',
      frequency: 'weekly',
      time: '03:00'
    }
  ],
  backups: [
    {
      name: 'Test Backup',
      type: 'full',
      schedule: 'daily'
    }
  ]
} as const

/**
 * Set up test environment with fixtures (for smoke tests)
 * Uses minimal fixtures to reduce setup time
 */
export async function setupTestEnvironment(page: Page, useFixtures = true): Promise<void> {
  if (useFixtures) {
    await setupMinimalFixtures(page)
  }
}

/**
 * Set up comprehensive test environment with fixtures (for integration tests)
 * Uses full fixtures with error simulation
 */
export async function setupComprehensiveTestEnvironment(page: Page, useFixtures = true): Promise<void> {
  if (useFixtures) {
    await setupComprehensiveFixtures(page)
  }
}

/**
 * Wait for page to be ready - simplified, deterministic approach
 * Avoids race conditions by using explicit element waits
 */
export async function waitForPageReady(page: Page, timeout = 10000): Promise<void> {
  // Step 1: Wait for DOM to be ready
  await page.waitForLoadState('domcontentloaded', { timeout: timeout / 2 })

  // Step 2: Wait for Vue app to mount - use specific selector
  const appSelector = '#app, [data-testid="app"], .q-layout'
  await page.locator(appSelector).first().waitFor({
    state: 'attached',
    timeout: timeout / 2
  })

  // Step 3: Wait for loading spinners to disappear (if any exist)
  const spinner = page.locator('.q-spinner, .loading, [data-loading="true"]')
  try {
    // Only wait if spinners are visible
    if (await spinner.count() > 0) {
      await spinner.first().waitFor({ state: 'hidden', timeout: timeout / 2 })
    }
  } catch {
    // Spinners may not exist or may have already disappeared
  }

  // Step 4: Wait for any page content to be visible
  // This confirms the page has actually rendered something useful
  const contentSelector = [
    'h1', 'h2',
    '[data-testid="page-title"]',
    '[data-testid="device-list"]',
    '[data-testid="plugin-list"]',
    '[data-testid="empty-state"]',
    '.q-table',
    '.q-card'
  ].join(', ')

  try {
    await page.locator(contentSelector).first().waitFor({
      state: 'visible',
      timeout: timeout / 2
    })
  } catch {
    // Page may not have expected content yet, continue anyway
    console.warn('Page content not detected within timeout, continuing...')
  }
}

/**
 * Navigate to a specific page and wait for it to load
 */
export async function navigateToPage(page: Page, pageName: string): Promise<void> {
  await page.click(SELECTORS.navItem(pageName))
  await waitForPageReady(page)
}

/**
 * Wait for API response and verify success
 */
export async function waitForApiResponse(page: Page, url: string, timeout = 10000): Promise<void> {
  const responsePromise = page.waitForResponse(response => 
    response.url().includes(url) && response.status() === 200, 
    { timeout }
  )
  await responsePromise
}

/**
 * Fill form field with data validation
 */
export async function fillFormField(page: Page, selector: string, value: string): Promise<void> {
  await page.fill(selector, value)
  await expect(page.locator(selector)).toHaveValue(value)
}

/**
 * Submit form and wait for response
 */
export async function submitForm(page: Page, apiEndpoint?: string): Promise<void> {
  const responsePromise = apiEndpoint 
    ? page.waitForResponse(response => response.url().includes(apiEndpoint))
    : null
    
  await page.click(SELECTORS.submitButton)
  
  if (responsePromise) {
    await responsePromise
  }
  
  await waitForPageReady(page)
}

/**
 * Handle error dialogs if they appear
 */
export async function dismissErrorDialog(page: Page): Promise<void> {
  try {
    const errorDialog = page.locator(SELECTORS.errorDialog)
    if (await errorDialog.isVisible()) {
      await page.click('[data-testid="error-dialog-close"]')
    }
  } catch {
    // Ignore if no error dialog
  }
}

/**
 * Create test device via API
 */
export async function createTestDevice(page: Page, device: typeof TEST_DATA.devices[0]): Promise<boolean> {
  try {
    const response = await page.evaluate(async (deviceData) => {
      const res = await fetch('http://localhost:8080/api/v1/devices', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'User-Agent': 'Playwright-E2E-Test/1.0 (Compatible; Testing)',
        },
        body: JSON.stringify(deviceData)
      })
      return { ok: res.ok, status: res.status, data: await res.json() }
    }, device)
    
    return response.ok
  } catch (error) {
    console.warn(`Could not create test device ${device.name}:`, error)
    return false
  }
}

/**
 * Delete test device via API
 */
export async function deleteTestDevice(page: Page, deviceId: string): Promise<boolean> {
  try {
    const response = await page.evaluate(async (id) => {
      const res = await fetch(`http://localhost:8080/api/v1/devices/${id}`, {
        method: 'DELETE',
        headers: {
          'User-Agent': 'Playwright-E2E-Test/1.0 (Compatible; Testing)',
        }
      })
      return { ok: res.ok, status: res.status }
    }, deviceId)
    
    return response.ok
  } catch (error) {
    console.warn(`Could not delete test device ${deviceId}:`, error)
    return false
  }
}

/**
 * Verify API response structure
 */
export async function verifyApiResponse(response: any, expectedKeys: string[]): Promise<void> {
  expect(response).toHaveProperty('success')
  expect(response.success).toBe(true)
  
  if (expectedKeys.length > 0 && response.data) {
    for (const key of expectedKeys) {
      expect(response.data).toHaveProperty(key)
    }
  }
}

/**
 * Mock API responses for testing
 */
export async function mockApiResponse(page: Page, endpoint: string, responseData: any): Promise<void> {
  await page.route(`**/api/v1/${endpoint}`, route => {
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        success: true,
        data: responseData
      })
    })
  })
}

/**
 * Test viewport responsiveness
 */
export async function testResponsiveDesign(page: Page, callback: () => Promise<void>): Promise<void> {
  const viewports = [
    { width: 1920, height: 1080 }, // Desktop
    { width: 768, height: 1024 },  // Tablet
    { width: 375, height: 667 }    // Mobile
  ]
  
  for (const viewport of viewports) {
    await page.setViewportSize(viewport)
    await callback()
  }
}

/**
 * Check accessibility basics
 */
export async function checkAccessibility(page: Page): Promise<void> {
  // Check for proper headings
  const headings = await page.locator('h1, h2, h3, h4, h5, h6').count()
  expect(headings).toBeGreaterThan(0)
  
  // Check for alt text on images
  const images = page.locator('img')
  const imageCount = await images.count()
  
  for (let i = 0; i < imageCount; i++) {
    const img = images.nth(i)
    await expect(img).toHaveAttribute('alt')
  }
  
  // Check for proper form labels
  const inputs = page.locator('input')
  const inputCount = await inputs.count()
  
  for (let i = 0; i < inputCount; i++) {
    const input = inputs.nth(i)
    const id = await input.getAttribute('id')
    if (id) {
      await expect(page.locator(`label[for="${id}"]`)).toBeVisible()
    }
  }
}

// ===== CONSOLIDATED EXPORTS FROM helpers/test-helpers.ts =====

export class TestHelpers {
  constructor(private page: Page) {}

  async waitForApiResponse(endpoint: string, timeout = 10000): Promise<boolean> {
    try {
      const response = await this.page.waitForResponse(
        response => response.url().includes(endpoint) && response.status() === 200,
        { timeout }
      )
      return response.ok()
    } catch {
      return false
    }
  }

  async checkApiHealth(): Promise<boolean> {
    try {
      const response = await this.page.request.get('http://localhost:8080/api/v1/devices', {
        headers: {
          'User-Agent': 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36'
        }
      })
      return response.ok()
    } catch {
      return false
    }
  }

  async navigateToPage(path: string): Promise<void> {
    const baseUrl = 'http://localhost:5173' // Use consistent port
    await this.page.goto(`${baseUrl}${path}`)
    await this.page.waitForLoadState('networkidle', { timeout: 15000 })
  }

  async measurePageLoadTime(): Promise<number> {
    const startTime = Date.now()
    await this.page.waitForLoadState('networkidle', { timeout: 10000 })
    return Date.now() - startTime
  }

  async measureApiResponseTime(apiCall: () => Promise<any>): Promise<number> {
    const startTime = Date.now()
    await apiCall()
    return Date.now() - startTime
  }

  async downloadFile(): Promise<string | null> {
    const downloadPromise = this.page.waitForEvent('download')
    // Trigger download action
    const download = await downloadPromise
    const path = await download.path()
    return path
  }

  async uploadFile(selector: string, filePath: string): Promise<void> {
    await this.page.setInputFiles(selector, filePath)
  }

  async waitForNotification(message: string, timeout = 5000): Promise<boolean> {
    try {
      await this.page.locator('.q-notification').filter({ hasText: message }).waitFor({ timeout })
      return true
    } catch {
      return false
    }
  }

  async getConsoleErrors(): Promise<string[]> {
    const errors: string[] = []
    this.page.on('console', msg => {
      if (msg.type() === 'error') {
        errors.push(msg.text())
      }
    })
    return errors
  }

  async checkAccessibility(): Promise<any> {
    // Basic accessibility checks
    const checks = {
      hasTitle: await this.page.title().then(title => title.length > 0),
      hasHeadings: await this.page.locator('h1, h2, h3, h4, h5, h6').count() > 0,
      hasAltText: await this.page.locator('img:not([alt])').count() === 0,
      hasFocusIndicators: await this.page.locator('button, a, input').first().isVisible(),
    }
    return checks
  }

  async capturePerformanceMetrics(): Promise<any> {
    const metrics = await this.page.evaluate(() => ({
      dom: performance.getEntriesByType('navigation')[0],
      resources: performance.getEntriesByType('resource').length,
      memory: (performance as any).memory ? {
        usedJSHeapSize: (performance as any).memory.usedJSHeapSize,
        totalJSHeapSize: (performance as any).memory.totalJSHeapSize,
        jsHeapSizeLimit: (performance as any).memory.jsHeapSizeLimit,
      } : null
    }))
    return metrics
  }
}

export const testData = {
  validDeviceExport: {
    filename: 'test-devices-export.json',
    expectedDevices: 17, // Based on current test data
    expectedStructure: ['devices', 'metadata', 'export_date']
  },
  
  invalidImportFile: {
    filename: 'invalid-import.json',
    content: '{"invalid": "data"}'
  },
  
  validImportFile: {
    filename: 'valid-import.json',
    content: JSON.stringify({
      devices: [
        {
          id: 99,
          ip: "192.168.1.200",
          mac: "AA:BB:CC:DD:EE:FF",
          type: "Test Device",
          name: "test-import-device",
          firmware: "test-version",
          status: "offline",
          settings: JSON.stringify({ model: "TEST", gen: 1 })
        }
      ],
      metadata: {
        version: "1.0",
        exported_by: "shelly-manager",
        export_date: new Date().toISOString()
      }
    })
  }
}

export async function createTestFile(content: string, filename: string): Promise<string> {
  const fs = require('fs')
  const path = require('path')
  const tmpDir = '/tmp'
  const filePath = path.join(tmpDir, filename)
  fs.writeFileSync(filePath, content)
  return filePath
}

export async function cleanupTestFiles(files: string[]): Promise<void> {
  const fs = require('fs')
  files.forEach(file => {
    try {
      fs.unlinkSync(file)
    } catch (error) {
      console.warn(`Could not cleanup test file ${file}:`, error)
    }
  })
}