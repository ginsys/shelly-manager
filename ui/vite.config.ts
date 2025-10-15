import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'node:path'
import { execSync } from 'node:child_process'

// https://vitejs.dev/config/
export default defineConfig({
  define: (() => {
    let gitSha = process.env.GIT_SHA || 'dev'
    try { if (!process.env.GIT_SHA) gitSha = execSync('git rev-parse --short HEAD').toString().trim() } catch {}
    const buildTime = new Date().toISOString()
    return {
      __UI_BUILD__: JSON.stringify({ git_sha: gitSha, build_time: buildTime })
    }
  })(),
  plugins: [vue()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src'),
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          // Core Vue ecosystem
          'vendor-vue': ['vue', 'vue-router', 'pinia'],

          // Large charting library - separate chunk
          'charts': ['echarts'],

          // Utility libraries
          'vendor-utils': ['axios', 'pako', 'crypto-browserify'],

          // UI framework
          'vendor-ui': ['quasar'],
        }
      },

      // External dependencies for development (CDN)
      external: process.env.NODE_ENV === 'development' ? ['echarts'] : []
    },

    // Stricter size limits
    chunkSizeWarningLimit: 300,  // Down from 500KB default

    // Enhanced minification
    minify: 'terser',
    terserOptions: {
      compress: {
        drop_console: true,      // Remove console.log
        drop_debugger: true,     // Remove debugger statements
        pure_funcs: ['console.log', 'console.warn'], // Remove specific calls
      },
      mangle: {
        safari10: true,          // Safari compatibility
      },
    },

    // Source map optimization
    sourcemap: process.env.NODE_ENV === 'development' ? true : false,
  },

  // Development server optimizations
  server: {
    port: 5173,
    strictPort: false,
    warmup: {
      clientFiles: ['./src/main.ts', './src/App.vue'],
    },
    proxy: {
      // Proxy API to Go backend for dev
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      // Let the Vite dev server fetch runtime config from Go server
      '/app-config.js': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      // Optional: metrics endpoints
      '/metrics': {
        target: 'http://localhost:8080',
        ws: true,
        changeOrigin: true,
      },
    },
  },

  // Dependency optimization
  optimizeDeps: {
    include: ['vue', 'vue-router', 'pinia', 'axios'],
    exclude: ['echarts'], // Large library - load separately
  },
  preview: {
    port: 5173,  // FIXED: Match Playwright expected port
    strictPort: false,
    // Enable CORS for E2E testing
    cors: true,
  },
})
