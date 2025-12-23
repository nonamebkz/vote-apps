package logger

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

// LogLevel represents the severity level of a log entry
type LogLevel string

const (
	DEBUG LogLevel = "DEBUG"
	INFO  LogLevel = "INFO"
	WARN  LogLevel = "WARN"
	ERROR LogLevel = "ERROR"
	FATAL LogLevel = "FATAL"
)

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     LogLevel               `json:"level"`
	Message   string                 `json:"message"`
	RequestID string                 `json:"request_id,omitempty"`
	UserID    uint                   `json:"user_id,omitempty"`
	Method    string                 `json:"method,omitempty"`
	Path      string                 `json:"path,omitempty"`
	Status    int                    `json:"status,omitempty"`
	Duration  string                 `json:"duration,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// Logger provides structured logging functionality
type Logger struct {
	level  LogLevel
	output *log.Logger
}

// NewLogger creates a new structured logger
func NewLogger(level LogLevel) *Logger {
	return &Logger{
		level:  level,
		output: log.New(os.Stdout, "", 0),
	}
}

// shouldLog checks if a message should be logged based on the current log level
func (l *Logger) shouldLog(level LogLevel) bool {
	levels := map[LogLevel]int{
		DEBUG: 0,
		INFO:  1,
		WARN:  2,
		ERROR: 3,
		FATAL: 4,
	}
	
	currentLevel, exists := levels[l.level]
	if !exists {
		currentLevel = levels[INFO]
	}
	
	messageLevel, exists := levels[level]
	if !exists {
		messageLevel = levels[INFO]
	}
	
	return messageLevel >= currentLevel
}

// log writes a structured log entry
func (l *Logger) log(level LogLevel, message string, fields map[string]interface{}) {
	if !l.shouldLog(level) {
		return
	}
	
	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     level,
		Message:   message,
		Fields:    fields,
	}
	
	// Add request-specific fields if available
	if requestID, ok := fields["request_id"].(string); ok {
		entry.RequestID = requestID
		delete(fields, "request_id")
	}
	
	if userID, ok := fields["user_id"].(uint); ok {
		entry.UserID = userID
		delete(fields, "user_id")
	}
	
	if method, ok := fields["method"].(string); ok {
		entry.Method = method
		delete(fields, "method")
	}
	
	if path, ok := fields["path"].(string); ok {
		entry.Path = path
		delete(fields, "path")
	}
	
	if status, ok := fields["status"].(int); ok {
		entry.Status = status
		delete(fields, "status")
	}
	
	if duration, ok := fields["duration"].(string); ok {
		entry.Duration = duration
		delete(fields, "duration")
	}
	
	if err, ok := fields["error"].(string); ok {
		entry.Error = err
		delete(fields, "error")
	}
	
	// Marshal to JSON
	jsonData, err := json.Marshal(entry)
	if err != nil {
		// Fallback to simple logging if JSON marshaling fails
		l.output.Printf("[%s] %s %s", level, message, err.Error())
		return
	}
	
	l.output.Println(string(jsonData))
}

// Debug logs a debug message
func (l *Logger) Debug(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(DEBUG, message, f)
}

// Info logs an info message
func (l *Logger) Info(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(INFO, message, f)
}

// Warn logs a warning message
func (l *Logger) Warn(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(WARN, message, f)
}

// Error logs an error message
func (l *Logger) Error(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(ERROR, message, f)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(FATAL, message, f)
	os.Exit(1)
}

// WithFields creates a new logger with additional fields
func (l *Logger) WithFields(fields map[string]interface{}) *FieldLogger {
	return &FieldLogger{
		logger: l,
		fields: fields,
	}
}

// FieldLogger is a logger with predefined fields
type FieldLogger struct {
	logger *Logger
	fields map[string]interface{}
}

// Debug logs a debug message with predefined fields
func (fl *FieldLogger) Debug(message string, additionalFields ...map[string]interface{}) {
	fields := make(map[string]interface{})
	for k, v := range fl.fields {
		fields[k] = v
	}
	if len(additionalFields) > 0 {
		for k, v := range additionalFields[0] {
			fields[k] = v
		}
	}
	fl.logger.log(DEBUG, message, fields)
}

// Info logs an info message with predefined fields
func (fl *FieldLogger) Info(message string, additionalFields ...map[string]interface{}) {
	fields := make(map[string]interface{})
	for k, v := range fl.fields {
		fields[k] = v
	}
	if len(additionalFields) > 0 {
		for k, v := range additionalFields[0] {
			fields[k] = v
		}
	}
	fl.logger.log(INFO, message, fields)
}

// Warn logs a warning message with predefined fields
func (fl *FieldLogger) Warn(message string, additionalFields ...map[string]interface{}) {
	fields := make(map[string]interface{})
	for k, v := range fl.fields {
		fields[k] = v
	}
	if len(additionalFields) > 0 {
		for k, v := range additionalFields[0] {
			fields[k] = v
		}
	}
	fl.logger.log(WARN, message, fields)
}

// Error logs an error message with predefined fields
func (fl *FieldLogger) Error(message string, additionalFields ...map[string]interface{}) {
	fields := make(map[string]interface{})
	for k, v := range fl.fields {
		fields[k] = v
	}
	if len(additionalFields) > 0 {
		for k, v := range additionalFields[0] {
			fields[k] = v
		}
	}
	fl.logger.log(ERROR, message, fields)
}

// Fatal logs a fatal message with predefined fields and exits
func (fl *FieldLogger) Fatal(message string, additionalFields ...map[string]interface{}) {
	fields := make(map[string]interface{})
	for k, v := range fl.fields {
		fields[k] = v
	}
	if len(additionalFields) > 0 {
		for k, v := range additionalFields[0] {
			fields[k] = v
		}
	}
	fl.logger.log(FATAL, message, fields)
	os.Exit(1)
}

// Global logger instance
var defaultLogger *Logger

// Initialize the default logger
func init() {
	level := INFO
	if envLevel := os.Getenv("LOG_LEVEL"); envLevel != "" {
		switch envLevel {
		case "DEBUG":
			level = DEBUG
		case "WARN":
			level = WARN
		case "ERROR":
			level = ERROR
		case "FATAL":
			level = FATAL
		default:
			level = INFO
		}
	}
	defaultLogger = NewLogger(level)
}

// Package-level logging functions
func Debug(message string, fields ...map[string]interface{}) {
	defaultLogger.Debug(message, fields...)
}

func Info(message string, fields ...map[string]interface{}) {
	defaultLogger.Info(message, fields...)
}

func Warn(message string, fields ...map[string]interface{}) {
	defaultLogger.Warn(message, fields...)
}

func Error(message string, fields ...map[string]interface{}) {
	defaultLogger.Error(message, fields...)
}

func Fatal(message string, fields ...map[string]interface{}) {
	defaultLogger.Fatal(message, fields...)
}

func WithFields(fields map[string]interface{}) *FieldLogger {
	return defaultLogger.WithFields(fields)
}

// HTTP request logging helpers
func LogHTTPRequest(method, path string, status int, duration time.Duration, requestID string, userID uint, err error) {
	fields := map[string]interface{}{
		"method":     method,
		"path":       path,
		"status":     status,
		"duration":   duration.String(),
		"request_id": requestID,
	}
	
	if userID > 0 {
		fields["user_id"] = userID
	}
	
	message := fmt.Sprintf("%s %s %d %v", method, path, status, duration)
	
	if err != nil {
		fields["error"] = err.Error()
		Error(message, fields)
	} else if status >= 400 {
		Warn(message, fields)
	} else {
		Info(message, fields)
	}
}