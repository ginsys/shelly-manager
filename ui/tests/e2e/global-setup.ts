import { chromium, FullConfig } from '@playwright/test'

async function globalSetup(config: FullConfig) {
  console.log('🚀 Starting E2E Test Environment Setup...')
  
  // Create a browser for setup operations with realistic context
  const browser = await chromium.launch()
  const context = await browser.newContext({
    userAgent: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
  })
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
      console.error('❌ Backend API is not accessible at http://localhost:8080')
      console.error('💡 Make sure the backend is running with: go run ./cmd/shelly-manager server')
      throw new Error('Backend API is not accessible - cannot proceed with tests')
    }
    
    // Wait for frontend to be ready
    console.log('⏳ Waiting for frontend...')
    const frontendUrl = process.env.CI ? 'http://localhost:5173' : 'http://localhost:5173'
    let frontendReady = false
    try {
      await page.goto(frontendUrl)
      await page.waitForLoadState('networkidle', { timeout: 30000 })
      
      // Verify the page has actual content (try multiple selectors as fallback)
      const contentSelectors = ['main', '#app', '[data-testid="app"]', 'body > div']
      let hasContent = false
      for (const selector of contentSelectors) {
        try {
          await page.waitForSelector(selector, { timeout: 5000 })
          hasContent = true
          break
        } catch {
          // Try next selector
        }
      }
      
      if (!hasContent) {
        throw new Error('Frontend loaded but no content found')
      }
      
      console.log('✅ Frontend ready')
      frontendReady = true
    } catch (error) {
      console.error(`❌ Frontend is not accessible at ${frontendUrl}`)
      console.error('💡 Make sure the frontend is running with: npm run dev (local) or npx serve dist (CI)')
      throw new Error(`Frontend is not accessible at ${frontendUrl} - cannot proceed with tests`)
    }
    
    // Setup test data (both services are now confirmed ready)
    console.log('🔧 Setting up test data...')
    await setupTestData(page)
    console.log('✅ Test data ready')
    
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
        try {
          const res = await fetch('http://localhost:8080/api/v1/devices', {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
              'User-Agent': 'Playwright-E2E-Test/1.0 (Compatible; Testing)',
            },
            body: JSON.stringify(deviceData)
          })
          return { ok: res.ok, status: res.status, statusText: res.statusText }
        } catch (error) {
          return { ok: false, error: error.message }
        }
      }, device)
      
      if (response.ok) {
        console.log(`📱 Created test device: ${device.name}`)
      } else {
        console.log(`⚠️ Device creation failed for ${device.name}: ${response.status} ${response.statusText || response.error}`)
      }
    } catch (error) {
      console.warn(`⚠️ Could not create test device ${device.name}:`, error)
    }
  }
}

export default globalSetup