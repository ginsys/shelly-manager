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
  // Remove test devices - we need to get the list first to find the IDs
  try {
    // Get all devices
    const devicesResponse = await requestContext.get('/api/v1/devices')
    if (devicesResponse.ok()) {
      const devicesData = await devicesResponse.json()
      const devices = devicesData.data || []
      
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
  }
}

export default globalTeardown