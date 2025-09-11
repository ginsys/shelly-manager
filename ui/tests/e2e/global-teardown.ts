import { FullConfig, request } from '@playwright/test'

async function globalTeardown(config: FullConfig) {
  console.log('🧹 Starting E2E Test Environment Teardown...')
  
  // Create a request context for API calls
  const requestContext = await request.newContext({
    baseURL: 'http://localhost:8080',
    extraHTTPHeaders: {
      'User-Agent': 'Playwright-E2E-Test/1.0 (Compatible; Testing)',
    },
  })
  
  try {
    // Clean up test data
    console.log('🗑️ Cleaning up test data...')
    await cleanupTestData(requestContext)
    console.log('✅ Test data cleaned')
    
  } catch (error) {
    console.warn('⚠️ Teardown had issues:', error)
  } finally {
    await requestContext.dispose()
  }
  
  console.log('✨ E2E Test Environment Teardown Complete')
}

async function cleanupTestData(requestContext: any) {
  // Remove test devices
  const testDeviceIds = ['test-device-1', 'test-device-2']
  
  for (const deviceId of testDeviceIds) {
    try {
      const response = await requestContext.delete(`/api/v1/devices/${deviceId}`)
      
      if (response.ok() || response.status() === 404) {
        console.log(`🗑️ Removed test device: ${deviceId}`)
      } else {
        console.log(`⚠️ Device cleanup failed for ${deviceId}: ${response.status()}`)
      }
    } catch (error) {
      console.warn(`⚠️ Could not remove test device ${deviceId}:`, error)
    }
  }
}

export default globalTeardown