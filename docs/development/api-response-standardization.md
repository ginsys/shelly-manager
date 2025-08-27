# API Response Standardization and Error Handling

## Executive Summary

The Shelly Manager API implements a comprehensive response standardization system that ensures consistent, secure, and informative API responses across all endpoints. The system provides structured error handling, security-aware response formatting, and extensive error code taxonomy to support robust client-side error handling and debugging.

### Response System Overview

| Component | Purpose | Security Features | Performance Impact |
|-----------|---------|------------------|-------------------|
| Standardized Format | Consistent API responses | Information disclosure prevention | <1ms overhead |
| Error Code System | Precise error identification | Security-aware error messages | Negligible |
| Response Builder | Fluent response construction | Automatic security headers | <0.5ms |
| Metadata System | Rich response context | Safe metadata exposure | <0.2ms |

### Key Benefits

- **Consistency**: All API responses follow the same structure
- **Security**: Error responses don't expose sensitive internal information
- **Debugging**: Rich error context for legitimate debugging needs
- **Client Integration**: Predictable response format simplifies client code
- **Monitoring**: Structured responses enable better observability

## Standardized Response Format

### Core Response Structure

All API responses follow a consistent JSON structure:

```json
{
  "success": true|false,
  "data": {...}|null,
  "error": {...}|null,
  "meta": {...}|null,
  "timestamp": "2024-01-15T10:30:00Z",
  "request_id": "req_abc123xyz"
}
```

#### Response Schema Definition
```go
type APIResponse struct {
    Success   bool        `json:"success"`            // Operation success indicator
    Data      interface{} `json:"data,omitempty"`     // Response payload (success only)
    Error     *APIError   `json:"error,omitempty"`    // Error details (failure only)
    Meta      *Metadata   `json:"meta,omitempty"`     // Additional metadata
    Timestamp time.Time   `json:"timestamp"`          // Response generation time
    RequestID string      `json:"request_id,omitempty"` // Request correlation ID
}
```

### Successful Response Examples

#### Simple Success Response
```json
{
  "success": true,
  "data": {
    "id": "device_123",
    "name": "Living Room Switch",
    "status": "online",
    "type": "shelly_1"
  },
  "timestamp": "2024-01-15T10:30:00Z",
  "request_id": "req_abc123xyz"
}
```

#### Success Response with Metadata
```json
{
  "success": true,
  "data": [
    {"id": "device_1", "name": "Device 1"},
    {"id": "device_2", "name": "Device 2"}
  ],
  "meta": {
    "pagination": {
      "page": 1,
      "page_size": 20,
      "total_pages": 5,
      "has_next": true,
      "has_previous": false
    },
    "count": 2,
    "total_count": 89,
    "version": "v1"
  },
  "timestamp": "2024-01-15T10:30:00Z",
  "request_id": "req_def456uvw"
}
```

#### Success Response with Cache Information
```json
{
  "success": true,
  "data": {
    "device_status": "online",
    "last_seen": "2024-01-15T10:29:45Z"
  },
  "meta": {
    "cache": {
      "cached": true,
      "cached_at": "2024-01-15T10:29:50Z",
      "expires_at": "2024-01-15T10:34:50Z",
      "ttl_seconds": 300
    }
  },
  "timestamp": "2024-01-15T10:30:00Z",
  "request_id": "req_ghi789rst"
}
```

### Error Response Format

#### Error Structure Definition
```go
type APIError struct {
    Code       string      `json:"code"`               // Machine-readable error code
    Message    string      `json:"message"`            // Human-readable error message
    Details    interface{} `json:"details,omitempty"`  // Additional error context
    StatusCode int         `json:"-"`                  // HTTP status (internal use)
}
```

#### Simple Error Response
```json
{
  "success": false,
  "error": {
    "code": "DEVICE_NOT_FOUND",
    "message": "The specified device could not be found"
  },
  "timestamp": "2024-01-15T10:30:00Z",
  "request_id": "req_jkl012mno"
}
```

