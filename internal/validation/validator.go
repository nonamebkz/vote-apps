package validation

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"unicode"
)

// ValidationError represents a single validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"`
}

// ValidationResult holds the result of validation
type ValidationResult struct {
	IsValid bool              `json:"is_valid"`
	Errors  []ValidationError `json:"errors"`
}

// Validator provides validation functionality
type Validator struct {
	errors []ValidationError
}

// NewValidator creates a new validator instance
func NewValidator() *Validator {
	return &Validator{
		errors: make([]ValidationError, 0),
	}
}

// AddError adds a validation error
func (v *Validator) AddError(field, message, value string) {
	v.errors = append(v.errors, ValidationError{
		Field:   field,
		Message: message,
		Value:   value,
	})
}

// IsValid returns true if there are no validation errors
func (v *Validator) IsValid() bool {
	return len(v.errors) == 0
}

// GetErrors returns all validation errors
func (v *Validator) GetErrors() []ValidationError {
	return v.errors
}

// GetResult returns the validation result
func (v *Validator) GetResult() ValidationResult {
	return ValidationResult{
		IsValid: v.IsValid(),
		Errors:  v.errors,
	}
}

// String validation functions

// ValidateRequired checks if a string field is not empty
func (v *Validator) ValidateRequired(field, value string) *Validator {
	if strings.TrimSpace(value) == "" {
		v.AddError(field, "This field is required", value)
	}
	return v
}

// ValidateMinLength checks minimum string length
func (v *Validator) ValidateMinLength(field, value string, minLength int) *Validator {
	if len(strings.TrimSpace(value)) < minLength {
		v.AddError(field, fmt.Sprintf("Must be at least %d characters long", minLength), value)
	}
	return v
}

// ValidateMaxLength checks maximum string length
func (v *Validator) ValidateMaxLength(field, value string, maxLength int) *Validator {
	if len(value) > maxLength {
		v.AddError(field, fmt.Sprintf("Must be no more than %d characters long", maxLength), value)
	}
	return v
}

// ValidateLength checks exact string length
func (v *Validator) ValidateLength(field, value string, length int) *Validator {
	if len(strings.TrimSpace(value)) != length {
		v.AddError(field, fmt.Sprintf("Must be exactly %d characters long", length), value)
	}
	return v
}

// ValidateEmail checks if string is a valid email format
func (v *Validator) ValidateEmail(field, value string) *Validator {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if value != "" && !emailRegex.MatchString(value) {
		v.AddError(field, "Must be a valid email address", value)
	}
	return v
}

// ValidateUsername checks if string is a valid username
func (v *Validator) ValidateUsername(field, value string) *Validator {
	// Username should contain only alphanumeric characters, underscores, and hyphens
	usernameRegex := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if value != "" && !usernameRegex.MatchString(value) {
		v.AddError(field, "Username can only contain letters, numbers, underscores, and hyphens", value)
	}
	return v
}

// ValidatePassword checks password strength
func (v *Validator) ValidatePassword(field, value string) *Validator {
	if len(value) < 6 {
		v.AddError(field, "Password must be at least 6 characters long", "")
		return v
	}
	
	// Check for at least one letter and one number (basic strength check)
	hasLetter := false
	hasNumber := false
	
	for _, char := range value {
		if unicode.IsLetter(char) {
			hasLetter = true
		}
		if unicode.IsNumber(char) {
			hasNumber = true
		}
	}
	
	if !hasLetter {
		v.AddError(field, "Password must contain at least one letter", "")
	}
	
	if !hasNumber {
		v.AddError(field, "Password must contain at least one number", "")
	}
	
	return v
}

// ValidateOneOf checks if value is one of the allowed values
func (v *Validator) ValidateOneOf(field, value string, allowedValues []string) *Validator {
	if value == "" {
		return v
	}
	
	for _, allowed := range allowedValues {
		if value == allowed {
			return v
		}
	}
	
	v.AddError(field, fmt.Sprintf("Must be one of: %s", strings.Join(allowedValues, ", ")), value)
	return v
}

