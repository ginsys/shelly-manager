import { defineConfig, devices } from '@playwright/test'

/**
 * @see https://playwright.dev/docs/test-configuration
 */
export default defineConfig({
  testDir: './tests/e2e',
  
  // Run tests in files in parallel - disabled to respect worker limits
  fullyParallel: false,
  
  // Fail the build on CI if you accidentally left test.only in the source code
  forbidOnly: !!process.env.CI,
  
  // Retry on CI only - reduced to 1 for faster execution
  retries: process.env.CI ? 1 : 0,
  
  // FORCE 2 workers to prevent SQLite concurrency issues - NUCLEAR APPROACH
  // OVERRIDE: Always force 2 workers regardless of system defaults or CLI overrides
  workers: 2,
  
  // Reporter to use
  reporter: [
    ['html', { open: 'never' }],
    ['json', { outputFile: 'test-results/results.json' }],
    process.env.CI ? ['github'] : ['list']
  ],
  
  // Increased global timeout for complex pages (was 30s)
  timeout: 120 * 1000,
  
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

  // Global metadata to override any dynamic worker detection
  metadata: {
    workers: 2,
    maxWorkers: 2,
    forceWorkers: true
  },

  // Test projects for different browsers and scenarios
  projects: [
    // Desktop browsers
    {
      name: 'chromium',
      use: { 
        ...devices['Desktop Chrome'],
        // Chromium-specific settings optimized for performance
        launchOptions: {
          args: [
            '--disable-dev-shm-usage',
            '--no-sandbox', 
            '--disable-gpu',
            '--disable-web-security',
            '--disable-features=VizDisplayCompositor'
          ],
        },
        // Add consistent timeouts for Chromium
        navigationTimeout: 60000,  // Increased from 45s
        actionTimeout: 20000,      // Increased from 15s
      },
    },
    {
      name: 'firefox',
      timeout: 120 * 1000, // Firefox-specific test timeout: 120s for better reliability
      use: {
        ...devices['Desktop Firefox'],
        launchOptions: {
          firefoxUserPrefs: {
            // Network optimizations - CRITICAL for timeout fix
            'network.http.max-connections-per-server': 32,
            'network.http.max-persistent-connections-per-server': 16,
            'network.http.response.timeout': 300000,  // 5min vs 45.1s hardcoded
            'network.http.request.timeout': 300000,   // 5min request timeout

            // Performance optimizations
            'browser.cache.disk.enable': false,       // No disk cache for tests
            'browser.cache.memory.capacity': 102400,  // 100MB memory cache
            'dom.max_script_run_time': 0,            // No script timeout
            'dom.max_chrome_script_run_time': 0,     // No chrome script timeout

            // Disable unnecessary features
            'browser.safebrowsing.enabled': false,
            'browser.safebrowsing.malware.enabled': false,
            'extensions.update.enabled': false,
            'app.update.enabled': false,
          }
        },

        // Override global timeouts specifically for Firefox
        actionTimeout: 60000,        // 60s vs 45.1s system limit
        navigationTimeout: 60000,    // 60s navigation timeout
      },

      // Firefox-specific test configuration
      retries: process.env.CI ? 2 : 1,  // Retry on CI
    },
    {
      name: 'webkit',
      use: { 
        ...devices['Desktop Safari'],
        // WebKit-specific settings
        launchOptions: {
          args: ['--disable-web-security'],
        },
        // Increased timeouts for WebKit stability
        navigationTimeout: 60000,
        actionTimeout: 25000,
      },
    },
    
    // Mobile browsers for responsive testing
    {
      name: 'Mobile Chrome',
      use: { 
        ...devices['Pixel 5'],
        // Mobile Chrome settings - increased timeout
        actionTimeout: 30000,
        navigationTimeout: 60000,
      },
    },
    {
      name: 'Mobile Safari',
      use: { 
        ...devices['iPhone 12'],
        // Mobile Safari specific settings - keep longer for slower device
        navigationTimeout: 60000,
        actionTimeout: 35000,
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