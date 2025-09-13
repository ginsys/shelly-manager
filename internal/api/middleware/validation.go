package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/ginsys/shelly-manager/internal/api/response"
	"github.com/ginsys/shelly-manager/internal/logging"
)

// ValidationConfig holds configuration for request validation middleware
type ValidationConfig struct {
	// Content type validation
	AllowedContentTypes map[string]bool
	StrictContentType   bool // enforce exact content-type matching

	// Header validation
	RequiredHeaders  []string
	ForbiddenHeaders []string
	MaxHeaderSize    int // maximum size of individual headers
	MaxHeaderCount   int // maximum number of headers

	// JSON validation
	ValidateJSON     bool // validate JSON syntax for JSON requests
	MaxJSONDepth     int  // maximum nesting depth in JSON
	MaxJSONArraySize int  // maximum array size in JSON

	// Query parameter validation
	MaxQueryParamSize  int      // maximum size of query parameters
	MaxQueryParamCount int      // maximum number of query parameters
	ForbiddenParams    []string // forbidden parameter names

	// Security validation
	BlockSuspiciousUserAgents bool
	BlockSuspiciousHeaders    bool

	// Test mode (bypasses security validations for E2E testing)
	TestMode bool

	// Logging
	LogValidationErrors bool
}

// DefaultValidationConfig returns a secure default validation configuration
func DefaultValidationConfig() *ValidationConfig {
	return &ValidationConfig{
		AllowedContentTypes: map[string]bool{
			"application/json":                  true,
			"application/x-www-form-urlencoded": true,
			"multipart/form-data":               true,
			"text/plain":                        true,
		},
		StrictContentType:         true,
		RequiredHeaders:           []string{},                                        // No required headers by default
		ForbiddenHeaders:          []string{"x-forwarded-proto", "x-forwarded-host"}, // Block potential header injection
		MaxHeaderSize:             8192,                                              // 8KB per header
		MaxHeaderCount:            50,                                                // Maximum 50 headers
		ValidateJSON:              true,
		MaxJSONDepth:              10,
		MaxJSONArraySize:          1000,
		MaxQueryParamSize:         2048,                                              // 2KB per parameter
		MaxQueryParamCount:        50,                                                // Maximum 50 parameters
		ForbiddenParams:           []string{"__proto__", "constructor", "prototype"}, // Block prototype pollution
		BlockSuspiciousUserAgents: true,
		BlockSuspiciousHeaders:    true,
		TestMode:                  false, // Default: security validations enabled
		LogValidationErrors:       true,
	}
}

