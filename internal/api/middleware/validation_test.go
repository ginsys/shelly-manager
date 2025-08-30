package middleware

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ginsys/shelly-manager/internal/logging"
)

func TestValidateContentTypeMiddleware(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})
	config := DefaultValidationConfig()

	tests := []struct {
		name           string
		method         string
		contentType    string
		expectedStatus int
		description    string
	}{
		{
			name:           "GET Request No Content-Type",
			method:         "GET",
			contentType:    "",
			expectedStatus: http.StatusOK,
			description:    "GET requests should not require Content-Type",
		},
		{
			name:           "POST Valid JSON Content-Type",
			method:         "POST",
			contentType:    "application/json",
			expectedStatus: http.StatusOK,
			description:    "Valid JSON content type should be accepted",
		},
		{
			name:           "POST Valid JSON with Charset",
			method:         "POST",
			contentType:    "application/json; charset=utf-8",
			expectedStatus: http.StatusOK,
			description:    "JSON with charset should be accepted",
		},
		{
			name:           "POST Valid Form Content-Type",
			method:         "POST",
			contentType:    "application/x-www-form-urlencoded",
			expectedStatus: http.StatusOK,
			description:    "Form content type should be accepted",
		},
		{
			name:           "POST Valid Multipart Content-Type",
			method:         "POST",
			contentType:    "multipart/form-data; boundary=----WebKitFormBoundary7MA4YWxkTrZu0gW",
			expectedStatus: http.StatusOK,
			description:    "Multipart content type should be accepted",
		},
		{
			name:           "POST Missing Content-Type Strict",
			method:         "POST",
			contentType:    "",
			expectedStatus: http.StatusBadRequest,
			description:    "POST without Content-Type should be rejected in strict mode",
		},
		{
			name:           "POST Invalid Content-Type",
			method:         "POST",
			contentType:    "application/xml",
			expectedStatus: http.StatusUnsupportedMediaType,
			description:    "Unsupported content type should be rejected",
		},
		{
			name:           "POST Malformed Content-Type",
			method:         "POST",
			contentType:    "application/json; charset=utf-8; boundary=",
			expectedStatus: http.StatusBadRequest,
			description:    "Malformed content type should be rejected",
		},
		{
			name:           "DELETE Request No Content-Type",
			method:         "DELETE",
			contentType:    "",
			expectedStatus: http.StatusOK,
			description:    "DELETE requests should not require Content-Type",
		},
		{
			name:           "OPTIONS Request No Content-Type",
			method:         "OPTIONS",
			contentType:    "",
			expectedStatus: http.StatusOK,
			description:    "OPTIONS requests should not require Content-Type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := ValidateContentTypeMiddleware(config, logger)

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			var body *strings.Reader
			if tt.method == "POST" || tt.method == "PUT" {
				body = strings.NewReader(`{"test": "data"}`)
			} else {
				body = strings.NewReader("")
			}

			req := httptest.NewRequest(tt.method, "/api/v1/test", body)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			rr := httptest.NewRecorder()
			middleware(handler).ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code, tt.description)
		})
	}
}

