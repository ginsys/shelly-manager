import { FullConfig, request } from '@playwright/test'
import * as fs from 'fs'

const BACKEND_URL = 'http://localhost:8080'
const REQUEST_TIMEOUT_MS = 5000

async function globalTeardown(config: FullConfig) {
  console.log('Starting E2E Test Environment Teardown...')

  // Create a request context for API calls with explicit timeout
  const requestContext = await request.newContext({
    baseURL: BACKEND_URL,
    extraHTTPHeaders: {
      'User-Agent': 'Playwright-E2E-Test/1.0 (Compatible; Testing)',
    },
    timeout: REQUEST_TIMEOUT_MS,
  })

  try {
    // Clean up test data
    console.log('Cleaning up test data...')
    await cleanupTestData(requestContext)
    console.log('Test data cleaned')

  } catch (error) {
    // Teardown failures should not fail the test suite
    console.warn('Teardown had issues:', error instanceof Error ? error.message : error)
  } finally {
    await requestContext.dispose()
  }

  // Clean up test database file
  cleanupTestDatabase()

  console.log('E2E Test Environment Teardown Complete')
}

/**
 * Clean up test database file
 */
function cleanupTestDatabase() {
  const testDbPath = '/tmp/shelly_test.db'
  try {
    if (fs.existsSync(testDbPath)) {
      fs.unlinkSync(testDbPath)
      console.log('Test database cleaned up')
    }
  } catch (error) {
    console.warn('Could not delete test database:', error)
  }
}

async function cleanupTestData(requestContext: any) {
  // Remove test devices - we need to get the list first to find the IDs
  try {
    // Get all devices
    const devicesResponse = await requestContext.get('/api/v1/devices')
    if (devicesResponse.ok()) {
      const devicesData = await devicesResponse.json()
      console.log('🔍 Devices API response structure:', JSON.stringify(devicesData, null, 2))
      const devices = devicesData.data?.devices || []
      
      // Find test devices by IP address (our test devices use specific IPs)
      const testDeviceIPs = ['192.168.1.100', '192.168.1.101']
      const testDevices = devices.filter((device: any) => 
        testDeviceIPs.includes(device.ip)
      )
      
      // Delete each test device
      for (const device of testDevices) {
        try {
          const deleteResponse = await requestContext.delete(`/api/v1/devices/${device.id}`)
          
          if (deleteResponse.ok() || deleteResponse.status() === 404) {
            console.log(`🗑️ Removed test device: ${device.name} (${device.ip})`)
          } else {
            console.log(`⚠️ Device cleanup failed for ${device.name}: ${deleteResponse.status()}`)
          }
        } catch (error) {
          console.warn(`⚠️ Could not remove test device ${device.name}:`, error)
        }
      }
    }
  } catch (error) {
    console.warn('⚠️ Could not retrieve devices for cleanup:', error)
    // Log the response if available for debugging
    if (error && typeof error === 'object' && 'response' in error) {
      try {
        const response = (error as any).response
        console.warn('⚠️ Cleanup error response status:', response?.status?.())
        console.warn('⚠️ Cleanup error response body:', await response?.text?.())
      } catch (e) {
        console.warn('⚠️ Could not read cleanup error response')
      }
    }
  }
}

export default globalTeardown