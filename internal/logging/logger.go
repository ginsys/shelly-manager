package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Logger levels
const (
	LevelDebug = "debug"
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
)

// Config holds logging configuration
type Config struct {
	Level  string // debug, info, warn, error
	Format string // json, text
	Output string // stdout, stderr, or file path
}

// Logger wraps slog.Logger with additional functionality
type Logger struct {
	*slog.Logger
	level  slog.Level
	config Config
}

// New creates a new structured logger
func New(config Config) (*Logger, error) {
	// Set default values
	if config.Level == "" {
		config.Level = LevelInfo
	}
	if config.Format == "" {
		config.Format = "text"
	}
	if config.Output == "" {
		config.Output = "stdout"
	}

	// Parse log level
	level, err := parseLevel(config.Level)
	if err != nil {
		return nil, err
	}

	// Get output writer
	writer, err := getWriter(config.Output)
	if err != nil {
		return nil, err
	}

	// Create handler options
	opts := &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Customize timestamp format
			if a.Key == slog.TimeKey {
				return slog.String("timestamp", a.Value.Time().Format("2006-01-02T15:04:05.000Z07:00"))
			}
			return a
		},
	}

	// Create handler based on format
	var handler slog.Handler
	switch config.Format {
	case "json":
		handler = slog.NewJSONHandler(writer, opts)
	default:
		handler = slog.NewTextHandler(writer, opts)
	}

	logger := slog.New(handler)

	return &Logger{
		Logger: logger,
		level:  level,
		config: config,
	}, nil
}

// parseLevel converts string level to slog.Level
func parseLevel(levelStr string) (slog.Level, error) {
	switch strings.ToLower(levelStr) {
	case LevelDebug:
		return slog.LevelDebug, nil
	case LevelInfo:
		return slog.LevelInfo, nil
	case LevelWarn:
		return slog.LevelWarn, nil
	case LevelError:
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, nil
	}
}

// getWriter returns the appropriate writer for output
func getWriter(output string) (io.Writer, error) {
	switch output {
	case "stdout":
		return os.Stdout, nil
	case "stderr":
		return os.Stderr, nil
	default:
		// Assume it's a file path
		file, err := os.OpenFile(output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, err
		}
		return file, nil
	}
}

// WithFields adds structured fields to the logger
func (l *Logger) WithFields(fields map[string]any) *Logger {
	args := make([]any, 0, len(fields)*2)
	for k, v := range fields {
		args = append(args, k, v)
	}
	
	return &Logger{
		Logger: l.Logger.With(args...),
		level:  l.level,
		config: l.config,
	}
}

// WithContext adds context values to the logger if they exist
func (l *Logger) WithContext(ctx context.Context) *Logger {
	// Extract common context values
	fields := make(map[string]any)
	
	if requestID := ctx.Value("request_id"); requestID != nil {
		fields["request_id"] = requestID
	}
	
	if userID := ctx.Value("user_id"); userID != nil {
		fields["user_id"] = userID
	}
	
	if len(fields) == 0 {
		return l
	}
	
	return l.WithFields(fields)
}

// Database operation logging helpers
func (l *Logger) LogDBOperation(operation, table string, duration int64, err error) {
	fields := map[string]any{
		"operation": operation,
		"table":     table,
		"duration":  duration, // microseconds
		"component": "database",
	}

	if err != nil {
		fields["error"] = err.Error()
		l.WithFields(fields).Error("Database operation failed")
	} else {
		l.WithFields(fields).Debug("Database operation completed")
	}
}

// HTTP operation logging helpers
func (l *Logger) LogHTTPRequest(method, path, remoteAddr string, statusCode int, duration int64) {
	fields := map[string]any{
		"method":      method,
		"path":        path,
		"remote_addr": remoteAddr,
		"status_code": statusCode,
		"duration":    duration, // milliseconds
		"component":   "http",
	}

	level := slog.LevelInfo
	if statusCode >= 400 {
		level = slog.LevelWarn
	}
	if statusCode >= 500 {
		level = slog.LevelError
	}

	l.WithFields(fields).Log(context.Background(), level, "HTTP request completed")
}

// Discovery operation logging helpers
func (l *Logger) LogDiscoveryOperation(operation string, network string, devicesFound int, duration int64, err error) {
	fields := map[string]any{
		"operation":      operation,
		"network":        network,
		"devices_found":  devicesFound,
		"duration":       duration, // milliseconds
		"component":      "discovery",
	}

	if err != nil {
		fields["error"] = err.Error()
		l.WithFields(fields).Error("Discovery operation failed")
	} else {
		l.WithFields(fields).Info("Discovery operation completed")
	}
}

// Device operation logging helpers
func (l *Logger) LogDeviceOperation(operation, deviceIP, deviceMAC string, err error) {
	fields := map[string]any{
		"operation":  operation,
		"device_ip":  deviceIP,
		"device_mac": deviceMAC,
		"component":  "device",
	}

	if err != nil {
		fields["error"] = err.Error()
		l.WithFields(fields).Error("Device operation failed")
	} else {
		l.WithFields(fields).Info("Device operation completed")
	}
}

// Application lifecycle logging helpers
func (l *Logger) LogAppStart(version, addr string) {
	l.WithFields(map[string]any{
		"version":   version,
		"address":   addr,
		"component": "app",
	}).Info("Application starting")
}

func (l *Logger) LogAppStop(reason string) {
	l.WithFields(map[string]any{
		"reason":    reason,
		"component": "app",
	}).Info("Application stopping")
}

// Configuration logging
func (l *Logger) LogConfigLoad(configPath string, err error) {
	fields := map[string]any{
		"config_path": configPath,
		"component":   "config",
	}

	if err != nil {
		fields["error"] = err.Error()
		l.WithFields(fields).Error("Configuration load failed")
	} else {
		l.WithFields(fields).Info("Configuration loaded successfully")
	}
}

// Default logger instance
var defaultLogger *Logger

// SetDefault sets the default logger instance
func SetDefault(logger *Logger) {
	defaultLogger = logger
}

// GetDefault returns the default logger instance
func GetDefault() *Logger {
	if defaultLogger == nil {
		// Create a basic logger if none is set
		config := Config{
			Level:  LevelInfo,
			Format: "text",
			Output: "stdout",
		}
		logger, _ := New(config)
		defaultLogger = logger
	}
	return defaultLogger
}