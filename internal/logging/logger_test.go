package logging

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNew_DefaultValues(t *testing.T) {
	// Test with empty config to verify defaults
	logger, err := New(Config{})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	if logger.config.Level != LevelInfo {
		t.Errorf("Expected default level %s, got %s", LevelInfo, logger.config.Level)
	}
	if logger.config.Format != "text" {
		t.Errorf("Expected default format text, got %s", logger.config.Format)
	}
	if logger.config.Output != "stdout" {
		t.Errorf("Expected default output stdout, got %s", logger.config.Output)
	}
}

func TestNew_ValidConfig(t *testing.T) {
	config := Config{
		Level:  LevelDebug,
		Format: "json",
		Output: "stderr",
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close() // Ensure proper cleanup

	if logger.config.Level != LevelDebug {
		t.Errorf("Expected level %s, got %s", LevelDebug, logger.config.Level)
	}
	if logger.config.Format != "json" {
		t.Errorf("Expected format json, got %s", logger.config.Format)
	}
	if logger.config.Output != "stderr" {
		t.Errorf("Expected output stderr, got %s", logger.config.Output)
	}
}

func TestNew_FileOutput(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test.log")

	config := Config{
		Level:  LevelInfo,
		Format: "text",
		Output: logFile,
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger with file output: %v", err)
	}
	defer logger.Close() // Ensure proper cleanup

	// Write a log message
	logger.Info("test message")

	// Verify file was created
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("Log file was not created")
	}

	// Read file contents
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "test message") {
		t.Error("Log message not found in file")
	}
}

func TestNew_InvalidFileOutput(t *testing.T) {
	config := Config{
		Level:  LevelInfo,
		Format: "text",
		Output: "/invalid/path/that/does/not/exist/test.log",
	}

	_, err := New(config)
	if err == nil {
		t.Error("Expected error for invalid file path")
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
		{"DEBUG", slog.LevelDebug}, // case insensitive
		{"INFO", slog.LevelInfo},
		{"invalid", slog.LevelInfo}, // fallback to info
		{"", slog.LevelInfo},        // fallback to info
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			result, err := parseLevel(test.input)
			if err != nil {
				t.Fatalf("parseLevel failed: %v", err)
			}
			if result != test.expected {
				t.Errorf("Expected level %v, got %v", test.expected, result)
			}
		})
	}
}

func TestGetWriter(t *testing.T) {
	tests := []struct {
		name   string
		output string
		valid  bool
	}{
		{"stdout", "stdout", true},
		{"stderr", "stderr", true},
		{"valid file", filepath.Join(t.TempDir(), "test.log"), true},
		{"invalid file", "/invalid/path/test.log", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			writer, file, err := getWriter(test.output)

			if test.valid && err != nil {
				t.Errorf("Expected valid writer, got error: %v", err)
			}
			if !test.valid && err == nil {
				t.Error("Expected error for invalid output")
			}
			if test.valid && writer == nil {
				t.Error("Writer should not be nil for valid output")
			}

			// Clean up file if it was created
			if file != nil {
				file.Close()
			}
		})
	}
}

