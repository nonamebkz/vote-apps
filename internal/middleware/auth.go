package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"polling-system/internal/models"
	"polling-system/pkg/auth"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	UserContextKey contextKey = "user"
)

// ErrorResponse represents an error response for middleware
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail represents error details for middleware
type ErrorDetail struct {
	Code      string    `json:"code"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// writeJSONError writes a JSON error response
func writeJSONError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	
	response := ErrorResponse{
		Error: ErrorDetail{
			Code:      code,
			Message:   message,
			Timestamp: time.Now(),
		},
	}
	
	json.NewEncoder(w).Encode(response)
}

// JWTAuth middleware validates JWT tokens
func JWTAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get the Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeJSONError(w, http.StatusUnauthorized, "MISSING_AUTH_HEADER", "Authorization header required")
			return
		}

		// Check if it starts with "Bearer "
		if !strings.HasPrefix(authHeader, "Bearer ") {
			writeJSONError(w, http.StatusUnauthorized, "INVALID_AUTH_FORMAT", "Authorization header must start with 'Bearer '")
			return
		}

		// Extract the token
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == "" {
			writeJSONError(w, http.StatusUnauthorized, "EMPTY_TOKEN", "Token cannot be empty")
			return
		}

		// Validate the token
		claims, err := auth.ValidateToken(tokenString)
		if err != nil {
			writeJSONError(w, http.StatusUnauthorized, "INVALID_TOKEN", "Invalid or expired token")
			return
		}

		// Check if token is expired (additional check)
		if claims.ExpiresAt != nil && claims.ExpiresAt.Before(time.Now()) {
			writeJSONError(w, http.StatusUnauthorized, "TOKEN_EXPIRED", "Token has expired")
			return
		}

		// Add user info to context
		ctx := context.WithValue(r.Context(), UserContextKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AdminOnly middleware ensures only admin users can access the endpoint
func AdminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get user from context
		claims, ok := r.Context().Value(UserContextKey).(*auth.Claims)
		if !ok {
			writeJSONError(w, http.StatusUnauthorized, "USER_NOT_FOUND", "User not found in context")
			return
		}

		// Check if user is admin
		if claims.Role != string(models.RoleAdmin) {
			writeJSONError(w, http.StatusForbidden, "ADMIN_REQUIRED", "Admin access required")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// VoterOnly middleware ensures only voter users can access the endpoint
func VoterOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get user from context
		claims, ok := r.Context().Value(UserContextKey).(*auth.Claims)
		if !ok {
			writeJSONError(w, http.StatusUnauthorized, "USER_NOT_FOUND", "User not found in context")
			return
		}

		// Check if user is voter
		if claims.Role != string(models.RoleVoter) {
			writeJSONError(w, http.StatusForbidden, "VOTER_REQUIRED", "Voter access required")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RoleRequired middleware ensures user has one of the specified roles
func RoleRequired(allowedRoles ...models.UserRole) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get user from context
			claims, ok := r.Context().Value(UserContextKey).(*auth.Claims)
			if !ok {
				writeJSONError(w, http.StatusUnauthorized, "USER_NOT_FOUND", "User not found in context")
				return
			}

			// Check if user has any of the allowed roles
			userRole := models.UserRole(claims.Role)
			for _, role := range allowedRoles {
				if userRole == role {
					next.ServeHTTP(w, r)
					return
				}
			}

			writeJSONError(w, http.StatusForbidden, "INSUFFICIENT_PERMISSIONS", "Insufficient permissions for this resource")
		})
	}
}

// GetUserFromContext extracts user claims from request context
func GetUserFromContext(r *http.Request) (*auth.Claims, bool) {
	claims, ok := r.Context().Value(UserContextKey).(*auth.Claims)
	return claims, ok
}

// RequireActiveUser middleware ensures the user account is active
func RequireActiveUser(userService interface{}) func(http.Handler) http.Handler {
	// For now, we'll implement a basic version that checks the token validity
	// In a full implementation, this would check the user's status in the database
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get user from context
			_, ok := r.Context().Value(UserContextKey).(*auth.Claims)
			if !ok {
				writeJSONError(w, http.StatusUnauthorized, "USER_NOT_FOUND", "User not found in context")
				return
			}

			// TODO: In a full implementation, check user status in database
			// For now, we assume if the token is valid, the user is active
			
			next.ServeHTTP(w, r)
		})
	}
}