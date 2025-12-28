package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"polling-system/internal/middleware"
	"polling-system/internal/models"
	"polling-system/internal/services"
	"polling-system/pkg/export"
)

// AdminHandler handles admin related requests
type AdminHandler struct {
	userService   *services.UserService
	pollService   *services.PollService
	voteService   *services.VoteService
	exportService *services.ExportService
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(userService *services.UserService, pollService *services.PollService, voteService *services.VoteService, exportService *services.ExportService) *AdminHandler {
	return &AdminHandler{
		userService:   userService,
		pollService:   pollService,
		voteService:   voteService,
		exportService: exportService,
	}
}

// CreateUserRequest represents a user creation request
type CreateUserRequest struct {
	Username string          `json:"username" validate:"required,min=3,max=50"`
	Password string          `json:"password" validate:"required,min=6"`
	Role     models.UserRole `json:"role" validate:"required,oneof=admin voter"`
}

// UpdateUserStatusRequest represents a user status update request
type UpdateUserStatusRequest struct {
	Status models.UserStatus `json:"status" validate:"required,oneof=active inactive"`
}

// ListUsers handles listing users
func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
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

	// Only admins can list users
	if claims.Role != string(models.RoleAdmin) {
		WriteError(w, http.StatusForbidden, "ADMIN_REQUIRED", "Only admins can list users", nil)
		return
	}

	// Get query parameters for filtering and pagination
	roleParam := GetQueryParam(r, "role")
	statusParam := GetQueryParam(r, "status")
	page := GetQueryParamInt(r, "page", 1)
	limit := GetQueryParamInt(r, "limit", 20)

	// Validate pagination parameters
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	// Calculate offset
	offset := (page - 1) * limit

	// Parse filters
	var role *models.UserRole
	if roleParam != "" {
		r := models.UserRole(roleParam)
		role = &r
	}

	var status *models.UserStatus
	if statusParam != "" {
		s := models.UserStatus(statusParam)
		status = &s
	}

	// Build filter
	filter := services.UserListFilter{
		Role:   role,
		Status: status,
		Limit:  limit,
		Offset: offset,
	}

	// List users using service
	users, total, err := h.userService.ListUsers(claims.UserID, filter)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "LIST_FAILED", "Failed to list users", map[string]string{"error": err.Error()})
		return
	}

	// Calculate pagination info
	totalPages := (total + int64(limit) - 1) / int64(limit)

	WriteSuccess(w, map[string]interface{}{
		"users": users,
		"pagination": map[string]interface{}{
			"page":        page,
			"limit":       limit,
			"total":       total,
			"total_pages": totalPages,
		},
	}, "Users retrieved successfully")
}

// CreateUser handles creating a new user
func (h *AdminHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST method is allowed", nil)
		return
	}

	// Get user from context
	claims, ok := middleware.GetUserFromContext(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, "USER_NOT_FOUND", "User not found in context", nil)
		return
	}

	// Only admins can create users
	if claims.Role != string(models.RoleAdmin) {
		WriteError(w, http.StatusForbidden, "ADMIN_REQUIRED", "Only admins can create users", nil)
		return
	}

	var req CreateUserRequest
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

	// Convert to service request
	serviceReq := services.CreateUserRequest{
		Username: req.Username,
		Password: req.Password,
		Role:     req.Role,
	}

	// Create user
	user, err := h.userService.CreateUser(claims.UserID, serviceReq)
	if err != nil {
		if err.Error() == "username already exists" {
			WriteError(w, http.StatusConflict, "USERNAME_EXISTS", "Username already exists", nil)
			return
		}
		WriteError(w, http.StatusInternalServerError, "CREATE_FAILED", "Failed to create user", map[string]string{"error": err.Error()})
		return
	}

	WriteSuccess(w, map[string]interface{}{
		"id":         user.ID,
		"username":   user.Username,
		"role":       user.Role,
		"status":     user.Status,
		"voter_id":   user.VoterID,
		"created_at": user.CreatedAt,
	}, "User created successfully")
}