func TestWithFields(t *testing.T) {
	// Create logger with custom output
	tempFile := filepath.Join(t.TempDir(), "fields-test.log")
	config := Config{
		Level:  LevelDebug,
		Format: "json",
		Output: tempFile,
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close() // Ensure proper cleanup

	// Test WithFields
	fields := map[string]any{
		"component": "test",
		"operation": "unit_test",
		"count":     42,
	}

	fieldLogger := logger.WithFields(fields)
	fieldLogger.Info("test message with fields")

	// Read the log file
	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// Parse JSON log entry
	var logEntry map[string]interface{}
	if err := json.Unmarshal(content, &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log: %v", err)
	}

	// Verify fields are present
	if logEntry["component"] != "test" {
		t.Error("Expected component field in log entry")
	}
	if logEntry["operation"] != "unit_test" {
		t.Error("Expected operation field in log entry")
	}
	if logEntry["count"] != float64(42) { // JSON numbers are float64
		t.Error("Expected count field in log entry")
	}
}

func TestWithContext(t *testing.T) {
	// Create logger
	config := Config{
		Level:  LevelDebug,
		Format: "json",
		Output: "stdout",
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close() // Ensure proper cleanup

	// Test context with values
	ctx := context.Background()
	ctx = context.WithValue(ctx, "request_id", "req-123")
	ctx = context.WithValue(ctx, "user_id", "user-456")

	contextLogger := logger.WithContext(ctx)

	// Verify logger was enhanced (not nil)
	if contextLogger == nil {
		t.Error("WithContext should return a logger")
	}

	// Test context without values
	emptyCtx := context.Background()
	emptyContextLogger := logger.WithContext(emptyCtx)

	// Should return the same logger when no context values
	if emptyContextLogger != logger {
		t.Log("WithContext returned new logger for empty context (this is acceptable)")
	}
}

func TestLogDBOperation(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "db-test.log")
	config := Config{
		Level:  LevelDebug,
		Format: "json",
		Output: tempFile,
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close() // Ensure proper cleanup

	// Test successful operation
	logger.LogDBOperation("SELECT", "devices", 1500, nil)

	// Test failed operation
	dbError := errors.New("connection timeout")
	logger.LogDBOperation("UPDATE", "devices", 3000, dbError)

	// Read and verify log contents
	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logLines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(logLines) < 2 {
		t.Fatalf("Expected at least 2 log entries, got %d", len(logLines))
	}

	// Parse first log entry (success)
	var successEntry map[string]interface{}
	if err := json.Unmarshal([]byte(logLines[0]), &successEntry); err != nil {
		t.Fatalf("Failed to parse success log entry: %v", err)
	}

	if successEntry["operation"] != "SELECT" {
		t.Error("Expected operation SELECT in success entry")
	}
	if successEntry["table"] != "devices" {
		t.Error("Expected table devices in success entry")
	}
	if successEntry["duration"] != float64(1500) {
		t.Error("Expected duration 1500 in success entry")
	}
	if successEntry["component"] != "database" {
		t.Error("Expected component database in success entry")
	}

	// Parse second log entry (error)
	var errorEntry map[string]interface{}
	if err := json.Unmarshal([]byte(logLines[1]), &errorEntry); err != nil {
		t.Fatalf("Failed to parse error log entry: %v", err)
	}

	if errorEntry["error"] != "connection timeout" {
		t.Error("Expected error message in error entry")
	}
	if errorEntry["level"] != "ERROR" {
		t.Error("Expected ERROR level for failed operation")
	}
}

func TestLogHTTPRequest(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "http-test.log")
	config := Config{
		Level:  LevelInfo,
		Format: "json",
		Output: tempFile,
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close() // Ensure proper cleanup

	// Test different status codes
	testCases := []struct {
		method        string
		path          string
		statusCode    int
		expectedLevel string
	}{
		{"GET", "/api/devices", 200, "INFO"},
		{"POST", "/api/devices", 400, "WARN"},
		{"PUT", "/api/devices/1", 500, "ERROR"},
	}

	for _, tc := range testCases {
		logger.LogHTTPRequest(tc.method, tc.path, "127.0.0.1", tc.statusCode, 150)
	}

	// Read and verify log contents
	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logLines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(logLines) != len(testCases) {
		t.Fatalf("Expected %d log entries, got %d", len(testCases), len(logLines))
	}

	for i, tc := range testCases {
		var logEntry map[string]interface{}
		if err := json.Unmarshal([]byte(logLines[i]), &logEntry); err != nil {
			t.Fatalf("Failed to parse log entry %d: %v", i, err)
		}

		if logEntry["method"] != tc.method {
			t.Errorf("Expected method %s, got %v", tc.method, logEntry["method"])
		}
		if logEntry["path"] != tc.path {
			t.Errorf("Expected path %s, got %v", tc.path, logEntry["path"])
		}
		if logEntry["status_code"] != float64(tc.statusCode) {
			t.Errorf("Expected status_code %d, got %v", tc.statusCode, logEntry["status_code"])
		}
		if logEntry["level"] != tc.expectedLevel {
			t.Errorf("Expected level %s, got %v", tc.expectedLevel, logEntry["level"])
		}
	}
}

func TestLogDiscoveryOperation(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "discovery-test.log")
	config := Config{
		Level:  LevelInfo,
		Format: "json",
		Output: tempFile,
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close() // Ensure proper cleanup

	// Test successful discovery
	logger.LogDiscoveryOperation("scan", "192.168.1.0/24", 5, 2000, nil)

	// Test failed discovery
	discoveryError := errors.New("network unreachable")
	logger.LogDiscoveryOperation("scan", "10.0.0.0/8", 0, 1000, discoveryError)

	// Verify logs
	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logLines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(logLines) < 2 {
		t.Fatalf("Expected at least 2 log entries, got %d", len(logLines))
	}

	// Check success entry
	var successEntry map[string]interface{}
	if err := json.Unmarshal([]byte(logLines[0]), &successEntry); err != nil {
		t.Fatalf("Failed to parse success log entry: %v", err)
	}

	if successEntry["devices_found"] != float64(5) {
		t.Error("Expected devices_found 5 in success entry")
	}
	if successEntry["network"] != "192.168.1.0/24" {
		t.Error("Expected network in success entry")
	}
	if successEntry["component"] != "discovery" {
		t.Error("Expected component discovery in success entry")
	}
}

func TestLogDeviceOperation(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "device-test.log")
	config := Config{
		Level:  LevelInfo,
		Format: "json",
		Output: tempFile,
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close() // Ensure proper cleanup

	// Test device operations
	logger.LogDeviceOperation("configure", "192.168.1.100", "AA:BB:CC:DD:EE:FF", nil)

	deviceError := errors.New("device not responding")
	logger.LogDeviceOperation("provision", "192.168.1.101", "11:22:33:44:55:66", deviceError)

	// Verify logs
	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "configure") {
		t.Error("Expected configure operation in logs")
	}
	if !strings.Contains(string(content), "device not responding") {
		t.Error("Expected error message in logs")
	}
}

func TestLogAppLifecycle(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "app-test.log")
	config := Config{
		Level:  LevelInfo,
		Format: "json",
		Output: tempFile,
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close() // Ensure proper cleanup

	// Test app lifecycle logging
	logger.LogAppStart("1.0.0", "0.0.0.0:8080")
	logger.LogAppStop("shutdown signal received")

	// Verify logs
	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)
	if !strings.Contains(logContent, "Application starting") {
		t.Error("Expected app start message in logs")
	}
	if !strings.Contains(logContent, "Application stopping") {
		t.Error("Expected app stop message in logs")
	}
	if !strings.Contains(logContent, "1.0.0") {
		t.Error("Expected version in logs")
	}
	if !strings.Contains(logContent, "shutdown signal received") {
		t.Error("Expected stop reason in logs")
	}
}