#### Detailed Error Response with Context
```json
{
  "success": false,
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "Request validation failed",
    "details": {
      "validation_errors": {
        "name": "Device name is required",
        "ip_address": "Invalid IP address format",
        "type": "Unsupported device type"
      },
      "received_data": {
        "name": "",
        "ip_address": "invalid-ip",
        "type": "unknown_device"
      }
    }
  },
  "timestamp": "2024-01-15T10:30:00Z",
  "request_id": "req_pqr345stu"
}
```

## Comprehensive Error Code System

### Error Code Categories

#### Client Errors (4xx HTTP Status Codes)
```go
const (
    // General client errors
    ErrCodeBadRequest          = "BAD_REQUEST"          // 400
    ErrCodeUnauthorized        = "UNAUTHORIZED"         // 401
    ErrCodeForbidden           = "FORBIDDEN"            // 403
    ErrCodeNotFound            = "NOT_FOUND"            // 404
    ErrCodeMethodNotAllowed    = "METHOD_NOT_ALLOWED"   // 405
    ErrCodeConflict            = "CONFLICT"             // 409
    ErrCodeValidationFailed    = "VALIDATION_FAILED"    // 400
    ErrCodeRateLimitExceeded   = "RATE_LIMIT_EXCEEDED"  // 429
    ErrCodeRequestTooLarge     = "REQUEST_TOO_LARGE"    // 413
    ErrCodeUnsupportedMedia    = "UNSUPPORTED_MEDIA_TYPE" // 415
)
```

#### Server Errors (5xx HTTP Status Codes)
```go
const (
    // General server errors
    ErrCodeInternalServer      = "INTERNAL_SERVER_ERROR"     // 500
    ErrCodeNotImplemented      = "NOT_IMPLEMENTED"           // 501
    ErrCodeServiceUnavailable  = "SERVICE_UNAVAILABLE"       // 503
    ErrCodeTimeout             = "REQUEST_TIMEOUT"           // 504
    ErrCodeDatabaseError       = "DATABASE_ERROR"            // 500
    ErrCodeExternalServiceError = "EXTERNAL_SERVICE_ERROR"   // 502
)
```

#### Application-Specific Error Codes
```go
const (
    // Device management errors
    ErrCodeDeviceNotFound      = "DEVICE_NOT_FOUND"
    ErrCodeDeviceOffline       = "DEVICE_OFFLINE"
    ErrCodeDeviceTimeout       = "DEVICE_TIMEOUT"
    ErrCodeDeviceUnavailable   = "DEVICE_UNAVAILABLE"
    ErrCodeInvalidDeviceType   = "INVALID_DEVICE_TYPE"
    
    // Configuration errors
    ErrCodeConfigurationError  = "CONFIGURATION_ERROR"
    ErrCodeInvalidConfiguration = "INVALID_CONFIGURATION"
    ErrCodeConfigurationConflict = "CONFIGURATION_CONFLICT"
    ErrCodeTemplateError       = "TEMPLATE_ERROR"
    ErrCodeTemplateNotFound    = "TEMPLATE_NOT_FOUND"
    
    // Provisioning errors
    ErrCodeProvisioningError   = "PROVISIONING_ERROR"
    ErrCodeProvisioningFailed  = "PROVISIONING_FAILED"
    ErrCodeProvisioningTimeout = "PROVISIONING_TIMEOUT"
    ErrCodeNetworkError        = "NETWORK_ERROR"
    
    // Monitoring and metrics errors
    ErrCodeMetricsError        = "METRICS_ERROR"
    ErrCodeMetricsUnavailable  = "METRICS_UNAVAILABLE"
    
    // Notification errors
    ErrCodeNotificationError   = "NOTIFICATION_ERROR"
    ErrCodeNotificationFailed  = "NOTIFICATION_FAILED"
    
    // Authentication and authorization errors
    ErrCodeInvalidCredentials  = "INVALID_CREDENTIALS"
    ErrCodeTokenExpired       = "TOKEN_EXPIRED"
    ErrCodeInvalidToken       = "INVALID_TOKEN"
    ErrCodePermissionDenied   = "PERMISSION_DENIED"
)
```

