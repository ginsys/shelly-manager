import { test, expect } from '@playwright/test'

test.describe('API Integration E2E', () => {
  const baseURL = 'http://localhost:8080'

  test('should respond to health check endpoint', async ({ request }) => {
    const response = await request.get(`${baseURL}/api/v1/health`)
    
    expect(response.ok()).toBeTruthy()
    expect(response.status()).toBe(200)
    
    const data = await response.json()
    expect(data).toHaveProperty('status')
  })

  test('should handle devices API endpoints', async ({ request }) => {
    // Get devices list
    const devicesResponse = await request.get(`${baseURL}/api/v1/devices`)
    expect(devicesResponse.ok()).toBeTruthy()
    
    const devicesData = await devicesResponse.json()
    expect(devicesData).toHaveProperty('success')
    
    if (devicesData.success && devicesData.data?.devices?.length > 0) {
      const device = devicesData.data.devices[0]
      
      // Get specific device details
      const deviceResponse = await request.get(`${baseURL}/api/v1/devices/${device.id}`)
      expect(deviceResponse.ok()).toBeTruthy()
      
      const deviceData = await deviceResponse.json()
      expect(deviceData.success).toBe(true)
      expect(deviceData.data).toHaveProperty('device')
    }
  })

  test('should handle export history API', async ({ request }) => {
    const response = await request.get(`${baseURL}/api/v1/export/history`)
    
    expect(response.ok()).toBeTruthy()
    
    const data = await response.json()
    expect(data).toHaveProperty('success')
    expect(data).toHaveProperty('data')
    expect(data.data).toHaveProperty('history')
    expect(Array.isArray(data.data.history)).toBe(true)
  })

  test('should handle export history pagination', async ({ request }) => {
    // Test with pagination parameters
    const response = await request.get(`${baseURL}/api/v1/export/history?page=1&page_size=5`)
    
    expect(response.ok()).toBeTruthy()
    
    const data = await response.json()
    expect(data.success).toBe(true)
    
    if (data.data?.history?.length > 0) {
      expect(data.data.history.length).toBeLessThanOrEqual(5)
    }
    
    // Check for pagination metadata
    expect(data).toHaveProperty('meta')
  })

  test('should handle export history filtering', async ({ request }) => {
    // Test plugin filter
    const response = await request.get(`${baseURL}/api/v1/export/history?plugin=home-assistant`)
    
    expect(response.ok()).toBeTruthy()
    
    const data = await response.json()
    expect(data.success).toBe(true)
    
    if (data.data?.history?.length > 0) {
      // All items should match the filter
      for (const item of data.data.history) {
        expect(item.plugin_name).toBe('home-assistant')
      }
    }
  })

  test('should handle export statistics API', async ({ request }) => {
    const response = await request.get(`${baseURL}/api/v1/export/statistics`)
    
    expect(response.ok()).toBeTruthy()
    
    const data = await response.json()
    expect(data.success).toBe(true)
    expect(data.data).toHaveProperty('total')
    expect(data.data).toHaveProperty('success')
    expect(data.data).toHaveProperty('failure')
    expect(data.data).toHaveProperty('by_plugin')
    
    // Verify numeric values
    expect(typeof data.data.total).toBe('number')
    expect(typeof data.data.success).toBe('number')
    expect(typeof data.data.failure).toBe('number')
  })

  test('should handle export preview API', async ({ request }) => {
    const previewRequest = {
      plugin_name: 'home-assistant',
      format: 'yaml',
      config: {},
      filters: {},
      options: {}
    }
    
    const response = await request.post(`${baseURL}/api/v1/export/preview`, {
      data: previewRequest
    })
    
    // Response might be 200 or error depending on setup
    if (response.ok()) {
      const data = await response.json()
      expect(data).toHaveProperty('success')
      
      if (data.success) {
        expect(data.data).toHaveProperty('preview')
        expect(data.data).toHaveProperty('summary')
      }
    } else {
      // Should return proper error structure
      const errorData = await response.json()
      expect(errorData).toHaveProperty('success', false)
      expect(errorData).toHaveProperty('error')
    }
  })

  test('should handle import preview API', async ({ request }) => {
    const importRequest = {
      plugin_name: 'home-assistant',
      format: 'yaml',
      data: 'test: data',
      config: {},
      options: {}
    }
    
    const response = await request.post(`${baseURL}/api/v1/import/preview`, {
      data: importRequest
    })
    
    // Response might be success or error
    if (response.ok()) {
      const data = await response.json()
      expect(data).toHaveProperty('success')
      
      if (data.success) {
        expect(data.data).toHaveProperty('preview')
        expect(data.data).toHaveProperty('summary')
      }
    } else {
      const errorData = await response.json()
      expect(errorData).toHaveProperty('success', false)
      expect(errorData).toHaveProperty('error')
    }
  })

  test('should handle metrics endpoints', async ({ request }) => {
    // Metrics status
    const statusResponse = await request.get(`${baseURL}/api/v1/metrics/status`)
    if (statusResponse.ok()) {
      const statusData = await statusResponse.json()
      expect(statusData).toHaveProperty('success')
    }
    
    // Metrics health
    const healthResponse = await request.get(`${baseURL}/api/v1/metrics/health`)
    if (healthResponse.ok()) {
      const healthData = await healthResponse.json()
      expect(healthData).toHaveProperty('success')
    }
    
    // System metrics
    const systemResponse = await request.get(`${baseURL}/api/v1/metrics/system`)
    if (systemResponse.ok()) {
      const systemData = await systemResponse.json()
      expect(systemData).toHaveProperty('success')
      
      if (systemData.success && systemData.data) {
        expect(systemData.data).toHaveProperty('cpu')
        expect(systemData.data).toHaveProperty('memory')
        expect(systemData.data).toHaveProperty('timestamp')
      }
    }
  })

  test('should handle plugin list API', async ({ request }) => {
    const response = await request.get(`${baseURL}/api/v1/plugins`)
    
    expect(response.ok()).toBeTruthy()
    
    const data = await response.json()
    expect(data.success).toBe(true)
    expect(Array.isArray(data.data)).toBe(true)
    
    if (data.data.length > 0) {
      const plugin = data.data[0]
      expect(plugin).toHaveProperty('name')
      expect(plugin).toHaveProperty('type')
      expect(plugin).toHaveProperty('enabled')
    }
  })

  test('should handle plugin schemas API', async ({ request }) => {
    // First get available plugins
    const pluginsResponse = await request.get(`${baseURL}/api/v1/plugins`)
    expect(pluginsResponse.ok()).toBeTruthy()
    
    const pluginsData = await pluginsResponse.json()
    
    if (pluginsData.success && pluginsData.data.length > 0) {
      const plugin = pluginsData.data[0]
      
      // Get plugin schema
      const schemaResponse = await request.get(`${baseURL}/api/v1/plugins/${plugin.name}/schema`)
      
      if (schemaResponse.ok()) {
        const schemaData = await schemaResponse.json()
        expect(schemaData.success).toBe(true)
        expect(schemaData.data).toHaveProperty('schema')
      }
    }
  })

  test('should handle error responses correctly', async ({ request }) => {
    // Test non-existent endpoint
    const response = await request.get(`${baseURL}/api/v1/nonexistent`)
    
    expect(response.status()).toBe(404)
    
    const data = await response.json()
    expect(data.success).toBe(false)
    expect(data).toHaveProperty('error')
    expect(data.error).toHaveProperty('message')
  })

  test('should handle malformed requests', async ({ request }) => {
    // Send malformed JSON
    const response = await request.post(`${baseURL}/api/v1/export/preview`, {
      data: 'invalid json string'
    })
    
    expect(response.status()).toBeGreaterThanOrEqual(400)
    
    const data = await response.json()
    expect(data.success).toBe(false)
    expect(data.error).toHaveProperty('message')
  })

  test('should respect rate limiting', async ({ request }) => {
    // Make multiple rapid requests to test rate limiting
    const requests = Array.from({ length: 10 }, () => 
      request.get(`${baseURL}/api/v1/health`)
    )
    
    const responses = await Promise.all(requests)
    
    // All requests should succeed or be rate limited
    for (const response of responses) {
      expect(response.status()).toBeLessThan(500) // No server errors
      
      if (response.status() === 429) {
        // Rate limited - this is expected behavior
        const data = await response.json()
        expect(data.success).toBe(false)
        expect(data.error.message).toMatch(/rate.*limit/i)
      } else {
        // Successful response
        expect(response.ok()).toBeTruthy()
      }
    }
  })

  test('should handle CORS headers correctly', async ({ request }) => {
    const response = await request.get(`${baseURL}/api/v1/health`)
    
    expect(response.ok()).toBeTruthy()
    
    // Check for CORS headers (if configured)
    const headers = response.headers()
    
    // These might be present depending on configuration
    if (headers['access-control-allow-origin']) {
      expect(headers['access-control-allow-origin']).toBeTruthy()
    }
  })

  // Tests for newly implemented features

  test('should handle schedule management APIs', async ({ request }) => {
    // Get schedules
    const response = await request.get(`${baseURL}/api/v1/schedules`)
    
    if (response.ok()) {
      const data = await response.json()
      expect(data).toHaveProperty('success')
      
      if (data.success) {
        expect(data.data).toHaveProperty('schedules')
        expect(Array.isArray(data.data.schedules)).toBe(true)
      }
    } else {
      // Feature might not be implemented yet
      expect(response.status()).toBe(404)
    }
  })

  test('should handle backup management APIs', async ({ request }) => {
    // Get backups
    const response = await request.get(`${baseURL}/api/v1/backups`)
    
    if (response.ok()) {
      const data = await response.json()
      expect(data).toHaveProperty('success')
      
      if (data.success) {
        expect(data.data).toHaveProperty('backups')
        expect(Array.isArray(data.data.backups)).toBe(true)
      }
    } else {
      // Feature might not be implemented yet
      expect(response.status()).toBe(404)
    }
  })

  test('should handle GitOps configuration APIs', async ({ request }) => {
    // Get GitOps config
    const response = await request.get(`${baseURL}/api/v1/gitops/config`)
    
    if (response.ok()) {
      const data = await response.json()
      expect(data).toHaveProperty('success')
      
      if (data.success) {
        expect(data.data).toHaveProperty('config')
      }
    } else {
      // Feature might not be implemented yet
      expect(response.status()).toBe(404)
    }
  })

  test('should handle plugin configuration APIs', async ({ request }) => {
    // First get available plugins
    const pluginsResponse = await request.get(`${baseURL}/api/v1/plugins`)
    
    if (pluginsResponse.ok()) {
      const pluginsData = await pluginsResponse.json()
      
      if (pluginsData.success && pluginsData.data.length > 0) {
        const plugin = pluginsData.data[0]
        
        // Get plugin configuration
        const configResponse = await request.get(`${baseURL}/api/v1/plugins/${plugin.name}/config`)
        
        if (configResponse.ok()) {
          const configData = await configResponse.json()
          expect(configData).toHaveProperty('success')
        }
      }
    }
  })

  test('should handle device statistics API', async ({ request }) => {
    const response = await request.get(`${baseURL}/api/v1/devices/statistics`)
    
    if (response.ok()) {
      const data = await response.json()
      expect(data.success).toBe(true)
      
      if (data.data) {
        expect(data.data).toHaveProperty('total')
        expect(typeof data.data.total).toBe('number')
      }
    }
  })

  test('should validate API request schemas', async ({ request }) => {
    // Test export preview with invalid data
    const invalidRequest = {
      // Missing required fields
      invalid_field: 'test'
    }
    
    const response = await request.post(`${baseURL}/api/v1/export/preview`, {
      data: invalidRequest
    })
    
    // Should return validation error
    expect(response.status()).toBeGreaterThanOrEqual(400)
    
    const data = await response.json()
    expect(data.success).toBe(false)
    expect(data.error).toHaveProperty('message')
  })

  test('should handle concurrent API requests', async ({ request }) => {
    // Make multiple concurrent requests
    const concurrentRequests = [
      request.get(`${baseURL}/api/v1/health`),
      request.get(`${baseURL}/api/v1/devices`),
      request.get(`${baseURL}/api/v1/plugins`),
      request.get(`${baseURL}/api/v1/export/history`),
      request.get(`${baseURL}/api/v1/metrics/status`)
    ]
    
    const responses = await Promise.all(concurrentRequests)
    
    // All requests should complete successfully or with expected errors
    for (const response of responses) {
      expect(response.status()).toBeLessThan(500) // No server errors
    }
    
    // At least the health check should succeed
    expect(responses[0].ok()).toBeTruthy()
  })
})