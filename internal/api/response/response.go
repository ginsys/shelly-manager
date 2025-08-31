package response

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	// RequestIDKey is the context key for request IDs
	RequestIDKey contextKey = "request_id"
)

// APIResponse represents the standardized API response format
type APIResponse struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     *APIError   `json:"error,omitempty"`
	Meta      *Metadata   `json:"meta,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	RequestID string      `json:"request_id,omitempty"`
}

// APIError represents detailed error information
type APIError struct {
	Code       string      `json:"code"`
	Message    string      `json:"message"`
	Details    interface{} `json:"details,omitempty"`
	StatusCode int         `json:"-"` // Internal use, not serialized
}

// Metadata contains additional response metadata
type Metadata struct {
	Page       *PaginationMeta `json:"pagination,omitempty"`
	Count      *int            `json:"count,omitempty"`
	TotalCount *int            `json:"total_count,omitempty"`
	Version    string          `json:"version,omitempty"`
	CacheInfo  *CacheInfo      `json:"cache,omitempty"`
}

// PaginationMeta contains pagination information
type PaginationMeta struct {
	Page       int  `json:"page"`
	PageSize   int  `json:"page_size"`
	TotalPages int  `json:"total_pages"`
	HasNext    bool `json:"has_next"`
	HasPrev    bool `json:"has_previous"`
}

// CacheInfo contains cache-related metadata
type CacheInfo struct {
	Cached    bool      `json:"cached"`
	CachedAt  time.Time `json:"cached_at,omitempty"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	TTL       int       `json:"ttl_seconds,omitempty"`
}

// Standard error codes
const (
	// Client errors (4xx)
	ErrCodeBadRequest        = "BAD_REQUEST"
	ErrCodeUnauthorized      = "UNAUTHORIZED"
	ErrCodeForbidden         = "FORBIDDEN"
	ErrCodeNotFound          = "NOT_FOUND"
	ErrCodeMethodNotAllowed  = "METHOD_NOT_ALLOWED"
	ErrCodeConflict          = "CONFLICT"
	ErrCodeValidationFailed  = "VALIDATION_FAILED"
	ErrCodeRateLimitExceeded = "RATE_LIMIT_EXCEEDED"
	ErrCodeRequestTooLarge   = "REQUEST_TOO_LARGE"
	ErrCodeUnsupportedMedia  = "UNSUPPORTED_MEDIA_TYPE"

	// Server errors (5xx)
	ErrCodeInternalServer       = "INTERNAL_SERVER_ERROR"
	ErrCodeNotImplemented       = "NOT_IMPLEMENTED"
	ErrCodeServiceUnavailable   = "SERVICE_UNAVAILABLE"
	ErrCodeTimeout              = "REQUEST_TIMEOUT"
	ErrCodeDatabaseError        = "DATABASE_ERROR"
	ErrCodeExternalServiceError = "EXTERNAL_SERVICE_ERROR"

	// Application-specific errors
	ErrCodeDeviceNotFound     = "DEVICE_NOT_FOUND"
	ErrCodeDeviceOffline      = "DEVICE_OFFLINE"
	ErrCodeConfigurationError = "CONFIGURATION_ERROR"
	ErrCodeTemplateError      = "TEMPLATE_ERROR"
	ErrCodeProvisioningError  = "PROVISIONING_ERROR"
	ErrCodeMetricsError       = "METRICS_ERROR"
	ErrCodeNotificationError  = "NOTIFICATION_ERROR"
)

// ResponseBuilder provides a fluent interface for building responses
type ResponseBuilder struct {
	response  *APIResponse
	logger    *logging.Logger
	requestID string
}

// NewResponseBuilder creates a new response builder
func NewResponseBuilder(logger *logging.Logger) *ResponseBuilder {
	return &ResponseBuilder{
		response: &APIResponse{
			Timestamp: time.Now().UTC(),
		},
		logger: logger,
	}
}

// WithRequestID sets the request ID
func (rb *ResponseBuilder) WithRequestID(requestID string) *ResponseBuilder {
	rb.requestID = requestID
	rb.response.RequestID = requestID
	return rb
}

// WithMeta adds metadata to the response
func (rb *ResponseBuilder) WithMeta(meta *Metadata) *ResponseBuilder {
	rb.response.Meta = meta
	return rb
}

