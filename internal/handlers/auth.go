package handlers

import (
	"net/http"

	"polling-system/internal/middleware"
	"polling-system/internal/models"
	"polling-system/internal/services"
)

// AuthHandler handles authentication related requests
type AuthHandler struct {
	authService *services.AuthService
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(authService *services.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

// RegisterRequest represents a registration request
type RegisterRequest struct {
	Username string `json:"username" validate:"required,min=3,max=50"`
	Password string `json:"password" validate:"required,min=6"`
	Role     string `json:"role" validate:"required,oneof=admin voter"`
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username  string `json:"username" validate:"required"`
	Password  string `json:"password" validate:"required"`
	ReturnURL string `json:"return_url,omitempty"`
}

// RefreshTokenRequest represents a refresh token request
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// Register handles user registration
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST method is allowed", nil)
		return
	}

	var req RegisterRequest
	if err := ParseJSONBody(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON format", map[string]string{"error": err.Error()})
		return
	}

	// Validate required fields
	if req.Username == "" {
		WriteError(w, http.StatusBadRequest, "MISSING_USERNAME", "Username is required", nil)
		return
	}
	if req.Password == "" {
		WriteError(w, http.StatusBadRequest, "MISSING_PASSWORD", "Password is required", nil)
		return
	}
	if req.Role == "" {
		WriteError(w, http.StatusBadRequest, "MISSING_ROLE", "Role is required", nil)
		return
	}

	// Validate username length
	if len(req.Username) < 3 || len(req.Username) > 50 {
		WriteError(w, http.StatusBadRequest, "INVALID_USERNAME", "Username must be between 3 and 50 characters", nil)
		return
	}

	// Validate password length
	if len(req.Password) < 6 {
		WriteError(w, http.StatusBadRequest, "WEAK_PASSWORD", "Password must be at least 6 characters long", nil)
		return
	}

	// Validate role
	if req.Role != "admin" && req.Role != "voter" {
		WriteError(w, http.StatusBadRequest, "INVALID_ROLE", "Role must be either 'admin' or 'voter'", nil)
		return
	}

	// Convert to service request
	serviceReq := services.RegisterRequest{
		Username: req.Username,
		Password: req.Password,
		Role:     models.UserRole(req.Role),
	}

	// Register user
	response, err := h.authService.Register(serviceReq)
	if err != nil {
		if err.Error() == "username already exists" {
			WriteError(w, http.StatusConflict, "USERNAME_EXISTS", "Username already exists", nil)
			return
		}
		WriteError(w, http.StatusInternalServerError, "REGISTRATION_FAILED", "Failed to register user", map[string]string{"error": err.Error()})
		return
	}

	// Return success response
	WriteSuccess(w, map[string]interface{}{
		"user": map[string]interface{}{
			"id":       response.User.ID,
			"username": response.User.Username,
			"role":     response.User.Role,
			"voter_id": response.User.VoterID,
		},
		"access_token":  response.AccessToken,
		"refresh_token": response.RefreshToken,
		"expires_in":    response.ExpiresIn,
	}, "User registered successfully")
}

// Login handles user login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST method is allowed", nil)
		return
	}

	var req LoginRequest
	if err := ParseJSONBody(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON format", map[string]string{"error": err.Error()})
		return
	}

	// Validate required fields
	if req.Username == "" {
		WriteError(w, http.StatusBadRequest, "MISSING_USERNAME", "Username is required", nil)
		return
	}
	if req.Password == "" {
		WriteError(w, http.StatusBadRequest, "MISSING_PASSWORD", "Password is required", nil)
		return
	}

	// Convert to service request
	serviceReq := services.LoginRequest{
		Username: req.Username,
		Password: req.Password,
	}

	// Authenticate user
	response, err := h.authService.Login(serviceReq)
	if err != nil {
		if err.Error() == "invalid username or password" || err.Error() == "user account is inactive" {
			WriteError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", err.Error(), nil)
			return
		}
		WriteError(w, http.StatusInternalServerError, "LOGIN_FAILED", "Failed to authenticate user", map[string]string{"error": err.Error()})
		return
	}

	// Return success response
	WriteSuccess(w, map[string]interface{}{
		"user": map[string]interface{}{
			"id":       response.User.ID,
			"username": response.User.Username,
			"role":     response.User.Role,
			"voter_id": response.User.VoterID,
		},
		"access_token":  response.AccessToken,
		"refresh_token": response.RefreshToken,
		"expires_in":    response.ExpiresIn,
		"return_url":    req.ReturnURL,
	}, "Login successful")
}

