package response

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// HandlerWrapper provides convenient methods for handlers using standardized responses
type HandlerWrapper struct {
	writer *ResponseWriter
	logger *logging.Logger
}

// NewHandlerWrapper creates a new handler wrapper
func NewHandlerWrapper(logger *logging.Logger) *HandlerWrapper {
	return &HandlerWrapper{
		writer: NewResponseWriter(logger),
		logger: logger,
	}
}

// Success writes a success response
func (hw *HandlerWrapper) Success(w http.ResponseWriter, r *http.Request, data interface{}) {
	hw.writer.WriteSuccess(w, r, data)
}

// SuccessWithMeta writes a success response with metadata
func (hw *HandlerWrapper) SuccessWithMeta(w http.ResponseWriter, r *http.Request, data interface{}, meta *Metadata) {
	hw.writer.WriteSuccessWithMeta(w, r, data, meta)
}

// Created writes a created response (201)
func (hw *HandlerWrapper) Created(w http.ResponseWriter, r *http.Request, data interface{}) {
	hw.writer.WriteCreated(w, r, data)
}

// NoContent writes a no content response (204)
func (hw *HandlerWrapper) NoContent(w http.ResponseWriter, r *http.Request) {
	hw.writer.WriteNoContent(w, r)
}

// Error writes an error response
func (hw *HandlerWrapper) Error(w http.ResponseWriter, r *http.Request, statusCode int, code, message string, details interface{}) {
	hw.writer.WriteError(w, r, statusCode, code, message, details)
}

// BadRequest writes a bad request error (400)
func (hw *HandlerWrapper) BadRequest(w http.ResponseWriter, r *http.Request, message string, details interface{}) {
	hw.writer.WriteError(w, r, http.StatusBadRequest, ErrCodeBadRequest, message, details)
}

// NotFound writes a not found error (404)
func (hw *HandlerWrapper) NotFound(w http.ResponseWriter, r *http.Request, resource string) {
	hw.writer.WriteNotFoundError(w, r, resource)
}

// InternalError writes an internal server error (500)
func (hw *HandlerWrapper) InternalError(w http.ResponseWriter, r *http.Request, err error) {
	hw.writer.WriteInternalError(w, r, err)
}

// ValidationError writes a validation error (400)
func (hw *HandlerWrapper) ValidationError(w http.ResponseWriter, r *http.Request, validationErrors interface{}) {
	hw.writer.WriteValidationError(w, r, validationErrors)
}

// Conflict writes a conflict error (409)
func (hw *HandlerWrapper) Conflict(w http.ResponseWriter, r *http.Request, message string, details interface{}) {
	hw.writer.WriteError(w, r, http.StatusConflict, ErrCodeConflict, message, details)
}

// Helper functions for extracting path parameters

// GetPathParam extracts a path parameter from the request using mux
func GetPathParam(r *http.Request, param string) string {
	vars := mux.Vars(r)
	return vars[param]
}

// GetPathParamInt extracts an integer path parameter from the request
func GetPathParamInt(r *http.Request, param string) (int, error) {
	vars := mux.Vars(r)
	if val, exists := vars[param]; exists {
		return strconv.Atoi(val)
	}
	return 0, fmt.Errorf("parameter %s not found", param)
}

// GetPathParamString extracts a string path parameter from the request
func GetPathParamString(r *http.Request, param string) (string, error) {
	vars := mux.Vars(r)
	if val, exists := vars[param]; exists {
		return val, nil
	}
	return "", fmt.Errorf("parameter %s not found", param)
}

// GetQueryParam extracts a query parameter with default value
func GetQueryParam(r *http.Request, param, defaultValue string) string {
	if value := r.URL.Query().Get(param); value != "" {
		return value
	}
	return defaultValue
}

// GetQueryParamInt extracts an integer query parameter with default value
func GetQueryParamInt(r *http.Request, param string, defaultValue int) int {
	if value := r.URL.Query().Get(param); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// GetQueryParamBool extracts a boolean query parameter with default value
func GetQueryParamBool(r *http.Request, param string, defaultValue bool) bool {
	if value := r.URL.Query().Get(param); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}

// Example of how to update existing handlers to use standardized responses
//
// OLD PATTERN:
// func (h *Handler) GetDevices(w http.ResponseWriter, r *http.Request) {
//     devices, err := h.DB.GetDevices()
//     if err != nil {
//         w.WriteHeader(http.StatusInternalServerError)
//         w.Header().Set("Content-Type", "application/json")
//         h.writeJSON(w, map[string]interface{}{
//             "success": false,
//             "error":   "Failed to get devices: " + err.Error(),
//         })
//         return
//     }
//     w.Header().Set("Content-Type", "application/json")
//     h.writeJSON(w, map[string]interface{}{
//         "success": true,
//         "devices": devices,
//     })
// }
//
// NEW PATTERN:
// func (h *Handler) GetDevices(w http.ResponseWriter, r *http.Request) {
//     respWrapper := response.NewHandlerWrapper(h.logger)
//
//     devices, err := h.DB.GetDevices()
//     if err != nil {
//         respWrapper.InternalError(w, r, err)
//         return
//     }
//
//     // Optional: Add metadata
//     meta := &response.Metadata{
//         Count: &len(devices),
//     }
//
//     respWrapper.SuccessWithMeta(w, r, map[string]interface{}{
//         "devices": devices,
//     }, meta)
// }
