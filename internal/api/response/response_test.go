package response

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
)

// contextKey is a custom type for context keys to avoid collisions
// Remove duplicate contextKey definition - use the one from response.go

func TestAPIResponseStructure(t *testing.T) {
	tests := []struct {
		name        string
		response    *APIResponse
		description string
	}{
		{
			name: "Success Response",
			response: &APIResponse{
				Success:   true,
				Data:      map[string]string{"message": "test"},
				Timestamp: time.Now(),
				RequestID: "req-123",
			},
			description: "Success response should have correct structure",
		},
		{
			name: "Error Response",
			response: &APIResponse{
				Success: false,
				Error: &APIError{
					Code:    ErrCodeBadRequest,
					Message: "Invalid input",
					Details: map[string]string{"field": "name is required"},
				},
				Timestamp: time.Now(),
				RequestID: "req-456",
			},
			description: "Error response should have correct structure",
		},
		{
			name: "Response with Metadata",
			response: &APIResponse{
				Success: true,
				Data:    []string{"item1", "item2", "item3"},
				Meta: &Metadata{
					Count:      intPtr(3),
					TotalCount: intPtr(10),
					Version:    "v1.0",
					Page: &PaginationMeta{
						Page:       1,
						PageSize:   10,
						TotalPages: 1,
						HasNext:    false,
						HasPrev:    false,
					},
				},
				Timestamp: time.Now(),
				RequestID: "req-789",
			},
			description: "Response with metadata should have correct structure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Serialize and deserialize to test JSON structure
			jsonData, err := json.Marshal(tt.response)
			require.NoError(t, err, "Should be able to marshal response")

			var unmarshaled APIResponse
			err = json.Unmarshal(jsonData, &unmarshaled)
			require.NoError(t, err, "Should be able to unmarshal response")

			// Verify basic structure
			assert.Equal(t, tt.response.Success, unmarshaled.Success, "Success field should match")
			assert.Equal(t, tt.response.RequestID, unmarshaled.RequestID, "RequestID field should match")

			if tt.response.Success {
				assert.NotNil(t, unmarshaled.Data, "Data should be present in success response")
				assert.Nil(t, unmarshaled.Error, "Error should be nil in success response")
			} else {
				assert.Nil(t, unmarshaled.Data, "Data should be nil in error response")
				assert.NotNil(t, unmarshaled.Error, "Error should be present in error response")
				assert.Equal(t, tt.response.Error.Code, unmarshaled.Error.Code, "Error code should match")
				assert.Equal(t, tt.response.Error.Message, unmarshaled.Error.Message, "Error message should match")
			}
		})
	}
}

func TestResponseBuilder(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})

	t.Run("Basic Response Building", func(t *testing.T) {
		builder := NewResponseBuilder(logger)

		// Test success response
		response := builder.WithRequestID("test-123").Success(map[string]string{"status": "ok"})

		assert.True(t, response.Success, "Should create success response")
		assert.Equal(t, "test-123", response.RequestID, "Should set request ID")
		assert.NotNil(t, response.Data, "Should have data")
		assert.Nil(t, response.Error, "Should not have error")
		assert.False(t, response.Timestamp.IsZero(), "Should have timestamp")
	})

	t.Run("Error Response Building", func(t *testing.T) {
		builder := NewResponseBuilder(logger)

		response := builder.WithRequestID("test-456").Error("INVALID_INPUT", "Input validation failed", map[string]string{"field": "email"})

		assert.False(t, response.Success, "Should create error response")
		assert.Equal(t, "test-456", response.RequestID, "Should set request ID")
		assert.Nil(t, response.Data, "Should not have data")
		assert.NotNil(t, response.Error, "Should have error")
		assert.Equal(t, "INVALID_INPUT", response.Error.Code, "Should set error code")
		assert.Equal(t, "Input validation failed", response.Error.Message, "Should set error message")
		assert.NotNil(t, response.Error.Details, "Should set error details")
	})

	t.Run("Response with Metadata", func(t *testing.T) {
		builder := NewResponseBuilder(logger)

		meta := &Metadata{
			Count:      intPtr(5),
			TotalCount: intPtr(50),
			Version:    "v1.0",
		}

		response := builder.WithMeta(meta).Success([]string{"a", "b", "c"})

		assert.NotNil(t, response.Meta, "Should have metadata")
		assert.Equal(t, 5, *response.Meta.Count, "Should set count")
		assert.Equal(t, 50, *response.Meta.TotalCount, "Should set total count")
		assert.Equal(t, "v1.0", response.Meta.Version, "Should set version")
	})

	t.Run("Response with Pagination", func(t *testing.T) {
		builder := NewResponseBuilder(logger)

		response := builder.WithPagination(2, 10, 45).Success([]string{"item1", "item2"})

		require.NotNil(t, response.Meta, "Should have metadata")
		require.NotNil(t, response.Meta.Page, "Should have pagination")

		page := response.Meta.Page
		assert.Equal(t, 2, page.Page, "Should set current page")
		assert.Equal(t, 10, page.PageSize, "Should set page size")
		assert.Equal(t, 5, page.TotalPages, "Should calculate total pages")
		assert.True(t, page.HasNext, "Should have next page")
		assert.True(t, page.HasPrev, "Should have previous page")
	})

	t.Run("Response with Count", func(t *testing.T) {
		builder := NewResponseBuilder(logger)

		response := builder.WithCount(15, 100).Success([]string{"a", "b", "c"})

		require.NotNil(t, response.Meta, "Should have metadata")
		assert.Equal(t, 15, *response.Meta.Count, "Should set count")
		assert.Equal(t, 100, *response.Meta.TotalCount, "Should set total count")
	})
}