func TestLogConfigLoad(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "config-test.log")
	config := Config{
		Level:  LevelInfo,
		Format: "json",
		Output: tempFile,
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close() // Ensure proper cleanup

	// Test successful config load
	logger.LogConfigLoad("/etc/shelly-manager.yaml", nil)

	// Test failed config load
	configError := errors.New("file not found")
	logger.LogConfigLoad("/invalid/config.yaml", configError)

	// Verify logs
	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)
	if !strings.Contains(logContent, "Configuration loaded successfully") {
		t.Error("Expected success message in logs")
	}
	if !strings.Contains(logContent, "Configuration load failed") {
		t.Error("Expected failure message in logs")
	}
	if !strings.Contains(logContent, "file not found") {
		t.Error("Expected error message in logs")
	}
}

func TestSetDefault_GetDefault(t *testing.T) {
	// Test GetDefault with no default set
	defaultLogger = nil // Reset
	logger := GetDefault()
	if logger == nil {
		t.Error("GetDefault should return a logger even when none is set")
	}

	// Test SetDefault
	customConfig := Config{
		Level:  LevelDebug,
		Format: "json",
		Output: "stderr",
	}

	customLogger, err := New(customConfig)
	if err != nil {
		t.Fatalf("Failed to create custom logger: %v", err)
	}

	SetDefault(customLogger)

	retrievedLogger := GetDefault()
	if retrievedLogger != customLogger {
		t.Error("GetDefault should return the custom logger after SetDefault")
	}

	// Verify the retrieved logger has custom config
	if retrievedLogger.config.Level != LevelDebug {
		t.Error("Retrieved logger should have debug level")
	}
}

