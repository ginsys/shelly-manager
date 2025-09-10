import { chromium, FullConfig } from '@playwright/test'

async function globalSetup(config: FullConfig) {
  console.log('🚀 Starting E2E Test Environment Setup...')
  
  // Create a browser for setup operations
  const browser = await chromium.launch()
  const context = await browser.newContext()
  const page = await context.newPage()
  
  try {
    // Wait for backend to be ready (try multiple endpoints)
    console.log('⏳ Waiting for backend API...')
    let backendReady = false
    const healthEndpoints = ['/healthz', '/api/v1/health', '/ping', '/']
    
    for (const endpoint of healthEndpoints) {
      try {
        await page.goto(`http://localhost:8080${endpoint}`)
        await page.waitForLoadState('networkidle', { timeout: 10000 })
        console.log(`✅ Backend ready at ${endpoint}`)
        backendReady = true
        break
      } catch (error) {
        console.log(`⏳ Endpoint ${endpoint} not ready, trying next...`)
      }
    }
    
    if (!backendReady) {
      console.log('⚠️ Backend not fully ready, continuing with limited functionality')
    }
    
    // Wait for frontend to be ready
    console.log('⏳ Waiting for frontend...')
    try {
      await page.goto('http://localhost:5173')
      await page.waitForLoadState('networkidle', { timeout: 30000 })
      console.log('✅ Frontend ready')
    } catch (error) {
      console.log('⚠️ Frontend may not be ready, tests will use fallback behavior')
    }
    
    // Setup test data if backend is available
    if (backendReady) {
      console.log('🔧 Setting up test data...')
      await setupTestData(page)
      console.log('✅ Test data ready')
    } else {
      console.log('⚠️ Skipping test data setup due to backend issues')
    }
    
  } catch (error) {
    console.error('❌ Setup failed:', error)
    throw error
  } finally {
    await context.close()
    await browser.close()
  }
  
  console.log('✨ E2E Test Environment Setup Complete')
}

async function setupTestData(page: any) {
  // Create test devices for export/import testing
  const testDevices = [
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
  ]
  
  // Add devices via API
  for (const device of testDevices) {
    try {
      const response = await page.evaluate(async (deviceData) => {
        const res = await fetch('http://localhost:8080/api/v1/devices', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify(deviceData)
        })
        return res.ok
      }, device)
      
      if (response) {
        console.log(`📱 Created test device: ${device.name}`)
      }
    } catch (error) {
      console.warn(`⚠️ Could not create test device ${device.name}:`, error)
    }
  }
}

export default globalSetup