### Error Code Usage Examples

#### Device Management Errors
```json
// Device not found
{
  "success": false,
  "error": {
    "code": "DEVICE_NOT_FOUND",
    "message": "Device with ID 'device_123' could not be found",
    "details": {
      "device_id": "device_123",
      "searched_locations": ["database", "cache", "discovery_service"]
    }
  },
  "timestamp": "2024-01-15T10:30:00Z",
  "request_id": "req_device_search"
}

// Device offline
{
  "success": false,
  "error": {
    "code": "DEVICE_OFFLINE",
    "message": "Device is currently offline and cannot be controlled",
    "details": {
      "device_id": "device_456",
      "device_name": "Kitchen Switch",
      "last_seen": "2024-01-15T09:15:00Z",
      "offline_duration_minutes": 75
    }
  },
  "timestamp": "2024-01-15T10:30:00Z",
  "request_id": "req_device_control"
}
```

#### Validation Errors
```json
// Complex validation error with field-specific details
{
  "success": false,
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "Request validation failed",
    "details": {
      "validation_errors": {
        "device": {
          "name": {
            "code": "FIELD_REQUIRED",
            "message": "Device name is required"
          },
          "ip_address": {
            "code": "INVALID_FORMAT",
            "message": "IP address must be in valid IPv4 format",
            "received": "not-an-ip",
            "expected_format": "xxx.xxx.xxx.xxx"
          },
          "port": {
            "code": "OUT_OF_RANGE",
            "message": "Port must be between 1 and 65535",
            "received": 70000,
            "valid_range": {"min": 1, "max": 65535}
          }
        }
      },
      "error_count": 3,
      "validation_rules": "device_creation_v1"
    }
  },
  "timestamp": "2024-01-15T10:30:00Z",
  "request_id": "req_device_create"
}
```

#### Security-Related Errors
```json
// Rate limiting error with retry information
{
  "success": false,
  "error": {
    "code": "RATE_LIMIT_EXCEEDED",
    "message": "Too many requests. Please try again later.",
    "details": {
      "limit": 1000,
      "window_duration": "1h",
      "reset_time": "2024-01-15T11:30:00Z",
      "retry_after_seconds": 3600
    }
  },
  "meta": {
    "retry_after": 3600
  },
  "timestamp": "2024-01-15T10:30:00Z",
  "request_id": "req_rate_limited"
}

// IP blocked error (security-aware message)
{
  "success": false,
  "error": {
    "code": "IP_BLOCKED",
    "message": "Your IP has been temporarily blocked due to suspicious activity.",
    "details": {
      "block_reason": "Multiple security violations detected",
      "contact": "security@company.com"
    }
  },
  "timestamp": "2024-01-15T10:30:00Z"
}
```

## Response Builder System

### Fluent Response Construction

The ResponseBuilder provides a fluent interface for constructing standardized responses:

```go
type ResponseBuilder struct {
    response  *APIResponse
    logger    *logging.Logger
    requestID string
}

// Usage examples
func (h *Handler) GetDevice(w http.ResponseWriter, r *http.Request) {
    builder := response.NewResponseBuilder(h.logger)
    
    // Extract device ID from path
    deviceID := mux.Vars(r)["id"]
    
    // Get device from service
    device, err := h.service.GetDevice(deviceID)
    if err != nil {
        if errors.Is(err, service.ErrDeviceNotFound) {
            resp := builder.WithRequestID(getRequestID(r)).
                Error("DEVICE_NOT_FOUND", "Device not found", map[string]interface{}{
                    "device_id": deviceID,
                })
            writeJSONResponse(w, http.StatusNotFound, resp)
            return
        }
        
        resp := builder.WithRequestID(getRequestID(r)).
            Error("INTERNAL_SERVER_ERROR", "Failed to retrieve device", nil)
        writeJSONResponse(w, http.StatusInternalServerError, resp)
        return
    }
    
    // Success response
    resp := builder.WithRequestID(getRequestID(r)).Success(device)
    writeJSONResponse(w, http.StatusOK, resp)
}
```

