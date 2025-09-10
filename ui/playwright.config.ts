import { defineConfig, devices } from '@playwright/test'

/**
 * @see https://playwright.dev/docs/test-configuration
 */
export default defineConfig({
  testDir: './tests/e2e',
  
  // Run tests in files in parallel
  fullyParallel: true,
  
  // Fail the build on CI if you accidentally left test.only in the source code
  forbidOnly: !!process.env.CI,
  
  // Retry on CI only
  retries: process.env.CI ? 2 : 0,
  
  // Opt out of parallel tests on CI
  workers: process.env.CI ? 1 : undefined,
  
  // Reporter to use
  reporter: [
    ['html', { open: 'never' }],
    ['json', { outputFile: 'test-results/results.json' }],
    process.env.CI ? ['github'] : ['list']
  ],
  
  // Global test timeout
  timeout: 30 * 1000,
  
  // Shared settings for all tests
  use: {
    // Base URL for the application
    baseURL: process.env.CI ? 'http://localhost:5173' : 'http://localhost:5173',
    
    // API endpoint for backend tests
    extraHTTPHeaders: {
      'Accept': 'application/json',
      'User-Agent': 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36'
    },
    
    // Collect trace when retrying the failed test
    trace: 'on-first-retry',
    
    // Capture screenshot on failure
    screenshot: 'only-on-failure',
    
    // Capture video on failure
    video: 'retain-on-failure',
  },

  // Test projects for different browsers and scenarios
  projects: [
    // Desktop browsers
    {
      name: 'chromium',
      use: { 
        ...devices['Desktop Chrome'],
        // Chromium-specific settings
        launchOptions: {
          args: ['--disable-dev-shm-usage', '--no-sandbox'],
        },
      },
    },
    {
      name: 'firefox',
      use: { 
        ...devices['Desktop Firefox'],
        // Firefox-specific settings
        launchOptions: {
          firefoxUserPrefs: {
            'network.http.speculative-parallel-limit': 0,
            'network.dns.disableIPv6': true,
          },
        },
        // Longer timeouts for Firefox navigation issues
        navigationTimeout: 60000,
        actionTimeout: 30000,
      },
    },
    {
      name: 'webkit',
      use: { 
        ...devices['Desktop Safari'],
        // WebKit-specific settings
        launchOptions: {
          args: ['--disable-web-security'],
        },
        // Extra timeouts for WebKit rendering
        navigationTimeout: 60000,
        actionTimeout: 30000,
      },
    },
    
    // Mobile browsers for responsive testing
    {
      name: 'Mobile Chrome',
      use: { 
        ...devices['Pixel 5'],
        // Mobile Chrome settings
        actionTimeout: 20000,
      },
    },
    {
      name: 'Mobile Safari',
      use: { 
        ...devices['iPhone 12'],
        // Mobile Safari specific settings
        navigationTimeout: 60000,
        actionTimeout: 30000,
      },
    },
    
    // API testing project
    {
      name: 'api-tests',
      testMatch: '**/*api*.spec.ts',
      use: {
        baseURL: 'http://localhost:8080',
        extraHTTPHeaders: {
          'Content-Type': 'application/json',
        }
      }
    }
  ],

  // Development server configuration (disabled - using external services)
  // webServer: undefined,

  // Global setup and teardown
  globalSetup: './tests/e2e/global-setup.ts',
  globalTeardown: './tests/e2e/global-teardown.ts',

  // Output directory for test artifacts
  outputDir: 'test-results/',

  // Test expectations
  expect: {
    // Maximum time expect() should wait for the condition to be met
    timeout: 10 * 1000,
    
    // Take screenshot when assertion fails
    toHaveScreenshot: {
      mode: 'only-on-failure',
    },
  },
})