func TestResponseWriter(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})
	writer := NewResponseWriter(logger)

	t.Run("WriteSuccess", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/test", nil)

		data := map[string]string{"status": "success"}
		writer.WriteSuccess(rr, req, data)

		assert.Equal(t, http.StatusOK, rr.Code, "Should return 200 status")
		assert.Equal(t, "application/json; charset=utf-8", rr.Header().Get("Content-Type"), "Should set JSON content type")

		var response APIResponse
		err := json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err, "Should decode JSON response")

		assert.True(t, response.Success, "Should be success response")
		assert.NotNil(t, response.Data, "Should have data")
		assert.Nil(t, response.Error, "Should not have error")
	})

	t.Run("WriteSuccessWithMeta", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/test", nil)

		data := []string{"item1", "item2"}
		meta := &Metadata{
			Count:   intPtr(2),
			Version: "v1.0",
		}

		writer.WriteSuccessWithMeta(rr, req, data, meta)

		assert.Equal(t, http.StatusOK, rr.Code, "Should return 200 status")

		var response APIResponse
		err := json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err, "Should decode JSON response")

		assert.True(t, response.Success, "Should be success response")
		assert.NotNil(t, response.Meta, "Should have metadata")
		assert.Equal(t, "v1.0", response.Meta.Version, "Should set version in metadata")
	})

	t.Run("WriteError", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/v1/test", nil)

		writer.WriteError(rr, req, http.StatusBadRequest, ErrCodeValidationFailed, "Validation error", map[string]string{"field": "required"})

		assert.Equal(t, http.StatusBadRequest, rr.Code, "Should return 400 status")
		assert.Equal(t, "application/json; charset=utf-8", rr.Header().Get("Content-Type"), "Should set JSON content type")

		var response APIResponse
		err := json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err, "Should decode JSON response")

		assert.False(t, response.Success, "Should be error response")
		assert.Nil(t, response.Data, "Should not have data")
		assert.NotNil(t, response.Error, "Should have error")
		assert.Equal(t, ErrCodeValidationFailed, response.Error.Code, "Should set error code")
		assert.Equal(t, "Validation error", response.Error.Message, "Should set error message")
		assert.NotNil(t, response.Error.Details, "Should set error details")
	})

	t.Run("WriteValidationError", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/v1/test", nil)

		validationErrors := map[string]interface{}{
			"email":    "invalid email format",
			"password": "password too short",
		}

		writer.WriteValidationError(rr, req, validationErrors)

		assert.Equal(t, http.StatusBadRequest, rr.Code, "Should return 400 status")

		var response APIResponse
		err := json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err, "Should decode JSON response")

		assert.False(t, response.Success, "Should be error response")
		assert.Equal(t, ErrCodeValidationFailed, response.Error.Code, "Should use validation failed error code")
		assert.Equal(t, "Validation failed", response.Error.Message, "Should use standard validation message")
		assert.Equal(t, validationErrors, response.Error.Details, "Should include validation details")
	})

	t.Run("WriteNotFoundError", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/users/123", nil)

		writer.WriteNotFoundError(rr, req, "User")

		assert.Equal(t, http.StatusNotFound, rr.Code, "Should return 404 status")

		var response APIResponse
		err := json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err, "Should decode JSON response")

		assert.False(t, response.Success, "Should be error response")
		assert.Equal(t, ErrCodeNotFound, response.Error.Code, "Should use not found error code")
		assert.Contains(t, response.Error.Message, "User", "Should include resource name in message")
	})

	t.Run("WriteInternalError", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/test", nil)

		writer.WriteInternalError(rr, req, fmt.Errorf("database connection failed"))

		assert.Equal(t, http.StatusInternalServerError, rr.Code, "Should return 500 status")

		var response APIResponse
		err := json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err, "Should decode JSON response")

		assert.False(t, response.Success, "Should be error response")
		assert.Equal(t, ErrCodeInternalServer, response.Error.Code, "Should use internal server error code")
		assert.Equal(t, "Internal server error", response.Error.Message, "Should use generic error message")
		assert.Nil(t, response.Error.Details, "Should not expose internal error details")
	})

	t.Run("WriteCreated", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/v1/users", nil)

		data := map[string]interface{}{"id": 123, "name": "John Doe"}
		writer.WriteCreated(rr, req, data)

		assert.Equal(t, http.StatusCreated, rr.Code, "Should return 201 status")

		var response APIResponse
		err := json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err, "Should decode JSON response")

		assert.True(t, response.Success, "Should be success response")
		assert.NotNil(t, response.Data, "Should have data")
	})

	t.Run("WriteNoContent", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("DELETE", "/api/v1/users/123", nil)

		writer.WriteNoContent(rr, req)

		assert.Equal(t, http.StatusNoContent, rr.Code, "Should return 204 status")
		assert.Empty(t, rr.Body.String(), "Should have empty body")
	})
}