func TestValidateHeadersMiddleware(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})

	tests := []struct {
		name           string
		config         *ValidationConfig
		setupHeaders   func(*http.Request)
		expectedStatus int
		description    string
	}{
		{
			name:   "Normal Headers",
			config: DefaultValidationConfig(),
			setupHeaders: func(r *http.Request) {
				r.Header.Set("Content-Type", "application/json")
				r.Header.Set("User-Agent", "TestAgent/1.0")
			},
			expectedStatus: http.StatusOK,
			description:    "Normal headers should be accepted",
		},
		{
			name: "Too Many Headers",
			config: func() *ValidationConfig {
				cfg := DefaultValidationConfig()
				cfg.MaxHeaderCount = 3
				return cfg
			}(),
			setupHeaders: func(r *http.Request) {
				for i := 0; i < 10; i++ {
					r.Header.Set(fmt.Sprintf("Header%d", i), "value")
				}
			},
			expectedStatus: http.StatusBadRequest,
			description:    "Too many headers should be rejected",
		},
		{
			name: "Large Header Value",
			config: func() *ValidationConfig {
				cfg := DefaultValidationConfig()
				cfg.MaxHeaderSize = 100
				return cfg
			}(),
			setupHeaders: func(r *http.Request) {
				r.Header.Set("Large-Header", strings.Repeat("x", 200))
			},
			expectedStatus: http.StatusBadRequest,
			description:    "Large header values should be rejected",
		},
		{
			name:   "Forbidden Header",
			config: DefaultValidationConfig(),
			setupHeaders: func(r *http.Request) {
				r.Header.Set("X-Forwarded-Proto", "https")
			},
			expectedStatus: http.StatusBadRequest,
			description:    "Forbidden headers should be rejected",
		},
		{
			name: "Required Header Missing",
			config: func() *ValidationConfig {
				cfg := DefaultValidationConfig()
				cfg.RequiredHeaders = []string{"Authorization"}
				return cfg
			}(),
			setupHeaders: func(r *http.Request) {
				// Don't set Authorization header
			},
			expectedStatus: http.StatusBadRequest,
			description:    "Missing required headers should be rejected",
		},
		{
			name: "Suspicious Header Content",
			config: func() *ValidationConfig {
				cfg := DefaultValidationConfig()
				cfg.BlockSuspiciousHeaders = true
				return cfg
			}(),
			setupHeaders: func(r *http.Request) {
				r.Header.Set("Custom-Header", "<script>alert('xss')</script>")
			},
			expectedStatus: http.StatusBadRequest,
			description:    "Suspicious header content should be rejected",
		},
		{
			name: "Suspicious User Agent",
			config: func() *ValidationConfig {
				cfg := DefaultValidationConfig()
				cfg.BlockSuspiciousUserAgents = true
				return cfg
			}(),
			setupHeaders: func(r *http.Request) {
				r.Header.Set("User-Agent", "sqlmap/1.0")
			},
			expectedStatus: http.StatusBadRequest,
			description:    "Suspicious user agents should be rejected",
		},
		{
			name: "Empty User Agent",
			config: func() *ValidationConfig {
				cfg := DefaultValidationConfig()
				cfg.BlockSuspiciousUserAgents = true
				return cfg
			}(),
			setupHeaders: func(r *http.Request) {
				r.Header.Set("User-Agent", "")
			},
			expectedStatus: http.StatusBadRequest,
			description:    "Empty user agent should be rejected when blocking suspicious agents",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := ValidateHeadersMiddleware(tt.config, logger)

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest("POST", "/api/v1/test", strings.NewReader(`{}`))
			tt.setupHeaders(req)

			rr := httptest.NewRecorder()
			middleware(handler).ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code, tt.description)
		})
	}
}

func TestValidateJSONMiddleware(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})
	config := DefaultValidationConfig()

	tests := []struct {
		name           string
		contentType    string
		requestBody    string
		expectedStatus int
		description    string
	}{
		{
			name:           "Valid JSON",
			contentType:    "application/json",
			requestBody:    `{"name": "test", "value": 123}`,
			expectedStatus: http.StatusOK,
			description:    "Valid JSON should be accepted",
		},
		{
			name:           "Invalid JSON Syntax",
			contentType:    "application/json",
			requestBody:    `{"name": "test", "value": 123`,
			expectedStatus: http.StatusBadRequest,
			description:    "Invalid JSON syntax should be rejected",
		},
		{
			name:           "Malformed JSON",
			contentType:    "application/json",
			requestBody:    `{name: "test"}`, // Missing quotes around key
			expectedStatus: http.StatusBadRequest,
			description:    "Malformed JSON should be rejected",
		},
		{
			name:           "Empty JSON Body",
			contentType:    "application/json",
			requestBody:    "",
			expectedStatus: http.StatusOK,
			description:    "Empty JSON body should be accepted",
		},
		{
			name:           "Non-JSON Content Type",
			contentType:    "text/plain",
			requestBody:    `{"name": "test"}`,
			expectedStatus: http.StatusOK,
			description:    "Non-JSON content should be skipped",
		},
		{
			name:           "Complex Valid JSON",
			contentType:    "application/json",
			requestBody:    `{"user": {"name": "John", "age": 30}, "items": [1, 2, 3], "active": true}`,
			expectedStatus: http.StatusOK,
			description:    "Complex valid JSON should be accepted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := ValidateJSONMiddleware(config, logger)

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest("POST", "/api/v1/test", strings.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", tt.contentType)

			rr := httptest.NewRecorder()
			middleware(handler).ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code, tt.description)
		})
	}
}

