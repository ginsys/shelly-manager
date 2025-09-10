import { Page, expect } from '@playwright/test'

/**
 * Test helper utilities for Shelly Manager E2E tests
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
 * Wait for page to be ready and network idle
 */
export async function waitForPageReady(page: Page, timeout = 30000): Promise<void> {
  await page.waitForLoadState('networkidle', { timeout })
  await page.waitForSelector('main', { timeout })
  
  // Wait for any loading spinners to disappear
  try {
    await page.waitForSelector(SELECTORS.loadingSpinner, { state: 'hidden', timeout: 5000 })
  } catch {
    // Ignore if no spinner found
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
        method: 'DELETE'
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