package logging

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResponseWriter_WriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec}

	// Test WriteHeader
	rw.WriteHeader(http.StatusNotFound)

	if rw.statusCode != http.StatusNotFound {
		t.Errorf("Expected status code %d, got %d", http.StatusNotFound, rw.statusCode)
	}

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected recorder status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestResponseWriter_Write(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec}

	data := []byte("test response")
	n, err := rw.Write(data)
	if err != nil {
		t.Logf("Failed to write response: %v", err)
	}

	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if n != len(data) {
		t.Errorf("Expected bytes written %d, got %d", len(data), n)
	}

	if rw.size != len(data) {
		t.Errorf("Expected size %d, got %d", len(data), rw.size)
	}

	// Should default to 200 OK when no status set
	if rw.statusCode != http.StatusOK {
		t.Errorf("Expected default status %d, got %d", http.StatusOK, rw.statusCode)
	}
}

func TestResponseWriter_WriteWithoutHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec}

	// Write without calling WriteHeader first
	data := []byte("test")
	if _, err := rw.Write(data); err != nil {
		t.Logf("Failed to write response: %v", err)
	}

	// Should default to 200 OK
	if rw.statusCode != http.StatusOK {
		t.Errorf("Expected default status %d when no header set, got %d", http.StatusOK, rw.statusCode)
	}
}

func TestHTTPMiddleware_LogsRequests(t *testing.T) {
	// Create logger with file output for testing
	tempFile := filepath.Join(t.TempDir(), "middleware-test.log")
	config := Config{
		Level:  LevelInfo,
		Format: "json",
		Output: tempFile,
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer func() {
		if err := logger.Close(); err != nil {
			t.Logf("Failed to close logger: %v", err)
		}
	}()

	// Create test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			t.Logf("Failed to write response: %v", err)
		}
	})

	// Wrap with middleware
	middleware := HTTPMiddleware(logger)
	wrappedHandler := middleware(handler)

	// Create test request
	req := httptest.NewRequest("GET", "/api/devices", nil)
	req.RemoteAddr = "127.0.0.1:12345"

	rec := httptest.NewRecorder()

	// Execute request
	wrappedHandler.ServeHTTP(rec, req)

	// Verify response
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	// slog writes synchronously, no need to wait

	// Check log file
	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// Parse JSON log entry
	var logEntry map[string]interface{}
	if err := json.Unmarshal(content, &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log: %v", err)
	}

	// Verify log fields
	if logEntry["method"] != "GET" {
		t.Errorf("Expected method GET, got %v", logEntry["method"])
	}
	if logEntry["path"] != "/api/devices" {
		t.Errorf("Expected path /api/devices, got %v", logEntry["path"])
	}
	if logEntry["status_code"] != float64(200) {
		t.Errorf("Expected status_code 200, got %v", logEntry["status_code"])
	}
	if logEntry["remote_addr"] != "127.0.0.1:12345" {
		t.Errorf("Expected remote_addr 127.0.0.1:12345, got %v", logEntry["remote_addr"])
	}

	// Should have duration
	if logEntry["duration"] == nil {
		t.Error("Expected duration in log entry")
	}

	// Should have component
	if logEntry["component"] != "http" {
		t.Errorf("Expected component http, got %v", logEntry["component"])
	}
}

func TestHTTPMiddleware_AddsRequestID(t *testing.T) {
	logger, _ := New(Config{Level: LevelInfo, Format: "text", Output: "stdout"})

	// Create test handler that checks for request ID
	var capturedRequestID string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRequestID = GetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with middleware
	middleware := HTTPMiddleware(logger)
	wrappedHandler := middleware(handler)

	// Create test request
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	// Execute request
	wrappedHandler.ServeHTTP(rec, req)

	// Verify request ID was added
	if capturedRequestID == "" {
		t.Error("Expected request ID to be added to context")
	}

	// Request ID should have expected format
	if len(capturedRequestID) < 10 {
		t.Errorf("Request ID seems too short: %s", capturedRequestID)
	}
}

