/**
 * Test utilities for Vue component testing
 * Provides common patterns for mounting components with plugins and mocking
 */

import { mount, VueWrapper } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import { vi } from 'vitest'
import { createRouter, createMemoryHistory, Router } from 'vue-router'
import type { Component } from 'vue'

/**
 * Create a test router with memory history
 */
export function createTestRouter(): Router {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', name: 'home', component: { template: '<div>Home</div>' } },
      { path: '/devices', name: 'devices', component: { template: '<div>Devices</div>' } },
      { path: '/metrics', name: 'metrics', component: { template: '<div>Metrics</div>' } }
    ]
  })
}

/**
 * Mount a component with all necessary plugins (Pinia, Router)
 */
export function mountWithPlugins(component: Component, options: any = {}) {
  const router = options.router || createTestRouter()

  return mount(component, {
    global: {
      plugins: [
        createTestingPinia({
          stubActions: false,
          initialState: options.initialState || {}
        }),
        router
      ],
      stubs: {
        'router-link': true,
        'router-view': true,
        ...options.stubs
      }
    },
    ...options
  })
}

/**
 * Mock an API module with specific methods
 */
export function mockApiModule(module: string, methods: Record<string, any>) {
  vi.mock(`@/api/${module}`, () => methods)
}

/**
 * Wait for all promises and Vue updates to complete
 */
export async function flushPromises() {
  return new Promise((resolve) => {
    setTimeout(resolve, 0)
  })
}

/**
 * Simulate a delay (useful for testing loading states)
 */
export function delay(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

/**
 * Create a mock API response
 */
export function mockApiResponse<T>(data: T, delay = 0) {
  return vi.fn().mockImplementation(() =>
    new Promise((resolve) => {
      setTimeout(() => resolve(data), delay)
    })
  )
}

/**
 * Create a mock API error
 */
export function mockApiError(message: string, code?: string, delay = 0) {
  return vi.fn().mockImplementation(() =>
    new Promise((_, reject) => {
      setTimeout(() => reject({
        message,
        response: {
          data: {
            error: { code, message }
          }
        }
      }), delay)
    })
  )
}