### ResponseWriter Helper Methods

The ResponseWriter provides convenient methods for common response patterns:

```go
type ResponseWriter struct {
    logger *logging.Logger
}

// Common response patterns
func (rw *ResponseWriter) WriteSuccess(w http.ResponseWriter, r *http.Request, data interface{}) {
    // Write successful response with automatic request ID extraction
}

func (rw *ResponseWriter) WriteError(w http.ResponseWriter, r *http.Request, statusCode int, code, message string, details interface{}) {
    // Write error response with logging
}

func (rw *ResponseWriter) WriteValidationError(w http.ResponseWriter, r *http.Request, validationErrors interface{}) {
    // Write validation error with standardized format
}

func (rw *ResponseWriter) WriteNotFoundError(w http.ResponseWriter, r *http.Request, resource string) {
    // Write not found error with resource context
}

func (rw *ResponseWriter) WriteInternalError(w http.ResponseWriter, r *http.Request, err error) {
    // Write internal error without exposing sensitive details
}
```

#### Usage Examples

```go
// Success with metadata
func (h *Handler) ListDevices(w http.ResponseWriter, r *http.Request) {
    writer := response.NewResponseWriter(h.logger)
    
    // Parse pagination parameters
    page, pageSize := parsePaginationParams(r)
    
    // Get devices with count
    devices, totalCount, err := h.service.ListDevices(page, pageSize)
    if err != nil {
        writer.WriteInternalError(w, r, err)
        return
    }
    
    // Build metadata
    meta := &response.Metadata{}
    meta = meta.WithPagination(page, pageSize, totalCount)
    meta = meta.WithCount(len(devices), totalCount)
    
    writer.WriteSuccessWithMeta(w, r, devices, meta)
}

// Validation error handling
func (h *Handler) CreateDevice(w http.ResponseWriter, r *http.Request) {
    writer := response.NewResponseWriter(h.logger)
    
    var req CreateDeviceRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writer.WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", 
            "Invalid JSON format", map[string]interface{}{
                "parse_error": err.Error(),
            })
        return
    }
    
    // Validate request
    if validationErrors := validateCreateDeviceRequest(req); len(validationErrors) > 0 {
        writer.WriteValidationError(w, r, validationErrors)
        return
    }
    
    // Create device...
}
```

## Security-Aware Error Handling

### Information Disclosure Prevention

The response system prevents sensitive information disclosure while providing useful debugging context:

#### Production vs Development Error Details

```go
func (rw *ResponseWriter) WriteInternalError(w http.ResponseWriter, r *http.Request, err error) {
    // Log the actual error for debugging (server-side only)
    if rw.logger != nil {
        rw.logger.WithFields(map[string]any{
            "method":     r.Method,
            "path":       r.URL.Path,
            "error":      err.Error(),
            "stack_trace": getStackTrace(err), // Only in development
            "request_id": getRequestIDFromContext(r),
            "component":  "api_response",
        }).Error("Internal server error")
    }
    
    // Return sanitized error to client (no sensitive details)
    errorResponse := "Internal server error"
    if isDevelopmentEnvironment() {
        errorResponse = err.Error() // More details in development
    }
    
    rw.WriteError(w, r, http.StatusInternalServerError, 
        response.ErrCodeInternalServer, errorResponse, nil)
}
```

### Safe Error Context