func TestValidateJSONStructure(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})

	tests := []struct {
		name           string
		config         *ValidationConfig
		requestBody    string
		expectedStatus int
		description    string
	}{
		{
			name: "Deep Nesting Within Limit",
			config: func() *ValidationConfig {
				cfg := DefaultValidationConfig()
				cfg.MaxJSONDepth = 5
				return cfg
			}(),
			requestBody:    `{"a": {"b": {"c": {"d": {"e": "value"}}}}}`, // 5 levels
			expectedStatus: http.StatusOK,
			description:    "JSON within depth limit should be accepted",
		},
		{
			name: "Deep Nesting Exceeds Limit",
			config: func() *ValidationConfig {
				cfg := DefaultValidationConfig()
				cfg.MaxJSONDepth = 3
				return cfg
			}(),
			requestBody:    `{"a": {"b": {"c": {"d": {"e": "value"}}}}}`, // 5 levels
			expectedStatus: http.StatusBadRequest,
			description:    "JSON exceeding depth limit should be rejected",
		},
		{
			name: "Large Array Within Limit",
			config: func() *ValidationConfig {
				cfg := DefaultValidationConfig()
				cfg.MaxJSONArraySize = 5
				return cfg
			}(),
			requestBody:    `{"items": [1, 2, 3, 4, 5]}`, // 5 elements
			expectedStatus: http.StatusOK,
			description:    "Array within size limit should be accepted",
		},
		{
			name: "Large Array Exceeds Limit",
			config: func() *ValidationConfig {
				cfg := DefaultValidationConfig()
				cfg.MaxJSONArraySize = 3
				return cfg
			}(),
			requestBody:    `{"items": [1, 2, 3, 4, 5]}`, // 5 elements
			expectedStatus: http.StatusBadRequest,
			description:    "Array exceeding size limit should be rejected",
		},
		{
			name: "Nested Arrays",
			config: func() *ValidationConfig {
				cfg := DefaultValidationConfig()
				cfg.MaxJSONArraySize = 2
				cfg.MaxJSONDepth = 3
				return cfg
			}(),
			requestBody:    `{"outer": [{"inner": [1, 2]}, {"inner": [3, 4]}]}`,
			expectedStatus: http.StatusOK,
			description:    "Nested arrays within limits should be accepted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := ValidateJSONMiddleware(tt.config, logger)

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest("POST", "/api/v1/test", strings.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			middleware(handler).ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code, tt.description)
		})
	}
}