// UpdateUserStatus handles updating user status
func (h *AdminHandler) UpdateUserStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only PUT method is allowed", nil)
		return
	}

	// Extract user ID from path
	userID, err := GetPathParamUint(r, "id")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID", nil)
		return
	}

	// Get user from context
	claims, ok := middleware.GetUserFromContext(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, "USER_NOT_FOUND", "User not found in context", nil)
		return
	}

	// Only admins can update user status
	if claims.Role != string(models.RoleAdmin) {
		WriteError(w, http.StatusForbidden, "ADMIN_REQUIRED", "Only admins can update user status", nil)
		return
	}

	// Prevent admin from deactivating themselves
	if userID == claims.UserID {
		WriteError(w, http.StatusConflict, "CANNOT_MODIFY_SELF", "Cannot modify your own status", nil)
		return
	}

	var req UpdateUserStatusRequest
	if err := ParseJSONBody(r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON format", map[string]string{"error": err.Error()})
		return
	}

	// Validate status
	if req.Status != models.UserStatusActive && req.Status != models.UserStatusInactive {
		WriteError(w, http.StatusBadRequest, "INVALID_STATUS", "Status must be 'active' or 'inactive'", nil)
		return
	}

	// Update user status
	var user *models.User
	if req.Status == models.UserStatusActive {
		user, err = h.userService.ActivateUser(claims.UserID, userID)
	} else {
		user, err = h.userService.DeactivateUser(claims.UserID, userID)
	}

	if err != nil {
		if err.Error() == "user not found" {
			WriteError(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found", nil)
			return
		}
		WriteError(w, http.StatusInternalServerError, "UPDATE_FAILED", "Failed to update user status", map[string]string{"error": err.Error()})
		return
	}

	WriteSuccess(w, map[string]interface{}{
		"id":         user.ID,
		"username":   user.Username,
		"role":       user.Role,
		"status":     user.Status,
		"voter_id":   user.VoterID,
		"updated_at": user.UpdatedAt,
	}, "User status updated successfully")
}

// GetParticipation handles getting participation statistics
func (h *AdminHandler) GetParticipation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET method is allowed", nil)
		return
	}

	// Extract poll ID from path
	pollID, err := GetPathParamUint(r, "id")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_POLL_ID", "Invalid poll ID", nil)
		return
	}

	// Get user from context
	claims, ok := middleware.GetUserFromContext(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, "USER_NOT_FOUND", "User not found in context", nil)
		return
	}

	// Check permissions: Admin always, Voter only if poll is closed
	if claims.Role != string(models.RoleAdmin) {
		// Get poll to check status
		poll, err := h.pollService.GetPoll(pollID)
		if err != nil {
			if err.Error() == "poll not found" || fmt.Sprintf("%v", err) == "poll not found" {
				WriteError(w, http.StatusNotFound, "POLL_NOT_FOUND", "Poll not found", nil)
				return
			}
			WriteError(w, http.StatusInternalServerError, "POLL_LOAD_FAILED", "Failed to load poll for permission check", map[string]string{"error": err.Error()})
			return
		}

		if poll.Status != models.PollStatusClosed {
			WriteError(w, http.StatusForbidden, "ADMIN_REQUIRED", "Only admins can view real-time participation statistics. Results will be available to voters once the poll is closed.", nil)
			return
		}
	}

	// Get participation statistics using VoteService
	participationStats, err := h.voteService.GetParticipationStatistics(pollID)
	if err != nil {
		if err.Error() == "poll not found" || fmt.Sprintf("%v", err) == "poll not found" {
			WriteError(w, http.StatusNotFound, "POLL_NOT_FOUND", "Poll not found", nil)
			return
		}
		WriteError(w, http.StatusInternalServerError, "STATS_FAILED", "Failed to get participation statistics", map[string]string{"error": err.Error()})
		return
	}

	// Get detailed poll statistics for option results
	pollStats, err := h.voteService.GetPollStatistics(pollID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "STATS_FAILED", "Failed to get poll statistics", map[string]string{"error": err.Error()})
		return
	}

	// Format response to match frontend PollStatistics interface
	optionResults := make([]map[string]interface{}, 0, len(pollStats.OptionStats))
	for _, optStat := range pollStats.OptionStats {
		optionResults = append(optionResults, map[string]interface{}{
			"option_id":   optStat.OptionID,
			"option_text": optStat.OptionText,
			"vote_count":  optStat.VoteCount,
			"percentage":  optStat.Percentage,
		})
	}

	response := map[string]interface{}{
		"total_votes":        pollStats.TotalVotes,
		"participation_rate": pollStats.ParticipationRate,
		"option_results":     optionResults,
		"total_users":        participationStats["total_users"],
		"users_voted":        participationStats["users_voted"],
		"users_not_voted":    participationStats["users_not_voted"],
	}

	WriteSuccess(w, response, "Participation statistics retrieved successfully")
}