func TestLoggerTextFormat(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "text-format-test.log")
	config := Config{
		Level:  LevelInfo,
		Format: "text",
		Output: tempFile,
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close() // Ensure proper cleanup

	logger.Info("test message")

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)
	if !strings.Contains(logContent, "test message") {
		t.Error("Expected log message in text format")
	}

	// Text format should not be JSON
	var jsonTest map[string]interface{}
	if json.Unmarshal(content, &jsonTest) == nil {
		t.Error("Text format should not be valid JSON")
	}
}

func TestLoggerJSONFormat(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "json-format-test.log")
	config := Config{
		Level:  LevelInfo,
		Format: "json",
		Output: tempFile,
	}

	logger, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close() // Ensure proper cleanup

	logger.Info("test json message")

	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// JSON format should be valid JSON
	var jsonEntry map[string]interface{}
	if err := json.Unmarshal(content, &jsonEntry); err != nil {
		t.Fatalf("JSON format should produce valid JSON: %v", err)
	}

	if jsonEntry["msg"] != "test json message" {
		t.Error("Expected message in JSON entry")
	}

	// Should have timestamp
	if jsonEntry["timestamp"] == nil {
		t.Error("Expected timestamp in JSON entry")
	}
}

// Benchmark tests
func BenchmarkLoggerCreation(b *testing.B) {
	config := Config{
		Level:  LevelInfo,
		Format: "json",
		Output: "stdout",
	}

	for i := 0; i < b.N; i++ {
		logger, err := New(config)
		if err != nil {
			b.Fatalf("Failed to create logger: %v", err)
		}
		_ = logger
	}
}

func BenchmarkWithFields(b *testing.B) {
	logger, _ := New(Config{Level: LevelInfo, Format: "json", Output: "stdout"})
	fields := map[string]any{
		"component": "benchmark",
		"operation": "test",
		"count":     42,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fieldLogger := logger.WithFields(fields)
		_ = fieldLogger
	}
}

func BenchmarkLogMessage(b *testing.B) {
	// Use a temp file to avoid stdout interference
	tempFile := filepath.Join(b.TempDir(), "bench.log")
	logger, _ := New(Config{Level: LevelInfo, Format: "text", Output: tempFile})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark log message")
	}
}

func TestTimeoutCreatingFileLogger(t *testing.T) {
	// Create a directory without write permissions to test error handling
	tempDir := t.TempDir()
	restrictedDir := filepath.Join(tempDir, "restricted")

	if err := os.Mkdir(restrictedDir, 0444); err != nil {
		t.Skipf("Cannot create restricted directory: %v", err)
	}

	config := Config{
		Level:  LevelInfo,
		Format: "text",
		Output: filepath.Join(restrictedDir, "test.log"),
	}

	_, err := New(config)
	if err == nil {
		t.Error("Expected error when creating logger with restricted directory")
	}
}

// Test edge cases and error conditions
func TestLoggerEdgeCases(t *testing.T) {
	// Test with nil fields
	logger, _ := New(Config{})

	// Should not panic with nil fields
	nilLogger := logger.WithFields(nil)
	if nilLogger == nil {
		t.Error("WithFields should handle nil fields gracefully")
	}

	// Test with empty fields map
	emptyLogger := logger.WithFields(map[string]any{})
	if emptyLogger == nil {
		t.Error("WithFields should handle empty fields map")
	}

	// Test context operations don't panic
	ctx := context.Background()
	contextLogger := logger.WithContext(ctx)
	if contextLogger == nil {
		t.Error("WithContext should not return nil")
	}

	// Test logging operations don't panic
	logger.LogDBOperation("", "", 0, nil)
	logger.LogHTTPRequest("", "", "", 0, 0)
	logger.LogDiscoveryOperation("", "", 0, 0, nil)
	logger.LogDeviceOperation("", "", "", nil)
	logger.LogAppStart("", "")
	logger.LogAppStop("")
	logger.LogConfigLoad("", nil)
}