func TestValidateQueryParamsMiddleware(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})

	tests := []struct {
		name           string
		config         *ValidationConfig
		queryString    string
		expectedStatus int
		description    string
	}{
		{
			name:           "Normal Query Params",
			config:         DefaultValidationConfig(),
			queryString:    "?name=test&value=123&active=true",
			expectedStatus: http.StatusOK,
			description:    "Normal query parameters should be accepted",
		},
		{
			name: "Too Many Query Params",
			config: func() *ValidationConfig {
				cfg := DefaultValidationConfig()
				cfg.MaxQueryParamCount = 3
				return cfg
			}(),
			queryString:    "?a=1&b=2&c=3&d=4&e=5",
			expectedStatus: http.StatusBadRequest,
			description:    "Too many query parameters should be rejected",
		},
		{
			name: "Large Query Param Value",
			config: func() *ValidationConfig {
				cfg := DefaultValidationConfig()
				cfg.MaxQueryParamSize = 10
				return cfg
			}(),
			queryString:    "?data=" + strings.Repeat("x", 20),
			expectedStatus: http.StatusBadRequest,
			description:    "Large query parameter values should be rejected",
		},
		{
			name:           "Forbidden Query Param",
			config:         DefaultValidationConfig(),
			queryString:    "?__proto__=malicious",
			expectedStatus: http.StatusBadRequest,
			description:    "Forbidden query parameters should be rejected",
		},
		{
			name:           "Suspicious Query Param Content",
			config:         DefaultValidationConfig(),
			queryString:    "?search=<script>alert('xss')</script>",
			expectedStatus: http.StatusBadRequest,
			description:    "Suspicious query parameter content should be rejected",
		},
		{
			name:           "SQL Injection in Query Param",
			config:         DefaultValidationConfig(),
			queryString:    "?id=" + url.QueryEscape("1' OR '1'='1"),
			expectedStatus: http.StatusBadRequest,
			description:    "SQL injection attempts in query params should be rejected",
		},
		{
			name:           "Path Traversal in Query Param",
			config:         DefaultValidationConfig(),
			queryString:    "?file=../../../etc/passwd",
			expectedStatus: http.StatusBadRequest,
			description:    "Path traversal attempts in query params should be rejected",
		},
		{
			name:           "Multiple Forbidden Params",
			config:         DefaultValidationConfig(),
			queryString:    "?constructor=evil&prototype=malicious",
			expectedStatus: http.StatusBadRequest,
			description:    "Multiple forbidden parameters should be rejected",
		},
		{
			name:           "Empty Query String",
			config:         DefaultValidationConfig(),
			queryString:    "",
			expectedStatus: http.StatusOK,
			description:    "Empty query string should be accepted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := ValidateQueryParamsMiddleware(tt.config, logger)

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest("GET", "/api/v1/test"+tt.queryString, nil)

			rr := httptest.NewRecorder()
			middleware(handler).ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code, tt.description)
		})
	}
}

func TestPrototypePollutionProtection(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})
	config := DefaultValidationConfig()

	prototypePollutionPayloads := []string{
		"__proto__",
		"constructor",
		"prototype",
	}

	middleware := ValidateQueryParamsMiddleware(config, logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for _, payload := range prototypePollutionPayloads {
		t.Run(fmt.Sprintf("PrototypePollution_%s", payload), func(t *testing.T) {
			queryString := fmt.Sprintf("?%s=malicious_value", payload)
			req := httptest.NewRequest("GET", "/api/v1/test"+queryString, nil)

			rr := httptest.NewRecorder()
			middleware(handler).ServeHTTP(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code,
				"Should block prototype pollution attempt with %s", payload)
		})
	}
}

