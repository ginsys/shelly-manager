import { FullConfig, request } from '@playwright/test'

async function globalSetup(config: FullConfig) {
  console.log('🚀 Starting E2E Test Environment Setup...')
  
  // Create a request context for API calls
  const requestContext = await request.newContext({
    baseURL: 'http://localhost:8080',
    extraHTTPHeaders: {
      'User-Agent': 'Playwright-E2E-Test/1.0 (Compatible; Testing)',
    },
  })
  
  try {
    // Wait for backend to be ready (try multiple endpoints)
    console.log('⏳ Waiting for backend API...')
    let backendReady = false
    const healthEndpoints = ['/healthz', '/api/v1/health', '/ping', '/']
    
    for (const endpoint of healthEndpoints) {
      try {
        const response = await requestContext.get(endpoint)
        if (response.ok()) {
          console.log(`✅ Backend ready at ${endpoint}`)
          backendReady = true
          break
        }
      } catch (error) {
        console.log(`⏳ Endpoint ${endpoint} not ready, trying next...`)
      }
    }
    
    if (!backendReady) {
      console.error('❌ Backend API is not accessible at http://localhost:8080')
      console.error('💡 Make sure the backend is running with: go run ./cmd/shelly-manager server')
      throw new Error('Backend API is not accessible - cannot proceed with tests')
    }
    
    // Setup test data (backend is ready)
    console.log('🔧 Setting up test data...')
    await setupTestData(requestContext)
    console.log('✅ Test data ready')
    
  } catch (error) {
    console.error('❌ Setup failed:', error)
    throw error
  } finally {
    await requestContext.dispose()
  }
  
  console.log('✨ E2E Test Environment Setup Complete')
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