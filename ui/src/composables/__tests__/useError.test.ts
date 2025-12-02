import { describe, it, expect, beforeEach } from 'vitest'
import { useError } from '../useError'
import type { AxiosError } from 'axios'

describe('useError', () => {
  beforeEach(() => {
    // Reset between tests
  })

  describe('Basic State Management', () => {
    it('initializes with no error', () => {
      const { error, hasError } = useError()
      expect(error.value).toBeNull()
      expect(hasError.value).toBe(false)
    })

    it('sets error state', () => {
      const { error, hasError, setError } = useError()
      setError('Test error')
      expect(error.value).toBeTruthy()
      expect(hasError.value).toBe(true)
    })

    it('clears error state', () => {
      const { error, hasError, setError, clearError } = useError()
      setError('Test error')
      expect(hasError.value).toBe(true)
      clearError()
      expect(error.value).toBeNull()
      expect(hasError.value).toBe(false)
    })
  })

  describe('String Error Normalization', () => {
    it('normalizes string errors', () => {
      const { error, setError } = useError()
      setError('Simple error message')

      expect(error.value).toEqual({
        code: 'UNKNOWN',
        message: 'Simple error message',
        context: undefined,
        suggestions: ['Try refreshing the page'],
        retryable: false
      })
    })

    it('normalizes string errors with context', () => {
      const { error, setError } = useError()
      setError('Failed to save', { action: 'Saving device', resource: 'Device', resourceId: 123 })

      expect(error.value?.message).toBe('Failed to save')
      expect(error.value?.context).toEqual({
        action: 'Saving device',
        resource: 'Device',
        resourceId: 123
      })
    })
  })

  describe('Error Object Normalization', () => {
    it('normalizes Error objects', () => {
      const { error, setError } = useError()
      const err = new Error('Something went wrong')
      setError(err)

      expect(error.value?.code).toBe('UNKNOWN')
      expect(error.value?.message).toBe('Something went wrong')
      expect(error.value?.retryable).toBe(false)
    })

    it('preserves context with Error objects', () => {
      const { error, setError } = useError()
      const err = new Error('Failed')
      setError(err, { action: 'Loading data' })

      expect(error.value?.context?.action).toBe('Loading data')
    })
  })

  describe('Axios Error Normalization', () => {
    it('normalizes axios errors with backend error structure', () => {
      const { error, setError } = useError()
      const axiosErr = {
        isAxiosError: true,
        message: 'Request failed',
        response: {
          status: 404,
          data: {
            error: {
              code: 'DEVICE_NOT_FOUND',
              message: 'Device not found',
              details: 'No device with ID 123'
            }
          }
        }
      } as unknown as AxiosError

      setError(axiosErr)

      expect(error.value?.code).toBe('DEVICE_NOT_FOUND')
      expect(error.value?.message).toBe('Device not found')
      expect(error.value?.details).toBe('No device with ID 123')
    })

    it('normalizes axios errors without backend structure using status code', () => {
      const { error, setError } = useError()
      const axiosErr = {
        isAxiosError: true,
        message: 'Request failed with status code 401',
        response: {
          status: 401,
          data: {}
        }
      } as unknown as AxiosError

      setError(axiosErr)

      expect(error.value?.code).toBe('UNAUTHORIZED')
    })

    it('maps common HTTP status codes correctly', () => {
      const testCases = [
        { status: 400, expectedCode: 'VALIDATION_FAILED' },
        { status: 401, expectedCode: 'UNAUTHORIZED' },
        { status: 403, expectedCode: 'PERMISSION_DENIED' },
        { status: 404, expectedCode: 'NOT_FOUND' },
        { status: 409, expectedCode: 'CONFLICT' },
        { status: 429, expectedCode: 'RATE_LIMIT_EXCEEDED' },
        { status: 500, expectedCode: 'INTERNAL_ERROR' },
        { status: 503, expectedCode: 'SERVICE_UNAVAILABLE' }
      ]

      testCases.forEach(({ status, expectedCode }) => {
        const { error, setError } = useError()
        const axiosErr = {
          isAxiosError: true,
          message: `Error ${status}`,
          response: {
            status,
            data: {}
          }
        } as unknown as AxiosError

        setError(axiosErr)
        expect(error.value?.code).toBe(expectedCode)
      })
    })

    it('handles network errors without response', () => {
      const { error, setError } = useError()
      const axiosErr = {
        isAxiosError: true,
        message: 'Network Error',
        code: 'ERR_NETWORK'
      } as unknown as AxiosError

      setError(axiosErr)

      expect(error.value?.code).toBe('NETWORK_ERROR')
    })

    it('handles timeout errors', () => {
      const { error, setError } = useError()
      const axiosErr = {
        isAxiosError: true,
        message: 'timeout of 5000ms exceeded',
        code: 'ECONNABORTED'
      } as unknown as AxiosError

      setError(axiosErr)

      expect(error.value?.code).toBe('TIMEOUT')
      expect(error.value?.retryable).toBe(true)
    })
  })

  describe('Error Suggestions', () => {
    it('provides suggestions for known error codes', () => {
      const { error, setError } = useError()
      const axiosErr = {
        isAxiosError: true,
        response: {
          status: 404,
          data: {
            error: {
              code: 'DEVICE_NOT_FOUND'
            }
          }
        }
      } as unknown as AxiosError

      setError(axiosErr)

      expect(error.value?.suggestions).toBeDefined()
      expect(error.value?.suggestions!.length).toBeGreaterThan(0)
    })

    it('provides default suggestions for unknown errors', () => {
      const { error, setError } = useError()
      setError('Unknown error')

      expect(error.value?.suggestions).toContain('Try refreshing the page')
    })
  })

  describe('Retry Detection', () => {
    it('marks retryable errors correctly', () => {
      const retryableCodes = [
        'NETWORK_ERROR',
        'TIMEOUT',
        'RATE_LIMIT_EXCEEDED',
        'SERVICE_UNAVAILABLE',
        'DEVICE_OFFLINE',
        'DEVICE_UNREACHABLE'
      ]

      retryableCodes.forEach(code => {
        const { error, setError } = useError()
        const axiosErr = {
          isAxiosError: true,
          response: {
            status: 503,
            data: {
              error: { code }
            }
          }
        } as unknown as AxiosError

        setError(axiosErr)
        expect(error.value?.retryable).toBe(true)
      })
    })

    it('marks non-retryable errors correctly', () => {
      const { error, setError } = useError()
      const axiosErr = {
        isAxiosError: true,
        response: {
          status: 400,
          data: {
            error: {
              code: 'VALIDATION_FAILED'
            }
          }
        }
      } as unknown as AxiosError

      setError(axiosErr)

      expect(error.value?.retryable).toBe(false)
    })
  })

  describe('Context Preservation', () => {
    it('preserves context across different error types', () => {
      const { error, setError } = useError()
      const context = {
        action: 'Updating device',
        resource: 'Device',
        resourceId: 'shelly-123'
      }

      // Test with string error
      setError('Error occurred', context)
      expect(error.value?.context).toEqual(context)

      // Test with Error object
      setError(new Error('Error occurred'), context)
      expect(error.value?.context).toEqual(context)

      // Test with axios error
      const axiosErr = {
        isAxiosError: true,
        message: 'Request failed',
        response: {
          status: 500,
          data: {}
        }
      } as unknown as AxiosError

      setError(axiosErr, context)
      expect(error.value?.context).toEqual(context)
    })
  })

  describe('Unknown Error Types', () => {
    it('handles null errors', () => {
      const { error, setError } = useError()
      setError(null)

      expect(error.value?.code).toBe('UNKNOWN')
      expect(error.value?.message).toBe('An unexpected error occurred')
    })

    it('handles undefined errors', () => {
      const { error, setError } = useError()
      setError(undefined)

      expect(error.value?.code).toBe('UNKNOWN')
      expect(error.value?.message).toBe('An unexpected error occurred')
    })

    it('handles object errors', () => {
      const { error, setError } = useError()
      setError({ custom: 'error object' })

      expect(error.value?.code).toBe('UNKNOWN')
      expect(error.value?.details).toContain('object')
    })
  })

  describe('Complex Scenarios', () => {
    it('handles complete axios error with all fields', () => {
      const { error, setError } = useError()
      const axiosErr = {
        isAxiosError: true,
        message: 'Request failed',
        response: {
          status: 503,
          data: {
            error: {
              code: 'SERVICE_UNAVAILABLE',
              message: 'Service temporarily unavailable',
              details: 'Database connection pool exhausted'
            }
          }
        }
      } as unknown as AxiosError

      const context = {
        action: 'Fetching devices',
        resource: 'DeviceList'
      }

      setError(axiosErr, context)

      expect(error.value).toMatchObject({
        code: 'SERVICE_UNAVAILABLE',
        message: 'Service temporarily unavailable',
        details: 'Database connection pool exhausted',
        context,
        retryable: true
      })
      expect(error.value?.suggestions).toBeDefined()
    })

    it('handles sequential errors correctly', () => {
      const { error, setError, clearError } = useError()

      // First error
      setError('First error')
      expect(error.value?.message).toBe('First error')

      // Second error (replaces first)
      setError('Second error')
      expect(error.value?.message).toBe('Second error')

      // Clear and set third error
      clearError()
      expect(error.value).toBeNull()

      setError('Third error')
      expect(error.value?.message).toBe('Third error')
    })
  })
})
