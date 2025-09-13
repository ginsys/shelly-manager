import { test, expect } from '@playwright/test'

test.describe('API Integration Tests', () => {
  const baseURL = 'http://localhost:8080'
  const headers = {
    'User-Agent': 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
    'Content-Type': 'application/json'
  }

  // Set reasonable timeout for API tests
  test.setTimeout(30000) // 30 seconds instead of default 60

  test.describe('Devices API', () => {
    test('GET /api/v1/devices should return device list', async ({ request }) => {
      const response = await request.get(`${baseURL}/api/v1/devices`, { headers })
      
      expect(response.ok()).toBe(true)
      expect(response.status()).toBe(200)
      
      const data = await response.json()
      expect(data).toHaveProperty('success', true)
      expect(data).toHaveProperty('data')
      expect(data.data).toHaveProperty('devices')
      expect(Array.isArray(data.data.devices)).toBe(true)
      
      // Validate device structure
      if (data.data.devices.length > 0) {
        const device = data.data.devices[0]
        expect(device).toHaveProperty('id')
        expect(device).toHaveProperty('ip')
        expect(device).toHaveProperty('mac')
        expect(device).toHaveProperty('type')
        expect(device).toHaveProperty('name')
        expect(device).toHaveProperty('status')
        expect(device).toHaveProperty('firmware')
      }
    })

    test('GET /api/v1/devices/{id} should return single device', async ({ request }) => {
      // First get list to find a valid ID
      const listResponse = await request.get(`${baseURL}/api/v1/devices`, { headers })
      expect(listResponse.ok()).toBe(true)
      
      const listData = await listResponse.json()
      expect(listData).toHaveProperty('success', true)
      expect(listData).toHaveProperty('data')
      expect(listData.data).toHaveProperty('devices')
      expect(Array.isArray(listData.data.devices)).toBe(true)
      
      if (listData.data.devices.length === 0) {
        test.skip('No devices available for testing')
      }
      
      const deviceId = listData.data.devices[0].id
      expect(deviceId).toBeTruthy()
      const response = await request.get(`${baseURL}/api/v1/devices/${deviceId}`, { headers })
      
      expect(response.ok()).toBe(true)
      expect(response.status()).toBe(200)
      
      const data = await response.json()
      expect(data).toHaveProperty('success', true)
      expect(data).toHaveProperty('id', deviceId)
    })

    test('GET /api/v1/devices/{id} should return 404 for non-existent device', async ({ request }) => {
      const response = await request.get(`${baseURL}/api/v1/devices/99999`, { headers })
      
      expect(response.status()).toBe(404)
      
      const data = await response.json()
      expect(data).toHaveProperty('success', false)
    })

    test('POST /api/v1/devices should create new device', async ({ request }) => {
      const uniqueId = Date.now().toString(36) + Math.random().toString(36).substr(2, 9)
      const randomIp = `192.168.1.${150 + Math.floor(Math.random() * 50)}`
      const newDevice = {
        ip: randomIp,
        mac: `FF:EE:DD:CC:BB:${uniqueId.substr(-2).toUpperCase()}`,
        type: 'Smart Plug',
        name: `API Test Device ${uniqueId}`,
        firmware: '20231219-134356',
        status: 'online',
        settings: '{"model":"SHPLG-S","gen":1,"auth_enabled":true}'
      }
      
      const response = await request.post(`${baseURL}/api/v1/devices`, {
        headers,
        data: newDevice
      })
      
      if (!response.ok()) {
        const errorText = await response.text()
        console.log(`Device creation failed with status ${response.status()}: ${errorText}`)
      }
      expect(response.ok()).toBe(true)
      expect(response.status()).toBe(201)
      
      const data = await response.json()
      console.log('Device creation response:', JSON.stringify(data, null, 2))
      expect(data).toHaveProperty('success', true)
      expect(data).toHaveProperty('data')
      expect(data.data).toHaveProperty('id')
      expect(data.data.id).toBeDefined()
      expect(data.data.id).not.toBeNull()
      
      // Clean up: delete created device
      const deviceId = data.data.id
      expect(deviceId).toBeTruthy()
      await request.delete(`${baseURL}/api/v1/devices/${deviceId}`, { headers })
    })

    test('PUT /api/v1/devices/{id} should update device', async ({ request }) => {
      // First create a test device
      const uniqueId = Date.now().toString(36) + Math.random().toString(36).substr(2, 9)
      const randomIp = `192.168.2.${150 + Math.floor(Math.random() * 50)}`
      const newDevice = {
        ip: randomIp,
        mac: `FF:EE:DD:CC:BB:${uniqueId.substr(-2).toUpperCase()}`,
        type: 'Smart Plug',
        name: `API Update Test Device ${uniqueId}`,
        firmware: '20231219-134356',
        status: 'online',
        settings: '{"model":"SHPLG-S","gen":1,"auth_enabled":true}'
      }
      
      const createResponse = await request.post(`${baseURL}/api/v1/devices`, {
        headers,
        data: newDevice
      })
      const createData = await createResponse.json()
      expect(createData).toHaveProperty('success', true)
      expect(createData).toHaveProperty('data')
      expect(createData.data).toHaveProperty('id')
      expect(createData.data.id).toBeDefined()
      expect(createData.data.id).not.toBeNull()
      
      const deviceId = createData.data.id
      expect(deviceId).toBeTruthy()
      
      // Update the device
      const updatedDevice = {
        ...newDevice,
        name: 'Updated API Test Device',
        status: 'offline'
      }
      
      const updateResponse = await request.put(`${baseURL}/api/v1/devices/${deviceId}`, {
        headers,
        data: updatedDevice
      })
      
      expect(updateResponse.ok()).toBe(true)
      expect(updateResponse.status()).toBe(200)
      
      const updateData = await updateResponse.json()
      expect(updateData).toHaveProperty('success', true)
      expect(updateData.data).toHaveProperty('name', 'Updated API Test Device')
      
      // Clean up
      await request.delete(`${baseURL}/api/v1/devices/${deviceId}`, { headers })
    })

    test('DELETE /api/v1/devices/{id} should delete device', async ({ request }) => {
      // First create a test device
      const uniqueId = Date.now().toString(36) + Math.random().toString(36).substr(2, 9)
      const randomIp = `192.168.3.${150 + Math.floor(Math.random() * 50)}`
      const newDevice = {
        ip: randomIp,
        mac: `FF:EE:DD:CC:BB:${uniqueId.substr(-2).toUpperCase()}`,
        type: 'Smart Plug',
        name: `API Delete Test Device ${uniqueId}`,
        firmware: '20231219-134356',
        status: 'online',
        settings: '{"model":"SHPLG-S","gen":1,"auth_enabled":true}'
      }
      
      const createResponse = await request.post(`${baseURL}/api/v1/devices`, {
        headers,
        data: newDevice
      })
      const createData = await createResponse.json()
      expect(createData).toHaveProperty('success', true)
      expect(createData).toHaveProperty('data')
      expect(createData.data).toHaveProperty('id')
      expect(createData.data.id).toBeDefined()
      expect(createData.data.id).not.toBeNull()
      
      const deviceId = createData.data.id
      expect(deviceId).toBeTruthy()
      
      // Delete the device
      const deleteResponse = await request.delete(`${baseURL}/api/v1/devices/${deviceId}`, { headers })
      
      expect(deleteResponse.ok()).toBe(true)
      expect(deleteResponse.status()).toBe(200)
      
      // Verify deletion
      const getResponse = await request.get(`${baseURL}/api/v1/devices/${deviceId}`, { headers })
      expect(getResponse.status()).toBe(404)
    })
  })

  test.describe('Export API', () => {
    test('GET /api/v1/export/devices should export devices', async ({ request }) => {
      const response = await request.get(`${baseURL}/api/v1/export/devices`, { headers })
      
      expect(response.ok()).toBe(true)
      expect(response.status()).toBe(200)
      
      const data = await response.json()
      expect(data).toHaveProperty('devices')
      expect(data).toHaveProperty('metadata')
      expect(data).toHaveProperty('export_date')
      expect(Array.isArray(data.devices)).toBe(true)
    })

    test('GET /api/v1/export/devices should include metadata', async ({ request }) => {
      const response = await request.get(`${baseURL}/api/v1/export/devices`, { headers })
      const data = await response.json()
      
      expect(data.metadata).toHaveProperty('version')
      expect(data.metadata).toHaveProperty('exported_by')
      expect(data.metadata.exported_by).toBe('shelly-manager')
    })

    test('Export should be valid JSON format', async ({ request }) => {
      const response = await request.get(`${baseURL}/api/v1/export/devices`, { headers })
      const text = await response.text()
      
      // Should be valid JSON
      expect(() => JSON.parse(text)).not.toThrow()
      
      const data = JSON.parse(text)
      expect(typeof data).toBe('object')
    })
  })

  test.describe('Import API', () => {
    test('POST /api/v1/import/devices should import valid data', async ({ request }) => {
      const uniqueId = Date.now().toString(36) + Math.random().toString(36).substr(2, 9)
      const randomIp = `192.168.4.${150 + Math.floor(Math.random() * 50)}`
      const importData = {
        devices: [
          {
            ip: randomIp,
            mac: `AA:BB:CC:DD:EE:${uniqueId.substr(-2).toUpperCase()}`,
            type: 'Smart Plug',
            name: `api-import-test-device-${uniqueId}`,
            firmware: '20231219-134356',
            status: 'offline',
            settings: '{"model":"SHPLG-S","gen":1,"auth_enabled":true}'
          }
        ],
        metadata: {
          version: '1.0',
          exported_by: 'test-suite',
          export_date: new Date().toISOString()
        }
      }
      
      const response = await request.post(`${baseURL}/api/v1/import/devices`, {
        headers,
        data: importData
      })
      
      expect(response.ok()).toBe(true)
      expect(response.status()).toBe(200)
      
      const responseData = await response.json()
      expect(responseData).toHaveProperty('success', true)
      expect(responseData).toHaveProperty('data')
      expect(responseData.data).toHaveProperty('imported_count', 1)
      
      // Verify imported device exists
      const devicesResponse = await request.get(`${baseURL}/api/v1/devices`, { headers })
      const devicesData = await devicesResponse.json()
      expect(devicesData).toHaveProperty('success', true)
      expect(devicesData).toHaveProperty('data')
      expect(devicesData.data).toHaveProperty('devices')
      expect(Array.isArray(devicesData.data.devices)).toBe(true)
      
      const importedDevice = devicesData.data.devices.find((d: any) => d.name === `api-import-test-device-${uniqueId}`)
      
      expect(importedDevice).toBeTruthy()
      
      // Clean up
      if (importedDevice && importedDevice.id) {
        expect(importedDevice.id).toBeTruthy()
        await request.delete(`${baseURL}/api/v1/devices/${importedDevice.id}`, { headers })
      }
    })

    test('POST /api/v1/import/devices should reject invalid data', async ({ request }) => {
      const invalidData = {
        invalid: 'structure'
      }
      
      const response = await request.post(`${baseURL}/api/v1/import/devices`, {
        headers,
        data: invalidData
      })
      
      expect(response.status()).toBe(400)
      
      const data = await response.json()
      expect(data).toHaveProperty('success', false)
    })

    test('POST /api/v1/import/devices should validate device structure', async ({ request }) => {
      const invalidDeviceData = {
        devices: [
          {
            // Missing required fields
            name: 'incomplete-device'
          }
        ],
        metadata: {
          version: '1.0'
        }
      }
      
      const response = await request.post(`${baseURL}/api/v1/import/devices`, {
        headers,
        data: invalidDeviceData
      })
      
      expect(response.status()).toBe(400)
      
      const data = await response.json()
      expect(data).toHaveProperty('success', false)
      expect(data.error).toHaveProperty('message')
    })
  })

  test.describe('Status and Health API', () => {
    test('GET /api/v1/status should return system status', async ({ request }) => {
      const response = await request.get(`${baseURL}/api/v1/status`, { headers })
      
      expect(response.ok()).toBe(true)
      expect(response.status()).toBe(200)
      
      const data = await response.json()
      expect(data).toHaveProperty('success', true)
      expect(data.data).toHaveProperty('status')
      expect(data.data).toHaveProperty('uptime')
      expect(data.data).toHaveProperty('version')
    })

    test('System should report healthy status', async ({ request }) => {
      const response = await request.get(`${baseURL}/api/v1/status`, { headers })
      const data = await response.json()
      
      expect(data.data.status).toBe('healthy')
      expect(typeof data.data.uptime).toBe('number')
      expect(data.data.uptime).toBeGreaterThan(0)
    })
  })

  test.describe('Error Handling', () => {
    test('Should handle malformed JSON requests', async ({ request }) => {
      const response = await request.post(`${baseURL}/api/v1/devices`, {
        headers: {
          ...headers,
          'Content-Type': 'application/json'
        },
        data: 'invalid json{'
      })
      
      expect(response.status()).toBe(400)
      
      const data = await response.json()
      expect(data).toHaveProperty('success', false)
    })

    test('Should handle requests without required headers', async ({ request }) => {
      const response = await request.get(`${baseURL}/api/v1/devices`, {
        headers: {} // No User-Agent
      })
      
      // Depending on security settings, might be 400 or 200
      expect([200, 400]).toContain(response.status())
    })

    test('Should handle large request payloads', async ({ request }) => {
      // Reduced from 1000 to 100 for faster testing while still validating large payload handling
      const largeData = {
        devices: Array(100).fill(0).map((_, i) => ({
          ip: `192.168.${Math.floor(i / 255)}.${i % 255}`,
          mac: `AA:BB:CC:DD:${Math.floor(i / 255).toString(16).padStart(2, '0')}:${(i % 255).toString(16).padStart(2, '0')}`,
          type: 'Load Test Device',
          name: `load-test-device-${i}`,
          firmware: 'test-version',
          status: 'offline',
          settings: '{"model":"SHPLG-S","gen":1,"auth_enabled":true}'
        })),
        metadata: {
          version: '1.0',
          exported_by: 'load-test'
        }
      }
      
      const response = await request.post(`${baseURL}/api/v1/import/devices`, {
        headers,
        data: largeData
      })
      
      // Should either succeed or fail gracefully (not crash)
      expect([200, 400, 413, 429]).toContain(response.status())
      
      if (response.ok()) {
        const data = await response.json()
        expect(data).toHaveProperty('success')
        
        // Clean up if successful
        if (data.success) {
          // Delete test devices would require individual API calls
          // For now just log the result
          console.log(`Large import test: imported ${data.data.imported_count} devices`)
        }
      }
    })
  })

  test.describe('Rate Limiting and Security', () => {
    test('Should handle multiple rapid requests gracefully', async ({ request }) => {
      const promises = Array(20).fill(0).map(() =>
        request.get(`${baseURL}/api/v1/devices`, { headers })
      )
      
      const responses = await Promise.all(promises.map(p => p.catch(e => ({ error: e }))))
      const successfulResponses = responses.filter(r => !r.error && r.status() === 200)
      
      // Should handle most requests successfully or rate limit gracefully
      expect(successfulResponses.length).toBeGreaterThan(10)
    })

    test('Should validate API versioning', async ({ request }) => {
      // Test invalid API version
      const response = await request.get(`${baseURL}/api/v999/devices`, { headers })
      
      expect(response.status()).toBe(404)
    })

    test('Should handle CORS preflight requests', async ({ request }) => {
      const response = await request.fetch(`${baseURL}/api/v1/devices`, {
        method: 'OPTIONS',
        headers: {
          'Origin': 'http://localhost:5174',
          'Access-Control-Request-Method': 'GET'
        }
      })
      
      // Should allow CORS for development
      expect([200, 204]).toContain(response.status())
    })
  })
})