package logging

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"time"
)

// contextKey is a type for context keys to avoid collisions
type contextKey string

const requestIDKey contextKey = "request_id"

// GetRequestID extracts the request ID from a context
func GetRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value(requestIDKey).(string); ok {
		return requestID
	}
	return ""
}

// WithRequestID adds a request ID to a context (useful for testing)
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(data []byte) (int, error) {
	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}
	n, err := rw.ResponseWriter.Write(data)
	rw.size += n
	return n, err
}

// Hijack implements http.Hijacker interface for WebSocket support
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// HTTPMiddleware returns a middleware that logs HTTP requests
func HTTPMiddleware(logger *Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap the response writer
			wrapped := &responseWriter{
				ResponseWriter: w,
				statusCode:     0,
			}

			// Add request ID to context if not present
			ctx := r.Context()
			if ctx.Value(requestIDKey) == nil {
				requestID := generateRequestID()
				ctx = context.WithValue(ctx, requestIDKey, requestID)
				r = r.WithContext(ctx)
			}

			// Call the next handler
			next.ServeHTTP(wrapped, r)

			// Calculate duration
			duration := time.Since(start).Milliseconds()

			// Log the request
			logger.LogHTTPRequest(
				r.Method,
				r.URL.Path,
				r.RemoteAddr,
				wrapped.statusCode,
				duration,
			)
		})
	}
}

// generateRequestID creates a simple request ID
func generateRequestID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

// randomString generates a cryptographically random string of given length
func randomString(length int) string {
	// Generate random bytes, need half the length since hex encoding doubles the size
	bytes := make([]byte, (length+1)/2)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if crypto/rand fails
		return time.Now().Format("150405")
	}

	// Convert to hex and truncate to desired length
	hexStr := hex.EncodeToString(bytes)
	if len(hexStr) > length {
		return hexStr[:length]
	}
	return hexStr
}

// Recovery middleware with logging
func RecoveryMiddleware(logger *Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logger.WithFields(map[string]any{
						"method":      r.Method,
						"path":        r.URL.Path,
						"remote_addr": r.RemoteAddr,
						"panic":       err,
						"component":   "http",
					}).Error("HTTP request panicked")

					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// CORS middleware with logging
func CORSMiddleware(logger *Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Set CORS headers
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			// Log CORS requests
			if origin != "" {
				logger.WithFields(map[string]any{
					"method":    r.Method,
					"path":      r.URL.Path,
					"origin":    origin,
					"component": "cors",
				}).Debug("CORS request processed")
			}

			// Handle preflight requests
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
