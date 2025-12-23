package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"polling-system/internal/validation"
)

// ValidationError represents a validation error for middleware
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"`
}

// WriteValidationError writes a validation error response
func WriteValidationError(w http.ResponseWriter, errors []ValidationError) {
	response := ErrorResponse{
		Error: ErrorDetail{
			Code:    "VALIDATION_ERROR",
			Message: "Request validation failed",
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(response)
}

// ValidationMiddleware provides request validation and sanitization
func ValidationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only validate JSON requests
		if r.Header.Get("Content-Type") == "application/json" && r.Body != nil {
			// Read the body
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST_BODY", 
					"Failed to read request body")
				return
			}
			
			// Restore the body for the next handler
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			
			// Parse JSON to check for basic structure
			var jsonData map[string]interface{}
			if err := json.Unmarshal(bodyBytes, &jsonData); err != nil {
				writeJSONError(w, http.StatusBadRequest, "INVALID_JSON", 
					"Request body must be valid JSON")
				return
			}
			
			// Sanitize string values in the JSON
			sanitizedData := sanitizeJSONData(jsonData)
			
			// Re-encode the sanitized data
			sanitizedBytes, err := json.Marshal(sanitizedData)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, "SANITIZATION_ERROR", 
					"Failed to sanitize request data")
				return
			}
			
			// Replace the body with sanitized data
			r.Body = io.NopCloser(bytes.NewBuffer(sanitizedBytes))
			r.ContentLength = int64(len(sanitizedBytes))
		}
		
		// Sanitize query parameters
		sanitizeQueryParams(r)
		
		// Sanitize path parameters (if needed)
		sanitizePathParams(r)
		
		next.ServeHTTP(w, r)
	})
}

// sanitizeJSONData recursively sanitizes string values in JSON data
func sanitizeJSONData(data interface{}) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		sanitized := make(map[string]interface{})
		for key, value := range v {
			sanitized[key] = sanitizeJSONData(value)
		}
		return sanitized
	case []interface{}:
		sanitized := make([]interface{}, len(v))
		for i, value := range v {
			sanitized[i] = sanitizeJSONData(value)
		}
		return sanitized
	case string:
		return validation.SanitizeString(v)
	default:
		return v
	}
}

// sanitizeQueryParams sanitizes URL query parameters
func sanitizeQueryParams(r *http.Request) {
	query := r.URL.Query()
	for key, values := range query {
		for i, value := range values {
			query[key][i] = validation.SanitizeString(value)
		}
	}
	r.URL.RawQuery = query.Encode()
}

// sanitizePathParams sanitizes path parameters (basic implementation)
func sanitizePathParams(r *http.Request) {
	// Basic sanitization of the path
	r.URL.Path = validation.RemoveControlCharacters(r.URL.Path)
}

// ContentTypeValidation middleware ensures proper content type for JSON endpoints
func ContentTypeValidation(requiredType string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only check content type for requests with body
			if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" {
				contentType := r.Header.Get("Content-Type")
				
				// Remove charset if present
				if idx := strings.Index(contentType, ";"); idx != -1 {
					contentType = contentType[:idx]
				}
				
				if contentType != requiredType {
					writeJSONError(w, http.StatusUnsupportedMediaType, "INVALID_CONTENT_TYPE", 
						"Content-Type must be "+requiredType)
					return
				}
			}
			
			next.ServeHTTP(w, r)
		})
	}
}

// MaxBodySize middleware limits request body size
func MaxBodySize(maxSize int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > maxSize {
				writeJSONError(w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", 
					"Request body too large")
				return
			}
			
			// Limit the reader to prevent large requests
			r.Body = http.MaxBytesReader(w, r.Body, maxSize)
			
			next.ServeHTTP(w, r)
		})
	}
}

// ValidateJSONStructure validates that JSON has required fields
func ValidateJSONStructure(requiredFields []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only validate JSON requests with body
			if r.Header.Get("Content-Type") == "application/json" && r.Body != nil {
				// Read the body
				bodyBytes, err := io.ReadAll(r.Body)
				if err != nil {
					writeJSONError(w, http.StatusBadRequest, "INVALID_REQUEST_BODY", 
						"Failed to read request body")
					return
				}
				
				// Restore the body for the next handler
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				
				// Parse JSON
				var jsonData map[string]interface{}
				if err := json.Unmarshal(bodyBytes, &jsonData); err != nil {
					writeJSONError(w, http.StatusBadRequest, "INVALID_JSON", 
						"Request body must be valid JSON")
					return
				}
				
				// Check for required fields
				var validationErrors []ValidationError
				for _, field := range requiredFields {
					if _, exists := jsonData[field]; !exists {
						validationErrors = append(validationErrors, ValidationError{
							Field:   field,
							Message: "This field is required",
							Value:   "",
						})
					}
				}
				
				if len(validationErrors) > 0 {
					WriteValidationError(w, validationErrors)
					return
				}
			}
			
			next.ServeHTTP(w, r)
		})
	}
}

// NoScriptInjection middleware prevents script injection in requests
func NoScriptInjection(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for script tags in query parameters
		for _, values := range r.URL.Query() {
			for _, value := range values {
				if containsScriptTags(value) {
					writeJSONError(w, http.StatusBadRequest, "SCRIPT_INJECTION_DETECTED", 
						"Script injection attempt detected")
					return
				}
			}
		}
		
		// Check for script tags in headers (user agent, referer, etc.)
		suspiciousHeaders := []string{"User-Agent", "Referer", "X-Forwarded-For"}
		for _, header := range suspiciousHeaders {
			if value := r.Header.Get(header); containsScriptTags(value) {
				writeJSONError(w, http.StatusBadRequest, "SCRIPT_INJECTION_DETECTED", 
					"Script injection attempt detected in headers")
				return
			}
		}
		
		next.ServeHTTP(w, r)
	})
}

// containsScriptTags checks if a string contains script tags or other dangerous patterns
func containsScriptTags(input string) bool {
	input = strings.ToLower(input)
	
	dangerousPatterns := []string{
		"<script",
		"</script>",
		"javascript:",
		"vbscript:",
		"onload=",
		"onerror=",
		"onclick=",
		"onmouseover=",
		"<iframe",
		"<object",
		"<embed",
		"<link",
		"<meta",
	}
	
	for _, pattern := range dangerousPatterns {
		if strings.Contains(input, pattern) {
			return true
		}
	}
	
	return false
}

// SQLInjectionPrevention middleware prevents basic SQL injection attempts
func SQLInjectionPrevention(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check query parameters for SQL injection patterns
		for _, values := range r.URL.Query() {
			for _, value := range values {
				if containsSQLInjection(value) {
					writeJSONError(w, http.StatusBadRequest, "SQL_INJECTION_DETECTED", 
						"SQL injection attempt detected")
					return
				}
			}
		}
		
		next.ServeHTTP(w, r)
	})
}

// containsSQLInjection checks for common SQL injection patterns
func containsSQLInjection(input string) bool {
	input = strings.ToLower(input)
	
	sqlPatterns := []string{
		"' or '1'='1",
		"' or 1=1",
		"' union select",
		"' drop table",
		"' delete from",
		"' insert into",
		"' update ",
		"; drop table",
		"; delete from",
		"; insert into",
		"; update ",
		"--",
		"/*",
		"*/",
		"xp_",
		"sp_cmdshell",
	}
	
	for _, pattern := range sqlPatterns {
		if strings.Contains(input, pattern) {
			return true
		}
	}
	
	return false
}