// ValidatePattern checks if value matches a regex pattern
func (v *Validator) ValidatePattern(field, value, pattern, message string) *Validator {
	if value == "" {
		return v
	}
	
	regex, err := regexp.Compile(pattern)
	if err != nil {
		v.AddError(field, "Invalid validation pattern", value)
		return v
	}
	
	if !regex.MatchString(value) {
		if message == "" {
			message = fmt.Sprintf("Must match pattern: %s", pattern)
		}
		v.AddError(field, message, value)
	}
	
	return v
}

// Numeric validation functions

// ValidateMin checks minimum numeric value
func (v *Validator) ValidateMin(field string, value, min int) *Validator {
	if value < min {
		v.AddError(field, fmt.Sprintf("Must be at least %d", min), fmt.Sprintf("%d", value))
	}
	return v
}

// ValidateMax checks maximum numeric value
func (v *Validator) ValidateMax(field string, value, max int) *Validator {
	if value > max {
		v.AddError(field, fmt.Sprintf("Must be no more than %d", max), fmt.Sprintf("%d", value))
	}
	return v
}

// ValidateRange checks if numeric value is within range
func (v *Validator) ValidateRange(field string, value, min, max int) *Validator {
	if value < min || value > max {
		v.AddError(field, fmt.Sprintf("Must be between %d and %d", min, max), fmt.Sprintf("%d", value))
	}
	return v
}

// Array validation functions

// ValidateArrayMinLength checks minimum array length
func (v *Validator) ValidateArrayMinLength(field string, array []interface{}, minLength int) *Validator {
	if len(array) < minLength {
		v.AddError(field, fmt.Sprintf("Must have at least %d items", minLength), fmt.Sprintf("%d items", len(array)))
	}
	return v
}

// ValidateArrayMaxLength checks maximum array length
func (v *Validator) ValidateArrayMaxLength(field string, array []interface{}, maxLength int) *Validator {
	if len(array) > maxLength {
		v.AddError(field, fmt.Sprintf("Must have no more than %d items", maxLength), fmt.Sprintf("%d items", len(array)))
	}
	return v
}

// Sanitization functions

// SanitizeString removes potentially dangerous characters and trims whitespace
func SanitizeString(input string) string {
	// Trim whitespace
	sanitized := strings.TrimSpace(input)
	
	// HTML escape to prevent XSS
	sanitized = html.EscapeString(sanitized)
	
	return sanitized
}

// SanitizeUsername removes non-alphanumeric characters except underscores and hyphens
func SanitizeUsername(input string) string {
	// Remove all characters except letters, numbers, underscores, and hyphens
	reg := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	sanitized := reg.ReplaceAllString(input, "")
	
	// Trim whitespace
	sanitized = strings.TrimSpace(sanitized)
	
	return sanitized
}

// SanitizeHTML removes HTML tags and escapes remaining content
func SanitizeHTML(input string) string {
	// Remove HTML tags
	htmlTagRegex := regexp.MustCompile(`<[^>]*>`)
	sanitized := htmlTagRegex.ReplaceAllString(input, "")
	
	// HTML escape the remaining content
	sanitized = html.EscapeString(sanitized)
	
	// Trim whitespace
	sanitized = strings.TrimSpace(sanitized)
	
	return sanitized
}

// SanitizeSQL removes potentially dangerous SQL characters
func SanitizeSQL(input string) string {
	// Remove common SQL injection characters
	dangerous := []string{"'", "\"", ";", "--", "/*", "*/", "xp_", "sp_"}
	
	sanitized := input
	for _, char := range dangerous {
		sanitized = strings.ReplaceAll(sanitized, char, "")
	}
	
	return strings.TrimSpace(sanitized)
}

// RemoveControlCharacters removes control characters from string
func RemoveControlCharacters(input string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return -1 // Remove the character
		}
		return r
	}, input)
}

// TruncateString truncates string to maximum length
func TruncateString(input string, maxLength int) string {
	if len(input) <= maxLength {
		return input
	}
	
	// Truncate and add ellipsis if needed
	if maxLength > 3 {
		return input[:maxLength-3] + "..."
	}
	
	return input[:maxLength]
}