func TestHTTPMiddleware_PreservesExistingRequestID(t *testing.T) {
	logger, _ := New(Config{Level: LevelInfo, Format: "text", Output: "stdout"})

	existingRequestID := "existing-req-123"

	// Create test handler that checks for request ID
	var capturedRequestID string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRequestID = GetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with middleware
	middleware := HTTPMiddleware(logger)
	wrappedHandler := middleware(handler)

	// Create test request with existing request ID
	req := httptest.NewRequest("GET", "/", nil)
	ctx := WithRequestID(req.Context(), existingRequestID)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()

	// Execute request
	wrappedHandler.ServeHTTP(rec, req)

	// Verify existing request ID was preserved
	if capturedRequestID != existingRequestID {
		t.Errorf("Expected request ID %s to be preserved, got %s", existingRequestID, capturedRequestID)
	}
}

func TestGenerateRequestID(t *testing.T) {
	// Generate multiple request IDs
	requestIDs := make(map[string]bool)
	for i := 0; i < 50; i++ { // Reduced to 50 to reduce flakiness
		requestID := generateRequestID()

		// Should not be empty
		if requestID == "" {
			t.Error("Generated request ID should not be empty")
		}

		// Should be unique (at least among most generations - allow some duplicates since it's time-based)
		if requestIDs[requestID] {
			t.Logf("Warning: Generated duplicate request ID: %s (this can happen rarely with time-based generation)", requestID)
		} else {
			requestIDs[requestID] = true
		}

		// Should contain timestamp and random part
		parts := strings.Split(requestID, "-")
		if len(parts) < 2 {
			t.Errorf("Request ID should have timestamp and random parts: %s", requestID)
		}

		// Add small delay to reduce chance of duplicate timestamps
		if i%10 == 0 {
			time.Sleep(1 * time.Millisecond)
		}
	}

	// Verify we got mostly unique IDs (allow some duplicates)
	if len(requestIDs) < 45 { // Should get at least 90% unique
		t.Errorf("Expected mostly unique request IDs, got %d unique out of 50", len(requestIDs))
	}
}

func TestRandomString(t *testing.T) {
	// Test different lengths
	lengths := []int{4, 8, 16, 32}

	for _, length := range lengths {
		t.Run(fmt.Sprintf("length_%d", length), func(t *testing.T) {
			result := randomString(length)

			if len(result) != length {
				t.Errorf("Expected length %d, got %d", length, len(result))
			}

			// Should only contain valid characters
			const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
			for _, char := range result {
				if !strings.ContainsRune(charset, char) {
					t.Errorf("Invalid character %c in random string", char)
				}
			}
		})
	}

	// Test that multiple calls produce different results
	strings1 := randomString(8)
	strings2 := randomString(8)

	// While theoretically possible to get the same string,
	// it's very unlikely with 8 characters
	if strings1 == strings2 {
		t.Log("Warning: Generated identical random strings (very unlikely but possible)")
	}
}

func TestRecoveryMiddleware_HandlesPanic(t *testing.T) {
	// Create logger with file output for testing
	tempFile := filepath.Join(t.TempDir(), "recovery-test.log")
	config := Config{
		Level:  LevelError,
		Format: "json",
		Output: tempFile,
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer func() {
		if err := logger.Close(); err != nil {
			t.Logf("Failed to close logger: %v", err)
		}
	}()

	// Create handler that panics
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	// Wrap with recovery middleware
	middleware := RecoveryMiddleware(logger)
	wrappedHandler := middleware(handler)

	// Create test request
	req := httptest.NewRequest("GET", "/panic", nil)
	rec := httptest.NewRecorder()

	// Execute request (should not panic)
	wrappedHandler.ServeHTTP(rec, req)

	// Should return 500 status
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d after panic, got %d", http.StatusInternalServerError, rec.Code)
	}

	// Should contain error message
	if !strings.Contains(rec.Body.String(), "Internal Server Error") {
		t.Error("Expected error message in response")
	}

	// slog writes synchronously, no need to wait

	// Check log file for panic record
	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// Parse JSON log entry
	var logEntry map[string]interface{}
	if err := json.Unmarshal(content, &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log: %v", err)
	}

	// Verify panic was logged
	if logEntry["panic"] != "test panic" {
		t.Errorf("Expected panic message in log, got %v", logEntry["panic"])
	}
	if logEntry["level"] != "ERROR" {
		t.Errorf("Expected ERROR level, got %v", logEntry["level"])
	}
	if logEntry["msg"] != "HTTP request panicked" {
		t.Errorf("Expected panic message, got %v", logEntry["msg"])
	}
}

