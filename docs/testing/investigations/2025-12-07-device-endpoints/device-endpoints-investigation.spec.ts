/**
 * TEMPORARY INVESTIGATION SCRIPT
 *
 * Purpose: Test all 27 device API endpoints to see which work and which fail
 * Status: Not for permanent CI - delete after investigation
 *
 * Run with:
 *   npx playwright test device-endpoints-investigation --reporter=list
 *
 * This will show status codes and basic response structure for each endpoint.
 */

import { test, expect } from '@playwright/test'

const API_BASE = 'http://localhost:8080'
const USER_AGENT = 'Shelly-Manager-Investigation/1.0'

test.describe('Device Endpoints Investigation', () => {
  let testDeviceId: number
  let request: any

  test.beforeAll(async ({ playwright }) => {
    request = await playwright.request.newContext({
      baseURL: API_BASE,
      extraHTTPHeaders: {
        'User-Agent': USER_AGENT,
      },
    })

    // Get existing device for testing
    const devicesResp = await request.get('/api/v1/devices')
    if (devicesResp.ok()) {
      const body = await devicesResp.json()
      const devices = body.data || []

      if (devices.length > 0) {
        testDeviceId = devices[0].id
        console.log(`✓ Using existing device ID: ${testDeviceId} (IP: ${devices[0].ip_address})`)
      } else {
        throw new Error('No devices available for testing. Run with global-setup or create devices first.')
      }
    } else {
      throw new Error(`Failed to fetch devices: ${devicesResp.status()}`)
    }
  })

  test.afterAll(async () => {
    await request.dispose()
  })

  // Helper to test an endpoint
  async function testEndpoint(
    name: string,
    method: string,
    path: string,
    body?: any,
    expectedStatus?: number[]
  ) {
    const actualPath = path.replace('{id}', String(testDeviceId))

    let response
    if (method === 'GET') {
      response = await request.get(actualPath)
    } else if (method === 'POST') {
      response = await request.post(actualPath, { data: body || {} })
    } else if (method === 'PUT') {
      response = await request.put(actualPath, { data: body || {} })
    } else if (method === 'DELETE') {
      response = await request.delete(actualPath)
    }

    const status = response.status()
    const isExpected = expectedStatus ? expectedStatus.includes(status) : status < 400

    console.log(`${method.padEnd(6)} ${actualPath.padEnd(60)} → ${status} ${isExpected ? '✓' : '✗'}`)

    if (response.ok()) {
      try {
        const json = await response.json()
        // Show basic structure
        if (json.data) {
          const keys = typeof json.data === 'object' ? Object.keys(json.data).slice(0, 5) : []
          console.log(`       Response keys: [${keys.join(', ')}${keys.length === 5 ? ', ...' : ''}]`)
        }
      } catch {
        console.log('       (non-JSON response)')
      }
    } else {
      try {
        const json = await response.json()
        if (json.error) {
          console.log(`       Error: ${json.error}`)
        }
      } catch {}
    }

    return { name, method, path: actualPath, status, isExpected }
  }

  test('01. Core CRUD - GET /api/v1/devices', async () => {
    await testEndpoint('List devices', 'GET', '/api/v1/devices')
  })

  test('02. Core CRUD - POST /api/v1/devices', async () => {
    await testEndpoint('Create device', 'POST', '/api/v1/devices', {
      name: 'Temp Investigation Device',
      ip_address: '172.31.103.199',
      device_type: 'shelly1',
      enabled: false
    })
  })

  test('03. Core CRUD - GET /api/v1/devices/{id}', async () => {
    await testEndpoint('Get single device', 'GET', `/api/v1/devices/{id}`)
  })

  test('04. Core CRUD - PUT /api/v1/devices/{id}', async () => {
    await testEndpoint('Update device', 'PUT', `/api/v1/devices/{id}`, {
      name: 'Updated Investigation Device'
    })
  })

  test('05. Control & Status - POST /api/v1/devices/{id}/control', async () => {
    // This will likely timeout for non-existent device, that's OK
    await testEndpoint('Control device', 'POST', `/api/v1/devices/{id}/control`, {
      action: 'status'
    }, [200, 408, 500])
  })

  test('06. Control & Status - GET /api/v1/devices/{id}/status', async () => {
    // Will timeout for offline device
    await testEndpoint('Get device status', 'GET', `/api/v1/devices/{id}/status`, undefined, [200, 408, 500])
  })

  test('07. Control & Status - GET /api/v1/devices/{id}/energy', async () => {
    // Will timeout for offline device
    await testEndpoint('Get device energy', 'GET', `/api/v1/devices/{id}/energy`, undefined, [200, 408, 500])
  })

  test('08. Configuration - GET /api/v1/devices/{id}/config', async () => {
    await testEndpoint('Get stored config', 'GET', `/api/v1/devices/{id}/config`, undefined, [200, 404])
  })

  test('09. Configuration - PUT /api/v1/devices/{id}/config', async () => {
    await testEndpoint('Update stored config', 'PUT', `/api/v1/devices/{id}/config`, {
      config: { test: true }
    }, [200, 400, 404])
  })

  test('10. Configuration - GET /api/v1/devices/{id}/config/current', async () => {
    // Gets live config from device
    await testEndpoint('Get live config', 'GET', `/api/v1/devices/{id}/config/current`, undefined, [200, 408, 500])
  })

  test('11. Configuration - GET /api/v1/devices/{id}/config/current/normalized', async () => {
    await testEndpoint('Get normalized live config', 'GET', `/api/v1/devices/{id}/config/current/normalized`, undefined, [200, 408, 500])
  })

  test('12. Configuration - GET /api/v1/devices/{id}/config/typed/normalized', async () => {
    await testEndpoint('Get typed normalized config', 'GET', `/api/v1/devices/{id}/config/typed/normalized`, undefined, [200, 404, 408, 500])
  })

  test('13. Configuration - POST /api/v1/devices/{id}/config/import', async () => {
    // Imports config from device to database
    await testEndpoint('Import config', 'POST', `/api/v1/devices/{id}/config/import`, {}, [200, 202, 408, 500])
  })

  test('14. Configuration - GET /api/v1/devices/{id}/config/status', async () => {
    await testEndpoint('Get import status', 'GET', `/api/v1/devices/{id}/config/status`, undefined, [200, 404])
  })

  test('15. Configuration - POST /api/v1/devices/{id}/config/export', async () => {
    // Exports config from database to device
    await testEndpoint('Export config', 'POST', `/api/v1/devices/{id}/config/export`, {}, [200, 202, 404, 408, 500])
  })

  test('16. Configuration - GET /api/v1/devices/{id}/config/drift', async () => {
    // This is known to return 500 if no config stored (Issue #3 from log analysis)
    await testEndpoint('Detect config drift', 'GET', `/api/v1/devices/{id}/config/drift`, undefined, [200, 404, 500])
  })

  test('17. Configuration - POST /api/v1/devices/{id}/config/apply-template', async () => {
    await testEndpoint('Apply config template', 'POST', `/api/v1/devices/{id}/config/apply-template`, {
      template_id: 1
    }, [200, 400, 404])
  })

  test('18. Configuration - GET /api/v1/devices/{id}/config/history', async () => {
    await testEndpoint('Get config history', 'GET', `/api/v1/devices/{id}/config/history`, undefined, [200, 404])
  })

  test('19. Configuration - GET /api/v1/devices/{id}/config/typed', async () => {
    await testEndpoint('Get typed config', 'GET', `/api/v1/devices/{id}/config/typed`, undefined, [200, 404])
  })

  test('20. Configuration - PUT /api/v1/devices/{id}/config/typed', async () => {
    await testEndpoint('Update typed config', 'PUT', `/api/v1/devices/{id}/config/typed`, {
      config: { test: true }
    }, [200, 400, 404])
  })

  test('21. Capability Config - PUT /api/v1/devices/{id}/config/relay', async () => {
    await testEndpoint('Update relay config', 'PUT', `/api/v1/devices/{id}/config/relay`, {
      relay: { enabled: true }
    }, [200, 400, 404])
  })

  test('22. Capability Config - PUT /api/v1/devices/{id}/config/dimming', async () => {
    await testEndpoint('Update dimming config', 'PUT', `/api/v1/devices/{id}/config/dimming`, {
      dimming: { enabled: false }
    }, [200, 400, 404])
  })

  test('23. Capability Config - PUT /api/v1/devices/{id}/config/roller', async () => {
    await testEndpoint('Update roller config', 'PUT', `/api/v1/devices/{id}/config/roller`, {
      roller: { enabled: false }
    }, [200, 400, 404])
  })

  test('24. Capability Config - PUT /api/v1/devices/{id}/config/power-metering', async () => {
    await testEndpoint('Update power metering config', 'PUT', `/api/v1/devices/{id}/config/power-metering`, {
      power_metering: { enabled: true }
    }, [200, 400, 404])
  })

  test('25. Capability Config - PUT /api/v1/devices/{id}/config/auth', async () => {
    await testEndpoint('Update device auth', 'PUT', `/api/v1/devices/{id}/config/auth`, {
      auth: { enabled: false }
    }, [200, 400, 404])
  })

  test('26. Other - GET /api/v1/devices/{id}/capabilities', async () => {
    await testEndpoint('Get device capabilities', 'GET', `/api/v1/devices/{id}/capabilities`, undefined, [200, 404])
  })

  test('27. Core CRUD - DELETE /api/v1/devices/{id}', async () => {
    // Delete the test device we created (if we created one)
    // Skip if using an existing device
    if (testDeviceId !== undefined) {
      // Let's not delete for now to preserve test data
      console.log('Skipping DELETE to preserve test device')
    }
  })

  test('Summary Report', async () => {
    console.log('\n========================================')
    console.log('DEVICE ENDPOINTS INVESTIGATION SUMMARY')
    console.log('========================================\n')
    console.log('Review the output above to see:')
    console.log('- Which endpoints return 200 (working)')
    console.log('- Which endpoints return 404 (missing handlers)')
    console.log('- Which endpoints return 500 (errors)')
    console.log('- Which endpoints return 408 (timeouts)')
    console.log('\nKnown issues from log analysis:')
    console.log('- Config drift returns 500 when no stored config (should be 404)')
    console.log('- Status/energy/control timeout on offline devices (expected)')
    console.log('\nThis test file can be deleted after investigation.')
  })
})
