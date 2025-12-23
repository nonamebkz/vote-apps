package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail represents error details
type ErrorDetail struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// SuccessResponse represents a success response
type SuccessResponse struct {
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
}

// WriteJSON writes a JSON response
func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// WriteError writes an error response
func WriteError(w http.ResponseWriter, status int, code, message string, details interface{}) {
	response := ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
			Details: details,
		},
	}
	WriteJSON(w, status, response)
}

// WriteSuccess writes a success response
func WriteSuccess(w http.ResponseWriter, data interface{}, message string) {
	response := SuccessResponse{
		Data:    data,
		Message: message,
	}
	WriteJSON(w, http.StatusOK, response)
}

// WriteFile writes a file response with appropriate headers
func WriteFile(w http.ResponseWriter, data []byte, filename, contentType string) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	
	if _, err := w.Write(data); err != nil {
		WriteError(w, http.StatusInternalServerError, "FILE_WRITE_ERROR", "Failed to write file", nil)
	}
}

// GetPathParam extracts a path parameter from the request
func GetPathParam(r *http.Request, key string) string {
	vars := mux.Vars(r)
	return vars[key]
}

// GetPathParamUint extracts a uint path parameter from the request
func GetPathParamUint(r *http.Request, key string) (uint, error) {
	param := GetPathParam(r, key)
	if param == "" {
		return 0, fmt.Errorf("missing parameter: %s", key)
	}
	
	value, err := strconv.ParseUint(param, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid parameter %s: %s", key, param)
	}
	
	return uint(value), nil
}

// GetQueryParam extracts a query parameter from the request
func GetQueryParam(r *http.Request, key string) string {
	return r.URL.Query().Get(key)
}

// GetQueryParamInt extracts an int query parameter from the request
func GetQueryParamInt(r *http.Request, key string, defaultValue int) int {
	param := GetQueryParam(r, key)
	if param == "" {
		return defaultValue
	}
	
	value, err := strconv.Atoi(param)
	if err != nil {
		return defaultValue
	}
	
	return value
}

// ParseJSONBody parses JSON request body into the provided struct
func ParseJSONBody(r *http.Request, v interface{}) error {
	if r.Body == nil {
		return fmt.Errorf("request body is empty")
	}
	
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	
	if err := decoder.Decode(v); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	
	return nil
}

// GetContentType returns the appropriate content type for export format
func GetContentType(format string) string {
	switch format {
	case "pdf":
		return "application/pdf"
	case "excel":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	default:
		return "application/octet-stream"
	}
}

// ValidationError represents a validation error
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
			Details: map[string]interface{}{
				"validation_errors": errors,
			},
		},
	}
	WriteJSON(w, http.StatusBadRequest, response)
}