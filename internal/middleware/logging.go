package middleware

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"time"

	"polling-system/internal/logger"
	"polling-system/pkg/auth"
)

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not implement http.Hijacker")
	}
	return h.Hijack()
}

// Logging middleware logs HTTP requests with structured logging
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap the response writer to capture status code
		wrapped := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Call the next handler
		next.ServeHTTP(wrapped, r)

		// Calculate duration
		duration := time.Since(start)

		// Get request ID from context
		requestID := GetRequestID(r.Context())

		// Get user ID from context if available
		var userID uint
		if claims, ok := r.Context().Value(UserContextKey).(*auth.Claims); ok {
			userID = claims.UserID
		}

		// Log the request using structured logging
		logger.LogHTTPRequest(
			r.Method,
			r.RequestURI,
			wrapped.statusCode,
			duration,
			requestID,
			userID,
			nil, // No error for successful requests
		)
	})
}
