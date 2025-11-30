import { FullConfig, request } from '@playwright/test'
import * as fs from 'fs'

// Configuration constants for fail-fast behavior
const BACKEND_URL = 'http://localhost:8080'
const MAX_RETRIES = 30          // 30 seconds max wait for backend
const RETRY_INTERVAL_MS = 1000  // Check every second
const REQUEST_TIMEOUT_MS = 5000 // Individual request timeout

async function globalSetup(config: FullConfig) {
  console.log('Starting E2E Test Environment Setup...')

  // Delete existing test database to ensure fresh start
  const testDbPath = '/tmp/shelly_test.db'
  try {
    if (fs.existsSync(testDbPath)) {
      fs.unlinkSync(testDbPath)
      console.log('Deleted existing test database')
    }
  } catch (error) {
    console.warn('Could not delete test database:', error)
  }

  // Create a request context for API calls with explicit timeout
  const requestContext = await request.newContext({
    baseURL: BACKEND_URL,
    extraHTTPHeaders: {
      'User-Agent': 'Playwright-E2E-Test/1.0 (Compatible; Testing)',
    },
    timeout: REQUEST_TIMEOUT_MS,
  })

  try {
    // Wait for backend with retry logic and clear progress
    await waitForBackend(requestContext)

    // Setup test data (backend is ready)
    console.log('Setting up test data...')
    await setupTestData(requestContext)
    console.log('Test data ready')

  } catch (error) {
    console.error('SETUP FAILED:', error instanceof Error ? error.message : error)
    // Re-throw to fail fast - don't let tests run with broken setup
    throw error
  } finally {
    await requestContext.dispose()
  }

  console.log('E2E Test Environment Setup Complete')
}

/**
 * Wait for backend API with retry logic and fail-fast behavior
 */
async function waitForBackend(requestContext: any): Promise<void> {
  console.log(`Waiting for backend API at ${BACKEND_URL}...`)

  const healthEndpoints = ['/healthz', '/api/v1/health', '/ping']

  for (let attempt = 1; attempt <= MAX_RETRIES; attempt++) {
    for (const endpoint of healthEndpoints) {
      try {
        const response = await requestContext.get(endpoint)
        if (response.ok()) {
          console.log(`Backend ready at ${endpoint} (attempt ${attempt}/${MAX_RETRIES})`)
          return
        }
      } catch {
        // Endpoint not ready, continue to next
      }
    }

    // Progress indicator every 5 seconds
    if (attempt % 5 === 0) {
      console.log(`Still waiting for backend... (${attempt}/${MAX_RETRIES}s)`)
    }

    await new Promise(resolve => setTimeout(resolve, RETRY_INTERVAL_MS))
  }

  // Fail fast with clear error message
  const errorMsg = `Backend API not accessible at ${BACKEND_URL} after ${MAX_RETRIES}s.
Make sure the backend is running: go run ./cmd/shelly-manager server`
  throw new Error(errorMsg)
}

async function setupTestData(requestContext: any) {
  // Create test devices for export/import testing
  // Using the format expected by the API (matching database.Device struct)
  const testDevices = [
    {
      ip: '192.168.1.100',
      mac: 'A4:CF:12:34:56:78',
      type: 'Smart Plug',
      name: 'Test Device 1', 
      firmware: '20231219-134356',
      status: 'online',
      settings: '{"model":"SHPLG-S","gen":1,"auth_enabled":true}'
    },
    {
      ip: '192.168.1.101',
      mac: 'A4:CF:12:34:56:79', 
      type: 'Smart Plug',
      name: 'Test Device 2',
      firmware: '20231219-134356',
      status: 'online',
      settings: '{"model":"SHPLG-S","gen":1,"auth_enabled":true}'
    }
  ]
  
  // Add devices via API
  for (const device of testDevices) {
    try {
      const response = await requestContext.post('/api/v1/devices', {
        headers: {
          'Content-Type': 'application/json',
        },
        data: device
      })
      
      if (response.ok()) {
        console.log(`📱 Created test device: ${device.name}`)
      } else {
        console.log(`⚠️ Device creation failed for ${device.name}: ${response.status()} ${response.statusText()}`)
        try {
          const errorBody = await response.text()
          console.log(`⚠️ Error response: ${errorBody}`)
        } catch (e) {
          console.log(`⚠️ Could not read error response: ${e}`)
        }
      }
    } catch (error) {
      console.warn(`⚠️ Could not create test device ${device.name}:`, error)
    }
  }
}

export default globalSetup