/**
 * Test fixture helper for loading mock data during E2E tests
 * Reduces API calls by providing consistent test data
 */
import { Page } from '@playwright/test'
import devicesFixture from './devices.json' with { type: 'json' }
import metricsFixture from './metrics.json' with { type: 'json' }
import pluginsFixture from './plugins.json' with { type: 'json' }
import exportHistoryFixture from './export-history.json' with { type: 'json' }

export interface FixtureOptions {
  /** Mock API calls with fixture data */
  mockAPI?: boolean
  /** Delay for mock responses (ms) */
  responseDelay?: number
  /** Simulate network failures */
  simulateFailures?: boolean
}

/**
 * Set up fixture mocking for a Playwright page
 * Intercepts API calls and returns fixture data instead
 */
export async function setupFixtures(page: Page, options: FixtureOptions = {}) {
  const {
    mockAPI = true,
    responseDelay = 100,
    simulateFailures = false
  } = options

  if (!mockAPI) {
    return
  }

  // Mock devices API
  await page.route('**/api/v1/devices**', async (route) => {
    if (simulateFailures && Math.random() < 0.1) {
      return route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'Simulated server error' })
      })
    }

    // Add response delay to simulate network latency
    if (responseDelay > 0) {
      await new Promise(resolve => setTimeout(resolve, responseDelay))
    }

    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(devicesFixture)
    })
  })

  // Mock metrics API
  await page.route('**/api/v1/metrics**', async (route) => {
    if (simulateFailures && Math.random() < 0.1) {
      return route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'Simulated server error' })
      })
    }

    if (responseDelay > 0) {
      await new Promise(resolve => setTimeout(resolve, responseDelay))
    }

    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(metricsFixture)
    })
  })

  // Mock health check API (frequently called)
  await page.route('**/healthz**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ status: 'ok', mode: 'test' })
    })
  })

  // Mock plugins API
  await page.route('**/api/v1/plugins**', async (route) => {
    if (simulateFailures && Math.random() < 0.1) {
      return route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'Simulated server error' })
      })
    }

    if (responseDelay > 0) {
      await new Promise(resolve => setTimeout(resolve, responseDelay))
    }

    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(pluginsFixture)
    })
  })

  // Mock export history API
  await page.route('**/api/v1/export/history**', async (route) => {
    if (simulateFailures && Math.random() < 0.1) {
      return route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'Simulated server error' })
      })
    }

    if (responseDelay > 0) {
      await new Promise(resolve => setTimeout(resolve, responseDelay))
    }

    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(exportHistoryFixture)
    })
  })

  // Mock WebSocket connections for metrics
  await page.route('**/ws/metrics**', async (route) => {
    // For WebSocket routes, just continue - they need special handling
    await route.continue()
  })
}

/**
 * Create a minimal fixture setup for smoke tests
 * Only mocks essential endpoints to reduce setup time
 */
export async function setupMinimalFixtures(page: Page) {
  await setupFixtures(page, {
    mockAPI: true,
    responseDelay: 50, // Faster response for smoke tests
    simulateFailures: false
  })
}

/**
 * Create a comprehensive fixture setup for integration tests
 * Includes error simulation and realistic delays
 */
export async function setupComprehensiveFixtures(page: Page) {
  await setupFixtures(page, {
    mockAPI: true,
    responseDelay: 200, // More realistic network delay
    simulateFailures: true // Include error scenarios
  })
}

/**
 * Get fixture data directly (for use in test assertions)
 */
export const fixtures = {
  devices: devicesFixture,
  metrics: metricsFixture,
  plugins: pluginsFixture,
  exportHistory: exportHistoryFixture
}

/**
 * Helper to create custom device fixture data
 */
export function createDeviceFixture(overrides: Partial<typeof devicesFixture.devices[0]>) {
  return {
    ...devicesFixture.devices[0],
    ...overrides,
    updated_at: new Date().toISOString()
  }
}

/**
 * Helper to create custom metrics fixture data
 */
export function createMetricsFixture(overrides: Partial<typeof metricsFixture>) {
  return {
    ...metricsFixture,
    ...overrides
  }
}