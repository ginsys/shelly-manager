import { defineConfig, devices } from '@playwright/test'

/**
 * Smoke test configuration - optimized for fast feedback
 * Run with: npx playwright test --config=playwright-smoke.config.ts
 * @see https://playwright.dev/docs/test-configuration
 */
export default defineConfig({
  testDir: './tests/e2e',
  
  // Only run smoke tests
  testMatch: ['**/smoke.spec.ts', '**/api-tests.spec.ts'],
  
  // Run tests in files in parallel
  fullyParallel: true,
  
  // Fail the build on CI if you accidentally left test.only in the source code
  forbidOnly: !!process.env.CI,
  
  // Retry once on failure for quick feedback
  retries: 1,
  
  // Only 1 worker for smoke tests to avoid resource contention
  workers: 1,
  
  // Reporter optimized for quick feedback
  reporter: [
    ['list'], // Simple output
    ['json', { outputFile: 'test-results/smoke-results.json' }],
  ],
  
  // Shorter timeout for faster feedback
  timeout: 30 * 1000,
  
  // Shared settings for smoke tests
  use: {
    // Base URL for the application
    baseURL: process.env.CI ? 'http://localhost:5173' : 'http://localhost:5173',
    
    // API endpoint for backend tests
    extraHTTPHeaders: {
      'Accept': 'application/json',
      'User-Agent': 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36'
    },
    
    // Minimal trace collection for faster execution
    trace: 'retain-on-failure',
    
    // Screenshots only on failure
    screenshot: 'only-on-failure',
    
    // No video recording for faster execution
    video: 'off',
  },

  // Only essential test projects for smoke testing
  projects: [
    // Desktop Chrome only for smoke tests
    {
      name: 'chromium-smoke',
      use: { 
        ...devices['Desktop Chrome'],
        // Optimized browser settings for speed
        launchOptions: {
          args: [
            '--disable-dev-shm-usage',
            '--no-sandbox', 
            '--disable-gpu',
            '--disable-web-security',
            '--disable-features=VizDisplayCompositor',
            '--disable-extensions',
            '--disable-plugins'
          ],
        },
        // Shorter timeouts for smoke tests
        navigationTimeout: 20000,
        actionTimeout: 10000,
      },
    },
    
    // API testing project for essential backend checks
    {
      name: 'api-smoke',
      testMatch: '**/api-tests.spec.ts',
      use: {
        baseURL: 'http://localhost:8080',
        extraHTTPHeaders: {
          'Content-Type': 'application/json',
        },
        // API tests should be faster
        navigationTimeout: 10000,
        actionTimeout: 5000,
      }
    }
  ],

  // Global setup and teardown
  globalSetup: './tests/e2e/global-setup.ts',
  globalTeardown: './tests/e2e/global-teardown.ts',

  // Output directory for test artifacts
  outputDir: 'test-results/smoke/',

  // Test expectations optimized for speed
  expect: {
    // Shorter timeout for smoke tests
    timeout: 5 * 1000,
  },
})