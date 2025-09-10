import { Page, expect } from '@playwright/test'

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
    const baseUrl = this.page.url().includes('5173') ? 'http://localhost:5173' : 'http://localhost:5174'
    await this.page.goto(`${baseUrl}${path}`)
    await this.page.waitForLoadState('networkidle')
  }

  async measurePageLoadTime(): Promise<number> {
    const startTime = Date.now()
    await this.page.waitForLoadState('networkidle')
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