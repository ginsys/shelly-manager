import { test, expect } from '@playwright/test'

/**
 * Smoke API tests - basic connectivity and response validation
 * Full API testing is covered in api-tests.spec.ts
 */
test.describe('Smoke API Tests', () => {
  const baseURL = 'http://localhost:8080'
  const headers = {
    'User-Agent': 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
    'Content-Type': 'application/json'
  }

  test.setTimeout(30000) // 30 seconds

  test('health endpoint responds', async ({ request }) => {
    const response = await request.get(`${baseURL}/health`, { headers })
    expect([200, 204]).toContain(response.status())
  })

  test('devices API returns valid JSON', async ({ request }) => {
    const response = await request.get(`${baseURL}/api/v1/devices`, { headers })
    expect(response.ok()).toBe(true)

    const data = await response.json()
    expect(data).toHaveProperty('success')
  })

  test('status API responds', async ({ request }) => {
    const response = await request.get(`${baseURL}/api/v1/status`, { headers })
    expect(response.ok()).toBe(true)

    const data = await response.json()
    expect(data).toHaveProperty('success', true)
  })
})
