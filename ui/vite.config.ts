import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'node:path'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src'),
    },
  },
  server: {
    port: 5173,
    strictPort: false,
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
  preview: {
    port: 4173,
  },
})