func TestContentTypeValidationEdgeCases(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})

	tests := []struct {
		name           string
		config         *ValidationConfig
		contentType    string
		expectedStatus int
		description    string
	}{
		{
			name: "Non-Strict Mode Missing Content-Type",
			config: func() *ValidationConfig {
				cfg := DefaultValidationConfig()
				cfg.StrictContentType = false
				return cfg
			}(),
			contentType:    "",
			expectedStatus: http.StatusOK,
			description:    "Non-strict mode should allow missing Content-Type",
		},
		{
			name:           "Case Insensitive Content-Type",
			config:         DefaultValidationConfig(),
			contentType:    "APPLICATION/JSON",
			expectedStatus: http.StatusOK,
			description:    "Content-Type should be case insensitive",
		},
		{
			name:           "Content-Type with Extra Parameters",
			config:         DefaultValidationConfig(),
			contentType:    "application/json; charset=utf-8; boundary=something",
			expectedStatus: http.StatusOK,
			description:    "Should handle content-type with multiple parameters",
		},
		{
			name:           "Malformed Content-Type Header",
			config:         DefaultValidationConfig(),
			contentType:    "application/json; charset",
			expectedStatus: http.StatusBadRequest,
			description:    "Should reject malformed content-type header",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := ValidateContentTypeMiddleware(tt.config, logger)

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest("POST", "/api/v1/test", strings.NewReader(`{}`))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			rr := httptest.NewRecorder()
			middleware(handler).ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code, tt.description)
		})
	}
}

func TestSuspiciousContentDetection(t *testing.T) {
	suspiciousPatterns := []string{
		"<script>alert('xss')</script>",
		"javascript:alert(1)",
		"data:text/html,<script>alert(1)</script>",
		"vbscript:msgbox(1)",
		"onload=alert(1)",
		"onerror=alert(1)",
		"onclick=alert(1)",
		"onmouseover=alert(1)",
		"eval(alert(1))",
		"alert(document.cookie)",
		"confirm('xss')",
		"prompt('xss')",
		"document.cookie",
		"document.domain",
		"../../../etc/passwd",
		"..\\..\\..\\windows\\system32",
		"/etc/passwd",
		"/proc/version",
		"union select * from users",
		"drop table users",
		"delete from users",
		"insert into users",
		"update users set",
		"' or 1=1 --",
	}

	for i, pattern := range suspiciousPatterns {
		t.Run(fmt.Sprintf("SuspiciousPattern_%d", i+1), func(t *testing.T) {
			result := containsSuspiciousContent(pattern)
			assert.True(t, result, "Should detect suspicious pattern: %s", pattern)
		})
	}

	// Test legitimate content
	legitimateContent := []string{
		"user@example.com",
		"normal text content",
		"123456789",
		"product-name-123",
		"https://example.com/api/endpoint",
		"GET /api/v1/users",
		"application/json",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
	}

	for i, content := range legitimateContent {
		t.Run(fmt.Sprintf("LegitimateContent_%d", i+1), func(t *testing.T) {
			result := containsSuspiciousContent(content)
			assert.False(t, result, "Should not flag legitimate content: %s", content)
		})
	}
}

func TestUserAgentValidation(t *testing.T) {
	maliciousUserAgents := []string{
		"sqlmap/1.0",
		"Nikto/2.1.6",
		"Nessus SOAP v3.0",
		"BurpSuite Professional",
		"Nmap Scripting Engine",
		"w3af.org",
		"skipfish/2.10b",
		"Arachni/v1.5.1",
		"WPScan v3.8.7",
		"python-requests/2.25.1", // Often used by bots
		"curl/7.68.0",            // Command line tool
		"wget/1.20.3",            // Command line tool
		"",                       // Empty user agent
		"x",                      // Too short
		strings.Repeat("x", 600), // Too long
	}

	for i, userAgent := range maliciousUserAgents {
		t.Run(fmt.Sprintf("MaliciousUserAgent_%d", i+1), func(t *testing.T) {
			result := isSuspiciousUserAgent(userAgent)
			assert.True(t, result, "Should detect malicious user agent: %s", userAgent)
		})
	}

	// Test legitimate user agents
	legitimateUserAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 14_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Mobile/15E148 Safari/604.1",
		"MyApp/1.0 (iOS 14.6; iPhone12,1)",
		"PostmanRuntime/7.28.0",
		"RestClient/1.0",
	}

	for i, userAgent := range legitimateUserAgents {
		t.Run(fmt.Sprintf("LegitimateUserAgent_%d", i+1), func(t *testing.T) {
			result := isSuspiciousUserAgent(userAgent)
			assert.False(t, result, "Should not flag legitimate user agent: %s", userAgent)
		})
	}
}

