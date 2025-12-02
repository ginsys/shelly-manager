import { ref, computed } from 'vue'
import type { AxiosError } from 'axios'
import type { AppError, ErrorContext } from '@/types/errors'
import { getErrorMessage, getErrorSuggestions, isErrorRetryable } from '@/types/errors'

export function useError() {
  const error = ref<AppError | null>(null)
  const hasError = computed(() => error.value !== null)

  function setError(err: unknown, context?: ErrorContext) {
    error.value = normalizeError(err, context)
  }

  function clearError() {
    error.value = null
  }

  function normalizeError(err: unknown, context?: ErrorContext): AppError {
    // Handle axios errors with backend error structure
    if (isAxiosError(err)) {
      const backendError = err.response?.data?.error

      if (backendError) {
        // Backend returned structured error
        const code = backendError.code || 'UNKNOWN'
        return {
          code,
          message: backendError.message || getErrorMessage(code),
          details: backendError.details,
          context,
          suggestions: getErrorSuggestions(code),
          retryable: isErrorRetryable(code)
        }
      }

      // Network or HTTP error without backend structure
      const code = getHTTPErrorCode(err)
      return {
        code,
        message: getErrorMessage(code),
        details: err.message,
        context,
        suggestions: getErrorSuggestions(code),
        retryable: isErrorRetryable(code)
      }
    }

    // Handle Error objects
    if (err instanceof Error) {
      return {
        code: 'UNKNOWN',
        message: err.message,
        context,
        suggestions: ['Try refreshing the page', 'Contact support if the problem persists'],
        retryable: false
      }
    }

    // Handle string errors
    if (typeof err === 'string') {
      return {
        code: 'UNKNOWN',
        message: err,
        context,
        suggestions: ['Try refreshing the page'],
        retryable: false
      }
    }

    // Unknown error type
    return {
      code: 'UNKNOWN',
      message: 'An unexpected error occurred',
      details: String(err),
      context,
      suggestions: ['Try refreshing the page', 'Check the console for more details'],
      retryable: false
    }
  }

  return { error, hasError, setError, clearError }
}

// Type guard for axios errors
function isAxiosError(err: unknown): err is AxiosError {
  return (err as any)?.isAxiosError === true
}

// Map HTTP status codes to error codes
function getHTTPErrorCode(err: AxiosError): string {
  const status = err.response?.status

  if (!status) {
    return err.code === 'ECONNABORTED' ? 'TIMEOUT' : 'NETWORK_ERROR'
  }

  const codeMap: Record<number, string> = {
    400: 'VALIDATION_FAILED',
    401: 'UNAUTHORIZED',
    403: 'PERMISSION_DENIED',
    404: 'NOT_FOUND',
    409: 'CONFLICT',
    429: 'RATE_LIMIT_EXCEEDED',
    500: 'INTERNAL_ERROR',
    503: 'SERVICE_UNAVAILABLE'
  }

  return codeMap[status] || 'UNKNOWN'
}