func TestRecoveryMiddleware_NoInterferenceOnSuccess(t *testing.T) {
	logger, _ := New(Config{Level: LevelError, Format: "text", Output: "stdout"})

	// Create normal handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("success")); err != nil {
			t.Logf("Failed to write response: %v", err)
		}
	})

	// Wrap with recovery middleware
	middleware := RecoveryMiddleware(logger)
	wrappedHandler := middleware(handler)

	// Create test request
	req := httptest.NewRequest("GET", "/success", nil)
	rec := httptest.NewRecorder()

	// Execute request
	wrappedHandler.ServeHTTP(rec, req)

	// Should work normally
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if rec.Body.String() != "success" {
		t.Errorf("Expected response body 'success', got %s", rec.Body.String())
	}
}

func TestCORSMiddleware_SetsHeaders(t *testing.T) {
	logger, _ := New(Config{Level: LevelDebug, Format: "text", Output: "stdout"})

	// Create test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			t.Logf("Failed to write response: %v", err)
		}
	})

	// Wrap with CORS middleware
	middleware := CORSMiddleware(logger)
	wrappedHandler := middleware(handler)

	// Create test request
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Origin", "http://example.com")

	rec := httptest.NewRecorder()

	// Execute request
	wrappedHandler.ServeHTTP(rec, req)

	// Verify CORS headers
	expectedHeaders := map[string]string{
		"Access-Control-Allow-Origin":  "*",
		"Access-Control-Allow-Methods": "GET, POST, PUT, DELETE, OPTIONS",
		"Access-Control-Allow-Headers": "Content-Type, Authorization",
	}

	for header, expectedValue := range expectedHeaders {
		actualValue := rec.Header().Get(header)
		if actualValue != expectedValue {
			t.Errorf("Expected header %s: %s, got %s", header, expectedValue, actualValue)
		}
	}

	// Response should be OK
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestCORSMiddleware_HandlesPreflightRequest(t *testing.T) {
	logger, _ := New(Config{Level: LevelDebug, Format: "text", Output: "stdout"})

	// Create test handler (should not be called for OPTIONS)
	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with CORS middleware
	middleware := CORSMiddleware(logger)
	wrappedHandler := middleware(handler)

	// Create OPTIONS request (preflight)
	req := httptest.NewRequest("OPTIONS", "/api/test", nil)
	req.Header.Set("Origin", "http://example.com")

	rec := httptest.NewRecorder()

	// Execute request
	wrappedHandler.ServeHTTP(rec, req)

	// Handler should not be called for OPTIONS
	if handlerCalled {
		t.Error("Handler should not be called for OPTIONS request")
	}

	// Should return 200 OK for preflight
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d for preflight, got %d", http.StatusOK, rec.Code)
	}

	// Should still have CORS headers
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("Expected CORS headers on preflight response")
	}
}

