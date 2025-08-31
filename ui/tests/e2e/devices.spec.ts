import { test, expect } from '@playwright/test'

test('devices page loads', async ({ page }) => {
  // Assumes backend is running via `make run` on :8080 serving SPA
  await page.goto('http://localhost:8080/')
  await expect(page.locator('h1')).toHaveText(/Devices/i)
})

