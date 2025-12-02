export interface AppError {
  code: string
  message: string
  details?: string
  context?: {
    action: string
    resource?: string
    resourceId?: string | number
  }
  suggestions?: string[]
  retryable?: boolean
}

export interface ErrorContext {
  action: string
  resource?: string
  resourceId?: string | number
}

// Error code to user-friendly message mapping
export const ERROR_MESSAGES: Record<string, string> = {
  // Device errors
  'DEVICE_NOT_FOUND': 'The device could not be found. It may have been deleted.',
  'DEVICE_OFFLINE': 'The device is currently offline. Check network connectivity.',
  'DEVICE_UNREACHABLE': 'Unable to connect to the device. Verify the IP address and network.',

  // Authentication errors
  'UNAUTHORIZED': 'You are not authorized. Please check your API key.',
  'INVALID_CREDENTIALS': 'Invalid credentials provided.',
  'SESSION_EXPIRED': 'Your session has expired. Please refresh the page.',

  // Validation errors
  'VALIDATION_FAILED': 'The input data is invalid. Please check your entries.',
  'INVALID_FORMAT': 'The data format is incorrect.',
  'MISSING_REQUIRED_FIELD': 'Required fields are missing.',

  // Network errors
  'NETWORK_ERROR': 'Network connection failed. Check your internet connection.',
  'TIMEOUT': 'The request timed out. Please try again.',
  'RATE_LIMIT_EXCEEDED': 'Too many requests. Please wait a moment and try again.',

  // Resource errors
  'NOT_FOUND': 'The requested resource was not found.',
  'ALREADY_EXISTS': 'A resource with this name already exists.',
  'CONFLICT': 'The operation conflicts with the current state.',

  // Server errors
  'INTERNAL_ERROR': 'An internal server error occurred. Please try again later.',
  'SERVICE_UNAVAILABLE': 'The service is temporarily unavailable.',
  'DATABASE_ERROR': 'A database error occurred.',

  // Operation errors
  'OPERATION_FAILED': 'The operation failed to complete.',
  'PERMISSION_DENIED': 'You do not have permission to perform this action.',
  'QUOTA_EXCEEDED': 'Resource quota has been exceeded.',

  // Default
  'UNKNOWN': 'An unknown error occurred.'
}

// Get suggestions based on error code
export function getErrorSuggestions(code: string): string[] {
  const suggestions: Record<string, string[]> = {
    'DEVICE_NOT_FOUND': [
      'Refresh the devices list to see current devices',
      'Check if the device was recently deleted'
    ],
    'DEVICE_OFFLINE': [
      'Verify the device is powered on',
      'Check network connectivity',
      'Ensure the device is on the same network'
    ],
    'UNAUTHORIZED': [
      'Verify your API key is correct',
      'Check if the key has the necessary permissions',
      'Refresh your browser to re-authenticate'
    ],
    'VALIDATION_FAILED': [
      'Review all input fields for errors',
      'Ensure required fields are filled',
      'Check data formats (IP addresses, URLs, etc.)'
    ],
    'RATE_LIMIT_EXCEEDED': [
      'Wait a few moments before trying again',
      'Reduce the frequency of requests'
    ],
    'NETWORK_ERROR': [
      'Check your internet connection',
      'Verify the server is reachable',
      'Try refreshing the page'
    ],
    'TIMEOUT': [
      'The operation may take longer than expected',
      'Try again in a few moments',
      'Check if the device/server is responding'
    ]
  }

  return suggestions[code] || [
    'Try refreshing the page',
    'Check the console for more details',
    'Contact support if the problem persists'
  ]
}

// Determine if error is retryable
export function isErrorRetryable(code: string): boolean {
  const retryableCodes = [
    'NETWORK_ERROR',
    'TIMEOUT',
    'RATE_LIMIT_EXCEEDED',
    'SERVICE_UNAVAILABLE',
    'DEVICE_OFFLINE',
    'DEVICE_UNREACHABLE'
  ]
  return retryableCodes.includes(code)
}

// Get user-friendly error message
export function getErrorMessage(code: string): string {
  return ERROR_MESSAGES[code] || ERROR_MESSAGES['UNKNOWN']
}