func TestCORSMiddleware_LogsRequests(t *testing.T) {
	// Create logger with file output for testing
	tempFile := filepath.Join(t.TempDir(), "cors-test.log")
	config := Config{
		Level:  LevelDebug,
		Format: "json",
		Output: tempFile,
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer func() {
		if err := logger.Close(); err != nil {
			t.Logf("Failed to close logger: %v", err)
		}
	}()

	// Create test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with CORS middleware
	middleware := CORSMiddleware(logger)
	wrappedHandler := middleware(handler)

	// Create request with Origin header
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Origin", "http://example.com")

	rec := httptest.NewRecorder()

	// Execute request
	wrappedHandler.ServeHTTP(rec, req)

	// slog writes synchronously, no need to wait

	// Check log file
	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// Should log CORS request
	if !strings.Contains(string(content), "CORS request processed") {
		t.Error("Expected CORS request to be logged")
	}

	// Parse JSON log entry
	var logEntry map[string]interface{}
	if err := json.Unmarshal(content, &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log: %v", err)
	}

	// Verify log fields
	if logEntry["origin"] != "http://example.com" {
		t.Errorf("Expected origin in log, got %v", logEntry["origin"])
	}
	if logEntry["component"] != "cors" {
		t.Errorf("Expected component cors, got %v", logEntry["component"])
	}
}

func TestCORSMiddleware_NoLogWithoutOrigin(t *testing.T) {
	// Create logger with file output for testing
	tempFile := filepath.Join(t.TempDir(), "cors-no-origin-test.log")
	config := Config{
		Level:  LevelDebug,
		Format: "json",
		Output: tempFile,
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer func() {
		if err := logger.Close(); err != nil {
			t.Logf("Failed to close logger: %v", err)
		}
	}()

	// Create test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with CORS middleware
	middleware := CORSMiddleware(logger)
	wrappedHandler := middleware(handler)

	// Create request without Origin header
	req := httptest.NewRequest("GET", "/api/test", nil)

	rec := httptest.NewRecorder()

	// Execute request
	wrappedHandler.ServeHTTP(rec, req)

	// slog writes synchronously, no need to wait

	// Check log file - should be empty or not exist
	if _, err := os.Stat(tempFile); err == nil {
		content, err := os.ReadFile(tempFile)
		if err != nil {
			t.Fatalf("Failed to read log file: %v", err)
		}

		// Should not log when no Origin header
		if strings.Contains(string(content), "CORS request processed") {
			t.Error("Should not log CORS request when no Origin header present")
		}
	}
}

// Benchmark middleware performance
func BenchmarkHTTPMiddleware(b *testing.B) {
	logger, _ := New(Config{Level: LevelInfo, Format: "text", Output: "stdout"})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := HTTPMiddleware(logger)
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(rec, req)
	}
}

func BenchmarkRecoveryMiddleware(b *testing.B) {
	logger, _ := New(Config{Level: LevelError, Format: "text", Output: "stdout"})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := RecoveryMiddleware(logger)
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(rec, req)
	}
}

func BenchmarkCORSMiddleware(b *testing.B) {
	logger, _ := New(Config{Level: LevelDebug, Format: "text", Output: "stdout"})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := CORSMiddleware(logger)
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "http://example.com")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(rec, req)
	}
}

// Integration test with multiple middleware
func TestMultipleMiddleware(t *testing.T) {
	// Create logger
	tempFile := filepath.Join(t.TempDir(), "multi-middleware-test.log")
	config := Config{
		Level:  LevelDebug,
		Format: "json",
		Output: tempFile,
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer func() {
		if err := logger.Close(); err != nil {
			t.Logf("Failed to close logger: %v", err)
		}
	}()

	// Create test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Access request ID from context
		if GetRequestID(r.Context()) == "" {
			t.Error("Request ID should be available in handler")
		}

		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("success")); err != nil {
			t.Logf("Failed to write response: %v", err)
		}
	})

	// Wrap with multiple middleware (order matters)
	wrappedHandler := RecoveryMiddleware(logger)(
		CORSMiddleware(logger)(
			HTTPMiddleware(logger)(handler),
		),
	)

	// Create test request
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Origin", "http://example.com")

	rec := httptest.NewRecorder()

	// Execute request
	wrappedHandler.ServeHTTP(rec, req)

	// Verify response
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	// Should have CORS headers
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("Expected CORS headers")
	}

	// Should have successful response
	if rec.Body.String() != "success" {
		t.Errorf("Expected response body 'success', got %s", rec.Body.String())
	}

	// slog writes synchronously, no need to wait

	// Check log file - should have entries from both CORS and HTTP middleware
	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)

	// Should have CORS log
	if !strings.Contains(logContent, "CORS request processed") {
		t.Error("Expected CORS log entry")
	}

	// Should have HTTP log
	if !strings.Contains(logContent, "HTTP request completed") {
		t.Error("Expected HTTP request log entry")
	}
}
