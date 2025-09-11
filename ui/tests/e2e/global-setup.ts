import { FullConfig } from '@playwright/test'

async function globalSetup(config: FullConfig) {
  console.log('🚀 Starting E2E Test Environment Setup...')
  
  try {
    // Wait for backend to be ready (try multiple endpoints)
    console.log('⏳ Waiting for backend API...')
    let backendReady = false
    const healthEndpoints = ['/healthz', '/api/v1/health', '/ping', '/']
    
    for (const endpoint of healthEndpoints) {
      try {
        const response = await fetch(`http://localhost:8080${endpoint}`)
        if (response.ok) {
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
    await setupTestData()
    console.log('✅ Test data ready')
    
  } catch (error) {
    console.error('❌ Setup failed:', error)
    throw error
  }
  
  console.log('✨ E2E Test Environment Setup Complete')
}

async function setupTestData() {
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
      const response = await fetch('http://localhost:8080/api/v1/devices', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'User-Agent': 'Playwright-E2E-Test/1.0 (Compatible; Testing)',
        },
        body: JSON.stringify(device)
      })
      
      if (response.ok) {
        console.log(`📱 Created test device: ${device.name}`)
      } else {
        console.log(`⚠️ Device creation failed for ${device.name}: ${response.status} ${response.statusText}`)
      }
    } catch (error) {
      console.warn(`⚠️ Could not create test device ${device.name}:`, error)
    }
  }
}

export default globalSetup