// GetDashboardStats handles getting dashboard statistics for admins
func (h *AdminHandler) GetDashboardStats(w http.ResponseWriter, r *http.Request) {
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

	// Only admins can view dashboard statistics
	if claims.Role != string(models.RoleAdmin) {
		WriteError(w, http.StatusForbidden, "ADMIN_REQUIRED", "Only admins can view dashboard statistics", nil)
		return
	}

	// Get dashboard statistics
	stats, err := h.pollService.GetDashboardStatistics()
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "STATS_FAILED", "Failed to get dashboard statistics", map[string]string{"error": err.Error()})
		return
	}

	WriteSuccess(w, stats, "Dashboard statistics retrieved successfully")
}

// ExportResults handles exporting poll results
func (h *AdminHandler) ExportResults(w http.ResponseWriter, r *http.Request) {
	// Extract poll ID from path
	pollID, err := GetPathParamUint(r, "id")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_POLL_ID", "Invalid poll ID", nil)
		return
	}

	// Get user from context
	claims, ok := middleware.GetUserFromContext(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, "USER_NOT_FOUND", "User not found in context", nil)
		return
	}

	// Only admins can export results
	if claims.Role != string(models.RoleAdmin) {
		WriteError(w, http.StatusForbidden, "ADMIN_REQUIRED", "Only admins can export poll results", nil)
		return
	}

	// Get format from query parameter (default to PDF)
	formatStr := GetQueryParam(r, "format")
	if formatStr == "" {
		formatStr = "pdf"
	}

	// Validate format
	var format export.ExportFormat
	switch formatStr {
	case "pdf":
		format = export.FormatPDF
	case "excel":
		format = export.FormatExcel
	default:
		WriteError(w, http.StatusBadRequest, "INVALID_FORMAT",
			fmt.Sprintf("Unsupported format: %s. Supported formats: pdf, excel", formatStr), nil)
		return
	}

	// Export poll results
	exportResponse, err := h.exportService.ExportPollResults(pollID, format, claims.UserID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "EXPORT_FAILED",
			fmt.Sprintf("Failed to export poll results: %v", err), nil)
		return
	}

	WriteSuccess(w, exportResponse, "Export completed successfully")
}

// DownloadExport handles downloading exported files
func (h *AdminHandler) DownloadExport(w http.ResponseWriter, r *http.Request) {
	// Extract poll ID from path
	pollID, err := GetPathParamUint(r, "id")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_POLL_ID", "Invalid poll ID", nil)
		return
	}

	// Extract filename from path
	filename := GetPathParam(r, "filename")
	if filename == "" {
		WriteError(w, http.StatusBadRequest, "MISSING_FILENAME", "Filename is required", nil)
		return
	}

	// Get user from context
	claims, ok := middleware.GetUserFromContext(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, "USER_NOT_FOUND", "User not found in context", nil)
		return
	}

	// Only admins can download exports
	if claims.Role != string(models.RoleAdmin) {
		WriteError(w, http.StatusForbidden, "ADMIN_REQUIRED", "Only admins can download exports", nil)
		return
	}

	// Verify user has permission to download this file
	if err := h.verifyDownloadPermission(pollID, claims.UserID); err != nil {
		WriteError(w, http.StatusForbidden, "ACCESS_DENIED",
			fmt.Sprintf("Access denied: %v", err), nil)
		return
	}

	// Construct file path (assuming export service stores files in a known directory)
	exportDir := h.exportService.GetExportStats()["export_directory"].(string)
	filePath := filepath.Join(exportDir, filename)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		WriteError(w, http.StatusNotFound, "FILE_NOT_FOUND", "Export file not found", nil)
		return
	}

	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "FILE_READ_ERROR",
			fmt.Sprintf("Failed to read file: %v", err), nil)
		return
	}

	// Determine content type based on file extension
	var contentType string
	ext := filepath.Ext(filename)
	switch ext {
	case ".pdf":
		contentType = "application/pdf"
	case ".xlsx":
		contentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	default:
		contentType = "application/octet-stream"
	}

	// Write file response
	WriteFile(w, data, filename, contentType)

	// Optional: Clean up file after download (uncomment if you want one-time downloads)
	// go func() {
	//     time.Sleep(5 * time.Second) // Give some time for download to complete
	//     os.Remove(filePath)
	// }()
}