// RefreshToken handles token refresh
func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST method is allowed", nil)
		return
	}

	var req RefreshTokenRequest
	if err := ParseJSONBody(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON format", map[string]string{"error": err.Error()})
		return
	}

	// Validate required fields
	if req.RefreshToken == "" {
		WriteError(w, http.StatusBadRequest, "MISSING_REFRESH_TOKEN", "Refresh token is required", nil)
		return
	}

	// Refresh token
	response, err := h.authService.RefreshToken(req.RefreshToken)
	if err != nil {
		if err.Error() == "invalid refresh token" || err.Error() == "user account is inactive" {
			WriteError(w, http.StatusUnauthorized, "INVALID_REFRESH_TOKEN", err.Error(), nil)
			return
		}
		WriteError(w, http.StatusInternalServerError, "REFRESH_FAILED", "Failed to refresh token", map[string]string{"error": err.Error()})
		return
	}

	// Return success response
	WriteSuccess(w, map[string]interface{}{
		"user": map[string]interface{}{
			"id":       response.User.ID,
			"username": response.User.Username,
			"role":     response.User.Role,
			"voter_id": response.User.VoterID,
		},
		"access_token":  response.AccessToken,
		"refresh_token": response.RefreshToken,
		"expires_in":    response.ExpiresIn,
	}, "Token refreshed successfully")
}

// Logout handles user logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST method is allowed", nil)
		return
	}

	// Get user from context (if authenticated)
	claims, ok := middleware.GetUserFromContext(r)
	if ok {
		// In a full implementation, you might want to blacklist the token
		// or store logout information in the database
		WriteSuccess(w, map[string]interface{}{
			"user_id": claims.UserID,
		}, "Logout successful")
	} else {
		WriteSuccess(w, nil, "Logout successful")
	}
}

// GetProfile handles getting current user profile
func (h *AuthHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET method is allowed", nil)
		return
	}

	// Get user from context
	claims, ok := middleware.GetUserFromContext(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, "USER_NOT_FOUND", "User not found in context", nil)
		return
	}

	// Get full user details from database
	user, err := h.authService.GetUserFromToken("")
	if err != nil {
		// Fallback to claims data
		WriteSuccess(w, map[string]interface{}{
			"id":       claims.UserID,
			"username": claims.Username,
			"role":     claims.Role,
		}, "Profile retrieved successfully")
		return
	}

	WriteSuccess(w, map[string]interface{}{
		"id":         user.ID,
		"username":   user.Username,
		"role":       user.Role,
		"voter_id":   user.VoterID,
		"status":     user.Status,
		"created_at": user.CreatedAt,
		"updated_at": user.UpdatedAt,
	}, "Profile retrieved successfully")
}

// VerifyToken handles token verification
func (h *AuthHandler) VerifyToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST method is allowed", nil)
		return
	}

	// Get user from context (middleware already validated the token)
	claims, ok := middleware.GetUserFromContext(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, "INVALID_TOKEN", "Invalid token", nil)
		return
	}

	WriteSuccess(w, map[string]interface{}{
		"valid":    true,
		"user_id":  claims.UserID,
		"username": claims.Username,
		"role":     claims.Role,
		"expires_at": claims.ExpiresAt,
	}, "Token is valid")
}