// WithPagination adds pagination metadata
func (rb *ResponseBuilder) WithPagination(page, pageSize, totalItems int) *ResponseBuilder {
	totalPages := (totalItems + pageSize - 1) / pageSize
	if rb.response.Meta == nil {
		rb.response.Meta = &Metadata{}
	}
	rb.response.Meta.Page = &PaginationMeta{
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
		HasNext:    page < totalPages,
		HasPrev:    page > 1,
	}
	return rb
}

// WithCount adds count information
func (rb *ResponseBuilder) WithCount(count, totalCount int) *ResponseBuilder {
	if rb.response.Meta == nil {
		rb.response.Meta = &Metadata{}
	}
	rb.response.Meta.Count = &count
	rb.response.Meta.TotalCount = &totalCount
	return rb
}

// Success creates a successful response
func (rb *ResponseBuilder) Success(data interface{}) *APIResponse {
	rb.response.Success = true
	rb.response.Data = data
	return rb.response
}

// Error creates an error response
func (rb *ResponseBuilder) Error(code, message string, details interface{}) *APIResponse {
	rb.response.Success = false
	rb.response.Error = &APIError{
		Code:    code,
		Message: message,
		Details: details,
	}
	return rb.response
}

// ErrorWithStatus creates an error response with HTTP status code
func (rb *ResponseBuilder) ErrorWithStatus(code, message string, statusCode int, details interface{}) *APIResponse {
	rb.response.Success = false
	rb.response.Error = &APIError{
		Code:       code,
		Message:    message,
		Details:    details,
		StatusCode: statusCode,
	}
	return rb.response
}

// ResponseWriter provides convenient methods for writing standardized responses
type ResponseWriter struct {
	logger *logging.Logger
}

// NewResponseWriter creates a new response writer
func NewResponseWriter(logger *logging.Logger) *ResponseWriter {
	return &ResponseWriter{logger: logger}
}

// WriteSuccess writes a successful JSON response
func (rw *ResponseWriter) WriteSuccess(w http.ResponseWriter, r *http.Request, data interface{}) {
	builder := NewResponseBuilder(rw.logger)
	if requestID := getRequestIDFromContext(r); requestID != "" {
		builder.WithRequestID(requestID)
	}

	response := builder.Success(data)
	// Ensure version metadata is present for observability
	if response.Meta == nil {
		response.Meta = &Metadata{}
	}
	if response.Meta.Version == "" {
		response.Meta.Version = "v1"
	}
	rw.writeJSONResponse(w, http.StatusOK, response)
}

// WriteSuccessWithMeta writes a successful response with metadata
func (rw *ResponseWriter) WriteSuccessWithMeta(w http.ResponseWriter, r *http.Request, data interface{}, meta *Metadata) {
	builder := NewResponseBuilder(rw.logger)
	if requestID := getRequestIDFromContext(r); requestID != "" {
		builder.WithRequestID(requestID)
	}

	response := builder.WithMeta(meta).Success(data)
	if response.Meta == nil {
		response.Meta = &Metadata{}
	}
	if response.Meta.Version == "" {
		response.Meta.Version = "v1"
	}
	rw.writeJSONResponse(w, http.StatusOK, response)
}

// WriteError writes an error response
func (rw *ResponseWriter) WriteError(w http.ResponseWriter, r *http.Request, statusCode int, code, message string, details interface{}) {
	builder := NewResponseBuilder(rw.logger)
	if requestID := getRequestIDFromContext(r); requestID != "" {
		builder.WithRequestID(requestID)
	}

	response := builder.ErrorWithStatus(code, message, statusCode, details)
	if response.Meta == nil {
		response.Meta = &Metadata{}
	}
	if response.Meta.Version == "" {
		response.Meta.Version = "v1"
	}
	rw.writeJSONResponse(w, statusCode, response)

	// Log error for monitoring
	if rw.logger != nil {
		rw.logger.WithFields(map[string]any{
			"method":      r.Method,
			"path":        r.URL.Path,
			"status_code": statusCode,
			"error_code":  code,
			"error_msg":   message,
			"request_id":  builder.requestID,
			"component":   "api_response",
		}).Error("API error response")
	}
}

// WriteValidationError writes a validation error response
func (rw *ResponseWriter) WriteValidationError(w http.ResponseWriter, r *http.Request, validationErrors interface{}) {
	rw.WriteError(w, r, http.StatusBadRequest, ErrCodeValidationFailed, "Validation failed", validationErrors)
}

// WriteNotFoundError writes a not found error response
func (rw *ResponseWriter) WriteNotFoundError(w http.ResponseWriter, r *http.Request, resource string) {
	message := fmt.Sprintf("%s not found", resource)
	rw.WriteError(w, r, http.StatusNotFound, ErrCodeNotFound, message, nil)
}

