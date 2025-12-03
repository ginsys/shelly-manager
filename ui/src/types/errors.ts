export interface ErrorContext {
  action: string
  resource?: string
  resourceId?: string | number
}

export interface AppError {
  code: string
  message: string
  details?: string
  context?: ErrorContext
  suggestions?: string[]
  retryable?: boolean
  severity?: 'info' | 'warning' | 'error'
}

export const ERROR_MESSAGES: Record<string, string> = {
  DEVICE_NOT_FOUND: 'The device could not be found. It may have been deleted.',
  DEVICE_OFFLINE: 'The device is currently offline. Check network connectivity.',
  VALIDATION_FAILED: 'The input data is invalid. Please check your entries.',
  UNAUTHORIZED: 'You are not authorized. Please check your API key.',
  RATE_LIMIT_EXCEEDED: 'Too many requests. Please wait and try again.',
  NETWORK_ERROR: 'A network error occurred. Check your connection.',
  UNKNOWN: 'An unexpected error occurred.',
}