// ExportResultsDirect handles direct export (returns file directly without saving)
func (h *AdminHandler) ExportResultsDirect(w http.ResponseWriter, r *http.Request) {
	// Extract poll ID from path
	pollID, err := GetPathParamUint(r, "id")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "INVALID_POLL_ID", "Invalid poll ID", nil)
		return
	}

	// Get user from context
	claims, ok := middleware.GetUserFromContext(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, "USER_NOT_FOUND", "User not found in context", nil)
		return
	}

	// Only admins can export results
	if claims.Role != string(models.RoleAdmin) {
		WriteError(w, http.StatusForbidden, "ADMIN_REQUIRED", "Only admins can export poll results", nil)
		return
	}

	// Get format from query parameter (default to PDF)
	formatStr := GetQueryParam(r, "format")
	if formatStr == "" {
		formatStr = "pdf"
	}

	// Validate format
	var format export.ExportFormat
	switch formatStr {
	case "pdf":
		format = export.FormatPDF
	case "excel":
		format = export.FormatExcel
	default:
		WriteError(w, http.StatusBadRequest, "INVALID_FORMAT",
			fmt.Sprintf("Unsupported format: %s. Supported formats: pdf, excel", formatStr), nil)
		return
	}

	// Export poll results to bytes
	data, filename, err := h.exportService.ExportPollResultsToBytes(pollID, format, claims.UserID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "EXPORT_FAILED",
			fmt.Sprintf("Failed to export poll results: %v", err), nil)
		return
	}

	// Determine content type
	contentType := GetContentType(string(format))

	// Write file response
	WriteFile(w, data, filename, contentType)
}

// GetExportFormats returns supported export formats
func (h *AdminHandler) GetExportFormats(w http.ResponseWriter, r *http.Request) {
	formats := h.exportService.GetSupportedFormats()
	WriteSuccess(w, map[string]interface{}{
		"formats": formats,
	}, "Supported export formats")
}

// CleanupExports handles cleanup of old export files
func (h *AdminHandler) CleanupExports(w http.ResponseWriter, r *http.Request) {
	// Get user from context
	claims, ok := middleware.GetUserFromContext(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, "USER_NOT_FOUND", "User not found in context", nil)
		return
	}

	// Only admins can cleanup exports
	if claims.Role != string(models.RoleAdmin) {
		WriteError(w, http.StatusForbidden, "ADMIN_REQUIRED", "Only admins can cleanup exports", nil)
		return
	}

	// Default to cleaning files older than 24 hours
	maxAge := 24 * time.Hour

	// Allow custom max age from query parameter (in hours)
	if maxAgeStr := GetQueryParam(r, "max_age_hours"); maxAgeStr != "" {
		if hours := GetQueryParamInt(r, "max_age_hours", 24); hours > 0 {
			maxAge = time.Duration(hours) * time.Hour
		}
	}

	if err := h.exportService.CleanupOldExports(maxAge); err != nil {
		WriteError(w, http.StatusInternalServerError, "CLEANUP_FAILED",
			fmt.Sprintf("Failed to cleanup exports: %v", err), nil)
		return
	}

	WriteSuccess(w, nil, fmt.Sprintf("Cleanup completed for files older than %v", maxAge))
}

// verifyDownloadPermission checks if user can download the export file
func (h *AdminHandler) verifyDownloadPermission(pollID, userID uint) error {
	// This is a simplified implementation
	// In production, you might want to store export metadata in database
	// and verify that the user who created the export is the one downloading it

	// For now, we'll just verify that the user has export permission for the poll
	// This uses the same logic as the export service
	_, _, err := h.exportService.ExportPollResultsToBytes(pollID, export.FormatPDF, userID)
	if err != nil {
		return fmt.Errorf("user does not have permission to access this poll's exports")
	}

	return nil
}
