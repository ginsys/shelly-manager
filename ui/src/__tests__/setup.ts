import { vi } from 'vitest'

// Mock axios for all tests
vi.mock('axios', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    patch: vi.fn(),
    delete: vi.fn(),
    create: vi.fn(() => ({
      get: vi.fn(),
      post: vi.fn(),
      put: vi.fn(),
      patch: vi.fn(),
      delete: vi.fn(),
      interceptors: {
        request: { use: vi.fn() },
        response: { use: vi.fn() }
      }
    }))
  }
}))

// Mock browser APIs
Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: vi.fn().mockImplementation(query => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: vi.fn(), // Deprecated
    removeListener: vi.fn(), // Deprecated
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })),
})

// Mock URL methods for file export/import tests
Object.defineProperty(URL, 'createObjectURL', {
  writable: true,
  value: vi.fn(() => 'blob:mock-url')
})

Object.defineProperty(URL, 'revokeObjectURL', {
  writable: true,
  value: vi.fn()
})

// Mock document methods
const originalCreateElement = document.createElement.bind(document)
Object.defineProperty(document, 'createElement', {
  writable: true,
  value: vi.fn((tagName: string) => {
    // For simple cases (like link elements for download), return a mock
    if (tagName === 'a') {
      const mockElement = {
        href: '',
        download: '',
        click: vi.fn(),
        appendChild: vi.fn(),
        removeChild: vi.fn(),
        setAttribute: vi.fn(),
        getAttribute: vi.fn(),
        removeAttribute: vi.fn()
      }
      return mockElement
    }
    // For everything else, use real createElement to avoid attribute issues
    return originalCreateElement(tagName)
  })
})

Object.defineProperty(document.body, 'appendChild', {
  writable: true,
  value: vi.fn()
})

Object.defineProperty(document.body, 'removeChild', {
  writable: true,
  value: vi.fn()
})