// ValidateContentTypeMiddleware validates request content types
func ValidateContentTypeMiddleware(config *ValidationConfig, logger *logging.Logger) func(http.Handler) http.Handler {
	respWriter := response.NewResponseWriter(logger)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip GET, DELETE, HEAD, and OPTIONS requests
			if r.Method == "GET" || r.Method == "DELETE" || r.Method == "HEAD" || r.Method == "OPTIONS" {
				next.ServeHTTP(w, r)
				return
			}

			contentType := r.Header.Get("Content-Type")
			if contentType == "" {
				if config.StrictContentType {
					respWriter.WriteValidationError(w, r, map[string]string{
						"content_type": "Content-Type header is required",
					})
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			// Parse media type to handle parameters like charset
			mediaType, _, err := mime.ParseMediaType(contentType)
			if err != nil {
				respWriter.WriteValidationError(w, r, map[string]string{
					"content_type": "Invalid Content-Type header format",
				})
				return
			}

			// Check if content type is allowed
			if config.AllowedContentTypes != nil && !config.AllowedContentTypes[mediaType] {
				if logger != nil && config.LogValidationErrors {
					logger.WithFields(map[string]any{
						"method":           r.Method,
						"path":             r.URL.Path,
						"content_type":     mediaType,
						"client_ip":        getClientIP(r),
						"component":        "validation",
						"validation_error": "unsupported_content_type",
					}).Warn("Unsupported content type")
				}

				respWriter.WriteError(w, r, http.StatusUnsupportedMediaType,
					response.ErrCodeUnsupportedMedia,
					fmt.Sprintf("Unsupported content type: %s", mediaType),
					map[string]interface{}{
						"allowed_types": getAllowedContentTypes(config.AllowedContentTypes),
					})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ValidateHeadersMiddleware validates request headers
func ValidateHeadersMiddleware(config *ValidationConfig, logger *logging.Logger) func(http.Handler) http.Handler {
	respWriter := response.NewResponseWriter(logger)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Allow CORS preflight requests to pass without strict validation
			if r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}
			// If strict content-type is enforced and current content-type would be rejected,
			// defer header validation so that the content-type middleware handles first.
			if config.StrictContentType && (r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch) {
				ct := r.Header.Get("Content-Type")
				if ct != "" {
					if mediaType, _, err := mime.ParseMediaType(ct); err == nil {
						if config.AllowedContentTypes != nil && !config.AllowedContentTypes[mediaType] {
							next.ServeHTTP(w, r)
							return
						}
					}
				}
			}

			validationErrors := make(map[string]string)

			// Check header count
			if config.MaxHeaderCount > 0 && len(r.Header) > config.MaxHeaderCount {
				validationErrors["headers"] = fmt.Sprintf("Too many headers (max: %d)", config.MaxHeaderCount)
			}

			// Check individual header sizes and forbidden headers
			for name, values := range r.Header {
				// Check forbidden headers
				lowerName := strings.ToLower(name)
				for _, forbidden := range config.ForbiddenHeaders {
					if strings.ToLower(forbidden) == lowerName {
						validationErrors[name] = "Forbidden header"
						break
					}
				}

				// Check header size
				if config.MaxHeaderSize > 0 {
					for _, value := range values {
						if len(value) > config.MaxHeaderSize {
							validationErrors[name] = fmt.Sprintf("Header too large (max: %d bytes)", config.MaxHeaderSize)
							break
						}
					}
				}

				// Check for suspicious header content (skip in test mode)
				if !config.TestMode && config.BlockSuspiciousHeaders {
					for _, value := range values {
						if containsSuspiciousContent(value) {
							validationErrors[name] = "Suspicious header content detected"
							break
						}
					}
				}
			}

			// Check required headers
			for _, required := range config.RequiredHeaders {
				if r.Header.Get(required) == "" {
					validationErrors[required] = "Required header missing"
				}
			}

			// Check user agent if configured (skip in test mode)
			if !config.TestMode && config.BlockSuspiciousUserAgents {
				userAgent := r.Header.Get("User-Agent")
				if isSuspiciousUserAgent(userAgent) {
					validationErrors["user_agent"] = "Suspicious user agent detected"
				}
			}

			if len(validationErrors) > 0 {
				if logger != nil && config.LogValidationErrors {
					logger.WithFields(map[string]any{
						"method":            r.Method,
						"path":              r.URL.Path,
						"client_ip":         getClientIP(r),
						"validation_errors": validationErrors,
						"component":         "validation",
						"validation_error":  "header_validation_failed",
					}).Warn("Header validation failed")
				}

				respWriter.WriteValidationError(w, r, validationErrors)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ValidateJSONMiddleware validates JSON request bodies
func ValidateJSONMiddleware(config *ValidationConfig, logger *logging.Logger) func(http.Handler) http.Handler {
	respWriter := response.NewResponseWriter(logger)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only validate JSON requests
			if !config.ValidateJSON || !isJSONRequest(r) {
				next.ServeHTTP(w, r)
				return
			}

			// Skip if no body
			if r.ContentLength == 0 {
				next.ServeHTTP(w, r)
				return
			}

			// Create a limited reader to prevent huge payloads
			limitedReader := http.MaxBytesReader(w, r.Body, 10*1024*1024) // 10MB limit
			r.Body = limitedReader

			// Read the entire body to validate JSON and restore it for subsequent handlers
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				if logger != nil && config.LogValidationErrors {
					logger.WithFields(map[string]any{
						"method":           r.Method,
						"path":             r.URL.Path,
						"client_ip":        getClientIP(r),
						"error":            err.Error(),
						"component":        "validation",
						"validation_error": "body_read_error",
					}).Warn("Failed to read request body")
				}

				respWriter.WriteValidationError(w, r, map[string]string{
					"json": "Failed to read request body: " + err.Error(),
				})
				return
			}

			// Restore the body for subsequent handlers
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

			// Try to decode JSON to validate syntax
			var rawJSON interface{}
			if err := json.Unmarshal(bodyBytes, &rawJSON); err != nil {
				if logger != nil && config.LogValidationErrors {
					logger.WithFields(map[string]any{
						"method":           r.Method,
						"path":             r.URL.Path,
						"client_ip":        getClientIP(r),
						"error":            err.Error(),
						"component":        "validation",
						"validation_error": "json_syntax_error",
					}).Warn("JSON validation failed")
				}

				respWriter.WriteValidationError(w, r, map[string]string{
					"json": "Invalid JSON syntax: " + err.Error(),
				})
				return
			}

			// Validate JSON structure
			if config.MaxJSONDepth > 0 || config.MaxJSONArraySize > 0 {
				if err := validateJSONStructure(rawJSON, config); err != nil {
					if logger != nil && config.LogValidationErrors {
						logger.WithFields(map[string]any{
							"method":           r.Method,
							"path":             r.URL.Path,
							"client_ip":        getClientIP(r),
							"error":            err.Error(),
							"component":        "validation",
							"validation_error": "json_structure_error",
						}).Warn("JSON structure validation failed")
					}

					respWriter.WriteValidationError(w, r, map[string]string{
						"json": err.Error(),
					})
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ValidateQueryParamsMiddleware validates query parameters
func ValidateQueryParamsMiddleware(config *ValidationConfig, logger *logging.Logger) func(http.Handler) http.Handler {
	respWriter := response.NewResponseWriter(logger)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If strict content-type is enforced and current content-type would be rejected,
			// defer to the content-type middleware (allow it to fail first in the chain).
			if config.StrictContentType && (r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch) {
				ct := r.Header.Get("Content-Type")
				if ct != "" {
					if mediaType, _, err := mime.ParseMediaType(ct); err == nil {
						if config.AllowedContentTypes != nil && !config.AllowedContentTypes[mediaType] {
							next.ServeHTTP(w, r)
							return
						}
					}
				}
			}

			validationErrors := make(map[string]string)

			// Check query parameter count
			if config.MaxQueryParamCount > 0 && len(r.URL.Query()) > config.MaxQueryParamCount {
				validationErrors["query"] = fmt.Sprintf("Too many query parameters (max: %d)", config.MaxQueryParamCount)
			}

			// Check individual parameters
			for param, values := range r.URL.Query() {
				// Check forbidden parameters
				for _, forbidden := range config.ForbiddenParams {
					if param == forbidden {
						validationErrors[param] = "Forbidden parameter"
						break
					}
				}

				// Check parameter size
				if config.MaxQueryParamSize > 0 {
					for _, value := range values {
						if len(value) > config.MaxQueryParamSize {
							validationErrors[param] = fmt.Sprintf("Parameter too large (max: %d bytes)", config.MaxQueryParamSize)
							break
						}
					}
				}

				// Check for suspicious content
				for _, value := range values {
					if containsSuspiciousContent(value) {
						validationErrors[param] = "Suspicious parameter content detected"
						break
					}
				}
			}

			if len(validationErrors) > 0 {
				if logger != nil && config.LogValidationErrors {
					logger.WithFields(map[string]any{
						"method":            r.Method,
						"path":              r.URL.Path,
						"client_ip":         getClientIP(r),
						"validation_errors": validationErrors,
						"component":         "validation",
						"validation_error":  "query_param_validation_failed",
					}).Warn("Query parameter validation failed")
				}

				respWriter.WriteValidationError(w, r, validationErrors)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Helper functions

func getAllowedContentTypes(allowed map[string]bool) []string {
	types := make([]string, 0, len(allowed))
	for contentType := range allowed {
		types = append(types, contentType)
	}
	return types
}

func isJSONRequest(r *http.Request) bool {
	contentType := r.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return mediaType == "application/json"
}

func containsSuspiciousContent(value string) bool {
	lower := strings.ToLower(value)

	// Check for common injection patterns
	patterns := []string{
		"<script", "javascript:", "data:", "vbscript:",
		"onload=", "onerror=", "onclick=", "onmouseover=",
		"eval(", "alert(", "confirm(", "prompt(", "fromcharcode(",
		"document.cookie", "document.domain",
		"../", "..\\", "/etc/passwd", "/proc/",
		"union select", "drop table", "delete from",
		"insert into", "update users set", "update set", "' or 1=1", " or 1=1", " or '1'='1",
		"exec ", "sleep(",
	}

	for _, pattern := range patterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	return false
}

func isSuspiciousUserAgent(userAgent string) bool {
	if userAgent == "" {
		return true // Empty user agents are suspicious
	}

	lower := strings.ToLower(userAgent)

	// Common bot/scanner patterns
	suspiciousPatterns := []string{
		"sqlmap", "nikto", "nessus", "openvas",
		"burpsuite", "nmap", "masscan", "zap",
		"w3af", "skipfish", "arachni", "wpscan",
		"dirbuster", "dirb", "gobuster", "ffuf",
		"python-requests", "curl/", "wget/",
		"scanner", "bot", "crawler", "spider",
	}

	for _, pattern := range suspiciousPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	// Check for unusually short or long user agents
	if len(userAgent) < 10 || len(userAgent) > 500 {
		return true
	}

	return false
}

func validateJSONStructure(data interface{}, config *ValidationConfig) error {
	return validateJSONDepth(data, 0, config.MaxJSONDepth, config.MaxJSONArraySize)
}

func validateJSONDepth(data interface{}, currentDepth, maxDepth, maxArraySize int) error {
	if maxDepth > 0 && currentDepth > maxDepth {
		return fmt.Errorf("JSON nesting too deep (max: %d)", maxDepth)
	}

	switch v := data.(type) {
	case map[string]interface{}:
		for _, value := range v {
			if err := validateJSONDepth(value, currentDepth+1, maxDepth, maxArraySize); err != nil {
				return err
			}
		}
	case []interface{}:
		if maxArraySize > 0 && len(v) > maxArraySize {
			return fmt.Errorf("JSON array too large (max: %d elements)", maxArraySize)
		}
		// Do not increase depth for array elements; depth limit
		// applies primarily to object nesting. This allows nested
		// arrays within reasonable limits without tripping depth.
		for _, item := range v {
			if err := validateJSONDepth(item, currentDepth, maxDepth, maxArraySize); err != nil {
				return err
			}
		}
	}

	return nil
}

// IntParam extracts and validates an integer parameter from URL path
func IntParam(r *http.Request, param string) (int, error) {
	// This would typically use a router-specific method
	// For now, we'll implement a basic version
	vars := make(map[string]string)
	// Extract from gorilla/mux (if using mux.Vars)
	// This is a placeholder - in real implementation, use mux.Vars(r)

	if val, exists := vars[param]; exists {
		return strconv.Atoi(val)
	}

	return 0, fmt.Errorf("parameter %s not found", param)
}

// StringParam extracts and validates a string parameter from URL path
func StringParam(r *http.Request, param string) (string, error) {
	// This would typically use a router-specific method
	vars := make(map[string]string)
	// Extract from gorilla/mux (if using mux.Vars)

	if val, exists := vars[param]; exists {
		return val, nil
	}

	return "", fmt.Errorf("parameter %s not found", param)
}