Error responses include safe contextual information that helps debugging without exposing sensitive data:

```go
// Safe: Provides useful context without sensitive information
{
  "success": false,
  "error": {
    "code": "DATABASE_ERROR",
    "message": "Database operation failed",
    "details": {
      "operation": "device_lookup",
      "table": "devices", // Safe to expose
      "affected_records": 0,
      "retry_recommended": true
      // NOTE: No SQL queries, connection strings, or internal paths
    }
  },
  "timestamp": "2024-01-15T10:30:00Z",
  "request_id": "req_database_error"
}

// Unsafe: Would expose sensitive information (NEVER returned)
{
  "error": {
    "code": "DATABASE_ERROR",
    "message": "Database operation failed",
    "details": {
      "sql_query": "SELECT * FROM devices WHERE secret_key = ?",
      "database_host": "10.0.1.100:5432",
      "connection_string": "postgres://user:password@host/db",
      "internal_error": "connection refused to internal.db.company.com"
    }
  }
}
```

## Metadata System

### Comprehensive Metadata Support

The metadata system provides rich context information for API responses:

```go
type Metadata struct {
    Page       *PaginationMeta `json:"pagination,omitempty"`
    Count      *int            `json:"count,omitempty"`
    TotalCount *int            `json:"total_count,omitempty"`
    Version    string          `json:"version,omitempty"`
    CacheInfo  *CacheInfo      `json:"cache,omitempty"`
    Timing     *TimingInfo     `json:"timing,omitempty"`
    Security   *SecurityMeta   `json:"security,omitempty"`
}
```

#### Pagination Metadata
```go
type PaginationMeta struct {
    Page       int  `json:"page"`           // Current page number (1-based)
    PageSize   int  `json:"page_size"`      // Items per page
    TotalPages int  `json:"total_pages"`    // Total number of pages
    HasNext    bool `json:"has_next"`       // More pages available
    HasPrev    bool `json:"has_previous"`   // Previous pages available
}

// Example pagination metadata
{
  "meta": {
    "pagination": {
      "page": 3,
      "page_size": 20,
      "total_pages": 15,
      "has_next": true,
      "has_previous": true
    },
    "count": 20,
    "total_count": 287
  }
}
```

#### Cache Information Metadata
```go
type CacheInfo struct {
    Cached    bool      `json:"cached"`         // Response served from cache
    CachedAt  time.Time `json:"cached_at,omitempty"`  // Cache creation time
    ExpiresAt time.Time `json:"expires_at,omitempty"` // Cache expiration time
    TTL       int       `json:"ttl_seconds,omitempty"` // Time to live in seconds
}

// Example cache metadata
{
  "meta": {
    "cache": {
      "cached": true,
      "cached_at": "2024-01-15T10:25:00Z",
      "expires_at": "2024-01-15T10:35:00Z",
      "ttl_seconds": 600
    }
  }
}
```

#### Performance Timing Metadata
```go
type TimingInfo struct {
    ProcessingTime    int64 `json:"processing_time_ms"`    // Server processing time
    DatabaseTime      int64 `json:"database_time_ms"`      // Database query time
    ExternalAPITime   int64 `json:"external_api_time_ms"`  // External service time
    CacheTime        int64 `json:"cache_time_ms"`         // Cache operation time
}

// Example timing metadata
{
  "meta": {
    "timing": {
      "processing_time_ms": 45,
      "database_time_ms": 12,
      "external_api_time_ms": 28,
      "cache_time_ms": 2
    }
  }
}
```

