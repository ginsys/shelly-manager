import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
import path from 'node:path'

export default defineConfig({
  plugins: [vue()],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/__tests__/setup.ts'],
    exclude: [
      '**/node_modules/**',
      '**/dist/**',
      '**/tests/e2e/**',        // Playwright E2E tests
      '**/tests/fixtures/**',   // Playwright test fixtures
      '**/.playwright/**',      // Playwright cache
      '**/playwright-report/**', // Playwright reports
    ],
    coverage: {
      enabled: true,
      provider: 'v8',
      reportsDirectory: 'coverage',
      reporter: ['text', 'lcov', 'html'],
      exclude: [
        '**/node_modules/**',
        '**/dist/**',
        '**/playwright-report/**',
        '**/tests/e2e/**',
        'playwright*.ts',
        'validate-navigation.js',
        'debug-brand-visibility.cjs',
        'src/App.vue',
        'src/main.ts',
        'src/layouts/**',
        'src/components/charts/**',
      ],
    },
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src'),
    },
  },
})