func TestRequestIDHandling(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})
	writer := NewResponseWriter(logger)

	t.Run("With Request ID in Context", func(t *testing.T) {
		rr := httptest.NewRecorder()

		// Create request with request ID in context
		ctx := context.WithValue(context.Background(), RequestIDKey, "req-12345")
		req := httptest.NewRequest("GET", "/api/v1/test", nil).WithContext(ctx)

		writer.WriteSuccess(rr, req, map[string]string{"status": "ok"})

		var response APIResponse
		err := json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err, "Should decode JSON response")

		assert.Equal(t, "req-12345", response.RequestID, "Should extract request ID from context")
	})

	t.Run("Without Request ID in Context", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/test", nil)

		writer.WriteSuccess(rr, req, map[string]string{"status": "ok"})

		var response APIResponse
		err := json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err, "Should decode JSON response")

		assert.Empty(t, response.RequestID, "Should have empty request ID")
	})
}

func TestConvenienceFunctions(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		data := map[string]string{"message": "success"}
		response := Success(data)

		assert.True(t, response.Success, "Should be success response")
		assert.Equal(t, data, response.Data, "Should set data")
		assert.Nil(t, response.Error, "Should not have error")
		assert.False(t, response.Timestamp.IsZero(), "Should have timestamp")
	})

	t.Run("Error", func(t *testing.T) {
		response := Error(ErrCodeBadRequest, "Invalid input")

		assert.False(t, response.Success, "Should be error response")
		assert.Nil(t, response.Data, "Should not have data")
		assert.NotNil(t, response.Error, "Should have error")
		assert.Equal(t, ErrCodeBadRequest, response.Error.Code, "Should set error code")
		assert.Equal(t, "Invalid input", response.Error.Message, "Should set error message")
		assert.Nil(t, response.Error.Details, "Should not have details")
	})

	t.Run("ErrorWithDetails", func(t *testing.T) {
		details := map[string]string{"field": "email", "issue": "invalid format"}
		response := ErrorWithDetails(ErrCodeValidationFailed, "Validation error", details)

		assert.False(t, response.Success, "Should be error response")
		assert.NotNil(t, response.Error, "Should have error")
		assert.Equal(t, ErrCodeValidationFailed, response.Error.Code, "Should set error code")
		assert.Equal(t, details, response.Error.Details, "Should set error details")
	})

	t.Run("ValidationError", func(t *testing.T) {
		details := map[string]string{"name": "required", "age": "must be positive"}
		response := ValidationError(details)

		assert.False(t, response.Success, "Should be error response")
		assert.Equal(t, ErrCodeValidationFailed, response.Error.Code, "Should use validation failed code")
		assert.Equal(t, "Validation failed", response.Error.Message, "Should use validation failed message")
		assert.Equal(t, details, response.Error.Details, "Should set validation details")
	})

	t.Run("NotFoundError", func(t *testing.T) {
		response := NotFoundError("User")

		assert.False(t, response.Success, "Should be error response")
		assert.Equal(t, ErrCodeNotFound, response.Error.Code, "Should use not found code")
		assert.Contains(t, response.Error.Message, "User", "Should include resource name")
	})

	t.Run("InternalError", func(t *testing.T) {
		response := InternalError()

		assert.False(t, response.Success, "Should be error response")
		assert.Equal(t, ErrCodeInternalServer, response.Error.Code, "Should use internal server error code")
		assert.Equal(t, "Internal server error", response.Error.Message, "Should use generic message")
	})
}