#### Security Metadata (Safe Information Only)
```go
type SecurityMeta struct {
    RequestValidated bool   `json:"request_validated"`    // Request passed validation
    SecurityLevel   string `json:"security_level"`       // Applied security level
    RateLimit       *RateLimitInfo `json:"rate_limit,omitempty"` // Rate limit info
}

type RateLimitInfo struct {
    Limit     int   `json:"limit"`           // Rate limit threshold
    Remaining int   `json:"remaining"`       // Remaining requests
    ResetTime int64 `json:"reset_time"`      // Reset timestamp
}

// Example security metadata
{
  "meta": {
    "security": {
      "request_validated": true,
      "security_level": "standard",
      "rate_limit": {
        "limit": 1000,
        "remaining": 847,
        "reset_time": 1642248600
      }
    }
  }
}
```

## Response Content Types and Headers

### Content-Type Management

All API responses include appropriate content-type headers:

```go
func (rw *ResponseWriter) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
    // Set proper content type with charset
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    
    // Set response status
    w.WriteHeader(statusCode)
    
    // Encode JSON response
    if err := json.NewEncoder(w).Encode(data); err != nil && rw.logger != nil {
        rw.logger.Error("Failed to encode JSON response", "error", err)
    }
}
```

### Security Headers Integration

Response writing integrates with the security header system:

```go
func WriteSecureJSONResponse(w http.ResponseWriter, r *http.Request, statusCode int, data interface{}) {
    // Apply security headers (already handled by middleware)
    // Content-Type set with secure defaults
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    
    // Prevent caching of sensitive responses
    if isSensitiveEndpoint(r.URL.Path) {
        w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
        w.Header().Set("Pragma", "no-cache")
        w.Header().Set("Expires", "0")
    }
    
    // Write response
    w.WriteHeader(statusCode)
    json.NewEncoder(w).Encode(data)
}
```

## Migration and Backwards Compatibility

### Existing Endpoint Migration

The response standardization system supports gradual migration of existing endpoints:

#### Migration Helper Functions
```go
// Wrapper for legacy endpoints
func MigrateLegacyResponse(w http.ResponseWriter, r *http.Request, legacyData interface{}, err error) {
    writer := response.NewResponseWriter(logging.GetDefault())
    
    if err != nil {
        // Convert legacy errors to standardized format
        standardError := convertLegacyError(err)
        writer.WriteError(w, r, standardError.StatusCode, standardError.Code, 
            standardError.Message, standardError.Details)
        return
    }
    
    // Wrap legacy data in standard format
    writer.WriteSuccess(w, r, legacyData)
}

func convertLegacyError(err error) *APIError {
    switch {
    case strings.Contains(err.Error(), "not found"):
        return &APIError{
            Code:       "NOT_FOUND",
            Message:    "Resource not found",
            StatusCode: http.StatusNotFound,
        }
    case strings.Contains(err.Error(), "invalid"):
        return &APIError{
            Code:       "BAD_REQUEST", 
            Message:    "Invalid request",
            StatusCode: http.StatusBadRequest,
        }
    default:
        return &APIError{
            Code:       "INTERNAL_SERVER_ERROR",
            Message:    "Internal server error",
            StatusCode: http.StatusInternalServerError,
        }
    }
}
```

### Version Management

The response system supports API versioning through metadata:

```go
func (rb *ResponseBuilder) WithVersion(version string) *ResponseBuilder {
    if rb.response.Meta == nil {
        rb.response.Meta = &Metadata{}
    }
    rb.response.Meta.Version = version
    return rb
}

// Usage in handlers
resp := builder.WithVersion("v1.2.0").Success(data)
```

## Client Integration Examples

### JavaScript/TypeScript Client

```typescript
interface APIResponse<T> {
  success: boolean;
  data?: T;
  error?: APIError;
  meta?: ResponseMetadata;
  timestamp: string;
  request_id?: string;
}

interface APIError {
  code: string;
  message: string;
  details?: any;
}

class APIClient {
  async request<T>(endpoint: string, options?: RequestInit): Promise<T> {
    const response = await fetch(`/api/v1${endpoint}`, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...options?.headers,
      },
    });
    
    const apiResponse: APIResponse<T> = await response.json();
    
    if (!apiResponse.success) {
      throw new APIError(apiResponse.error!);
    }
    
    return apiResponse.data!;
  }
}

// Usage
try {
  const device = await apiClient.request<Device>('/devices/123');
  console.log('Device:', device);
} catch (error) {
  if (error instanceof APIError) {
    switch (error.code) {
      case 'DEVICE_NOT_FOUND':
        showNotFoundMessage();
        break;
      case 'DEVICE_OFFLINE':
        showOfflineMessage(error.details.offline_duration_minutes);
        break;
      default:
        showGenericError(error.message);
    }
  }
}
```