func TestMultipartFormValidation(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})
	config := DefaultValidationConfig()

	// Create multipart form data
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add form fields
	_ = writer.WriteField("name", "test")
	_ = writer.WriteField("value", "123")

	// Add file field
	fileWriter, err := writer.CreateFormFile("file", "test.txt")
	require.NoError(t, err)
	_, _ = fileWriter.Write([]byte("test file content"))

	_ = writer.Close()

	middleware := ValidateContentTypeMiddleware(config, logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/api/v1/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rr := httptest.NewRecorder()
	middleware(handler).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Valid multipart form should be accepted")
}

func TestValidationMiddlewareChaining(t *testing.T) {
	logger, _ := logging.New(logging.Config{Level: "debug", Format: "text", Output: "stdout"})
	config := DefaultValidationConfig()

	// Chain all validation middleware
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Apply middleware in reverse order (since they wrap the handler)
	chainedHandler := ValidateQueryParamsMiddleware(config, logger)(
		ValidateJSONMiddleware(config, logger)(
			ValidateHeadersMiddleware(config, logger)(
				ValidateContentTypeMiddleware(config, logger)(handler),
			),
		),
	)

	// Test with valid request
	t.Run("ValidRequest", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/test?param=value",
			strings.NewReader(`{"name": "test"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "TestAgent/1.0")

		rr := httptest.NewRecorder()
		chainedHandler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code, "Valid request should pass all validation")
	})

	// Test with invalid request (multiple validation failures)
	t.Run("InvalidRequest", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/test?__proto__=evil&search=<script>alert('xss')</script>",
			strings.NewReader(`{"invalid": json`))
		req.Header.Set("Content-Type", "application/xml") // Unsupported
		req.Header.Set("User-Agent", "sqlmap/1.0")        // Suspicious

		rr := httptest.NewRecorder()
		chainedHandler.ServeHTTP(rr, req)

		// Should fail at the first validation step (content-type)
		assert.Equal(t, http.StatusUnsupportedMediaType, rr.Code,
			"Invalid request should be rejected at first failing validation")
	})
}

func BenchmarkValidationMiddleware(b *testing.B) {
	logger, _ := logging.New(logging.Config{Level: "error", Format: "text", Output: "stdout"}) // Reduce logging for benchmarks
	config := DefaultValidationConfig()

	b.Run("ContentTypeValidation", func(b *testing.B) {
		middleware := ValidateContentTypeMiddleware(config, logger)
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		body := strings.NewReader(`{"test": "data"}`)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = body.Seek(0, 0)
			req := httptest.NewRequest("POST", "/api/v1/test", body)
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			middleware(handler).ServeHTTP(rr, req)
		}
	})

	b.Run("HeaderValidation", func(b *testing.B) {
		middleware := ValidateHeadersMiddleware(config, logger)
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest("POST", "/api/v1/test", nil)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("User-Agent", "TestAgent/1.0")
			rr := httptest.NewRecorder()
			middleware(handler).ServeHTTP(rr, req)
		}
	})

	b.Run("JSONValidation", func(b *testing.B) {
		middleware := ValidateJSONMiddleware(config, logger)
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		body := strings.NewReader(`{"name": "test", "value": 123, "active": true}`)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = body.Seek(0, 0)
			req := httptest.NewRequest("POST", "/api/v1/test", body)
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			middleware(handler).ServeHTTP(rr, req)
		}
	})

	b.Run("QueryParamValidation", func(b *testing.B) {
		middleware := ValidateQueryParamsMiddleware(config, logger)
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest("GET", "/api/v1/test?name=value&id=123&active=true", nil)
			rr := httptest.NewRecorder()
			middleware(handler).ServeHTTP(rr, req)
		}
	})
}