func TestErrorCodeMapping(t *testing.T) {
	tests := []struct {
		statusCode   int
		expectedCode string
		description  string
	}{
		{http.StatusBadRequest, ErrCodeBadRequest, "400 should map to BAD_REQUEST"},
		{http.StatusUnauthorized, ErrCodeUnauthorized, "401 should map to UNAUTHORIZED"},
		{http.StatusForbidden, ErrCodeForbidden, "403 should map to FORBIDDEN"},
		{http.StatusNotFound, ErrCodeNotFound, "404 should map to NOT_FOUND"},
		{http.StatusMethodNotAllowed, ErrCodeMethodNotAllowed, "405 should map to METHOD_NOT_ALLOWED"},
		{http.StatusConflict, ErrCodeConflict, "409 should map to CONFLICT"},
		{http.StatusRequestEntityTooLarge, ErrCodeRequestTooLarge, "413 should map to REQUEST_TOO_LARGE"},
		{http.StatusUnsupportedMediaType, ErrCodeUnsupportedMedia, "415 should map to UNSUPPORTED_MEDIA_TYPE"},
		{http.StatusTooManyRequests, ErrCodeRateLimitExceeded, "429 should map to RATE_LIMIT_EXCEEDED"},
		{http.StatusInternalServerError, ErrCodeInternalServer, "500 should map to INTERNAL_SERVER_ERROR"},
		{http.StatusNotImplemented, ErrCodeNotImplemented, "501 should map to NOT_IMPLEMENTED"},
		{http.StatusServiceUnavailable, ErrCodeServiceUnavailable, "503 should map to SERVICE_UNAVAILABLE"},
		{http.StatusRequestTimeout, ErrCodeTimeout, "408 should map to REQUEST_TIMEOUT"},
		{999, ErrCodeInternalServer, "Unknown status should map to INTERNAL_SERVER_ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			code := GetErrorCodeForStatus(tt.statusCode)
			assert.Equal(t, tt.expectedCode, code, tt.description)
		})
	}
}