// WriteInternalError writes an internal server error response
func (rw *ResponseWriter) WriteInternalError(w http.ResponseWriter, r *http.Request, err error) {
	// Log the actual error for debugging
	if rw.logger != nil {
		rw.logger.WithFields(map[string]any{
			"method":     r.Method,
			"path":       r.URL.Path,
			"error":      err.Error(),
			"request_id": getRequestIDFromContext(r),
			"component":  "api_response",
		}).Error("Internal server error")
	}

	// Don't expose internal error details to clients
	rw.WriteError(w, r, http.StatusInternalServerError, ErrCodeInternalServer, "Internal server error", nil)
}

// WriteCreated writes a created response (201)
func (rw *ResponseWriter) WriteCreated(w http.ResponseWriter, r *http.Request, data interface{}) {
	builder := NewResponseBuilder(rw.logger)
	if requestID := getRequestIDFromContext(r); requestID != "" {
		builder.WithRequestID(requestID)
	}

	response := builder.Success(data)
	rw.writeJSONResponse(w, http.StatusCreated, response)
}

// WriteNoContent writes a no content response (204)
func (rw *ResponseWriter) WriteNoContent(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

// writeJSONResponse writes a JSON response with proper headers
func (rw *ResponseWriter) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil && rw.logger != nil {
		rw.logger.Error("Failed to encode JSON response", "error", err)
	}
}

// Convenience functions for quick responses

// Success creates a successful response
func Success(data interface{}) *APIResponse {
	return &APIResponse{
		Success:   true,
		Data:      data,
		Timestamp: time.Now().UTC(),
	}
}

// Error creates an error response
func Error(code, message string) *APIResponse {
	return &APIResponse{
		Success: false,
		Error: &APIError{
			Code:    code,
			Message: message,
		},
		Timestamp: time.Now().UTC(),
	}
}

// ErrorWithDetails creates an error response with details
func ErrorWithDetails(code, message string, details interface{}) *APIResponse {
	return &APIResponse{
		Success: false,
		Error: &APIError{
			Code:    code,
			Message: message,
			Details: details,
		},
		Timestamp: time.Now().UTC(),
	}
}

// ValidationError creates a validation error response
func ValidationError(details interface{}) *APIResponse {
	return ErrorWithDetails(ErrCodeValidationFailed, "Validation failed", details)
}

// NotFoundError creates a not found error response
func NotFoundError(resource string) *APIResponse {
	message := fmt.Sprintf("%s not found", resource)
	return Error(ErrCodeNotFound, message)
}

// InternalError creates an internal server error response
func InternalError() *APIResponse {
	return Error(ErrCodeInternalServer, "Internal server error")
}

// Helper function to extract request ID from context
func getRequestIDFromContext(r *http.Request) string {
	if ctx := r.Context(); ctx != nil {
		// Prefer the request ID set by the logging middleware
		if rid := logging.GetRequestID(ctx); rid != "" {
			return rid
		}
		// Fallback to local context key if present
		if requestID, ok := ctx.Value(RequestIDKey).(string); ok {
			return requestID
		}
	}
	return ""
}

// HTTP status code to error code mapping
var statusCodeToErrorCode = map[int]string{
	http.StatusBadRequest:            ErrCodeBadRequest,
	http.StatusUnauthorized:          ErrCodeUnauthorized,
	http.StatusForbidden:             ErrCodeForbidden,
	http.StatusNotFound:              ErrCodeNotFound,
	http.StatusMethodNotAllowed:      ErrCodeMethodNotAllowed,
	http.StatusConflict:              ErrCodeConflict,
	http.StatusRequestEntityTooLarge: ErrCodeRequestTooLarge,
	http.StatusUnsupportedMediaType:  ErrCodeUnsupportedMedia,
	http.StatusTooManyRequests:       ErrCodeRateLimitExceeded,
	http.StatusInternalServerError:   ErrCodeInternalServer,
	http.StatusNotImplemented:        ErrCodeNotImplemented,
	http.StatusServiceUnavailable:    ErrCodeServiceUnavailable,
	http.StatusRequestTimeout:        ErrCodeTimeout,
}

// GetErrorCodeForStatus returns the appropriate error code for HTTP status
func GetErrorCodeForStatus(statusCode int) string {
	if code, exists := statusCodeToErrorCode[statusCode]; exists {
		return code
	}
	return ErrCodeInternalServer
}
