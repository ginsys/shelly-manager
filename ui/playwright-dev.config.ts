import { defineConfig, devices } from '@playwright/test'

/**
 * Development/CI focused Playwright configuration
 * Single browser (Chromium) for 10-15 minute test execution target
 * @see https://playwright.dev/docs/test-configuration
 */
export default defineConfig({
  testDir: './tests/e2e',

  // Run tests in files in parallel for maximum speed
  fullyParallel: true,

  // Fail the build on CI if you accidentally left test.only in the source code
  forbidOnly: !!process.env.CI,

  // No retries for development speed - CI can use 1 retry
  retries: process.env.CI ? 1 : 0,

  // Optimize workers for single browser: use more workers for speed
  workers: process.env.CI ? 4 : 6,

  // Reporter optimized for development
  reporter: [
    ['list'],
    ['json', { outputFile: 'test-results/dev-results.json' }],
    process.env.CI ? ['github'] : ['line']
  ],

  // Reduced timeout for faster feedback
  timeout: 60 * 1000, // 60s vs 120s for development speed

  // Global setup and teardown
  globalSetup: './tests/e2e/global-setup.ts',
  globalTeardown: './tests/e2e/global-teardown.ts',

  // Shared settings optimized for development
  use: {
    // Base URL for the application
    baseURL: 'http://localhost:5173',

    // Optimized HTTP headers for API tests
    extraHTTPHeaders: {
      'Accept': 'application/json',
      'User-Agent': 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36'
    },

    // Minimal tracing for development speed
    trace: 'retain-on-failure',

    // Screenshots only on failure to save time
    screenshot: 'only-on-failure',

    // No video capture for development speed
    video: 'off',

    // Faster navigation timeouts
    navigationTimeout: 30000,  // 30s for development
    actionTimeout: 15000,      // 15s for development
  },

  // Single browser project: Chromium only for development/CI speed
  projects: [
    {
      name: 'chromium-dev',
      use: {
        ...devices['Desktop Chrome'],
        // Chromium optimized for maximum performance
        launchOptions: {
          args: [
            '--disable-dev-shm-usage',
            '--no-sandbox',
            '--disable-gpu',
            '--disable-web-security',
            '--disable-features=VizDisplayCompositor',
            '--disable-background-timer-throttling',
            '--disable-backgrounding-occluded-windows',
            '--disable-renderer-backgrounding',
            '--disable-ipc-flooding-protection',
            '--memory-pressure-off',
          ],
        },
        // Aggressive timeouts for development speed
        navigationTimeout: 30000,
        actionTimeout: 15000,
      },
    },
  ],

  // Configure web server for development mode
  webServer: {
    command: 'npm run preview',
    port: 5173,
    timeout: 30 * 1000,
    reuseExistingServer: !process.env.CI,
  },
});