func TestPaginationCalculation(t *testing.T) {
	tests := []struct {
		name         string
		page         int
		pageSize     int
		totalItems   int
		expectedMeta *PaginationMeta
		description  string
	}{
		{
			name:       "First Page with More Pages",
			page:       1,
			pageSize:   10,
			totalItems: 25,
			expectedMeta: &PaginationMeta{
				Page:       1,
				PageSize:   10,
				TotalPages: 3,
				HasNext:    true,
				HasPrev:    false,
			},
			description: "First page should not have previous, should have next",
		},
		{
			name:       "Middle Page",
			page:       2,
			pageSize:   10,
			totalItems: 25,
			expectedMeta: &PaginationMeta{
				Page:       2,
				PageSize:   10,
				TotalPages: 3,
				HasNext:    true,
				HasPrev:    true,
			},
			description: "Middle page should have both previous and next",
		},
		{
			name:       "Last Page",
			page:       3,
			pageSize:   10,
			totalItems: 25,
			expectedMeta: &PaginationMeta{
				Page:       3,
				PageSize:   10,
				TotalPages: 3,
				HasNext:    false,
				HasPrev:    true,
			},
			description: "Last page should have previous, should not have next",
		},
		{
			name:       "Single Page",
			page:       1,
			pageSize:   10,
			totalItems: 5,
			expectedMeta: &PaginationMeta{
				Page:       1,
				PageSize:   10,
				TotalPages: 1,
				HasNext:    false,
				HasPrev:    false,
			},
			description: "Single page should not have previous or next",
		},
		{
			name:       "Exact Page Boundary",
			page:       2,
			pageSize:   10,
			totalItems: 20,
			expectedMeta: &PaginationMeta{
				Page:       2,
				PageSize:   10,
				TotalPages: 2,
				HasNext:    false,
				HasPrev:    true,
			},
			description: "Exact boundary should calculate correctly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, _ := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})
			builder := NewResponseBuilder(logger)

			response := builder.WithPagination(tt.page, tt.pageSize, tt.totalItems).Success([]string{})

			require.NotNil(t, response.Meta, "Should have metadata")
			require.NotNil(t, response.Meta.Page, "Should have pagination metadata")

			page := response.Meta.Page
			assert.Equal(t, tt.expectedMeta.Page, page.Page, "Page should match")
			assert.Equal(t, tt.expectedMeta.PageSize, page.PageSize, "PageSize should match")
			assert.Equal(t, tt.expectedMeta.TotalPages, page.TotalPages, "TotalPages should match")
			assert.Equal(t, tt.expectedMeta.HasNext, page.HasNext, "HasNext should match")
			assert.Equal(t, tt.expectedMeta.HasPrev, page.HasPrev, "HasPrev should match")
		})
	}
}

func TestJSONResponseGeneration(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})
	writer := NewResponseWriter(logger)

	t.Run("Complex Data Structure", func(t *testing.T) {
		complexData := map[string]interface{}{
			"user": map[string]interface{}{
				"id":    123,
				"name":  "John Doe",
				"email": "john@example.com",
				"roles": []string{"admin", "user"},
			},
			"settings": map[string]interface{}{
				"theme":         "dark",
				"notifications": true,
				"language":      "en",
			},
			"metadata": map[string]interface{}{
				"created_at": time.Now().Format(time.RFC3339),
				"updated_at": time.Now().Format(time.RFC3339),
			},
		}

		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/user/profile", nil)

		writer.WriteSuccess(rr, req, complexData)

		assert.Equal(t, http.StatusOK, rr.Code, "Should return 200 status")

		var response APIResponse
		err := json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err, "Should decode complex JSON response")

		assert.True(t, response.Success, "Should be success response")
		assert.NotNil(t, response.Data, "Should have data")

		// Verify nested data structure is preserved
		data, ok := response.Data.(map[string]interface{})
		require.True(t, ok, "Data should be map")

		user, ok := data["user"].(map[string]interface{})
		require.True(t, ok, "User should be map")
		assert.Equal(t, float64(123), user["id"], "User ID should be preserved")
		assert.Equal(t, "John Doe", user["name"], "User name should be preserved")
	})

	t.Run("Array Data", func(t *testing.T) {
		arrayData := []map[string]interface{}{
			{"id": 1, "name": "Item 1"},
			{"id": 2, "name": "Item 2"},
			{"id": 3, "name": "Item 3"},
		}

		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/items", nil)

		writer.WriteSuccess(rr, req, arrayData)

		var response APIResponse
		err := json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err, "Should decode array response")

		data, ok := response.Data.([]interface{})
		require.True(t, ok, "Data should be array")
		assert.Len(t, data, 3, "Should have 3 items")
	})
}

func TestCacheInfoSerialization(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})

	cacheInfo := &CacheInfo{
		Cached:    true,
		CachedAt:  time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
		TTL:       3600,
	}

	meta := &Metadata{
		Version:   "v1.0",
		CacheInfo: cacheInfo,
	}

	builder := NewResponseBuilder(logger)
	response := builder.WithMeta(meta).Success(map[string]string{"status": "ok"})

	// Serialize and check JSON
	jsonData, err := json.Marshal(response)
	require.NoError(t, err, "Should marshal response with cache info")

	var unmarshaled APIResponse
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err, "Should unmarshal response with cache info")

	require.NotNil(t, unmarshaled.Meta, "Should have metadata")
	require.NotNil(t, unmarshaled.Meta.CacheInfo, "Should have cache info")

	cache := unmarshaled.Meta.CacheInfo
	assert.True(t, cache.Cached, "Should preserve cached flag")
	assert.Equal(t, 3600, cache.TTL, "Should preserve TTL")
}

