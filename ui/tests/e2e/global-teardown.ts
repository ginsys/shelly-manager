import { chromium, FullConfig } from '@playwright/test'

async function globalTeardown(config: FullConfig) {
  console.log('🧹 Starting E2E Test Environment Teardown...')
  
  // Create a browser for teardown operations
  const browser = await chromium.launch()
  const context = await browser.newContext()
  const page = await context.newPage()
  
  try {
    // Clean up test data
    console.log('🗑️ Cleaning up test data...')
    await cleanupTestData(page)
    console.log('✅ Test data cleaned')
    
  } catch (error) {
    console.warn('⚠️ Teardown had issues:', error)
  } finally {
    await context.close()
    await browser.close()
  }
  
  console.log('✨ E2E Test Environment Teardown Complete')
}

async function cleanupTestData(page: any) {
  // Remove test devices
  const testDeviceIds = ['test-device-1', 'test-device-2']
  
  for (const deviceId of testDeviceIds) {
    try {
      const response = await page.evaluate(async (id) => {
        const res = await fetch(`http://localhost:8080/api/v1/devices/${id}`, {
          method: 'DELETE'
        })
        return res.ok
      }, deviceId)
      
      if (response) {
        console.log(`🗑️ Removed test device: ${deviceId}`)
      }
    } catch (error) {
      console.warn(`⚠️ Could not remove test device ${deviceId}:`, error)
    }
  }
}

export default globalTeardown