package middleware

import (
	"context"
	"log"
	"net/http"
	"runtime/debug"
	"time"
)

// ErrorHandlingMiddleware provides centralized error handling and recovery
func ErrorHandling(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// Log the panic with stack trace
				log.Printf("PANIC: %v\nStack trace:\n%s", err, debug.Stack())
				
				// Return a generic error response
				writeJSONError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", 
					"An unexpected error occurred. Please try again later.")
			}
		}()
		
		next.ServeHTTP(w, r)
	})
}

// RequestID middleware adds a unique request ID to each request
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Generate a simple request ID (in production, use a proper UUID library)
		requestID := generateRequestID()
		
		// Add request ID to response headers
		w.Header().Set("X-Request-ID", requestID)
		
		// Add request ID to request context for logging
		ctx := r.Context()
		ctx = setRequestID(ctx, requestID)
		
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// generateRequestID generates a simple request ID
func generateRequestID() string {
	// Simple implementation - in production use proper UUID
	return time.Now().Format("20060102150405") + "-" + randomString(6)
}

// randomString generates a random string of given length
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}

// Context helpers for request ID
type requestContextKey string

const requestIDKey requestContextKey = "request_id"

func setRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

func GetRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value(requestIDKey).(string); ok {
		return requestID
	}
	return ""
}

// SecurityHeaders middleware adds security headers
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Security headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		
		// Content Security Policy (adjust as needed for your frontend)
		w.Header().Set("Content-Security-Policy", 
			"default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self' ws: wss:")
		
		next.ServeHTTP(w, r)
	})
}

// MethodNotAllowed handles HTTP method not allowed errors
func MethodNotAllowed() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSONError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", 
			"HTTP method not allowed for this endpoint")
	})
}

// NotFound handles 404 errors
func NotFound() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSONError(w, http.StatusNotFound, "ENDPOINT_NOT_FOUND", 
			"The requested endpoint was not found")
	})
}