### Go Client Example

```go
type APIResponse struct {
    Success   bool        `json:"success"`
    Data      interface{} `json:"data,omitempty"`
    Error     *APIError   `json:"error,omitempty"`
    Meta      *Metadata   `json:"meta,omitempty"`
    Timestamp time.Time   `json:"timestamp"`
    RequestID string      `json:"request_id,omitempty"`
}

func (c *Client) makeRequest(endpoint string, result interface{}) error {
    resp, err := http.Get(c.baseURL + "/api/v1" + endpoint)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    var apiResp APIResponse
    if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
        return err
    }
    
    if !apiResp.Success {
        return &APIError{
            Code:    apiResp.Error.Code,
            Message: apiResp.Error.Message,
            Details: apiResp.Error.Details,
        }
    }
    
    // Unmarshal data into result
    dataBytes, _ := json.Marshal(apiResp.Data)
    return json.Unmarshal(dataBytes, result)
}
```

## Best Practices and Guidelines

### Response Design Guidelines

1. **Consistency**: Always use the standardized response format
2. **Security**: Never expose sensitive information in error messages
3. **Clarity**: Provide clear, actionable error messages
4. **Context**: Include relevant context without revealing internal details
5. **Performance**: Keep response payloads minimal and efficient

### Error Handling Best Practices

1. **Specific Error Codes**: Use specific error codes rather than generic ones
2. **Safe Details**: Include helpful details that don't compromise security
3. **User-Friendly Messages**: Write error messages for end users, not developers
4. **Logging**: Log detailed error information server-side for debugging
5. **Recovery**: Provide guidance on how to resolve errors when possible

### Testing Response Formats

```go
func TestStandardizedResponses(t *testing.T) {
    tests := []struct {
        name           string
        handler        http.HandlerFunc
        expectedStatus int
        expectedCode   string
        hasData        bool
        hasError       bool
    }{
        {
            name:           "successful device retrieval",
            handler:        testHandler.GetDevice,
            expectedStatus: http.StatusOK,
            hasData:        true,
            hasError:       false,
        },
        {
            name:           "device not found",
            handler:        testHandler.GetNonexistentDevice,
            expectedStatus: http.StatusNotFound,
            expectedCode:   "DEVICE_NOT_FOUND",
            hasData:        false,
            hasError:       true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            req := httptest.NewRequest("GET", "/api/v1/devices/123", nil)
            rr := httptest.NewRecorder()
            
            tt.handler(rr, req)
            
            assert.Equal(t, tt.expectedStatus, rr.Code)
            
            var response APIResponse
            err := json.Unmarshal(rr.Body.Bytes(), &response)
            assert.NoError(t, err)
            
            // Validate response structure
            if tt.hasData {
                assert.True(t, response.Success)
                assert.NotNil(t, response.Data)
                assert.Nil(t, response.Error)
            }
            
            if tt.hasError {
                assert.False(t, response.Success)
                assert.Nil(t, response.Data)
                assert.NotNil(t, response.Error)
                if tt.expectedCode != "" {
                    assert.Equal(t, tt.expectedCode, response.Error.Code)
                }
            }
            
            // Validate required fields
            assert.NotZero(t, response.Timestamp)
        })
    }
}
```

The standardized response system provides a robust foundation for consistent, secure, and maintainable API responses while supporting rich error handling and client integration patterns.