func TestErrorResponseSanitization(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})
	writer := NewResponseWriter(logger)

	t.Run("Internal Error Sanitization", func(t *testing.T) {
		// This should not expose internal details
		internalErr := fmt.Errorf("database connection failed: user=admin password=secret host=internal.db.server")

		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/test", nil)

		writer.WriteInternalError(rr, req, internalErr)

		var response APIResponse
		err := json.NewDecoder(rr.Body).Decode(&response)
		require.NoError(t, err, "Should decode error response")

		assert.False(t, response.Success, "Should be error response")
		assert.Equal(t, "Internal server error", response.Error.Message, "Should use generic message")
		assert.Nil(t, response.Error.Details, "Should not expose internal details")

		// Verify response body doesn't contain sensitive information
		responseBody := rr.Body.String()
		assert.NotContains(t, responseBody, "password=secret", "Should not expose passwords")
		assert.NotContains(t, responseBody, "internal.db.server", "Should not expose internal hostnames")
	})
}

func TestErrorCodes(t *testing.T) {
	// Test that all error codes are properly defined
	errorCodes := []string{
		ErrCodeBadRequest,
		ErrCodeUnauthorized,
		ErrCodeForbidden,
		ErrCodeNotFound,
		ErrCodeMethodNotAllowed,
		ErrCodeConflict,
		ErrCodeValidationFailed,
		ErrCodeRateLimitExceeded,
		ErrCodeRequestTooLarge,
		ErrCodeUnsupportedMedia,
		ErrCodeInternalServer,
		ErrCodeNotImplemented,
		ErrCodeServiceUnavailable,
		ErrCodeTimeout,
		ErrCodeDatabaseError,
		ErrCodeExternalServiceError,
		ErrCodeDeviceNotFound,
		ErrCodeDeviceOffline,
		ErrCodeConfigurationError,
		ErrCodeTemplateError,
		ErrCodeProvisioningError,
		ErrCodeMetricsError,
		ErrCodeNotificationError,
	}

	for _, code := range errorCodes {
		t.Run(fmt.Sprintf("ErrorCode_%s", code), func(t *testing.T) {
			assert.NotEmpty(t, code, "Error code should not be empty")
			assert.True(t, len(code) > 0, "Error code should have content")

			// Error codes should be in UPPER_CASE format
			assert.Equal(t, code, strings.ToUpper(code), "Error code should be uppercase")
		})
	}
}

func BenchmarkResponseGeneration(b *testing.B) {
	logger, _ := logging.New(logging.Config{Level: "error", Format: "text", Output: "stdout"}) // Reduce logging for benchmarks
	writer := NewResponseWriter(logger)

	b.Run("SimpleSuccess", func(b *testing.B) {
		data := map[string]string{"status": "ok"}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/api/v1/test", nil)
			writer.WriteSuccess(rr, req, data)
		}
	})

	b.Run("ComplexDataStructure", func(b *testing.B) {
		complexData := map[string]interface{}{
			"users": []map[string]interface{}{
				{"id": 1, "name": "User 1", "active": true},
				{"id": 2, "name": "User 2", "active": false},
				{"id": 3, "name": "User 3", "active": true},
			},
			"meta": map[string]interface{}{
				"total":       3,
				"page":        1,
				"per_page":    10,
				"total_pages": 1,
			},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/api/v1/users", nil)
			writer.WriteSuccess(rr, req, complexData)
		}
	})

	b.Run("ErrorResponse", func(b *testing.B) {
		validationErrors := map[string]string{
			"email":    "invalid format",
			"password": "too short",
			"name":     "required",
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/api/v1/register", nil)
			writer.WriteValidationError(rr, req, validationErrors)
		}
	})

	b.Run("ResponseBuilding", func(b *testing.B) {
		builder := NewResponseBuilder(logger)
		data := map[string]string{"message": "test"}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			builder.WithRequestID(fmt.Sprintf("req-%d", i)).Success(data)
		}
	})
}

// Helper function
func intPtr(i int) *int {
	return &i
}
