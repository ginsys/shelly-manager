import { computed, ref } from 'vue'
import type { AppError, ErrorContext } from '@/types/errors'
import { ERROR_MESSAGES } from '@/types/errors'

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
    // Axios error (duck-typing to avoid importing axios types)
    const maybeAxios = err as any
    if (maybeAxios && (maybeAxios.isAxiosError || maybeAxios.response || maybeAxios.request)) {
      const apiError = maybeAxios.appError || maybeAxios.response?.data?.error
      const code = apiError?.code || 'NETWORK_ERROR'
      const message = apiError?.message || ERROR_MESSAGES[code] || ERROR_MESSAGES.UNKNOWN
      const details = apiError?.details || (maybeAxios.message ?? '')
      const retryable = [429, 502, 503, 504].includes(maybeAxios.response?.status)
      return { code, message, details, context, suggestions: getSuggestions(code), retryable, severity: retryable ? 'warning' : 'error' }
    }
    // Plain Error or string
    if (err instanceof Error) {
      return { code: 'UNKNOWN', message: err.message, context }
    }
    if (typeof err === 'string') {
      return { code: 'UNKNOWN', message: err, context }
    }
    return { code: 'UNKNOWN', message: ERROR_MESSAGES.UNKNOWN, context }
  }

  function getSuggestions(code: string): string[] {
    switch (code) {
      case 'UNAUTHORIZED':
        return ['Verify your admin API key', 'Sign in again if needed']
      case 'DEVICE_OFFLINE':
        return ['Check device power and network', 'Try again shortly']
      case 'NETWORK_ERROR':
        return ['Check your internet connection', 'Retry the request']
      case 'RATE_LIMIT_EXCEEDED':
        return ['Wait a moment and try again']
      default:
        return []
    }
  }

  return { error, hasError, setError, clearError, normalizeError }
}

